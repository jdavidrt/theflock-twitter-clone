// Package domain holds the API's entity structs (D-51, D-53). Field order and JSON tags are
// explicit camelCase so wire format never depends on Go's field order or name.
package domain

import "time"

// User is an account (D-12). PasswordHash is never marshalled to JSON.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"-"`
	Bio          string    `json:"bio"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Follow is an edge in the follow graph, keyed by (FollowerID, FolloweeID).
type Follow struct {
	FollowerID string    `json:"followerId"`
	FolloweeID string    `json:"followeeId"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Tweet is a top-level tweet when ParentTweetID is nil, or a reply otherwise (D-47).
// DeletedAt marks a soft delete (D-16); nil means the tweet is live.
type Tweet struct {
	ID            string     `json:"id"`
	AuthorID      string     `json:"authorId"`
	Content       string     `json:"content"`
	ParentTweetID *string    `json:"parentTweetId"`
	CreatedAt     time.Time  `json:"createdAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

// IsReply reports whether the tweet is a reply rather than a top-level tweet.
func (t Tweet) IsReply() bool { return t.ParentTweetID != nil }

// IsDeleted reports whether the tweet has been soft-deleted.
func (t Tweet) IsDeleted() bool { return t.DeletedAt != nil }

// Like is an edge keyed by (UserID, TweetID).
type Like struct {
	UserID    string    `json:"userId"`
	TweetID   string    `json:"tweetId"`
	CreatedAt time.Time `json:"createdAt"`
}
