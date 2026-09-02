package searchcleanup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	searchbackfill "github.com/JekYUlll/Dipole/internal/operations/search/backfill"
	searchreconcile "github.com/JekYUlll/Dipole/internal/operations/search/reconcile"
)

type Store interface {
	Inspect(context.Context, uint64) (published uint64, nonPublished uint64, err error)
	DeletePublishedBatch(context.Context, uint64, int) (uint64, error)
}

type Config struct {
	BatchSize            int
	Execute              bool
	MaintenanceConfirmed bool
	Operator             string
}

type Result struct {
	JobName           string `json:"job_name"`
	SnapshotID        string `json:"snapshot_id"`
	ManifestVersionID string `json:"manifest_version_id"`
	DataVersionID     string `json:"data_version_id"`
	Operator          string `json:"operator,omitempty"`
	ReconciledAt      string `json:"reconciled_at"`
	HighWatermarkID   uint64 `json:"high_watermark_id"`
	EligibleCount     uint64 `json:"eligible_count"`
	DeletedCount      uint64 `json:"deleted_count"`
	DryRun            bool   `json:"dry_run"`
}

type Cleaner struct {
	store   Store
	receipt searchbackfill.ArchiveReceipt
	report  searchreconcile.Report
	config  Config
}

func New(store Store, receipt searchbackfill.ArchiveReceipt, report searchreconcile.Report, cfg Config) (*Cleaner, error) {
	if store == nil {
		return nil, errors.New("Search Outbox cleanup store is required")
	}
	if cfg.BatchSize <= 0 || cfg.BatchSize > searchbackfill.MaxBatchSize {
		return nil, errors.New("Search Outbox cleanup batch size is invalid")
	}
	if receipt.SchemaVersion != searchbackfill.ArchiveReceiptSchemaV1 || receipt.HighWatermarkID == 0 || strings.TrimSpace(receipt.SnapshotID) == "" || receipt.Manifest.VersionID == "" || receipt.Data.VersionID == "" {
		return nil, errors.New("Search Outbox cleanup archive receipt is invalid")
	}
	if !validReconciliation(report, receipt.HighWatermarkID) {
		return nil, errors.New("Search Outbox cleanup reconciliation report is not a consistent receipt match")
	}
	if cfg.Execute && !cfg.MaintenanceConfirmed {
		return nil, errors.New("Search Outbox cleanup execution requires maintenance confirmation")
	}
	if cfg.Execute && strings.TrimSpace(cfg.Operator) == "" {
		return nil, errors.New("Search Outbox cleanup execution requires an operator")
	}
	cfg.Operator = strings.TrimSpace(cfg.Operator)
	return &Cleaner{store: store, receipt: receipt, report: report, config: cfg}, nil
}

func (c *Cleaner) Run(ctx context.Context) (Result, error) {
	published, nonPublished, err := c.store.Inspect(ctx, c.receipt.HighWatermarkID)
	if err != nil {
		return Result{}, fmt.Errorf("inspect Search Outbox cleanup range: %w", err)
	}
	result := Result{
		JobName: c.report.JobName, SnapshotID: c.receipt.SnapshotID,
		ManifestVersionID: c.receipt.Manifest.VersionID, DataVersionID: c.receipt.Data.VersionID,
		Operator: c.config.Operator, ReconciledAt: c.report.CompletedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		HighWatermarkID: c.receipt.HighWatermarkID, EligibleCount: published, DryRun: !c.config.Execute,
	}
	if nonPublished > 0 {
		return result, fmt.Errorf("Search Outbox cleanup range contains %d non-published mutations", nonPublished)
	}
	if !c.config.Execute {
		return result, nil
	}
	for result.DeletedCount < published {
		deleted, err := c.store.DeletePublishedBatch(ctx, c.receipt.HighWatermarkID, c.config.BatchSize)
		if err != nil {
			return result, fmt.Errorf("delete Search Outbox cleanup batch: %w", err)
		}
		if deleted == 0 {
			return result, errors.New("Search Outbox cleanup stopped before eligible rows were deleted")
		}
		result.DeletedCount += deleted
	}
	return result, nil
}

func validReconciliation(report searchreconcile.Report, highWatermark uint64) bool {
	return strings.TrimSpace(report.JobName) != "" && report.SourceHighWatermark == highWatermark && report.Consistent &&
		!report.CompletedAt.IsZero() && report.SourceCount == report.TargetCount && report.SourceCount == report.TargetFoundCount &&
		report.SourceCount == report.HashMatchedCount && report.MissingCount == 0 && report.HashMismatchCount == 0 && report.ExtraCount == 0
}
