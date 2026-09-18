package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
)

// testBcryptCost mirrors config.Config.BcryptCost() under APP_ENV=test (D-06): low, so tests
// hashing real passwords stay fast.
const testBcryptCost = 4

func newService() *auth.Service {
	return auth.New(memory.New(), testBcryptCost, "test-secret-at-least-32-characters-long")
}

func TestRegisterSuccess(t *testing.T) {
	t.Parallel()
	svc := newService()
	u, err := svc.Register(auth.RegisterInput{
		Email:    "Alice@Example.com",
		Username: "  Alice_01  ",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.ID == "" {
		t.Error("Register() did not assign an id")
	}
	if u.Email != "alice@example.com" {
		t.Errorf("Email = %q, want normalized alice@example.com", u.Email)
	}
	if u.Username != "alice_01" {
		t.Errorf("Username = %q, want normalized alice_01", u.Username)
	}
	if u.DisplayName != "alice_01" {
		t.Errorf("DisplayName = %q, want default to username when omitted (D-01)", u.DisplayName)
	}
	if u.PasswordHash == "" || u.PasswordHash == "Password123!" {
		t.Error("PasswordHash must be set and must not be the plaintext password")
	}
}

func TestRegisterExplicitDisplayName(t *testing.T) {
	t.Parallel()
	svc := newService()
	u, err := svc.Register(auth.RegisterInput{
		Email: "bob@example.com", Username: "bob", Password: "Password123!", DisplayName: "  Bob Ross  ",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if u.DisplayName != "Bob Ross" {
		t.Errorf("DisplayName = %q, want trimmed Bob Ross", u.DisplayName)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	t.Parallel()
	svc := newService()
	_, err := svc.Register(auth.RegisterInput{Email: "not-an-email", Username: "ab", Password: "short"})

	var vErr *auth.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("Register() error = %v, want *auth.ValidationError", err)
	}
	for _, field := range []string{"email", "username", "password"} {
		if len(vErr.Fields[field]) == 0 {
			t.Errorf("Fields[%q] is empty, want at least one error", field)
		}
	}
}

func TestRegisterReservedUsername(t *testing.T) {
	t.Parallel()
	svc := newService()
	_, err := svc.Register(auth.RegisterInput{Email: "a@example.com", Username: "admin", Password: "Password123!"})

	var vErr *auth.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("Register() error = %v, want *auth.ValidationError", err)
	}
	if len(vErr.Fields["username"]) == 0 {
		t.Error("Fields[\"username\"] should report the reserved-name error")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	t.Parallel()
	svc := newService()
	in := auth.RegisterInput{Email: "dup@example.com", Username: "first", Password: "Password123!"}
	if _, err := svc.Register(in); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	in.Username = "second"
	_, err := svc.Register(in)
	var cErr *auth.ConflictError
	if !errors.As(err, &cErr) {
		t.Fatalf("second Register() error = %v, want *auth.ConflictError", err)
	}
	if cErr.Field != "email" {
		t.Errorf("ConflictError.Field = %q, want email", cErr.Field)
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	t.Parallel()
	svc := newService()
	in := auth.RegisterInput{Email: "one@example.com", Username: "dupname", Password: "Password123!"}
	if _, err := svc.Register(in); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	in.Email = "two@example.com"
	_, err := svc.Register(in)
	var cErr *auth.ConflictError
	if !errors.As(err, &cErr) {
		t.Fatalf("second Register() error = %v, want *auth.ConflictError", err)
	}
	if cErr.Field != "username" {
		t.Errorf("ConflictError.Field = %q, want username", cErr.Field)
	}
}

func TestLoginSuccess(t *testing.T) {
	t.Parallel()
	svc := newService()
	if _, err := svc.Register(auth.RegisterInput{Email: "carol@example.com", Username: "carol", Password: "Password123!"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	u, err := svc.Login("Carol@Example.com", "Password123!")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if u.Username != "carol" {
		t.Errorf("Login() returned %q, want carol", u.Username)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	t.Parallel()
	svc := newService()
	if _, err := svc.Register(auth.RegisterInput{Email: "dave@example.com", Username: "dave", Password: "Password123!"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := svc.Login("dave@example.com", "WrongPassword1"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	t.Parallel()
	svc := newService()
	if _, err := svc.Login("nobody@example.com", "Password123!"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials (no enumeration, D-08)", err)
	}
}

func TestValidationErrorMessageMentionsFields(t *testing.T) {
	t.Parallel()
	err := &auth.ValidationError{Fields: auth.FieldErrors{"email": {"is required"}}}
	if err.Error() == "" {
		t.Error("ValidationError.Error() must not be empty")
	}
}

func TestConflictErrorMessageNamesTheField(t *testing.T) {
	t.Parallel()
	err := &auth.ConflictError{Field: "username"}
	if got := err.Error(); !strings.Contains(got, "username") {
		t.Errorf("ConflictError.Error() = %q, want it to name the field", got)
	}
}
