package embedded

import (
	"path/filepath"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type stubCoreCapability struct{}

func (stubCoreCapability) GetUserByUUID(string) (*model.User, error)                 { return nil, nil }
func (stubCoreCapability) CanSendDirectMessage(string, string) (bool, error)         { return false, nil }
func (stubCoreCapability) GetGroupByUUID(string) (*model.Group, error)               { return nil, nil }
func (stubCoreCapability) GetGroupMember(string, string) (*model.GroupMember, error) { return nil, nil }
func (stubCoreCapability) ListGroupMembers(string) ([]*model.GroupMember, error)     { return nil, nil }
func (stubCoreCapability) GetOwnedFile(string, string) (*model.UploadedFile, error)  { return nil, nil }
func (stubCoreCapability) ListOwnedFiles(string, string, int) (*application.OwnedFilePage, error) {
	return &application.OwnedFilePage{}, nil
}
func (stubCoreCapability) ListSearchConversationKeys(string) ([]string, error) { return nil, nil }

func TestNewMessagingServicesBuildsSharedServiceSet(t *testing.T) {
	setMessagingTestConfig(t)

	services := NewMessagingServices(&Repositories{}, MessagingDependencies{})

	if services == nil {
		t.Fatal("NewMessagingServices() returned nil")
	}

	required := map[string]any{
		"core":          services.Core,
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
	setMessagingTestConfig(t)
	services := NewMessagingServices(&Repositories{}, MessagingDependencies{Core: stubCoreCapability{}})
	if services == nil || services.Messages == nil || services.Core == nil {
		t.Fatal("expected messaging services with injected core capability")
	}
	if _, ok := services.Core.(stubCoreCapability); !ok {
		t.Fatalf("injected Core capability was not preserved: %T", services.Core)
	}
}

func setMessagingTestConfig(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	t.Setenv("DIPOLE_CONFIG_FILE", filepath.Join(root, "configs", "config.dist.yaml"))
}
