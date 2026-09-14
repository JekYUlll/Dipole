package app

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/eventlineage"
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

type scheduledGroupCapabilityStub struct {
	application.AgentCapabilityV1
	read      *application.AgentConversationReadV1
	err       error
	principal string
}

func (s *scheduledGroupCapabilityStub) ReadConversation(_ context.Context, invocation application.AgentInvocationV1, _ string, _ int) (*application.AgentConversationReadV1, error) {
	s.principal = invocation.PrincipalUUID
	return s.read, s.err
}

func TestApprovedGroupPublicationRechecksMembershipAndBoundContent(t *testing.T) {
	for _, scenario := range []string{"member", "left", "denied", "wrong-group", "changed-content", "missing-check"} {
		t.Run(scenario, func(t *testing.T) {
			args, _ := application.AgentMessageCommandToolArgumentsSHA256ForConversationV1("summary", "group:G1")
			tool := &application.AgentToolInvocationV1{
				InvocationUUID: "INV-G", TaskUUID: "TASK", RunUUID: "RUN", TenantID: "dipole", PrincipalUUID: "U1", AgentUUID: "AI",
				Transport: application.AgentToolTransportMCP, CapabilityID: application.AgentCapabilityGroupReplySend,
				Status: application.AgentToolInvocationStatusRunning, ApprovalUUID: "APR", ArgumentsSHA256: args,
			}
			capability := &scheduledGroupCapabilityStub{read: &application.AgentConversationReadV1{Found: true, TargetUUID: "G1", TargetType: model.MessageTargetGroup}}
			if scenario == "left" {
				capability.read.Found = false
			}
			if scenario == "denied" {
				capability.err = application.ErrAgentCapabilityDenied
			}
			if scenario == "wrong-group" {
				capability.read.TargetUUID = "G2"
			}
			var port application.AgentCapabilityV1 = capability
			if scenario == "missing-check" {
				port = nil
			}
			sender := &agentMessageCommandSenderStub{message: &model.Message{UUID: "MSG-G"}}
			service, err := NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: tool}, agentToolAuditResolverStub{invocation: application.AgentInvocationV1{TenantID: "dipole", PrincipalUUID: "U1", AgentUUID: "AI"}}, sender, port)
			if err != nil {
				t.Fatal(err)
			}
			request := application.AgentMessageCommandExecutionRequestV1{TaskUUID: "TASK", RunUUID: "RUN", InvocationUUID: "INV-G", Kind: application.AgentMessageCommandGroupReplyV1, Content: "summary", ConversationKey: "group:G1"}
			if scenario == "changed-content" {
				request.Content = "unapproved"
			}
			result, err := service.Execute(context.Background(), request)
			if scenario == "member" {
				if err != nil || result == nil || sender.command.ConversationKey != "group:G1" || capability.principal != "U1" {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			} else if !errors.Is(err, application.ErrAgentCommandDenied) || sender.command.CommandID != "" {
				t.Fatalf("denied publication sent a command: %+v err=%v", sender.command, err)
			}
		})
	}
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
	service, err := NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: tool}, agentToolAuditResolverStub{invocation: invocation}, sender, nil)
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
	tool.Status = application.AgentToolInvocationStatusCompleted
	tool.ActionReference = &application.AgentToolActionReferenceV1{
		ResourceType: application.AgentToolActionResourceMessage, ResourceUUID: result.MessageUUID,
		CommandKind: result.Kind, CommandID: result.CommandID,
	}
	sender.command = application.AgentMessageCommandV1{}
	request := application.AgentMessageCommandExecutionRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-1", Kind: application.AgentMessageCommandSystemMessageV1, Content: "notice",
	}
	replay, err := service.Execute(context.Background(), request)
	if err != nil || *replay != *result || sender.command.CommandID != "" {
		t.Fatalf("completed invocation replay=%+v err=%v command=%+v", replay, err, sender.command)
	}
	request.Content = "unapproved"
	if _, err := service.Execute(context.Background(), request); !errors.Is(err, application.ErrAgentCommandDenied) {
		t.Fatalf("completed invocation accepted changed content: %v", err)
	}
	request.Content = "notice"
	tool.ActionReference = nil
	if _, err := service.Execute(context.Background(), request); !errors.Is(err, application.ErrAgentCommandConflict) {
		t.Fatalf("completed invocation accepted missing receipt: %v", err)
	}
}

func TestAgentMessageCommandExecutionBindsAuthorizedAssistantReplyWithoutApproval(t *testing.T) {
	invocation := application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Permissions: []string{application.AgentPermissionMessageWrite},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: model.DirectConversationKey("U100", "UAI"), Actions: []string{application.AgentResourceActionWrite}}},
	}
	argumentsSHA, err := application.AgentMessageCommandToolArgumentsSHA256V1(invocation.PrincipalUUID, invocation.AgentUUID, "hello")
	if err != nil {
		t.Fatalf("derive Tool arguments digest: %v", err)
	}
	tool := &application.AgentToolInvocationV1{
		InvocationUUID: "INV-REPLY", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Transport: application.AgentToolTransportMCP, CapabilityID: application.AgentCapabilityAssistantReplySend, ArgumentsSHA256: argumentsSHA, Status: application.AgentToolInvocationStatusRunning,
	}
	sender := &agentMessageCommandSenderStub{message: &model.Message{UUID: "MSG-REPLY"}}
	service, err := NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: tool}, agentToolAuditResolverStub{invocation: invocation}, sender, nil)
	if err != nil {
		t.Fatalf("new Message Command execution: %v", err)
	}
	result, err := service.Execute(context.Background(), application.AgentMessageCommandExecutionRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-REPLY", Kind: application.AgentMessageCommandAssistantReplyV1, Content: "hello",
	})
	if err != nil || result.Kind != application.AgentMessageCommandAssistantReplyV1 || sender.command.Kind != application.AgentMessageCommandAssistantReplyV1 {
		t.Fatalf("assistant reply result=%+v command=%+v err=%v", result, sender.command, err)
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
		{name: "failed", edit: func(value *application.AgentToolInvocationV1) {
			value.Status = application.AgentToolInvocationStatusFailed
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
			service, _ := NewAgentMessageCommandExecutionV1(agentMessageCommandToolReaderStub{invocation: &tool}, agentToolAuditResolverStub{invocation: identity}, sender, nil)
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
	if _, err := NewAgentMessageCommandExecutionV1(nil, agentToolAuditResolverStub{}, &agentMessageCommandSenderStub{}, nil); err == nil {
		t.Fatal("expected missing Tool reader to fail")
	}
}
