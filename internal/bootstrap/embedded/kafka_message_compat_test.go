package embedded

import (
	"encoding/json"
	"testing"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

func TestDecodeMessageEventPayloadPreservesSyncFanoutPresence(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantValue *bool
	}{
		{name: "legacy field omitted", payload: `{"message_id":"M1","sender_uuid":"U1"}`},
		{name: "fanout explicitly disabled", payload: `{"message_id":"M2","sender_uuid":"U1","sync_fanout":false}`, wantValue: boolPointer(false)},
		{name: "fanout explicitly enabled", payload: `{"message_id":"M3","sender_uuid":"U1","sync_fanout":true}`, wantValue: boolPointer(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeMessageEventPayload(platformKafka.Event{Envelope: &platformKafka.Envelope{
				EventType: "message.direct.created",
				Payload:   json.RawMessage(tt.payload),
			}})
			if err != nil {
				t.Fatalf("decode message event payload: %v", err)
			}
			if tt.wantValue == nil {
				if decoded.SyncFanout != nil {
					t.Fatalf("expected omitted sync_fanout to remain nil, got %v", *decoded.SyncFanout)
				}
				return
			}
			if decoded.SyncFanout == nil || *decoded.SyncFanout != *tt.wantValue {
				t.Fatalf("expected sync_fanout=%v, got %v", *tt.wantValue, decoded.SyncFanout)
			}
		})
	}
}

func TestDecodeMessageEventPayloadPreservesOptionalConversationSequence(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    uint64
	}{
		{name: "legacy field omitted", payload: `{"message_id":"M1","sender_uuid":"U1"}`},
		{name: "sequence present", payload: `{"message_id":"M2","sender_uuid":"U1","message_seq":42}`, want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeMessageEventPayload(platformKafka.Event{Envelope: &platformKafka.Envelope{
				EventType: "message.direct.created",
				Payload:   json.RawMessage(tt.payload),
			}})
			if err != nil {
				t.Fatalf("decode message event payload: %v", err)
			}
			if decoded.MessageSeq != tt.want {
				t.Fatalf("message sequence = %d, want %d", decoded.MessageSeq, tt.want)
			}
		})
	}
}

func TestDecodeMessageEventPayloadNormalizesLegacyCreatedMutation(t *testing.T) {
	decoded, err := decodeMessageEventPayload(platformKafka.Event{Envelope: &platformKafka.Envelope{
		EventType: "message.direct.created",
		Payload:   json.RawMessage(`{"message_id":"M1","sender_uuid":"U1"}`),
	}})
	if err != nil {
		t.Fatalf("decode legacy created mutation: %v", err)
	}
	if decoded.MutationType != service.MessageMutationCreated || decoded.Revision != 1 || decoded.ActorUUID != "U1" {
		t.Fatalf("unexpected normalized mutation: %+v", decoded)
	}
}

func boolPointer(value bool) *bool {
	return &value
}
