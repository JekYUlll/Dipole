package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/model"
)

func toolTestContext(principalUserUUID string) context.Context {
	return withExecutionContext(context.Background(), ExecutionContext{
		PrincipalUserUUID: principalUserUUID,
		AgentUUID:         "UAI",
	})
}

func TestUserProfileToolInvokableRun(t *testing.T) {
	t.Parallel()

	tool := NewUserProfileTool(&stubAIUserReader{
		users: map[string]*model.User{
			"U100": {
				UUID:     "U100",
				Nickname: "Alice",
				Avatar:   "avatar.png",
				UserType: model.UserTypeNormal,
				Status:   model.UserStatusNormal,
			},
		},
	})

	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.Name != ToolGetUserProfile {
		t.Fatalf("expected tool name %s, got %s", ToolGetUserProfile, info.Name)
	}

	result, err := tool.(*userProfileTool).InvokableRun(toolTestContext("U100"), `{}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolUserProfile
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if !payload.Found || payload.UUID != "U100" || payload.Nickname != "Alice" {
		t.Fatalf("unexpected tool payload: %+v", payload)
	}
}

func TestRecentMessageSearchToolInvokableRun(t *testing.T) {
	t.Parallel()

	tool := NewRecentMessageSearchTool(&stubAIMessageReader{
		messages: []*model.Message{
			{UUID: "M1", SenderUUID: "U100", MessageType: model.MessageTypeText, Content: "hello there"},
			{UUID: "M2", SenderUUID: "UAI", MessageType: model.MessageTypeAIText, Content: "I can help with redis cache"},
			{UUID: "M3", SenderUUID: "U100", MessageType: model.MessageTypeText, Content: "tell me about cache strategy"},
		},
	}, "UAI")

	result, err := tool.(*recentMessageSearchTool).InvokableRun(toolTestContext("U100"), `{"query":"cache","limit":2}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolRecentMessageSearchResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if !payload.Found {
		t.Fatalf("expected matches, got %+v", payload)
	}
	if len(payload.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(payload.Matches))
	}
	if payload.Matches[0].Role != schema.User {
		t.Fatalf("expected newest match from user first, got %+v", payload.Matches[0])
	}
}

type stubSystemMessageSender struct {
	message    *model.Message
	senderUUID string
	targetUUID string
	content    string
	err        error
}

func (s *stubSystemMessageSender) SendSystemDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error) {
	s.senderUUID = senderUUID
	s.targetUUID = targetUUID
	s.content = content
	if s.err != nil {
		return nil, s.err
	}
	return s.message, nil
}

