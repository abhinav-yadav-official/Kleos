package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	IsAdmin bool   `json:"is_admin,omitempty"`
}

type AccessClaims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// TokenManager signs tokens with the current secret and verifies tokens that
// were signed with either the current or the previous secret (for grace-period
// JWT rotation). The `kid` header on the JWT picks the secret on parse; tokens
// without a kid (legacy) try current then previous.
type TokenManager struct {
	currentKid     string
	currentSecret  []byte
	previousSecret []byte // optional; empty disables previous-secret verification
	accessTTL      time.Duration
}

const (
	KidCurrent  = "current"
	KidPrevious = "previous"
)

// NewTokenManager constructs a manager with a single (current) secret. Backward
// compatible with code that has not yet adopted rotation.
func NewTokenManager(secret string, accessTTL time.Duration) TokenManager {
	return TokenManager{
		currentKid:    KidCurrent,
		currentSecret: []byte(secret),
		accessTTL:     accessTTL,
	}
}

// NewTokenManagerWithRotation lets callers wire a previous secret for the grace
// period; tokens signed by either will still verify until natural expiry.
func NewTokenManagerWithRotation(currentSecret, previousSecret string, accessTTL time.Duration) TokenManager {
	m := NewTokenManager(currentSecret, accessTTL)
	if previousSecret != "" {
		m.previousSecret = []byte(previousSecret)
	}
	return m
}

func (m TokenManager) CreateAccessToken(user User) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = m.currentKid
	return tok.SignedString(m.currentSecret)
}

func (m TokenManager) ParseAccessToken(raw string) (AccessClaims, error) {
	// Try current secret first; on failure, fall back to previous-secret if
	// configured. kid header is informational and not used for dispatch — that
	// way rotating the "current" secret still verifies tokens signed under the
	// old "current" because we retry with previous.
	for _, secret := range m.candidateSecrets() {
		claims := AccessClaims{}
		token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return secret, nil
		})
		if err == nil && token.Valid {
			return claims, nil
		}
	}
	return AccessClaims{}, errors.New("invalid token")
}

func (m TokenManager) candidateSecrets() [][]byte {
	out := [][]byte{m.currentSecret}
	if len(m.previousSecret) > 0 {
		out = append(out, m.previousSecret)
	}
	return out
}

func NewRefreshToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
