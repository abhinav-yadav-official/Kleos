package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
	ErrInvalidAccess      = errors.New("invalid access token")
)

type Service struct {
	pool       *pgxpool.Pool
	tokens     TokenManager
	refreshTTL time.Duration
}

type Result struct {
	User    User
	Access  string
	Refresh string
}

func NewService(pool *pgxpool.Pool, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		pool:       pool,
		tokens:     NewTokenManager(jwtSecret, accessTTL),
		refreshTTL: refreshTTL,
	}
}

func (s *Service) Signup(ctx context.Context, email, password, name string) (Result, error) {
	email = normalizeEmail(email)
	if email == "" || len(password) < 8 {
		return Result{}, ErrInvalidCredentials
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Result{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	user := User{}
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id::text, email::text, COALESCE(name, '')
	`, email, hash, name).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO preferences (user_id) VALUES ($1)`, user.ID); err != nil {
		return Result{}, err
	}
	result, err := s.issueTokens(ctx, tx, user)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (Result, error) {
	user, hash, err := s.userByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return Result{}, ErrInvalidCredentials
	}
	if err := CheckPassword(hash, password); err != nil {
		return Result{}, ErrInvalidCredentials
	}
	return s.issueTokens(ctx, s.pool, user)
}

func (s *Service) Refresh(ctx context.Context, refresh string) (Result, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tokenHash := HashRefreshToken(refresh)
	user := User{}
	var expiresAt time.Time
	var revokedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT u.id::text, u.email::text, COALESCE(u.name, ''), rt.expires_at, rt.revoked_at
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1 AND u.is_active = true
	`, tokenHash).Scan(&user.ID, &user.Email, &user.Name, &expiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrInvalidRefresh
		}
		return Result{}, err
	}
	if revokedAt != nil || time.Now().After(expiresAt) {
		return Result{}, ErrInvalidRefresh
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1`, tokenHash); err != nil {
		return Result{}, err
	}
	result, err := s.issueTokens(ctx, tx, user)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *Service) Logout(ctx context.Context, refresh string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, HashRefreshToken(refresh))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidRefresh
	}
	return nil
}

func (s *Service) UserFromAccessToken(ctx context.Context, access string) (User, error) {
	claims, err := s.tokens.ParseAccessToken(access)
	if err != nil {
		return User{}, ErrInvalidAccess
	}
	user := User{}
	err = s.pool.QueryRow(ctx, `
		SELECT id::text, email::text, COALESCE(name, ''), is_admin
		FROM users
		WHERE id = $1 AND is_active = true
	`, claims.UserID).Scan(&user.ID, &user.Email, &user.Name, &user.IsAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidAccess
		}
		return User{}, err
	}
	return user, nil
}

type tokenStore interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (s *Service) issueTokens(ctx context.Context, store tokenStore, user User) (Result, error) {
	access, err := s.tokens.CreateAccessToken(user)
	if err != nil {
		return Result{}, err
	}
	refresh, err := NewRefreshToken()
	if err != nil {
		return Result{}, err
	}
	_, err = store.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, user.ID, HashRefreshToken(refresh), time.Now().Add(s.refreshTTL))
	if err != nil {
		return Result{}, err
	}
	return Result{User: user, Access: access, Refresh: refresh}, nil
}

func (s *Service) userByEmail(ctx context.Context, email string) (User, string, error) {
	user := User{}
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, email::text, COALESCE(name, ''), password_hash
		FROM users
		WHERE email = $1 AND is_active = true
	`, email).Scan(&user.ID, &user.Email, &user.Name, &hash)
	return user, hash, err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
