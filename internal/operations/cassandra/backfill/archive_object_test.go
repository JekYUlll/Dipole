package cassandrabackfill

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	platformarchive "github.com/JekYUlll/Dipole/internal/platform/archive"
)

func TestPublishAndRestoreMessageArchiveUsesPinnedObjectVersions(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "snapshot.json")
	manifest, err := CreateArchive(context.Background(), &sourceStub{highWatermark: 1, messages: []SourceMessage{message(1)}}, manifestPath, "snapshot-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	store := &archiveObjectStoreStub{objects: map[string][]byte{}}
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	receipt, err := PublishArchive(context.Background(), store, manifestPath, "cassandra", now, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("publish archive: %v", err)
	}
	if receipt.EntriesSHA256 != manifest.EntriesSHA256 || receipt.Manifest.VersionID == "" || receipt.Data.VersionID == "" || len(store.putOrder) != 2 {
		t.Fatalf("receipt = %+v order=%v", receipt, store.putOrder)
	}
	if store.putOrder[0] != receipt.Data.ObjectKey || store.putOrder[1] != receipt.Manifest.ObjectKey {
		t.Fatalf("expected data before manifest, got %v", store.putOrder)
	}
	restoredManifest, err := RestoreArchive(context.Background(), store, receipt, filepath.Join(dir, "restore"))
	if err != nil {
		t.Fatalf("restore archive: %v", err)
	}
	if _, err := OpenArchive(restoredManifest); err != nil {
		t.Fatalf("open restored archive: %v", err)
	}
	for _, version := range store.getOrder {
		if version.VersionID == "" {
			t.Fatalf("restore requested latest object: %+v", version)
		}
	}
	store.objects[archiveObjectVersionKey(receipt.Data)] = []byte("tampered")
	if _, err := RestoreArchive(context.Background(), store, receipt, filepath.Join(dir, "tampered")); err == nil {
		t.Fatal("expected tampered object version to fail")
	}
}

type archiveObjectStoreStub struct {
	objects  map[string][]byte
	putOrder []string
	getOrder []platformarchive.ObjectVersion
}

func (s *archiveObjectStoreStub) ValidateRetention(context.Context, time.Duration) error { return nil }

func (s *archiveObjectStoreStub) PutFile(_ context.Context, key, localPath string, _ time.Time) (platformarchive.ObjectVersion, error) {
	payload, err := os.ReadFile(localPath)
	if err != nil {
		return platformarchive.ObjectVersion{}, err
	}
	version := platformarchive.ObjectVersion{ObjectKey: key, VersionID: fmt.Sprintf("v%d", len(s.putOrder)+1), ETag: "etag"}
	s.objects[archiveObjectVersionKey(version)] = payload
	s.putOrder = append(s.putOrder, key)
	return version, nil
}

func (s *archiveObjectStoreStub) GetVersion(_ context.Context, version platformarchive.ObjectVersion, writer io.Writer) error {
	s.getOrder = append(s.getOrder, version)
	payload, ok := s.objects[archiveObjectVersionKey(version)]
	if !ok {
		return fmt.Errorf("missing object version")
	}
	_, err := io.Copy(writer, bytes.NewReader(payload))
	return err
}

func archiveObjectVersionKey(version platformarchive.ObjectVersion) string {
	return version.ObjectKey + "@" + version.VersionID
}
