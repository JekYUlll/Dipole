package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/service"
)

// LocalAuthApplication keeps authentication use cases behind the Core boundary.
type LocalAuthApplication struct {
	*service.AuthService
}

func NewAuthApplication(users applicationPort.UserStore, tokens *service.TokenService) *LocalAuthApplication {
	return &LocalAuthApplication{AuthService: service.NewAuthService(users, tokens)}
}
