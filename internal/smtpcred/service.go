package smtpcred

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	appcrypto "github.com/abhinav-yadav-official/Kleos/internal/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultWarmupDay1Limit = 5

type Service struct {
	pool   *pgxpool.Pool
	codec  *appcrypto.AESGCM
	dialer func(ctx context.Context, input verifyInput) error
}

type verifyInput struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	UseTLS    bool
}

func NewService(pool *pgxpool.Pool, codec *appcrypto.AESGCM) *Service {
	return &Service{pool: pool, codec: codec, dialer: verifySMTP}
}

func (s *Service) Create(ctx context.Context, userID string, input CreateInput) (Credential, error) {
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.FromEmail = strings.TrimSpace(input.FromEmail)
	if input.Label == "" || input.Host == "" || input.Port == 0 || input.Username == "" || input.Password == "" || input.FromEmail == "" {
		return Credential{}, errors.New("missing required SMTP field")
	}
	ciphertext, nonce, err := s.codec.EncryptString(input.Password)
	if err != nil {
		return Credential{}, err
	}

	record := Credential{}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO smtp_credentials (
			user_id, label, host, port, username, password_cipher, password_nonce,
			from_email, from_name, use_tls
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id::text, user_id::text, label, host, port, username,
			from_email::text, COALESCE(from_name, ''), use_tls, verified_at,
			COALESCE(last_error, ''), is_primary, created_at
	`, userID, input.Label, input.Host, input.Port, input.Username, ciphertext, nonce,
		input.FromEmail, input.FromName, input.UseTLS).Scan(
		&record.ID, &record.UserID, &record.Label, &record.Host, &record.Port, &record.Username,
		&record.FromEmail, &record.FromName, &record.UseTLS, &record.VerifiedAt,
		&record.LastError, &record.IsPrimary, &record.CreatedAt,
	)
	return record, err
}

func (s *Service) List(ctx context.Context, userID string) ([]Credential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, user_id::text, label, host, port, username,
			from_email::text, COALESCE(from_name, ''), use_tls, verified_at,
			COALESCE(last_error, ''), is_primary, created_at
		FROM smtp_credentials
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []Credential{}
	for rows.Next() {
		record := Credential{}
		if err := rows.Scan(
			&record.ID, &record.UserID, &record.Label, &record.Host, &record.Port, &record.Username,
			&record.FromEmail, &record.FromName, &record.UseTLS, &record.VerifiedAt,
			&record.LastError, &record.IsPrimary, &record.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Service) Verify(ctx context.Context, userID, id string) (VerifyResult, error) {
	input, err := s.verifyInput(ctx, userID, id)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := s.dialer(ctx, input); err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE smtp_credentials SET last_error = $1 WHERE id = $2 AND user_id = $3`, err.Error(), id, userID)
		return VerifyResult{OK: false, Detail: err.Error()}, nil
	}
	_, err = s.pool.Exec(ctx, `UPDATE smtp_credentials SET verified_at = now(), last_error = NULL WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := ensureWarmupState(ctx, s.pool, userID, id, defaultWarmupDay1Limit); err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{OK: true, Detail: "ok"}, nil
}

func (s *Service) SetPrimary(ctx context.Context, userID, id string) (Credential, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Credential{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `UPDATE smtp_credentials SET is_primary = false WHERE user_id = $1`, userID); err != nil {
		return Credential{}, err
	}

	record := Credential{}
	err = tx.QueryRow(ctx, `
		UPDATE smtp_credentials
		SET is_primary = true
		WHERE id = $1 AND user_id = $2
		RETURNING id::text, user_id::text, label, host, port, username,
			from_email::text, COALESCE(from_name, ''), use_tls, verified_at,
			COALESCE(last_error, ''), is_primary, created_at
	`, id, userID).Scan(
		&record.ID, &record.UserID, &record.Label, &record.Host, &record.Port, &record.Username,
		&record.FromEmail, &record.FromName, &record.UseTLS, &record.VerifiedAt,
		&record.LastError, &record.IsPrimary, &record.CreatedAt,
	)
	if err != nil {
		return Credential{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Credential{}, err
	}
	return record, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM smtp_credentials WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Service) verifyInput(ctx context.Context, userID, id string) (verifyInput, error) {
	var input verifyInput
	var ciphertext []byte
	var nonce []byte
	err := s.pool.QueryRow(ctx, `
		SELECT host, port, username, password_cipher, password_nonce, from_email::text, use_tls
		FROM smtp_credentials
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&input.Host, &input.Port, &input.Username, &ciphertext, &nonce, &input.FromEmail, &input.UseTLS)
	if err != nil {
		return verifyInput{}, err
	}
	password, err := s.codec.DecryptString(ciphertext, nonce)
	if err != nil {
		return verifyInput{}, err
	}
	input.Password = password
	return input, nil
}

type warmupExec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func ensureWarmupState(ctx context.Context, exec warmupExec, userID, smtpID string, day1Limit int) error {
	_, err := exec.Exec(ctx, `
		INSERT INTO warmup_state (user_id, smtp_id, start_date, current_day, todays_sent, todays_limit, last_rollover)
		VALUES ($1::uuid, $2::uuid, CURRENT_DATE, 1, 0, $3, CURRENT_DATE)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, smtpID, day1Limit)
	return err
}

func verifySMTP(ctx context.Context, input verifyInput) error {
	addr := fmt.Sprintf("%s:%d", input.Host, input.Port)
	tlsConfig := &tls.Config{ServerName: input.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	if input.UseTLS && input.Port == 465 {
		dialer := tls.Dialer{Config: tlsConfig}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		dialer := net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, input.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if input.UseTLS && input.Port != 465 {
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}

	auth := smtp.PlainAuth("", input.Username, input.Password, input.Host)
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Noop(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- client.Quit()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New("smtp quit timeout")
	}
}
