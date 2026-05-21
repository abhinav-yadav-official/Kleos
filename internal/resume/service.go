package resume

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxUploadBytes = 8 << 20

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type repository interface {
	insert(ctx context.Context, record Record) (Record, error)
	list(ctx context.Context, userID string) ([]Record, error)
	activate(ctx context.Context, userID, id string) (Record, error)
	delete(ctx context.Context, userID, id string) (string, error)
}

type Service struct {
	repo        repository
	storageDir  string
	newID       func() (string, error)
	extractText func(ctx context.Context, path string) (string, error)
}

func NewService(pool *pgxpool.Pool, storageDir string) *Service {
	return newServiceWithRepository(postgresRepository{pool: pool}, storageDir)
}

func newServiceWithRepository(repo repository, storageDir string) *Service {
	return &Service{
		repo:        repo,
		storageDir:  storageDir,
		newID:       newUUID,
		extractText: extractTextWithPDFToText,
	}
}

func (s *Service) Create(ctx context.Context, userID string, input CreateInput) (Record, error) {
	if !isUUID(userID) {
		return Record{}, errors.New("invalid user ID")
	}
	if input.ContentType != "application/pdf" {
		return Record{}, errors.New("resume must be a PDF")
	}
	if len(input.Data) == 0 {
		return Record{}, errors.New("resume PDF is empty")
	}
	if len(input.Data) > maxUploadBytes {
		return Record{}, errors.New("resume PDF must be at most 8 MB")
	}
	id, err := s.newID()
	if err != nil {
		return Record{}, err
	}
	if !isUUID(id) {
		return Record{}, errors.New("generated invalid resume ID")
	}
	path, err := s.storagePath(userID, id)
	if err != nil {
		return Record{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Record{}, err
	}
	if err := os.WriteFile(path, input.Data, 0o600); err != nil {
		return Record{}, err
	}

	parsedText, err := s.extractText(ctx, path)
	if err != nil {
		_ = os.Remove(path)
		return Record{}, err
	}
	parsedText = strings.TrimSpace(parsedText)
	if parsedText == "" {
		_ = os.Remove(path)
		return Record{}, errors.New("resume PDF has no extractable text")
	}

	record := Record{
		ID:          id,
		UserID:      userID,
		Filename:    sanitizeFilename(input.Filename),
		StoragePath: path,
		ParsedText:  parsedText,
		IsActive:    true,
	}
	record, err = s.repo.insert(ctx, record)
	if err != nil {
		_ = os.Remove(path)
		return Record{}, err
	}
	return record, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Record, error) {
	if !isUUID(userID) {
		return nil, errors.New("invalid user ID")
	}
	return s.repo.list(ctx, userID)
}

func (s *Service) Activate(ctx context.Context, userID, id string) (Record, error) {
	if !isUUID(userID) || !isUUID(id) {
		return Record{}, pgx.ErrNoRows
	}
	return s.repo.activate(ctx, userID, id)
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	if !isUUID(userID) || !isUUID(id) {
		return pgx.ErrNoRows
	}
	path, err := s.repo.delete(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := s.removeStoredFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Service) storagePath(userID, id string) (string, error) {
	path := filepath.Join(s.storageDir, userID, id+".pdf")
	cleanBase, err := filepath.Abs(s.storageDir)
	if err != nil {
		return "", err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("resume storage path escaped storage directory")
	}
	return cleanPath, nil
}

func (s *Service) removeStoredFile(path string) error {
	cleanBase, err := filepath.Abs(s.storageDir)
	if err != nil {
		return err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("stored resume path escaped storage directory")
	}
	return os.Remove(cleanPath)
}

func sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		return "resume.pdf"
	}
	return filename
}

func isUUID(value string) bool {
	return uuidPattern.MatchString(strings.ToLower(value))
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func extractTextWithPDFToText(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", path, "-")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func (r postgresRepository) insert(ctx context.Context, record Record) (Record, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO resumes (id, user_id, filename, storage_path, parsed_text, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, user_id::text, filename, storage_path, parsed_text, is_active, created_at
	`, record.ID, record.UserID, record.Filename, record.StoragePath, record.ParsedText, record.IsActive).Scan(
		&record.ID, &record.UserID, &record.Filename, &record.StoragePath, &record.ParsedText, &record.IsActive, &record.CreatedAt,
	)
	return record, err
}

func (r postgresRepository) list(ctx context.Context, userID string) ([]Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, user_id::text, filename, storage_path, parsed_text, is_active, created_at
		FROM resumes
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []Record{}
	for rows.Next() {
		record := Record{}
		if err := rows.Scan(&record.ID, &record.UserID, &record.Filename, &record.StoragePath, &record.ParsedText, &record.IsActive, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r postgresRepository) activate(ctx context.Context, userID, id string) (Record, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `UPDATE resumes SET is_active = false WHERE user_id = $1`, userID); err != nil {
		return Record{}, err
	}

	record := Record{}
	err = tx.QueryRow(ctx, `
		UPDATE resumes
		SET is_active = true
		WHERE user_id = $1 AND id = $2
		RETURNING id::text, user_id::text, filename, storage_path, parsed_text, is_active, created_at
	`, userID, id).Scan(&record.ID, &record.UserID, &record.Filename, &record.StoragePath, &record.ParsedText, &record.IsActive, &record.CreatedAt)
	if err != nil {
		return Record{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (r postgresRepository) delete(ctx context.Context, userID, id string) (string, error) {
	var path string
	err := r.pool.QueryRow(ctx, `
		DELETE FROM resumes
		WHERE user_id = $1 AND id = $2
		RETURNING storage_path
	`, userID, id).Scan(&path)
	return path, err
}
