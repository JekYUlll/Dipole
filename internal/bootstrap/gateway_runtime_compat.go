package bootstrap

import "github.com/JekYUlll/Dipole/internal/config"

// Exported compatibility helpers support the Gateway-owned runtime while
// shared TLS and validation implementations are being retired.
func ValidateTimelineNotifyMode(cfg config.Message) error { return validateTimelineNotifyMode(cfg) }
func EnsureTLSFiles(cfg config.TLS) error                 { return ensureTLSFiles(cfg) }
