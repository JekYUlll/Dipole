package searchbackfill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveSourceVerifiesAndPagesImmutableSnapshot(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "snapshot.ndjson")
	items := []SourceMutation{{SourceID: 2, Mutation: mutation("M2", 1)}, {SourceID: 3, Mutation: mutation("M1", 2)}}
	data := archiveLines(t, items)
	if err := os.WriteFile(dataPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	manifest := ArchiveManifest{SchemaVersion: ArchiveSchemaV1, SnapshotID: "snapshot-3", HighWatermarkID: 3, EntryCount: 2, EntriesSHA256: hex.EncodeToString(sum[:]), DataFile: filepath.Base(dataPath)}
	manifestPath := filepath.Join(dir, "snapshot.json")
	payload, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	source, err := OpenArchive(manifestPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	descriptor, _ := source.Descriptor(context.Background(), 3)
	if descriptor.Kind != SourceKindEventArchive || descriptor.SHA256 != manifest.EntriesSHA256 {
		t.Fatalf("unexpected descriptor: %+v", descriptor)
	}
	page, err := source.ListAfter(context.Background(), 0, 3, 1)
	if err != nil || len(page) != 1 || page[0].SourceID != 2 {
		t.Fatalf("unexpected archive page: %+v err=%v", page, err)
	}

	if err := os.WriteFile(dataPath, append(data, 'x'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenArchive(manifestPath); err == nil {
		t.Fatal("expected modified archive to fail integrity verification")
	}
}

func TestCreateArchiveStreamsFixedSourceAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "snapshot.json")
	source := &sourceStub{highWatermark: 3, items: []SourceMutation{
		{SourceID: 2, Mutation: mutation("M2", 1)},
		{SourceID: 3, Mutation: mutation("M1", 2)},
	}}
	manifest, err := CreateArchive(context.Background(), source, manifestPath, "snapshot-3", 1)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if manifest.EntryCount != 2 || manifest.HighWatermarkID != 3 || len(manifest.EntriesSHA256) != 64 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	archive, err := OpenArchive(manifestPath)
	if err != nil {
		t.Fatalf("open generated archive: %v", err)
	}
	page, err := archive.ListAfter(context.Background(), 2, 3, 10)
	if err != nil || len(page) != 1 || page[0].Mutation.MessageUUID != "M1" {
		t.Fatalf("unexpected generated page: %+v err=%v", page, err)
	}
	if _, err := CreateArchive(context.Background(), source, manifestPath, "snapshot-3", 1); err == nil {
		t.Fatal("expected immutable archive overwrite to fail")
	}
}

func archiveLines(t *testing.T, items []SourceMutation) []byte {
	t.Helper()
	var result []byte
	for _, item := range items {
		line, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, line...)
		result = append(result, '\n')
	}
	return result
}
