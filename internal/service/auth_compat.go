package service

import (
	"github.com/JekYUlll/Dipole/internal/model"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

var (
	ErrUserAlreadyExists  = coreauth.ErrUserAlreadyExists
	ErrInvalidCredentials = coreauth.ErrInvalidCredentials
	ErrUserDisabled       = coreauth.ErrUserDisabled
	ErrInvalidTelephone   = coreauth.ErrInvalidTelephone
)

type RegisterInput = coreauth.RegisterInput
type LoginInput = coreauth.LoginInput
type AuthResult = coreauth.AuthResult
type AgentMCPGrantInput = coreauth.AgentMCPGrantInput
type AgentMCPGrantResult = coreauth.AgentMCPGrantResult
type AuthService = coreauth.AuthService

func NewAuthService(repo interface {
	Create(*model.User) error
	GetByTelephone(string) (*model.User, error)
}, tokenService interface {
	Issue(*model.User) (string, error)
	IssueAgentMCPAccessToken(string, string, []string, bool) (string, error)
	Revoke(string) error
}) *AuthService {
	return coreauth.NewAuthService(repo, tokenService)
}
