package searchprojector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
)

type stubIndex struct {
	mutation *model.MessageSearchMutation
	err      error
	calls    int
}

func (s *stubIndex) Apply(mutation *model.MessageSearchMutation) error {
	s.calls++
	s.mutation = mutation
	return s.err
}

func (s *stubIndex) Search(model.MessageSearchQuery) ([]*model.MessageSearchDocument, error) {
	return nil, nil
}

func TestProjectorMapsCreatedAndEditedToSearchableMutations(t *testing.T) {
	for _, mutationType := range []messagedomain.MessageMutationType{messagedomain.MessageMutationCreated, messagedomain.MessageMutationEdited} {
		index := &stubIndex{}
		projector, _ := New(index)
		revision := uint64(2)
		if mutationType == messagedomain.MessageMutationCreated {
			revision = 1
		}
		eventType, _ := messagedomain.MessageMutationEventType(model.MessageTargetDirect, mutationType)
		err := projector.Project(context.Background(), event(t, eventType, messagedomain.MessageEventPayload{
			MutationType: mutationType, Revision: revision, ActorUUID: "U1", MessageID: "M1",
			ConversationKey: "direct:U1:U2", MessageSeq: 7, SenderUUID: "U1", TargetUUID: "U2",
			TargetType: model.MessageTargetDirect, Content: "approved", SentAt: time.Now().UTC(),
		}))
		if err != nil || index.calls != 1 || index.mutation.Type != model.MessageSearchMutationUpsert || index.mutation.Revision != revision {
			t.Fatalf("project %s: mutation=%+v calls=%d err=%v", mutationType, index.mutation, index.calls, err)
		}
	}
}

func TestProjectorMapsRecallAndDeleteToTombstones(t *testing.T) {
	for _, mutationType := range []messagedomain.MessageMutationType{messagedomain.MessageMutationRecalled, messagedomain.MessageMutationDeleted} {
		index := &stubIndex{}
		projector, _ := New(index)
		eventType, _ := messagedomain.MessageMutationEventType(model.MessageTargetGroup, mutationType)
		err := projector.Project(context.Background(), event(t, eventType, messagedomain.MessageEventPayload{
			MutationType: mutationType, Revision: 3, ActorUUID: "U1", MessageID: "M1", TargetType: model.MessageTargetGroup,
		}))
		if err != nil || index.mutation.Type != model.MessageSearchMutationTombstone || index.mutation.Document != nil {
			t.Fatalf("project %s tombstone: mutation=%+v err=%v", mutationType, index.mutation, err)
		}
	}
}

func TestProjectorRejectsChannelAndRevisionConflicts(t *testing.T) {
	index := &stubIndex{}
	projector, _ := New(index)
	tests := []messagedomain.MessageEventPayload{
		{MutationType: messagedomain.MessageMutationCreated, Revision: 2, ActorUUID: "U1", MessageID: "M1", TargetType: model.MessageTargetDirect},
		{MutationType: messagedomain.MessageMutationCreated, Revision: 1, ActorUUID: "U1", MessageID: "M1", TargetType: model.MessageTargetGroup},
	}
	for _, payload := range tests {
		if err := projector.Project(context.Background(), event(t, "message.direct.created", payload)); err == nil {
			t.Fatalf("expected invalid payload to fail: %+v", payload)
		}
	}
	if index.calls != 0 {
		t.Fatalf("invalid events reached index %d times", index.calls)
	}
}

func TestProjectorPropagatesIndexFailure(t *testing.T) {
	expected := errors.New("Elasticsearch unavailable")
	index := &stubIndex{err: expected}
	projector, _ := New(index)
	err := projector.Project(context.Background(), event(t, "message.direct.created", messagedomain.MessageEventPayload{
		MutationType: messagedomain.MessageMutationCreated, Revision: 1, ActorUUID: "U1", MessageID: "M1",
		ConversationKey: "direct:U1:U2", MessageSeq: 1, SenderUUID: "U1", TargetType: model.MessageTargetDirect, SentAt: time.Now().UTC(),
	}))
	if !errors.Is(err, expected) {
		t.Fatalf("expected index failure, got %v", err)
	}
}

func event(t *testing.T, eventType string, payload messagedomain.MessageEventPayload) platformKafka.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return platformKafka.Event{Envelope: &platformKafka.Envelope{EventID: "E1", EventType: eventType, Version: "v1", Payload: raw}}
}
