package apphttp

import (
	"context"

	"github.com/abhinav-yadav-official/Kleos/internal/preferences"
)

type fakePreferencesService struct {
	records map[string]preferences.Record
}

func newFakePreferencesService() *fakePreferencesService {
	return &fakePreferencesService{records: map[string]preferences.Record{}}
}

func (s *fakePreferencesService) Get(ctx context.Context, userID string) (preferences.Record, error) {
	record, exists := s.records[userID]
	if !exists {
		record = preferences.Default(userID)
		s.records[userID] = record
	}
	return record, nil
}

func (s *fakePreferencesService) Replace(ctx context.Context, userID string, input preferences.Record) (preferences.Record, error) {
	input.UserID = userID
	s.records[userID] = input
	return input, nil
}
