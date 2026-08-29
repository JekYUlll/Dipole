package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

func TestNewGroupEventHandlerFansOutRecipients(t *testing.T) {
	sender := &directReadSender{}
	payload, err := json.Marshal(service.GroupEventPayload{
		GroupUUID: "G1", Name: "group", MemberUUIDs: []string{"U1", "U2"},
		RecipientUUIDs: []string{"U1", "U2"}, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "group.updated", Payload: payload}}
	dataBuilder := func(p service.GroupEventPayload) wsTransport.GroupUpdatedEventData {
		return wsTransport.GroupUpdatedEventData{GroupUUID: p.GroupUUID, Name: p.Name}
	}

	if err := NewGroupEventHandler(sender, wsTransport.TypeGroupUpdated, dataBuilder)(context.Background(), event); err != nil {
		t.Fatalf("handle group event: %v", err)
	}
	if sender.user != "U2" || sender.type_ != wsTransport.TypeGroupUpdated {
		t.Fatalf("expected final recipient delivery, got user=%q type=%q", sender.user, sender.type_)
	}
	data, ok := sender.data.(wsTransport.GroupUpdatedEventData)
	if !ok || data.GroupUUID != "G1" || data.Name != "group" {
		t.Fatalf("unexpected group event data: %#v", sender.data)
	}
}

func TestNewGroupEventHandlerRejectsMalformedEvent(t *testing.T) {
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "group.updated", Payload: []byte(`{"group_uuid":`)}}
	buildData := func(service.GroupEventPayload) wsTransport.GroupUpdatedEventData {
		return wsTransport.GroupUpdatedEventData{}
	}
	if err := NewGroupEventHandler(&directReadSender{}, wsTransport.TypeGroupUpdated, buildData)(context.Background(), event); err == nil {
		t.Fatal("malformed group event must retain the Kafka record")
	}
}
