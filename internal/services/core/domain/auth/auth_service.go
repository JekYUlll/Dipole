package coreauth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/JekYUlll/Dipole/internal/model"
)

var (
	ErrUserAlreadyExists      = errors.New("user already exists")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUserDisabled           = errors.New("user is disabled")
	ErrInvalidTelephone       = errors.New("invalid telephone")
	ErrInvalidCurrentPassword = errors.New("invalid current password")
	ErrPasswordUnchanged      = errors.New("new password must differ from current password")
	ErrInvalidPassword        = errors.New("invalid password")
	telephonePattern          = regexp.MustCompile(`^1[3-9]\d{9}$`)
)

type RegisterInput struct {
	Nickname  string
	Telephone string
	Password  string
	Email     string
}

type LoginInput struct {
	Telephone string
	Password  string
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

type AuthResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

type AgentMCPGrantInput struct {
	UserUUID string
	Resource string
	Scopes   []string
	Consent  bool
}

type AgentMCPGrantResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
	Resource    string
	Scope       string
}

type authRepository interface {
	Create(user *model.User) error
	GetByTelephone(telephone string) (*model.User, error)
	Update(user *model.User) error
}

type tokenIssuer interface {
	Issue(user *model.User) (string, error)
	IssueAgentMCPAccessToken(userUUID, resource string, scopes []string, consent bool) (string, error)
	Revoke(token string) error
}

type AuthService struct {
	repo         authRepository
	tokenService tokenIssuer
}

func NewAuthService(repo authRepository, tokenService tokenIssuer) *AuthService {
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

// ChangePassword verifies the authenticated user's current secret before
// replacing its hash. The initiating session is revoked so the user signs in
// again with the new secret.
func (s *AuthService) ChangePassword(user *model.User, token string, input ChangePasswordInput) error {
	if user == nil || user.Status == model.UserStatusDisabled {
		return ErrUserDisabled
	}
	// Profile-cache entries intentionally exclude PasswordHash. Resolve the
	// credential record through the authoritative telephone lookup instead.
	storedUser, err := s.repo.GetByTelephone(user.Telephone)
	if err != nil {
		return fmt.Errorf("resolve password change user: %w", err)
	}
	if storedUser == nil || storedUser.UUID != user.UUID {
		return ErrInvalidCurrentPassword
	}
	if storedUser.Status == model.UserStatusDisabled {
		return ErrUserDisabled
	}
	if !validPassword(input.CurrentPassword) || !validPassword(input.NewPassword) {
		return ErrInvalidPassword
	}
	if input.CurrentPassword == input.NewPassword {
		return ErrPasswordUnchanged
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		return ErrInvalidCurrentPassword
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash changed password: %w", err)
	}
	storedUser.PasswordHash = string(passwordHash)
	if err := s.repo.Update(storedUser); err != nil {
		return fmt.Errorf("persist changed password: %w", err)
	}
	if err := s.tokenService.Revoke(token); err != nil {
		return fmt.Errorf("revoke session after password change: %w", err)
	}
	return nil
}

func (s *AuthService) IssueAgentMCPGrant(input AgentMCPGrantInput) (*AgentMCPGrantResult, error) {
	token, err := s.tokenService.IssueAgentMCPAccessToken(input.UserUUID, input.Resource, input.Scopes, input.Consent)
	if err != nil {
		return nil, err
	}
	return &AgentMCPGrantResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(agentMCPTokenTTL.Seconds()),
		Resource:    AgentMCPResourceIdentifier(),
		Scope:       AgentMCPReadScope,
	}, nil
}

func generateUserUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate user uuid: %w", err))
	}

	return "U" + strings.ToUpper(hex.EncodeToString(buf))
}

func validPassword(password string) bool {
	return len(password) >= 6 && len(password) <= 32
}
