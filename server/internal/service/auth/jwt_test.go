package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
)

func TestIssueAndVerifyTokenRoundTrip(t *testing.T) {
	t.Parallel()
	svc := newService()
	token, err := svc.IssueToken("user-123")
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("IssueToken() returned an empty string")
	}

	userID, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if userID != "user-123" {
		t.Errorf("VerifyToken() = %q, want user-123", userID)
	}
}

func TestVerifyTokenRejectsGarbage(t *testing.T) {
	t.Parallel()
	svc := newService()
	if _, err := svc.VerifyToken("not-a-jwt"); err == nil {
		t.Error("VerifyToken() should reject a malformed token")
	}
}

func TestVerifyTokenRejectsWrongSecret(t *testing.T) {
	t.Parallel()
	signer := auth.New(nil, testBcryptCost, "signer-secret-at-least-32-characters")
	verifier := auth.New(nil, testBcryptCost, "verifier-secret-at-least-32-characters")

	token, err := signer.IssueToken("user-123")
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	if _, err := verifier.VerifyToken(token); err == nil {
		t.Error("VerifyToken() should reject a token signed with a different secret")
	}
}

func TestVerifyTokenRejectsExpiredToken(t *testing.T) {
	t.Parallel()
	secret := []byte("expired-secret-at-least-32-characters!!")
	claims := jwt.RegisteredClaims{
		Subject:   "user-123",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-8 * 24 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("signing expired token: %v", err)
	}

	svc := auth.New(nil, testBcryptCost, string(secret))
	if _, err := svc.VerifyToken(expired); err == nil {
		t.Error("VerifyToken() should reject an expired token")
	}
}

func TestVerifyTokenRejectsAlgNone(t *testing.T) {
	t.Parallel()
	// A token forged with alg=none and no signature must be rejected even though its claims
	// look valid — this is what the explicit HMAC-method check in VerifyToken guards against.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   "user-123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing alg=none token: %v", err)
	}

	svc := newService()
	if _, err := svc.VerifyToken(unsigned); err == nil {
		t.Error("VerifyToken() should reject an alg=none token")
	}
}

func TestTokenTTLIsSevenDays(t *testing.T) {
	t.Parallel()
	if auth.TokenTTL != 7*24*time.Hour {
		t.Errorf("TokenTTL = %v, want 7 days (D-09)", auth.TokenTTL)
	}
}
