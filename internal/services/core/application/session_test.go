package coreapplication

import "testing"

func TestNewSessionApplicationBuildsLocalAdapter(t *testing.T) {
	if NewSessionApplication(SessionDependencies{}) == nil {
		t.Fatal("NewSessionApplication() returned nil")
	}
}
