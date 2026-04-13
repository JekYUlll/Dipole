package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/model"
)

const (
	ToolGetUserProfile         = "get_user_profile"
	ToolGetConversationContext = "get_conversation_context"
)

type userProfileTool struct {
	users userReader
}

type conversationContextTool struct {
	builder       directContextBuilder
	assistantUUID string
}

type getUserProfileInput struct {
	UserUUID string `json:"user_uuid"`
}

type getConversationContextInput struct {
	UserUUID string `json:"user_uuid"`
}

type toolUserProfile struct {
	Found    bool   `json:"found"`
	UUID     string `json:"uuid,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	UserType int8   `json:"user_type,omitempty"`
	Status   int8   `json:"status,omitempty"`
}

type toolConversationContext struct {
	Found        bool               `json:"found"`
	Reason       string             `json:"reason,omitempty"`
	EndUser      *toolUserProfile   `json:"end_user,omitempty"`
	Assistant    *toolUserProfile   `json:"assistant,omitempty"`
	MessageCount int                `json:"message_count"`
	Messages     []toolMessageEntry `json:"messages,omitempty"`
}

type toolMessageEntry struct {
	Role    schema.RoleType `json:"role"`
	Content string          `json:"content"`
}

func NewTools(builder directContextBuilder, users userReader, assistantUUID string) []einoTool.BaseTool {
	tools := make([]einoTool.BaseTool, 0, 2)
	if users != nil {
		tools = append(tools, NewUserProfileTool(users))
	}
	if builder != nil {
		tools = append(tools, NewConversationContextTool(builder, assistantUUID))
	}
	return tools
}

func NewUserProfileTool(users userReader) einoTool.BaseTool {
	return &userProfileTool{users: users}
}

func NewConversationContextTool(builder directContextBuilder, assistantUUID string) einoTool.BaseTool {
	return &conversationContextTool{
		builder:       builder,
		assistantUUID: strings.TrimSpace(assistantUUID),
	}
}

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

func (t *conversationContextTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ToolGetConversationContext,
		Desc: "Get the recent direct conversation context between the end user and the Dipole AI assistant. Use it when you need the latest exchanged messages before answering.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"user_uuid": {
				Type:     schema.String,
				Desc:     "The UUID of the end user in the AI direct conversation.",
				Required: true,
			},
		}),
	}, nil
}

func (t *conversationContextTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einoTool.Option) (string, error) {
	_ = opts

	if t == nil || t.builder == nil {
		return "", errors.New("conversation context tool is not initialized")
	}
	if strings.TrimSpace(t.assistantUUID) == "" {
		return "", errors.New("conversation context tool assistant uuid is empty")
	}

	var input getConversationContextInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("decode get_conversation_context input: %w", err)
	}

	userUUID := strings.TrimSpace(input.UserUUID)
	if userUUID == "" {
		return "", errors.New("user_uuid is required")
	}

	conversation, err := t.builder.BuildDirectContext(userUUID, t.assistantUUID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAIUserNotFound):
			return marshalToolResult(toolConversationContext{
				Found:  false,
				Reason: "user_not_found",
			})
		case errors.Is(err, ErrAIAssistantNotFound):
			return marshalToolResult(toolConversationContext{
				Found:  false,
				Reason: "assistant_not_found",
			})
		default:
			return "", fmt.Errorf("get conversation context: %w", err)
		}
	}

	result := toolConversationContext{
		Found:        true,
		EndUser:      toToolUserProfile(conversation.EndUser),
		Assistant:    toToolUserProfile(conversation.Assistant),
		MessageCount: len(conversation.Messages),
		Messages:     make([]toolMessageEntry, 0, len(conversation.Messages)),
	}
	for _, message := range conversation.Messages {
		if message == nil {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		result.Messages = append(result.Messages, toolMessageEntry{
			Role:    message.Role,
			Content: content,
		})
	}

	return marshalToolResult(result)
}

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
