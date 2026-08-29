package coreapplication

import "testing"

func TestNewUserApplicationBuildsLocalAdapter(t *testing.T) {
	if NewUserApplication(nil, UserDependencies{}) == nil {
		t.Fatal("NewUserApplication() returned nil")
	}
}
