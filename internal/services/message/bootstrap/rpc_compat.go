package bootstrap

import (
	"github.com/JekYUlll/Dipole/internal/application"
	legacybootstrap "github.com/JekYUlll/Dipole/internal/bootstrap"
	"github.com/JekYUlll/Dipole/internal/config"
)

// Message still borrows the shared RPC transport through this narrow seam.
type InternalRPCServer = legacybootstrap.InternalRPCServer

func NewMessageRPCServer(cfg config.InternalRPC, messages application.MessageApplication) (*InternalRPCServer, error) {
	return legacybootstrap.NewMessageRPCServer(cfg, messages)
}
