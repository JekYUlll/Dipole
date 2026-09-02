package application

import (
	"context"
	"errors"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
)

const AgentCapabilityVersionV1 = "dipole.agent.capability.v1"

var ErrAgentCapabilityDenied = errors.New("agent capability access denied")
var ErrAgentCapabilityUnavailable = errors.New("agent capability is unavailable")

type AgentConversationReadV1 struct {
	Found      bool
	Reason     string
	TargetUUID string
	TargetType int8
	Messages   []*model.Message
}

// AgentConversationSearchEvidenceV1 is bounded, untrusted retrieval evidence.
// It deliberately excludes Search implementation details and credentials.
type AgentConversationSearchEvidenceV1 struct {
	MessageUUID     string
	ConversationKey string
	MessageSeq      uint64
	Revision        uint64
	SenderUUID      string
	MessageType     int8
	Content         string
	SentAt          time.Time
	QuerySHA256     string
}

// AgentCapabilityV1 is the transport-neutral boundary used by Agent runtimes.
// Identity arguments must come from a trusted execution context.
type AgentCapabilityV1 interface {
	GetUserProfile(ctx context.Context, invocation AgentInvocationV1, subjectUUID string) (*model.User, error)
	ListDirectMessages(ctx context.Context, invocation AgentInvocationV1, limit int) ([]*model.Message, error)
	ListConversations(ctx context.Context, invocation AgentInvocationV1, limit int) ([]*model.Conversation, error)
	ReadConversation(ctx context.Context, invocation AgentInvocationV1, targetUUID string, limit int) (*AgentConversationReadV1, error)
	SearchConversations(ctx context.Context, invocation AgentInvocationV1, query string, limit int) ([]*AgentConversationSearchEvidenceV1, error)
	SendSystemMessage(ctx context.Context, invocation AgentInvocationV1, content string) (*model.Message, error)
}
