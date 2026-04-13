package service

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubAuthRepository struct {
	usersByTelephone map[string]*model.User
	createdUser      *model.User
	createErr        error
	getErr           error
}

func (r *stubAuthRepository) Create(user *model.User) error {
	if r.createErr != nil {
		return r.createErr
	}

	r.createdUser = user
	if r.usersByTelephone == nil {
		r.usersByTelephone = make(map[string]*model.User)
	}
	r.usersByTelephone[user.Telephone] = user
	return nil
}

func (r *stubAuthRepository) GetByTelephone(telephone string) (*model.User, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.usersByTelephone == nil {
		return nil, nil
	}

	return r.usersByTelephone[telephone], nil
}

type stubTokenIssuer struct {
	issuedToken string
	issueErr    error
	revoked     string
	revokeErr   error
}

func (s *stubTokenIssuer) Issue(user *model.User) (string, error) {
	if s.issueErr != nil {
		return "", s.issueErr
	}
	if s.issuedToken == "" {
		s.issuedToken = "TOKEN123"
	}
	return s.issuedToken, nil
}

func (s *stubTokenIssuer) Revoke(token string) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revoked = token
	return nil
}

func TestAuthServiceRegisterSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubAuthRepository{}
	tokenIssuer := &stubTokenIssuer{issuedToken: "REGISTER_TOKEN"}
	service := NewAuthService(repo, tokenIssuer)

	result, err := service.Register(RegisterInput{
		Nickname:  "Alice",
		Telephone: "13800138000",
		Password:  "123456",
		Email:     "Alice@Example.com",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Token != "REGISTER_TOKEN" {
		t.Fatalf("expected token REGISTER_TOKEN, got %s", result.Token)
	}
	if repo.createdUser == nil {
		t.Fatal("expected user to be created")
	}
	if repo.createdUser.Email != "alice@example.com" {
		t.Fatalf("expected normalized email, got %s", repo.createdUser.Email)
	}
	if repo.createdUser.Nickname != "Alice" {
		t.Fatalf("expected nickname Alice, got %s", repo.createdUser.Nickname)
	}
	if repo.createdUser.PasswordHash == "123456" || repo.createdUser.PasswordHash == "" {
		t.Fatalf("expected hashed password, got %q", repo.createdUser.PasswordHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.createdUser.PasswordHash), []byte("123456")); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
}

func TestAuthServiceRegisterRejectsDuplicateTelephone(t *testing.T) {
	t.Parallel()

	repo := &stubAuthRepository{
		usersByTelephone: map[string]*model.User{
			"13800138000": {Telephone: "13800138000"},
		},
	}
	service := NewAuthService(repo, &stubTokenIssuer{})

	_, err := service.Register(RegisterInput{
		Nickname:  "Alice",
		Telephone: "13800138000",
		Password:  "123456",
	})
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestAuthServiceLoginSuccess(t *testing.T) {
	t.Parallel()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &stubAuthRepository{
		usersByTelephone: map[string]*model.User{
			"13800138000": {
				UUID:         "U100",
				Telephone:    "13800138000",
				PasswordHash: string(passwordHash),
				Status:       model.UserStatusNormal,
			},
		},
	}
	tokenIssuer := &stubTokenIssuer{issuedToken: "LOGIN_TOKEN"}
	service := NewAuthService(repo, tokenIssuer)

	result, err := service.Login(LoginInput{
		Telephone: "13800138000",
		Password:  "123456",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Token != "LOGIN_TOKEN" {
		t.Fatalf("expected token LOGIN_TOKEN, got %s", result.Token)
	}
	if result.User == nil || result.User.UUID != "U100" {
		t.Fatalf("expected logged in user U100, got %+v", result.User)
	}
}

func TestAuthServiceLoginRejectsDisabledUser(t *testing.T) {
	t.Parallel()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &stubAuthRepository{
		usersByTelephone: map[string]*model.User{
			"13800138000": {
				Telephone:    "13800138000",
				PasswordHash: string(passwordHash),
				Status:       model.UserStatusDisabled,
			},
		},
	}
	service := NewAuthService(repo, &stubTokenIssuer{})

	_, err = service.Login(LoginInput{
		Telephone: "13800138000",
		Password:  "123456",
	})
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}

func TestAuthServiceLogoutRevokesToken(t *testing.T) {
	t.Parallel()

	tokenIssuer := &stubTokenIssuer{}
	service := NewAuthService(&stubAuthRepository{}, tokenIssuer)

	if err := service.Logout("TOKEN123"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tokenIssuer.revoked != "TOKEN123" {
		t.Fatalf("expected revoked token TOKEN123, got %s", tokenIssuer.revoked)
	}
}
