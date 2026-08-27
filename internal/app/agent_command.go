package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

const maxAgentCommandIDLengthV1 = 128

type agentCommandMessages interface {
	SendAssistantTextMessageContext(ctx context.Context, assistantUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
	SendSystemDirectMessageCommandContext(ctx context.Context, senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error)
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
	if err := application.AuthorizeAgentCapabilityV1(command.Invocation, descriptor); err != nil {
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

	switch command.Kind {
	case application.AgentMessageCommandAssistantReplyV1:
		return c.messages.SendAssistantTextMessageContext(ctx, agentUUID, principalUUID, content, clientMessageID)
	case application.AgentMessageCommandSystemMessageV1:
		return c.messages.SendSystemDirectMessageCommandContext(ctx, agentUUID, principalUUID, content, clientMessageID)
	default:
		return nil, application.ErrAgentCommandDenied
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
