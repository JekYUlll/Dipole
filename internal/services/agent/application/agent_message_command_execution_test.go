package agentapplication_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

type agentMessageCommandToolReaderStub struct {
	invocation *application.AgentToolInvocationV1
	err        error
}

func (s agentMessageCommandToolReaderStub) GetToolInvocation(context.Context, string) (*application.AgentToolInvocationV1, error) {
	return s.invocation, s.err
}

type agentMessageCommandSenderStub struct {
	command application.AgentMessageCommandV1
	lineage eventlineage.Lineage
	message *model.Message
	err     error
}

func (s *agentMessageCommandSenderStub) SendMessage(ctx context.Context, command application.AgentMessageCommandV1) (*model.Message, error) {
	s.command = command
	s.lineage = eventlineage.FromContext(ctx)
	if s.message != nil && s.message.ClientMessageID == "" {
		s.message.ClientMessageID, _ = application.AgentCommandClientMessageIDV1(command.Kind, command.CommandID)
	}
	return s.message, s.err
}

func TestAgentMessageCommandExecutionBindsApprovedToolAndDerivesCommand(t *testing.T) {
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Permissions: []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: model.DirectConversationKey("U100", "UAI"), Actions: []string{application.AgentResourceActionWrite}}},
	}
	argumentsSHA, err := application.AgentMessageCommandToolArgumentsSHA256V1(invocation.PrincipalUUID, invocation.AgentUUID, "notice")
	if err != nil {
		t.Fatalf("derive Tool arguments digest: %v", err)
	}
	tool := &application.AgentToolInvocationV1{
		InvocationUUID: "INV-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Transport: application.AgentToolTransportMCP, CapabilityID: application.AgentCapabilitySystemMessageSend, ArgumentsSHA256: argumentsSHA, Status: application.AgentToolInvocationStatusRunning,
		ApprovalUUID: "APR-1", RequestID: "REQ-1", TraceID: "TRACE-1",
	}
	sender := &agentMessageCommandSenderStub{message: &model.Message{UUID: "MSG-1"}}
	service, err := agentapplication.NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: tool}, agentToolAuditResolverStub{invocation: invocation}, agentToolApprovalReaderStub{}, sender)
	if err != nil {
		t.Fatalf("new Message Command execution: %v", err)
	}
	result, err := service.Execute(context.Background(), application.AgentMessageCommandExecutionRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-1", Kind: application.AgentMessageCommandSystemMessageV1, Content: " notice ",
	})
	if err != nil {
		t.Fatalf("execute Message Command: %v", err)
	}
	if result.MessageUUID != "MSG-1" || result.CommandID == "" || result.ClientMessageID == "" || result.Kind != application.AgentMessageCommandSystemMessageV1 {
		t.Fatalf("unexpected Message Command result: %+v", result)
	}
	if sender.command.CommandID != result.CommandID || sender.command.Content != "notice" || sender.command.Invocation.RequestID != "REQ-1" || sender.command.Invocation.TraceID != "TRACE-1" {
		t.Fatalf("unexpected derived command: %+v", sender.command)
	}
	if sender.lineage.Origin.ID != "UAI" || sender.lineage.AgentTaskID != "TASK-1" {
		t.Fatalf("missing Agent action lineage: %+v", sender.lineage)
	}
}

