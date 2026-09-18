package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/social"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// profileResponse is the D-23/D-24 profile shape: public fields plus counts computed on read
// and, from the viewer's perspective, whether they already follow this user. Email is
// intentionally omitted — it is not a public profile field (D-12).
type profileResponse struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	DisplayName    string    `json:"displayName"`
	Bio            string    `json:"bio"`
	CreatedAt      time.Time `json:"createdAt"`
	TweetCount     int       `json:"tweetCount"`
	FollowerCount  int       `json:"followerCount"`
	FollowingCount int       `json:"followingCount"`
	IsFollowedByMe bool      `json:"isFollowedByMe"`
}

func newProfileResponse(p social.Profile) profileResponse {
	return profileResponse{
		ID:             p.User.ID,
		Username:       p.User.Username,
		DisplayName:    p.User.DisplayName,
		Bio:            p.User.Bio,
		CreatedAt:      p.User.CreatedAt,
		TweetCount:     p.TweetCount,
		FollowerCount:  p.FollowerCount,
		FollowingCount: p.FollowingCount,
		IsFollowedByMe: p.IsFollowedByMe,
	}
}

// handleGetProfile is GET /api/users/{username} (D-23).
func handleGetProfile(deps Deps, svc *social.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		profile, err := svc.GetProfile(viewerID, r.PathValue("username"))
		if err != nil {
			writeSocialError(w, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, newProfileResponse(profile))
	}
}

type updateProfileRequest struct {
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
}

// handleUpdateMe is PATCH /api/users/me (D-12).
func handleUpdateMe(deps Deps, svc *social.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := userIDFromContext(r.Context())
		var req updateProfileRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		user, err := svc.UpdateProfile(userID, req.DisplayName, req.Bio)
		if err != nil {
			writeSocialError(w, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

// handleFollow is POST /api/users/{username}/follow (D-21): idempotent, self-follow rejected.
func handleFollow(deps Deps, svc *social.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		profile, err := svc.Follow(viewerID, r.PathValue("username"))
		if err != nil {
			writeSocialError(w, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, newProfileResponse(profile))
	}
}

// handleUnfollow is DELETE /api/users/{username}/follow (D-21): idempotent no-op if not
// currently following.
func handleUnfollow(deps Deps, svc *social.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		profile, err := svc.Unfollow(viewerID, r.PathValue("username"))
		if err != nil {
			writeSocialError(w, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, newProfileResponse(profile))
	}
}

// followListItemResponse is one row of a followers/following list (D-24).
type followListItemResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Bio            string `json:"bio"`
	IsFollowedByMe bool   `json:"isFollowedByMe"`
}

// followListResponse is the D-19 cursor-paginated envelope for followers/following.
type followListResponse struct {
	Items      []followListItemResponse `json:"items"`
	NextCursor *string                  `json:"nextCursor"`
}

func newFollowListResponse(page store.Page[social.FollowListItem]) followListResponse {
	items := make([]followListItemResponse, 0, len(page.Items))
	for _, it := range page.Items {
		items = append(items, followListItemResponse{
			ID:             it.User.ID,
			Username:       it.User.Username,
			DisplayName:    it.User.DisplayName,
			Bio:            it.User.Bio,
			IsFollowedByMe: it.IsFollowedByMe,
		})
	}
	var next *string
	if page.NextCursor != nil {
		s := store.EncodeCursor(*page.NextCursor)
		next = &s
	}
	return followListResponse{Items: items, NextCursor: next}
}

// followListFunc is the shape shared by social.Service's ListFollowers and ListFollowing, so
// one handler factory serves both routes.
type followListFunc func(viewerID, username string, cursor *store.Cursor, limit int) (store.Page[social.FollowListItem], error)

func handleFollowList(deps Deps, list followListFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := userIDFromContext(r.Context())
		cursor, limit, err := parseCursorAndLimit(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error(), nil)
			return
		}
		page, err := list(viewerID, r.PathValue("username"), cursor, limit)
		if err != nil {
			writeSocialError(w, deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, newFollowListResponse(page))
	}
}

// searchResultResponse is one row of a user-search result (D-25).
type searchResultResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// searchUsersResponse is the D-25 search envelope: capped, not paginated.
type searchUsersResponse struct {
	Items []searchResultResponse `json:"items"`
}

// handleSearchUsers is GET /api/search/users?q= (D-25).
func handleSearchUsers(deps Deps, svc *social.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := svc.SearchUsers(r.URL.Query().Get("q"))
		if err != nil {
			writeSocialError(w, deps.Logger, err)
			return
		}
		items := make([]searchResultResponse, 0, len(users))
		for _, u := range users {
			items = append(items, searchResultResponse{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName})
		}
		writeJSON(w, http.StatusOK, searchUsersResponse{Items: items})
	}
}

// writeSocialError maps a social.Service error onto the D-52 envelope.
func writeSocialError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var vErr *social.ValidationError
	switch {
	case errors.As(err, &vErr):
		writeError(w, http.StatusBadRequest, CodeValidation, "Validation failed", vErr.Fields)
	case errors.Is(err, social.ErrSelfFollow):
		writeError(w, http.StatusBadRequest, CodeValidation, "Cannot follow yourself", nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeNotFound, "User not found", nil)
	default:
		if logger != nil {
			logger.Error("social service error", "error", err)
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "Internal server error", nil)
	}
}
