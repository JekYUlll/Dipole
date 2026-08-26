package app

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

type coreFileStoreStub struct {
	file *model.UploadedFile
}

func (s coreFileStoreStub) Create(*model.UploadedFile) error { return nil }
func (s coreFileStoreStub) GetByUUID(string) (*model.UploadedFile, error) {
	return s.file, nil
}

func TestLocalCoreCapabilityHidesUnownedFile(t *testing.T) {
	capability := NewLocalCoreCapability(&Repositories{Files: coreFileStoreStub{file: &model.UploadedFile{
		UUID: "F1", UploaderUUID: "U1", FileName: "owned.txt",
	}}})

	file, err := capability.GetOwnedFile("U1", "F1")
	if err != nil || file == nil || file.UUID != "F1" {
		t.Fatalf("expected owned file, got %+v err=%v", file, err)
	}
	file, err = capability.GetOwnedFile("U2", "F1")
	if err != nil || file != nil {
		t.Fatalf("expected unowned file to stay hidden, got %+v err=%v", file, err)
	}
}