func TestAgentMessageCommandExecutionRejectsUnboundOrDriftingTool(t *testing.T) {
	argumentsSHA, err := application.AgentMessageCommandToolArgumentsSHA256V1("U100", "UAI", "notice")
	if err != nil {
		t.Fatalf("derive Tool arguments digest: %v", err)
	}
	base := &application.AgentToolInvocationV1{
		InvocationUUID: "INV-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Transport: application.AgentToolTransportMCP, CapabilityID: application.AgentCapabilitySystemMessageSend, ArgumentsSHA256: argumentsSHA, Status: application.AgentToolInvocationStatusRunning, ApprovalUUID: "APR-1",
	}
	identity := application.AgentInvocationV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI"}
	request := application.AgentMessageCommandExecutionRequestV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-1", Kind: application.AgentMessageCommandSystemMessageV1, Content: "notice"}
	for _, test := range []struct {
		name string
		edit func(*application.AgentToolInvocationV1)
	}{
		{name: "missing approval", edit: func(value *application.AgentToolInvocationV1) { value.ApprovalUUID = "" }},
		{name: "terminal", edit: func(value *application.AgentToolInvocationV1) {
			value.Status = application.AgentToolInvocationStatusCompleted
		}},
		{name: "wrong run", edit: func(value *application.AgentToolInvocationV1) { value.RunUUID = "RUN-2" }},
		{name: "wrong transport", edit: func(value *application.AgentToolInvocationV1) { value.Transport = "native" }},
		{name: "wrong capability", edit: func(value *application.AgentToolInvocationV1) {
			value.CapabilityID = application.AgentCapabilityAssistantReplySend
		}},
		{name: "argument drift", edit: func(value *application.AgentToolInvocationV1) { value.ArgumentsSHA256 = testAuditSHA }},
		{name: "identity drift", edit: func(value *application.AgentToolInvocationV1) { value.PrincipalUUID = "U999" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			tool := *base
			test.edit(&tool)
			sender := &agentMessageCommandSenderStub{}
			service, _ := agentapplication.NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: &tool}, agentToolAuditResolverStub{invocation: identity}, agentToolApprovalReaderStub{}, sender)
			if _, err := service.Execute(context.Background(), request); !errors.Is(err, application.ErrAgentCommandDenied) {
				t.Fatalf("execution error = %v", err)
			}
			if sender.command.CommandID != "" {
				t.Fatalf("denied command reached sender: %+v", sender.command)
			}
		})
	}
}

func TestAgentMessageCommandExecutionRejectsMissingDependencies(t *testing.T) {
	if _, err := agentapplication.NewAgentMessageCommandExecutionV1(nil, agentToolAuditResolverStub{}, agentToolApprovalReaderStub{}, &agentMessageCommandSenderStub{}); err == nil {
		t.Fatal("expected missing Tool reader to fail")
	}
	if _, err := agentapplication.NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{}, agentToolAuditResolverStub{}, nil, &agentMessageCommandSenderStub{}); err == nil {
		t.Fatal("expected missing approval reader to fail")
	}
}

func TestAgentMessageCommandExecutionGroupReplyRecoversConversationFromApproval(t *testing.T) {
	const groupConversation = "group:G1"
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Permissions: []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard, Actions: []string{application.AgentResourceActionWrite}}},
	}
	argumentsSHA, err := application.AgentMessageCommandToolArgumentsSHA256ForConversationV1("hi team", groupConversation)
	if err != nil {
		t.Fatalf("derive group Tool arguments digest: %v", err)
	}
	tool := &application.AgentToolInvocationV1{
		InvocationUUID: "INV-G", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Transport: application.AgentToolTransportMCP, CapabilityID: application.AgentCapabilityGroupReplySend, ArgumentsSHA256: argumentsSHA,
		Status: application.AgentToolInvocationStatusRunning, ApprovalUUID: "APR-G", RequestID: "REQ-1", TraceID: "TRACE-1",
	}
	approval := &application.AgentApprovalV1{
		ApprovalUUID: "APR-G", TaskUUID: "TASK-1", CapabilityID: application.AgentCapabilityGroupReplySend,
		ResourceScope:   application.AgentResourceScopeV1{ResourceType: application.AgentResourceTypeConversation, ResourceID: groupConversation, Actions: []string{application.AgentResourceActionWrite}},
		ArgumentsSHA256: argumentsSHA, Status: application.AgentApprovalStatusConsumed,
	}
	sender := &agentMessageCommandSenderStub{message: &model.Message{UUID: "MSG-G"}}
	service, err := agentapplication.NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: tool}, agentToolAuditResolverStub{invocation: invocation}, agentToolApprovalReaderStub{approval: approval}, sender)
	if err != nil {
		t.Fatalf("new Message Command execution: %v", err)
	}
	result, err := service.Execute(context.Background(), application.AgentMessageCommandExecutionRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-G", Kind: application.AgentMessageCommandGroupReplyV1, Content: "hi team",
	})
	if err != nil {
		t.Fatalf("execute group reply command: %v", err)
	}
	if result.MessageUUID != "MSG-G" || result.Kind != application.AgentMessageCommandGroupReplyV1 {
		t.Fatalf("unexpected group reply result: %+v", result)
	}
	if sender.command.ConversationKey != groupConversation {
		t.Fatalf("group reply targeted %q, want %q", sender.command.ConversationKey, groupConversation)
	}
}

