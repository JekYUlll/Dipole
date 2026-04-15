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

	"github.com/JekYUlll/Dipole/internal/model"
)

const (
	ToolGetUserProfile          = "get_user_profile"
	ToolListUserConversations   = "list_user_conversations"
	ToolReadConversation        = "read_conversation"
)

type userProfileTool struct {
	users userReader
}

type recentMessageSearchTool struct {
	messages      messageReader
	assistantUUID string
}

type systemMessageTool struct {
	sender        systemMessageSender
	assistantUUID string
}

type listUserConversationsTool struct {
	conversations conversationReader
}

type readConversationTool struct {
	conversations conversationReader
	messages      messageReader
}

type toolExecutionState struct {
	mu           sync.Mutex
	sentMessages []*model.Message
}

type toolExecutionStateKey struct{}

// interfaces

type conversationReader interface {
	ListByUserUUID(userUUID string, limit int) ([]*model.Conversation, error)
	GetByUserAndConversationKey(userUUID, conversationKey string) (*model.Conversation, error)
}

type systemMessageSender interface {
	SendSystemDirectMessage(senderUUID, targetUUID, content string) (*model.Message, error)
}

// input types

type getUserProfileInput struct {
	UserUUID string `json:"user_uuid"`
}

type searchRecentMessagesInput struct {
	UserUUID string `json:"user_uuid"`
	Query    string `json:"query"`
	Limit    int    `json:"limit"`
}

type sendSystemMessageInput struct {
	UserUUID string `json:"user_uuid"`
	Content  string `json:"content"`
}

type listUserConversationsInput struct {
	UserUUID string `json:"user_uuid"`
	Limit    int    `json:"limit"`
}

type readConversationInput struct {
	UserUUID   string `json:"user_uuid"`
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

func NewTools(users userReader, messages messageReader, conversations conversationReader, sender systemMessageSender, assistantUUID string) []einoTool.BaseTool {
	tools := make([]einoTool.BaseTool, 0, 5)
	if users != nil {
		tools = append(tools, NewUserProfileTool(users))
	}
	if messages != nil {
		tools = append(tools, NewRecentMessageSearchTool(messages, assistantUUID))
	}
	if conversations != nil {
		tools = append(tools, NewListUserConversationsTool(conversations))
		if messages != nil {
			tools = append(tools, NewReadConversationTool(conversations, messages))
		}
	}
	if sender != nil {
		tools = append(tools, NewSystemMessageTool(sender, assistantUUID))
	}
	return tools
}

func NewUserProfileTool(users userReader) einoTool.BaseTool {
	return &userProfileTool{users: users}
}

func NewRecentMessageSearchTool(messages messageReader, assistantUUID string) einoTool.BaseTool {
	return &recentMessageSearchTool{
		messages:      messages,
		assistantUUID: strings.TrimSpace(assistantUUID),
	}
}

func NewSystemMessageTool(sender systemMessageSender, assistantUUID string) einoTool.BaseTool {
	return &systemMessageTool{
		sender:        sender,
		assistantUUID: strings.TrimSpace(assistantUUID),
	}
}

func NewListUserConversationsTool(conversations conversationReader) einoTool.BaseTool {
	return &listUserConversationsTool{conversations: conversations}
}

func NewReadConversationTool(conversations conversationReader, messages messageReader) einoTool.BaseTool {
	return &readConversationTool{conversations: conversations, messages: messages}
}

// tool Info and InvokableRun implementations

func (t *userProfileTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ToolGetUserProfile,
		Desc: "Get a concise user profile by user UUID. Use it when you need nickname, avatar, user type, or current user status.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"user_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the user you want to inspect.",
				Required: true,
			},
		}),
	}, nil
}

