package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	coreauth "github.com/JekYUlll/Dipole/internal/services/core/domain/auth"
)

// LocalAuthApplication keeps authentication use cases behind the Core boundary.
type LocalAuthApplication struct {
	*coreauth.AuthService
}

func NewAuthApplication(users applicationPort.UserStore, tokens *coreauth.TokenService) *LocalAuthApplication {
	return &LocalAuthApplication{AuthService: coreauth.NewAuthService(users, tokens)}
}
