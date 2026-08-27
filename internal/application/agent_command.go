package application

import (
	"context"
	"errors"

	"github.com/JekYUlll/Dipole/internal/model"
)

const AgentCommandVersionV1 = "dipole.agent.command.v1"

var ErrAgentCommandDenied = errors.New("agent command denied")

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
