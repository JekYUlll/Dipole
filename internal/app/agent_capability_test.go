package app

import (
	"context"
	"errors"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/correlation"
)

type agentCapabilityCoreStub struct {
	users     map[string]*model.User
	requested string
}

func (s *agentCapabilityCoreStub) GetUserByUUID(uuid string) (*model.User, error) {
	s.requested = uuid
	return s.users[uuid], nil
}
func (*agentCapabilityCoreStub) CanSendDirectMessage(string, string) (bool, error) { return false, nil }
func (*agentCapabilityCoreStub) GetGroupByUUID(string) (*model.Group, error)       { return nil, nil }
func (*agentCapabilityCoreStub) GetGroupMember(string, string) (*model.GroupMember, error) {
	return nil, nil
}
func (*agentCapabilityCoreStub) ListGroupMembers(string) ([]*model.GroupMember, error) {
	return nil, nil
}
func (*agentCapabilityCoreStub) GetOwnedFile(string, string) (*model.UploadedFile, error) {
	return nil, nil
}
func (*agentCapabilityCoreStub) ListSearchConversationKeys(string) ([]string, error) { return nil, nil }

type agentCapabilityMessagesStub struct {
	directTarget string
	groupTarget  string
	items        []*model.Message
}

type agentCapabilityCommandsStub struct {
	command application.AgentMessageCommandV1
	ids     correlation.IDs
}

func (s *agentCapabilityCommandsStub) SendMessage(ctx context.Context, command application.AgentMessageCommandV1) (*model.Message, error) {
	s.command = command
	s.ids = correlation.FromContext(ctx)
	return &model.Message{UUID: "M-SYSTEM", MessageType: model.MessageTypeSystem}, nil
}

func (s *agentCapabilityMessagesStub) ListDirectMessages(_ string, target string, _ uint, _ int) ([]*model.Message, error) {
	s.directTarget = target
	return s.items, nil
}
func (s *agentCapabilityMessagesStub) ListGroupMessages(_ string, target string, _ uint, _ int) ([]*model.Message, error) {
	s.groupTarget = target
	return s.items, nil
}

type agentCapabilityConversationsStub struct {
	items []*model.Conversation
	found *model.Conversation
}

func (s *agentCapabilityConversationsStub) ListForAgent(string, int) ([]*model.Conversation, error) {
	return s.items, nil
}
func (s *agentCapabilityConversationsStub) FindForUser(string, string) (*model.Conversation, error) {
	return s.found, nil
}

func TestLocalAgentCapabilityV1RestrictsProfileSubjects(t *testing.T) {
	t.Parallel()

	core := &agentCapabilityCoreStub{users: map[string]*model.User{
		"U100": {UUID: "U100"},
		"UAI":  {UUID: "UAI", UserType: model.UserTypeAssistant},
	}}
	capability, err := NewLocalAgentCapabilityV1(core, &agentCapabilityMessagesStub{}, &agentCapabilityConversationsStub{}, &agentCapabilityCommandsStub{})
	if err != nil {
		t.Fatalf("new Agent Capability: %v", err)
	}
	invocation := agentCapabilityTestInvocation()
	if _, err := capability.GetUserProfile(context.Background(), invocation, "U100"); err != nil || core.requested != "U100" {
		t.Fatalf("principal profile: requested=%q err=%v", core.requested, err)
	}
	if _, err := capability.GetUserProfile(context.Background(), invocation, "UAI"); err != nil || core.requested != "UAI" {
		t.Fatalf("Agent profile: requested=%q err=%v", core.requested, err)
	}
	if _, err := capability.GetUserProfile(context.Background(), invocation, "U999"); !errors.Is(err, application.ErrAgentCapabilityDenied) {
		t.Fatalf("expected foreign profile denial, got %v", err)
	}
}

func TestLocalAgentCapabilityV1RoutesAuthorizedConversationReads(t *testing.T) {
	t.Parallel()

	messages := &agentCapabilityMessagesStub{items: []*model.Message{{UUID: "M1"}}}
	conversations := &agentCapabilityConversationsStub{}
	capability, err := NewLocalAgentCapabilityV1(&agentCapabilityCoreStub{}, messages, conversations, &agentCapabilityCommandsStub{})
	if err != nil {
		t.Fatalf("new Agent Capability: %v", err)
	}

	invocation := agentCapabilityTestInvocation()
	denied, err := capability.ReadConversation(context.Background(), invocation, "U999", 20)
	if err != nil || denied.Found || messages.directTarget != "" || messages.groupTarget != "" {
		t.Fatalf("denied read reached Message Application: result=%+v direct=%q group=%q err=%v", denied, messages.directTarget, messages.groupTarget, err)
	}

	conversations.found = &model.Conversation{TargetUUID: "U200", TargetType: model.MessageTargetDirect}
	direct, err := capability.ReadConversation(context.Background(), invocation, "U200", 20)
	if err != nil || !direct.Found || messages.directTarget != "U200" || len(direct.Messages) != 1 {
		t.Fatalf("direct read: result=%+v target=%q err=%v", direct, messages.directTarget, err)
	}

	conversations.found = &model.Conversation{TargetUUID: "G1", TargetType: model.MessageTargetGroup}
	group, err := capability.ReadConversation(context.Background(), invocation, "G1", 20)
	if err != nil || !group.Found || messages.groupTarget != "G1" || len(group.Messages) != 1 {
		t.Fatalf("group read: result=%+v target=%q err=%v", group, messages.groupTarget, err)
	}
}

