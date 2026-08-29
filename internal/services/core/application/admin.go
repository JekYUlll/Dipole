package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	coreadmin "github.com/JekYUlll/Dipole/internal/services/core/domain/admin"
)

type AdminRealtimeStats interface {
	OnlineUserCount() int
	TotalConnectionCount() int
}

// LocalAdminApplication keeps administrative Core use cases behind the service boundary.
type LocalAdminApplication struct {
	*coreadmin.AdminService
}

func NewAdminApplication(repository applicationPort.AdminOverviewStore, realtime AdminRealtimeStats) *LocalAdminApplication {
	return &LocalAdminApplication{AdminService: coreadmin.NewAdminService(repository, realtime)}
}
