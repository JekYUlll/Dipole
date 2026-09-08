package agentapplication

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
)

type AgentMessageCommandExecutionServiceV1 struct {
	tools     application.AgentToolInvocationReaderV1
	resolver  application.AgentInvocationResolverV1
	approvals application.AgentToolApprovalReaderV1
	commands  application.AgentCommandV1
}

var _ application.AgentMessageCommandExecutionV1 = (*AgentMessageCommandExecutionServiceV1)(nil)

func NewAgentMessageCommandExecutionV1(tools application.AgentToolInvocationReaderV1, resolver application.AgentInvocationResolverV1, approvals application.AgentToolApprovalReaderV1, commands application.AgentCommandV1) (*AgentMessageCommandExecutionServiceV1, error) {
	if tools == nil || resolver == nil || approvals == nil || commands == nil {
		return nil, errors.New("Agent Message Command execution dependencies are required")
	}
	return &AgentMessageCommandExecutionServiceV1{tools: tools, resolver: resolver, approvals: approvals, commands: commands}, nil
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
	// Route B/B2: recover the group conversation the reply targets from the
	// tool's consumed group_reply approval scope. The scope (conversation
	// group:<uuid>, write) is what authorized this write, so it is the
	// authoritative binding — the runtime never stamps the conversation into
	// the tool's ArgumentsJSON (message writes carry no arguments payload).
	var groupConversationID string
	if request.Kind == application.AgentMessageCommandGroupReplyV1 {
		groupConversationID, err = s.groupReplyConversationID(ctx, *tool)
		if err != nil {
			return nil, err
		}
	}
	wantArgumentsSHA, err := s.wantArgumentsSHA(request, invocation, groupConversationID)
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
		command.ConversationKey = groupConversationID
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
// (Route B/B2) the conversation is the group the trigger mentioned, recovered
// from the consumed approval scope, so Core re-derives the digest from that
// conversation id rather than a principal-derived direct key.
func (s *AgentMessageCommandExecutionServiceV1) wantArgumentsSHA(request application.AgentMessageCommandExecutionRequestV1, invocation application.AgentInvocationV1, groupConversationID string) (string, error) {
	if request.Kind == application.AgentMessageCommandGroupReplyV1 {
		return application.AgentMessageCommandToolArgumentsSHA256ForConversationV1(request.Content, groupConversationID)
	}
	return application.AgentMessageCommandToolArgumentsSHA256V1(invocation.PrincipalUUID, invocation.AgentUUID, request.Content)
}

// groupReplyConversationID recovers the group conversation a Route B/B2 reply
// targets from the tool invocation's consumed group_reply approval. The scope
// that authorized the write (conversation group:<uuid>, write) is the
// authoritative source of the conversation id — the runtime cannot stamp it
// into the tool's ArgumentsJSON because message-write invocations carry no
// arguments payload (ValidateAgentMCPToolCommandV1 forbids arguments without an
// external profile binding).
func (s *AgentMessageCommandExecutionServiceV1) groupReplyConversationID(ctx context.Context, tool application.AgentToolInvocationV1) (string, error) {
	approvalUUID := strings.TrimSpace(tool.ApprovalUUID)
	if approvalUUID == "" {
		return "", application.ErrAgentCommandDenied
	}
	approval, err := s.approvals.GetApproval(ctx, approvalUUID)
	if err != nil {
		return "", fmt.Errorf("load group reply approval: %w", err)
	}
	if approval == nil || approval.ApprovalUUID != approvalUUID || approval.TaskUUID != tool.TaskUUID ||
		approval.CapabilityID != application.AgentCapabilityGroupReplySend ||
		approval.Status != application.AgentApprovalStatusConsumed ||
		approval.ArgumentsSHA256 != tool.ArgumentsSHA256 ||
		approval.ResourceScope.ResourceType != application.AgentResourceTypeConversation {
		return "", application.ErrAgentCommandDenied
	}
	conversationID := strings.TrimSpace(approval.ResourceScope.ResourceID)
	if conversationID == "" || conversationID == "group:" || !strings.HasPrefix(conversationID, "group:") {
		return "", application.ErrAgentCommandDenied
	}
	return conversationID, nil
}
