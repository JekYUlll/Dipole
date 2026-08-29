package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/service"
)

type AdminRealtimeStats interface {
	OnlineUserCount() int
	TotalConnectionCount() int
}

// LocalAdminApplication keeps administrative Core use cases behind the service boundary.
type LocalAdminApplication struct {
	*service.AdminService
}

func NewAdminApplication(repository applicationPort.AdminOverviewStore, realtime AdminRealtimeStats) *LocalAdminApplication {
	return &LocalAdminApplication{AdminService: service.NewAdminService(repository, realtime)}
}
