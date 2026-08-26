package cassandrareconcile

import (
	"context"
	"fmt"
	"testing"
	"time"

	cassandrabackfill "github.com/JekYUlll/Dipole/internal/backfill/cassandra"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/model"
)

type sourceStub struct {
	messages []cassandrabackfill.SourceMessage
}

func (s *sourceStub) ListAfter(_ context.Context, afterID, throughID uint64, limit int) ([]cassandrabackfill.SourceMessage, error) {
	result := make([]cassandrabackfill.SourceMessage, 0, limit)
	for _, message := range s.messages {
		if message.SourceID > afterID && message.SourceID <= throughID {
			result = append(result, message)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

type targetStub struct {
	records map[string]cassandradata.TimelineRecord
}

func (s *targetStub) Lookup(_ context.Context, conversationKey string, sequence uint64) (cassandradata.TimelineRecord, bool, error) {
	record, ok := s.records[fmt.Sprintf("%s:%d", conversationKey, sequence)]
	return record, ok, nil
}

func TestReconcilerReportsConsistentSnapshot(t *testing.T) {
	messages := []cassandrabackfill.SourceMessage{
		sourceMessage(1, "direct:U1:U2", 1),
		sourceMessage(2, "group:G1", 1),
		sourceMessage(3, "direct:U1:U2", 2),
	}
	target := targetFor(t, messages)
	reconciler := mustReconciler(t, &sourceStub{messages: messages}, target, Config{
		JobName: "timeline-v1", HighWatermarkID: 3, BatchSize: 2, SampleModulus: 1, MaxExamples: 10,
	})

	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatalf("reconcile snapshot: %v", err)
	}
	if !report.Consistent || report.SourceCount != 3 || report.TargetFoundCount != 3 || report.HashMatchedCount != 3 || report.SampledCount != 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestReconcilerReportsMissingHashSampleAndSequenceMismatch(t *testing.T) {
	messages := []cassandrabackfill.SourceMessage{
		sourceMessage(1, "direct:U1:U2", 1),
		sourceMessage(2, "direct:U1:U2", 3),
		sourceMessage(3, "group:G1", 1),
	}
	target := targetFor(t, messages)
	delete(target.records, "group:G1:1")
	record := target.records["direct:U1:U2:3"]
	record.PayloadHash = "different-hash"
	record.Projection.Content = "different-content"
	target.records["direct:U1:U2:3"] = record
	reconciler := mustReconciler(t, &sourceStub{messages: messages}, target, Config{
		JobName: "timeline-v1", HighWatermarkID: 3, BatchSize: 10, SampleModulus: 1, MaxExamples: 2,
	})

	report, err := reconciler.Run(context.Background())
	if err != nil {
		t.Fatalf("reconcile inconsistent snapshot: %v", err)
	}
	if report.Consistent || report.MissingCount != 1 || report.HashMismatchCount != 1 || report.SampleMismatchCount != 1 || report.SourceSeqGapCount != 1 {
		t.Fatalf("unexpected mismatch report: %+v", report)
	}
	if len(report.Examples) != 2 {
		t.Fatalf("bounded examples=%+v", report.Examples)
	}
}

func TestReconcilerStopsAtFixedHighWatermark(t *testing.T) {
	messages := []cassandrabackfill.SourceMessage{
		sourceMessage(1, "direct:U1:U2", 1),
		sourceMessage(2, "direct:U1:U2", 2),
	}
	target := targetFor(t, messages[:1])
	reconciler := mustReconciler(t, &sourceStub{messages: messages}, target, Config{
		JobName: "timeline-v1", HighWatermarkID: 1, BatchSize: 10, SampleModulus: 100, MaxExamples: 10,
	})

	report, err := reconciler.Run(context.Background())
	if err != nil || !report.Consistent || report.SourceCount != 1 || report.SampledCount != 1 {
		t.Fatalf("fixed snapshot report=%+v err=%v", report, err)
	}
}

func TestReconcilerRejectsOversizedBatch(t *testing.T) {
	_, err := New(&sourceStub{}, &targetStub{}, Config{
		JobName: "timeline-v1", BatchSize: cassandrabackfill.MaxBatchSize + 1, SampleModulus: 100,
	})
	if err == nil {
		t.Fatal("expected oversized reconciliation batch to fail")
	}
}

func sourceMessage(id uint64, conversation string, sequence uint64) cassandrabackfill.SourceMessage {
	return cassandrabackfill.SourceMessage{SourceID: id, Message: model.Message{
		UUID: fmt.Sprintf("M-%d", id), ClientMessageID: fmt.Sprintf("C-%d", id),
		ConversationKey: conversation, Seq: sequence, SenderUUID: "U1",
		TargetType: model.MessageTargetDirect, TargetUUID: "U2", MessageType: model.MessageTypeText,
		Content: fmt.Sprintf("payload-%d", id), SentAt: time.Unix(int64(id), 0).UTC(),
	}}
}

func targetFor(t *testing.T, messages []cassandrabackfill.SourceMessage) *targetStub {
	t.Helper()
	records := make(map[string]cassandradata.TimelineRecord, len(messages))
	for _, message := range messages {
		projection := cassandrabackfill.ProjectionForMessage(message.Message)
		hash, err := projection.PayloadHash()
		if err != nil {
			t.Fatalf("hash target projection: %v", err)
		}
		records[fmt.Sprintf("%s:%d", message.Message.ConversationKey, message.Message.Seq)] = cassandradata.TimelineRecord{Projection: projection, PayloadHash: hash}
	}
	return &targetStub{records: records}
}

func mustReconciler(t *testing.T, source Source, target Target, cfg Config) *Reconciler {
	t.Helper()
	reconciler, err := New(source, target, cfg)
	if err != nil {
		t.Fatalf("new reconciler: %v", err)
	}
	return reconciler
}
