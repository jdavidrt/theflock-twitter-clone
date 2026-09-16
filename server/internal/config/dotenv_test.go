package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotenv(t *testing.T) {
	t.Parallel()
	input := `
# a comment line
PORT=3000
export APP_ENV=test
DOUBLE="quoted value # not a comment"
SINGLE='single quoted'
INLINE=value # trailing comment
SPACED =  padded
OVERRIDDEN=first
OVERRIDDEN=second
EMPTY=
`
	got, err := ParseDotenv(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"PORT":       "3000",
		"APP_ENV":    "test",
		"DOUBLE":     "quoted value # not a comment",
		"SINGLE":     "single quoted",
		"INLINE":     "value",
		"SPACED":     "padded",
		"OVERRIDDEN": "second",
		"EMPTY":      "",
	}
	if len(got) != len(want) {
		t.Errorf("parsed %d keys, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseDotenvSyntaxErrors(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"just words", "=novalue", "BAD KEY=1", "OK=1\nbroken line"} {
		_, err := ParseDotenv(strings.NewReader(input))
		if err == nil {
			t.Errorf("input %q: expected a syntax error", input)
			continue
		}
		if !strings.Contains(err.Error(), "line ") {
			t.Errorf("input %q: error should name the line, got %q", input, err.Error())
		}
	}
	_, err := ParseDotenv(strings.NewReader("OK=1\nbroken line"))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected the error to point at line 2, got %v", err)
	}
}

// The LoadDotenv tests mutate the process environment, so they are not parallel and use
// key names no other test or real configuration uses.

func writeTempEnv(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDotenvSetsMissingAndKeepsExisting(t *testing.T) {
	const existing, fresh = "FLOCK_DOTENV_EXISTING", "FLOCK_DOTENV_FRESH"
	t.Setenv(existing, "from-process")
	t.Cleanup(func() { _ = os.Unsetenv(fresh) })
	_ = os.Unsetenv(fresh)

	path := writeTempEnv(t, ".env", existing+"=from-file\n"+fresh+"=from-file\n")
	loaded, err := LoadDotenv(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != path {
		t.Errorf("loaded = %q, want %q", loaded, path)
	}
	if got := os.Getenv(existing); got != "from-process" {
		t.Errorf("existing variable was overridden: %q", got)
	}
	if got := os.Getenv(fresh); got != "from-file" {
		t.Errorf("fresh variable = %q, want from-file", got)
	}
}

func TestLoadDotenvFirstCandidateWins(t *testing.T) {
	const key = "FLOCK_DOTENV_FIRST"
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	_ = os.Unsetenv(key)

	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.env")
	first := writeTempEnv(t, "first.env", key+"=first\n")
	second := writeTempEnv(t, "second.env", key+"=second\n")

	loaded, err := LoadDotenv(missing, first, second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != first {
		t.Errorf("loaded = %q, want %q (missing candidates are skipped, later ones ignored)", loaded, first)
	}
	if got := os.Getenv(key); got != "first" {
		t.Errorf("%s = %q, want first", key, got)
	}
}

func TestLoadDotenvNoCandidateIsNotAnError(t *testing.T) {
	loaded, err := LoadDotenv(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != "" {
		t.Errorf("loaded = %q, want empty", loaded)
	}
}

func TestLoadDotenvReportsSyntaxErrorsWithPath(t *testing.T) {
	path := writeTempEnv(t, "bad.env", "this is not valid\n")
	_, err := LoadDotenv(path)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error should name the file and line, got %q", err.Error())
	}
}

func TestDotenvCandidatesCoverRootAndServerWorkingDirectories(t *testing.T) {
	t.Parallel()
	if len(DotenvCandidates) != 2 || DotenvCandidates[0] != ".env" || filepath.Base(DotenvCandidates[1]) != ".env" {
		t.Errorf("unexpected candidates %v", DotenvCandidates)
	}
}
