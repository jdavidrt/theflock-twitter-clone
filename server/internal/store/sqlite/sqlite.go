// Package sqlite implements store.Store on database/sql and the pure-Go driver
// modernc.org/sqlite (D-65/D-66 Phase 2 — no CGO, no C toolchain required). schema.sql is
// applied at boot with CREATE TABLE IF NOT EXISTS; there is one schema and no migration
// history to manage.
package sqlite

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	sqlitedriver "modernc.org/sqlite"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

//go:embed schema.sql
var schemaSQL string

// schemaVersion is bumped whenever schema.sql changes in a way that matters to evaluators of
// the schema_version table. There is no migration runner (D-66): a bump is a manual step.
const schemaVersion = 1

// SQLite extended result codes for a PRIMARY KEY / UNIQUE violation
// (https://www.sqlite.org/rescode.html#constraint_primarykey), surfaced by modernc.org/sqlite
// through (*sqlitedriver.Error).Code().
const (
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintUnique     = 2067
)

// Store is a store.Store implementation backed by a SQLite file. The zero value is not usable;
// use Open.
type Store struct {
	db *sql.DB
}

var _ store.Store = (*Store)(nil)

// Open opens (creating if necessary) the SQLite file at path and applies schema.sql. The
// connection pool is capped at one connection: D-64 notes "one writer at a time is fine at
// this scale," and a single connection sidesteps SQLITE_BUSY errors under concurrent requests
// without a retry loop — simpler and just as correct for this app's load.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to sqlite %s: %w", path, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying schema to %s: %w", path, err)
	}
	if err := ensureSchemaVersion(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func ensureSchemaVersion(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&count); err != nil {
		return fmt.Errorf("reading schema_version: %w", err)
	}
	if count == 0 {
		if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, schemaVersion); err != nil {
			return fmt.Errorf("seeding schema_version: %w", err)
		}
	}
	return nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

// IsEmpty reports whether the users table has no rows, so cmd/api can decide whether to seed a
// fresh database on boot (D-66).
func (s *Store) IsEmpty() (bool, error) {
	n, err := s.count(`SELECT COUNT(*) FROM users`, nil)
	return n == 0, err
}

// Truncate deletes every row from every data table, in FK-safe order, for cmd/seed's
// truncate-then-insert semantics (D-39). schema_version is left alone.
func (s *Store) Truncate() error {
	for _, table := range []string{"likes", "follows", "tweets", "users"} {
		if _, err := s.db.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("truncating %s: %w", table, err)
		}
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// count runs a single-value COUNT(*) query. arg is nil for a query with no parameters.
func (s *Store) count(query string, arg any) (int, error) {
	var (
		row *sql.Row
		n   int
	)
	if arg == nil {
		row = s.db.QueryRow(query)
	} else {
		row = s.db.QueryRow(query, arg)
	}
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("counting: %w", err)
	}
	return n, nil
}

// constraintCode extracts SQLite's extended result code from a constraint-violation error, if
// err is one.
func constraintCode(err error) (int, bool) {
	var serr *sqlitedriver.Error
	if errors.As(err, &serr) {
		return serr.Code(), true
	}
	return 0, false
}

// timeLayout is a fixed-width RFC 3339 UTC layout (always a 9-digit zero-padded fraction) so
// that ordering the TEXT column lexicographically ("created_at ASC/DESC" — D-19) always matches
// chronological order, with no ambiguity from RFC3339Nano's variable-width trimmed fractions.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(timeLayout, s) }

// nullableString adapts a possibly-nil *string for a database/sql argument: a nil pointer binds
// SQL NULL, matching tweets.parent_tweet_id.
func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// trimPage drops the lookahead row (if any) a paginated query fetched to detect a next page —
// callers ask for limit+1 rows — and computes the continuation cursor from it, mirroring
// store/memory's slicePage semantics (D-19).
func trimPage[T any](items []T, limit int, key func(T) (time.Time, string)) ([]T, *store.Cursor) {
	if len(items) <= limit {
		return items, nil
	}
	page := items[:limit]
	ca, id := key(page[limit-1])
	c := store.Cursor{CreatedAt: ca, ID: id}
	return page, &c
}
