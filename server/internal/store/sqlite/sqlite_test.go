package sqlite_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/sqlite"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/storetest"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening sqlite store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("closing sqlite store: %v", err)
		}
	})
	return st
}

func TestSQLiteStoreConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		return newTestStore(t)
	})
}

func TestOpenAppliesSchemaIdempotently(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "reopen.db")

	st1, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	empty, err := st1.IsEmpty()
	if err != nil {
		t.Fatalf("IsEmpty: %v", err)
	}
	if !empty {
		t.Fatalf("a fresh database should report IsEmpty() = true")
	}
	if err := st1.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	// Reopening the same file must not fail on the CREATE TABLE IF NOT EXISTS schema or the
	// schema_version seed row (D-66).
	st2, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("second Open (reapplying schema): %v", err)
	}
	defer st2.Close()
}

func TestTruncateThenIsEmpty(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)

	if _, err := st.CreateUser(domain.User{
		ID: "u1", Username: "alice", Email: "alice@example.com", PasswordHash: "x",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if empty, err := st.IsEmpty(); err != nil || empty {
		t.Fatalf("IsEmpty after insert: got (%v, %v), want (false, nil)", empty, err)
	}

	if err := st.Truncate(); err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	if empty, err := st.IsEmpty(); err != nil || !empty {
		t.Fatalf("IsEmpty after Truncate: got (%v, %v), want (true, nil)", empty, err)
	}
}
