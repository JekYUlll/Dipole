package syncbaseline

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Entry struct {
	SyncSeq         uint64 `json:"sync_seq"`
	UserUUID        string `json:"user_uuid"`
	MessageUUID     string `json:"message_uuid"`
	ConversationKey string `json:"conversation_key"`
	MessageSeq      uint64 `json:"message_seq"`
}

type Manifest struct {
	JobName              string    `json:"job_name"`
	HighWatermarkSyncSeq uint64    `json:"high_watermark_sync_seq"`
	FirstCreatedOutboxID uint64    `json:"first_created_outbox_id"`
	LastCreatedOutboxID  uint64    `json:"last_created_outbox_id"`
	EntryCount           uint64    `json:"entry_count"`
	EntriesSHA256        string    `json:"entries_sha256"`
	CapturedAt           time.Time `json:"captured_at"`
}

type Mismatch struct {
	Kind     string `json:"kind"`
	Expected *Entry `json:"expected,omitempty"`
	Actual   *Entry `json:"actual,omitempty"`
}

type Report struct {
	JobName          string     `json:"job_name"`
	HighWatermarkSeq uint64     `json:"high_watermark_sync_seq"`
	ExpectedRows     int        `json:"expected_rows"`
	ActualRows       int        `json:"actual_rows"`
	Missing          int        `json:"missing"`
	Extra            int        `json:"extra"`
	Conflicting      int        `json:"conflicting"`
	Consistent       bool       `json:"consistent"`
	Examples         []Mismatch `json:"examples,omitempty"`
}

func Digest(entries []Entry) (string, []Entry, error) {
	canonical, err := canonicalEntries(entries)
	if err != nil {
		return "", nil, err
	}
	hash := sha256.New()
	for _, entry := range canonical {
		_, _ = fmt.Fprintf(hash, "%d\x00%d:%s\x00%d:%s\x00%d:%s\x00%d\n",
			entry.SyncSeq,
			len(entry.UserUUID), entry.UserUUID,
			len(entry.MessageUUID), entry.MessageUUID,
			len(entry.ConversationKey), entry.ConversationKey,
			entry.MessageSeq,
		)
	}
	return hex.EncodeToString(hash.Sum(nil)), canonical, nil
}

func Compare(jobName string, highWatermark uint64, expected, actual []Entry, maxExamples int) (Report, error) {
	jobName = strings.TrimSpace(jobName)
	if jobName == "" {
		return Report{}, errors.New("Sync baseline job name is required")
	}
	if maxExamples < 0 {
		return Report{}, errors.New("Sync baseline max examples cannot be negative")
	}
	expectedByKey, expectedCanonical, err := indexEntries(expected, highWatermark, true)
	if err != nil {
		return Report{}, fmt.Errorf("validate expected Sync baseline: %w", err)
	}
	actualByKey, actualCanonical, err := indexEntries(actual, highWatermark, false)
	if err != nil {
		return Report{}, fmt.Errorf("validate actual Sync baseline: %w", err)
	}
	report := Report{
		JobName: jobName, HighWatermarkSeq: highWatermark,
		ExpectedRows: len(expectedCanonical), ActualRows: len(actualCanonical), Consistent: true,
	}
	for _, expectedEntry := range expectedCanonical {
		actualEntry, ok := actualByKey[semanticKey(expectedEntry)]
		if !ok {
			report.Missing++
			report.addExample(maxExamples, Mismatch{Kind: "missing", Expected: entryPointer(expectedEntry)})
			continue
		}
		if actualEntry != expectedEntry {
			report.Conflicting++
			report.addExample(maxExamples, Mismatch{
				Kind: "conflicting", Expected: entryPointer(expectedEntry), Actual: entryPointer(actualEntry),
			})
		}
	}
	for _, actualEntry := range actualCanonical {
		if _, ok := expectedByKey[semanticKey(actualEntry)]; ok {
			continue
		}
		report.Extra++
		report.addExample(maxExamples, Mismatch{Kind: "extra", Actual: entryPointer(actualEntry)})
	}
	report.Consistent = report.Missing == 0 && report.Extra == 0 && report.Conflicting == 0
	return report, nil
}

func (r *Report) addExample(limit int, mismatch Mismatch) {
	if len(r.Examples) < limit {
		r.Examples = append(r.Examples, mismatch)
	}
}

func indexEntries(entries []Entry, highWatermark uint64, enforceHighWatermark bool) (map[string]Entry, []Entry, error) {
	_, canonical, err := Digest(entries)
	if err != nil {
		return nil, nil, err
	}
	indexed := make(map[string]Entry, len(canonical))
	for _, entry := range canonical {
		if enforceHighWatermark && entry.SyncSeq > highWatermark {
			return nil, nil, fmt.Errorf("sync sequence %d exceeds high watermark %d", entry.SyncSeq, highWatermark)
		}
		key := semanticKey(entry)
		if _, exists := indexed[key]; exists {
			return nil, nil, fmt.Errorf("duplicate recipient/message key %q", key)
		}
		indexed[key] = entry
	}
	return indexed, canonical, nil
}

func canonicalEntries(entries []Entry) ([]Entry, error) {
	canonical := append([]Entry(nil), entries...)
	for index := range canonical {
		entry := &canonical[index]
		entry.UserUUID = strings.TrimSpace(entry.UserUUID)
		entry.MessageUUID = strings.TrimSpace(entry.MessageUUID)
		entry.ConversationKey = strings.TrimSpace(entry.ConversationKey)
		if entry.SyncSeq == 0 || entry.MessageSeq == 0 || entry.UserUUID == "" || entry.MessageUUID == "" || entry.ConversationKey == "" {
			return nil, fmt.Errorf("invalid Sync baseline entry: %+v", *entry)
		}
	}
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].SyncSeq != canonical[j].SyncSeq {
			return canonical[i].SyncSeq < canonical[j].SyncSeq
		}
		return semanticKey(canonical[i]) < semanticKey(canonical[j])
	})
	for index := 1; index < len(canonical); index++ {
		if canonical[index-1].SyncSeq == canonical[index].SyncSeq {
			return nil, fmt.Errorf("duplicate Sync baseline sequence %d", canonical[index].SyncSeq)
		}
	}
	return canonical, nil
}

func semanticKey(entry Entry) string {
	return entry.UserUUID + "\x00" + entry.MessageUUID
}

func entryPointer(entry Entry) *Entry {
	copy := entry
	return &copy
}
