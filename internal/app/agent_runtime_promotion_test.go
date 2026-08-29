package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

type runtimePromotionStoreStub struct {
	lookup application.AgentRuntimePromotionGrantLookupV1
	grant  *application.AgentRuntimePromotionGrantV1
	err    error
}

func (s *runtimePromotionStoreStub) CreateRuntimePromotionGrant(context.Context, application.AgentRuntimePromotionGrantV1) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *runtimePromotionStoreStub) GetActiveRuntimePromotionGrant(_ context.Context, lookup application.AgentRuntimePromotionGrantLookupV1) (*application.AgentRuntimePromotionGrantV1, error) {
	s.lookup = lookup
	return s.grant, s.err
}

func (s *runtimePromotionStoreStub) RevokeRuntimePromotionGrant(context.Context, string, time.Time) (bool, error) {
	return false, errors.New("not implemented")
}

func TestPersistentAgentActiveRunPromotionAuthorizer(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	grant := application.AgentRuntimePromotionGrantV1{
		GrantUUID: "PROMOTE-1", TenantID: "dipole", RuntimeID: "dipole-agent", CandidateVersion: "runtime-v7",
		DefinitionUUID: "DEF-UAI", DefinitionVersion: 7, PolicyVersion: application.AgentRuntimePromotionPolicyVersionV2,
		EvidenceSHA256: strings.Repeat("a", 64), EvalSuiteSHA256: strings.Repeat("b", 64),
		GrantedByUUID: "OPERATOR-1", ReviewedByUUID: "OPERATOR-2", ValidFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	store := &runtimePromotionStoreStub{grant: &grant}
	authorizer, err := agentapplication.NewPersistentAgentActiveRunPromotionAuthorizerV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new promotion authorizer: %v", err)
	}
	request := activeRunPromotionRequestV1()
	if err := authorizer.AuthorizeActiveRun(context.Background(), request); err != nil {
		t.Fatalf("authorize active Run: %v", err)
	}
	if store.lookup.TenantID != "dipole" || store.lookup.RuntimeID != "dipole-agent" || store.lookup.CandidateVersion != "runtime-v7" ||
		store.lookup.DefinitionUUID != "DEF-UAI" || store.lookup.DefinitionVersion != 7 || !store.lookup.At.Equal(now) {
		t.Fatalf("promotion lookup drifted: %+v", store.lookup)
	}

	store.grant = nil
	if err := authorizer.AuthorizeActiveRun(context.Background(), request); !errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("missing grant error = %v, want policy denied", err)
	}
	store.err = errors.New("database unavailable")
	if err := authorizer.AuthorizeActiveRun(context.Background(), request); err == nil || errors.Is(err, application.ErrAgentExecutionPolicyDenied) {
		t.Fatalf("store failure error = %v, want internal failure", err)
	}
}

func activeRunPromotionRequestV1() application.AgentActiveRunPromotionRequestV1 {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	return application.AgentActiveRunPromotionRequestV1{
		RuntimeID: "dipole-agent", CandidateVersion: "runtime-v7",
		Task: application.AgentTaskV1{
			TaskUUID: "TASK-1", DefinitionUUID: "DEF-UAI", DefinitionVersion: 7, TenantID: "dipole", PrincipalUUID: "U100",
			AgentUUID: "UAI", Status: application.AgentTaskStatusCreated, TriggerType: "manual", TriggerRef: "promotion", Goal: "promotion",
		},
		Definition: application.AgentDefinitionVersionV1{
			DefinitionUUID: "DEF-UAI", Version: 7, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI",
			Status: application.AgentDefinitionStatusActive, Permissions: []string{application.AgentPermissionMessageWrite},
			Scopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"write"}}}, ValidFrom: now,
		},
	}
}
