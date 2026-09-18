package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
)

func (s *Store) Like(l domain.Like) error {
	_, err := s.db.Exec(
		`INSERT INTO likes (user_id, tweet_id, created_at) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, tweet_id) DO NOTHING`, // idempotent (D-22)
		l.UserID, l.TweetID, formatTime(l.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("creating like: %w", err)
	}
	return nil
}

func (s *Store) Unlike(userID, tweetID string) error {
	if _, err := s.db.Exec(`DELETE FROM likes WHERE user_id = ? AND tweet_id = ?`, userID, tweetID); err != nil {
		return fmt.Errorf("deleting like: %w", err)
	}
	return nil
}

func (s *Store) LikeCount(tweetID string) (int, error) {
	return s.count(`SELECT COUNT(*) FROM likes WHERE tweet_id = ?`, tweetID)
}

func (s *Store) LikedByMe(userID, tweetID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM likes WHERE user_id = ? AND tweet_id = ?`, userID, tweetID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking like: %w", err)
	}
	return true, nil
}

func (s *Store) LikedBy(tweetID string) ([]domain.User, error) {
	rows, err := s.db.Query(
		`SELECT u.id, u.username, u.email, u.display_name, u.password_hash, u.bio, u.created_at, u.updated_at
		 FROM likes l JOIN users u ON u.id = l.user_id
		 WHERE l.tweet_id = ?
		 ORDER BY u.username ASC`,
		tweetID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing likers: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning liker row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
