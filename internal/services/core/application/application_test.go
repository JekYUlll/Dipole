package coreapplication

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type conversationStoreStub struct{}

func (conversationStoreStub) ListSearchConversationKeys(string) ([]string, error) { return nil, nil }

type userStoreStub struct{}

func (userStoreStub) GetByUUID(string) (*model.User, error) { return nil, nil }

type contactStoreStub struct{}

func (contactStoreStub) CanSendDirectMessage(string, string) (bool, error) { return false, nil }

type groupStoreStub struct{}

func (groupStoreStub) GetByUUID(string) (*model.Group, error)               { return nil, nil }
func (groupStoreStub) GetMember(string, string) (*model.GroupMember, error) { return nil, nil }
func (groupStoreStub) ListMembers(string) ([]*model.GroupMember, error)     { return nil, nil }

type coreFileStoreStub struct {
	file *model.UploadedFile
}

func (s coreFileStoreStub) Create(*model.UploadedFile) error { return nil }
func (s coreFileStoreStub) GetByUUID(string) (*model.UploadedFile, error) {
	return s.file, nil
}
func (s coreFileStoreStub) ListByUploaderBeforeID(uploaderUUID string, beforeID uint, limit int) ([]*model.UploadedFile, error) {
	if s.file == nil || s.file.UploaderUUID != uploaderUUID || limit < 1 || beforeID <= s.file.ID {
		return nil, nil
	}
	return []*model.UploadedFile{s.file}, nil
}

func TestLocalCoreCapabilityHidesUnownedFile(t *testing.T) {
	capability := New(Dependencies{
		Users: userStoreStub{}, Contacts: contactStoreStub{}, Groups: groupStoreStub{},
		Conversations: conversationStoreStub{}, Files: coreFileStoreStub{file: &model.UploadedFile{
			UUID: "F1", UploaderUUID: "U1", FileName: "owned.txt",
		}},
	})

	file, err := capability.GetOwnedFile("U1", "F1")
	if err != nil || file == nil || file.UUID != "F1" {
		t.Fatalf("expected owned file, got %+v err=%v", file, err)
	}
	file, err = capability.GetOwnedFile("U2", "F1")
	if err != nil || file != nil {
		t.Fatalf("expected unowned file to stay hidden, got %+v err=%v", file, err)
	}
}

func TestLocalCoreCapabilityListsOwnedFilesWithPublicCursor(t *testing.T) {
	first := &model.UploadedFile{ID: 3, UUID: "F3", UploaderUUID: "U1", FileName: "latest.txt"}
	capability := New(Dependencies{Files: coreFileStoreStub{file: first}})
	page, err := capability.ListOwnedFiles("U1", "", 20)
	if err != nil || len(page.Files) != 1 || page.NextCursor != "F3" || page.HasMore {
		t.Fatalf("unexpected owned file page: %+v err=%v", page, err)
	}
	if _, err := capability.ListOwnedFiles("U2", "F3", 20); err == nil {
		t.Fatal("expected foreign cursor rejection")
	}
}
