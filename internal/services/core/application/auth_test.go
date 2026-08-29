package coreapplication

import "testing"

func TestNewAuthApplicationBuildsLocalAdapter(t *testing.T) {
	if NewAuthApplication(nil, nil) == nil {
		t.Fatal("NewAuthApplication() returned nil")
	}
}
