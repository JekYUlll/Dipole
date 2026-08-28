package app

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
)

type agentCommandMessagesStub struct {
	kind              application.AgentMessageCommandKindV1
	sender            string
	target            string
	content           string
	clientMessageID   string
	ids               correlation.IDs
	lineage           eventlineage.Lineage
	sendErr           error
	receipt           *application.MessageCommandReceipt
	receiptErr        error
	receiptSender     string
	receiptClientID   string
	receiptContextErr error
	sendMessage       *model.Message
}

func (s *agentCommandMessagesStub) SendAssistantTextMessageContext(ctx context.Context, sender, target, content, clientMessageID string) (*model.Message, error) {
	s.record(ctx, application.AgentMessageCommandAssistantReplyV1, sender, target, content, clientMessageID)
	if s.sendErr != nil {
		return nil, s.sendErr
	}
	if s.sendMessage != nil {
		return s.sendMessage, nil
	}
	return commandStubMessage("M-REPLY", sender, target, content, clientMessageID, model.MessageTypeAIText), nil
}

func (s *agentCommandMessagesStub) SendSystemDirectMessageCommandContext(ctx context.Context, sender, target, content, clientMessageID string) (*model.Message, error) {
	s.record(ctx, application.AgentMessageCommandSystemMessageV1, sender, target, content, clientMessageID)
	if s.sendErr != nil {
		return nil, s.sendErr
	}
	if s.sendMessage != nil {
		return s.sendMessage, nil
	}
	return commandStubMessage("M-SYSTEM", sender, target, content, clientMessageID, model.MessageTypeSystem), nil
}

func (s *agentCommandMessagesStub) GetMessageCommandReceiptContext(ctx context.Context, sender, clientMessageID string) (*application.MessageCommandReceipt, error) {
	s.receiptSender, s.receiptClientID = sender, clientMessageID
	s.receiptContextErr = ctx.Err()
	return s.receipt, s.receiptErr
}

func commandStubMessage(uuid, sender, target, content, clientMessageID string, messageType int8) *model.Message {
	return &model.Message{
		UUID: uuid, SenderUUID: sender, TargetUUID: target, TargetType: model.MessageTargetDirect,
		ConversationKey: model.DirectConversationKey(sender, target), Content: content,
		ClientMessageID: clientMessageID, MessageType: messageType,
	}
}

