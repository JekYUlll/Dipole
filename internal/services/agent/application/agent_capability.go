package agentapplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type AgentCapabilityMessages interface {
	ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
}

type AgentCapabilityConversations interface {
	ListForAgent(userUUID string, limit int) ([]*model.Conversation, error)
	FindForUser(userUUID, targetUUID string) (*model.Conversation, error)
}

type LocalAgentCapabilityV1 struct {
	core          application.CoreCapability
	messages      AgentCapabilityMessages
	conversations AgentCapabilityConversations
	commands      application.AgentCommandV1
}

var _ application.AgentCapabilityV1 = (*LocalAgentCapabilityV1)(nil)

func NewLocalAgentCapabilityV1(core application.CoreCapability, messages AgentCapabilityMessages, conversations AgentCapabilityConversations, commands application.AgentCommandV1) (*LocalAgentCapabilityV1, error) {
	if core == nil {
		return nil, errors.New("Agent Capability Core dependency is required")
	}
	if messages == nil {
		return nil, errors.New("Agent Capability Message dependency is required")
	}
	if conversations == nil {
		return nil, errors.New("Agent Capability Conversation dependency is required")
	}
	if commands == nil {
		return nil, errors.New("Agent Capability Command dependency is required")
	}
	return &LocalAgentCapabilityV1{core: core, messages: messages, conversations: conversations, commands: commands}, nil
}

func (c *LocalAgentCapabilityV1) GetUserProfile(_ context.Context, invocation application.AgentInvocationV1, subjectUUID string) (*model.User, error) {
	subjectUUID = strings.TrimSpace(subjectUUID)
	if err := authorizeLocalAgentCapabilityForResourceV1(invocation, application.AgentCapabilityUserProfileRead, application.AgentResourceTypeUser, subjectUUID, application.AgentResourceActionRead); err != nil {
		return nil, err
	}
	principalUUID := strings.TrimSpace(invocation.PrincipalUUID)
	agentUUID := strings.TrimSpace(invocation.AgentUUID)
	if principalUUID == "" || agentUUID == "" || subjectUUID == "" || (subjectUUID != principalUUID && subjectUUID != agentUUID) {
		return nil, application.ErrAgentCapabilityDenied
	}
	user, err := c.core.GetUserByUUID(subjectUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Capability user profile: %w", err)
	}
	return user, nil
}

func (c *LocalAgentCapabilityV1) ListDirectMessages(_ context.Context, invocation application.AgentInvocationV1, limit int) ([]*model.Message, error) {
	conversationKey := model.DirectConversationKey(invocation.PrincipalUUID, invocation.AgentUUID)
	if err := authorizeLocalAgentCapabilityForResourceV1(invocation, application.AgentCapabilityDirectMessagesRead, application.AgentResourceTypeConversation, conversationKey, application.AgentResourceActionRead); err != nil {
		return nil, err
	}
	principalUUID := strings.TrimSpace(invocation.PrincipalUUID)
	agentUUID := strings.TrimSpace(invocation.AgentUUID)
	items, err := c.messages.ListDirectMessages(principalUUID, agentUUID, 0, limit)
	if err != nil {
		return nil, fmt.Errorf("list Agent direct messages: %w", err)
	}
	return items, nil
}

func (c *LocalAgentCapabilityV1) ListConversations(_ context.Context, invocation application.AgentInvocationV1, limit int) ([]*model.Conversation, error) {
	if err := authorizeLocalAgentCapabilityForResourceV1(invocation, application.AgentCapabilityConversationsList, application.AgentResourceTypeConversation, application.AgentResourceWildcard, application.AgentResourceActionList); err != nil {
		return nil, err
	}
	principalUUID := strings.TrimSpace(invocation.PrincipalUUID)
	items, err := c.conversations.ListForAgent(principalUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Agent conversations: %w", err)
	}
	return items, nil
}

