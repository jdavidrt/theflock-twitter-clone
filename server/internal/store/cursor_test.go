package store

import (
	"errors"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	want := Cursor{CreatedAt: time.Date(2026, 9, 16, 12, 34, 56, 789000000, time.UTC), ID: "tweet-123"}
	got, err := DecodeCursor(EncodeCursor(want))
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	tests := []string{
		"not-base64url!!!",
		"aGVsbG8", // valid base64url, but no "|" separator
		"",
	}
	for _, s := range tests {
		if _, err := DecodeCursor(s); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("DecodeCursor(%q): got %v, want ErrInvalidCursor", s, err)
		}
	}
}

func TestCursorBeforeOrdersDescending(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	later := Cursor{CreatedAt: t1, ID: "a"}
	earlier := Cursor{CreatedAt: t0, ID: "a"}
	if !earlier.Before(later) {
		t.Error("an earlier timestamp should sort Before a later one")
	}
	if later.Before(earlier) {
		t.Error("a later timestamp should not sort Before an earlier one")
	}

	// Same timestamp: smaller id continues a "createdAt DESC, id DESC" page (D-19).
	smallerID := Cursor{CreatedAt: t0, ID: "a"}
	largerID := Cursor{CreatedAt: t0, ID: "b"}
	if !smallerID.Before(largerID) {
		t.Error("at equal timestamps, the smaller id should sort Before the larger one")
	}
}

func TestCursorAfterOrdersAscending(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	later := Cursor{CreatedAt: t1, ID: "a"}
	earlier := Cursor{CreatedAt: t0, ID: "a"}
	if !later.After(earlier) {
		t.Error("a later timestamp should sort After an earlier one")
	}

	smallerID := Cursor{CreatedAt: t0, ID: "a"}
	largerID := Cursor{CreatedAt: t0, ID: "b"}
	if !largerID.After(smallerID) {
		t.Error("at equal timestamps, the larger id should sort After the smaller one")
	}
}
