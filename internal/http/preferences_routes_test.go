package apphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abhinav-yadav-official/Kleos/internal/preferences"
)

func TestPreferencesRoutesGetAndPutRoundTrip(t *testing.T) {
	auth := newFakeAuthService()
	authResult, err := auth.Signup(context.Background(), "prefs@example.com", "password123", "Prefs User")
	if err != nil {
		t.Fatalf("signup fake user: %v", err)
	}
	prefs := newFakePreferencesService()
	router := NewRouter(Dependencies{Auth: auth, Preferences: prefs})

	getDefaultReq := httptest.NewRequest(http.MethodGet, "/api/preferences", nil)
	getDefaultReq.Header.Set("Authorization", "Bearer "+authResult.Access)
	getDefault := httptest.NewRecorder()
	router.ServeHTTP(getDefault, getDefaultReq)
	if getDefault.Code != http.StatusOK {
		t.Fatalf("get default status = %d, body = %s", getDefault.Code, getDefault.Body.String())
	}
	var defaultBody preferencesResponse
	decodeResponse(t, getDefault, &defaultBody)
	if defaultBody.Preferences.TonePreset != "warm" {
		t.Fatalf("default tone preset = %q", defaultBody.Preferences.TonePreset)
	}

	update := postJSONWithMethodAndToken(t, router, http.MethodPut, "/api/preferences", authResult.Access, preferences.Record{
		JobTitles:       []string{"Founding Engineer", "Backend Engineer"},
		JobFunctions:    []string{"engineering", "platform"},
		ExperienceLevel: "senior",
		Locations:       []string{"Remote", "Bengaluru"},
		KeywordsInclude: []string{"Go", "Postgres"},
		KeywordsExclude: []string{"PHP"},
		RemoteOnly:      true,
		TonePreset:      "technical",
		ToneAddendum:    "Keep it concise.",
	})
	if update.Code != http.StatusOK {
		t.Fatalf("put status = %d, body = %s", update.Code, update.Body.String())
	}
	var updateBody preferencesResponse
	decodeResponse(t, update, &updateBody)
	if updateBody.Preferences.TonePreset != "technical" {
		t.Fatalf("updated tone preset = %q", updateBody.Preferences.TonePreset)
	}
	if len(updateBody.Preferences.JobTitles) != 2 || updateBody.Preferences.JobTitles[0] != "Founding Engineer" {
		t.Fatalf("updated job titles = %#v", updateBody.Preferences.JobTitles)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/preferences", nil)
	getReq.Header.Set("Authorization", "Bearer "+authResult.Access)
	get := httptest.NewRecorder()
	router.ServeHTTP(get, getReq)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", get.Code, get.Body.String())
	}
	var getBody preferencesResponse
	if err := json.NewDecoder(get.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getBody.Preferences.TonePreset != "technical" || !getBody.Preferences.RemoteOnly {
		t.Fatalf("preferences did not round-trip: %+v", getBody.Preferences)
	}
}

func postJSONWithMethodAndToken(t *testing.T, handler http.Handler, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
