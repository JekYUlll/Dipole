package application

import (
	"errors"
	"testing"
)

func TestAgentCapabilityV1DescriptorsAreVersionedAndRiskClassified(t *testing.T) {
	t.Parallel()

	want := map[string]AgentCapabilityRiskV1{
		AgentCapabilityUserProfileRead:    AgentCapabilityRiskRead,
		AgentCapabilityDirectMessagesRead: AgentCapabilityRiskRead,
		AgentCapabilityConversationsList:  AgentCapabilityRiskRead,
		AgentCapabilityConversationRead:   AgentCapabilityRiskRead,
		AgentCapabilitySystemMessageSend:  AgentCapabilityRiskWrite,
	}
	for id, risk := range want {
		descriptor, ok := AgentCapabilityDescriptorByIDV1(id)
		if !ok {
			t.Fatalf("missing descriptor %s", id)
		}
		if descriptor.ID != id || descriptor.Risk != risk || descriptor.RequiredPermission == "" {
			t.Fatalf("invalid descriptor %s: %+v", id, descriptor)
		}
	}
	if _, ok := AgentCapabilityDescriptorByIDV1("unknown"); ok {
		t.Fatal("unknown capability must not resolve")
	}
}

func TestAuthorizeAgentCapabilityV1(t *testing.T) {
	t.Parallel()

	base := AgentInvocationV1{
		TenantID:        "dipole",
		PrincipalUUID:   "U100",
		AgentUUID:       "UAI",
		DelegatedByUUID: "U100",
		Permissions:     []string{AgentPermissionConversationRead, AgentPermissionMessageWrite},
	}

	tests := []struct {
		name       string
		invocation AgentInvocationV1
		descriptor AgentCapabilityDescriptorV1
		wantErr    bool
	}{
		{name: "read allowed", invocation: base, descriptor: mustAgentDescriptor(t, AgentCapabilityConversationRead)},
		{name: "write allowed", invocation: base, descriptor: mustAgentDescriptor(t, AgentCapabilitySystemMessageSend)},
		{name: "missing tenant", invocation: mutateInvocation(base, func(v *AgentInvocationV1) { v.TenantID = "" }), descriptor: mustAgentDescriptor(t, AgentCapabilityConversationRead), wantErr: true},
		{name: "delegator mismatch", invocation: mutateInvocation(base, func(v *AgentInvocationV1) { v.DelegatedByUUID = "U999" }), descriptor: mustAgentDescriptor(t, AgentCapabilityConversationRead), wantErr: true},
		{name: "missing permission", invocation: mutateInvocation(base, func(v *AgentInvocationV1) { v.Permissions = nil }), descriptor: mustAgentDescriptor(t, AgentCapabilityConversationRead), wantErr: true},
		{name: "sensitive write needs approval", invocation: base, descriptor: AgentCapabilityDescriptorV1{ID: "message.bulk.send", Risk: AgentCapabilityRiskWrite, RequiredPermission: AgentPermissionMessageWrite, ApprovalRequired: true}, wantErr: true},
		{name: "destructive needs approval", invocation: base, descriptor: AgentCapabilityDescriptorV1{ID: "message.delete", Risk: AgentCapabilityRiskDestructive, RequiredPermission: AgentPermissionMessageWrite}, wantErr: true},
		{name: "approved destructive", invocation: mutateInvocation(base, func(v *AgentInvocationV1) { v.ApprovedCapabilities = []string{"message.delete"} }), descriptor: AgentCapabilityDescriptorV1{ID: "message.delete", Risk: AgentCapabilityRiskDestructive, RequiredPermission: AgentPermissionMessageWrite}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := AuthorizeAgentCapabilityV1(test.invocation, test.descriptor)
			if test.wantErr && !errors.Is(err, ErrAgentCapabilityDenied) {
				t.Fatalf("expected policy denial, got %v", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected authorization, got %v", err)
			}
		})
	}
}

func mustAgentDescriptor(t *testing.T, id string) AgentCapabilityDescriptorV1 {
	t.Helper()
	descriptor, ok := AgentCapabilityDescriptorByIDV1(id)
	if !ok {
		t.Fatalf("missing descriptor %s", id)
	}
	return descriptor
}

func mutateInvocation(base AgentInvocationV1, mutate func(*AgentInvocationV1)) AgentInvocationV1 {
	copy := base
	copy.Permissions = append([]string(nil), base.Permissions...)
	copy.ApprovedCapabilities = append([]string(nil), base.ApprovedCapabilities...)
	mutate(&copy)
	return copy
}
