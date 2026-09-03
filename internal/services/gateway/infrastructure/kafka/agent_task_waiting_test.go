package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

func TestAgentTaskWaitingHandlerDeliversOwnerLocator(t *testing.T) {
	sender := &directReadSender{}
	payload, err := json.Marshal(application.AgentTaskWaitingNotificationV1{TenantID: "dipole", PrincipalUUID: "U100", TaskUUID: "TASK-1", PendingKind: "approval", Revision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAgentTaskWaitingHandler(sender)(context.Background(), platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: application.AgentTaskWaitingEventTypeV1, Payload: payload}}); err != nil {
		t.Fatal(err)
	}
	data, ok := sender.data.(wsTransport.AgentTaskWaitingEventData)
	if sender.user != "U100" || sender.type_ != wsTransport.TypeAgentTaskWaiting || !ok || data.TaskUUID != "TASK-1" || data.PendingKind != "approval" || data.Revision != 2 {
		t.Fatalf("sender=%+v data=%#v", sender, sender.data)
	}
}

func TestAgentTaskWaitingHandlerRejectsMalformedPayload(t *testing.T) {
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: application.AgentTaskWaitingEventTypeV1, Payload: []byte(`{"principal_uuid":"U100"}`)}}
	if err := NewAgentTaskWaitingHandler(&directReadSender{})(context.Background(), event); err == nil {
		t.Fatal("malformed Agent Task wait must retain the Kafka record")
	}
}
