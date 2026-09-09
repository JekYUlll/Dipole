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

type AgentConversationSearchResultV1 struct {
	MessageUUID      string
	ConversationKey  string
	MessageSeq       uint64
	SenderUUID       string
	Content          string
	SentAtUnixMillis int64
}

// AgentCapabilityV1 is the transport-neutral boundary used by Agent runtimes.
// Identity arguments must come from a trusted execution context.
type AgentCapabilityV1 interface {
	GetUserProfile(ctx context.Context, invocation AgentInvocationV1, subjectUUID string) (*model.User, error)
	ListDirectMessages(ctx context.Context, invocation AgentInvocationV1, limit int) ([]*model.Message, error)
	ListConversations(ctx context.Context, invocation AgentInvocationV1, limit int) ([]*model.Conversation, error)
	ReadConversation(ctx context.Context, invocation AgentInvocationV1, targetUUID string, limit int) (*AgentConversationReadV1, error)
	SearchConversations(ctx context.Context, invocation AgentInvocationV1, text string, limit int) ([]*AgentConversationSearchResultV1, error)
	SendSystemMessage(ctx context.Context, invocation AgentInvocationV1, content string) (*model.Message, error)
}
