package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JekYUlll/Dipole/internal/compat/service"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
)

type sessionKickController struct {
	user      string
	ids       []string
	reason    string
	all       bool
	allCalls  int
	kickCalls int
}

func (c *sessionKickController) DisconnectConnections(userUUID string, connectionIDs []string, reason string) int {
	c.user, c.ids, c.reason, c.kickCalls = userUUID, connectionIDs, reason, c.kickCalls+1
	return len(connectionIDs)
}

func (c *sessionKickController) DisconnectAllConnections(userUUID string, reason string) int {
	c.user, c.reason, c.all, c.allCalls = userUUID, reason, true, c.allCalls+1
	return 1
}

func TestNewSessionKickHandlerDisconnectsSelectedConnections(t *testing.T) {
	controller := &sessionKickController{}
	payload, err := json.Marshal(service.SessionKickEventPayload{
		UserUUID: "U1", ConnectionIDs: []string{"C1", "C2"}, Reason: "security",
	})
	if err != nil {
		t.Fatal(err)
	}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "session.force_logout", Payload: payload}}
	if err := NewSessionKickHandler(controller)(context.Background(), event); err != nil {
		t.Fatalf("handle selected session kick: %v", err)
	}
	if controller.kickCalls != 1 || controller.allCalls != 0 || controller.user != "U1" || controller.reason != "security" {
		t.Fatalf("unexpected selected kick: %+v", controller)
	}
	if len(controller.ids) != 2 || controller.ids[0] != "C1" || controller.ids[1] != "C2" {
		t.Fatalf("unexpected connection ids: %+v", controller.ids)
	}
}

func TestNewSessionKickHandlerDisconnectsAllConnections(t *testing.T) {
	controller := &sessionKickController{}
	payload, err := json.Marshal(service.SessionKickEventPayload{UserUUID: "U1", All: true, Reason: "logout_all"})
	if err != nil {
		t.Fatal(err)
	}
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "session.force_logout", Payload: payload}}
	if err := NewSessionKickHandler(controller)(context.Background(), event); err != nil {
		t.Fatalf("handle all session kick: %v", err)
	}
	if !controller.all || controller.allCalls != 1 || controller.kickCalls != 0 || controller.user != "U1" || controller.reason != "logout_all" {
		t.Fatalf("unexpected all kick: %+v", controller)
	}
}

func TestNewSessionKickHandlerRejectsMalformedEvent(t *testing.T) {
	event := platformKafka.Event{Envelope: &platformKafka.Envelope{EventType: "session.force_logout", Payload: []byte(`{"user_uuid":`)}}
	if err := NewSessionKickHandler(&sessionKickController{})(context.Background(), event); err == nil {
		t.Fatal("malformed session kick must retain the Kafka record")
	}
}
