package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

const userColumns = "id, username, email, display_name, password_hash, bio, created_at, updated_at"

// CreateUser checks username and email uniqueness explicitly before inserting, rather than
// parsing which column a UNIQUE-constraint error names: the exact error text is a SQLite
// implementation detail, while service/auth.conflictField string-matches "email" in the wrapped
// message to build the D-52 per-field error — a fragile thing to reconstruct from driver text
// when it's cheap to just ask first, matching store/memory's field-specific ErrConflict wrapping.
func (s *Store) CreateUser(u domain.User) (domain.User, error) {
	if taken, err := s.exists(`SELECT 1 FROM users WHERE username = ?`, u.Username); err != nil {
		return domain.User{}, err
	} else if taken {
		return domain.User{}, fmt.Errorf("username %q: %w", u.Username, store.ErrConflict)
	}
	if taken, err := s.exists(`SELECT 1 FROM users WHERE email = ?`, u.Email); err != nil {
		return domain.User{}, err
	} else if taken {
		return domain.User{}, fmt.Errorf("email %q: %w", u.Email, store.ErrConflict)
	}

	_, err := s.db.Exec(
		`INSERT INTO users (`+userColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Email, u.DisplayName, u.PasswordHash, u.Bio, formatTime(u.CreatedAt), formatTime(u.UpdatedAt),
	)
	if err != nil {
		if code, ok := constraintCode(err); ok && (code == sqliteConstraintUnique || code == sqliteConstraintPrimaryKey) {
			// The pre-checks above should have already caught this; reachable only if two
			// requests race between check and insert. MaxOpenConns(1) serializes writes so this
			// is effectively unreachable today, but it's a safe generic fallback either way.
			return domain.User{}, fmt.Errorf("user %q: %w", u.Username, store.ErrConflict)
		}
		return domain.User{}, fmt.Errorf("creating user: %w", err)
	}
	return u, nil
}

func (s *Store) exists(query, arg string) (bool, error) {
	var found int
	err := s.db.QueryRow(query, arg).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking existence: %w", err)
	}
	return true, nil
}

func (s *Store) GetUserByID(id string) (domain.User, error) {
	return s.getUser(`SELECT `+userColumns+` FROM users WHERE id = ?`, id)
}

func (s *Store) GetUserByUsername(username string) (domain.User, error) {
	return s.getUser(`SELECT `+userColumns+` FROM users WHERE username = ?`, username)
}

func (s *Store) GetUserByEmail(email string) (domain.User, error) {
	return s.getUser(`SELECT `+userColumns+` FROM users WHERE email = ?`, email)
}

func (s *Store) getUser(query, arg string) (domain.User, error) {
	u, err := scanUser(s.db.QueryRow(query, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, store.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("reading user: %w", err)
	}
	return u, nil
}

func scanUser(row rowScanner) (domain.User, error) {
	var (
		u                    domain.User
		createdAt, updatedAt string
	)
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Bio, &createdAt, &updatedAt); err != nil {
		return domain.User{}, err
	}
	var err error
	if u.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.User{}, fmt.Errorf("parsing users.created_at: %w", err)
	}
	if u.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.User{}, fmt.Errorf("parsing users.updated_at: %w", err)
	}
	return u, nil
}

func (s *Store) UpdateUserProfile(userID, displayName, bio string) (domain.User, error) {
	res, err := s.db.Exec(
		`UPDATE users SET display_name = ?, bio = ?, updated_at = ? WHERE id = ?`,
		displayName, bio, formatTime(time.Now().UTC()), userID,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf("updating user profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.User{}, store.ErrNotFound
	}
	return s.GetUserByID(userID)
}

// SearchUsers matches query as a case-insensitive substring of username or displayName (D-25).
// instr(lower(...), lower(?)) works for non-ASCII text too, unlike a bare LIKE.
func (s *Store) SearchUsers(query string, limit int) ([]domain.User, error) {
	if limit <= 0 || limit > store.MaxLimit {
		limit = store.DefaultLimit
	}
	q := strings.ToLower(query)
	rows, err := s.db.Query(
		`SELECT `+userColumns+` FROM users
		 WHERE instr(lower(username), ?) > 0 OR instr(lower(display_name), ?) > 0
		 ORDER BY username ASC
		 LIMIT ?`,
		q, q, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searching users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning user search row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
