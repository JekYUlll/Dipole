package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	searchbackfill "github.com/JekYUlll/Dipole/internal/operations/search/backfill"
)

func TestSearchArchiveStoreEnforcesRetentionAndPinnedReads(t *testing.T) {
	client := &searchArchiveClientStub{payload: []byte("archive")}
	store := newSearchArchiveStore(client, "dipole-search-archives", 30*24*time.Hour)
	if err := store.ValidateRetention(context.Background(), 29*24*time.Hour); err == nil {
		t.Fatal("expected retention below policy to fail")
	}
	if err := store.ValidateRetention(context.Background(), 30*24*time.Hour); err != nil || client.validatedBucket != "dipole-search-archives" {
		t.Fatalf("validate retention: bucket=%s err=%v", client.validatedBucket, err)
	}
	version := searchbackfill.ArchiveObjectVersion{Bucket: "dipole-search-archives", ObjectKey: "search/snapshot/data.ndjson", VersionID: "v1"}
	var output bytes.Buffer
	if err := store.GetVersion(context.Background(), version, &output); err != nil {
		t.Fatalf("get object version: %v", err)
	}
	if output.String() != "archive" || client.readVersion.VersionID != "v1" {
		t.Fatalf("unexpected versioned read: output=%s version=%+v", output.String(), client.readVersion)
	}
	version.Bucket = "other-bucket"
	if err := store.GetVersion(context.Background(), version, &output); err == nil {
		t.Fatal("expected receipt bucket mismatch to fail")
	}
}

type searchArchiveClientStub struct {
	payload         []byte
	validatedBucket string
	readVersion     searchbackfill.ArchiveObjectVersion
}

func (s *searchArchiveClientStub) ValidateBucket(_ context.Context, bucket string) error {
	s.validatedBucket = bucket
	return nil
}
func (s *searchArchiveClientStub) PutFile(_ context.Context, _, objectKey, _ string, _ time.Time) (searchbackfill.ArchiveObjectVersion, error) {
	return searchbackfill.ArchiveObjectVersion{Bucket: "dipole-search-archives", ObjectKey: objectKey, VersionID: "v1"}, nil
}
func (s *searchArchiveClientStub) GetVersion(_ context.Context, _ string, version searchbackfill.ArchiveObjectVersion) (io.ReadCloser, error) {
	s.readVersion = version
	return io.NopCloser(bytes.NewReader(s.payload)), nil
}
