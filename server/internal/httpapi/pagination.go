package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// parseCursorAndLimit reads the `cursor`/`limit` query parameters shared by every
// cursor-paginated endpoint (D-19): limit is 1-50, defaulting to store.DefaultLimit; cursor is
// the opaque base64url string from a previous page's nextCursor. An invalid value is reported
// to the caller so the handler can answer 400 instead of silently substituting a default.
func parseCursorAndLimit(r *http.Request) (*store.Cursor, int, error) {
	limit := store.DefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > store.MaxLimit {
			return nil, 0, fmt.Errorf("limit must be an integer between 1 and %d", store.MaxLimit)
		}
		limit = n
	}

	var cursor *store.Cursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, err := store.DecodeCursor(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor: %w", err)
		}
		cursor = &c
	}

	return cursor, limit, nil
}
