package application

import (
	"context"
	"errors"

	"github.com/JekYUlll/Dipole/internal/model"
)

const AgentCapabilityVersionV1 = "dipole.agent.capability.v1"

var ErrAgentCapabilityDenied = errors.New("agent capability access denied")

type AgentConversationReadV1 struct {
	Found      bool
	Reason     string
	TargetUUID string
	TargetType int8
	Messages   []*model.Message
}

// AgentCapabilityV1 is the transport-neutral boundary used by Agent runtimes.
// Identity arguments must come from a trusted execution context.
type AgentCapabilityV1 interface {
	GetUserProfile(ctx context.Context, principalUUID, agentUUID, subjectUUID string) (*model.User, error)
	ListDirectMessages(ctx context.Context, principalUUID, agentUUID string, limit int) ([]*model.Message, error)
	ListConversations(ctx context.Context, principalUUID string, limit int) ([]*model.Conversation, error)
	ReadConversation(ctx context.Context, principalUUID, targetUUID string, limit int) (*AgentConversationReadV1, error)
	SendSystemMessage(ctx context.Context, agentUUID, principalUUID, content string) (*model.Message, error)
}
