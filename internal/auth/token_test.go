package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestAccessTokenHasKidHeader(t *testing.T) {
	manager := NewTokenManager("test-secret", 15*time.Minute)
	token, err := manager.CreateAccessToken(User{ID: "u1", Email: "u@x"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Decode JWT header without verifying signature.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token shape: %s", token)
	}
	hdr, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if !strings.Contains(string(hdr), `"kid":"current"`) {
		t.Fatalf("expected kid=current in header, got %s", hdr)
	}
}

func TestRotationAcceptsPreviousSecret(t *testing.T) {
	old := NewTokenManager("old-secret", 15*time.Minute)
	oldToken, err := old.CreateAccessToken(User{ID: "u1", Email: "u@x"})
	if err != nil {
		t.Fatalf("create old: %v", err)
	}
	// Rotate: new secret becomes current, old becomes previous.
	rotated := NewTokenManagerWithRotation("new-secret", "old-secret", 15*time.Minute)
	if _, err := rotated.ParseAccessToken(oldToken); err != nil {
		t.Fatalf("rotated manager should accept old token: %v", err)
	}
	// Without the previous-secret wiring, the old token must fail.
	newOnly := NewTokenManager("new-secret", 15*time.Minute)
	if _, err := newOnly.ParseAccessToken(oldToken); err == nil {
		t.Fatal("expected reject when previous-secret not configured")
	}
	// New tokens still verify under rotation.
	newToken, err := rotated.CreateAccessToken(User{ID: "u2", Email: "u2@x"})
	if err != nil {
		t.Fatalf("create new: %v", err)
	}
	if _, err := rotated.ParseAccessToken(newToken); err != nil {
		t.Fatalf("rotated manager should accept new token: %v", err)
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	manager := NewTokenManager("test-secret", 15*time.Minute)

	token, err := manager.CreateAccessToken(User{
		ID:    "11111111-1111-1111-1111-111111111111",
		Email: "a@example.com",
		Name:  "A",
	})
	if err != nil {
		t.Fatalf("CreateAccessToken returned error: %v", err)
	}

	claims, err := manager.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken returned error: %v", err)
	}
	if claims.UserID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("UserID = %q", claims.UserID)
	}
	if claims.Email != "a@example.com" {
		t.Fatalf("Email = %q", claims.Email)
	}
}
