package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type stubCoreCapability struct{}

func (stubCoreCapability) GetUserByUUID(string) (*model.User, error)                 { return nil, nil }
func (stubCoreCapability) CanSendDirectMessage(string, string) (bool, error)         { return false, nil }
func (stubCoreCapability) GetGroupByUUID(string) (*model.Group, error)               { return nil, nil }
func (stubCoreCapability) GetGroupMember(string, string) (*model.GroupMember, error) { return nil, nil }
func (stubCoreCapability) ListGroupMembers(string) ([]*model.GroupMember, error)     { return nil, nil }
func (stubCoreCapability) GetOwnedFile(string, string) (*model.UploadedFile, error)  { return nil, nil }

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

	services := NewMessagingServices(&Repositories{}, MessagingDependencies{})

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

func TestNewMessagingServicesAcceptsRemoteCompatibleCoreCapability(t *testing.T) {
	services := NewMessagingServices(&Repositories{}, MessagingDependencies{Core: stubCoreCapability{}})
	if services == nil || services.Messages == nil {
		t.Fatal("expected messaging services with injected core capability")
	}
}
