package syncapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	syncdomain "github.com/JekYUlll/Dipole/internal/services/sync/domain"
)

// LocalApplication is the Sync service's local application adapter.
// Transport clients implement the same application port for remote mode.
type LocalApplication struct {
	*syncdomain.SyncService
}

var _ applicationPort.SyncApplication = (*LocalApplication)(nil)

func New(syncStore applicationPort.SyncStore, core applicationPort.CoreCapability) *LocalApplication {
	return &LocalApplication{SyncService: syncdomain.NewSyncService(syncStore, core)}
}
