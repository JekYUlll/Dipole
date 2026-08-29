package service

import (
	"github.com/JekYUlll/Dipole/internal/application"
	coreadmin "github.com/JekYUlll/Dipole/internal/services/core/domain/admin"
)

var ErrAdminRequired = coreadmin.ErrAdminRequired

type AdminOverview = coreadmin.AdminOverview
type AdminService = coreadmin.AdminService

func NewAdminService(repo interface {
	OverviewCounts() (*application.AdminOverviewCounts, error)
}, realtime interface {
	OnlineUserCount() int
	TotalConnectionCount() int
}) *AdminService {
	return coreadmin.NewAdminService(repo, realtime)
}
