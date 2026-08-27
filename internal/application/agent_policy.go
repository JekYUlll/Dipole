package application

import (
	"fmt"
	"strings"
)

type AgentCapabilityRiskV1 string

const (
	AgentCapabilityRiskRead        AgentCapabilityRiskV1 = "read"
	AgentCapabilityRiskWrite       AgentCapabilityRiskV1 = "write"
	AgentCapabilityRiskDestructive AgentCapabilityRiskV1 = "destructive"

	AgentCapabilityUserProfileRead    = "user.profile.read"
	AgentCapabilityDirectMessagesRead = "message.direct.read"
	AgentCapabilityConversationsList  = "conversation.list"
	AgentCapabilityConversationRead   = "conversation.read"
	AgentCapabilitySystemMessageSend  = "message.system.send"

	AgentPermissionUserProfileRead  = "user.profile.read"
	AgentPermissionConversationList = "conversation.list"
	AgentPermissionConversationRead = "conversation.read"
	AgentPermissionMessageWrite     = "message.write"
)

type AgentCapabilityDescriptorV1 struct {
	ID                 string
	Risk               AgentCapabilityRiskV1
	RequiredPermission string
	ApprovalRequired   bool
}

type AgentInvocationV1 struct {
	TenantID             string
	PrincipalUUID        string
	AgentUUID            string
	DelegatedByUUID      string
	Permissions          []string
	ApprovedCapabilities []string
	RequestID            string
	TraceID              string
	EventID              string
}

var agentCapabilityDescriptorsV1 = map[string]AgentCapabilityDescriptorV1{
	AgentCapabilityUserProfileRead: {
		ID: AgentCapabilityUserProfileRead, Risk: AgentCapabilityRiskRead, RequiredPermission: AgentPermissionUserProfileRead,
	},
	AgentCapabilityDirectMessagesRead: {
		ID: AgentCapabilityDirectMessagesRead, Risk: AgentCapabilityRiskRead, RequiredPermission: AgentPermissionConversationRead,
	},
	AgentCapabilityConversationsList: {
		ID: AgentCapabilityConversationsList, Risk: AgentCapabilityRiskRead, RequiredPermission: AgentPermissionConversationList,
	},
	AgentCapabilityConversationRead: {
		ID: AgentCapabilityConversationRead, Risk: AgentCapabilityRiskRead, RequiredPermission: AgentPermissionConversationRead,
	},
	AgentCapabilitySystemMessageSend: {
		ID: AgentCapabilitySystemMessageSend, Risk: AgentCapabilityRiskWrite, RequiredPermission: AgentPermissionMessageWrite,
	},
}

func AgentCapabilityDescriptorByIDV1(id string) (AgentCapabilityDescriptorV1, bool) {
	descriptor, ok := agentCapabilityDescriptorsV1[strings.TrimSpace(id)]
	return descriptor, ok
}

func AuthorizeAgentCapabilityV1(invocation AgentInvocationV1, descriptor AgentCapabilityDescriptorV1) error {
	if strings.TrimSpace(invocation.TenantID) == "" || strings.TrimSpace(invocation.PrincipalUUID) == "" || strings.TrimSpace(invocation.AgentUUID) == "" {
		return fmt.Errorf("%w: tenant, principal, and Agent identity are required", ErrAgentCapabilityDenied)
	}
	delegator := strings.TrimSpace(invocation.DelegatedByUUID)
	if delegator != "" && delegator != strings.TrimSpace(invocation.PrincipalUUID) {
		return fmt.Errorf("%w: delegation does not match principal", ErrAgentCapabilityDenied)
	}
	if strings.TrimSpace(descriptor.ID) == "" || !validAgentCapabilityRiskV1(descriptor.Risk) || strings.TrimSpace(descriptor.RequiredPermission) == "" {
		return fmt.Errorf("%w: invalid capability descriptor", ErrAgentCapabilityDenied)
	}
	if !containsAgentPolicyValue(invocation.Permissions, descriptor.RequiredPermission) {
		return fmt.Errorf("%w: missing permission %s", ErrAgentCapabilityDenied, descriptor.RequiredPermission)
	}
	if descriptor.ApprovalRequired || descriptor.Risk == AgentCapabilityRiskDestructive {
		if !containsAgentPolicyValue(invocation.ApprovedCapabilities, descriptor.ID) {
			return fmt.Errorf("%w: capability %s requires approval", ErrAgentCapabilityDenied, descriptor.ID)
		}
	}
	return nil
}

func validAgentCapabilityRiskV1(risk AgentCapabilityRiskV1) bool {
	return risk == AgentCapabilityRiskRead || risk == AgentCapabilityRiskWrite || risk == AgentCapabilityRiskDestructive
}

func containsAgentPolicyValue(values []string, expected string) bool {
	expected = strings.TrimSpace(expected)
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}
