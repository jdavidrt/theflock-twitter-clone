package memory

import (
	"sort"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// keyFunc extracts the (createdAt, id) pair a list is ordered and paginated by (D-19).
type keyFunc[T any] func(T) (time.Time, string)

func sortDescBy[T any](rows []T, key keyFunc[T]) {
	sort.Slice(rows, func(i, j int) bool {
		ai, aid := key(rows[i])
		bi, bid := key(rows[j])
		if !ai.Equal(bi) {
			return ai.After(bi)
		}
		return aid > bid
	})
}

func sortAscBy[T any](rows []T, key keyFunc[T]) {
	sort.Slice(rows, func(i, j int) bool {
		ai, aid := key(rows[i])
		bi, bid := key(rows[j])
		if !ai.Equal(bi) {
			return ai.Before(bi)
		}
		return aid < bid
	})
}

// paginateDesc slices a "createdAt DESC, id DESC"-sorted slice to the page after cursor.
// rows must already be sorted in that order (D-19).
func paginateDesc[T any](rows []T, cursor *store.Cursor, limit int, key keyFunc[T]) ([]T, *store.Cursor) {
	start := 0
	if cursor != nil {
		for i, r := range rows {
			ca, id := key(r)
			if (store.Cursor{CreatedAt: ca, ID: id}).Before(*cursor) {
				start = i
				break
			}
			start = i + 1
		}
	}
	return slicePage(rows, start, limit, key)
}

// paginateAsc is the ascending-order counterpart used by replies (D-48).
func paginateAsc[T any](rows []T, cursor *store.Cursor, limit int, key keyFunc[T]) ([]T, *store.Cursor) {
	start := 0
	if cursor != nil {
		for i, r := range rows {
			ca, id := key(r)
			if (store.Cursor{CreatedAt: ca, ID: id}).After(*cursor) {
				start = i
				break
			}
			start = i + 1
		}
	}
	return slicePage(rows, start, limit, key)
}

func slicePage[T any](rows []T, start, limit int, key keyFunc[T]) ([]T, *store.Cursor) {
	if limit <= 0 || limit > store.MaxLimit {
		limit = store.DefaultLimit
	}
	if start > len(rows) {
		start = len(rows)
	}
	end := start + limit
	if end > len(rows) {
		end = len(rows)
	}
	page := rows[start:end]

	var next *store.Cursor
	if end < len(rows) {
		ca, id := key(page[len(page)-1])
		c := store.Cursor{CreatedAt: ca, ID: id}
		next = &c
	}
	return page, next
}
