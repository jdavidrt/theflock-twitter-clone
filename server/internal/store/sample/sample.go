// Package sample loads and validates server/data/sample.json (D-67) and inserts it into any
// store.Store through the same interface the live API uses — the loader is also what
// cmd/seed will call in Step 11 to fill the SQLite store from the identical file.
package sample

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// MaxContentCodePoints mirrors D-13; kept local so the loader has no dependency on a
// not-yet-written internal/validation package.
const MaxContentCodePoints = 280

// Counts summarizes what Load inserted, for the boot log (D-66 "loaded N users...").
type Counts struct {
	Users   int
	Follows int
	Tweets  int
	Likes   int
}

// rawFile mirrors the on-disk JSON shape exactly (D-67); it is intentionally distinct from
// the domain structs because users carry a plain "password" field here, not a hash.
type rawFile struct {
	Users   []rawUser   `json:"users"`
	Follows []rawFollow `json:"follows"`
	Tweets  []rawTweet  `json:"tweets"`
	Likes   []rawLike   `json:"likes"`
}

type rawUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Bio         string `json:"bio"`
	CreatedAt   string `json:"createdAt"`
}

type rawFollow struct {
	FollowerID string `json:"followerId"`
	FolloweeID string `json:"followeeId"`
	CreatedAt  string `json:"createdAt"`
}

type rawTweet struct {
	ID            string  `json:"id"`
	AuthorID      string  `json:"authorId"`
	Content       string  `json:"content"`
	ParentTweetID *string `json:"parentTweetId"`
	CreatedAt     string  `json:"createdAt"`
}

type rawLike struct {
	UserID    string `json:"userId"`
	TweetID   string `json:"tweetId"`
	CreatedAt string `json:"createdAt"`
}

// Load reads path, validates its contents, hashes each distinct password once at bcryptCost
// (D-67), and inserts everything into dest through the Store interface. It returns how much
// was loaded so the caller can log it.
func Load(path string, bcryptCost int, dest store.Store) (Counts, error) {
	raw, err := parseFile(path)
	if err != nil {
		return Counts{}, err
	}
	if err := validate(raw); err != nil {
		return Counts{}, err
	}
	return insert(raw, bcryptCost, dest)
}

func parseFile(path string) (rawFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rawFile{}, fmt.Errorf("reading sample data %q: %w", path, err)
	}
	var raw rawFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return rawFile{}, fmt.Errorf("parsing sample data %q: %w", path, err)
	}
	return raw, nil
}

