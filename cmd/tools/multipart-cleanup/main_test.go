package main

import (
	"testing"

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
