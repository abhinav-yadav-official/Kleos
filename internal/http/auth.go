package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/abhinav-yadav-official/Kleos/internal/auth"
	"github.com/go-chi/chi/v5"
)

var errUnauthorized = errors.New("unauthorized")

type AuthService interface {
	Signup(ctx context.Context, email, password, name string) (AuthResult, error)
	Login(ctx context.Context, email, password string) (AuthResult, error)
	Refresh(ctx context.Context, refresh string) (AuthResult, error)
	Logout(ctx context.Context, refresh string) error
	UserFromAccessToken(ctx context.Context, access string) (auth.User, error)
}

type AuthResult = auth.Result

type authResponse struct {
	User    auth.User `json:"user"`
	Access  string    `json:"access"`
	Refresh string    `json:"refresh"`
}

func newAuthResponse(result AuthResult) authResponse {
	return authResponse{User: result.User, Access: result.Access, Refresh: result.Refresh}
}

func registerAuthRoutes(r chi.Router, service AuthService) {
	r.Post("/api/auth/signup", func(w http.ResponseWriter, r *http.Request) {
		var req authCredentialsRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := service.Signup(r.Context(), req.Email, req.Password, req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, "signup_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, newAuthResponse(result))
	})

	r.Post("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req authCredentialsRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := service.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeJSON(w, http.StatusOK, newAuthResponse(result))
	})

	r.Post("/api/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		result, err := service.Refresh(r.Context(), req.Refresh)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_refresh", "invalid refresh token")
			return
		}
		writeJSON(w, http.StatusOK, newAuthResponse(result))
	})

	r.Post("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := service.Logout(r.Context(), req.Refresh); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_refresh", "invalid refresh token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	r.Get("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		user, err := service.UserFromAccessToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]auth.User{"user": user})
	})
}

type authCredentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type refreshRequest struct {
	Refresh string `json:"refresh"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func bearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errUnauthorized
	}
	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || token == "" {
		return "", errUnauthorized
	}
	return token, nil
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}