// validate checks structural and referential integrity (D-67) and collects every problem
// found so a hand edit is fixed in one pass, matching internal/config.Load's style.
func validate(raw rawFile) error {
	var errs []error

	userByID := make(map[string]rawUser, len(raw.Users))
	usernames := make(map[string]bool, len(raw.Users))
	emails := make(map[string]bool, len(raw.Users))
	for i, u := range raw.Users {
		if u.ID == "" {
			errs = append(errs, fmt.Errorf("users[%d]: missing id", i))
			continue
		}
		if _, dup := userByID[u.ID]; dup {
			errs = append(errs, fmt.Errorf("users[%d]: duplicate id %q", i, u.ID))
		}
		if u.Username == "" || u.Username != strings.ToLower(u.Username) {
			errs = append(errs, fmt.Errorf("users[%d] (%s): username must be lowercase and non-empty", i, u.ID))
		}
		if usernames[u.Username] {
			errs = append(errs, fmt.Errorf("users[%d]: duplicate username %q", i, u.Username))
		}
		usernames[u.Username] = true
		if u.Email == "" || u.Email != strings.ToLower(u.Email) {
			errs = append(errs, fmt.Errorf("users[%d] (%s): email must be lowercase and non-empty", i, u.ID))
		}
		if emails[u.Email] {
			errs = append(errs, fmt.Errorf("users[%d]: duplicate email %q", i, u.Email))
		}
		emails[u.Email] = true
		if u.Password == "" {
			errs = append(errs, fmt.Errorf("users[%d] (%s): missing password", i, u.ID))
		}
		if _, err := time.Parse(time.RFC3339, u.CreatedAt); err != nil {
			errs = append(errs, fmt.Errorf("users[%d] (%s): invalid createdAt %q: %w", i, u.ID, u.CreatedAt, err))
		}
		userByID[u.ID] = u
	}

	followPairs := make(map[string]bool, len(raw.Follows))
	for i, f := range raw.Follows {
		if _, ok := userByID[f.FollowerID]; !ok {
			errs = append(errs, fmt.Errorf("follows[%d]: unknown followerId %q", i, f.FollowerID))
		}
		if _, ok := userByID[f.FolloweeID]; !ok {
			errs = append(errs, fmt.Errorf("follows[%d]: unknown followeeId %q", i, f.FolloweeID))
		}
		if f.FollowerID == f.FolloweeID {
			errs = append(errs, fmt.Errorf("follows[%d]: self-follow (%q)", i, f.FollowerID))
		}
		key := f.FollowerID + "|" + f.FolloweeID
		if followPairs[key] {
			errs = append(errs, fmt.Errorf("follows[%d]: duplicate follow %s -> %s", i, f.FollowerID, f.FolloweeID))
		}
		followPairs[key] = true
		if _, err := time.Parse(time.RFC3339, f.CreatedAt); err != nil {
			errs = append(errs, fmt.Errorf("follows[%d]: invalid createdAt %q: %w", i, f.CreatedAt, err))
		}
	}

	tweetByID := make(map[string]rawTweet, len(raw.Tweets))
	for i, t := range raw.Tweets {
		if t.ID == "" {
			errs = append(errs, fmt.Errorf("tweets[%d]: missing id", i))
			continue
		}
		if _, dup := tweetByID[t.ID]; dup {
			errs = append(errs, fmt.Errorf("tweets[%d]: duplicate id %q", i, t.ID))
		}
		tweetByID[t.ID] = t
	}
	for i, t := range raw.Tweets {
		if _, ok := userByID[t.AuthorID]; !ok {
			errs = append(errs, fmt.Errorf("tweets[%d] (%s): unknown authorId %q", i, t.ID, t.AuthorID))
		}
		trimmed := strings.TrimSpace(t.Content)
		if trimmed == "" {
			errs = append(errs, fmt.Errorf("tweets[%d] (%s): content is empty after trimming", i, t.ID))
		}
		if n := utf8.RuneCountInString(trimmed); n > MaxContentCodePoints {
			errs = append(errs, fmt.Errorf("tweets[%d] (%s): content is %d code points, max %d", i, t.ID, n, MaxContentCodePoints))
		}
		if t.ParentTweetID != nil {
			if *t.ParentTweetID == t.ID {
				errs = append(errs, fmt.Errorf("tweets[%d] (%s): tweet cannot be its own parent", i, t.ID))
			} else if _, ok := tweetByID[*t.ParentTweetID]; !ok {
				errs = append(errs, fmt.Errorf("tweets[%d] (%s): unknown parentTweetId %q", i, t.ID, *t.ParentTweetID))
			}
		}
		if _, err := time.Parse(time.RFC3339, t.CreatedAt); err != nil {
			errs = append(errs, fmt.Errorf("tweets[%d] (%s): invalid createdAt %q: %w", i, t.ID, t.CreatedAt, err))
		}
	}

	likePairs := make(map[string]bool, len(raw.Likes))
	for i, l := range raw.Likes {
		if _, ok := userByID[l.UserID]; !ok {
			errs = append(errs, fmt.Errorf("likes[%d]: unknown userId %q", i, l.UserID))
		}
		if _, ok := tweetByID[l.TweetID]; !ok {
			errs = append(errs, fmt.Errorf("likes[%d]: unknown tweetId %q", i, l.TweetID))
		}
		key := l.UserID + "|" + l.TweetID
		if likePairs[key] {
			errs = append(errs, fmt.Errorf("likes[%d]: duplicate like %s -> %s", i, l.UserID, l.TweetID))
		}
		likePairs[key] = true
		if _, err := time.Parse(time.RFC3339, l.CreatedAt); err != nil {
			errs = append(errs, fmt.Errorf("likes[%d]: invalid createdAt %q: %w", i, l.CreatedAt, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid sample data: %w", errors.Join(errs...))
	}
	return nil
}

// insert assumes validate has already passed, so every parse and lookup below is infallible.
func insert(raw rawFile, bcryptCost int, dest store.Store) (Counts, error) {
	hashCache := make(map[string]string, len(raw.Users))
	hashOf := func(password string) (string, error) {
		if h, ok := hashCache[password]; ok {
			return h, nil
		}
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
		if err != nil {
			return "", fmt.Errorf("hashing sample password: %w", err)
		}
		hashCache[password] = string(h)
		return string(h), nil
	}

	for _, u := range raw.Users {
		hash, err := hashOf(u.Password)
		if err != nil {
			return Counts{}, err
		}
		createdAt, _ := time.Parse(time.RFC3339, u.CreatedAt)
		_, err = dest.CreateUser(domain.User{
			ID:           u.ID,
			Username:     u.Username,
			Email:        u.Email,
			DisplayName:  u.DisplayName,
			PasswordHash: hash,
			Bio:          u.Bio,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		})
		if err != nil {
			return Counts{}, fmt.Errorf("loading user %q: %w", u.Username, err)
		}
	}

	for _, t := range orderedByParent(raw.Tweets) {
		createdAt, _ := time.Parse(time.RFC3339, t.CreatedAt)
		_, err := dest.CreateTweet(domain.Tweet{
			ID:            t.ID,
			AuthorID:      t.AuthorID,
			Content:       strings.TrimSpace(t.Content),
			ParentTweetID: t.ParentTweetID,
			CreatedAt:     createdAt,
		})
		if err != nil {
			return Counts{}, fmt.Errorf("loading tweet %q: %w", t.ID, err)
		}
	}

	for _, f := range raw.Follows {
		createdAt, _ := time.Parse(time.RFC3339, f.CreatedAt)
		if err := dest.Follow(domain.Follow{FollowerID: f.FollowerID, FolloweeID: f.FolloweeID, CreatedAt: createdAt}); err != nil {
			return Counts{}, fmt.Errorf("loading follow %s -> %s: %w", f.FollowerID, f.FolloweeID, err)
		}
	}

	for _, l := range raw.Likes {
		createdAt, _ := time.Parse(time.RFC3339, l.CreatedAt)
		if err := dest.Like(domain.Like{UserID: l.UserID, TweetID: l.TweetID, CreatedAt: createdAt}); err != nil {
			return Counts{}, fmt.Errorf("loading like %s -> %s: %w", l.UserID, l.TweetID, err)
		}
	}

	return Counts{
		Users:   len(raw.Users),
		Follows: len(raw.Follows),
		Tweets:  len(raw.Tweets),
		Likes:   len(raw.Likes),
	}, nil
}

// orderedByParent returns tweets with every parent placed before its children, so a store
// that enforces the parentTweetId foreign key (SQLite, Step 11) can insert them in one pass
// regardless of the order they appear in the file. validate has already guaranteed every
// parent reference resolves, so this always terminates.
func orderedByParent(tweets []rawTweet) []rawTweet {
	byID := make(map[string]rawTweet, len(tweets))
	for _, t := range tweets {
		byID[t.ID] = t
	}
	inserted := make(map[string]bool, len(tweets))
	ordered := make([]rawTweet, 0, len(tweets))

	var visit func(t rawTweet)
	visit = func(t rawTweet) {
		if inserted[t.ID] {
			return
		}
		if t.ParentTweetID != nil {
			visit(byID[*t.ParentTweetID])
		}
		inserted[t.ID] = true
		ordered = append(ordered, t)
	}
	for _, t := range tweets {
		visit(t)
	}
	return ordered
}
