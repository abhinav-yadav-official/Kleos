package apphttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type checkFunc func() error

func (f checkFunc) Ping() error {
	return f()
}

func TestHealthzReturnsOK(t *testing.T) {
	router := NewRouter(Dependencies{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != `{"ok":true}`+"\n" {
		t.Fatalf("body = %q, want ok JSON", rec.Body.String())
	}
}

func TestReadyzReturnsOKWhenDependenciesPing(t *testing.T) {
	router := NewRouter(Dependencies{
		DB:    checkFunc(func() error { return nil }),
		Redis: checkFunc(func() error { return nil }),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readyz", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzReturnsUnavailableWhenDependencyFails(t *testing.T) {
	router := NewRouter(Dependencies{
		DB:    checkFunc(func() error { return errors.New("db down") }),
		Redis: checkFunc(func() error { return nil }),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readyz", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
