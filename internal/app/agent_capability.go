package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type agentCapabilityMessages interface {
	ListDirectMessages(currentUserUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error)
	ListGroupMessages(currentUserUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error)
	SendSystemDirectMessageContext(ctx context.Context, senderUUID, targetUUID, content string) (*model.Message, error)
}

type agentCapabilityConversations interface {
	ListForAgent(userUUID string, limit int) ([]*model.Conversation, error)
	FindForUser(userUUID, targetUUID string) (*model.Conversation, error)
}

type LocalAgentCapabilityV1 struct {
	core          application.CoreCapability
	messages      agentCapabilityMessages
	conversations agentCapabilityConversations
}

var _ application.AgentCapabilityV1 = (*LocalAgentCapabilityV1)(nil)

func NewLocalAgentCapabilityV1(core application.CoreCapability, messages agentCapabilityMessages, conversations agentCapabilityConversations) (*LocalAgentCapabilityV1, error) {
	if core == nil {
		return nil, errors.New("Agent Capability Core dependency is required")
	}
	if messages == nil {
		return nil, errors.New("Agent Capability Message dependency is required")
	}
	if conversations == nil {
		return nil, errors.New("Agent Capability Conversation dependency is required")
	}
	return &LocalAgentCapabilityV1{core: core, messages: messages, conversations: conversations}, nil
}

func (c *LocalAgentCapabilityV1) GetUserProfile(_ context.Context, principalUUID, agentUUID, subjectUUID string) (*model.User, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	agentUUID = strings.TrimSpace(agentUUID)
	subjectUUID = strings.TrimSpace(subjectUUID)
	if principalUUID == "" || agentUUID == "" || subjectUUID == "" || (subjectUUID != principalUUID && subjectUUID != agentUUID) {
		return nil, application.ErrAgentCapabilityDenied
	}
	user, err := c.core.GetUserByUUID(subjectUUID)
	if err != nil {
		return nil, fmt.Errorf("get Agent Capability user profile: %w", err)
	}
	return user, nil
}

func (c *LocalAgentCapabilityV1) ListDirectMessages(_ context.Context, principalUUID, agentUUID string, limit int) ([]*model.Message, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	agentUUID = strings.TrimSpace(agentUUID)
	if principalUUID == "" || agentUUID == "" {
		return nil, application.ErrAgentCapabilityDenied
	}
	items, err := c.messages.ListDirectMessages(principalUUID, agentUUID, 0, limit)
	if err != nil {
		return nil, fmt.Errorf("list Agent direct messages: %w", err)
	}
	return items, nil
}

func (c *LocalAgentCapabilityV1) ListConversations(_ context.Context, principalUUID string, limit int) ([]*model.Conversation, error) {
	principalUUID = strings.TrimSpace(principalUUID)
	if principalUUID == "" {
		return nil, application.ErrAgentCapabilityDenied
	}
	items, err := c.conversations.ListForAgent(principalUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Agent conversations: %w", err)
	}
	return items, nil
}

func (c *LocalAgentCapabilityV1) ReadConversation(_ context.Context, principalUUID, targetUUID string, limit int) (*application.AgentConversationReadV1, error) {
	principalUUID = strings.TrimSpace(principalUUID)
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

func (c *LocalAgentCapabilityV1) SendSystemMessage(ctx context.Context, agentUUID, principalUUID, content string) (*model.Message, error) {
	agentUUID = strings.TrimSpace(agentUUID)
	principalUUID = strings.TrimSpace(principalUUID)
	if agentUUID == "" || principalUUID == "" {
		return nil, application.ErrAgentCapabilityDenied
	}
	message, err := c.messages.SendSystemDirectMessageContext(ctx, agentUUID, principalUUID, content)
	if err != nil {
		return nil, fmt.Errorf("send Agent system message: %w", err)
	}
	return message, nil
}
