package app

import applicationPort "github.com/JekYUlll/Dipole/internal/application"
import messageapplication "github.com/JekYUlll/Dipole/internal/services/message/application"

var _ applicationPort.MessageApplication = (*messageapplication.LocalApplication)(nil)
var _ applicationPort.CoreCapability = (*LocalCoreCapability)(nil)
