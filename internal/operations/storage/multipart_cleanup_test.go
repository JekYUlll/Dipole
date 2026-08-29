package storageops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

type multipartClientStub struct {
	uploads []minio.ObjectMultipartInfo
	aborted []struct{ key, uploadID string }
	failKey string
}

func (s *multipartClientStub) ListIncompleteUploads(context.Context, string, string, bool) <-chan minio.ObjectMultipartInfo {
	ch := make(chan minio.ObjectMultipartInfo, len(s.uploads))
	for _, upload := range s.uploads {
		ch <- upload
	}
	close(ch)
	return ch
}

func (s *multipartClientStub) AbortMultipartUpload(_ context.Context, _ string, key, uploadID string) error {
	if key == s.failKey {
		return errors.New("abort failed")
	}
	s.aborted = append(s.aborted, struct{ key, uploadID string }{key: key, uploadID: uploadID})
	return nil
}

func TestRunMultipartCleanupDryRunSelectsOnlyExpiredUploads(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	client := &multipartClientStub{uploads: []minio.ObjectMultipartInfo{
		{Key: "message-files/old.bin", UploadID: "old", Initiated: now.Add(-2 * time.Hour)},
		{Key: "message-files/new.bin", UploadID: "new", Initiated: now.Add(-10 * time.Minute)},
	}}

	report := RunMultipartCleanup(context.Background(), client, "files", "message-files/", now.Add(-time.Hour), false)
	if report.Scanned != 2 || report.Selected != 1 || len(report.Candidates) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Candidates[0].Status != "eligible" || len(client.aborted) != 0 {
		t.Fatalf("dry-run mutated storage: %+v", report)
	}
}

func TestRunMultipartCleanupExecuteRecordsAbortFailure(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	client := &multipartClientStub{failKey: "message-files/fail.bin", uploads: []minio.ObjectMultipartInfo{
		{Key: "message-files/fail.bin", UploadID: "fail", Initiated: now.Add(-2 * time.Hour)},
		{Key: "message-files/ok.bin", UploadID: "ok", Initiated: now.Add(-3 * time.Hour)},
	}}

	report := RunMultipartCleanup(context.Background(), client, "files", "message-files/", now.Add(-time.Hour), true)
	if report.Aborted != 1 || report.Failed != 1 || report.Selected != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(client.aborted) != 1 || client.aborted[0].key != "message-files/ok.bin" || client.aborted[0].uploadID != "ok" {
		t.Fatalf("unexpected aborts: %v", client.aborted)
	}
}

func TestNormalizePrefix(t *testing.T) {
	if got := NormalizePrefix(" /message-files/ "); got != "message-files/" {
		t.Fatalf("got %q", got)
	}
}
