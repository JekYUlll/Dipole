package searchops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	searchcleanup "github.com/JekYUlll/Dipole/internal/cleanup/search"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	searchreconcile "github.com/JekYUlll/Dipole/internal/reconcile/search"
)

type SearchCleanupOptions struct {
	ReceiptPath          string
	ReconcileReportPath  string
	TargetIndex          string
	BatchSize            int
	Execute              bool
	MaintenanceConfirmed bool
	Operator             string
}

func RunSearchOutboxCleanup(ctx context.Context, options SearchCleanupOptions) (searchcleanup.Result, error) {
	receipt, err := searchbackfill.ReadArchiveReceipt(options.ReceiptPath)
	if err != nil {
		return searchcleanup.Result{}, fmt.Errorf("read Search cleanup archive receipt: %w", err)
	}
	report, err := readSearchReconcileReport(options.ReconcileReportPath)
	if err != nil {
		return searchcleanup.Result{}, err
	}
	temporary, err := os.MkdirTemp("", "dipole-search-cleanup-verify-")
	if err != nil {
		return searchcleanup.Result{}, err
	}
	defer os.RemoveAll(temporary)
	if _, err := RestoreSearchArchive(ctx, options.ReceiptPath, temporary); err != nil {
		return searchcleanup.Result{}, fmt.Errorf("verify Search cleanup archive versions: %w", err)
	}
	db, err := openSearchMaintenanceMySQL(ctx, "Outbox cleanup")
	if err != nil {
		return searchcleanup.Result{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return searchcleanup.Result{}, err
	}
	checkpoints, err := mysqldata.NewSearchBackfillCheckpointStore(store, options.TargetIndex)
	if err != nil {
		return searchcleanup.Result{}, err
	}
	descriptor := searchbackfill.SourceDescriptor{Kind: searchbackfill.SourceKindEventArchive, SnapshotID: receipt.SnapshotID, SHA256: receipt.EntriesSHA256}
	highWatermark, err := checkpoints.CompletedHighWatermarkForSource(ctx, report.JobName, descriptor)
	if err != nil {
		return searchcleanup.Result{}, err
	}
	if highWatermark != receipt.HighWatermarkID {
		return searchcleanup.Result{}, fmt.Errorf("Search cleanup Job high watermark does not match archive receipt")
	}
	cleanupStore, err := mysqldata.NewSearchOutboxCleanupStore(store)
	if err != nil {
		return searchcleanup.Result{}, err
	}
	cleaner, err := searchcleanup.New(cleanupStore, receipt, report, searchcleanup.Config{
		BatchSize: options.BatchSize, Execute: options.Execute, MaintenanceConfirmed: options.MaintenanceConfirmed, Operator: options.Operator,
	})
	if err != nil {
		return searchcleanup.Result{}, err
	}
	return cleaner.Run(ctx)
}

func readSearchReconcileReport(path string) (searchreconcile.Report, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return searchreconcile.Report{}, fmt.Errorf("read Search cleanup reconciliation report: %w", err)
	}
	var report searchreconcile.Report
	if err := json.Unmarshal(payload, &report); err != nil {
		return searchreconcile.Report{}, fmt.Errorf("decode Search cleanup reconciliation report: %w", err)
	}
	return report, nil
}
