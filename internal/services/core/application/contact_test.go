package coreapplication

import "testing"

func TestNewContactApplicationBuildsLocalAdapter(t *testing.T) {
	if NewContactApplication(nil, nil, ContactDependencies{}) == nil {
		t.Fatal("NewContactApplication() returned nil")
	}
}
