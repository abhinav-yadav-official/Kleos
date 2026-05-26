package apphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMTPRoutesCreateListVerifyPrimaryAndDelete(t *testing.T) {
	auth := newFakeAuthService()
	authResult, err := auth.Signup(context.Background(), "smtp@example.com", "password123", "SMTP User", true)
	if err != nil {
		t.Fatalf("signup fake user: %v", err)
	}
	smtp := newFakeSMTPService()
	router := NewRouter(Dependencies{Auth: auth, SMTP: smtp})

	create := postJSONWithToken(t, router, "/api/smtp", authResult.Access, map[string]any{
		"label":      "Gmail personal",
		"host":       "smtp.gmail.com",
		"port":       587,
		"username":   "smtp@example.com",
		"password":   "app-password",
		"from_email": "smtp@example.com",
		"from_name":  "SMTP User",
		"use_tls":    true,
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", create.Code, create.Body.String())
	}
	if create.Body.String() == "" || contains(create.Body.String(), "app-password") {
		t.Fatalf("create response leaked password: %s", create.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/smtp", nil)
	listReq.Header.Set("Authorization", "Bearer "+authResult.Access)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, listReq)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}

	verify := postJSONWithToken(t, router, "/api/smtp/smtp-1/verify", authResult.Access, map[string]any{})
	if verify.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", verify.Code, verify.Body.String())
	}

	primary := postJSONWithToken(t, router, "/api/smtp/smtp-1/primary", authResult.Access, map[string]any{})
	if primary.Code != http.StatusOK {
		t.Fatalf("primary status = %d, body = %s", primary.Code, primary.Body.String())
	}
	if !contains(primary.Body.String(), `"is_primary":true`) {
		t.Fatalf("primary response should mark credential primary: %s", primary.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/smtp/smtp-1", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+authResult.Access)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func postJSONWithToken(t *testing.T, handler http.Handler, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	body := bytes.NewReader(bodyBytes)
	realReq := httptest.NewRequest(http.MethodPost, path, body)
	realReq.Header.Set("Content-Type", "application/json")
	realReq.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, realReq)
	return rec
}

func contains(value, needle string) bool {
	return strings.Contains(value, needle)
}
