package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

const (
	ToolGetUserProfile        = "get_user_profile"
	ToolListUserConversations = "list_user_conversations"
	ToolReadConversation      = "read_conversation"
)

type userProfileTool struct {
	capability    application.AgentCapabilityV1
	assistantUUID string
}

type recentMessageSearchTool struct {
	capability    application.AgentCapabilityV1
	assistantUUID string
}

type systemMessageTool struct {
	capability    application.AgentCapabilityV1
	assistantUUID string
}

type listUserConversationsTool struct {
	capability application.AgentCapabilityV1
}

type readConversationTool struct {
	capability application.AgentCapabilityV1
}

type toolExecutionState struct {
	mu           sync.Mutex
	sentMessages []*model.Message
}

type toolExecutionStateKey struct{}

// input types

type getUserProfileInput struct {
}

type searchRecentMessagesInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type sendSystemMessageInput struct {
	Content string `json:"content"`
}

type listUserConversationsInput struct {
	Limit int `json:"limit"`
}

type readConversationInput struct {
	TargetUUID string `json:"target_uuid"`
	Limit      int    `json:"limit"`
}

// result types

type toolUserProfile struct {
	Found    bool   `json:"found"`
	UUID     string `json:"uuid,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	UserType int8   `json:"user_type,omitempty"`
	Status   int8   `json:"status,omitempty"`
}

type toolRecentMessageSearchResult struct {
	Found   bool               `json:"found"`
	Query   string             `json:"query"`
	Matches []toolMessageMatch `json:"matches,omitempty"`
}

type toolMessageMatch struct {
	MessageType int8            `json:"message_type"`
	Role        schema.RoleType `json:"role"`
	Content     string          `json:"content"`
}

type toolSendMessageResult struct {
	Sent        bool   `json:"sent"`
	MessageUUID string `json:"message_uuid,omitempty"`
	TargetUUID  string `json:"target_uuid,omitempty"`
	MessageType int8   `json:"message_type,omitempty"`
}

type toolConversationSummary struct {
	TargetUUID    string    `json:"target_uuid"`
	TargetType    string    `json:"target_type"`
	LastPreview   string    `json:"last_preview"`
	UnreadCount   int       `json:"unread_count"`
	LastMessageAt time.Time `json:"last_message_at"`
}

type toolListConversationsResult struct {
	Count         int                       `json:"count"`
	Conversations []toolConversationSummary `json:"conversations"`
}

type toolConversationMessage struct {
	SenderUUID  string    `json:"sender_uuid"`
	IsSelf      bool      `json:"is_self"`
	MessageType int8      `json:"message_type"`
	Content     string    `json:"content"`
	SentAt      time.Time `json:"sent_at"`
}

type toolReadConversationResult struct {
	Found        bool                      `json:"found"`
	Reason       string                    `json:"reason,omitempty"`
	TargetUUID   string                    `json:"target_uuid"`
	TargetType   string                    `json:"target_type"`
	MessageCount int                       `json:"message_count"`
	Messages     []toolConversationMessage `json:"messages,omitempty"`
}

// constructors

func NewTools(capability application.AgentCapabilityV1, assistantUUID string) []einoTool.BaseTool {
	if capability == nil {
		return nil
	}
	return []einoTool.BaseTool{
		NewUserProfileTool(capability, assistantUUID),
		NewRecentMessageSearchTool(capability, assistantUUID),
		NewListUserConversationsTool(capability),
		NewReadConversationTool(capability),
		NewSystemMessageTool(capability, assistantUUID),
	}
}

func NewUserProfileTool(capability application.AgentCapabilityV1, assistantUUID string) einoTool.BaseTool {
	return &userProfileTool{capability: capability, assistantUUID: strings.TrimSpace(assistantUUID)}
}

func NewRecentMessageSearchTool(capability application.AgentCapabilityV1, assistantUUID string) einoTool.BaseTool {
	return &recentMessageSearchTool{
		capability:    capability,
		assistantUUID: strings.TrimSpace(assistantUUID),
	}
}

func NewSystemMessageTool(capability application.AgentCapabilityV1, assistantUUID string) einoTool.BaseTool {
	return &systemMessageTool{
		capability:    capability,
		assistantUUID: strings.TrimSpace(assistantUUID),
	}
}

func NewListUserConversationsTool(capability application.AgentCapabilityV1) einoTool.BaseTool {
	return &listUserConversationsTool{capability: capability}
}

func NewReadConversationTool(capability application.AgentCapabilityV1) einoTool.BaseTool {
	return &readConversationTool{capability: capability}
}

// tool Info and InvokableRun implementations

func (t *userProfileTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        ToolGetUserProfile,
		Desc:        "Get the current user's concise profile. Use it when you need nickname, avatar, user type, or current user status.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (t *userProfileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = opts

	if t == nil || t.capability == nil {
		return "", errors.New("user profile tool is not initialized")
	}

	var input getUserProfileInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode get_user_profile input: %w", err)
	}

	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return "", err
	}

	user, err := t.capability.GetUserProfile(ctx, execution.PrincipalUserUUID, execution.AgentUUID, execution.PrincipalUserUUID)
	if err != nil {
		return "", fmt.Errorf("get user profile: %w", err)
	}

	result := toolUserProfile{Found: false}
	if user != nil {
		result = *toToolUserProfile(user)
	}

	return marshalToolResult(result)
}

func (t *recentMessageSearchTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "search_recent_messages",
		Desc: "Search recent direct conversation messages between the end user and the Dipole AI assistant by keyword. Use it when the user asks you to recall something said earlier.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "The keyword or short phrase to search in recent messages.",
				Required: true,
			},
			"limit": {
				Type: schema.Integer,
				Desc: "Maximum number of matches to return. Default is 5 and maximum is 10.",
			},
		}),
	}, nil
}

func (t *recentMessageSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = opts

	if t == nil || t.capability == nil {
		return "", errors.New("recent message search tool is not initialized")
	}
	if strings.TrimSpace(t.assistantUUID) == "" {
		return "", errors.New("recent message search tool assistant uuid is empty")
	}

	var input searchRecentMessagesInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode search_recent_messages input: %w", err)
	}

	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return "", errors.New("query is required")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	items, err := t.capability.ListDirectMessages(ctx, execution.PrincipalUserUUID, t.assistantUUID, 50)
	if err != nil {
		return "", fmt.Errorf("list recent messages: %w", err)
	}

	lowerQuery := strings.ToLower(query)
	result := toolRecentMessageSearchResult{
		Found:   false,
		Query:   query,
		Matches: make([]toolMessageMatch, 0, limit),
	}
	for i := len(items) - 1; i >= 0 && len(result.Matches) < limit; i-- {
		item := items[i]
		content := strings.TrimSpace(renderMessageContent(item))
		if content == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(content), lowerQuery) {
			continue
		}

		role := schema.User
		if strings.TrimSpace(item.SenderUUID) == strings.TrimSpace(t.assistantUUID) {
			role = schema.Assistant
		}
		result.Matches = append(result.Matches, toolMessageMatch{
			MessageType: item.MessageType,
			Role:        role,
			Content:     content,
		})
	}
	result.Found = len(result.Matches) > 0

	return marshalToolResult(result)
}

func (t *systemMessageTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "send_system_message",
		Desc: "Send a system message to the current user when you intentionally need to deliver an explicit system-style notification.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"content": {
				Type:     schema.String,
				Desc:     "The system message content to send. Maximum 500 characters.",
				Required: true,
			},
		}),
	}, nil
}

func (t *systemMessageTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = opts

	if t == nil || t.capability == nil {
		return "", errors.New("system message tool is not initialized")
	}
	if strings.TrimSpace(t.assistantUUID) == "" {
		return "", errors.New("system message tool assistant uuid is empty")
	}

	var input sendSystemMessageInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode send_system_message input: %w", err)
	}

	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	if execution.AgentUUID != t.assistantUUID {
		return "", errors.New("agent execution context does not match system message sender")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return "", errors.New("content is required")
	}
	if len([]rune(content)) > 500 {
		content = string([]rune(content)[:500])
	}

	message, err := t.capability.SendSystemMessage(ctx, t.assistantUUID, execution.PrincipalUserUUID, content)
	if err != nil {
		return "", fmt.Errorf("send system message: %w", err)
	}
	recordToolSentMessage(ctx, message)

	return marshalToolResult(toolSendMessageResult{
		Sent:        message != nil,
		MessageUUID: message.UUID,
		TargetUUID:  execution.PrincipalUserUUID,
		MessageType: model.MessageTypeSystem,
	})
}

func (t *listUserConversationsTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ToolListUserConversations,
		Desc: "List the user's recent conversations with a short preview of the last message. Use this to discover which conversations are available before reading one.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"limit": {
				Type: schema.Integer,
				Desc: "Maximum number of conversations to return. Default is 10, maximum is 20.",
			},
		}),
	}, nil
}

func (t *listUserConversationsTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = opts

	if t == nil || t.capability == nil {
		return "", errors.New("list user conversations tool is not initialized")
	}

	var input listUserConversationsInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode list_user_conversations input: %w", err)
	}

	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return "", err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	convs, err := t.capability.ListConversations(ctx, execution.PrincipalUserUUID, limit)
	if err != nil {
		return "", fmt.Errorf("list user conversations: %w", err)
	}

	summaries := make([]toolConversationSummary, 0, len(convs))
	for _, c := range convs {
		if c == nil {
			continue
		}
		targetType := "direct"
		if c.TargetType == model.MessageTargetGroup {
			targetType = "group"
		}
		summaries = append(summaries, toolConversationSummary{
			TargetUUID:    c.TargetUUID,
			TargetType:    targetType,
			LastPreview:   c.LastMessagePreview,
			UnreadCount:   c.UnreadCount,
			LastMessageAt: c.LastMessageAt,
		})
	}

	return marshalToolResult(toolListConversationsResult{
		Count:         len(summaries),
		Conversations: summaries,
	})
}

func (t *readConversationTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ToolReadConversation,
		Desc: "Read recent messages from one of the user's conversations. Only conversations the user participates in are accessible. Use list_user_conversations first to discover available target UUIDs.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"target_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the other participant (user UUID for direct, group UUID for group).",
				Required: true,
			},
			"limit": {
				Type: schema.Integer,
				Desc: "Number of recent messages to return. Default is 20, maximum is 50.",
			},
		}),
	}, nil
}

func (t *readConversationTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = opts

	if t == nil || t.capability == nil {
		return "", errors.New("read conversation tool is not initialized")
	}

	var input readConversationInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode read_conversation input: %w", err)
	}

	execution, err := requireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	userUUID := execution.PrincipalUserUUID
	targetUUID := strings.TrimSpace(input.TargetUUID)
	if targetUUID == "" {
		return "", errors.New("target_uuid is required")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	targetType := "direct"
	if strings.HasPrefix(targetUUID, "G") {
		targetType = "group"
	}

	read, err := t.capability.ReadConversation(ctx, userUUID, targetUUID, limit)
	if err != nil {
		return "", fmt.Errorf("read conversation capability: %w", err)
	}
	if read == nil || !read.Found {
		reason := "conversation_not_found_or_not_accessible"
		if read != nil && strings.TrimSpace(read.Reason) != "" {
			reason = read.Reason
		}
		return marshalToolResult(toolReadConversationResult{
			Found:      false,
			Reason:     reason,
			TargetUUID: targetUUID,
			TargetType: targetType,
		})
	}

	if read.TargetType == model.MessageTargetGroup {
		targetType = "group"
	}

	messages := make([]toolConversationMessage, 0, len(read.Messages))
	for _, item := range read.Messages {
		if item == nil {
			continue
		}
		content := strings.TrimSpace(renderMessageContent(item))
		if content == "" {
			continue
		}
		messages = append(messages, toolConversationMessage{
			SenderUUID:  item.SenderUUID,
			IsSelf:      strings.TrimSpace(item.SenderUUID) == userUUID,
			MessageType: item.MessageType,
			Content:     content,
			SentAt:      item.SentAt,
		})
	}

	return marshalToolResult(toolReadConversationResult{
		Found:        true,
		TargetUUID:   targetUUID,
		TargetType:   targetType,
		MessageCount: len(messages),
		Messages:     messages,
	})
}

// helpers

func toToolUserProfile(user *model.User) *toolUserProfile {
	if user == nil {
		return nil
	}

	return &toolUserProfile{
		Found:    true,
		UUID:     user.UUID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		UserType: user.UserType,
		Status:   user.Status,
	}
}

func marshalToolResult(result any) (string, error) {
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal tool result: %w", err)
	}
	return string(payload), nil
}

func withToolExecutionState(ctx context.Context, state *toolExecutionState) context.Context {
	if state == nil {
		return ctx
	}
	return context.WithValue(ctx, toolExecutionStateKey{}, state)
}

func getToolExecutionState(ctx context.Context) *toolExecutionState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(toolExecutionStateKey{}).(*toolExecutionState)
	return state
}

func recordToolSentMessage(ctx context.Context, message *model.Message) {
	if message == nil {
		return
	}
	state := getToolExecutionState(ctx)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.sentMessages = append(state.sentMessages, message)
}

func latestToolSentMessage(ctx context.Context) *model.Message {
	state := getToolExecutionState(ctx)
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.sentMessages) == 0 {
		return nil
	}
	return state.sentMessages[len(state.sentMessages)-1]
}
