package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	syncbaseline "github.com/JekYUlll/Dipole/internal/baseline/sync"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
)

var ErrUnsafeSyncBaselineRestore = errors.New("Sync baseline restore requires a missing-only reconciliation")

type SyncBaselineStore struct {
	store *Store
}

func NewSyncBaselineStore(store *Store) (*SyncBaselineStore, error) {
	if store == nil {
		return nil, errors.New("Sync baseline MySQL store is required")
	}
	return &SyncBaselineStore{store: store}, nil
}

func (s *SyncBaselineStore) Capture(ctx context.Context, jobName string) (syncbaseline.Manifest, error) {
	jobName = strings.TrimSpace(jobName)
	if jobName == "" {
		return syncbaseline.Manifest{}, errors.New("Sync baseline job name is required")
	}
	err := s.store.WithinTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(q *generated.Queries) error {
		if _, err := q.GetSyncInboxBaselineJob(ctx, jobName); err == nil {
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read existing Sync baseline job: %w", err)
		}

		highWatermark, err := q.GetSyncInboxHighWatermark(ctx)
		if err != nil {
			return fmt.Errorf("read Sync Inbox high watermark: %w", err)
		}
		bounds, err := q.GetSyncCreatedOutboxBounds(ctx)
		if err != nil {
			return fmt.Errorf("read created Outbox bounds: %w", err)
		}
		if highWatermark < 0 || bounds.FirstCreatedOutboxID < 0 || bounds.LastCreatedOutboxID < 0 {
			return errors.New("Sync baseline source returned a negative unsigned boundary")
		}
		rows, err := q.ListLegacySyncInboxThrough(ctx, uint64(highWatermark))
		if err != nil {
			return fmt.Errorf("list legacy Sync Inbox snapshot: %w", err)
		}
		entries := make([]syncbaseline.Entry, 0, len(rows))
		for _, row := range rows {
			entries = append(entries, baselineEntry(row.SyncSeq, row.UserUuid, row.MessageUuid, row.ConversationKey, row.MessageSeq))
		}
		digest, entries, err := syncbaseline.Digest(entries)
		if err != nil {
			return fmt.Errorf("digest legacy Sync Inbox snapshot: %w", err)
		}
		if err := q.CreateSyncInboxBaselineJob(ctx, generated.CreateSyncInboxBaselineJobParams{
			JobName: jobName, SourceHighWatermarkSyncSeq: uint64(highWatermark),
			FirstCreatedOutboxID: uint64(bounds.FirstCreatedOutboxID), LastCreatedOutboxID: uint64(bounds.LastCreatedOutboxID),
			EntryCount: uint64(len(entries)), EntriesSha256: digest,
		}); err != nil {
			return fmt.Errorf("create Sync baseline job: %w", err)
		}
		for _, entry := range entries {
			if err := q.CreateSyncInboxBaselineEntry(ctx, generated.CreateSyncInboxBaselineEntryParams{
				JobName: jobName, SyncSeq: entry.SyncSeq, UserUuid: entry.UserUUID,
				MessageUuid: entry.MessageUUID, ConversationKey: entry.ConversationKey, MessageSeq: entry.MessageSeq,
			}); err != nil {
				return fmt.Errorf("archive Sync baseline sequence %d: %w", entry.SyncSeq, err)
			}
		}
		return nil
	})
	if err != nil {
		if IsDuplicateKey(err) {
			manifest, _, loadErr := s.Load(ctx, jobName)
			return manifest, loadErr
		}
		return syncbaseline.Manifest{}, err
	}
	manifest, _, err := s.Load(ctx, jobName)
	return manifest, err
}

func (s *SyncBaselineStore) Load(ctx context.Context, jobName string) (syncbaseline.Manifest, []syncbaseline.Entry, error) {
	jobName = strings.TrimSpace(jobName)
	job, err := s.store.Queries().GetSyncInboxBaselineJob(ctx, jobName)
	if err != nil {
		return syncbaseline.Manifest{}, nil, fmt.Errorf("read Sync baseline job: %w", err)
	}
	rows, err := s.store.Queries().ListSyncInboxBaselineEntries(ctx, jobName)
	if err != nil {
		return syncbaseline.Manifest{}, nil, fmt.Errorf("list Sync baseline entries: %w", err)
	}
	entries := make([]syncbaseline.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, baselineEntry(row.SyncSeq, row.UserUuid, row.MessageUuid, row.ConversationKey, row.MessageSeq))
	}
	digest, entries, err := syncbaseline.Digest(entries)
	if err != nil {
		return syncbaseline.Manifest{}, nil, fmt.Errorf("validate Sync baseline entries: %w", err)
	}
	if uint64(len(entries)) != job.EntryCount || digest != job.EntriesSha256 {
		return syncbaseline.Manifest{}, nil, fmt.Errorf("Sync baseline archive integrity mismatch: rows=%d/%d sha256=%s/%s",
			len(entries), job.EntryCount, digest, job.EntriesSha256)
	}
	manifest := syncbaseline.Manifest{
		JobName: job.JobName, HighWatermarkSyncSeq: job.SourceHighWatermarkSyncSeq,
		FirstCreatedOutboxID: job.FirstCreatedOutboxID, LastCreatedOutboxID: job.LastCreatedOutboxID,
		EntryCount: job.EntryCount, EntriesSHA256: job.EntriesSha256, CapturedAt: job.CapturedAt,
	}
	return manifest, entries, nil
}

