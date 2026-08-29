package bootstrap

import "fmt"

// validateStandaloneCoreMode remains available to compatibility tests and
// callers while the standalone Core runtime lives in its service bootstrap.
func validateStandaloneCoreMode(mode string) error {
	if mode != "remote" {
		return fmt.Errorf("standalone Core service requires gateway.mode=remote; use embedded runtime for local compatibility")
	}
	return nil
}
