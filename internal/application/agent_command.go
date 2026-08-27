package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/JekYUlll/Dipole/internal/model"
)

const AgentCommandVersionV1 = "dipole.agent.command.v1"

var (
	ErrAgentCommandDenied   = errors.New("agent command denied")
	ErrAgentCommandConflict = errors.New("agent command result conflicts with the request")
)

type AgentMessageCommandKindV1 string

const (
	AgentMessageCommandAssistantReplyV1 AgentMessageCommandKindV1 = "assistant_reply"
	AgentMessageCommandSystemMessageV1  AgentMessageCommandKindV1 = "system_message"
)

type AgentMessageCommandV1 struct {
	CommandID  string                    `json:"command_id"`
	Kind       AgentMessageCommandKindV1 `json:"kind"`
	Invocation AgentInvocationV1         `json:"invocation"`
	Content    string                    `json:"content"`
}

// AgentCommandV1 is the transport-neutral write boundary for Agent runtimes.
// Sender and target identities are derived from the trusted invocation.
type AgentCommandV1 interface {
	SendMessage(ctx context.Context, command AgentMessageCommandV1) (*model.Message, error)
}

func AgentCommandClientMessageIDV1(kind AgentMessageCommandKindV1, commandID string) (string, error) {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" || len(commandID) > 128 {
		return "", ErrAgentCommandDenied
	}
	switch kind {
	case AgentMessageCommandAssistantReplyV1, AgentMessageCommandSystemMessageV1:
	default:
		return "", ErrAgentCommandDenied
	}
	canonical := AgentCommandVersionV1 + "\n" + string(kind) + "\n" + commandID
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}