func TestAgentMessageCommandExecutionGroupReplyRejectsUnusableApprovalScope(t *testing.T) {
	const groupConversation = "group:G1"
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Permissions: []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard, Actions: []string{application.AgentResourceActionWrite}}},
	}
	argumentsSHA, err := application.AgentMessageCommandToolArgumentsSHA256ForConversationV1("hi team", groupConversation)
	if err != nil {
		t.Fatalf("derive group Tool arguments digest: %v", err)
	}
	tool := &application.AgentToolInvocationV1{
		InvocationUUID: "INV-G", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Transport: application.AgentToolTransportMCP, CapabilityID: application.AgentCapabilityGroupReplySend, ArgumentsSHA256: argumentsSHA,
		Status: application.AgentToolInvocationStatusRunning, ApprovalUUID: "APR-G",
	}
	base := application.AgentApprovalV1{
		ApprovalUUID: "APR-G", TaskUUID: "TASK-1", CapabilityID: application.AgentCapabilityGroupReplySend,
		ResourceScope:   application.AgentResourceScopeV1{ResourceType: application.AgentResourceTypeConversation, ResourceID: groupConversation, Actions: []string{application.AgentResourceActionWrite}},
		ArgumentsSHA256: argumentsSHA, Status: application.AgentApprovalStatusConsumed,
	}
	request := application.AgentMessageCommandExecutionRequestV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-G", Kind: application.AgentMessageCommandGroupReplyV1, Content: "hi team"}
	for _, test := range []struct {
		name string
		edit func(*application.AgentApprovalV1)
	}{
		{name: "missing approval", edit: func(value *application.AgentApprovalV1) { *value = application.AgentApprovalV1{} }},
		{name: "not consumed", edit: func(value *application.AgentApprovalV1) { value.Status = application.AgentApprovalStatusApproved }},
		{name: "wrong capability", edit: func(value *application.AgentApprovalV1) { value.CapabilityID = application.AgentCapabilityAssistantReplySend }},
		{name: "wrong task", edit: func(value *application.AgentApprovalV1) { value.TaskUUID = "TASK-2" }},
		{name: "arguments drift", edit: func(value *application.AgentApprovalV1) { value.ArgumentsSHA256 = testAuditSHA }},
		{name: "non group scope", edit: func(value *application.AgentApprovalV1) { value.ResourceScope.ResourceID = model.DirectConversationKey("U100", "UAI") }},
		{name: "empty group scope", edit: func(value *application.AgentApprovalV1) { value.ResourceScope.ResourceID = "group:" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			approval := base
			test.edit(&approval)
			sender := &agentMessageCommandSenderStub{}
			service, _ := agentapplication.NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: tool}, agentToolAuditResolverStub{invocation: invocation}, agentToolApprovalReaderStub{approval: &approval}, sender)
			if _, err := service.Execute(context.Background(), request); !errors.Is(err, application.ErrAgentCommandDenied) {
				t.Fatalf("execution error = %v", err)
			}
			if sender.command.CommandID != "" {
				t.Fatalf("denied group reply reached sender: %+v", sender.command)
			}
		})
	}
}
