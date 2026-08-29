package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

func toolTestContext(principalUserUUID string) context.Context {
	execution := newExecutionContext(ExecutionContext{
		TenantID:          defaultAgentTenantID,
		PrincipalUserUUID: principalUserUUID,
		AgentUUID:         "UAI",
		DelegatedByUUID:   principalUserUUID,
	}, embeddedAgentPermissionsV1(), nil, embeddedAgentResourceScopesV1())
	return withExecutionContext(context.Background(), execution)
}

func TestUserProfileToolInvokableRun(t *testing.T) {
	t.Parallel()

	tool := NewUserProfileTool(&stubAgentCapability{
		users: map[string]*model.User{
			"U100": {
				UUID:     "U100",
				Nickname: "Alice",
				Avatar:   "avatar.png",
				UserType: model.UserTypeNormal,
				Status:   model.UserStatusNormal,
			},
		},
	}, "UAI")

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

	tool := NewRecentMessageSearchTool(&stubAgentCapability{
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

func TestSystemMessageToolInvokableRun(t *testing.T) {
	t.Parallel()

	capability := &stubAgentCapability{
		sentMessage: &model.Message{
			UUID:        "MSYS1",
			MessageType: model.MessageTypeSystem,
		},
	}
	tool := NewSystemMessageTool(capability, "UAI")

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
	if capability.senderUUID != "UAI" || capability.targetUUID != "U100" || capability.content != "maintenance notice" {
		t.Fatalf("unexpected sender args: %+v", capability)
	}
}

func TestListUserConversationsToolInvokableRun(t *testing.T) {
	t.Parallel()

	tool := NewListUserConversationsTool(&stubAgentCapability{
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

	tool := NewReadConversationTool(&stubAgentCapability{
		read: &application.AgentConversationReadV1{
			Found: true, TargetUUID: "U200", TargetType: model.MessageTargetDirect,
			Messages: []*model.Message{
				{UUID: "M1", SenderUUID: "U100", MessageType: model.MessageTypeText, Content: "hi"},
				{UUID: "M2", SenderUUID: "U200", MessageType: model.MessageTypeText, Content: "hello"},
			},
		},
	})

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

	tool := NewReadConversationTool(&stubAgentCapability{read: &application.AgentConversationReadV1{Found: false}})

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
			_, err := NewUserProfileTool(&stubAgentCapability{}, "UAI").(*userProfileTool).InvokableRun(ctx, `{}`)
			return err
		}},
		{name: "search_recent_messages", run: func(ctx context.Context) error {
			_, err := NewRecentMessageSearchTool(&stubAgentCapability{}, "UAI").(*recentMessageSearchTool).InvokableRun(ctx, `{"query":"x"}`)
			return err
		}},
		{name: "send_system_message", run: func(ctx context.Context) error {
			_, err := NewSystemMessageTool(&stubAgentCapability{}, "UAI").(*systemMessageTool).InvokableRun(ctx, `{"content":"x"}`)
			return err
		}},
		{name: ToolListUserConversations, run: func(ctx context.Context) error {
			_, err := NewListUserConversationsTool(&stubAgentCapability{}).(*listUserConversationsTool).InvokableRun(ctx, `{}`)
			return err
		}},
		{name: ToolReadConversation, run: func(ctx context.Context) error {
			_, err := NewReadConversationTool(&stubAgentCapability{}).(*readConversationTool).InvokableRun(ctx, `{"target_uuid":"U200"}`)
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

	tools := NewTools(&stubAgentCapability{}, "UAI")
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

	tool := NewSystemMessageTool(&stubAgentCapability{}, "UAI")
	execution := newExecutionContext(ExecutionContext{
		TenantID: defaultAgentTenantID, PrincipalUserUUID: "U100", AgentUUID: "UOTHER", DelegatedByUUID: "U100",
	}, embeddedAgentPermissionsV1(), nil, embeddedAgentResourceScopesV1())
	ctx := withExecutionContext(context.Background(), execution)
	_, err := tool.(*systemMessageTool).InvokableRun(ctx, `{"content":"x"}`)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected agent identity mismatch, got %v", err)
	}
}

func TestAgentToolsEnforceCapabilityPermissionsBeforeAdapters(t *testing.T) {
	t.Parallel()

	capability := &stubAgentCapability{read: &application.AgentConversationReadV1{Found: true}}
	readExecution := newExecutionContext(ExecutionContext{
		TenantID: defaultAgentTenantID, PrincipalUserUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
	}, []string{application.AgentPermissionUserProfileRead}, nil)
	readCtx := withExecutionContext(context.Background(), readExecution)
	if _, err := NewReadConversationTool(capability).(*readConversationTool).InvokableRun(readCtx, `{"target_uuid":"U200"}`); !errors.Is(err, application.ErrAgentCapabilityDenied) {
		t.Fatalf("expected read permission denial, got %v", err)
	}
	if capability.conversationReads != 0 {
		t.Fatalf("denied read reached Capability adapter %d times", capability.conversationReads)
	}

	writeExecution := newExecutionContext(ExecutionContext{
		TenantID: defaultAgentTenantID, PrincipalUserUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
	}, []string{application.AgentPermissionConversationRead}, nil)
	writeCtx := withExecutionContext(context.Background(), writeExecution)
	if _, err := NewSystemMessageTool(capability, "UAI").(*systemMessageTool).InvokableRun(writeCtx, `{"content":"notice"}`); !errors.Is(err, application.ErrAgentCapabilityDenied) {
		t.Fatalf("expected write permission denial, got %v", err)
	}
	if capability.targetUUID != "" {
		t.Fatalf("denied write reached Capability adapter target %q", capability.targetUUID)
	}
}
