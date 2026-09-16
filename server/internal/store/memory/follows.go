package memory

import (
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

func followKey(followerID, followeeID string) string { return followerID + "|" + followeeID }

func (s *Store) Follow(f domain.Follow) error {
	if f.FollowerID == f.FolloweeID {
		return store.ErrSelfFollow
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := followKey(f.FollowerID, f.FolloweeID)
	if _, exists := s.follows[key]; exists {
		return nil // idempotent (D-21)
	}
	s.follows[key] = f
	return nil
}

func (s *Store) Unfollow(followerID, followeeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.follows, followKey(followerID, followeeID))
	return nil
}

func (s *Store) IsFollowing(followerID, followeeID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.follows[followKey(followerID, followeeID)]
	return ok, nil
}

func (s *Store) FollowerCount(userID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, f := range s.follows {
		if f.FolloweeID == userID {
			n++
		}
	}
	return n, nil
}

func (s *Store) FollowingCount(userID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, f := range s.follows {
		if f.FollowerID == userID {
			n++
		}
	}
	return n, nil
}

func (s *Store) ListFollowers(userID string, cursor *store.Cursor, limit int) (store.Page[domain.User], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []domain.Follow
	for _, f := range s.follows {
		if f.FolloweeID == userID {
			rows = append(rows, f)
		}
	}
	key := func(f domain.Follow) (time.Time, string) { return f.CreatedAt, f.FollowerID }
	sortDescBy(rows, key)
	page, next := paginateDesc(rows, cursor, limit, key)

	users := make([]domain.User, 0, len(page))
	for _, f := range page {
		users = append(users, s.users[f.FollowerID])
	}
	return store.Page[domain.User]{Items: users, NextCursor: next}, nil
}

func (s *Store) ListFollowing(userID string, cursor *store.Cursor, limit int) (store.Page[domain.User], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []domain.Follow
	for _, f := range s.follows {
		if f.FollowerID == userID {
			rows = append(rows, f)
		}
	}
	key := func(f domain.Follow) (time.Time, string) { return f.CreatedAt, f.FolloweeID }
	sortDescBy(rows, key)
	page, next := paginateDesc(rows, cursor, limit, key)

	users := make([]domain.User, 0, len(page))
	for _, f := range page {
		users = append(users, s.users[f.FolloweeID])
	}
	return store.Page[domain.User]{Items: users, NextCursor: next}, nil
}
