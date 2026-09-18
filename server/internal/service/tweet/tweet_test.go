package tweet_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/tweet"
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

func TestCreateValidatesContent(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)

	if _, err := svc.Create(users["alice"].ID, ""); err == nil {
		t.Fatal("Create(empty content) error = nil, want ValidationError")
	} else {
		var vErr *tweet.ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("Create(empty content) error = %v, want *ValidationError", err)
		}
	}

	if _, err := svc.Create(users["alice"].ID, strings.Repeat("a", 281)); err == nil {
		t.Fatal("Create(281 chars) error = nil, want ValidationError")
	}
}

func TestCreateTrimsAndReturnsView(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)

	v, err := svc.Create(users["alice"].ID, "  hello world  ")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if v.Tweet.Content != "hello world" {
		t.Errorf("Content = %q, want trimmed", v.Tweet.Content)
	}
	if v.Author.Username != "alice" {
		t.Errorf("Author = %+v, want alice", v.Author)
	}
	if v.LikeCount != 0 || v.ReplyCount != 0 || v.LikedByMe {
		t.Errorf("fresh tweet counts = %+v, want all zero/false", v)
	}
}

func TestGetExcludesDeletedTweet(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)
	v, err := svc.Create(users["alice"].ID, "hello")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := svc.Delete(users["alice"].ID, v.Tweet.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := svc.Get(users["alice"].ID, v.Tweet.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get(deleted tweet) error = %v, want ErrNotFound", err)
	}
}

func TestDeleteByNonAuthorReturnsForbidden(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob")
	svc := tweet.New(st)
	v, err := svc.Create(users["alice"].ID, "hello")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := svc.Delete(users["bob"].ID, v.Tweet.ID); !errors.Is(err, tweet.ErrForbidden) {
		t.Errorf("Delete(non-author) error = %v, want ErrForbidden", err)
	}
}

func TestDeleteMissingTweetReturnsNotFound(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)
	if err := svc.Delete(users["alice"].ID, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete(missing) error = %v, want ErrNotFound", err)
	}
}

func TestTimelineExcludesNonFollowedAndIncludesOwnTweets(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice", "bob", "carol")
	svc := tweet.New(st)

	if _, err := svc.Create(users["alice"].ID, "alice tweet"); err != nil {
		t.Fatalf("Create(alice) error = %v", err)
	}
	if _, err := svc.Create(users["bob"].ID, "bob tweet"); err != nil {
		t.Fatalf("Create(bob) error = %v", err)
	}
	if _, err := svc.Create(users["carol"].ID, "carol tweet"); err != nil {
		t.Fatalf("Create(carol) error = %v", err)
	}
	if err := st.Follow(domain.Follow{FollowerID: users["alice"].ID, FolloweeID: users["bob"].ID}); err != nil {
		t.Fatalf("Follow: %v", err)
	}

	page, err := svc.Timeline(users["alice"].ID, nil, 20)
	if err != nil {
		t.Fatalf("Timeline() error = %v", err)
	}
	var contents []string
	for _, v := range page.Items {
		contents = append(contents, v.Tweet.Content)
	}
	for _, want := range []string{"alice tweet", "bob tweet"} {
		found := false
		for _, c := range contents {
			if c == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Timeline() = %v, want it to include %q", contents, want)
		}
	}
	for _, c := range contents {
		if c == "carol tweet" {
			t.Errorf("Timeline() = %v, must not include carol's tweet (not followed)", contents)
		}
	}
}

func TestLikeIsIdempotentAndUnlikeRemoves(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)
	v, err := svc.Create(users["alice"].ID, "hello")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := svc.Like(users["alice"].ID, v.Tweet.ID); err != nil {
		t.Fatalf("Like() error = %v", err)
	}
	liked, err := svc.Like(users["alice"].ID, v.Tweet.ID)
	if err != nil {
		t.Fatalf("Like() (second time) error = %v", err)
	}
	if liked.LikeCount != 1 {
		t.Errorf("LikeCount after liking twice = %d, want 1", liked.LikeCount)
	}
	if !liked.LikedByMe {
		t.Error("LikedByMe = false, want true")
	}

	unliked, err := svc.Unlike(users["alice"].ID, v.Tweet.ID)
	if err != nil {
		t.Fatalf("Unlike() error = %v", err)
	}
	if unliked.LikeCount != 0 || unliked.LikedByMe {
		t.Errorf("after Unlike() = %+v, want count 0 and LikedByMe false", unliked)
	}
}

func TestLikeMissingTweetReturnsNotFound(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)
	if _, err := svc.Like(users["alice"].ID, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Like(missing) error = %v, want ErrNotFound", err)
	}
}

func TestListByUsernameUnknownReturnsNotFound(t *testing.T) {
	t.Parallel()
	st, users := newStoreWithUsers(t, "alice")
	svc := tweet.New(st)
	if _, err := svc.ListByUsername(users["alice"].ID, "ghost", nil, 20); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ListByUsername(unknown) error = %v, want ErrNotFound", err)
	}
}
