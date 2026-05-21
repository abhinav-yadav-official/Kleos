package apphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func TestResumeRoutesUploadListActivateAndDelete(t *testing.T) {
	auth := newFakeAuthService()
	authResult, err := auth.Signup(context.Background(), "resume@example.com", "password123", "Resume User")
	if err != nil {
		t.Fatalf("signup fake user: %v", err)
	}
	resumes := newFakeResumeService()
	router := NewRouter(Dependencies{Auth: auth, Resumes: resumes})

	upload := postMultipartWithToken(t, router, "/api/resumes", authResult.Access, "resume.pdf", "application/pdf", []byte("%PDF-1.4\nresume text"))
	if upload.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", upload.Code, upload.Body.String())
	}
	if !strings.Contains(upload.Body.String(), "Experienced Go developer") {
		t.Fatalf("upload response should include parsed preview: %s", upload.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/resumes", nil)
	listReq.Header.Set("Authorization", "Bearer "+authResult.Access)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, listReq)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}

	activate := postJSONWithToken(t, router, "/api/resumes/resume-1/activate", authResult.Access, map[string]any{})
	if activate.Code != http.StatusOK {
		t.Fatalf("activate status = %d, body = %s", activate.Code, activate.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/resumes/resume-1", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+authResult.Access)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestResumeUploadRejectsNonPDF(t *testing.T) {
	auth := newFakeAuthService()
	authResult, err := auth.Signup(context.Background(), "bad-resume@example.com", "password123", "Resume User")
	if err != nil {
		t.Fatalf("signup fake user: %v", err)
	}
	router := NewRouter(Dependencies{Auth: auth, Resumes: newFakeResumeService()})

	upload := postMultipartWithToken(t, router, "/api/resumes", authResult.Access, "resume.txt", "text/plain", []byte("plain text"))
	if upload.Code != http.StatusBadRequest {
		t.Fatalf("upload status = %d, body = %s", upload.Code, upload.Body.String())
	}
	var body errorResponse
	if err := json.NewDecoder(upload.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "resume_invalid_type" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func postMultipartWithToken(t *testing.T, handler http.Handler, path, token, filename, contentType string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
