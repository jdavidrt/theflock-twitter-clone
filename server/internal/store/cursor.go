package store

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Cursor identifies a position in a createdAt+id ordered list (D-19).
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// ErrInvalidCursor is returned by DecodeCursor for a malformed or corrupt cursor string.
var ErrInvalidCursor = errors.New("invalid cursor")

// EncodeCursor base64url-encodes "<createdAt RFC3339Nano>|<id>" (D-19).
func EncodeCursor(c Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor reverses EncodeCursor, rejecting anything that does not round-trip.
func DecodeCursor(s string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return Cursor{}, fmt.Errorf("%w: malformed payload", ErrInvalidCursor)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}
	return Cursor{CreatedAt: createdAt, ID: id}, nil
}

// Before reports whether c sorts strictly after other in "createdAt DESC, id DESC" order —
// i.e. whether c is a valid continuation row for a page whose cursor is other (D-19).
func (c Cursor) Before(other Cursor) bool {
	if !c.CreatedAt.Equal(other.CreatedAt) {
		return c.CreatedAt.Before(other.CreatedAt)
	}
	return c.ID < other.ID
}

// After reports whether c sorts strictly after other in "createdAt ASC, id ASC" order —
// the continuation direction used by ascending lists such as replies (D-48).
func (c Cursor) After(other Cursor) bool {
	if !c.CreatedAt.Equal(other.CreatedAt) {
		return c.CreatedAt.After(other.CreatedAt)
	}
	return c.ID > other.ID
}