func (s *agentCommandMessagesStub) record(ctx context.Context, kind application.AgentMessageCommandKindV1, sender, target, content, clientMessageID string) {
	s.kind, s.sender, s.target, s.content, s.clientMessageID = kind, sender, target, content, clientMessageID
	s.ids = correlation.FromContext(ctx)
	s.lineage = eventlineage.FromContext(ctx)
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
	ctx := eventlineage.WithContext(context.Background(), eventlineage.Lineage{
		Origin:           eventlineage.Origin{Type: eventlineage.OriginAgent, ID: "UAI"},
		CausationEventID: "EVENT-1", AgentTaskID: "TASK-1",
	})
	message, err := commands.SendMessage(ctx, command)
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
	if messages.lineage.Origin.ID != "UAI" || messages.lineage.AgentTaskID != "TASK-1" || messages.lineage.CausationEventID != "EVENT-1" {
		t.Fatalf("command lost event lineage: %+v", messages.lineage)
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

func TestLocalAgentCommandV1RecoversCommittedReceiptAfterUncertainSend(t *testing.T) {
	t.Parallel()

	sendErr := context.DeadlineExceeded
	messages := &agentCommandMessagesStub{sendErr: sendErr}
	commands, err := NewLocalAgentCommandV1(messages)
	if err != nil {
		t.Fatalf("new Agent Command: %v", err)
	}
	command := application.AgentMessageCommandV1{
		CommandID: "trigger:M100:assistant-reply", Kind: application.AgentMessageCommandAssistantReplyV1,
		Invocation: agentCapabilityTestInvocation(), Content: "hello",
	}
	clientMessageID := mustAgentCommandClientMessageIDV1(t, command.Kind, command.CommandID)
	messages.receipt = &application.MessageCommandReceipt{
		Status:  application.MessageCommandReceiptStatusCommitted,
		Message: commandStubMessage("M-RECOVERED", command.Invocation.AgentUUID, command.Invocation.PrincipalUUID, command.Content, clientMessageID, model.MessageTypeAIText),
	}

	parent, cancel := context.WithCancel(context.Background())
	cancel()
	message, err := commands.SendMessage(parent, command)
	if err != nil || message.UUID != "M-RECOVERED" || messages.receiptSender != command.Invocation.AgentUUID || messages.receiptClientID != clientMessageID || messages.receiptContextErr != nil {
		t.Fatalf("recovered message=%+v sender=%q client=%q receipt_ctx=%v err=%v", message, messages.receiptSender, messages.receiptClientID, messages.receiptContextErr, err)
	}
}

func TestLocalAgentCommandV1RejectsAbsentOrConflictingReceipt(t *testing.T) {
	t.Parallel()

	command := application.AgentMessageCommandV1{
		CommandID: "trigger:M100:system-message", Kind: application.AgentMessageCommandSystemMessageV1,
		Invocation: agentCapabilityTestInvocation(), Content: "notice",
	}
	clientMessageID := mustAgentCommandClientMessageIDV1(t, command.Kind, command.CommandID)
	for _, test := range []struct {
		name    string
		receipt *application.MessageCommandReceipt
		wantErr error
	}{
		{name: "absent", receipt: &application.MessageCommandReceipt{Status: application.MessageCommandReceiptStatusAbsent}, wantErr: context.DeadlineExceeded},
		{name: "content drift", receipt: &application.MessageCommandReceipt{
			Status:  application.MessageCommandReceiptStatusCommitted,
			Message: commandStubMessage("M-CONFLICT", command.Invocation.AgentUUID, command.Invocation.PrincipalUUID, "different", clientMessageID, model.MessageTypeSystem),
		}, wantErr: application.ErrAgentCommandConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages := &agentCommandMessagesStub{sendErr: context.DeadlineExceeded, receipt: test.receipt}
			commands, err := NewLocalAgentCommandV1(messages)
			if err != nil {
				t.Fatalf("new Agent Command: %v", err)
			}
			if _, err := commands.SendMessage(context.Background(), command); !errors.Is(err, test.wantErr) {
				t.Fatalf("receipt error=%v want=%v", err, test.wantErr)
			}
		})
	}
}

func TestLocalAgentCommandV1PreservesReceiptFailureAndRejectsSendBindingDrift(t *testing.T) {
	t.Parallel()

	command := application.AgentMessageCommandV1{
		CommandID: "trigger:M100:assistant-reply", Kind: application.AgentMessageCommandAssistantReplyV1,
		Invocation: agentCapabilityTestInvocation(), Content: "hello",
	}
	receiptErr := errors.New("receipt backend unavailable")
	messages := &agentCommandMessagesStub{sendErr: context.DeadlineExceeded, receiptErr: receiptErr}
	commands, err := NewLocalAgentCommandV1(messages)
	if err != nil {
		t.Fatalf("new Agent Command: %v", err)
	}
	if _, err := commands.SendMessage(context.Background(), command); !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, receiptErr) {
		t.Fatalf("joined recovery error=%v", err)
	}

	messages = &agentCommandMessagesStub{sendMessage: commandStubMessage(
		"M-WRONG", command.Invocation.AgentUUID, command.Invocation.PrincipalUUID, "different",
		mustAgentCommandClientMessageIDV1(t, command.Kind, command.CommandID), model.MessageTypeAIText,
	)}
	commands, err = NewLocalAgentCommandV1(messages)
	if err != nil {
		t.Fatalf("new Agent Command for drift: %v", err)
	}
	if _, err := commands.SendMessage(context.Background(), command); !errors.Is(err, application.ErrAgentCommandConflict) {
		t.Fatalf("send binding drift error=%v", err)
	}
}

func TestAgentCommandV1ClientMessageIDMatchesLanguageNeutralGoldenVector(t *testing.T) {
	t.Parallel()

	got, err := application.AgentCommandClientMessageIDV1(
		application.AgentMessageCommandAssistantReplyV1,
		"trigger:M100:assistant-reply",
	)
	if err != nil {
		t.Fatalf("derive Agent Command client message ID: %v", err)
	}
	const want = "15ad8e7f820975681dee493f2ad1f98c1db80f3a43adbe6d6a46680b8e5a6922"
	if got != want {
		t.Fatalf("Agent Command client message ID = %q, want %q", got, want)
	}
}

func mustAgentCommandClientMessageIDV1(t *testing.T, kind application.AgentMessageCommandKindV1, commandID string) string {
	t.Helper()
	clientMessageID, err := application.AgentCommandClientMessageIDV1(kind, commandID)
	if err != nil {
		t.Fatalf("derive Agent Command client message ID: %v", err)
	}
	return clientMessageID
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
	outOfScope := agentCapabilityTestInvocation()
	outOfScope.ResourceScopes = []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: "group:G1", Actions: []string{application.AgentResourceActionWrite}}}
	tests = append(tests, application.AgentMessageCommandV1{CommandID: "C6", Kind: application.AgentMessageCommandAssistantReplyV1, Invocation: outOfScope, Content: "hello"})
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
