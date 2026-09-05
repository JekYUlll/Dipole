package application

import (
	"errors"
	"testing"
	"time"
)

func TestProjectAgentApprovedCapabilitiesV1UsesExplicitWriteAllowlist(t *testing.T) {
	t.Parallel()
	definition := AgentDefinitionVersionV1{DefinitionUUID: "DEF-1", Version: 1, TenantID: "dipole", OwnerUUID: "U1", AgentUUID: "UAI",
		Status: AgentDefinitionStatusActive, Permissions: []string{AgentPermissionMessageWrite},
		Scopes: []AgentResourceScopeV1{{ResourceType: AgentResourceTypeConversation, ResourceID: "direct:U1:UAI", Actions: []string{AgentResourceActionWrite}}}, ValidFrom: time.Unix(1, 0)}
	capabilities, err := ProjectAgentApprovedCapabilitiesV1(definition)
	if err != nil || len(capabilities) != 2 || capabilities[0] != AgentCapabilitySystemMessageSend || capabilities[1] != AgentCapabilityAssistantReplySend {
		t.Fatalf("capabilities=%v err=%v", capabilities, err)
	}
	definition.Scopes[0].Actions = []string{AgentResourceActionRead}
	if capabilities, err := ProjectAgentApprovedCapabilitiesV1(definition); err != nil || len(capabilities) != 0 {
		t.Fatalf("read-only projection=%v err=%v", capabilities, err)
	}
	definition.Scopes[0].Actions = []string{AgentResourceActionWrite}
	definition.Permissions = []string{AgentPermissionConversationRead}
	if capabilities, err := ProjectAgentApprovedCapabilitiesV1(definition); err != nil || len(capabilities) != 0 {
		t.Fatalf("missing-permission projection=%v err=%v", capabilities, err)
	}
}

func TestValidateAgentApprovedCapabilitiesV1RejectsModeAndAllowlistDrift(t *testing.T) {
	t.Parallel()
	if err := ValidateAgentApprovedCapabilitiesV1("active", []string{AgentCapabilitySystemMessageSend}); err != nil {
		t.Fatalf("valid active projection: %v", err)
	}
	for _, test := range []struct {
		mode string
		ids  []string
	}{
		{mode: "shadow", ids: []string{AgentCapabilitySystemMessageSend}},
		{mode: "active", ids: []string{"message.future.send"}},
		{mode: "active", ids: []string{AgentCapabilitySystemMessageSend, AgentCapabilitySystemMessageSend}},
	} {
		if err := ValidateAgentApprovedCapabilitiesV1(test.mode, test.ids); !errors.Is(err, ErrAgentCapabilityDenied) {
			t.Fatalf("Validate(%s, %v) = %v", test.mode, test.ids, err)
		}
	}
}

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

func TestAuthorizeAgentCapabilityForResourceV1(t *testing.T) {
	t.Parallel()

	descriptor := mustAgentDescriptor(t, AgentCapabilityConversationRead)
	base := AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions: []string{AgentPermissionConversationRead},
		ResourceScopes: []AgentResourceScopeV1{
			{ResourceType: AgentResourceTypeConversation, ResourceID: "group:G1", Actions: []string{AgentResourceActionRead}},
			{ResourceType: AgentResourceTypeConversation, ResourceID: AgentResourceWildcard, Actions: []string{AgentResourceActionList}},
		},
	}

	tests := []struct {
		name         string
		invocation   AgentInvocationV1
		resourceType string
		resourceID   string
		action       string
		wantErr      bool
	}{
		{name: "exact scope", invocation: base, resourceType: AgentResourceTypeConversation, resourceID: "group:G1", action: AgentResourceActionRead},
		{name: "wildcard resource", invocation: base, resourceType: AgentResourceTypeConversation, resourceID: AgentResourceWildcard, action: AgentResourceActionList},
		{name: "wildcard applies to concrete resource", invocation: base, resourceType: AgentResourceTypeConversation, resourceID: "direct:U100:U200", action: AgentResourceActionList},
		{name: "different resource denied", invocation: base, resourceType: AgentResourceTypeConversation, resourceID: "group:G2", action: AgentResourceActionRead, wantErr: true},
		{name: "different action denied", invocation: base, resourceType: AgentResourceTypeConversation, resourceID: "group:G1", action: AgentResourceActionWrite, wantErr: true},
		{name: "different resource type denied", invocation: base, resourceType: AgentResourceTypeUser, resourceID: "group:G1", action: AgentResourceActionRead, wantErr: true},
		{name: "empty scopes denied", invocation: mutateInvocation(base, func(v *AgentInvocationV1) { v.ResourceScopes = nil }), resourceType: AgentResourceTypeConversation, resourceID: "group:G1", action: AgentResourceActionRead, wantErr: true},
		{name: "empty request denied", invocation: base, resourceType: AgentResourceTypeConversation, resourceID: "", action: AgentResourceActionRead, wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := AuthorizeAgentCapabilityForResourceV1(test.invocation, descriptor, test.resourceType, test.resourceID, test.action)
			if test.wantErr && !errors.Is(err, ErrAgentCapabilityDenied) {
				t.Fatalf("expected resource policy denial, got %v", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected resource authorization, got %v", err)
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
	copy.ResourceScopes = append([]AgentResourceScopeV1(nil), base.ResourceScopes...)
	mutate(&copy)
	return copy
}
