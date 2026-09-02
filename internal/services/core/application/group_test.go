package coreapplication

import "testing"

func TestNewGroupApplicationBuildsLocalAdapter(t *testing.T) {
	if NewGroupApplication(nil, nil, GroupDependencies{}) == nil {
		t.Fatal("NewGroupApplication() returned nil")
	}
}
