package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

func (s *Store) Follow(f domain.Follow) error {
	if f.FollowerID == f.FolloweeID {
		return store.ErrSelfFollow
	}
	_, err := s.db.Exec(
		`INSERT INTO follows (follower_id, followee_id, created_at) VALUES (?, ?, ?)
		 ON CONFLICT (follower_id, followee_id) DO NOTHING`, // idempotent (D-21)
		f.FollowerID, f.FolloweeID, formatTime(f.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("creating follow: %w", err)
	}
	return nil
}

func (s *Store) Unfollow(followerID, followeeID string) error {
	if _, err := s.db.Exec(`DELETE FROM follows WHERE follower_id = ? AND followee_id = ?`, followerID, followeeID); err != nil {
		return fmt.Errorf("deleting follow: %w", err)
	}
	return nil
}

func (s *Store) IsFollowing(followerID, followeeID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM follows WHERE follower_id = ? AND followee_id = ?`, followerID, followeeID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking follow: %w", err)
	}
	return true, nil
}

func (s *Store) FollowerCount(userID string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM follows WHERE followee_id = ?`, userID)
}

func (s *Store) FollowingCount(userID string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM follows WHERE follower_id = ?`, userID)
}

// ListFollowers and ListFollowing order by the follow edge's created_at descending (D-24); the
// cursor's tie-break id is the *listed* user's id (the follower, or the followee), matching
// store/memory's key functions.

const followUserColumns = "u.id, u.username, u.email, u.display_name, u.password_hash, u.bio, u.created_at, u.updated_at"

func (s *Store) ListFollowers(userID string, cursor *store.Cursor, limit int) (store.Page[domain.User], error) {
	return s.listFollowUsers(
		`SELECT `+followUserColumns+`, f.created_at
		 FROM follows f JOIN users u ON u.id = f.follower_id
		 WHERE f.followee_id = ?`,
		userID, cursor, limit,
	)
}

func (s *Store) ListFollowing(userID string, cursor *store.Cursor, limit int) (store.Page[domain.User], error) {
	return s.listFollowUsers(
		`SELECT `+followUserColumns+`, f.created_at
		 FROM follows f JOIN users u ON u.id = f.followee_id
		 WHERE f.follower_id = ?`,
		userID, cursor, limit,
	)
}

type followedUser struct {
	user       domain.User
	followedAt time.Time
}

func (s *Store) listFollowUsers(baseQuery, userID string, cursor *store.Cursor, limit int) (store.Page[domain.User], error) {
	if limit <= 0 || limit > store.MaxLimit {
		limit = store.DefaultLimit
	}
	query := baseQuery
	args := []any{userID}
	if cursor != nil {
		query += ` AND (f.created_at, u.id) < (?, ?)`
		args = append(args, formatTime(cursor.CreatedAt), cursor.ID)
	}
	query += ` ORDER BY f.created_at DESC, u.id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return store.Page[domain.User]{}, fmt.Errorf("listing follow users: %w", err)
	}
	defer rows.Close()

	var results []followedUser
	for rows.Next() {
		var (
			u                                    domain.User
			userCreatedAt, updatedAt, followedAt string
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Bio, &userCreatedAt, &updatedAt, &followedAt); err != nil {
			return store.Page[domain.User]{}, fmt.Errorf("scanning follow user row: %w", err)
		}
		var err error
		if u.CreatedAt, err = parseTime(userCreatedAt); err != nil {
			return store.Page[domain.User]{}, fmt.Errorf("parsing users.created_at: %w", err)
		}
		if u.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return store.Page[domain.User]{}, fmt.Errorf("parsing users.updated_at: %w", err)
		}
		ca, err := parseTime(followedAt)
		if err != nil {
			return store.Page[domain.User]{}, fmt.Errorf("parsing follows.created_at: %w", err)
		}
		results = append(results, followedUser{user: u, followedAt: ca})
	}
	if err := rows.Err(); err != nil {
		return store.Page[domain.User]{}, fmt.Errorf("listing follow users: %w", err)
	}

	page, next := trimPage(results, limit, func(fu followedUser) (time.Time, string) { return fu.followedAt, fu.user.ID })
	users := make([]domain.User, len(page))
	for i, fu := range page {
		users[i] = fu.user
	}
	return store.Page[domain.User]{Items: users, NextCursor: next}, nil
}
