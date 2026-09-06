package agentapplication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
)

type AgentMessageCommandExecutionServiceV1 struct {
	tools    application.AgentToolInvocationReaderV1
	resolver application.AgentInvocationResolverV1
	commands application.AgentCommandV1
}

var _ application.AgentMessageCommandExecutionV1 = (*AgentMessageCommandExecutionServiceV1)(nil)

func NewAgentMessageCommandExecutionV1(tools application.AgentToolInvocationReaderV1, resolver application.AgentInvocationResolverV1, commands application.AgentCommandV1) (*AgentMessageCommandExecutionServiceV1, error) {
	if tools == nil || resolver == nil || commands == nil {
		return nil, errors.New("Agent Message Command execution dependencies are required")
	}
	return &AgentMessageCommandExecutionServiceV1{tools: tools, resolver: resolver, commands: commands}, nil
}

func (s *AgentMessageCommandExecutionServiceV1) Execute(ctx context.Context, request application.AgentMessageCommandExecutionRequestV1) (*application.AgentMessageCommandExecutionResultV1, error) {
	request.TaskUUID, request.RunUUID, request.InvocationUUID = strings.TrimSpace(request.TaskUUID), strings.TrimSpace(request.RunUUID), strings.TrimSpace(request.InvocationUUID)
	request.Content = strings.TrimSpace(request.Content)
	if request.TaskUUID == "" || request.RunUUID == "" || request.InvocationUUID == "" || request.Content == "" {
		return nil, application.ErrAgentCommandDenied
	}
	wantCapability, err := AgentCommandCapabilityIDV1(request.Kind)
	if err != nil {
		return nil, application.ErrAgentCommandDenied
	}
	tool, err := s.tools.GetToolInvocation(ctx, request.InvocationUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent Tool invocation for Message Command: %w", err)
	}
	if tool == nil || tool.InvocationUUID != request.InvocationUUID || tool.TaskUUID != request.TaskUUID || tool.RunUUID != request.RunUUID ||
		tool.Transport != application.AgentToolTransportMCP || tool.Status != application.AgentToolInvocationStatusRunning || strings.TrimSpace(tool.ApprovalUUID) == "" || tool.CapabilityID != wantCapability {
		return nil, application.ErrAgentCommandDenied
	}
	invocation, err := s.resolver.Resolve(ctx, request.TaskUUID, request.RunUUID)
	if err != nil || invocation.TenantID != tool.TenantID || invocation.PrincipalUUID != tool.PrincipalUUID || invocation.AgentUUID != tool.AgentUUID {
		return nil, application.ErrAgentCommandDenied
	}
	wantArgumentsSHA, err := s.wantArgumentsSHA(request, invocation, *tool)
	if err != nil || tool.ArgumentsSHA256 != wantArgumentsSHA {
		return nil, application.ErrAgentCommandDenied
	}
	commandID, err := application.AgentMessageCommandIDV1(request.InvocationUUID, request.Kind)
	if err != nil {
		return nil, application.ErrAgentCommandDenied
	}
	invocation.RequestID, invocation.TraceID = strings.TrimSpace(tool.RequestID), strings.TrimSpace(tool.TraceID)
	ctx = eventlineage.AgentAction(ctx, invocation.AgentUUID, request.TaskUUID, "")
	command := application.AgentMessageCommandV1{CommandID: commandID, Kind: request.Kind, Invocation: invocation, Content: request.Content}
	if request.Kind == application.AgentMessageCommandGroupReplyV1 {
		conversationID, err := agentMessageCommandConversationIDFromArgumentsV1(tool.ArgumentsJSON)
		if err != nil {
			return nil, err
		}
		command.ConversationKey = conversationID
	}
	message, err := s.commands.SendMessage(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("execute Agent Message Command: %w", err)
	}
	clientMessageID, err := application.AgentCommandClientMessageIDV1(request.Kind, commandID)
	if err != nil || message == nil || strings.TrimSpace(message.UUID) == "" || strings.TrimSpace(message.ClientMessageID) != clientMessageID {
		return nil, application.ErrAgentCommandConflict
	}
	return &application.AgentMessageCommandExecutionResultV1{
		MessageUUID: strings.TrimSpace(message.UUID), ClientMessageID: clientMessageID, CommandID: commandID, Kind: request.Kind,
	}, nil
}

// wantArgumentsSHA re-derives the tool invocation arguments digest the runtime
// committed at begin time. For 1v1 replies (assistant_reply / system_message)
// the conversation is the owner's direct Agent conversation; for group replies
// (Route B/B2) the conversation is the group the trigger mentioned, which the
// runtime stamped into the tool's ArgumentsJSON, so Core re-derives the digest
// from that conversation id rather than a principal-derived direct key.
func (s *AgentMessageCommandExecutionServiceV1) wantArgumentsSHA(request application.AgentMessageCommandExecutionRequestV1, invocation application.AgentInvocationV1, tool application.AgentToolInvocationV1) (string, error) {
	if request.Kind == application.AgentMessageCommandGroupReplyV1 {
		conversationID, err := agentMessageCommandConversationIDFromArgumentsV1(tool.ArgumentsJSON)
		if err != nil {
			return "", err
		}
		return application.AgentMessageCommandToolArgumentsSHA256ForConversationV1(request.Content, conversationID)
	}
	return application.AgentMessageCommandToolArgumentsSHA256V1(invocation.PrincipalUUID, invocation.AgentUUID, request.Content)
}

func agentMessageCommandConversationIDFromArgumentsV1(argumentsJSON string) (string, error) {
	trimmed := strings.TrimSpace(argumentsJSON)
	if trimmed == "" {
		return "", application.ErrAgentCommandDenied
	}
	var decoded struct {
		ConversationID string `json:"conversationId"`
	}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return "", application.ErrAgentCommandDenied
	}
	conversationID := strings.TrimSpace(decoded.ConversationID)
	if conversationID == "" || !strings.HasPrefix(conversationID, "group:") {
		return "", application.ErrAgentCommandDenied
	}
	return conversationID, nil
}
