package bootstrap

import (
	"encoding/json"
	"testing"

	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

func TestDecodeMessageEventPayloadPreservesSyncFanoutPresence(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantValue *bool
	}{
		{name: "legacy field omitted", payload: `{"message_id":"M1"}`},
		{name: "hot group explicitly disabled", payload: `{"message_id":"M2","sync_fanout":false}`, wantValue: boolPointer(false)},
		{name: "normal group explicitly enabled", payload: `{"message_id":"M3","sync_fanout":true}`, wantValue: boolPointer(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeMessageEventPayload(platformKafka.Event{Envelope: &platformKafka.Envelope{
				Payload: json.RawMessage(tt.payload),
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

func boolPointer(value bool) *bool {
	return &value
}
