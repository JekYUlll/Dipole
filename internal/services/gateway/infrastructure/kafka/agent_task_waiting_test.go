package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
)

type agentTaskWaitingObserver struct{ outcomes []string }

func (o *agentTaskWaitingObserver) Observe(outcome string) { o.outcomes = append(o.outcomes, outcome) }

type offlineAgentTaskWaitingSender struct{}

func (offlineAgentTaskWaitingSender) SendEventToUser(string, string, any) int { return 0 }

func TestAgentTaskWaitingHandlerDeliversOwnerLocator(t *testing.T) {
	sender := &directReadSender{}
	observer := &agentTaskWaitingObserver{}
	payload, err := json.Marshal(application.AgentTaskWaitingNotificationV1{TenantID: "dipole", PrincipalUUID: "U100", TaskUUID: "TASK-1", PendingKind: "approval", Revision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAgentTaskWaitingHandler(sender, observer)(context.Background(), platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: application.AgentTaskWaitingEventTypeV1, Payload: payload}}); err != nil {
		t.Fatal(err)
	}
	data, ok := sender.data.(wsTransport.AgentTaskWaitingEventData)
	if sender.user != "U100" || sender.type_ != wsTransport.TypeAgentTaskWaiting || !ok || data.TaskUUID != "TASK-1" || data.PendingKind != "approval" || data.Revision != 2 {
		t.Fatalf("sender=%+v data=%#v", sender, sender.data)
	}
	if got := observer.outcomes; len(got) != 1 || got[0] != "online" {
		t.Fatalf("outcomes=%v", got)
	}
}

func TestAgentTaskWaitingHandlerRejectsMalformedPayload(t *testing.T) {
	observer := &agentTaskWaitingObserver{}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: application.AgentTaskWaitingEventTypeV1, Payload: []byte(`{"principal_uuid":"U100"}`)}}
	if err := NewAgentTaskWaitingHandler(&directReadSender{}, observer)(context.Background(), event); err == nil {
		t.Fatal("malformed Agent Task wait must retain the Kafka record")
	}
	if got := observer.outcomes; len(got) != 1 || got[0] != "invalid" {
		t.Fatalf("outcomes=%v", got)
	}
}

func TestAgentTaskWaitingHandlerTracksOfflineLocator(t *testing.T) {
	observer := &agentTaskWaitingObserver{}
	payload, err := json.Marshal(application.AgentTaskWaitingNotificationV1{TenantID: "dipole", PrincipalUUID: "U100", TaskUUID: "TASK-1", PendingKind: "approval", Revision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAgentTaskWaitingHandler(offlineAgentTaskWaitingSender{}, observer)(context.Background(), platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: application.AgentTaskWaitingEventTypeV1, Payload: payload}}); err != nil {
		t.Fatal(err)
	}
	if got := observer.outcomes; len(got) != 1 || got[0] != "offline" {
		t.Fatalf("outcomes=%v", got)
	}
}
