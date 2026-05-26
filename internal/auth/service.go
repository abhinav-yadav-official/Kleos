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
	return NewServiceWithRotation(pool, jwtSecret, "", accessTTL, refreshTTL)
}

// NewServiceWithRotation lets callers wire a previous JWT secret for the
// rotation grace period. Tokens signed under either secret will verify until
// natural expiry.
func NewServiceWithRotation(pool *pgxpool.Pool, jwtSecret, jwtSecretPrevious string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		pool:       pool,
		tokens:     NewTokenManagerWithRotation(jwtSecret, jwtSecretPrevious, accessTTL),
		refreshTTL: refreshTTL,
	}
}

var ErrTOSNotAccepted = errors.New("terms of service must be accepted")

func (s *Service) Signup(ctx context.Context, email, password, name string, tosAccepted bool) (Result, error) {
	email = normalizeEmail(email)
	if email == "" || len(password) < 8 {
		return Result{}, ErrInvalidCredentials
	}
	if !tosAccepted {
		return Result{}, ErrTOSNotAccepted
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
		INSERT INTO users (email, password_hash, name, tos_accepted_at)
		VALUES ($1, $2, $3, now())
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

// DeleteAccount permanently removes the user and cascades to all owned rows
// (refresh tokens, preferences, smtp_credentials, resumes, campaigns,
// campaign_matches, email_drafts, sent_emails, warmup_state). audit_log rows
// are preserved with user_id=NULL.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidAccess
	}
	return nil
}

// EnsureGoogleUser returns a verified user linked to the Google subject.
// Insert path also marks ToS accepted (clicking "Continue with Google" implies
// acceptance — the button copy must state this on the frontend) and stamps
// email_verified_at since Google asserts the email is verified.
func (s *Service) EnsureGoogleUser(ctx context.Context, googleSub, email, name string) (Result, error) {
	googleSub = strings.TrimSpace(googleSub)
	email = strings.ToLower(strings.TrimSpace(email))
	if googleSub == "" || email == "" {
		return Result{}, errors.New("auth: google subject and email are required")
	}
	const q = `
WITH updated_google AS (
	UPDATE users
	SET email_verified_at = COALESCE(email_verified_at, now()),
	    tos_accepted_at = COALESCE(tos_accepted_at, now()),
	    updated_at = now()
	WHERE google_sub = $1
	RETURNING id::text, email::text, COALESCE(name, ''), is_admin
), inserted_or_linked AS (
	INSERT INTO users (email, google_sub, name, email_verified_at, tos_accepted_at)
	SELECT $2, $1, NULLIF($3, ''), now(), now()
	WHERE NOT EXISTS (SELECT 1 FROM updated_google)
	ON CONFLICT (email) DO UPDATE
	SET google_sub = EXCLUDED.google_sub,
	    email_verified_at = COALESCE(users.email_verified_at, now()),
	    tos_accepted_at = COALESCE(users.tos_accepted_at, now()),
	    name = COALESCE(NULLIF(EXCLUDED.name, ''), users.name),
	    updated_at = now()
	WHERE users.google_sub IS NULL OR users.google_sub = EXCLUDED.google_sub
	RETURNING id::text, email::text, COALESCE(name, ''), is_admin
)
SELECT id, email, name, is_admin FROM updated_google
UNION ALL
SELECT id, email, name, is_admin FROM inserted_or_linked
LIMIT 1`
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var user User
	if err := tx.QueryRow(ctx, q, googleSub, email, name).Scan(&user.ID, &user.Email, &user.Name, &user.IsAdmin); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, errors.New("email already linked to another google account")
		}
		return Result{}, err
	}
	// Ensure preferences row exists for first-time google users; safe no-op
	// otherwise due to ON CONFLICT.
	if _, err := tx.Exec(ctx,
		`INSERT INTO preferences (user_id) VALUES ($1::uuid) ON CONFLICT (user_id) DO NOTHING`,
		user.ID); err != nil {
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
