package syncapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/service"
)

// LocalApplication is the Sync service's local application adapter.
// Transport clients implement the same application port for remote mode.
type LocalApplication struct {
	*service.SyncService
}

var _ applicationPort.SyncApplication = (*LocalApplication)(nil)

func New(syncStore applicationPort.SyncStore, core applicationPort.CoreCapability) *LocalApplication {
	return &LocalApplication{SyncService: service.NewSyncService(syncStore, core)}
}
