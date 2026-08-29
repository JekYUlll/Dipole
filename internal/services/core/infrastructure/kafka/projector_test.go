package kafka

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

type projectionStub struct {
	groupCalls  int
	directCalls int
	group       *model.Message
	direct      *model.Message
	err         error
}

func (s *projectionStub) InitGroupConversations(string, []string, time.Time) error {
	s.groupCalls++
	return s.err
}

func (s *projectionStub) UpdateDirectConversations(message *model.Message) error {
	s.directCalls++
	s.direct = message
	return s.err
}

func (s *projectionStub) UpdateGroupConversations(message *model.Message) error {
	s.groupCalls++
	s.group = message
	return s.err
}

func TestConversationProjectionPreservesMessageContract(t *testing.T) {
	projector := &projectionStub{}
	payload := messagedomain.MessageEventPayload{
		MessageID: "M1", ConversationKey: "group:G1", MessageSeq: 9,
		SenderUUID: "U1", TargetUUID: "G1", TargetType: model.MessageTargetGroup,
		MessageType: model.MessageTypeText, Content: "hello", SentAt: time.Now().UTC(),
	}
	event := projectionEvent(t, "message.group.created", payload)
	if err := updateConversation(projector, true)(context.Background(), event); err != nil {
		t.Fatalf("project group message: %v", err)
	}
	if projector.groupCalls != 1 || projector.group == nil || projector.group.UUID != "M1" || projector.group.Seq != 9 {
		t.Fatalf("unexpected projected message: calls=%d message=%+v", projector.groupCalls, projector.group)
	}
}

func TestConversationProjectionPropagatesDecodeAndStoreErrors(t *testing.T) {
	projector := &projectionStub{err: errors.New("store unavailable")}
	if err := updateConversation(projector, false)(context.Background(), platformKafka.Event{}); err == nil {
		t.Fatal("expected missing envelope error")
	}
	if err := updateConversation(projector, false)(context.Background(), projectionEvent(t, "message.direct.created", messagedomain.MessageEventPayload{MessageID: "M1"})); err == nil {
		t.Fatal("expected invalid message contract error")
	}
	valid := messagedomain.MessageEventPayload{
		MessageID: "M1", ConversationKey: "direct:U1:U2", MessageSeq: 1,
		SenderUUID: "U1", TargetUUID: "U2", TargetType: model.MessageTargetDirect,
		MessageType: model.MessageTypeText, Content: "hello", SentAt: time.Now().UTC(),
	}
	if err := updateConversation(projector, false)(context.Background(), projectionEvent(t, "message.direct.created", valid)); !errors.Is(err, projector.err) {
		t.Fatalf("expected store error, got %v", err)
	}
}

func projectionEvent(t *testing.T, eventType string, payload any) platformKafka.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal projection event: %v", err)
	}
	return platformKafka.Event{Envelope: &platformKafka.Envelope{EventID: "E1", EventType: eventType, Version: "v1", Payload: raw}}
}
