// Package auth implements registration, login and JWT session issuance (D-53). It is the only
// package that knows how to hash/compare passwords or sign/verify session tokens; httpapi
// handlers call into it and never touch bcrypt or the JWT library directly.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/validation"
)

// ErrInvalidCredentials is returned by Login for any failure reason (D-08): a nonexistent
// email and a wrong password are indistinguishable to the caller, so there is nothing to
// enumerate.
var ErrInvalidCredentials = errors.New("invalid email or password")

// FieldErrors maps a request field name to the problems found with it, ready to drop into the
// D-52 error envelope's `details`.
type FieldErrors map[string][]string

// ValidationError is returned by Register when the input fails field validation.
type ValidationError struct {
	Fields FieldErrors
}

func (e *ValidationError) Error() string { return "validation failed" }

// ConflictError is returned by Register when the email or username is already taken (D-08).
type ConflictError struct {
	Field string // "email" or "username"
}

func (e *ConflictError) Error() string { return fmt.Sprintf("%s is already taken", e.Field) }

// Service implements registration, login and JWT issuance/verification against a Store.
type Service struct {
	store      store.Store
	bcryptCost int
	jwtSecret  []byte
}

// New builds a Service. bcryptCost is config.Config.BcryptCost() (D-06); jwtSecret is
// config.Config.JWTSecret (D-09).
func New(st store.Store, bcryptCost int, jwtSecret string) *Service {
	return &Service{store: st, bcryptCost: bcryptCost, jwtSecret: []byte(jwtSecret)}
}

// RegisterInput is the raw (unvalidated, unnormalized) registration payload.
type RegisterInput struct {
	Email       string
	Username    string
	Password    string
	DisplayName string
}

// Register validates input (D-01…D-06), defaults an omitted DisplayName to the username,
// hashes the password and creates the user. It returns *ValidationError for bad input and
// *ConflictError when the email or username is already taken.
func (s *Service) Register(in RegisterInput) (domain.User, error) {
	fields := FieldErrors{}

	email, errs := validation.Email(in.Email)
	if len(errs) > 0 {
		fields["email"] = errs
	}
	username, errs := validation.Username(in.Username)
	if len(errs) > 0 {
		fields["username"] = errs
	}
	if errs := validation.Password(in.Password); len(errs) > 0 {
		fields["password"] = errs
	}

	// DisplayName is optional and defaults to the username (D-01); only a non-empty value is
	// run through the length check, so omitting it is never itself a validation error.
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = username
	} else if _, dnErrs := validation.DisplayName(displayName); len(dnErrs) > 0 {
		fields["displayName"] = dnErrs
	}

	if len(fields) > 0 {
		return domain.User{}, &ValidationError{Fields: fields}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.bcryptCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now().UTC()
	created, err := s.store.CreateUser(domain.User{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return domain.User{}, &ConflictError{Field: conflictField(err)}
		}
		return domain.User{}, err
	}
	return created, nil
}

// conflictField recovers which field a store.ErrConflict was about from the wrapping error's
// message (store/memory.CreateUser formats it as `field "value": <ErrConflict>`).
func conflictField(err error) string {
	if strings.Contains(err.Error(), "email") {
		return "email"
	}
	return "username"
}

// Login verifies email+password and returns the user on success (D-07/D-08). Every failure —
// unknown email, wrong password — returns the same ErrInvalidCredentials.
func (s *Service) Login(email, password string) (domain.User, error) {
	u, err := s.store.GetUserByEmail(validation.NormalizeEmail(email))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.User{}, ErrInvalidCredentials
		}
		return domain.User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return domain.User{}, ErrInvalidCredentials
	}
	return u, nil
}
