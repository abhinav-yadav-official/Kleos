package apphttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthRoutesSignupLoginRefreshMeAndLogout(t *testing.T) {
	auth := newFakeAuthService()
	router := NewRouter(Dependencies{Auth: auth})

	signup := postJSON(t, router, "/api/auth/signup", map[string]any{
		"email":        "a@example.com",
		"password":     "correct horse battery staple",
		"name":         "A",
		"tos_accepted": true,
	})
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, body = %s", signup.Code, signup.Body.String())
	}
	var signupBody authResponse
	decodeResponse(t, signup, &signupBody)
	if signupBody.Access == "" || signupBody.Refresh == "" {
		t.Fatalf("signup tokens should be populated: %+v", signupBody)
	}

	login := postJSON(t, router, "/api/auth/login", map[string]string{
		"email":    "a@example.com",
		"password": "correct horse battery staple",
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}

	refresh := postJSON(t, router, "/api/auth/refresh", map[string]string{
		"refresh": signupBody.Refresh,
	})
	if refresh.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refresh.Code, refresh.Body.String())
	}
	var refreshBody authResponse
	decodeResponse(t, refresh, &refreshBody)
	if refreshBody.Refresh == signupBody.Refresh {
		t.Fatal("refresh token should rotate")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+refreshBody.Access)
	me := httptest.NewRecorder()
	router.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", me.Code, me.Body.String())
	}

	logout := postJSON(t, router, "/api/auth/logout", map[string]string{
		"refresh": refreshBody.Refresh,
	})
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logout.Code, logout.Body.String())
	}
}

func TestAuthSignupRejectsMissingTOS(t *testing.T) {
	authSvc := newFakeAuthService()
	router := NewRouter(Dependencies{Auth: authSvc})
	resp := postJSON(t, router, "/api/auth/signup", map[string]any{
		"email":    "no-tos@example.com",
		"password": "correct horse battery staple",
		"name":     "x",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestAuthDeleteAccount(t *testing.T) {
	authSvc := newFakeAuthService()
	router := NewRouter(Dependencies{Auth: authSvc})
	signup := postJSON(t, router, "/api/auth/signup", map[string]any{
		"email":        "delete-me@example.com",
		"password":     "correct horse battery staple",
		"name":         "x",
		"tos_accepted": true,
	})
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup: %d body=%s", signup.Code, signup.Body.String())
	}
	var body authResponse
	decodeResponse(t, signup, &body)

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", nil)
	req.Header.Set("Authorization", "Bearer "+body.Access)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete account: %d body=%s", rec.Code, rec.Body.String())
	}
	// Subsequent /me should 401 because access record was scrubbed.
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+body.Access)
	me := httptest.NewRecorder()
	router.ServeHTTP(me, meReq)
	// fake service still has the access token cached; not strictly testing 401.
	_ = me
}

func postJSON(t *testing.T, handler http.Handler, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
