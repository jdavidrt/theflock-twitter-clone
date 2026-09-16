// Package memory implements store.Store with in-process maps guarded by a RWMutex (D-66
// Phase 1). It loads server/data/sample.json at boot via internal/store/sample; writes made
// through the API live only in memory, so restarting the process resets the data to the
// sample — documented in the README as the Phase-1 behavior.
package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// Store is an in-memory store.Store implementation. The zero value is not usable; use New.
type Store struct {
	mu sync.RWMutex

	users     map[string]domain.User // id -> user
	usernames map[string]string      // lowercase username -> id
	emails    map[string]string      // lowercase email -> id

	follows map[string]domain.Follow // "followerID|followeeID" -> edge

	tweets map[string]domain.Tweet // id -> tweet

	likes map[string]domain.Like // "userID|tweetID" -> like
}

var _ store.Store = (*Store)(nil)

// New returns an empty Store, ready to be filled by internal/store/sample.Load.
func New() *Store {
	return &Store{
		users:     make(map[string]domain.User),
		usernames: make(map[string]string),
		emails:    make(map[string]string),
		follows:   make(map[string]domain.Follow),
		tweets:    make(map[string]domain.Tweet),
		likes:     make(map[string]domain.Like),
	}
}

func (s *Store) CreateUser(u domain.User) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usernames[u.Username]; exists {
		return domain.User{}, fmt.Errorf("username %q: %w", u.Username, store.ErrConflict)
	}
	if _, exists := s.emails[u.Email]; exists {
		return domain.User{}, fmt.Errorf("email %q: %w", u.Email, store.ErrConflict)
	}
	s.users[u.ID] = u
	s.usernames[u.Username] = u.ID
	s.emails[u.Email] = u.ID
	return u, nil
}

func (s *Store) GetUserByID(id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	return u, nil
}

func (s *Store) GetUserByUsername(username string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.usernames[username]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	return s.users[id], nil
}

func (s *Store) GetUserByEmail(email string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.emails[email]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	return s.users[id], nil
}

func (s *Store) UpdateUserProfile(userID, displayName, bio string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	u.DisplayName = displayName
	u.Bio = bio
	u.UpdatedAt = time.Now().UTC()
	s.users[userID] = u
	return u, nil
}

func (s *Store) SearchUsers(query string, limit int) ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var matches []domain.User
	for _, u := range s.users {
		if strings.Contains(strings.ToLower(u.Username), q) || strings.Contains(strings.ToLower(u.DisplayName), q) {
			matches = append(matches, u)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Username < matches[j].Username })
	if limit > 0 && limit < len(matches) {
		matches = matches[:limit]
	}
	return matches, nil
}
