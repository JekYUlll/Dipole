package agentapplication_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

func subscriptionMessageApprovalFixture(now time.Time) (*agentPolicyStoreStub, application.AgentApprovalRequestV1) {
	definition := activeAgentDefinitionV1(1, now.Add(-time.Hour), []string{application.AgentPermissionMessageWrite})
	definition.OwnerUUID = "U100"
	definition.Scopes = []application.AgentResourceScopeV1{{
		ResourceType: application.AgentResourceTypeConversation, ResourceID: application.AgentResourceWildcard,
		Actions: []string{application.AgentResourceActionWrite},
	}}
	store := &agentPolicyStoreStub{
		definitions: map[string]*application.AgentDefinitionVersionV1{
			definitionKeyV1(definition.DefinitionUUID, definition.Version): &definition,
		},
		tasks: map[string]*application.AgentTaskV1{"TASK-1": {
			TaskUUID: "TASK-1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning,
			TriggerType: "message.direct.created", TriggerRef: "M1", TriggerSubscriptionUUID: "SUB-1",
		}},
		runs: map[string]*application.AgentRunV1{"RUN-1": {
			RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning,
		}},
		subscriptions: map[string]*application.AgentEventSubscriptionV1{"SUB-1": {
			SubscriptionUUID: "SUB-1", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version,
			TenantID: "dipole", AgentUUID: "UAI", Status: application.AgentSubscriptionStatusActive,
			EventType: "message.direct.created", ResourceType: "conversation", ResourceID: "*",
			FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: []byte(`{}`), CreatedByUUID: "U100",
		}},
	}
	scope := application.AgentResourceScopeV1{
		ResourceType: application.AgentResourceTypeConversation, ResourceID: model.DirectConversationKey("U100", "UAI"),
		Actions: []string{application.AgentResourceActionWrite},
	}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	request := application.AgentApprovalRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "active",
		Approval: application.AgentApprovalV1{
			ApprovalUUID: "approval:sub-1", TaskUUID: "TASK-1", CapabilityID: application.AgentCapabilitySystemMessageSend,
			ResourceScope: scope, ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
			ExpiresAt: now.Add(30 * time.Minute),
		},
	}
	return store, request
}

func TestAutoApproveSubscriptionMessageMintsApprovedGrantIdempotently(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	store, request := subscriptionMessageApprovalFixture(now)
	service, err := agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new Approval service: %v", err)
	}
	grant, err := service.AutoApproveSubscriptionMessage(context.Background(), request)
	if err != nil || grant.Status != application.AgentApprovalStatusApproved || grant.ApprovedByUUID != "U100" {
		t.Fatalf("auto approve subscription message: grant=%+v err=%v", grant, err)
	}
	if grant.ResourceScope.ResourceID != model.DirectConversationKey("U100", "UAI") {
		t.Fatalf("grant scope not owner Agent conversation: %+v", grant.ResourceScope)
	}
	stored := store.approvals["approval:sub-1"]
	if stored == nil || stored.Status != application.AgentApprovalStatusApproved || stored.ApprovedByUUID != "U100" {
		t.Fatalf("grant not persisted approved: %+v", stored)
	}
	replay, err := service.AutoApproveSubscriptionMessage(context.Background(), request)
	if err != nil || replay.ApprovalUUID != "approval:sub-1" {
		t.Fatalf("idempotent replay: grant=%+v err=%v", replay, err)
	}
	conflict := request
	conflict.Approval.ArgumentsSHA256 = strings.Repeat("c", 64)
	if _, err := service.AutoApproveSubscriptionMessage(context.Background(), conflict); !errors.Is(err, application.ErrAgentApprovalDenied) {
		t.Fatalf("conflicting binding error = %v, want denial", err)
	}
}

func TestAutoApproveSubscriptionMessageRejectsInvalidBindings(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	cases := map[string]func(store *agentPolicyStoreStub, request *application.AgentApprovalRequestV1){
		"non-active mode": func(_ *agentPolicyStoreStub, request *application.AgentApprovalRequestV1) {
			request.Mode = "shadow"
			request.RunUUID = "RUN-1"
		},
		"task not subscription-triggered": func(store *agentPolicyStoreStub, _ *application.AgentApprovalRequestV1) {
			store.tasks["TASK-1"].TriggerSubscriptionUUID = ""
		},
		"subscription owner mismatch": func(store *agentPolicyStoreStub, _ *application.AgentApprovalRequestV1) {
			store.subscriptions["SUB-1"].CreatedByUUID = "U999"
		},
		"definition owner mismatch": func(store *agentPolicyStoreStub, _ *application.AgentApprovalRequestV1) {
			for _, definition := range store.definitions {
				definition.OwnerUUID = "U999"
			}
		},
		"definition does not authorize writes": func(store *agentPolicyStoreStub, _ *application.AgentApprovalRequestV1) {
			for _, definition := range store.definitions {
				definition.Permissions = []string{application.AgentPermissionConversationRead}
			}
		},
		"scope not owner conversation": func(_ *agentPolicyStoreStub, request *application.AgentApprovalRequestV1) {
			otherScope := application.AgentResourceScopeV1{
				ResourceType: application.AgentResourceTypeConversation, ResourceID: "direct:U100:UOTHER",
				Actions: []string{application.AgentResourceActionWrite},
			}
			otherHash, _ := application.AgentResourceScopeSHA256V1(otherScope)
			request.Approval.ResourceScope = otherScope
			request.Approval.ScopeSHA256 = otherHash
		},
		"wrong capability": func(_ *agentPolicyStoreStub, request *application.AgentApprovalRequestV1) {
			request.Approval.CapabilityID = "message.bulk.send"
		},
		"expired grant": func(_ *agentPolicyStoreStub, request *application.AgentApprovalRequestV1) {
			request.Approval.ExpiresAt = now.Add(-time.Minute)
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			store, request := subscriptionMessageApprovalFixture(now)
			mutate(store, &request)
			service, err := agentapplication.NewPersistentAgentApprovalServiceV1WithClock(store, func() time.Time { return now })
			if err != nil {
				t.Fatalf("new Approval service: %v", err)
			}
			if _, err := service.AutoApproveSubscriptionMessage(context.Background(), request); !errors.Is(err, application.ErrAgentApprovalDenied) {
				t.Fatalf("%s error = %v, want denial", name, err)
			}
			if _, minted := store.approvals["approval:sub-1"]; minted {
				t.Fatalf("%s unexpectedly minted a grant", name)
			}
		})
	}
}
