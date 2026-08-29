package runtime

import (
	"os"

	"github.com/JekYUlll/Dipole/internal/config"
)

// ValidateTLSFiles checks the certificate and key paths before a service
// starts its TLS listener.
func ValidateTLSFiles(tlsCfg config.TLS) error {
	if _, err := os.Stat(tlsCfg.CertFile); err != nil {
		return err
	}
	if _, err := os.Stat(tlsCfg.KeyFile); err != nil {
		return err
	}
	return nil
}
