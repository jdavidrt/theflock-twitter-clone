package memory

import (
	"sort"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
)

func likeKey(userID, tweetID string) string { return userID + "|" + tweetID }

func (s *Store) Like(l domain.Like) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := likeKey(l.UserID, l.TweetID)
	if _, exists := s.likes[key]; exists {
		return nil // idempotent (D-22)
	}
	s.likes[key] = l
	return nil
}

func (s *Store) Unlike(userID, tweetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.likes, likeKey(userID, tweetID))
	return nil
}

func (s *Store) LikeCount(tweetID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, l := range s.likes {
		if l.TweetID == tweetID {
			n++
		}
	}
	return n, nil
}

func (s *Store) LikedByMe(userID, tweetID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.likes[likeKey(userID, tweetID)]
	return ok, nil
}

func (s *Store) LikedBy(tweetID string) ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users []domain.User
	for _, l := range s.likes {
		if l.TweetID == tweetID {
			if u, ok := s.users[l.UserID]; ok {
				users = append(users, u)
			}
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users, nil
}
