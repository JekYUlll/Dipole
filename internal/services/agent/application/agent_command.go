package agentapplication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

const maxAgentCommandIDLengthV1 = 128
const agentCommandReceiptRecoveryTimeoutV1 = 2 * time.Second

type AgentCommandMessages interface {
	SendAssistantTextMessageContext(ctx context.Context, assistantUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	SendSystemDirectMessageCommandContext(ctx context.Context, senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	// SendAssistantGroupMessageContext delivers an AI-text message from the
	// assistant into a group conversation. Route B/B2 group @-mention replies.
	SendAssistantGroupMessageContext(ctx context.Context, assistantUUID, groupUUID, content, clientMessageID string) (*model.Message, error)
	GetMessageCommandReceiptContext(ctx context.Context, senderUUID, clientMessageID string) (*application.MessageCommandReceipt, error)
}

type LocalAgentCommandV1 struct {
	messages AgentCommandMessages
}

var _ application.AgentCommandV1 = (*LocalAgentCommandV1)(nil)

func NewLocalAgentCommandV1(messages AgentCommandMessages) (*LocalAgentCommandV1, error) {
	if messages == nil {
		return nil, errors.New("Agent Command Message dependency is required")
	}
	return &LocalAgentCommandV1{messages: messages}, nil
}

func (c *LocalAgentCommandV1) SendMessage(ctx context.Context, command application.AgentMessageCommandV1) (*model.Message, error) {
	commandID := strings.TrimSpace(command.CommandID)
	content := strings.TrimSpace(command.Content)
	if commandID == "" || len(commandID) > maxAgentCommandIDLengthV1 || content == "" {
		return nil, application.ErrAgentCommandDenied
	}

	capabilityID, err := AgentCommandCapabilityIDV1(command.Kind)
	if err != nil {
		return nil, err
	}
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(capabilityID)
	if !ok {
		return nil, application.ErrAgentCommandDenied
	}
	// Route B/B2: a group reply targets the group conversation the trigger
	// mentioned (group:<uuid>); 1v1 replies target the owner's direct Agent
	// conversation. The scope check below pins the capability to that one
	// conversation so a group reply cannot widen into other conversations.
	conversationKey := strings.TrimSpace(command.ConversationKey)
	if conversationKey == "" {
		conversationKey = model.DirectConversationKey(command.Invocation.PrincipalUUID, command.Invocation.AgentUUID)
	}
	if err := application.AuthorizeAgentCapabilityForResourceV1(command.Invocation, descriptor, application.AgentResourceTypeConversation, conversationKey, application.AgentResourceActionWrite); err != nil {
		return nil, fmt.Errorf("%w: %w", application.ErrAgentCommandDenied, err)
	}

	agentUUID := strings.TrimSpace(command.Invocation.AgentUUID)
	principalUUID := strings.TrimSpace(command.Invocation.PrincipalUUID)
	ctx = correlation.WithContext(ctx, correlation.IDs{
		RequestID: strings.TrimSpace(command.Invocation.RequestID),
		TraceID:   strings.TrimSpace(command.Invocation.TraceID),
		EventID:   strings.TrimSpace(command.Invocation.EventID),
	})
	clientMessageID, err := application.AgentCommandClientMessageIDV1(command.Kind, commandID)
	if err != nil {
		return nil, err
	}

	var message *model.Message
	switch command.Kind {
	case application.AgentMessageCommandAssistantReplyV1:
		message, err = c.messages.SendAssistantTextMessageContext(ctx, agentUUID, principalUUID, content, clientMessageID)
	case application.AgentMessageCommandSystemMessageV1:
		message, err = c.messages.SendSystemDirectMessageCommandContext(ctx, agentUUID, principalUUID, content, clientMessageID)
	case application.AgentMessageCommandGroupReplyV1:
		groupUUID := strings.TrimPrefix(conversationKey, "group:")
		if groupUUID == "" || groupUUID == conversationKey {
			return nil, application.ErrAgentCommandDenied
		}
		message, err = c.messages.SendAssistantGroupMessageContext(ctx, agentUUID, groupUUID, content, clientMessageID)
	default:
		return nil, application.ErrAgentCommandDenied
	}
	if err == nil {
		if !agentCommandMessageMatchesV1(message, command.Kind, agentUUID, messageTargetForKind(command.Kind, agentUUID, principalUUID, conversationKey), conversationKey, content, clientMessageID) {
			return nil, application.ErrAgentCommandConflict
		}
		return message, nil
	}
	recoveryCtx, cancelRecovery := context.WithTimeout(context.WithoutCancel(ctx), agentCommandReceiptRecoveryTimeoutV1)
	defer cancelRecovery()
	receipt, receiptErr := c.messages.GetMessageCommandReceiptContext(recoveryCtx, agentUUID, clientMessageID)
	if receiptErr != nil {
		return nil, fmt.Errorf("recover Agent Message Command receipt: %w", errors.Join(err, receiptErr))
	}
	if receipt == nil || receipt.Status == application.MessageCommandReceiptStatusAbsent {
		return nil, err
	}
	if receipt.Status != application.MessageCommandReceiptStatusCommitted || !agentCommandMessageMatchesV1(receipt.Message, command.Kind, agentUUID, messageTargetForKind(command.Kind, agentUUID, principalUUID, conversationKey), conversationKey, content, clientMessageID) {
		return nil, application.ErrAgentCommandConflict
	}
	return receipt.Message, nil
}

// messageTargetForKind returns the message target UUID the receipt/match check
// expects for a given command kind: the principal for 1v1 replies, the group
// UUID for Route B/B2 group replies.
func messageTargetForKind(kind application.AgentMessageCommandKindV1, agentUUID, principalUUID, conversationKey string) string {
	if kind == application.AgentMessageCommandGroupReplyV1 {
		return strings.TrimPrefix(conversationKey, "group:")
	}
	return principalUUID
}

func agentCommandMessageMatchesV1(message *model.Message, kind application.AgentMessageCommandKindV1, senderUUID, targetUUID, conversationKey, content, clientMessageID string) bool {
	if message == nil || strings.TrimSpace(message.SenderUUID) != senderUUID || strings.TrimSpace(message.TargetUUID) != targetUUID ||
		strings.TrimSpace(message.ConversationKey) != conversationKey ||
		strings.TrimSpace(message.ClientMessageID) != clientMessageID || strings.TrimSpace(message.Content) != content {
		return false
	}
	switch kind {
	case application.AgentMessageCommandAssistantReplyV1:
		return message.TargetType == model.MessageTargetDirect && message.MessageType == model.MessageTypeAIText
	case application.AgentMessageCommandSystemMessageV1:
		return message.TargetType == model.MessageTargetDirect && message.MessageType == model.MessageTypeSystem
	case application.AgentMessageCommandGroupReplyV1:
		return message.TargetType == model.MessageTargetGroup && message.MessageType == model.MessageTypeAIText
	default:
		return false
	}
}

func AgentCommandCapabilityIDV1(kind application.AgentMessageCommandKindV1) (string, error) {
	switch kind {
	case application.AgentMessageCommandAssistantReplyV1:
		return application.AgentCapabilityAssistantReplySend, nil
	case application.AgentMessageCommandSystemMessageV1:
		return application.AgentCapabilitySystemMessageSend, nil
	case application.AgentMessageCommandGroupReplyV1:
		return application.AgentCapabilityGroupReplySend, nil
	default:
		return "", application.ErrAgentCommandDenied
	}
}
