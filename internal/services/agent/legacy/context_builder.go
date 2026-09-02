package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type ContextBuilder struct {
	capability         application.AgentCapabilityV1
	maxContextMessages int
}

type ConversationContext struct {
	EndUser   *model.User
	Assistant *model.User
	Messages  []*schema.Message
}

func NewContextBuilder(capability application.AgentCapabilityV1, maxContextMessages int) *ContextBuilder {
	if maxContextMessages <= 0 {
		maxContextMessages = 12
	}

	return &ContextBuilder{
		capability:         capability,
		maxContextMessages: maxContextMessages,
	}
}

func (b *ContextBuilder) BuildDirectContext(ctx context.Context, userUUID, assistantUUID string) (*ConversationContext, error) {
	userUUID = strings.TrimSpace(userUUID)
	assistantUUID = strings.TrimSpace(assistantUUID)
	execution, err := requireAuthorizedExecutionForResource(ctx, application.AgentCapabilityUserProfileRead, application.AgentResourceTypeUser, userUUID, application.AgentResourceActionRead)
	if err != nil {
		return nil, fmt.Errorf("authorize ai context profile: %w", err)
	}
	if execution.PrincipalUserUUID != userUUID || execution.AgentUUID != assistantUUID {
		return nil, application.ErrAgentCapabilityDenied
	}
	invocation := execution.invocationV1()
	endUser, err := b.capability.GetUserProfile(ctx, invocation, userUUID)
	if err != nil {
		return nil, fmt.Errorf("get ai conversation user: %w", err)
	}
	if endUser == nil {
		return nil, ErrAIUserNotFound
	}

	assistant, err := b.capability.GetUserProfile(ctx, invocation, assistantUUID)
	if err != nil {
		return nil, fmt.Errorf("get ai assistant user: %w", err)
	}
	if assistant == nil || !assistant.IsAssistant() {
		return nil, ErrAIAssistantNotFound
	}
	if err := authorizeExecutionCapabilityForResource(execution, application.AgentCapabilityDirectMessagesRead, application.AgentResourceTypeConversation, model.DirectConversationKey(userUUID, assistantUUID), application.AgentResourceActionRead); err != nil {
		return nil, fmt.Errorf("authorize ai context messages: %w", err)
	}

	items, err := b.capability.ListDirectMessages(ctx, invocation, b.maxContextMessages)
	if err != nil {
		return nil, fmt.Errorf("list ai conversation messages: %w", err)
	}

	messages := make([]*schema.Message, 0, len(items))
	for _, item := range items {
		if msg := buildSchemaMessage(item, assistant.UUID); msg != nil {
			messages = append(messages, msg)
		}
	}

	return &ConversationContext{
		EndUser:   endUser,
		Assistant: assistant,
		Messages:  messages,
	}, nil
}

func buildSchemaMessage(message *model.Message, assistantUUID string) *schema.Message {
	if message == nil {
		return nil
	}

	content := renderMessageContent(message)
	if content == "" {
		return nil
	}

	if strings.TrimSpace(message.SenderUUID) == strings.TrimSpace(assistantUUID) {
		return schema.AssistantMessage(content, nil)
	}

	return schema.UserMessage(content)
}

func renderMessageContent(message *model.Message) string {
	switch message.MessageType {
	case model.MessageTypeText, model.MessageTypeAIText:
		return strings.TrimSpace(message.Content)
	case model.MessageTypeFile:
		parts := []string{"[file]"}
		if name := strings.TrimSpace(message.FileName); name != "" {
			parts = append(parts, name)
		}
		if message.FileSize > 0 {
			parts = append(parts, fmt.Sprintf("(%d bytes)", message.FileSize))
		}
		if ct := strings.TrimSpace(message.FileContentType); ct != "" {
			parts = append(parts, "content-type:"+ct)
		}
		if url := strings.TrimSpace(message.FileURL); url != "" {
			parts = append(parts, "url:"+url)
		}
		return strings.Join(parts, " ")
	case model.MessageTypeSystem:
		content := strings.TrimSpace(message.Content)
		if content == "" {
			return ""
		}
		return "[system] " + content
	default:
		return ""
	}
}
