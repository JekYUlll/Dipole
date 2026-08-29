package searchbackfill

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublishAndRestoreArchiveUsesPinnedObjectVersions(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "snapshot.json")
	source := &sourceStub{highWatermark: 3, items: []SourceMutation{{SourceID: 3, Mutation: mutation("M1", 2)}}}
	manifest, err := CreateArchive(context.Background(), source, manifestPath, "snapshot-3", 10)
	if err != nil {
		t.Fatal(err)
	}
	store := &versionedObjectStoreStub{objects: map[string][]byte{}}
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	receipt, err := PublishArchive(context.Background(), store, manifestPath, "search", now, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("publish archive: %v", err)
	}
	if receipt.EntriesSHA256 != manifest.EntriesSHA256 || receipt.Manifest.VersionID == "" || receipt.Data.VersionID == "" || len(store.putOrder) != 2 {
		t.Fatalf("unexpected receipt: %+v order=%v", receipt, store.putOrder)
	}
	if store.putOrder[0] != receipt.Data.ObjectKey || store.putOrder[1] != receipt.Manifest.ObjectKey {
		t.Fatalf("expected data before manifest, got %v", store.putOrder)
	}

	restoreDir := filepath.Join(dir, "restore")
	restoredManifest, err := RestoreArchive(context.Background(), store, receipt, restoreDir)
	if err != nil {
		t.Fatalf("restore archive: %v", err)
	}
	if _, err := OpenArchive(restoredManifest); err != nil {
		t.Fatalf("open restored archive: %v", err)
	}
	for _, requested := range store.getOrder {
		if requested.VersionID == "" {
			t.Fatalf("restore read latest object: %+v", requested)
		}
	}

	store.objects[objectVersionKey(receipt.Data)] = []byte("tampered")
	if _, err := RestoreArchive(context.Background(), store, receipt, filepath.Join(dir, "tampered")); err == nil {
		t.Fatal("expected receipt/hash mismatch after tampered version")
	}
}

type versionedObjectStoreStub struct {
	objects  map[string][]byte
	putOrder []string
	getOrder []ArchiveObjectVersion
}

func (s *versionedObjectStoreStub) ValidateRetention(context.Context, time.Duration) error {
	return nil
}

func (s *versionedObjectStoreStub) PutFile(_ context.Context, key, localPath string, _ time.Time) (ArchiveObjectVersion, error) {
	payload, err := os.ReadFile(localPath)
	if err != nil {
		return ArchiveObjectVersion{}, err
	}
	version := ArchiveObjectVersion{ObjectKey: key, VersionID: fmt.Sprintf("v%d", len(s.putOrder)+1), ETag: "etag"}
	s.objects[objectVersionKey(version)] = payload
	s.putOrder = append(s.putOrder, key)
	return version, nil
}

func (s *versionedObjectStoreStub) GetVersion(_ context.Context, version ArchiveObjectVersion, writer io.Writer) error {
	s.getOrder = append(s.getOrder, version)
	payload, ok := s.objects[objectVersionKey(version)]
	if !ok {
		return fmt.Errorf("missing object version")
	}
	_, err := io.Copy(writer, bytes.NewReader(payload))
	return err
}

func objectVersionKey(version ArchiveObjectVersion) string {
	return version.ObjectKey + "@" + version.VersionID
}
