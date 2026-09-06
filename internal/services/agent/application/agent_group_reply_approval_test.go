package agentapplication_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

// TestAutoApproveGroupReplyAllowsLowRiskAgentOwnedDefinition verifies that a
// group @-mention interactive task pinned to the shared low-risk assistant
// Definition gets an already-approved group_reply grant scoped to the group
// conversation. Route B/B2.
func TestAutoApproveGroupReplyAllowsLowRiskAgentOwnedDefinition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: agentapplication.LowRiskAssistantDefinitionUUIDV1, Version: 1,
		TenantID: "dipole", OwnerUUID: "UAI00000000000000LOW01", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	store := &agentPolicyStoreStub{
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(definition.DefinitionUUID, definition.Version): &definition,
		},
		tasks: map[string]*application.AgentTaskV1{"TASK-G1": {
			TaskUUID: "TASK-G1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U200", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "agent.interactive.requested", TriggerRef: "interactive:group-abc",
		}},
		runs: map[string]*application.AgentRunV1{"RUN-G1": {
			RunUUID: "RUN-G1", TaskUUID: "TASK-G1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
		}},
	}
	scope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation, ResourceID: model.GroupConversationKey("G-ROOM-1"),
		Actions:      []string{application.AgentResourceActionWrite},
	}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	request := application.AgentApprovalRequestV1{
		TaskUUID: "TASK-G1", RunUUID: "RUN-G1", RuntimeID: "dipole-agent", Mode: "active",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: "approval:group-1", TaskUUID: "TASK-G1", CapabilityID: application.AgentCapabilityGroupReplySend,
			ResourceScope: scope, ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
			ExpiresAt: now.Add(30 * time.Minute),
		},
	}
	service, err := agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new Approval service: %v", err)
	}
	grant, err := service.AutoApproveGroupReply(context.Background(), request)
	if err != nil {
		t.Fatalf("group reply approval: %v", err)
	}
	if grant.Status != application.AgentApprovalStatusApproved || grant.ApprovedByUUID != "U200" {
		t.Fatalf("group grant = %+v", grant)
	}
	if grant.ResourceScope.ResourceID != "group:G-ROOM-1" {
		t.Fatalf("group grant scope = %+v", grant.ResourceScope)
	}
}

// TestAutoApproveGroupReplyRejectsNonGroupScope verifies that a group_reply
// grant whose scope is not a group conversation is denied.
func TestAutoApproveGroupReplyRejectsNonGroupScope(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: agentapplication.LowRiskAssistantDefinitionUUIDV1, Version: 1,
		TenantID: "dipole", OwnerUUID: "UAI00000000000000LOW01", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	store := &agentPolicyStoreStub{
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(definition.DefinitionUUID, definition.Version): &definition,
		},
		tasks: map[string]*application.AgentTaskV1{"TASK-G2": {
			TaskUUID: "TASK-G2", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U200", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "agent.interactive.requested", TriggerRef: "interactive:group-def",
		}},
		runs: map[string]*application.AgentRunV1{"RUN-G2": {
			RunUUID: "RUN-G2", TaskUUID: "TASK-G2", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
		}},
	}
	// A direct conversation scope must be rejected for a group reply.
	scope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation, ResourceID: model.DirectConversationKey("U200", "UAI"),
		Actions:      []string{application.AgentResourceActionWrite},
	}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	request := application.AgentApprovalRequestV1{
		TaskUUID: "TASK-G2", RunUUID: "RUN-G2", RuntimeID: "dipole-agent", Mode: "active",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: "approval:group-2", TaskUUID: "TASK-G2", CapabilityID: application.AgentCapabilityGroupReplySend,
			ResourceScope: scope, ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
			ExpiresAt: now.Add(30 * time.Minute),
		},
	}
	service, err := agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new Approval service: %v", err)
	}
	if _, err := service.AutoApproveGroupReply(context.Background(), request); err == nil {
		t.Fatalf("group reply with direct scope must be rejected")
	}
}

// TestAutoApproveGroupReplyRejectsNonLowRiskDefinition verifies that a group
// reply pinned to a per-user owner Definition (not the shared low-risk
// assistant) is denied, so group replies cannot widen under a user grant.
func TestAutoApproveGroupReplyRejectsNonLowRiskDefinition(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "definition:owner-u200", Version: 1,
		TenantID: "dipole", OwnerUUID: "U200", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive,
		Permissions: []string{application.AgentPermissionConversationRead, application.AgentPermissionMessageWrite},
		Scopes: []application.AgentResourceScopeV1{{
			ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
			Actions: []string{application.AgentResourceActionRead, application.AgentResourceActionWrite},
		}},
		ValidFrom: now.Add(-time.Hour),
	}
	store := &agentPolicyStoreStub{
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(definition.DefinitionUUID, definition.Version): &definition,
		},
		tasks: map[string]*application.AgentTaskV1{"TASK-G3": {
			TaskUUID: "TASK-G3", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U200", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "agent.interactive.requested", TriggerRef: "interactive:group-owner",
		}},
		runs: map[string]*application.AgentRunV1{"RUN-G3": {
			RunUUID: "RUN-G3", TaskUUID: "TASK-G3", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
		}},
	}
	scope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation, ResourceID: model.GroupConversationKey("G-ROOM-2"),
		Actions:      []string{application.AgentResourceActionWrite},
	}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	request := application.AgentApprovalRequestV1{
		TaskUUID: "TASK-G3", RunUUID: "RUN-G3", RuntimeID: "dipole-agent", Mode: "active",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: "approval:group-3", TaskUUID: "TASK-G3", CapabilityID: application.AgentCapabilityGroupReplySend,
			ResourceScope: scope, ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
			ExpiresAt: now.Add(30 * time.Minute),
		},
	}
	service, err := agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new Approval service: %v", err)
	}
	if _, err := service.AutoApproveGroupReply(context.Background(), request); err == nil {
		t.Fatalf("group reply under a per-user Definition must be rejected")
	}
}