func (s *SyncBaselineStore) Reconcile(ctx context.Context, jobName string, maxExamples int) (syncbaseline.Report, error) {
	manifest, expected, err := s.Load(ctx, jobName)
	if err != nil {
		return syncbaseline.Report{}, err
	}
	actual, err := s.listCurrentLegacy(ctx)
	if err != nil {
		return syncbaseline.Report{}, err
	}
	return syncbaseline.Compare(manifest.JobName, manifest.HighWatermarkSyncSeq, expected, actual, maxExamples)
}

func (s *SyncBaselineStore) Restore(ctx context.Context, jobName string, maxExamples int) (syncbaseline.Report, error) {
	manifest, expected, err := s.Load(ctx, jobName)
	if err != nil {
		return syncbaseline.Report{}, err
	}
	actual, err := s.listCurrentLegacy(ctx)
	if err != nil {
		return syncbaseline.Report{}, err
	}
	fullReport, err := syncbaseline.Compare(manifest.JobName, manifest.HighWatermarkSyncSeq, expected, actual, len(expected)+len(actual))
	if err != nil {
		return syncbaseline.Report{}, err
	}
	if fullReport.Extra > 0 || fullReport.Conflicting > 0 {
		return limitedReport(fullReport, maxExamples), ErrUnsafeSyncBaselineRestore
	}
	actualKeys := make(map[string]struct{}, len(actual))
	for _, entry := range actual {
		actualKeys[baselineSemanticKey(entry)] = struct{}{}
	}
	if err := s.store.WithinTx(ctx, nil, func(q *generated.Queries) error {
		for _, entry := range expected {
			if _, exists := actualKeys[baselineSemanticKey(entry)]; exists {
				continue
			}
			if _, err := q.EnsureUserSyncState(ctx, entry.UserUUID); err != nil {
				return fmt.Errorf("ensure Sync state for baseline recipient %s: %w", entry.UserUUID, err)
			}
			if err := q.RestoreSyncInboxBaselineEntry(ctx, generated.RestoreSyncInboxBaselineEntryParams{
				SyncSeq: entry.SyncSeq, UserUuid: entry.UserUUID, MessageUuid: entry.MessageUUID,
				ConversationKey: entry.ConversationKey, MessageSeq: entry.MessageSeq,
			}); err != nil {
				return fmt.Errorf("restore Sync baseline sequence %d: %w", entry.SyncSeq, err)
			}
		}
		return nil
	}); err != nil {
		return limitedReport(fullReport, maxExamples), err
	}
	return s.Reconcile(ctx, jobName, maxExamples)
}

func (s *SyncBaselineStore) listCurrentLegacy(ctx context.Context) ([]syncbaseline.Entry, error) {
	rows, err := s.store.Queries().ListLegacySyncInbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("list current legacy Sync Inbox: %w", err)
	}
	entries := make([]syncbaseline.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, baselineEntry(row.SyncSeq, row.UserUuid, row.MessageUuid, row.ConversationKey, row.MessageSeq))
	}
	return entries, nil
}

func baselineEntry(syncSeq uint64, userUUID, messageUUID, conversationKey string, messageSeq uint64) syncbaseline.Entry {
	return syncbaseline.Entry{
		SyncSeq: syncSeq, UserUUID: userUUID, MessageUUID: messageUUID,
		ConversationKey: conversationKey, MessageSeq: messageSeq,
	}
}

func baselineSemanticKey(entry syncbaseline.Entry) string {
	return entry.UserUUID + "\x00" + entry.MessageUUID
}

func limitedReport(report syncbaseline.Report, maxExamples int) syncbaseline.Report {
	if maxExamples < 0 {
		maxExamples = 0
	}
	if len(report.Examples) > maxExamples {
		report.Examples = append([]syncbaseline.Mismatch(nil), report.Examples[:maxExamples]...)
	}
	return report
}
