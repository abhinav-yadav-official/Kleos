package apphttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/resume"
	"github.com/go-chi/chi/v5"
)

const maxResumeUploadBytes = 8 << 20

type ResumeService interface {
	Create(ctx context.Context, userID string, input resume.CreateInput) (resume.Record, error)
	List(ctx context.Context, userID string) ([]resume.Record, error)
	Activate(ctx context.Context, userID, id string) (resume.Record, error)
	Delete(ctx context.Context, userID, id string) error
}

type resumeResponse struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	Filename          string    `json:"filename"`
	ParsedTextPreview string    `json:"parsed_text_preview"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

func newResumeResponse(record resume.Record) resumeResponse {
	preview := strings.TrimSpace(record.ParsedText)
	if len(preview) > 500 {
		preview = preview[:500]
	}
	return resumeResponse{
		ID:                record.ID,
		UserID:            record.UserID,
		Filename:          record.Filename,
		ParsedTextPreview: preview,
		IsActive:          record.IsActive,
		CreatedAt:         record.CreatedAt,
	}
}

func registerResumeRoutes(r chi.Router, authService AuthService, resumeService ResumeService) {
	r.Post("/api/resumes", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		input, ok := parseResumeUpload(w, r)
		if !ok {
			return
		}
		record, err := resumeService.Create(r.Context(), user.ID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "resume_create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]resumeResponse{"resume": newResumeResponse(record)})
	})

	r.Get("/api/resumes", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		records, err := resumeService.List(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "resume_list_failed", "could not list resumes")
			return
		}
		responses := make([]resumeResponse, 0, len(records))
		for _, record := range records {
			responses = append(responses, newResumeResponse(record))
		}
		writeJSON(w, http.StatusOK, map[string][]resumeResponse{"resumes": responses})
	})

	r.Post("/api/resumes/{id}/activate", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		record, err := resumeService.Activate(r.Context(), user.ID, chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "resume_not_found", "resume not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]resumeResponse{"resume": newResumeResponse(record)})
	})

	r.Delete("/api/resumes/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if err := resumeService.Delete(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
			writeError(w, http.StatusNotFound, "resume_not_found", "resume not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}

func parseResumeUpload(w http.ResponseWriter, r *http.Request) (resume.CreateInput, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxResumeUploadBytes+1024)
	if err := r.ParseMultipartForm(maxResumeUploadBytes); err != nil {
		status := http.StatusBadRequest
		code := "resume_invalid_multipart"
		message := "request must be multipart/form-data with a file field"
		if errors.Is(err, http.ErrNotMultipart) || strings.Contains(err.Error(), "request body too large") {
			status = http.StatusBadRequest
			code = "resume_too_large"
			message = "resume PDF must be at most 8 MB"
		}
		writeError(w, status, code, message)
		return resume.CreateInput{}, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "resume_missing_file", "multipart field file is required")
		return resume.CreateInput{}, false
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType != "application/pdf" {
		writeError(w, http.StatusBadRequest, "resume_invalid_type", "resume must be a PDF")
		return resume.CreateInput{}, false
	}
	var buffer bytes.Buffer
	limit := int64(maxResumeUploadBytes + 1)
	written, err := io.CopyN(&buffer, file, limit)
	if err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "resume_read_failed", "could not read resume upload")
		return resume.CreateInput{}, false
	}
	if written > maxResumeUploadBytes {
		writeError(w, http.StatusBadRequest, "resume_too_large", "resume PDF must be at most 8 MB")
		return resume.CreateInput{}, false
	}
	return resume.CreateInput{
		Filename:    header.Filename,
		ContentType: contentType,
		Data:        buffer.Bytes(),
	}, true
}
