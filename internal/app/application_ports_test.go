package app

import applicationPort "github.com/JekYUlll/Dipole/internal/application"

var _ applicationPort.MessageApplication = (*LocalMessageApplication)(nil)
var _ applicationPort.SyncApplication = (*LocalSyncApplication)(nil)
var _ applicationPort.CoreCapability = (*LocalCoreCapability)(nil)
