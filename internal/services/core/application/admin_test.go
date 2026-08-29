package coreapplication

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestNewAdminApplicationBuildsLocalAdapter(t *testing.T) {
	t.Chdir("../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	if NewAdminApplication(nil, nil) == nil {
		t.Fatal("NewAdminApplication() returned nil")
	}
	_ = config.AppConfig()
}
