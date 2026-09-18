// Package validation holds pure, per-field validation functions (D-54). Handlers and services
// never write validation rules of their own — they call into this package and turn the results
// into the D-52 `details` object. Constants here are the source of truth the client's
// client/src/lib/validation.ts mirrors.
package validation

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Field length bounds (D-01…D-06).
const (
	UsernameMinLength     = 3
	UsernameMaxLength     = 20
	DisplayNameMinLength  = 1
	DisplayNameMaxLength  = 50
	BioMaxLength          = 160
	PasswordMinLength     = 8
	PasswordMaxLength     = 72
	TweetContentMinLength = 1
	TweetContentMaxLength = 280
	SearchQueryMinLength  = 1
	SearchQueryMaxLength  = 50
)

var usernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// emailRegexp is intentionally simple (D-02: "RFC-ish"), not a full RFC 5322 implementation.
var emailRegexp = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// reservedUsernames cannot be registered because they collide with client routes (D-04).
var reservedUsernames = map[string]struct{}{
	"login": {}, "register": {}, "logout": {}, "search": {}, "tweet": {}, "tweets": {},
	"settings": {}, "api": {}, "me": {}, "home": {}, "admin": {}, "explore": {},
	"notifications": {}, "messages": {},
}

// CountCodePoints counts s in Unicode code points, matching the client's `[...content].length`
// (D-13). A Go rune is a Unicode code point, so multi-byte characters (emoji, accented letters)
// count once each, not once per UTF-8 byte or UTF-16 unit.
func CountCodePoints(s string) int {
	return utf8.RuneCountInString(s)
}

// NormalizeUsername trims and lowercases raw the same way Username does, without validating it.
func NormalizeUsername(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// Username validates and normalizes a username (D-03/D-04): 3-20 chars, letters/digits/
// underscore only, stored lowercase, not on the reserved list. It always returns the
// normalized form, even when errs is non-empty, so callers can echo it back in error UIs.
func Username(raw string) (normalized string, errs []string) {
	normalized = NormalizeUsername(raw)
	if n := CountCodePoints(normalized); n < UsernameMinLength || n > UsernameMaxLength {
		errs = append(errs, fmt.Sprintf("must be between %d and %d characters", UsernameMinLength, UsernameMaxLength))
	}
	if normalized != "" && !usernameRegexp.MatchString(normalized) {
		errs = append(errs, "must contain only letters, numbers and underscores")
	}
	if _, reserved := reservedUsernames[normalized]; reserved {
		errs = append(errs, "is reserved and cannot be used")
	}
	return normalized, errs
}

// NormalizeEmail trims and lowercases raw the same way Email does, without validating it.
func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// Email validates and normalizes an email address (D-02): trimmed, lowercased, RFC-ish format.
func Email(raw string) (normalized string, errs []string) {
	normalized = NormalizeEmail(raw)
	if normalized == "" || !emailRegexp.MatchString(normalized) {
		errs = append(errs, "must be a valid email address")
	}
	return normalized, errs
}

// Password validates a password (D-05): 8-72 code points, no composition rules, no trimming
// (leading/trailing whitespace is significant in a password). 72 is bcrypt's effective input
// limit.
func Password(raw string) (errs []string) {
	if n := CountCodePoints(raw); n < PasswordMinLength || n > PasswordMaxLength {
		errs = append(errs, fmt.Sprintf("must be between %d and %d characters", PasswordMinLength, PasswordMaxLength))
	}
	return errs
}

// DisplayName validates and normalizes a display name (D-12): 1-50 code points, trimmed.
// Callers handle the "omitted, defaults to username" case (D-01) themselves — an empty raw
// value here is reported as too short, not treated as valid.
func DisplayName(raw string) (normalized string, errs []string) {
	normalized = strings.TrimSpace(raw)
	if n := CountCodePoints(normalized); n < DisplayNameMinLength || n > DisplayNameMaxLength {
		errs = append(errs, fmt.Sprintf("must be between %d and %d characters", DisplayNameMinLength, DisplayNameMaxLength))
	}
	return normalized, errs
}

// Bio validates and normalizes a profile bio (D-12): 0-160 code points, trimmed. An empty bio
// is valid — it is an optional field.
func Bio(raw string) (normalized string, errs []string) {
	normalized = strings.TrimSpace(raw)
	if n := CountCodePoints(normalized); n > BioMaxLength {
		errs = append(errs, fmt.Sprintf("must be at most %d characters", BioMaxLength))
	}
	return normalized, errs
}

// TweetContent validates and normalizes tweet content (D-13/D-14): 1-280 code points after
// trimming leading/trailing whitespace. Internal newlines are preserved — only the outer
// whitespace is trimmed.
func TweetContent(raw string) (normalized string, errs []string) {
	normalized = strings.TrimSpace(raw)
	if n := CountCodePoints(normalized); n < TweetContentMinLength || n > TweetContentMaxLength {
		errs = append(errs, fmt.Sprintf("must be between %d and %d characters", TweetContentMinLength, TweetContentMaxLength))
	}
	return normalized, errs
}

// SearchQuery validates and normalizes a user-search query (D-25): 1-50 code points, trimmed.
// An empty (or whitespace-only) query is rejected rather than treated as "match everything".
func SearchQuery(raw string) (normalized string, errs []string) {
	normalized = strings.TrimSpace(raw)
	if n := CountCodePoints(normalized); n < SearchQueryMinLength || n > SearchQueryMaxLength {
		errs = append(errs, fmt.Sprintf("must be between %d and %d characters", SearchQueryMinLength, SearchQueryMaxLength))
	}
	return normalized, errs
}
