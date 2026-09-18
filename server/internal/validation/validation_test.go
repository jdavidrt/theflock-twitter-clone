package validation_test

import (
	"strings"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/validation"
)

func TestCountCodePoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		// A multi-byte emoji is one code point but four UTF-8 bytes and two UTF-16 units
		// (D-13) — this pins that the counter matches neither of those.
		{"multi-byte emoji", "hi\U0001F600!", 4},
		{"accented letters", "café", 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := validation.CountCodePoints(tc.in); got != tc.want {
				t.Errorf("CountCodePoints(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestUsername(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantNorm   string
		wantErrLen int
	}{
		{"valid lowercase", "alice", "alice", 0},
		{"normalizes case and trims", "  Alice_01  ", "alice_01", 0},
		{"too short", "ab", "ab", 1},
		{"too long", strings.Repeat("a", 21), strings.Repeat("a", 21), 1},
		{"minimum length ok", "abc", "abc", 0},
		{"maximum length ok", strings.Repeat("a", 20), strings.Repeat("a", 20), 0},
		{"rejects symbols", "alice-01", "alice-01", 1},
		{"rejects spaces inside", "alice 01", "alice 01", 1},
		{"reserved username", "admin", "admin", 1},
		{"reserved username case-insensitive", "Search", "search", 1},
		{"empty", "", "", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			norm, errs := validation.Username(tc.in)
			if norm != tc.wantNorm {
				t.Errorf("Username(%q) normalized = %q, want %q", tc.in, norm, tc.wantNorm)
			}
			if len(errs) != tc.wantErrLen {
				t.Errorf("Username(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}

func TestUsernameAllReservedNamesRejected(t *testing.T) {
	t.Parallel()
	reserved := []string{
		"login", "register", "logout", "search", "tweet", "tweets",
		"settings", "api", "me", "home", "admin", "explore",
		"notifications", "messages",
	}
	for _, name := range reserved {
		if _, errs := validation.Username(name); len(errs) == 0 {
			t.Errorf("Username(%q) should be rejected as reserved", name)
		}
	}
}

func TestEmail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantNorm   string
		wantErrLen int
	}{
		{"valid", "Alice@Example.com", "alice@example.com", 0},
		{"trims whitespace", "  bob@example.com  ", "bob@example.com", 0},
		{"missing at sign", "aliceexample.com", "aliceexample.com", 1},
		{"missing domain dot", "alice@example", "alice@example", 1},
		{"empty", "", "", 1},
		{"contains space", "alice bob@example.com", "alice bob@example.com", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			norm, errs := validation.Email(tc.in)
			if norm != tc.wantNorm {
				t.Errorf("Email(%q) normalized = %q, want %q", tc.in, norm, tc.wantNorm)
			}
			if len(errs) != tc.wantErrLen {
				t.Errorf("Email(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}

func TestPassword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantErrLen int
	}{
		{"valid", "Password123!", 0},
		{"minimum length ok", strings.Repeat("a", 8), 0},
		{"maximum length ok", strings.Repeat("a", 72), 0},
		{"too short", "short1", 1},
		{"too long", strings.Repeat("a", 73), 1},
		{"empty", "", 1},
		{"multi-byte characters count as code points, not bytes", strings.Repeat("\U0001F600", 8), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if errs := validation.Password(tc.in); len(errs) != tc.wantErrLen {
				t.Errorf("Password(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}

func TestBio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantNorm   string
		wantErrLen int
	}{
		{"valid", "Backend engineer. Coffee before code.", "Backend engineer. Coffee before code.", 0},
		{"empty is valid", "", "", 0},
		{"trims whitespace", "  hello  ", "hello", 0},
		{"whitespace-only trims to empty and is valid", "   ", "", 0},
		{"maximum length ok", strings.Repeat("a", 160), strings.Repeat("a", 160), 0},
		{"too long", strings.Repeat("a", 161), strings.Repeat("a", 161), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			norm, errs := validation.Bio(tc.in)
			if norm != tc.wantNorm {
				t.Errorf("Bio(%q) normalized = %q, want %q", tc.in, norm, tc.wantNorm)
			}
			if len(errs) != tc.wantErrLen {
				t.Errorf("Bio(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}

func TestTweetContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantNorm   string
		wantErrLen int
	}{
		{"valid", "hello world", "hello world", 0},
		{"trims outer whitespace", "  hello  ", "hello", 0},
		{"preserves internal newlines", "line one\nline two", "line one\nline two", 0},
		{"empty rejected", "", "", 1},
		{"whitespace-only rejected", "   ", "", 1},
		{"maximum length ok", strings.Repeat("a", 280), strings.Repeat("a", 280), 0},
		{"too long", strings.Repeat("a", 281), strings.Repeat("a", 281), 1},
		// A multi-byte emoji is one code point but four UTF-8 bytes (D-13) — 280 emoji must
		// stay valid and 281 must not, matching the client's `[...content].length` count.
		{"280 multi-byte emoji ok", strings.Repeat("\U0001F600", 280), strings.Repeat("\U0001F600", 280), 0},
		{"281 multi-byte emoji rejected", strings.Repeat("\U0001F600", 281), strings.Repeat("\U0001F600", 281), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			norm, errs := validation.TweetContent(tc.in)
			if norm != tc.wantNorm {
				t.Errorf("TweetContent(%q) normalized = %q, want %q", tc.in, norm, tc.wantNorm)
			}
			if len(errs) != tc.wantErrLen {
				t.Errorf("TweetContent(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}

func TestSearchQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantNorm   string
		wantErrLen int
	}{
		{"valid", "alice", "alice", 0},
		{"trims whitespace", "  alice  ", "alice", 0},
		{"single character ok", "a", "a", 0},
		{"empty rejected", "", "", 1},
		{"whitespace-only rejected", "   ", "", 1},
		{"maximum length ok", strings.Repeat("a", 50), strings.Repeat("a", 50), 0},
		{"too long", strings.Repeat("a", 51), strings.Repeat("a", 51), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			norm, errs := validation.SearchQuery(tc.in)
			if norm != tc.wantNorm {
				t.Errorf("SearchQuery(%q) normalized = %q, want %q", tc.in, norm, tc.wantNorm)
			}
			if len(errs) != tc.wantErrLen {
				t.Errorf("SearchQuery(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		in         string
		wantNorm   string
		wantErrLen int
	}{
		{"valid", "Alice Doe", "Alice Doe", 0},
		{"trims whitespace", "  Alice  ", "Alice", 0},
		{"empty reported as too short", "", "", 1},
		{"whitespace-only reported as too short", "   ", "", 1},
		{"too long", strings.Repeat("a", 51), strings.Repeat("a", 51), 1},
		{"maximum length ok", strings.Repeat("a", 50), strings.Repeat("a", 50), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			norm, errs := validation.DisplayName(tc.in)
			if norm != tc.wantNorm {
				t.Errorf("DisplayName(%q) normalized = %q, want %q", tc.in, norm, tc.wantNorm)
			}
			if len(errs) != tc.wantErrLen {
				t.Errorf("DisplayName(%q) errs = %v, want %d error(s)", tc.in, errs, tc.wantErrLen)
			}
		})
	}
}
