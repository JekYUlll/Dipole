package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

type agentCommandMessages interface {
	SendAssistantTextMessageContext(ctx context.Context, assistantUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	SendSystemDirectMessageCommandContext(ctx context.Context, senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	GetMessageCommandReceiptContext(ctx context.Context, senderUUID, clientMessageID string) (*application.MessageCommandReceipt, error)
}

type LocalAgentCommandV1 struct {
	messages agentCommandMessages
}

var _ application.AgentCommandV1 = (*LocalAgentCommandV1)(nil)

func NewLocalAgentCommandV1(messages agentCommandMessages) (*LocalAgentCommandV1, error) {
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

	capabilityID, err := agentCommandCapabilityIDV1(command.Kind)
	if err != nil {
		return nil, err
	}
	descriptor, ok := application.AgentCapabilityDescriptorByIDV1(capabilityID)
	if !ok {
		return nil, application.ErrAgentCommandDenied
	}
	conversationKey := model.DirectConversationKey(command.Invocation.PrincipalUUID, command.Invocation.AgentUUID)
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
	clientMessageID := agentCommandClientMessageIDV1(command.Kind, commandID)

	var message *model.Message
	switch command.Kind {
	case application.AgentMessageCommandAssistantReplyV1:
		message, err = c.messages.SendAssistantTextMessageContext(ctx, agentUUID, principalUUID, content, clientMessageID)
	case application.AgentMessageCommandSystemMessageV1:
		message, err = c.messages.SendSystemDirectMessageCommandContext(ctx, agentUUID, principalUUID, content, clientMessageID)
	default:
		return nil, application.ErrAgentCommandDenied
	}
	if err == nil {
		if !agentCommandMessageMatchesV1(message, command.Kind, agentUUID, principalUUID, content, clientMessageID) {
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
	if receipt.Status != application.MessageCommandReceiptStatusCommitted || !agentCommandMessageMatchesV1(receipt.Message, command.Kind, agentUUID, principalUUID, content, clientMessageID) {
		return nil, application.ErrAgentCommandConflict
	}
	return receipt.Message, nil
}

func agentCommandMessageMatchesV1(message *model.Message, kind application.AgentMessageCommandKindV1, senderUUID, targetUUID, content, clientMessageID string) bool {
	if message == nil || strings.TrimSpace(message.SenderUUID) != senderUUID || strings.TrimSpace(message.TargetUUID) != targetUUID ||
		message.TargetType != model.MessageTargetDirect || strings.TrimSpace(message.ConversationKey) != model.DirectConversationKey(senderUUID, targetUUID) ||
		strings.TrimSpace(message.ClientMessageID) != clientMessageID || strings.TrimSpace(message.Content) != content {
		return false
	}
	switch kind {
	case application.AgentMessageCommandAssistantReplyV1:
		return message.MessageType == model.MessageTypeAIText
	case application.AgentMessageCommandSystemMessageV1:
		return message.MessageType == model.MessageTypeSystem
	default:
		return false
	}
}

func agentCommandCapabilityIDV1(kind application.AgentMessageCommandKindV1) (string, error) {
	switch kind {
	case application.AgentMessageCommandAssistantReplyV1:
		return application.AgentCapabilityAssistantReplySend, nil
	case application.AgentMessageCommandSystemMessageV1:
		return application.AgentCapabilitySystemMessageSend, nil
	default:
		return "", application.ErrAgentCommandDenied
	}
}

func agentCommandClientMessageIDV1(kind application.AgentMessageCommandKindV1, commandID string) string {
	canonical := application.AgentCommandVersionV1 + "\n" + string(kind) + "\n" + strings.TrimSpace(commandID)
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}
