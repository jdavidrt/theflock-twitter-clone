package social_test

import (
	"errors"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/social"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
)

func newStoreWithUsers(t *testing.T, usernames ...string) (store.Store, map[string]domain.User) {
	t.Helper()
	st := memory.New()
	users := make(map[string]domain.User, len(usernames))
	for _, name := range usernames {
		u, err := st.CreateUser(domain.User{
			ID:           name + "-id",
			Username:     name,
			Email:        name + "@example.com",
			DisplayName:  name,
			PasswordHash: "hashed",
		})
		if err != nil {
			t.Fatalf("CreateUser(%s): %v", name, err)
		}
		users[name] = u
	}
	return st, users
}

func TestGetProfileComputesCountsAndFollowState(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob")
	if err := st.Follow(domain.Follow{FollowerID: users["bob"].ID, FolloweeID: users["alice"].ID}); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	svc := social.New(st)
	profile, err := svc.GetProfile(users["bob"].ID, "alice")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if profile.FollowerCount != 1 {
		t.Errorf("FollowerCount = %d, want 1", profile.FollowerCount)
	}
	if !profile.IsFollowedByMe {
		t.Error("IsFollowedByMe = false, want true (bob follows alice)")
	}

	reverse, err := svc.GetProfile(users["alice"].ID, "bob")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if reverse.IsFollowedByMe {
		t.Error("IsFollowedByMe = true, want false (alice does not follow bob)")
	}
}

func TestGetProfileUnknownUsernameReturnsNotFound(t *testing.T) {
	t.Parallel()
	st, _ := newStoreWithUsers(t, "alice")
	svc := social.New(st)
	if _, err := svc.GetProfile("alice-id", "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetProfile() error = %v, want ErrNotFound", err)
	}
}

func TestGetProfileIsCaseInsensitiveOnUsername(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := social.New(st)
	if _, err := svc.GetProfile(users["alice"].ID, "  Alice  "); err != nil {
		t.Errorf("GetProfile(Alice) error = %v, want it to resolve case-insensitively", err)
	}
}

func TestUpdateProfileValidatesAndApplies(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := social.New(st)

	updated, err := svc.UpdateProfile(users["alice"].ID, "  Alice Doe  ", "  new bio  ")
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if updated.DisplayName != "Alice Doe" {
		t.Errorf("DisplayName = %q, want trimmed Alice Doe", updated.DisplayName)
	}
	if updated.Bio != "new bio" {
		t.Errorf("Bio = %q, want trimmed new bio", updated.Bio)
	}
}

func TestUpdateProfileRejectsInvalidFields(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := social.New(st)

	_, err := svc.UpdateProfile(users["alice"].ID, "", "x")
	var vErr *social.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("UpdateProfile() error = %v, want *ValidationError", err)
	}
	if len(vErr.Fields["displayName"]) == 0 {
		t.Error("Fields[\"displayName\"] should report the empty-name error")
	}

	tooLongBio := make([]byte, 161)
	for i := range tooLongBio {
		tooLongBio[i] = 'a'
	}
	_, err = svc.UpdateProfile(users["alice"].ID, "Alice", string(tooLongBio))
	if !errors.As(err, &vErr) {
		t.Fatalf("UpdateProfile() error = %v, want *ValidationError", err)
	}
	if len(vErr.Fields["bio"]) == 0 {
		t.Error("Fields[\"bio\"] should report the too-long error")
	}
}

func TestFollowIsIdempotentAndUpdatesCounts(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob")
	svc := social.New(st)

	profile, err := svc.Follow(users["bob"].ID, "alice")
	if err != nil {
		t.Fatalf("Follow() error = %v", err)
	}
	if profile.FollowerCount != 1 || !profile.IsFollowedByMe {
		t.Fatalf("Follow() profile = %+v, want FollowerCount=1 IsFollowedByMe=true", profile)
	}

	profile, err = svc.Follow(users["bob"].ID, "alice") // repeat: idempotent (D-21)
	if err != nil {
		t.Fatalf("second Follow() error = %v", err)
	}
	if profile.FollowerCount != 1 {
		t.Errorf("FollowerCount after repeat follow = %d, want still 1", profile.FollowerCount)
	}
}