func (t *userProfileTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = ctx
	_ = opts

	if t == nil || t.users == nil {
		return "", errors.New("user profile tool is not initialized")
	}

	var input getUserProfileInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode get_user_profile input: %w", err)
	}

	userUUID := strings.TrimSpace(input.UserUUID)
	if userUUID == "" {
		return "", errors.New("user_uuid is required")
	}

	user, err := t.users.GetByUUID(userUUID)
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
			"user_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the end user in the AI direct conversation.",
				Required: true,
			},
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
	_ = ctx
	_ = opts

	if t == nil || t.messages == nil {
		return "", errors.New("recent message search tool is not initialized")
	}
	if strings.TrimSpace(t.assistantUUID) == "" {
		return "", errors.New("recent message search tool assistant uuid is empty")
	}

	var input searchRecentMessagesInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode search_recent_messages input: %w", err)
	}

	userUUID := strings.TrimSpace(input.UserUUID)
	query := strings.TrimSpace(input.Query)
	if userUUID == "" {
		return "", errors.New("user_uuid is required")
	}
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

	items, err := t.messages.ListByConversationKey(model.DirectConversationKey(userUUID, t.assistantUUID), 0, 50)
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
			"user_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the target user.",
				Required: true,
			},
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

	if t == nil || t.sender == nil {
		return "", errors.New("system message tool is not initialized")
	}
	if strings.TrimSpace(t.assistantUUID) == "" {
		return "", errors.New("system message tool assistant uuid is empty")
	}

	var input sendSystemMessageInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode send_system_message input: %w", err)
	}

	userUUID := strings.TrimSpace(input.UserUUID)
	content := strings.TrimSpace(input.Content)
	if userUUID == "" {
		return "", errors.New("user_uuid is required")
	}
	if content == "" {
		return "", errors.New("content is required")
	}
	if len([]rune(content)) > 500 {
		content = string([]rune(content)[:500])
	}

	message, err := t.sender.SendSystemDirectMessage(t.assistantUUID, userUUID, content)
	if err != nil {
		return "", fmt.Errorf("send system message: %w", err)
	}
	recordToolSentMessage(ctx, message)

	return marshalToolResult(toolSendMessageResult{
		Sent:        message != nil,
		MessageUUID: message.UUID,
		TargetUUID:  userUUID,
		MessageType: model.MessageTypeSystem,
	})
}

func (t *listUserConversationsTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ToolListUserConversations,
		Desc: "List the user's recent conversations with a short preview of the last message. Use this to discover which conversations are available before reading one.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"user_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the user whose conversations to list.",
				Required: true,
			},
			"limit": {
				Type: schema.Integer,
				Desc: "Maximum number of conversations to return. Default is 10, maximum is 20.",
			},
		}),
	}, nil
}

func (t *listUserConversationsTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = ctx
	_ = opts

	if t == nil || t.conversations == nil {
		return "", errors.New("list user conversations tool is not initialized")
	}

	var input listUserConversationsInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode list_user_conversations input: %w", err)
	}

	userUUID := strings.TrimSpace(input.UserUUID)
	if userUUID == "" {
		return "", errors.New("user_uuid is required")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	convs, err := t.conversations.ListByUserUUID(userUUID, limit)
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
			"user_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the user requesting the read.",
				Required: true,
			},
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
	_ = ctx
	_ = opts

	if t == nil || t.conversations == nil || t.messages == nil {
		return "", errors.New("read conversation tool is not initialized")
	}

	var input readConversationInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode read_conversation input: %w", err)
	}

	userUUID := strings.TrimSpace(input.UserUUID)
	targetUUID := strings.TrimSpace(input.TargetUUID)
	if userUUID == "" {
		return "", errors.New("user_uuid is required")
	}
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

	// Determine conversation key and type from target UUID prefix.
	var conversationKey string
	targetType := "direct"
	if strings.HasPrefix(targetUUID, "G") {
		conversationKey = model.GroupConversationKey(targetUUID)
		targetType = "group"
	} else {
		conversationKey = model.DirectConversationKey(userUUID, targetUUID)
	}

	// Permission check: verify the user is a participant in this conversation.
	conv, err := t.conversations.GetByUserAndConversationKey(userUUID, conversationKey)
	if err != nil {
		return "", fmt.Errorf("check conversation access: %w", err)
	}
	if conv == nil {
		return marshalToolResult(toolReadConversationResult{
			Found:      false,
			Reason:     "conversation_not_found_or_not_accessible",
			TargetUUID: targetUUID,
			TargetType: targetType,
		})
	}

	items, err := t.messages.ListByConversationKey(conversationKey, 0, limit)
	if err != nil {
		return "", fmt.Errorf("read conversation messages: %w", err)
	}

	messages := make([]toolConversationMessage, 0, len(items))
	for _, item := range items {
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

