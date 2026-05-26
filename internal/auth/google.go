package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	googleAuthEndpoint     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint    = "https://oauth2.googleapis.com/token"
	googleUserInfoEndpoint = "https://openidconnect.googleapis.com/v1/userinfo"
)

// GoogleOAuth handles the server-side Google OpenID Connect flow.
// Ported from leetdrill/internal/auth/google.go to keep behavior aligned.
type GoogleOAuth struct {
	ClientID         string
	ClientSecret     string
	RedirectURL      string
	AuthEndpoint     string
	TokenEndpoint    string
	UserInfoEndpoint string
	HTTPClient       *http.Client
}

// GoogleUser is the identity data Kleos needs from Google.
type GoogleUser struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// GoogleOAuthFromEnv constructs a Google OAuth client when credentials exist.
// appBase is the public origin (e.g. https://abhiyadav.in); the callback
// always lives at appBase + /kleos/api/auth/google/callback.
func GoogleOAuthFromEnv(appBase string) (*GoogleOAuth, error) {
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	if clientID == "" && clientSecret == "" {
		return nil, nil
	}
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("auth: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must both be set")
	}
	appBase = strings.TrimRight(strings.TrimSpace(appBase), "/")
	if appBase == "" {
		return nil, errors.New("auth: app base required for google oauth")
	}
	// APP_BASE_URL may already include the /kleos prefix; tolerate either form.
	appBase = strings.TrimSuffix(appBase, "/kleos")
	return &GoogleOAuth{
		ClientID:         clientID,
		ClientSecret:     clientSecret,
		RedirectURL:      appBase + "/kleos/api/auth/google/callback",
		AuthEndpoint:     googleAuthEndpoint,
		TokenEndpoint:    googleTokenEndpoint,
		UserInfoEndpoint: googleUserInfoEndpoint,
		HTTPClient:       &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// AuthCodeURL returns the Google authorization URL for state.
func (g *GoogleOAuth) AuthCodeURL(state string) string {
	v := url.Values{}
	v.Set("client_id", g.ClientID)
	v.Set("redirect_uri", g.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", "openid email profile")
	v.Set("state", state)
	v.Set("prompt", "select_account")
	return g.AuthEndpoint + "?" + v.Encode()
}

// ExchangeUser exchanges an auth code and reads the Google userinfo endpoint.
func (g *GoogleOAuth) ExchangeUser(ctx context.Context, code string) (GoogleUser, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return GoogleUser{}, errors.New("auth: missing google code")
	}
	token, err := g.exchangeToken(ctx, code)
	if err != nil {
		return GoogleUser{}, err
	}
	user, err := g.userInfo(ctx, token)
	if err != nil {
		return GoogleUser{}, err
	}
	user.Sub = strings.TrimSpace(user.Sub)
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	if user.Sub == "" || user.Email == "" {
		return GoogleUser{}, errors.New("auth: google user missing subject or email")
	}
	if !user.EmailVerified {
		return GoogleUser{}, errors.New("auth: google email is not verified")
	}
	return user, nil
}

func (g *GoogleOAuth) exchangeToken(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("redirect_uri", g.RedirectURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("google token exchange: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("google token exchange: status %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode google token: %w", err)
	}
	if body.AccessToken == "" {
		return "", errors.New("auth: google token response missing access token")
	}
	return body.AccessToken, nil
}

func (g *GoogleOAuth) userInfo(ctx context.Context, token string) (GoogleUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.UserInfoEndpoint, nil)
	if err != nil {
		return GoogleUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("google userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GoogleUser{}, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return GoogleUser{}, fmt.Errorf("decode google userinfo: %w", err)
	}
	return user, nil
}

func (g *GoogleOAuth) httpClient() *http.Client {
	if g.HTTPClient != nil {
		return g.HTTPClient
	}
	return http.DefaultClient
}
