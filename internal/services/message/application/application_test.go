package messageapplication

import "testing"

func TestNewBuildsLocalApplication(t *testing.T) {
	if New(nil, nil, Dependencies{}) == nil {
		t.Fatal("New() returned nil")
	}
}
