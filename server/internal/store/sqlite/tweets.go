package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

const tweetColumns = "id, author_id, content, parent_tweet_id, created_at, deleted_at"

func (s *Store) CreateTweet(t domain.Tweet) (domain.Tweet, error) {
	_, err := s.db.Exec(
		`INSERT INTO tweets (`+tweetColumns+`) VALUES (?, ?, ?, ?, ?, NULL)`,
		t.ID, t.AuthorID, t.Content, nullableString(t.ParentTweetID), formatTime(t.CreatedAt),
	)
	if err != nil {
		return domain.Tweet{}, fmt.Errorf("creating tweet: %w", err)
	}
	return t, nil
}

// GetTweet excludes soft-deleted tweets (D-16): a deleted tweet reads as not found.
func (s *Store) GetTweet(id string) (domain.Tweet, error) {
	t, err := scanTweet(s.db.QueryRow(`SELECT `+tweetColumns+` FROM tweets WHERE id = ? AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tweet{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Tweet{}, fmt.Errorf("reading tweet: %w", err)
	}
	return t, nil
}

func scanTweet(row rowScanner) (domain.Tweet, error) {
	var (
		t             domain.Tweet
		parentTweetID sql.NullString
		createdAt     string
		deletedAt     sql.NullString
	)
	if err := row.Scan(&t.ID, &t.AuthorID, &t.Content, &parentTweetID, &createdAt, &deletedAt); err != nil {
		return domain.Tweet{}, err
	}
	var err error
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Tweet{}, fmt.Errorf("parsing tweets.created_at: %w", err)
	}
	if parentTweetID.Valid {
		id := parentTweetID.String
		t.ParentTweetID = &id
	}
	if deletedAt.Valid {
		dt, err := parseTime(deletedAt.String)
		if err != nil {
			return domain.Tweet{}, fmt.Errorf("parsing tweets.deleted_at: %w", err)
		}
		t.DeletedAt = &dt
	}
	return t, nil
}

// SoftDeleteTweet sets deleted_at. The WHERE clause's "deleted_at IS NULL" means both "the
// tweet does not exist" and "it is already deleted" affect zero rows, so both map to
// ErrNotFound — deletion is not idempotent past the first call (D-16), matching store/memory.
func (s *Store) SoftDeleteTweet(id string) error {
	res, err := s.db.Exec(`UPDATE tweets SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("deleting tweet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ListTweetsByAuthor returns the author's top-level, non-deleted tweets, newest first.
func (s *Store) ListTweetsByAuthor(authorID string, cursor *store.Cursor, limit int) (store.Page[domain.Tweet], error) {
	return s.listTweets(
		`SELECT `+tweetColumns+` FROM tweets WHERE author_id = ? AND parent_tweet_id IS NULL AND deleted_at IS NULL`,
		[]any{authorID}, cursor, limit, descOrder,
	)
}

// Timeline returns top-level, non-deleted tweets authored by viewerID or by users viewerID
// follows, newest first (D-17).
func (s *Store) Timeline(viewerID string, cursor *store.Cursor, limit int) (store.Page[domain.Tweet], error) {
	return s.listTweets(
		`SELECT `+tweetColumns+` FROM tweets
		 WHERE parent_tweet_id IS NULL AND deleted_at IS NULL
		   AND (author_id = ? OR author_id IN (SELECT followee_id FROM follows WHERE follower_id = ?))`,
		[]any{viewerID, viewerID}, cursor, limit, descOrder,
	)
}

// ListReplies returns parentTweetID's direct, non-deleted replies, oldest first (D-48).
func (s *Store) ListReplies(parentTweetID string, cursor *store.Cursor, limit int) (store.Page[domain.Tweet], error) {
	return s.listTweets(
		`SELECT `+tweetColumns+` FROM tweets WHERE parent_tweet_id = ? AND deleted_at IS NULL`,
		[]any{parentTweetID}, cursor, limit, ascOrder,
	)
}

type sortOrder int

const (
	descOrder sortOrder = iota
	ascOrder
)

// listTweets fetches limit+1 rows past any cursor so trimPage can tell whether a next page
// exists, mirroring store/memory's slicePage semantics (D-19).
func (s *Store) listTweets(baseQuery string, args []any, cursor *store.Cursor, limit int, order sortOrder) (store.Page[domain.Tweet], error) {
	if limit <= 0 || limit > store.MaxLimit {
		limit = store.DefaultLimit
	}
	query := baseQuery
	if cursor != nil {
		op := "<"
		if order == ascOrder {
			op = ">"
		}
		query += ` AND (created_at, id) ` + op + ` (?, ?)`
		args = append(args, formatTime(cursor.CreatedAt), cursor.ID)
	}
	if order == ascOrder {
		query += ` ORDER BY created_at ASC, id ASC LIMIT ?`
	} else {
		query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	}
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return store.Page[domain.Tweet]{}, fmt.Errorf("listing tweets: %w", err)
	}
	defer rows.Close()

	var tweets []domain.Tweet
	for rows.Next() {
		t, err := scanTweet(rows)
		if err != nil {
			return store.Page[domain.Tweet]{}, fmt.Errorf("scanning tweet row: %w", err)
		}
		tweets = append(tweets, t)
	}
	if err := rows.Err(); err != nil {
		return store.Page[domain.Tweet]{}, fmt.Errorf("listing tweets: %w", err)
	}

	page, next := trimPage(tweets, limit, func(t domain.Tweet) (time.Time, string) { return t.CreatedAt, t.ID })
	return store.Page[domain.Tweet]{Items: page, NextCursor: next}, nil
}

// Ancestors walks up from tweetID's parent to the root, root-first, capped at maxHops. Unlike
// every other read method it does not exclude soft-deleted tweets — see the store.Store doc
// comment (D-49 needs the deleted-ancestor placeholder data).
func (s *Store) Ancestors(tweetID string, maxHops int) ([]domain.Tweet, error) {
	current, err := s.getTweetIncludingDeleted(tweetID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var chain []domain.Tweet
	for hops := 0; current.ParentTweetID != nil && hops < maxHops; hops++ {
		parent, err := s.getTweetIncludingDeleted(*current.ParentTweetID)
		if errors.Is(err, store.ErrNotFound) {
			break
		}
		if err != nil {
			return nil, err
		}
		chain = append(chain, parent)
		current = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func (s *Store) getTweetIncludingDeleted(id string) (domain.Tweet, error) {
	t, err := scanTweet(s.db.QueryRow(`SELECT `+tweetColumns+` FROM tweets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tweet{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Tweet{}, fmt.Errorf("reading tweet: %w", err)
	}
	return t, nil
}

// TweetCount is the author's live, top-level tweet count (D-23).
func (s *Store) TweetCount(authorID string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM tweets WHERE author_id = ? AND parent_tweet_id IS NULL AND deleted_at IS NULL`, authorID)
}

// ReplyCount excludes deleted replies (D-49).
func (s *Store) ReplyCount(tweetID string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM tweets WHERE parent_tweet_id = ? AND deleted_at IS NULL`, tweetID)
}