func TestSystemMessageToolInvokableRun(t *testing.T) {
	t.Parallel()

	sender := &stubSystemMessageSender{
		message: &model.Message{
			UUID:        "MSYS1",
			MessageType: model.MessageTypeSystem,
		},
	}
	tool := NewSystemMessageTool(sender, "UAI")

	result, err := tool.(*systemMessageTool).InvokableRun(toolTestContext("U100"), `{"content":"maintenance notice"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolSendMessageResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if !payload.Sent || payload.MessageUUID != "MSYS1" {
		t.Fatalf("unexpected tool payload: %+v", payload)
	}
	if sender.senderUUID != "UAI" || sender.targetUUID != "U100" || sender.content != "maintenance notice" {
		t.Fatalf("unexpected sender args: %+v", sender)
	}
}

type stubConversationReader struct {
	conversations []*model.Conversation
	byKey         map[string]*model.Conversation
}

func (s *stubConversationReader) ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error) {
	return s.conversations, nil
}

func (s *stubConversationReader) GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error) {
	if s.byKey == nil {
		return nil, nil
	}
	return s.byKey[conversationKey], nil
}

func TestListUserConversationsToolInvokableRun(t *testing.T) {
	t.Parallel()

	tool := NewListUserConversationsTool(&stubConversationReader{
		conversations: []*model.Conversation{
			{TargetUUID: "U200", TargetType: model.MessageTargetDirect, LastMessagePreview: "hey", UnreadCount: 2},
			{TargetUUID: "GXYZ", TargetType: model.MessageTargetGroup, LastMessagePreview: "hello group", UnreadCount: 0},
		},
	})

	result, err := tool.(*listUserConversationsTool).InvokableRun(toolTestContext("U100"), `{}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolListConversationsResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if payload.Count != 2 {
		t.Fatalf("expected 2 conversations, got %d", payload.Count)
	}
	if payload.Conversations[0].TargetType != "direct" {
		t.Fatalf("expected direct, got %s", payload.Conversations[0].TargetType)
	}
	if payload.Conversations[1].TargetType != "group" {
		t.Fatalf("expected group, got %s", payload.Conversations[1].TargetType)
	}
}

func TestReadConversationToolInvokableRun(t *testing.T) {
	t.Parallel()

	convKey := model.DirectConversationKey("U100", "U200")
	tool := NewReadConversationTool(
		&stubConversationReader{
			byKey: map[string]*model.Conversation{
				convKey: {ConversationKey: convKey},
			},
		},
		&stubAIMessageReader{
			messages: []*model.Message{
				{UUID: "M1", SenderUUID: "U100", MessageType: model.MessageTypeText, Content: "hi"},
				{UUID: "M2", SenderUUID: "U200", MessageType: model.MessageTypeText, Content: "hello"},
			},
		},
	)

	result, err := tool.(*readConversationTool).InvokableRun(toolTestContext("U100"), `{"target_uuid":"U200"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolReadConversationResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if !payload.Found {
		t.Fatalf("expected found, got %+v", payload)
	}
	if payload.MessageCount != 2 {
		t.Fatalf("expected 2 messages, got %d", payload.MessageCount)
	}
	if !payload.Messages[0].IsSelf {
		t.Fatalf("expected first message to be self (U100)")
	}
	if payload.Messages[1].IsSelf {
		t.Fatalf("expected second message to not be self (U200)")
	}
}

func TestReadConversationToolPermissionDenied(t *testing.T) {
	t.Parallel()

	tool := NewReadConversationTool(
		&stubConversationReader{byKey: map[string]*model.Conversation{}},
		&stubAIMessageReader{},
	)

	result, err := tool.(*readConversationTool).InvokableRun(toolTestContext("U100"), `{"target_uuid":"U999"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolReadConversationResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if payload.Found {
		t.Fatalf("expected not found for inaccessible conversation")
	}
}

func TestAgentToolsFailClosedWithoutExecutionContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(context.Context) error
	}{
		{name: ToolGetUserProfile, run: func(ctx context.Context) error {
			_, err := NewUserProfileTool(&stubAIUserReader{}).(*userProfileTool).InvokableRun(ctx, `{}`)
			return err
		}},
		{name: "search_recent_messages", run: func(ctx context.Context) error {
			_, err := NewRecentMessageSearchTool(&stubAIMessageReader{}, "UAI").(*recentMessageSearchTool).InvokableRun(ctx, `{"query":"x"}`)
			return err
		}},
		{name: "send_system_message", run: func(ctx context.Context) error {
			_, err := NewSystemMessageTool(&stubSystemMessageSender{}, "UAI").(*systemMessageTool).InvokableRun(ctx, `{"content":"x"}`)
			return err
		}},
		{name: ToolListUserConversations, run: func(ctx context.Context) error {
			_, err := NewListUserConversationsTool(&stubConversationReader{}).(*listUserConversationsTool).InvokableRun(ctx, `{}`)
			return err
		}},
		{name: ToolReadConversation, run: func(ctx context.Context) error {
			_, err := NewReadConversationTool(&stubConversationReader{}, &stubAIMessageReader{}).(*readConversationTool).InvokableRun(ctx, `{"target_uuid":"U200"}`)
			return err
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.run(context.Background()); !errors.Is(err, ErrExecutionContextMissing) {
				t.Fatalf("expected missing execution context, got %v", err)
			}
		})
	}
}

func TestAgentToolSchemasDoNotExposeModelControlledIdentity(t *testing.T) {
	t.Parallel()

	tools := NewTools(&stubAIUserReader{}, &stubAIMessageReader{}, &stubConversationReader{}, &stubSystemMessageSender{}, "UAI")
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Fatalf("read tool info: %v", err)
		}
		payload, err := json.Marshal(info.ParamsOneOf)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", info.Name, err)
		}
		if strings.Contains(string(payload), "user_uuid") {
			t.Errorf("tool %s exposes user_uuid in model schema: %s", info.Name, payload)
		}
	}
}

func TestSystemMessageToolRejectsMismatchedAgentIdentity(t *testing.T) {
	t.Parallel()

	tool := NewSystemMessageTool(&stubSystemMessageSender{}, "UAI")
	ctx := withExecutionContext(context.Background(), ExecutionContext{PrincipalUserUUID: "U100", AgentUUID: "UOTHER"})
	_, err := tool.(*systemMessageTool).InvokableRun(ctx, `{"content":"x"}`)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected agent identity mismatch, got %v", err)
	}
}
