package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/model"
)

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

	result, err := tool.(*userProfileTool).InvokableRun(context.Background(), `{"user_uuid":"U100"}`)
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

func TestConversationContextToolInvokableRun(t *testing.T) {
	tool := NewConversationContextTool(&stubContextBuilder{
		context: &ConversationContext{
			EndUser:   &model.User{UUID: "U100", Nickname: "Alice", UserType: model.UserTypeNormal},
			Assistant: &model.User{UUID: "UAI", Nickname: "Dipole AI", UserType: model.UserTypeAssistant},
			Messages: []*schema.Message{
				schema.UserMessage("hello"),
				schema.AssistantMessage("hi", nil),
			},
		},
	}, "UAI")

	result, err := tool.(*conversationContextTool).InvokableRun(context.Background(), `{"user_uuid":"U100"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload toolConversationContext
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid json result, got %v", err)
	}
	if !payload.Found {
		t.Fatalf("expected found conversation context, got %+v", payload)
	}
	if payload.MessageCount != 2 {
		t.Fatalf("expected 2 context messages, got %d", payload.MessageCount)
	}
	if len(payload.Messages) != 2 || payload.Messages[0].Role != schema.User {
		t.Fatalf("unexpected context messages: %+v", payload.Messages)
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

	result, err := tool.(*recentMessageSearchTool).InvokableRun(context.Background(), `{"user_uuid":"U100","query":"cache","limit":2}`)
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

	result, err := tool.(*systemMessageTool).InvokableRun(context.Background(), `{"user_uuid":"U100","content":"maintenance notice"}`)
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
