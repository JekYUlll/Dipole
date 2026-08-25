package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMessagingServicesBuildsSharedServiceSet(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(filepath.Join(workingDirectory, "..", "..")); err != nil {
		t.Fatalf("change to repository root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	services := NewMessagingServices(NewRepositories(), MessagingDependencies{})

	if services == nil {
		t.Fatal("NewMessagingServices() returned nil")
	}

	required := map[string]any{
		"files":         services.Files,
		"messages":      services.Messages,
		"conversations": services.Conversations,
		"sync":          services.Sync,
	}
	for name, applicationService := range required {
		if applicationService == nil {
			t.Errorf("service %s is nil", name)
		}
	}
}
