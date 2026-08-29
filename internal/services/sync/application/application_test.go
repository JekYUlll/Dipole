package syncapplication

import "testing"

func TestNewBuildsLocalApplication(t *testing.T) {
	if New(nil, nil) == nil {
		t.Fatal("New() returned nil")
	}
}
