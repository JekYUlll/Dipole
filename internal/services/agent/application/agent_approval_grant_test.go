package agentapplication

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestPersistentAgentApprovalGrantResolverReturnsOneExactActiveGrant(t *testing.T) {
	now := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	scope := application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"write"}}
	scopeHash, _ := application.AgentResourceScopeSHA256V1(scope)
	store := &agentApprovalGrantStoreStub{
		run:  &application.AgentRunV1{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning},
		task: &application.AgentTaskV1{TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusRunning},
		grants: []application.AgentApprovalV1{{
			ApprovalUUID: "APR-1", TaskUUID: "TASK-1", CapabilityID: "message.system.send", ResourceScope: scope,
			ScopeSHA256: scopeHash, ArgumentsSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64),
			Status: application.AgentApprovalStatusApproved, ApprovedByUUID: "U100", ExpiresAt: now.Add(time.Minute),
		}},
	}
	resolver, err := NewPersistentAgentApprovalGrantResolverV1WithClock(store, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	grant, err := resolver.ResolveGrant(context.Background(), application.AgentApprovalGrantRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: "active",
		CapabilityID: "message.system.send", ResourceScope: scope, ArgumentsSHA256: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatalf("resolve grant: %v", err)
	}
	if grant.ApprovalUUID != "APR-1" || grant.NonceSHA256 != strings.Repeat("b", 64) {
		t.Fatalf("unexpected grant: %+v", grant)
	}
	if store.limit != 2 || store.scopeSHA256 != scopeHash {
		t.Fatalf("expected bounded exact lookup, got limit=%d scope=%s", store.limit, store.scopeSHA256)
	}
}

func TestPersistentAgentApprovalGrantResolverFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	scope := application.AgentResourceScopeV1{ResourceType: "conversation", ResourceID: "direct:U100:UAI", Actions: []string{"write"}}
	valid := application.AgentApprovalV1{ApprovalUUID: "APR-1", TaskUUID: "TASK-1", CapabilityID: "message.system.send", ResourceScope: scope}
	tests := []struct {
		name   string
		store  agentApprovalGrantStoreStub
		mode   string
		grants []application.AgentApprovalV1
	}{
		{name: "shadow run", mode: "active", store: agentApprovalGrantStoreStub{run: &application.AgentRunV1{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}}},
		{name: "no grant", mode: "active", store: agentApprovalGrantStoreStub{run: activeApprovalRun(), task: activeApprovalTask()}},
		{name: "ambiguous grant", mode: "active", store: agentApprovalGrantStoreStub{run: activeApprovalRun(), task: activeApprovalTask()}, grants: []application.AgentApprovalV1{valid, valid}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.store.grants = test.grants
			resolver, _ := NewPersistentAgentApprovalGrantResolverV1WithClock(&test.store, func() time.Time { return now })
			_, err := resolver.ResolveGrant(context.Background(), application.AgentApprovalGrantRequestV1{
				TaskUUID: "TASK-1", RunUUID: "RUN-1", RuntimeID: "dipole-agent", Mode: test.mode,
				CapabilityID: "message.system.send", ResourceScope: scope, ArgumentsSHA256: strings.Repeat("a", 64),
			})
			if !errors.Is(err, application.ErrAgentApprovalDenied) {
				t.Fatalf("expected denied, got %v", err)
			}
		})
	}
}

type agentApprovalGrantStoreStub struct {
	run         *application.AgentRunV1
	task        *application.AgentTaskV1
	grants      []application.AgentApprovalV1
	scopeSHA256 string
	limit       int
}

func (s *agentApprovalGrantStoreStub) GetRun(context.Context, string) (*application.AgentRunV1, error) {
	return s.run, nil
}
func (s *agentApprovalGrantStoreStub) GetTask(context.Context, string) (*application.AgentTaskV1, error) {
	return s.task, nil
}
func (s *agentApprovalGrantStoreStub) ListApprovedAgentApprovalGrants(_ context.Context, taskUUID, capabilityID, scopeSHA256, argumentsSHA256 string, _ time.Time, limit int) ([]application.AgentApprovalV1, error) {
	s.scopeSHA256, s.limit = scopeSHA256, limit
	return s.grants, nil
}

func activeApprovalRun() *application.AgentRunV1 {
	return &application.AgentRunV1{RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "active", Status: application.AgentRunStatusRunning}
}

func activeApprovalTask() *application.AgentTaskV1 {
	return &application.AgentTaskV1{TaskUUID: "TASK-1", PrincipalUUID: "U100", Status: application.AgentTaskStatusRunning}
}
