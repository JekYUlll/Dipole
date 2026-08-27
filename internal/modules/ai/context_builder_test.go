package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type stubAgentCapability struct {
	users             map[string]*model.User
	messages          []*model.Message
	conversations     []*model.Conversation
	read              *application.AgentConversationReadV1
	sentMessage       *model.Message
	profileRequested  string
	directReads       int
	conversationReads int
	senderUUID        string
	targetUUID        string
	content           string
	err               error
}

func (s *stubAgentCapability) GetUserProfile(_ context.Context, _ application.AgentInvocationV1, subjectUUID string) (*model.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.profileRequested = subjectUUID
	return s.users[subjectUUID], nil
}

func (s *stubAgentCapability) ListDirectMessages(context.Context, application.AgentInvocationV1, int) ([]*model.Message, error) {
	s.directReads++
	return s.messages, s.err
}

func (s *stubAgentCapability) ListConversations(context.Context, application.AgentInvocationV1, int) ([]*model.Conversation, error) {
	return s.conversations, s.err
}

func (s *stubAgentCapability) ReadConversation(context.Context, application.AgentInvocationV1, string, int) (*application.AgentConversationReadV1, error) {
	s.conversationReads++
	return s.read, s.err
}

func (s *stubAgentCapability) SendSystemMessage(_ context.Context, invocation application.AgentInvocationV1, content string) (*model.Message, error) {
	s.senderUUID, s.targetUUID, s.content = invocation.AgentUUID, invocation.PrincipalUUID, content
	return s.sentMessage, s.err
}

func TestContextBuilderBuildDirectContext(t *testing.T) {
	t.Parallel()

	builder := NewContextBuilder(
		&stubAgentCapability{
			messages: []*model.Message{
				{
					UUID:        "M1",
					SenderUUID:  "U100",
					MessageType: model.MessageTypeText,
					Content:     "hello",
				},
				{
					UUID:        "M2",
					SenderUUID:  "UAI",
					MessageType: model.MessageTypeAIText,
					Content:     "hi, how can I help?",
				},
				{
					UUID:            "M3",
					SenderUUID:      "U100",
					MessageType:     model.MessageTypeFile,
					FileName:        "hello.txt",
					FileSize:        128,
					FileContentType: "text/plain",
					FileURL:         "http://example.com/hello.txt",
				},
			},
			users: map[string]*model.User{
				"U100": {UUID: "U100", UserType: model.UserTypeNormal},
				"UAI":  {UUID: "UAI", UserType: model.UserTypeAssistant},
			},
		},
		12,
	)

	context, err := builder.BuildDirectContext(toolTestContext("U100"), "U100", "UAI")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(context.Messages) != 3 {
		t.Fatalf("expected 3 context messages, got %d", len(context.Messages))
	}
	if context.Messages[0].Role != schema.User {
		t.Fatalf("expected first message to be user, got %s", context.Messages[0].Role)
	}
	if context.Messages[1].Role != schema.Assistant {
		t.Fatalf("expected second message to be assistant, got %s", context.Messages[1].Role)
	}
	if context.Messages[2].Role != schema.User {
		t.Fatalf("expected file message to remain user-side context, got %s", context.Messages[2].Role)
	}
	if context.Messages[2].Content == "" {
		t.Fatalf("expected rendered file content, got empty string")
	}
}

func TestContextBuilderStopsBeforeMessagesWithoutReadPermission(t *testing.T) {
	t.Parallel()

	capability := &stubAgentCapability{users: map[string]*model.User{
		"U100": {UUID: "U100"},
		"UAI":  {UUID: "UAI", UserType: model.UserTypeAssistant},
	}}
	builder := NewContextBuilder(capability, 12)
	execution := newExecutionContext(ExecutionContext{
		TenantID: defaultAgentTenantID, PrincipalUserUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
	}, []string{application.AgentPermissionUserProfileRead}, nil)
	ctx := withExecutionContext(context.Background(), execution)

	if _, err := builder.BuildDirectContext(ctx, "U100", "UAI"); !errors.Is(err, application.ErrAgentCapabilityDenied) {
		t.Fatalf("expected missing conversation permission denial, got %v", err)
	}
	if capability.directReads != 0 {
		t.Fatalf("denied context build reached Message capability %d times", capability.directReads)
	}
}
