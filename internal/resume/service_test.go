package resume

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceCreateStoresPDFAndParsedText(t *testing.T) {
	storageDir := t.TempDir()
	repo := newFakeRepository()
	service := newServiceWithRepository(repo, storageDir)
	service.newID = func() (string, error) { return "11111111-1111-4111-8111-111111111111", nil }
	service.extractText = func(ctx context.Context, path string) (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read stored PDF: %v", err)
		}
		if string(data) != "%PDF-1.4\nbody" {
			t.Fatalf("stored PDF content = %q", string(data))
		}
		return "Parsed resume text", nil
	}

	record, err := service.Create(context.Background(), "22222222-2222-4222-8222-222222222222", CreateInput{
		Filename:    "../resume.pdf",
		ContentType: "application/pdf",
		Data:        []byte("%PDF-1.4\nbody"),
	})
	if err != nil {
		t.Fatalf("create resume: %v", err)
	}

	wantPath := filepath.Join(storageDir, "22222222-2222-4222-8222-222222222222", "11111111-1111-4111-8111-111111111111.pdf")
	if record.StoragePath != wantPath {
		t.Fatalf("storage path = %q, want %q", record.StoragePath, wantPath)
	}
	if record.Filename != "resume.pdf" {
		t.Fatalf("filename = %q", record.Filename)
	}
	if record.ParsedText != "Parsed resume text" {
		t.Fatalf("parsed text = %q", record.ParsedText)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("stored PDF stat: %v", err)
	}
}

func TestServiceCreateRejectsEmptyParsedTextAndRemovesFile(t *testing.T) {
	storageDir := t.TempDir()
	service := newServiceWithRepository(newFakeRepository(), storageDir)
	service.newID = func() (string, error) { return "11111111-1111-4111-8111-111111111111", nil }
	service.extractText = func(ctx context.Context, path string) (string, error) {
		return " \n\t", nil
	}

	_, err := service.Create(context.Background(), "22222222-2222-4222-8222-222222222222", CreateInput{
		Filename:    "resume.pdf",
		ContentType: "application/pdf",
		Data:        []byte("%PDF-1.4\nbody"),
	})
	if err == nil {
		t.Fatal("expected empty parsed text error")
	}

	path := filepath.Join(storageDir, "22222222-2222-4222-8222-222222222222", "11111111-1111-4111-8111-111111111111.pdf")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("stored PDF should be removed, stat err = %v", statErr)
	}
}

func TestServiceCreateRejectsPathTraversalUserID(t *testing.T) {
	storageDir := t.TempDir()
	service := newServiceWithRepository(newFakeRepository(), storageDir)
	service.newID = func() (string, error) { return "11111111-1111-4111-8111-111111111111", nil }

	_, err := service.Create(context.Background(), "../evil", CreateInput{
		Filename:    "resume.pdf",
		ContentType: "application/pdf",
		Data:        []byte("%PDF-1.4\nbody"),
	})
	if err == nil {
		t.Fatal("expected invalid user ID error")
	}
}

type fakeRepository struct {
	records map[string]Record
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{records: map[string]Record{}}
}

func (r *fakeRepository) insert(ctx context.Context, record Record) (Record, error) {
	r.records[record.ID] = record
	return record, nil
}

func (r *fakeRepository) list(ctx context.Context, userID string) ([]Record, error) {
	records := []Record{}
	for _, record := range r.records {
		if record.UserID == userID {
			records = append(records, record)
		}
	}
	return records, nil
}

func (r *fakeRepository) activate(ctx context.Context, userID, id string) (Record, error) {
	record, ok := r.records[id]
	if !ok || record.UserID != userID {
		return Record{}, os.ErrNotExist
	}
	record.IsActive = true
	r.records[id] = record
	return record, nil
}

func (r *fakeRepository) delete(ctx context.Context, userID, id string) (string, error) {
	record, ok := r.records[id]
	if !ok || record.UserID != userID {
		return "", os.ErrNotExist
	}
	delete(r.records, id)
	return record.StoragePath, nil
}
