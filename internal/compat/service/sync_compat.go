package service

import (
	"github.com/JekYUlll/Dipole/internal/application"
	syncdomain "github.com/JekYUlll/Dipole/internal/services/sync/domain"
)

var (
	ErrSyncDeviceIDRequired = syncdomain.ErrSyncDeviceIDRequired
	ErrSyncDeviceIDInvalid  = syncdomain.ErrSyncDeviceIDInvalid
	ErrSyncCheckpointAhead  = syncdomain.ErrSyncCheckpointAhead
	ErrSyncGroupRequired    = syncdomain.ErrSyncGroupRequired
	ErrSyncGroupForbidden   = syncdomain.ErrSyncGroupForbidden
	ErrSyncGroupLimit       = syncdomain.ErrSyncGroupLimit
)

type SyncService = syncdomain.SyncService

func NewSyncService(repo application.SyncStore, authorizers ...syncdomain.SyncGroupAuthorizer) *SyncService {
	return syncdomain.NewSyncService(repo, authorizers...)
}
