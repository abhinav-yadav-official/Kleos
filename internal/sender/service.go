package sender

import (
	"context"
	"errors"

	"github.com/abhinav-yadav-official/Kleos/internal/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool      *pgxpool.Pool
	codec     *crypto.AESGCM
	transport Transport
	// config knobs (sensible defaults).
	dailySendCap int
	day1Limit    int
	cap          int
	warmupDays   int
	growth       float64
}

func NewService(pool *pgxpool.Pool, codec *crypto.AESGCM, transport Transport) *Service {
	return &Service{
		pool:         pool,
		codec:        codec,
		transport:    transport,
		dailySendCap: 100,
		day1Limit:    DefaultWarmupDay1Limit,
		cap:          DefaultWarmupCap,
		warmupDays:   DefaultWarmupDays,
		growth:       DefaultWarmupGrowth,
	}
}

// Outcome describes the final state for the processed match.
type Outcome struct {
	State        string // "sent" | "failed" | "skipped" | "queued"
	Reason       string
	MessageID    string
	SMTPResponse string
	Class        ErrorClass
	Err          error
}

var (
	ErrNoChosenDraft   = errors.New("no chosen draft for match")
	ErrNoWarmupState   = errors.New("warmup_state row missing for user")
	ErrWarmupPaused    = errors.New("warmup paused")
	ErrLimitReached    = errors.New("todays_limit reached")
)

type sendContext struct {
	MatchID         string
	UserID          string
	DraftID         string
	RecruiterID     string
	RecruiterEmail  string
	Subject         string
	BodyText        string
	SMTPID          string
	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPasswordEnc []byte
	SMTPPasswordNon []byte
	FromEmail       string
	FromName        string
	UseTLS          bool
}

func (s *Service) loadContext(ctx context.Context, matchID string) (sendContext, error) {
	var c sendContext
	err := s.pool.QueryRow(ctx, `
		SELECT m.id::text, ca.user_id::text,
			d.id::text, r.id::text, r.email::text,
			d.subject, d.body_text,
			sc.id::text, sc.host, sc.port, sc.username,
			sc.password_cipher, sc.password_nonce,
			sc.from_email::text, COALESCE(sc.from_name, ''), sc.use_tls
		FROM campaign_matches m
		JOIN campaigns ca ON ca.id = m.campaign_id
		JOIN email_drafts d ON d.match_id = m.id AND d.chosen = true
		JOIN recruiters r ON r.id = d.recruiter_id
		JOIN smtp_credentials sc ON sc.id = ca.smtp_id
		WHERE m.id = $1::uuid
	`, matchID).Scan(&c.MatchID, &c.UserID, &c.DraftID, &c.RecruiterID, &c.RecruiterEmail,
		&c.Subject, &c.BodyText, &c.SMTPID, &c.SMTPHost, &c.SMTPPort, &c.SMTPUser,
		&c.SMTPPasswordEnc, &c.SMTPPasswordNon, &c.FromEmail, &c.FromName, &c.UseTLS)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNoChosenDraft
	}
	return c, err
}

// SendOne fully processes the given match: loads context, checks warmup +
// denylist + dedup, decrypts SMTP creds, sends, persists sent_emails, updates
// warmup counter, and transitions the match state. Caller is responsible for
// applying the §11 random jitter *before* calling SendOne.
func (s *Service) SendOne(ctx context.Context, matchID string) Outcome {
	sc, err := s.loadContext(ctx, matchID)
	if err != nil {
		return Outcome{State: "failed", Reason: "load_context", Err: err}
	}

	// Pre-check: denylist for this recruiter email.
	var blocked bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM email_denylist WHERE email = $1)`,
		sc.RecruiterEmail).Scan(&blocked); err != nil {
		return Outcome{State: "failed", Reason: "denylist_check", Err: err}
	}
	if blocked {
		_ = s.transitionState(ctx, sc.MatchID, "skipped")
		return Outcome{State: "skipped", Reason: "denylisted"}
	}

	// Pre-check: dedup guard for (user_id, recruiter_email).
	var dupe bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sent_emails WHERE user_id = $1::uuid AND recruiter_email = $2)`,
		sc.UserID, sc.RecruiterEmail).Scan(&dupe); err != nil {
		return Outcome{State: "failed", Reason: "dedup_check", Err: err}
	}
	if dupe {
		_ = s.transitionState(ctx, sc.MatchID, "skipped")
		return Outcome{State: "skipped", Reason: "already_sent"}
	}

	// Warmup gating.
	var paused bool
	var sent, limit int
	err = s.pool.QueryRow(ctx, `
		SELECT paused, todays_sent, todays_limit
		FROM warmup_state WHERE user_id = $1::uuid
	`, sc.UserID).Scan(&paused, &sent, &limit)
	if errors.Is(err, pgx.ErrNoRows) {
		return Outcome{State: "queued", Reason: "no_warmup_state", Err: ErrNoWarmupState}
	}
	if err != nil {
		return Outcome{State: "failed", Reason: "warmup_load", Err: err}
	}
	if paused {
		return Outcome{State: "queued", Reason: "warmup_paused", Err: ErrWarmupPaused}
	}
	if sent >= limit {
		return Outcome{State: "queued", Reason: "limit_reached", Err: ErrLimitReached}
	}

	// Decrypt SMTP password.
	password, err := s.codec.DecryptString(sc.SMTPPasswordEnc, sc.SMTPPasswordNon)
	if err != nil {
		return Outcome{State: "failed", Reason: "decrypt", Err: err}
	}

	msg, msgID := BuildMessage(Message{
		FromEmail: sc.FromEmail, FromName: sc.FromName,
		ToEmail: sc.RecruiterEmail, Subject: sc.Subject, BodyText: sc.BodyText,
	})

	creds := Credentials{
		Host: sc.SMTPHost, Port: sc.SMTPPort,
		Username: sc.SMTPUser, Password: password,
		FromEmail: sc.FromEmail, FromName: sc.FromName, UseTLS: sc.UseTLS,
	}

	result, err := s.transport.Send(ctx, creds, sc.RecruiterEmail, msg)
	if err == nil {
		return s.recordSuccess(ctx, sc, msgID, result)
	}

	class := ClassifyError(err)
	return s.recordFailure(ctx, sc, msgID, err, class)
}