func (c *LocalAgentCapabilityV1) ReadConversation(_ context.Context, invocation application.AgentInvocationV1, targetUUID string, limit int) (*application.AgentConversationReadV1, error) {
	if err := authorizeLocalAgentCapabilityV1(invocation, application.AgentCapabilityConversationRead); err != nil {
		return nil, err
	}
	principalUUID := strings.TrimSpace(invocation.PrincipalUUID)
	targetUUID = strings.TrimSpace(targetUUID)
	if principalUUID == "" || targetUUID == "" {
		return nil, application.ErrAgentCapabilityDenied
	}
	conversation, err := c.conversations.FindForUser(principalUUID, targetUUID)
	if err != nil {
		return nil, fmt.Errorf("authorize Agent conversation read: %w", err)
	}
	if conversation == nil {
		return &application.AgentConversationReadV1{
			Found: false, Reason: "conversation_not_found_or_not_accessible", TargetUUID: targetUUID,
		}, nil
	}
	conversationKey := strings.TrimSpace(conversation.ConversationKey)
	if conversationKey == "" {
		if conversation.TargetType == model.MessageTargetGroup {
			conversationKey = model.GroupConversationKey(targetUUID)
		} else {
			conversationKey = model.DirectConversationKey(principalUUID, targetUUID)
		}
	}
	if err := authorizeLocalAgentCapabilityForResourceV1(invocation, application.AgentCapabilityConversationRead, application.AgentResourceTypeConversation, conversationKey, application.AgentResourceActionRead); err != nil {
		return nil, err
	}

	var messages []*model.Message
	if conversation.TargetType == model.MessageTargetGroup {
		messages, err = c.messages.ListGroupMessages(principalUUID, targetUUID, 0, limit)
	} else {
		messages, err = c.messages.ListDirectMessages(principalUUID, targetUUID, 0, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("read Agent conversation: %w", err)
	}
	return &application.AgentConversationReadV1{
		Found: true, TargetUUID: targetUUID, TargetType: conversation.TargetType, Messages: messages,
	}, nil
}

func (c *LocalAgentCapabilityV1) SendSystemMessage(ctx context.Context, invocation application.AgentInvocationV1, content string) (*model.Message, error) {
	conversationKey := model.DirectConversationKey(invocation.PrincipalUUID, invocation.AgentUUID)
	if err := authorizeLocalAgentCapabilityForResourceV1(invocation, application.AgentCapabilitySystemMessageSend, application.AgentResourceTypeConversation, conversationKey, application.AgentResourceActionWrite); err != nil {
		return nil, err
	}
	commandID := agentCapabilitySystemCommandIDV1(invocation, content)
	message, err := c.commands.SendMessage(ctx, application.AgentMessageCommandV1{
		CommandID: commandID, Kind: application.AgentMessageCommandSystemMessageV1,
		Invocation: invocation, Content: content,
	})
	if err != nil {
		return nil, fmt.Errorf("send Agent system message: %w", err)
	}
	return message, nil
}

func authorizeLocalAgentCapabilityForResourceV1(invocation application.AgentInvocationV1, capabilityID, resourceType, resourceID, action string) error {
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(capabilityID)
	if !ok {
		return application.ErrAgentCapabilityDenied
	}
	return application.AuthorizeAgentCapabilityForResourceV1(invocation, descriptor, resourceType, resourceID, action)
}

func agentCapabilitySystemCommandIDV1(invocation application.AgentInvocationV1, content string) string {
	canonical := strings.TrimSpace(invocation.AgentUUID) + "\n" + strings.TrimSpace(invocation.PrincipalUUID) + "\n" +
		strings.TrimSpace(invocation.EventID) + "\n" + strings.TrimSpace(content)
	digest := sha256.Sum256([]byte(canonical))
	return "system:" + hex.EncodeToString(digest[:])
}

func authorizeLocalAgentCapabilityV1(invocation application.AgentInvocationV1, capabilityID string) error {
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(capabilityID)
	if !ok {
		return application.ErrAgentCapabilityDenied
	}
	return application.AuthorizeAgentCapabilityV1(invocation, descriptor)
}
