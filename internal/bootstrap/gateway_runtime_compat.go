package bootstrap

import "github.com/JekYUlll/Dipole/internal/config"

// Exported compatibility helper supports embedded callers while Gateway
// validation ownership moves out of the shared bootstrap.
func ValidateTimelineNotifyMode(cfg config.Message) error { return validateTimelineNotifyMode(cfg) }