func (s *Service) recordSuccess(ctx context.Context, sc sendContext, msgID string, r SendResult) Outcome {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Outcome{State: "failed", Reason: "tx_begin", Err: err}
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO sent_emails (user_id, match_id, draft_id, recruiter_email, smtp_id, message_id, status, smtp_response)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, 'sent', $7)
	`, sc.UserID, sc.MatchID, sc.DraftID, sc.RecruiterEmail, sc.SMTPID, msgID, r.SMTPResponse); err != nil {
		return Outcome{State: "failed", Reason: "insert_sent", Err: err}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE warmup_state SET todays_sent = todays_sent + 1 WHERE user_id = $1::uuid`,
		sc.UserID); err != nil {
		return Outcome{State: "failed", Reason: "bump_warmup", Err: err}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE campaign_matches SET state = 'sent' WHERE id = $1::uuid`,
		sc.MatchID); err != nil {
		return Outcome{State: "failed", Reason: "transition", Err: err}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log (user_id, actor, action, target, meta)
		VALUES ($1::uuid, 'system', 'email_sent', $2, jsonb_build_object('message_id', $3::text))
	`, sc.UserID, sc.RecruiterEmail, msgID); err != nil {
		return Outcome{State: "failed", Reason: "audit", Err: err}
	}
	if err := tx.Commit(ctx); err != nil {
		return Outcome{State: "failed", Reason: "tx_commit", Err: err}
	}
	return Outcome{State: "sent", MessageID: msgID, SMTPResponse: r.SMTPResponse}
}

func (s *Service) recordFailure(ctx context.Context, sc sendContext, msgID string, err error, class ErrorClass) Outcome {
	status := "smtp_error"
	switch class {
	case ClassRecipientReject:
		status = "bounced"
	case ClassAuthFailure:
		status = "permanent_fail"
	case ClassTransient:
		// Leave match in 'queued' so retry layer can pick it up; do not insert sent_emails.
		return Outcome{State: "queued", Reason: "transient_error", Class: class, Err: err}
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO sent_emails (user_id, match_id, draft_id, recruiter_email, smtp_id, message_id, status, smtp_response)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, $7, $8)
	`, sc.UserID, sc.MatchID, sc.DraftID, sc.RecruiterEmail, sc.SMTPID, msgID, status, truncate(err.Error(), 500))

	if class == ClassRecipientReject {
		_, _ = s.pool.Exec(ctx,
			`INSERT INTO email_denylist (email, reason) VALUES ($1, 'hard_bounce')
			 ON CONFLICT (email) DO NOTHING`, sc.RecruiterEmail)
	}
	if class == ClassAuthFailure {
		_, _ = s.pool.Exec(ctx,
			`UPDATE smtp_credentials SET last_error = $2 WHERE id = $1::uuid`,
			sc.SMTPID, truncate(err.Error(), 500))
		_, _ = s.pool.Exec(ctx,
			`UPDATE warmup_state SET paused = true, notes = $2 WHERE user_id = $1::uuid`,
			sc.UserID, "smtp_auth_failure")
	}
	_ = s.transitionState(ctx, sc.MatchID, "failed")
	return Outcome{State: "failed", Reason: class.String(), Class: class, Err: err, MessageID: msgID}
}

func (s *Service) transitionState(ctx context.Context, matchID, state string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE campaign_matches SET state = $2 WHERE id = $1::uuid`, matchID, state)
	return err
}

// EnsureWarmupState creates the warmup_state row for a user (idempotent). Used
// after the first verified SMTP credential.
func (s *Service) EnsureWarmupState(ctx context.Context, userID, smtpID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO warmup_state (user_id, smtp_id, start_date, current_day, todays_sent, todays_limit, last_rollover)
		VALUES ($1::uuid, $2::uuid, CURRENT_DATE, 1, 0, $3, CURRENT_DATE)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, smtpID, s.day1Limit)
	return err
}

// Rollover advances every non-paused warmup row by one day (or graduates if
// past warmup_days). Caller schedules at 00:05 UTC.
func (s *Service) Rollover(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT user_id::text, current_day,
		       COALESCE((SELECT daily_send_cap FROM users WHERE id = ws.user_id), 100)
		FROM warmup_state ws
		WHERE NOT paused AND last_rollover < CURRENT_DATE
	`)
	if err != nil {
		return 0, err
	}
	type rowT struct {
		UserID   string
		Day      int
		DailyCap int
	}
	var rs []rowT
	for rows.Next() {
		var r rowT
		if err := rows.Scan(&r.UserID, &r.Day, &r.DailyCap); err != nil {
			rows.Close()
			return 0, err
		}
		rs = append(rs, r)
	}
	rows.Close()
	bumped := 0
	for _, r := range rs {
		next := r.Day + 1
		limit := DayNLimit(next, s.day1Limit, s.cap, s.warmupDays, r.DailyCap, s.growth)
		if _, err := s.pool.Exec(ctx, `
			UPDATE warmup_state
			SET current_day = $2, todays_sent = 0, todays_limit = $3,
			    last_rollover = CURRENT_DATE
			WHERE user_id = $1::uuid
		`, r.UserID, next, limit); err != nil {
			return bumped, err
		}
		bumped++
	}
	return bumped, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
