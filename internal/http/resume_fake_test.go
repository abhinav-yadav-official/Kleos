package apphttp

import (
	"context"
	"errors"

	"github.com/abhinav-yadav-official/Kleos/internal/resume"
)

type fakeResumeService struct {
	records map[string]resume.Record
}

func newFakeResumeService() *fakeResumeService {
	return &fakeResumeService{records: map[string]resume.Record{}}
}

func (s *fakeResumeService) Create(ctx context.Context, userID string, input resume.CreateInput) (resume.Record, error) {
	record := resume.Record{
		ID:          "resume-1",
		UserID:      userID,
		Filename:    input.Filename,
		StoragePath: "/data/resumes/user-1/resume-1.pdf",
		ParsedText:  "Experienced Go developer with distributed systems background.",
		IsActive:    true,
	}
	s.records[record.ID] = record
	return record, nil
}

func (s *fakeResumeService) List(ctx context.Context, userID string) ([]resume.Record, error) {
	result := make([]resume.Record, 0, len(s.records))
	for _, record := range s.records {
		if record.UserID == userID {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *fakeResumeService) Activate(ctx context.Context, userID, id string) (resume.Record, error) {
	record, exists := s.records[id]
	if !exists || record.UserID != userID {
		return resume.Record{}, errors.New("not found")
	}
	record.IsActive = true
	s.records[id] = record
	return record, nil
}

func (s *fakeResumeService) Delete(ctx context.Context, userID, id string) error {
	record, exists := s.records[id]
	if !exists || record.UserID != userID {
		return errors.New("not found")
	}
	delete(s.records, id)
	return nil
}