func TestLocalAgentCapabilityV1ListsConversationsAndPreservesCommandContext(t *testing.T) {
	t.Parallel()

	messages := &agentCapabilityMessagesStub{}
	conversations := &agentCapabilityConversationsStub{items: []*model.Conversation{
		{TargetUUID: "U200"},
	}}
	commands := &agentCapabilityCommandsStub{}
	capability, err := NewLocalAgentCapabilityV1(&agentCapabilityCoreStub{}, messages, conversations, commands)
	if err != nil {
		t.Fatalf("new Agent Capability: %v", err)
	}
	invocation := agentCapabilityTestInvocation()
	items, err := capability.ListConversations(context.Background(), invocation, 10)
	if err != nil || len(items) != 1 || items[0].TargetUUID != "U200" {
		t.Fatalf("list conversations: items=%+v err=%v", items, err)
	}

	ctx := correlation.WithContext(context.Background(), correlation.IDs{RequestID: "R1", TraceID: "T1"})
	message, err := capability.SendSystemMessage(ctx, invocation, "notice")
	if err != nil || message.UUID != "M-SYSTEM" {
		t.Fatalf("send system message: message=%+v err=%v", message, err)
	}
	if commands.command.Kind != application.AgentMessageCommandSystemMessageV1 || commands.command.Invocation.AgentUUID != "UAI" || commands.command.Invocation.PrincipalUUID != "U100" || commands.command.Content != "notice" || commands.ids.RequestID != "R1" || commands.ids.TraceID != "T1" {
		t.Fatalf("command boundary lost identity or correlation: command=%+v ids=%+v", commands.command, commands.ids)
	}
	if commands.command.CommandID == "" {
		t.Fatal("system capability did not provide a stable command ID")
	}
}

func TestLocalAgentCapabilityV1EnforcesInvocationPolicy(t *testing.T) {
	t.Parallel()

	capability, err := NewLocalAgentCapabilityV1(&agentCapabilityCoreStub{}, &agentCapabilityMessagesStub{}, &agentCapabilityConversationsStub{}, &agentCapabilityCommandsStub{})
	if err != nil {
		t.Fatalf("new Agent Capability: %v", err)
	}
	invocation := agentCapabilityTestInvocation()
	invocation.Permissions = []string{application.AgentPermissionUserProfileRead}
	if _, err := capability.ListConversations(context.Background(), invocation, 10); !errors.Is(err, application.ErrAgentCapabilityDenied) {
		t.Fatalf("expected adapter policy denial, got %v", err)
	}
}

func agentCapabilityTestInvocation() application.AgentInvocationV1 {
	return application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", DelegatedByUUID: "U100",
		Permissions: []string{
			application.AgentPermissionUserProfileRead,
			application.AgentPermissionConversationList,
			application.AgentPermissionConversationRead,
			application.AgentPermissionMessageWrite,
		},
	}
}

func TestNewLocalAgentCapabilityV1RejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	core := &agentCapabilityCoreStub{}
	messages := &agentCapabilityMessagesStub{}
	conversations := &agentCapabilityConversationsStub{}
	commands := &agentCapabilityCommandsStub{}
	if _, err := NewLocalAgentCapabilityV1(nil, messages, conversations, commands); err == nil {
		t.Fatal("expected missing Core dependency rejection")
	}
	if _, err := NewLocalAgentCapabilityV1(core, nil, conversations, commands); err == nil {
		t.Fatal("expected missing Message dependency rejection")
	}
	if _, err := NewLocalAgentCapabilityV1(core, messages, nil, commands); err == nil {
		t.Fatal("expected missing Conversation dependency rejection")
	}
	if _, err := NewLocalAgentCapabilityV1(core, messages, conversations, nil); err == nil {
		t.Fatal("expected missing Command dependency rejection")
	}
}
