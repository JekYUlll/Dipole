package syncprojector

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

type projectionStoreStub struct {
	items []*model.SyncProjection
}

func (s *projectionStoreStub) Apply(item *model.SyncProjection) error {
	s.items = append(s.items, item)
	return nil
}

func TestProjectorMapsCreatedMessageAndRecipients(t *testing.T) {
	store := &projectionStoreStub{}
	projector, err := New(store)
	if err != nil {
		t.Fatalf("new projector: %v", err)
	}
	payload := map[string]any{
		"message_id": "M1", "conversation_key": "direct:U1:U2", "message_seq": 7,
		"sender_uuid": "U1", "target_uuid": "U2", "target_type": 0,
		"recipient_uuids": []string{"U2", "U1", "U2"}, "sync_fanout": true,
	}
	if err := projector.Project(context.Background(), syncEvent(t, "message.direct.created", payload)); err != nil {
		t.Fatalf("project event: %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("projected items = %d, want 1", len(store.items))
	}
	item := store.items[0]
	if item.MessageUUID != "M1" || item.MessageSeq != 7 || item.ConversationKey != "direct:U1:U2" || len(item.RecipientUUIDs) != 3 {
		t.Fatalf("unexpected projection: %+v", item)
	}
}

func TestProjectorSkipsHotGroupFanout(t *testing.T) {
	store := &projectionStoreStub{}
	projector, _ := New(store)
	if err := projector.Project(context.Background(), syncEvent(t, "message.group.created", map[string]any{
		"message_id": "M2", "conversation_key": "group:G1", "message_seq": 8,
		"sender_uuid": "U1", "target_uuid": "G1", "target_type": 1,
		"recipient_uuids": []string{"U1", "U2"}, "sync_fanout": false,
	})); err != nil {
		t.Fatalf("project hot group event: %v", err)
	}
	if len(store.items) != 0 {
		t.Fatalf("hot group created %d Inbox projections", len(store.items))
	}
}

func TestProjectorRecoversLegacyDirectRecipients(t *testing.T) {
	store := &projectionStoreStub{}
	projector, _ := New(store)
	if err := projector.Project(context.Background(), syncEvent(t, "message.direct.created", map[string]any{
		"message_id": "M3", "conversation_key": "direct:U1:U2", "message_seq": 9,
		"sender_uuid": "U1", "target_uuid": "U2", "target_type": 0,
	})); err != nil {
		t.Fatalf("project legacy direct event: %v", err)
	}
	if got := store.items[0].RecipientUUIDs; len(got) != 2 || got[0] != "U1" || got[1] != "U2" {
		t.Fatalf("legacy recipients = %v", got)
	}
}

func TestProjectorRejectsLegacyGroupWithoutRecipients(t *testing.T) {
	store := &projectionStoreStub{}
	projector, _ := New(store)
	err := projector.Project(context.Background(), syncEvent(t, "message.group.created", map[string]any{
		"message_id": "M4", "conversation_key": "group:G1", "message_seq": 10,
		"sender_uuid": "U1", "target_uuid": "G1", "target_type": 1,
	}))
	if err == nil {
		t.Fatal("expected group event without recipient snapshot to fail")
	}
}

func syncEvent(t *testing.T, eventType string, payload any) platformkafka.Event {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return platformkafka.Event{Envelope: &platformkafka.Envelope{EventID: "E1", EventType: eventType, Version: "v1", Payload: encoded}}
}
