package service

import (
	"github.com/JekYUlll/Dipole/internal/model"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

var (
	ErrInvalidToken         = coreauth.ErrInvalidToken
	ErrInvalidAgentMCPGrant = coreauth.ErrInvalidAgentMCPGrant
)

const (
	AgentMCPResource  = coreauth.AgentMCPResource
	AgentMCPReadScope = coreauth.AgentMCPReadScope
)

type TokenSession = coreauth.TokenSession
type TokenService = coreauth.TokenService

func NewTokenService() *TokenService {
	return coreauth.NewTokenService()
}

func AgentMCPResourceIdentifier() string {
	return coreauth.AgentMCPResourceIdentifier()
}

func ValidateAgentMCPResource(resource string) error {
	return coreauth.ValidateAgentMCPResource(resource)
}

var _ interface {
	Issue(*model.User) (string, error)
} = (*TokenService)(nil)
