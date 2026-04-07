package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/repository"
)

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrInvalidTelephone   = errors.New("invalid telephone")
	telephonePattern      = regexp.MustCompile(`^1[3-9]\d{9}$`)
)

type RegisterInput struct {
	Nickname  string `json:"nickname" binding:"required,min=2,max=20"`
	Telephone string `json:"telephone" binding:"required,len=11"`
	Password  string `json:"password" binding:"required,min=6,max=32"`
	Email     string `json:"email" binding:"omitempty,email,max=64"`
}

type LoginInput struct {
	Telephone string `json:"telephone" binding:"required,len=11"`
	Password  string `json:"password" binding:"required,min=6,max=32"`
}

type AuthResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

type AuthService struct {
	repo         *repository.UserRepository
	tokenService *TokenService
}

func NewAuthService(repo *repository.UserRepository, tokenService *TokenService) *AuthService {
	return &AuthService{
		repo:         repo,
		tokenService: tokenService,
	}
}

func (s *AuthService) Register(input RegisterInput) (*AuthResult, error) {
	telephone := strings.TrimSpace(input.Telephone)
	if !telephonePattern.MatchString(telephone) {
		return nil, ErrInvalidTelephone
	}

	existingUser, err := s.repo.GetByTelephone(telephone)
	if err != nil {
		return nil, fmt.Errorf("check telephone exists: %w", err)
	}
	if existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		UUID:         generateUserUUID(),
		Nickname:     strings.TrimSpace(input.Nickname),
		Telephone:    telephone,
		Email:        strings.ToLower(strings.TrimSpace(input.Email)),
		Avatar:       model.DefaultAvatarURL,
		PasswordHash: string(passwordHash),
		Status:       model.UserStatusNormal,
		IsAdmin:      false,
	}

	if err := s.repo.Create(user); err != nil {
		return nil, fmt.Errorf("register user: %w", err)
	}

	token, err := s.tokenService.Issue(user)
	if err != nil {
		return nil, fmt.Errorf("issue token after register: %w", err)
	}

	return &AuthResult{
		Token: token,
		User:  user,
	}, nil
}

func (s *AuthService) Login(input LoginInput) (*AuthResult, error) {
	user, err := s.repo.GetByTelephone(strings.TrimSpace(input.Telephone))
	if err != nil {
		return nil, fmt.Errorf("get user by telephone in login: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if user.Status == model.UserStatusDisabled {
		return nil, ErrUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenService.Issue(user)
	if err != nil {
		return nil, fmt.Errorf("issue token after login: %w", err)
	}

	return &AuthResult{
		Token: token,
		User:  user,
	}, nil
}

func (s *AuthService) Logout(token string) error {
	if err := s.tokenService.Revoke(token); err != nil {
		return fmt.Errorf("logout: %w", err)
	}

	return nil
}

func generateUserUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate user uuid: %w", err))
	}

	return "U" + strings.ToUpper(hex.EncodeToString(buf))
}
