package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DotenvCandidates are the places the bootstrap looks for a .env file, in order: the working
// directory (running from the repo root) and its parent (running from server/, as `npm run dev` does).
var DotenvCandidates = []string{".env", filepath.Join("..", ".env")}

// LoadDotenv reads the first existing candidate file and sets each KEY=VALUE pair that is not
// already present in the process environment (existing variables win — D-43). It returns the
// path that was loaded, or "" when no candidate exists; a missing file is not an error.
func LoadDotenv(candidates ...string) (string, error) {
	for _, path := range candidates {
		found, err := loadDotenvFile(path)
		if err != nil {
			return "", err
		}
		if found {
			return path, nil
		}
	}
	return "", nil
}

func loadDotenvFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	vars, err := ParseDotenv(f)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	for k, v := range vars {
		if _, set := os.LookupEnv(k); set {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return false, fmt.Errorf("set %s: %w", k, err)
		}
	}
	return true, nil
}

// ParseDotenv parses the subset of dotenv syntax this project uses: blank lines, `#` comment
// lines, `KEY=VALUE`, an optional `export ` prefix, values wrapped in single or double quotes,
// and ` # trailing comments` after unquoted values. Later keys override earlier ones. Anything
// else is a syntax error naming the line number.
func ParseDotenv(r io.Reader) (map[string]string, error) {
	vars := map[string]string{}
	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", n)
		}
		vars[key] = cleanValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

// cleanValue strips one pair of matching surrounding quotes, or, for unquoted values, drops a
// trailing ` # comment`.
func cleanValue(v string) string {
	if len(v) >= 2 {
		if q := v[0]; (q == '"' || q == '\'') && v[len(v)-1] == q {
			return v[1 : len(v)-1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}
