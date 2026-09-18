package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/tweet"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// tweetAuthorResponse is the author summary embedded in every tweet response.
type tweetAuthorResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// tweetResponse is the D-23 tweet shape: the row plus its author and counts computed on read.
type tweetResponse struct {
	ID            string              `json:"id"`
	Author        tweetAuthorResponse `json:"author"`
	Content       string              `json:"content"`
	ParentTweetID *string             `json:"parentTweetId"`
	CreatedAt     time.Time           `json:"createdAt"`
	LikeCount     int                 `json:"likeCount"`
	ReplyCount    int                 `json:"replyCount"`
	LikedByMe     bool                `json:"likedByMe"`
}

func newTweetResponse(v tweet.View) tweetResponse {
	return tweetResponse{
		ID: v.Tweet.ID,
		Author: tweetAuthorResponse{
			ID:          v.Author.ID,
			Username:    v.Author.Username,
			DisplayName: v.Author.DisplayName,
		},
		Content:       v.Tweet.Content,
		ParentTweetID: v.Tweet.ParentTweetID,
		CreatedAt:     v.Tweet.CreatedAt,
		LikeCount:     v.LikeCount,
		ReplyCount:    v.ReplyCount,
		LikedByMe:     v.LikedByMe,
	}
}

// tweetListResponse is the D-19 cursor-paginated envelope for tweet lists.
type tweetListResponse struct {
	Items      []tweetResponse `json:"items"`
	NextCursor *string         `json:"nextCursor"`
}

func newTweetListResponse(page store.Page[tweet.View]) tweetListResponse {
	items := make([]tweetResponse, 0, len(page.Items))
	for _, v := range page.Items {
		items = append(items, newTweetResponse(v))
	}
	var next *string
	if page.NextCursor != nil {
		s := store.EncodeCursor(*page.NextCursor)
		next = &s
	}
	return tweetListResponse{Items: items, NextCursor: next}
}

type createTweetRequest struct {
	Content string `json:"content"`
}

// handleCreateTweet is POST /api/tweets (D-13/D-14).
func handleCreateTweet(deps Deps, svc *tweet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := userIDFromContext(r.Context())
		var req createTweetRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		v, err := svc.Create(userID, req.Content)
		if err != nil {
			writeTweetError(w, deps.Logger, err, "Tweet not found")
			return
		}
		writeJSON(w, http.StatusCreated, newTweetResponse(v))
	}
}

// handleGetTweet is GET /api/tweets/{id} (404 if missing or deleted, D-16).
func handleGetTweet(deps Deps, svc *tweet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		v, err := svc.Get(viewerID, r.PathValue("id"))
		if err != nil {
			writeTweetError(w, deps.Logger, err, "Tweet not found")
			return
		}
		writeJSON(w, http.StatusOK, newTweetResponse(v))
	}
}

// handleDeleteTweet is DELETE /api/tweets/{id}: author-only, soft delete (D-16).
func handleDeleteTweet(deps Deps, svc *tweet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		if err := svc.Delete(viewerID, r.PathValue("id")); err != nil {
			writeTweetError(w, deps.Logger, err, "Tweet not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleListUserTweets is GET /api/users/{username}/tweets: top-level, not deleted, paginated.
func handleListUserTweets(deps Deps, svc *tweet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		cursor, limit, err := parseCursorAndLimit(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error(), nil)
			return
		}
		page, err := svc.ListByUsername(viewerID, r.PathValue("username"), cursor, limit)
		if err != nil {
			writeTweetError(w, deps.Logger, err, "User not found")
			return
		}
		writeJSON(w, http.StatusOK, newTweetListResponse(page))
	}
}

// handleTimeline is GET /api/timeline (D-17/D-19).
func handleTimeline(deps Deps, svc *tweet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		cursor, limit, err := parseCursorAndLimit(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error(), nil)
			return
		}
		page, err := svc.Timeline(viewerID, cursor, limit)
		if err != nil {
			writeTweetError(w, deps.Logger, err, "Tweet not found")
			return
		}
		writeJSON(w, http.StatusOK, newTweetListResponse(page))
	}
}

// likeFunc is the shape shared by tweet.Service's Like and Unlike, so one handler factory
// serves both routes.
type likeFunc func(viewerID, tweetID string) (tweet.View, error)

// handleLikeAction backs POST/DELETE /api/tweets/{id}/like (D-22).
func handleLikeAction(deps Deps, action likeFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		v, err := action(viewerID, r.PathValue("id"))
		if err != nil {
			writeTweetError(w, deps.Logger, err, "Tweet not found")
			return
		}
		writeJSON(w, http.StatusOK, newTweetResponse(v))
	}
}

// writeTweetError maps a tweet.Service error onto the D-52 envelope. notFoundMessage lets each
// call site say whether a store.ErrNotFound means the tweet or the referenced user was missing.
func writeTweetError(w http.ResponseWriter, logger *slog.Logger, err error, notFoundMessage string) {
	var vErr *tweet.ValidationError
	switch {
	case errors.As(err, &vErr):
		writeError(w, http.StatusBadRequest, CodeValidation, "Validation failed", vErr.Fields)
	case errors.Is(err, tweet.ErrForbidden):
		writeError(w, http.StatusForbidden, CodeForbidden, "Only the author can delete this tweet", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeNotFound, notFoundMessage, nil)
	default:
		if logger != nil {
			logger.Error("tweet service error", "error", err)
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "Internal server error", nil)
	}
}
