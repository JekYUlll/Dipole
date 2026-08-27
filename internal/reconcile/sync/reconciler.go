package syncreconcile

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	syncbackfill "github.com/JekYUlll/Dipole/internal/backfill/sync"
	"github.com/JekYUlll/Dipole/internal/model"
)

type Source interface {
	ListAfter(context.Context, uint64, uint64, int) ([]syncbackfill.SourceItem, error)
}

type Snapshot interface {
	CompletedHighWatermark(context.Context, string) (uint64, error)
}

type Target interface {
	ListByMessageUUID(context.Context, string) ([]model.SyncInboxLocator, error)
}

type Config struct {
	JobName     string
	BatchSize   int
	MaxExamples int
}

type Mismatch struct {
	Type        string `json:"type"`
	MessageUUID string `json:"message_uuid"`
	UserUUID    string `json:"user_uuid"`
	Expected    string `json:"expected,omitempty"`
	Actual      string `json:"actual,omitempty"`
}

type Report struct {
	JobName           string     `json:"job_name"`
	HighWatermarkID   uint64     `json:"high_watermark_id"`
	Events            uint64     `json:"events"`
	ExpectedRows      uint64     `json:"expected_rows"`
	ActualRows        uint64     `json:"actual_rows"`
	MissingRows       uint64     `json:"missing_rows"`
	ExtraRows         uint64     `json:"extra_rows"`
	LocatorMismatches uint64     `json:"locator_mismatches"`
	Consistent        bool       `json:"consistent"`
	Examples          []Mismatch `json:"examples,omitempty"`
}

type Reconciler struct {
	source   Source
	snapshot Snapshot
	target   Target
	config   Config
}

func NewReconciler(source Source, snapshot Snapshot, target Target, cfg Config) (*Reconciler, error) {
	switch {
	case source == nil:
		return nil, errors.New("Sync reconciliation source is required")
	case snapshot == nil:
		return nil, errors.New("Sync reconciliation snapshot is required")
	case target == nil:
		return nil, errors.New("Sync reconciliation target is required")
	case strings.TrimSpace(cfg.JobName) == "":
		return nil, errors.New("Sync reconciliation job name is required")
	case cfg.BatchSize <= 0 || cfg.BatchSize > syncbackfill.MaxBatchSize:
		return nil, fmt.Errorf("Sync reconciliation batch size must be within 1..%d", syncbackfill.MaxBatchSize)
	case cfg.MaxExamples < 0:
		return nil, errors.New("Sync reconciliation max examples cannot be negative")
	}
	return &Reconciler{source: source, snapshot: snapshot, target: target, config: cfg}, nil
}

func (r *Reconciler) Run(ctx context.Context) (Report, error) {
	highWatermark, err := r.snapshot.CompletedHighWatermark(ctx, r.config.JobName)
	if err != nil {
		return Report{}, fmt.Errorf("read completed Sync replay snapshot: %w", err)
	}
	report := Report{JobName: r.config.JobName, HighWatermarkID: highWatermark, Consistent: true}
	lastID := uint64(0)
	for lastID < highWatermark {
		items, listErr := r.source.ListAfter(ctx, lastID, highWatermark, r.config.BatchSize)
		if listErr != nil {
			return report, fmt.Errorf("list Sync reconciliation source: %w", listErr)
		}
		if len(items) == 0 {
			return report, fmt.Errorf("Sync reconciliation source ended before high watermark %d", highWatermark)
		}
		for _, item := range items {
			if item.SourceID <= lastID || item.SourceID > highWatermark || item.Projection == nil {
				return report, fmt.Errorf("invalid Sync reconciliation source item %d after %d", item.SourceID, lastID)
			}
			actual, targetErr := r.target.ListByMessageUUID(ctx, item.Projection.MessageUUID)
			if targetErr != nil {
				return report, fmt.Errorf("list Sync target for %s: %w", item.Projection.MessageUUID, targetErr)
			}
			r.compare(&report, item, actual)
			report.Events++
			lastID = item.SourceID
		}
	}
	report.Consistent = report.MissingRows == 0 && report.ExtraRows == 0 && report.LocatorMismatches == 0
	return report, nil
}

func (r *Reconciler) compare(report *Report, item syncbackfill.SourceItem, actual []model.SyncInboxLocator) {
	expected := make(map[string]model.SyncInboxLocator)
	if item.Fanout {
		for _, userUUID := range normalizedRecipients(item.Projection.RecipientUUIDs) {
			expected[userUUID] = model.SyncInboxLocator{
				UserUUID: userUUID, MessageUUID: item.Projection.MessageUUID,
				ConversationKey: item.Projection.ConversationKey, MessageSeq: item.Projection.MessageSeq,
			}
		}
	}
	report.ExpectedRows += uint64(len(expected))
	report.ActualRows += uint64(len(actual))
	seen := make(map[string]struct{}, len(actual))
	for _, row := range actual {
		seen[row.UserUUID] = struct{}{}
		want, ok := expected[row.UserUUID]
		if !ok {
			report.ExtraRows++
			r.addExample(report, Mismatch{Type: "extra", MessageUUID: item.Projection.MessageUUID, UserUUID: row.UserUUID, Actual: locatorString(row)})
			continue
		}
		if want.MessageUUID != row.MessageUUID || want.ConversationKey != row.ConversationKey || want.MessageSeq != row.MessageSeq {
			report.LocatorMismatches++
			r.addExample(report, Mismatch{Type: "locator_mismatch", MessageUUID: item.Projection.MessageUUID, UserUUID: row.UserUUID, Expected: locatorString(want), Actual: locatorString(row)})
		}
	}
	for userUUID, want := range expected {
		if _, ok := seen[userUUID]; ok {
			continue
		}
		report.MissingRows++
		r.addExample(report, Mismatch{Type: "missing", MessageUUID: item.Projection.MessageUUID, UserUUID: userUUID, Expected: locatorString(want)})
	}
}

func (r *Reconciler) addExample(report *Report, mismatch Mismatch) {
	if len(report.Examples) < r.config.MaxExamples {
		report.Examples = append(report.Examples, mismatch)
	}
}

func normalizedRecipients(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func locatorString(locator model.SyncInboxLocator) string {
	return fmt.Sprintf("%s@%s#%d", locator.MessageUUID, locator.ConversationKey, locator.MessageSeq)
}
