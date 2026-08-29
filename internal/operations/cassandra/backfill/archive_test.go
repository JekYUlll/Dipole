package cassandrabackfill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestMessageArchivePreservesCompleteMessageAndDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "snapshot.json")
	expires := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	source := &sourceStub{highWatermark: 7, messages: []SourceMessage{{SourceID: 7, Message: model.Message{
		ID: 7, UUID: "M-7", ClientMessageID: "C-7", ConversationKey: "direct:U-1:U-2", Seq: 9,
		SenderUUID: "U-1", TargetType: model.MessageTargetDirect, TargetUUID: "U-2",
		MessageType: model.MessageTypeFile, Content: strings.Repeat("payload", 20_000), FileID: "F-7", FileName: "report.pdf",
		FileSize: 42, FileURL: "s3://bucket/F-7", FileContentType: "application/pdf", FileExpiresAt: &expires,
		SentAt: time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC),
	}}}}

	manifest, err := CreateArchive(context.Background(), source, manifestPath, "snapshot-7", 1)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if manifest.EntryCount != 1 || manifest.HighWatermarkID != 7 || len(manifest.EntriesSHA256) != 64 {
		t.Fatalf("manifest = %+v", manifest)
	}
	archive, err := OpenArchive(manifestPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	descriptor, err := archive.Descriptor(context.Background(), 7)
	if err != nil || descriptor.Kind != SourceKindMessageArchive || descriptor.SnapshotID != "snapshot-7" || descriptor.SHA256 != manifest.EntriesSHA256 {
		t.Fatalf("descriptor = %+v err=%v", descriptor, err)
	}
	page, err := archive.ListAfter(context.Background(), 0, 7, 10)
	if err != nil || len(page) != 1 {
		t.Fatalf("page = %+v err=%v", page, err)
	}
	got := page[0].Message
	if got.ClientMessageID != "C-7" || got.ConversationKey != "direct:U-1:U-2" || len(got.Content) != 140_000 || got.FileName != "report.pdf" || got.FileExpiresAt == nil || !got.FileExpiresAt.Equal(expires) {
		t.Fatalf("archive lost complete message fields: %+v", got)
	}

	dataPath := filepath.Join(dir, manifest.DataFile)
	file, err := os.OpenFile(dataPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("tampered"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenArchive(manifestPath); err == nil {
		t.Fatal("expected tampered archive to fail integrity verification")
	}
}

func TestMessageArchiveRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "snapshot.json")
	source := &sourceStub{highWatermark: 1, messages: []SourceMessage{message(1)}}
	if _, err := CreateArchive(context.Background(), source, manifestPath, "snapshot-1", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateArchive(context.Background(), source, manifestPath, "snapshot-1", 1); err == nil {
		t.Fatal("expected immutable archive overwrite to fail")
	}
}