func TestFollowSelfIsRejected(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := social.New(st)

	if _, err := svc.Follow(users["alice"].ID, "alice"); !errors.Is(err, social.ErrSelfFollow) {
		t.Errorf("Follow(self) error = %v, want ErrSelfFollow", err)
	}
}

func TestFollowUnknownUsernameReturnsNotFound(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := social.New(st)

	if _, err := svc.Follow(users["alice"].ID, "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Follow(ghost) error = %v, want ErrNotFound", err)
	}
}

func TestUnfollowWhenNotFollowingIsNoOp(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob")
	svc := social.New(st)

	profile, err := svc.Unfollow(users["bob"].ID, "alice")
	if err != nil {
		t.Fatalf("Unfollow() error = %v, want no error for a no-op unfollow", err)
	}
	if profile.IsFollowedByMe {
		t.Error("IsFollowedByMe = true after a no-op unfollow, want false")
	}
}

func TestUnfollowRemovesAnExistingEdge(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob")
	svc := social.New(st)

	if _, err := svc.Follow(users["bob"].ID, "alice"); err != nil {
		t.Fatalf("Follow() error = %v", err)
	}
	profile, err := svc.Unfollow(users["bob"].ID, "alice")
	if err != nil {
		t.Fatalf("Unfollow() error = %v", err)
	}
	if profile.IsFollowedByMe || profile.FollowerCount != 0 {
		t.Errorf("profile after Unfollow = %+v, want IsFollowedByMe=false FollowerCount=0", profile)
	}
}

func TestListFollowersReportsViewerFollowState(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob", "carol")
	// bob and carol both follow alice; carol also follows bob.
	if err := st.Follow(domain.Follow{FollowerID: users["bob"].ID, FolloweeID: users["alice"].ID}); err != nil {
		t.Fatal(err)
	}
	if err := st.Follow(domain.Follow{FollowerID: users["carol"].ID, FolloweeID: users["alice"].ID}); err != nil {
		t.Fatal(err)
	}
	if err := st.Follow(domain.Follow{FollowerID: users["carol"].ID, FolloweeID: users["bob"].ID}); err != nil {
		t.Fatal(err)
	}

	svc := social.New(st)
	page, err := svc.ListFollowers(users["carol"].ID, "alice", nil, 20)
	if err != nil {
		t.Fatalf("ListFollowers() error = %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("ListFollowers() items = %d, want 2", len(page.Items))
	}
	byUsername := map[string]social.FollowListItem{}
	for _, item := range page.Items {
		byUsername[item.User.Username] = item
	}
	if !byUsername["bob"].IsFollowedByMe {
		t.Error("bob entry: IsFollowedByMe = false, want true (carol follows bob)")
	}
	if byUsername["carol"].IsFollowedByMe {
		t.Error("carol entry: IsFollowedByMe = true, want false (carol does not follow herself)")
	}
}

func TestListFollowingUnknownUsernameReturnsNotFound(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := social.New(st)
	if _, err := svc.ListFollowing(users["alice"].ID, "ghost", nil, 20); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ListFollowing(ghost) error = %v, want ErrNotFound", err)
	}
}

func TestSearchUsersMatchesUsernameOrDisplayName(t *testing.T) {
	t.Parallel()
	st, _ := newStoreWithUsers(t, "alice", "bob")
	svc := social.New(st)

	results, err := svc.SearchUsers("ali")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if len(results) != 1 || results[0].Username != "alice" {
		t.Fatalf("SearchUsers(ali) = %+v, want just alice", results)
	}
}

func TestSearchUsersRejectsEmptyQuery(t *testing.T) {
	t.Parallel()
	st, _ := newStoreWithUsers(t, "alice")
	svc := social.New(st)

	if _, err := svc.SearchUsers("   "); err == nil {
		t.Fatal("SearchUsers(whitespace) error = nil, want ValidationError")
	} else {
		var vErr *social.ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("SearchUsers(whitespace) error = %v, want *ValidationError", err)
		}
	}
}
