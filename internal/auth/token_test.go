package auth

import (
	"testing"
	"time"
)

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
