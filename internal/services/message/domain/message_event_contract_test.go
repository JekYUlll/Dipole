package messagedomain

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
)

func TestDecodeMessageEventPayloadCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType string
		payload   string
		wantType  MessageMutationType
		wantRev   uint64
		wantActor string
		wantErr   error
	}{
		{
			name:      "legacy created defaults",
			eventType: "message.direct.created",
			payload:   `{"message_id":"M1","sender_uuid":"U1","target_type":0}`,
			wantType:  MessageMutationCreated,
			wantRev:   1,
			wantActor: "U1",
		},
		{
			name:      "current producer tolerates additive minor fields",
			eventType: "message.group.created",
			payload:   `{"mutation_type":"created","revision":1,"actor_uuid":"U1","message_id":"M2","sender_uuid":"U1","target_type":1,"future_field":{"enabled":true}}`,
			wantType:  MessageMutationCreated,
			wantRev:   1,
			wantActor: "U1",
		},
		{
			name:      "send requested remains a pre-persistence command",
			eventType: "message.direct.send_requested",
			payload:   `{"message_id":"M-REQUEST","sender_uuid":"U1","target_type":0}`,
		},
		{
			name:      "channel target conflict",
			eventType: "message.group.created",
			payload:   `{"message_id":"M3","sender_uuid":"U1","target_type":0}`,
			wantErr:   ErrMessageEventChannelMismatch,
		},
		{
			name:      "unsupported message event",
			eventType: "message.direct.pinned",
			payload:   `{"message_id":"M4","sender_uuid":"U1","target_type":0}`,
			wantErr:   ErrUnsupportedMessageEventType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, err := DecodeMessageEventPayload(test.eventType, json.RawMessage(test.payload))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("decode error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if payload.MutationType != test.wantType || payload.Revision != test.wantRev || payload.ActorUUID != test.wantActor {
				t.Fatalf("unexpected normalized payload: %+v", payload)
			}
		})
	}
}

func TestDecodeMessageEventPayloadRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := DecodeMessageEventPayload("message.direct.created", json.RawMessage(`{"message_id":`))
	if err == nil {
		t.Fatal("expected malformed payload to fail")
	}
}

func TestMessageEventTargetType(t *testing.T) {
	t.Parallel()

	direct, err := MessageEventTargetType("message.direct.created")
	if err != nil || direct != model.MessageTargetDirect {
		t.Fatalf("direct target = %d, %v", direct, err)
	}
	group, err := MessageEventTargetType("message.group.deleted")
	if err != nil || group != model.MessageTargetGroup {
		t.Fatalf("group target = %d, %v", group, err)
	}
}
