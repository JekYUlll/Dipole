package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// AgentMessageCommandGroupReplyV1 is the Route B/B2 group @-mention reply:
	// the assistant sends an AI-text message into the group conversation that
	// triggered the task. The target is a group (TargetTypeGroup), not a user.
	AgentMessageCommandGroupReplyV1 AgentMessageCommandKindV1 = "group_reply"
)

type AgentMessageCommandV1 struct {
	CommandID  string                    `json:"command_id"`
	Kind       AgentMessageCommandKindV1 `json:"kind"`
	Invocation AgentInvocationV1         `json:"invocation"`
	Content    string                    `json:"content"`
	// ConversationKey is the conversation the command targets. For 1v1 replies
	// it is the owner's direct Agent conversation; for Route B/B2 group replies
	// it is the group conversation (group:<uuid>) the trigger mentioned. Empty
	// falls back to the direct conversation for backward compatibility.
	ConversationKey string `json:"conversation_key,omitempty"`
}

// AgentCommandV1 is the transport-neutral write boundary for Agent runtimes.
// Sender and target identities are derived from the trusted invocation.
type AgentCommandV1 interface {
	SendMessage(ctx context.Context, command AgentMessageCommandV1) (*model.Message, error)
}

type AgentMessageCommandExecutionRequestV1 struct {
	TaskUUID       string
	RunUUID        string
	InvocationUUID string
	Kind           AgentMessageCommandKindV1
	Content        string
}

type AgentMessageCommandExecutionResultV1 struct {
	MessageUUID     string
	ClientMessageID string
	CommandID       string
	Kind            AgentMessageCommandKindV1
}

type AgentMessageCommandExecutionV1 interface {
	Execute(ctx context.Context, request AgentMessageCommandExecutionRequestV1) (*AgentMessageCommandExecutionResultV1, error)
}

func AgentCommandClientMessageIDV1(kind AgentMessageCommandKindV1, commandID string) (string, error) {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" || len(commandID) > 128 {
		return "", ErrAgentCommandDenied
	}
	switch kind {
	case AgentMessageCommandAssistantReplyV1, AgentMessageCommandSystemMessageV1, AgentMessageCommandGroupReplyV1:
	default:
		return "", ErrAgentCommandDenied
	}
	canonical := AgentCommandVersionV1 + "\n" + string(kind) + "\n" + commandID
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func AgentMessageCommandToolArgumentsSHA256V1(principalUUID, agentUUID, content string) (string, error) {
	principalUUID, agentUUID, content = strings.TrimSpace(principalUUID), strings.TrimSpace(agentUUID), strings.TrimSpace(content)
	if principalUUID == "" || agentUUID == "" || content == "" {
		return "", ErrAgentCommandDenied
	}
	payload, err := json.Marshal(struct {
		Content        string `json:"content"`
		ConversationID string `json:"conversationId"`
	}{Content: content, ConversationID: model.DirectConversationKey(principalUUID, agentUUID)})
	if err != nil {
		return "", ErrAgentCommandDenied
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

// AgentMessageCommandToolArgumentsSHA256ForConversationV1 derives the tool
// invocation arguments digest for a message command scoped to an explicit
// conversation (e.g. a group conversation for Route B/B2 group replies). The
// digest must match what the runtime hashed over canonical MCP JSON when it
// began the Tool Invocation, so Core can re-verify the binding.
func AgentMessageCommandToolArgumentsSHA256ForConversationV1(content, conversationID string) (string, error) {
	content, conversationID = strings.TrimSpace(content), strings.TrimSpace(conversationID)
	if content == "" || conversationID == "" {
		return "", ErrAgentCommandDenied
	}
	return agentMessageCommandArgumentsDigestV1(content, conversationID)
}

func agentMessageCommandArgumentsDigestV1(content, conversationID string) (string, error) {
	payload, err := json.Marshal(struct {
		Content        string `json:"content"`
		ConversationID string `json:"conversationId"`
	}{Content: content, ConversationID: conversationID})
	if err != nil {
		return "", ErrAgentCommandDenied
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func AgentMessageCommandIDV1(invocationUUID string, kind AgentMessageCommandKindV1) (string, error) {
	invocationUUID = strings.TrimSpace(invocationUUID)
	if invocationUUID == "" || len(invocationUUID) > 64 {
		return "", ErrAgentCommandDenied
	}
	switch kind {
	case AgentMessageCommandAssistantReplyV1, AgentMessageCommandSystemMessageV1, AgentMessageCommandGroupReplyV1:
	default:
		return "", ErrAgentCommandDenied
	}
	digest := sha256.Sum256([]byte(AgentCommandVersionV1 + "\ntool\n" + invocationUUID + "\n" + string(kind)))
	return "tool:" + hex.EncodeToString(digest[:]), nil
}
