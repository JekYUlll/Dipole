package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storageops "github.com/JekYUlll/Dipole/internal/operations/storage"
)

func TestReconciliationHasDrift(t *testing.T) {
	if reconciliationHasDrift(nil) {
		t.Fatal("nil reconciliation report reported drift")
	}
	if reconciliationHasDrift(&storageops.MultipartReconciliationReport{}) {
		t.Fatal("empty reconciliation report reported drift")
	}
	if !reconciliationHasDrift(&storageops.MultipartReconciliationReport{MissingRedis: 1}) {
		t.Fatal("missing Redis session was not reported as drift")
	}
	if !reconciliationHasDrift(&storageops.MultipartReconciliationReport{MissingMinIO: 1}) {
		t.Fatal("missing MinIO upload was not reported as drift")
	}
}

func TestWriteMultipartReconciliationMetricsPublishesAtomicLowCardinalityGauges(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "multipart.prom")
	report := &storageops.MultipartReconciliationReport{
		RedisKeysScanned: 4, MinIOUploadsSeen: 3, MissingRedis: 1, MissingMinIO: 2, Complete: true,
	}
	if err := writeMultipartReconciliationMetrics(path, report, time.Unix(123, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"dipole_multipart_reconciliation_complete 1",
		"dipole_multipart_reconciliation_drift 1",
		"dipole_multipart_reconciliation_redis_keys_scanned 4",
		"dipole_multipart_reconciliation_minio_uploads_seen 3",
		"dipole_multipart_reconciliation_missing_redis 1",
		"dipole_multipart_reconciliation_missing_minio 2",
		"dipole_multipart_reconciliation_last_run_timestamp_seconds 123",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics output missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "session_id") || strings.Contains(text, "object_key") {
		t.Fatalf("metrics output contains high-cardinality fields: %s", text)
	}
}

func TestWriteMultipartReconciliationMetricsRequiresReport(t *testing.T) {
	if err := writeMultipartReconciliationMetrics(filepath.Join(t.TempDir(), "multipart.prom"), nil, time.Time{}); err == nil {
		t.Fatal("nil report was accepted")
	}
}

func TestWriteMultipartReconciliationMetricsPublishFailureKeepsTargetAndCleansTemp(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "multipart.prom")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	report := &storageops.MultipartReconciliationReport{Complete: true}
	if err := writeMultipartReconciliationMetrics(target, report, time.Unix(456, 0).UTC()); err == nil {
		t.Fatal("publishing over a directory unexpectedly succeeded")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed publish removed target: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("failed publish replaced the target")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".multipart-reconciliation-") {
			t.Fatalf("failed publish leaked temporary file %q", entry.Name())
		}
	}
}
