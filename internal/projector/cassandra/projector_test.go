package cassandraprojector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

type stubTimelineAppender struct {
	projection cassandradata.TimelineProjection
	result     cassandradata.AppendResult
	err        error
	calls      int
}

func (s *stubTimelineAppender) Append(_ context.Context, projection cassandradata.TimelineProjection) (cassandradata.AppendResult, error) {
	s.calls++
	s.projection = projection
	return s.result, s.err
}

func TestProjectorMapsCreatedEnvelopeToTimeline(t *testing.T) {
	appender := &stubTimelineAppender{result: cassandradata.AppendResult{Inserted: true}}
	projector, err := New(appender)
	if err != nil {
		t.Fatalf("create projector: %v", err)
	}

	event := createdEvent(t, directCreatedEvent, createdMessagePayload{
		MutationType:    "created",
		Revision:        1,
		MessageID:       "M100",
		ClientMessageID: "C100",
		ConversationKey: "direct:U100:U200",
		MessageSeq:      42,
		SenderUUID:      "U100",
		TargetUUID:      "U200",
		TargetType:      0,
		MessageType:     2,
		Content:         "hello",
		SentAt:          time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC),
	})
	if err := projector.Project(context.Background(), event); err != nil {
		t.Fatalf("project created event: %v", err)
	}
	if appender.calls != 1 {
		t.Fatalf("expected one append, got %d", appender.calls)
	}
	projection := appender.projection
	if projection.EventID != "E100" || projection.EventVersion != "v1" || projection.MessageUUID != "M100" || projection.MessageSeq != 42 {
		t.Fatalf("unexpected projection identity: %+v", projection)
	}
	if projection.ConversationKey != "direct:U100:U200" || projection.Content != "hello" || projection.MessageType != 2 {
		t.Fatalf("unexpected projection payload: %+v", projection)
	}
}

func TestProjectorAcceptsLegacyCreatedDefaults(t *testing.T) {
	appender := &stubTimelineAppender{result: cassandradata.AppendResult{Duplicate: true}}
	projector, _ := New(appender)
	event := createdEvent(t, groupCreatedEvent, createdMessagePayload{
		MessageID:       "M200",
		ConversationKey: "group:G200",
		MessageSeq:      1,
		SenderUUID:      "U100",
		TargetUUID:      "G200",
		TargetType:      1,
		SentAt:          time.Now().UTC(),
	})
	if err := projector.Project(context.Background(), event); err != nil {
		t.Fatalf("project legacy created event: %v", err)
	}
}

func TestProjectorRejectsMutationAndTargetConflicts(t *testing.T) {
	appender := &stubTimelineAppender{}
	projector, _ := New(appender)
	tests := []createdMessagePayload{
		{MutationType: "edited", Revision: 2, TargetType: 0},
		{MutationType: "created", Revision: 1, TargetType: 1},
	}
	for _, payload := range tests {
		payload.MessageID = "M1"
		payload.ConversationKey = "direct:U1:U2"
		payload.MessageSeq = 1
		payload.SentAt = time.Now().UTC()
		if err := projector.Project(context.Background(), createdEvent(t, directCreatedEvent, payload)); err == nil {
			t.Fatalf("expected invalid payload to fail: %+v", payload)
		}
	}
	if appender.calls != 0 {
		t.Fatalf("expected invalid events not to append, got %d calls", appender.calls)
	}
}

func TestProjectorPropagatesTimelineFailure(t *testing.T) {
	expected := errors.New("Cassandra unavailable")
	appender := &stubTimelineAppender{err: expected}
	projector, _ := New(appender)
	payload := createdMessagePayload{
		MessageID:       "M1",
		ConversationKey: "direct:U1:U2",
		MessageSeq:      1,
		TargetType:      0,
		SentAt:          time.Now().UTC(),
	}
	err := projector.Project(context.Background(), createdEvent(t, directCreatedEvent, payload))
	if !errors.Is(err, expected) {
		t.Fatalf("expected timeline error, got %v", err)
	}
}

func createdEvent(t *testing.T, eventType string, payload createdMessagePayload) platformKafka.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal event payload: %v", err)
	}
	return platformKafka.Event{Envelope: &platformKafka.Envelope{
		EventID:   "E100",
		EventType: eventType,
		Version:   "v1",
		Payload:   raw,
	}}
}
