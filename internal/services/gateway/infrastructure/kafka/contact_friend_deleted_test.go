package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

func TestNewContactFriendDeletedHandlerDeliversUserScopedEvent(t *testing.T) {
	sender := &directReadSender{}
	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	payload, err := json.Marshal(application.ContactFriendDeletedPayload{
		UserUUID: "U1", FriendUUID: "U2", OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{
		EventType: "contact.friend.deleted", Payload: payload,
	}}

	if err := NewContactFriendDeletedHandler(sender)(context.Background(), event); err != nil {
		t.Fatalf("handle contact deletion: %v", err)
	}
	if sender.user != "U1" || sender.type_ != wsTransport.TypeContactFriendDeleted {
		t.Fatalf("unexpected delivery target: user=%q type=%q", sender.user, sender.type_)
	}
	data, ok := sender.data.(wsTransport.ContactFriendDeletedEventData)
	if !ok || data.UserUUID != "U1" || data.FriendUUID != "U2" || !data.OccurredAt.Equal(occurredAt) {
		t.Fatalf("unexpected contact deletion data: %#v", sender.data)
	}
}

func TestNewContactFriendDeletedHandlerRejectsMalformedEvent(t *testing.T) {
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{
		EventType: "contact.friend.deleted", Payload: []byte(`{"user_uuid":`),
	}}
	if err := NewContactFriendDeletedHandler(&directReadSender{})(context.Background(), event); err == nil {
		t.Fatal("malformed contact deletion must retain the Kafka record")
	}
}
