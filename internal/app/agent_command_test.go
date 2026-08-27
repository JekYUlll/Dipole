package app

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

type agentCommandMessagesStub struct {
	kind            application.AgentMessageCommandKindV1
	sender          string
	target          string
	content         string
	clientMessageID string
	ids             correlation.IDs
}

func (s *agentCommandMessagesStub) SendAssistantTextMessageContext(ctx context.Context, sender, target, content, clientMessageID string) (*model.Message, error) {
	s.record(ctx, application.AgentMessageCommandAssistantReplyV1, sender, target, content, clientMessageID)
	return &model.Message{UUID: "M-REPLY", ClientMessageID: clientMessageID, MessageType: model.MessageTypeAIText}, nil
}

func (s *agentCommandMessagesStub) SendSystemDirectMessageCommandContext(ctx context.Context, sender, target, content, clientMessageID string) (*model.Message, error) {
	s.record(ctx, application.AgentMessageCommandSystemMessageV1, sender, target, content, clientMessageID)
	return &model.Message{UUID: "M-SYSTEM", ClientMessageID: clientMessageID, MessageType: model.MessageTypeSystem}, nil
}

func (s *agentCommandMessagesStub) record(ctx context.Context, kind application.AgentMessageCommandKindV1, sender, target, content, clientMessageID string) {
	s.kind, s.sender, s.target, s.content, s.clientMessageID = kind, sender, target, content, clientMessageID
	s.ids = correlation.FromContext(ctx)
}

func TestLocalAgentCommandV1RoutesTrustedIdentityAndCorrelation(t *testing.T) {
	t.Parallel()

	messages := &agentCommandMessagesStub{}
	commands, err := NewLocalAgentCommandV1(messages)
	if err != nil {
		t.Fatalf("new Agent Command: %v", err)
	}
	invocation := agentCapabilityTestInvocation()
	invocation.RequestID = "REQ-1"
	invocation.TraceID = "TRACE-1"
	invocation.EventID = "EVENT-1"
	command := application.AgentMessageCommandV1{
		CommandID:  "trigger:M100:assistant-reply",
		Kind:       application.AgentMessageCommandAssistantReplyV1,
		Invocation: invocation,
		Content:    "hello",
	}
	message, err := commands.SendMessage(context.Background(), command)
	if err != nil {
		t.Fatalf("send Agent Message Command: %v", err)
	}
	if message.UUID != "M-REPLY" || messages.kind != application.AgentMessageCommandAssistantReplyV1 {
		t.Fatalf("unexpected command route: message=%+v stub=%+v", message, messages)
	}
	if messages.sender != "UAI" || messages.target != "U100" || messages.content != "hello" {
		t.Fatalf("command did not derive trusted participants: %+v", messages)
	}
	if messages.clientMessageID == "" || len(messages.clientMessageID) > 64 || messages.clientMessageID != message.ClientMessageID {
		t.Fatalf("invalid derived client message ID %q", messages.clientMessageID)
	}
	if messages.ids != (correlation.IDs{RequestID: "REQ-1", TraceID: "TRACE-1", EventID: "EVENT-1"}) {
		t.Fatalf("command lost correlation IDs: %+v", messages.ids)
	}
}

func TestLocalAgentCommandV1UsesStableIdempotencyKey(t *testing.T) {
	t.Parallel()

	messages := &agentCommandMessagesStub{}
	commands, err := NewLocalAgentCommandV1(messages)
	if err != nil {
		t.Fatalf("new Agent Command: %v", err)
	}
	command := application.AgentMessageCommandV1{
		CommandID:  "trigger:M100:system-message:1",
		Kind:       application.AgentMessageCommandSystemMessageV1,
		Invocation: agentCapabilityTestInvocation(),
		Content:    "notice",
	}
	first, err := commands.SendMessage(context.Background(), command)
	if err != nil {
		t.Fatalf("first command: %v", err)
	}
	second, err := commands.SendMessage(context.Background(), command)
	if err != nil {
		t.Fatalf("replayed command: %v", err)
	}
	if first.ClientMessageID == "" || first.ClientMessageID != second.ClientMessageID {
		t.Fatalf("command replay changed idempotency key: first=%q second=%q", first.ClientMessageID, second.ClientMessageID)
	}
}

func TestAgentCommandV1ClientMessageIDMatchesLanguageNeutralGoldenVector(t *testing.T) {
	t.Parallel()

	got := agentCommandClientMessageIDV1(
		application.AgentMessageCommandAssistantReplyV1,
		"trigger:M100:assistant-reply",
	)
	const want = "15ad8e7f820975681dee493f2ad1f98c1db80f3a43adbe6d6a46680b8e5a6922"
	if got != want {
		t.Fatalf("Agent Command client message ID = %q, want %q", got, want)
	}
}

func TestLocalAgentCommandV1FailsClosed(t *testing.T) {
	t.Parallel()

	messages := &agentCommandMessagesStub{}
	commands, err := NewLocalAgentCommandV1(messages)
	if err != nil {
		t.Fatalf("new Agent Command: %v", err)
	}
	tests := []application.AgentMessageCommandV1{
		{CommandID: "C1", Kind: application.AgentMessageCommandAssistantReplyV1, Content: "hello"},
		{CommandID: "C2", Kind: application.AgentMessageCommandKindV1("unknown"), Invocation: agentCapabilityTestInvocation(), Content: "hello"},
		{CommandID: "", Kind: application.AgentMessageCommandAssistantReplyV1, Invocation: agentCapabilityTestInvocation(), Content: "hello"},
		{CommandID: "C4", Kind: application.AgentMessageCommandAssistantReplyV1, Invocation: agentCapabilityTestInvocation(), Content: ""},
	}
	denied := agentCapabilityTestInvocation()
	denied.Permissions = nil
	tests = append(tests, application.AgentMessageCommandV1{CommandID: "C5", Kind: application.AgentMessageCommandAssistantReplyV1, Invocation: denied, Content: "hello"})
	for _, command := range tests {
		if _, err := commands.SendMessage(context.Background(), command); !errors.Is(err, application.ErrAgentCommandDenied) {
			t.Fatalf("command %+v should be denied, got %v", command, err)
		}
	}
	if messages.kind != "" {
		t.Fatalf("denied command reached Message Service: %+v", messages)
	}
	if _, err := commands.SendMessage(context.Background(), tests[len(tests)-1]); !errors.Is(err, application.ErrAgentCapabilityDenied) {
		t.Fatalf("policy denial cause was not preserved, got %v", err)
	}
}

func TestNewLocalAgentCommandV1RejectsMissingDependency(t *testing.T) {
	t.Parallel()

	if _, err := NewLocalAgentCommandV1(nil); err == nil {
		t.Fatal("expected missing Message dependency rejection")
	}
}
