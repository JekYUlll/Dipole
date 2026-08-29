package coreapplication

import "testing"

func TestNewFileApplicationBuildsLocalAdapter(t *testing.T) {
	t.Chdir("../../../..")
	t.Setenv("DIPOLE_CONFIG_FILE", "configs/config.dist.yaml")
	if NewFileApplication(nil, nil, nil) == nil {
		t.Fatal("NewFileApplication() returned nil")
	}
}
