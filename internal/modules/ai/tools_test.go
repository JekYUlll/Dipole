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
