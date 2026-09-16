package storetest

import (
	"errors"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

func testUsers(t *testing.T, newStore NewStoreFunc) {
	t.Run("CreateAndGetByIDUsernameEmail", func(t *testing.T) {
		s := newStore(t)
		u := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))

		byID, err := s.GetUserByID(u.ID)
		requireNoError(t, err)
		if byID.Username != "alice" {
			t.Fatalf("GetUserByID: got username %q, want alice", byID.Username)
		}

		byUsername, err := s.GetUserByUsername("alice")
		requireNoError(t, err)
		if byUsername.ID != u.ID {
			t.Fatalf("GetUserByUsername: got id %q, want %q", byUsername.ID, u.ID)
		}

		byEmail, err := s.GetUserByEmail("alice@example.com")
		requireNoError(t, err)
		if byEmail.ID != u.ID {
			t.Fatalf("GetUserByEmail: got id %q, want %q", byEmail.ID, u.ID)
		}
	})

	t.Run("DuplicateUsernameConflicts", func(t *testing.T) {
		s := newStore(t)
		mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		_, err := s.CreateUser(fixtureUser("alice", "someone-else@example.com"))
		if !errors.Is(err, store.ErrConflict) {
			t.Fatalf("CreateUser with duplicate username: got %v, want ErrConflict", err)
		}
	})

	t.Run("DuplicateEmailConflicts", func(t *testing.T) {
		s := newStore(t)
		mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		_, err := s.CreateUser(fixtureUser("someone-else", "alice@example.com"))
		if !errors.Is(err, store.ErrConflict) {
			t.Fatalf("CreateUser with duplicate email: got %v, want ErrConflict", err)
		}
	})

	t.Run("GetMissingReturnsNotFound", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.GetUserByID("nope"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("GetUserByID(missing): got %v, want ErrNotFound", err)
		}
		if _, err := s.GetUserByUsername("nope"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("GetUserByUsername(missing): got %v, want ErrNotFound", err)
		}
		if _, err := s.GetUserByEmail("nope@example.com"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("GetUserByEmail(missing): got %v, want ErrNotFound", err)
		}
	})

	t.Run("UpdateProfileChangesDisplayNameAndBio", func(t *testing.T) {
		s := newStore(t)
		u := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))

		updated, err := s.UpdateUserProfile(u.ID, "Alice W.", "new bio")
		requireNoError(t, err)
		if updated.DisplayName != "Alice W." || updated.Bio != "new bio" {
			t.Fatalf("UpdateUserProfile: got %+v", updated)
		}
		if updated.Username != u.Username {
			t.Fatalf("UpdateUserProfile changed the immutable username: got %q", updated.Username)
		}

		reread, err := s.GetUserByID(u.ID)
		requireNoError(t, err)
		if reread.DisplayName != "Alice W." || reread.Bio != "new bio" {
			t.Fatalf("update did not persist: got %+v", reread)
		}
	})

	t.Run("UpdateProfileMissingReturnsNotFound", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.UpdateUserProfile("nope", "Name", "bio"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("UpdateUserProfile(missing): got %v, want ErrNotFound", err)
		}
	})

	t.Run("SearchUsersMatchesUsernameOrDisplayNameCaseInsensitively", func(t *testing.T) {
		s := newStore(t)
		mustCreateUser(t, s, fixtureUserFull("alice", "alice@example.com", "Alice Chen", baseTime))
		mustCreateUser(t, s, fixtureUserFull("bob", "bob@example.com", "Bob Alicewood", baseTime))
		mustCreateUser(t, s, fixtureUserFull("carol", "carol@example.com", "Carol", baseTime))

		results, err := s.SearchUsers("ALI", 20)
		requireNoError(t, err)
		got := idsOf(results, userID)
		if len(got) != 2 {
			t.Fatalf("SearchUsers(ALI): got %d results, want 2 (%v)", len(got), got)
		}
		if results[0].Username != "alice" || results[1].Username != "bob" {
			t.Fatalf("SearchUsers: got order %q, %q; want alice, bob (username ASC)", results[0].Username, results[1].Username)
		}
	})

	t.Run("SearchUsersRespectsLimit", func(t *testing.T) {
		s := newStore(t)
		mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		mustCreateUser(t, s, fixtureUser("alicia", "alicia@example.com"))
		mustCreateUser(t, s, fixtureUser("aliyah", "aliyah@example.com"))

		results, err := s.SearchUsers("ali", 2)
		requireNoError(t, err)
		if len(results) != 2 {
			t.Fatalf("SearchUsers with limit 2: got %d results", len(results))
		}
	})
}
