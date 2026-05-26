package apphttp

import (
	"context"
	"errors"
	"fmt"

	"github.com/abhinav-yadav-official/Kleos/internal/auth"
)

type fakeAuthService struct {
	usersByEmail map[string]fakeUser
	accessUsers  map[string]auth.User
	refreshUsers map[string]auth.User
	nextToken    int
}

type fakeUser struct {
	user     auth.User
	password string
}

func newFakeAuthService() *fakeAuthService {
	return &fakeAuthService{
		usersByEmail: map[string]fakeUser{},
		accessUsers:  map[string]auth.User{},
		refreshUsers: map[string]auth.User{},
	}
}

func (s *fakeAuthService) Signup(ctx context.Context, email, password, name string, tosAccepted bool) (AuthResult, error) {
	if !tosAccepted {
		return AuthResult{}, errors.New("terms of service must be accepted")
	}
	if _, exists := s.usersByEmail[email]; exists {
		return AuthResult{}, errors.New("email already exists")
	}
	user := auth.User{ID: fmt.Sprintf("user-%d", len(s.usersByEmail)+1), Email: email, Name: name}
	s.usersByEmail[email] = fakeUser{user: user, password: password}
	return s.issue(user), nil
}

func (s *fakeAuthService) DeleteAccount(ctx context.Context, userID string) error {
	for email, rec := range s.usersByEmail {
		if rec.user.ID == userID {
			delete(s.usersByEmail, email)
			return nil
		}
	}
	return errors.New("not found")
}

func (s *fakeAuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	record, exists := s.usersByEmail[email]
	if !exists || record.password != password {
		return AuthResult{}, errors.New("invalid credentials")
	}
	return s.issue(record.user), nil
}

func (s *fakeAuthService) Refresh(ctx context.Context, refresh string) (AuthResult, error) {
	user, exists := s.refreshUsers[refresh]
	if !exists {
		return AuthResult{}, errors.New("invalid refresh")
	}
	delete(s.refreshUsers, refresh)
	return s.issue(user), nil
}

func (s *fakeAuthService) Logout(ctx context.Context, refresh string) error {
	if _, exists := s.refreshUsers[refresh]; !exists {
		return errors.New("invalid refresh")
	}
	delete(s.refreshUsers, refresh)
	return nil
}

func (s *fakeAuthService) UserFromAccessToken(ctx context.Context, access string) (auth.User, error) {
	user, exists := s.accessUsers[access]
	if !exists {
		return auth.User{}, errors.New("invalid access")
	}
	return user, nil
}

func (s *fakeAuthService) issue(user auth.User) AuthResult {
	s.nextToken++
	access := fmt.Sprintf("access-%d", s.nextToken)
	refresh := fmt.Sprintf("refresh-%d", s.nextToken)
	s.accessUsers[access] = user
	s.refreshUsers[refresh] = user
	return AuthResult{User: user, Access: access, Refresh: refresh}
}
