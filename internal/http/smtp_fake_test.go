package apphttp

import (
	"context"
	"errors"

	"github.com/abhinav-yadav-official/Kleos/internal/smtpcred"
)

type fakeSMTPService struct {
	records map[string]smtpcred.Credential
}

func newFakeSMTPService() *fakeSMTPService {
	return &fakeSMTPService{records: map[string]smtpcred.Credential{}}
}

func (s *fakeSMTPService) Create(ctx context.Context, userID string, input smtpcred.CreateInput) (smtpcred.Credential, error) {
	record := smtpcred.Credential{
		ID:        "smtp-1",
		UserID:    userID,
		Label:     input.Label,
		Host:      input.Host,
		Port:      input.Port,
		Username:  input.Username,
		FromEmail: input.FromEmail,
		FromName:  input.FromName,
		UseTLS:    input.UseTLS,
	}
	s.records[record.ID] = record
	return record, nil
}

func (s *fakeSMTPService) List(ctx context.Context, userID string) ([]smtpcred.Credential, error) {
	result := make([]smtpcred.Credential, 0, len(s.records))
	for _, record := range s.records {
		if record.UserID == userID {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *fakeSMTPService) Verify(ctx context.Context, userID, id string) (smtpcred.VerifyResult, error) {
	if _, exists := s.records[id]; !exists {
		return smtpcred.VerifyResult{}, errors.New("not found")
	}
	return smtpcred.VerifyResult{OK: true, Detail: "ok"}, nil
}

func (s *fakeSMTPService) SetPrimary(ctx context.Context, userID, id string) (smtpcred.Credential, error) {
	record, exists := s.records[id]
	if !exists || record.UserID != userID {
		return smtpcred.Credential{}, errors.New("not found")
	}
	for key, value := range s.records {
		if value.UserID == userID {
			value.IsPrimary = false
			s.records[key] = value
		}
	}
	record.IsPrimary = true
	s.records[id] = record
	return record, nil
}

func (s *fakeSMTPService) Delete(ctx context.Context, userID, id string) error {
	if _, exists := s.records[id]; !exists {
		return errors.New("not found")
	}
	delete(s.records, id)
	return nil
}
