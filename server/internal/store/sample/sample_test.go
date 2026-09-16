package sample

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
)

// realSampleDataPath finds server/data/sample.json regardless of the working directory
// go test uses (it runs from this package's own directory).
const realSampleDataPath = "../../../data/sample.json"

func TestLoadRealSampleFile(t *testing.T) {
	st := memory.New()
	counts, err := Load(realSampleDataPath, bcrypt.MinCost, st)
	if err != nil {
		t.Fatalf("Load(%s): %v", realSampleDataPath, err)
	}

	if counts.Users < 10 {
		t.Errorf("Users = %d, want at least 10 (D-67 seed requirement)", counts.Users)
	}
	if counts.Tweets == 0 || counts.Follows == 0 || counts.Likes == 0 {
		t.Errorf("expected tweets, follows and likes to be non-zero, got %+v", counts)
	}

	alice, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername(alice): %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(alice.PasswordHash), []byte("Password123!")); err != nil {
		t.Errorf("alice's stored hash does not match the documented sample password: %v", err)
	}

	following, err := st.FollowingCount(alice.ID)
	if err != nil {
		t.Fatalf("FollowingCount(alice): %v", err)
	}
	if following < 6 {
		t.Errorf("alice follows %d users, want at least 6 (D-67)", following)
	}

	page, err := st.Timeline(alice.ID, nil, store.MaxLimit)
	if err != nil {
		t.Fatalf("Timeline(alice): %v", err)
	}
	if len(page.Items) == 0 {
		t.Error("alice's timeline is empty; the sample data should give her a rich timeline")
	}
}

func TestLoadHashesEachDistinctPasswordOnce(t *testing.T) {
	st := memory.New()
	if _, err := Load(realSampleDataPath, bcrypt.MinCost, st); err != nil {
		t.Fatalf("Load: %v", err)
	}

	alice, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername(alice): %v", err)
	}
	bob, err := st.GetUserByUsername("bob")
	if err != nil {
		t.Fatalf("GetUserByUsername(bob): %v", err)
	}
	// Every sample user shares the same plaintext password (D-67); the loader's hash cache
	// means they end up with the literal same hash string, not just two hashes that both
	// verify — that is the specific behavior "hashing each distinct password once" promises.
	if alice.PasswordHash != bob.PasswordHash {
		t.Errorf("expected alice and bob to share a cached hash for the same password, got distinct hashes")
	}
}

func TestLoadUsesTheGivenBcryptCost(t *testing.T) {
	st := memory.New()
	const cost = bcrypt.MinCost + 1
	if _, err := Load(realSampleDataPath, cost, st); err != nil {
		t.Fatalf("Load: %v", err)
	}
	alice, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername(alice): %v", err)
	}
	got, err := bcrypt.Cost([]byte(alice.PasswordHash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if got != cost {
		t.Errorf("bcrypt cost = %d, want %d", got, cost)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"), bcrypt.MinCost, memory.New())
	if err == nil {
		t.Fatal("Load(missing file): got nil error")
	}
}

func TestValidateCatchesHandEditMistakes(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name: "self-follow",
			json: `{
				"users": [{"id":"u1","username":"alice","email":"alice@example.com","displayName":"Alice","password":"Password123!","bio":"","createdAt":"2026-09-01T00:00:00Z"}],
				"follows": [{"followerId":"u1","followeeId":"u1","createdAt":"2026-09-01T00:00:00Z"}],
				"tweets": [], "likes": []
			}`,
			wantErr: "self-follow",
		},
		{
			name: "duplicate username",
			json: `{
				"users": [
					{"id":"u1","username":"alice","email":"a1@example.com","displayName":"Alice","password":"Password123!","bio":"","createdAt":"2026-09-01T00:00:00Z"},
					{"id":"u2","username":"alice","email":"a2@example.com","displayName":"Alice2","password":"Password123!","bio":"","createdAt":"2026-09-01T00:00:00Z"}
				],
				"follows": [], "tweets": [], "likes": []
			}`,
			wantErr: "duplicate username",
		},
		{
			name: "content over 280 code points",
			json: `{
				"users": [{"id":"u1","username":"alice","email":"alice@example.com","displayName":"Alice","password":"Password123!","bio":"","createdAt":"2026-09-01T00:00:00Z"}],
				"follows": [],
				"tweets": [{"id":"t1","authorId":"u1","content":"` + strings.Repeat("a", 281) + `","parentTweetId":null,"createdAt":"2026-09-01T00:00:00Z"}],
				"likes": []
			}`,
			wantErr: "281 code points",
		},
		{
			name: "dangling parent reference",
			json: `{
				"users": [{"id":"u1","username":"alice","email":"alice@example.com","displayName":"Alice","password":"Password123!","bio":"","createdAt":"2026-09-01T00:00:00Z"}],
				"follows": [],
				"tweets": [{"id":"t1","authorId":"u1","content":"hello","parentTweetId":"does-not-exist","createdAt":"2026-09-01T00:00:00Z"}],
				"likes": []
			}`,
			wantErr: "unknown parentTweetId",
		},
		{
			name: "duplicate like pair",
			json: `{
				"users": [{"id":"u1","username":"alice","email":"alice@example.com","displayName":"Alice","password":"Password123!","bio":"","createdAt":"2026-09-01T00:00:00Z"}],
				"follows": [],
				"tweets": [{"id":"t1","authorId":"u1","content":"hello","parentTweetId":null,"createdAt":"2026-09-01T00:00:00Z"}],
				"likes": [
					{"userId":"u1","tweetId":"t1","createdAt":"2026-09-01T00:00:00Z"},
					{"userId":"u1","tweetId":"t1","createdAt":"2026-09-01T00:01:00Z"}
				]
			}`,
			wantErr: "duplicate like",
		},
		{
			name: "invalid timestamp",
			json: `{
				"users": [{"id":"u1","username":"alice","email":"alice@example.com","displayName":"Alice","password":"Password123!","bio":"","createdAt":"not-a-date"}],
				"follows": [], "tweets": [], "likes": []
			}`,
			wantErr: "invalid createdAt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.json")
			if err := os.WriteFile(path, []byte(tt.json), 0o644); err != nil {
				t.Fatalf("writing fixture: %v", err)
			}
			_, err := Load(path, bcrypt.MinCost, memory.New())
			if err == nil {
				t.Fatalf("Load(%s): got nil error, want one mentioning %q", tt.name, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load(%s) error = %q, want it to mention %q", tt.name, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadPropagatesStoreErrors(t *testing.T) {
	// A store that already has "alice" registered makes CreateUser conflict, which Load
	// must surface rather than swallow.
	st := memory.New()
	if _, err := Load(realSampleDataPath, bcrypt.MinCost, st); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	_, err := Load(realSampleDataPath, bcrypt.MinCost, st)
	if err == nil {
		t.Fatal("second Load into the same store: got nil error, want a conflict")
	}
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("second Load error = %v, want it to wrap store.ErrConflict", err)
	}
}
