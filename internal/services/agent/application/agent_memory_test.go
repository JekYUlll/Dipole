package agentapplication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type agentMemoryStoreStub struct {
	query application.AgentMemoryQueryV1
	items []application.AgentMemoryV1
}

func (*agentMemoryStoreStub) CreateMemory(context.Context, application.AgentMemoryV1) error {
	return nil
}
func (*agentMemoryStoreStub) RevokeMemory(context.Context, string, time.Time) error { return nil }
func (s *agentMemoryStoreStub) ListContextMemories(_ context.Context, query application.AgentMemoryQueryV1) ([]application.AgentMemoryV1, error) {
	s.query = query
	return append([]application.AgentMemoryV1(nil), s.items...), nil
}

type agentMemoryInvocationResolverStub struct {
	invocation application.AgentInvocationV1
	err        error
}

type agentMemoryTaskReaderStub struct{ task *application.AgentTaskV1 }

func (s agentMemoryTaskReaderStub) GetTask(context.Context, string) (*application.AgentTaskV1, error) {
	return s.task, nil
}

func (s agentMemoryInvocationResolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return s.invocation, s.err
}

func TestPersistentAgentMemoryResolverUsesPinnedInvocationIdentity(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	store := &agentMemoryStoreStub{items: []application.AgentMemoryV1{
		{MemoryUUID: "MEM-B", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", MemoryType: application.AgentMemoryTypeSemantic,
			Status: application.AgentMemoryStatusActive, ResourceType: "conversation", ResourceID: "group:G1", Content: "second", Priority: 10,
			Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M2"}, ValidFrom: now.Add(-time.Hour), CreatedAt: now.Add(-time.Minute)},
		{MemoryUUID: "MEM-A", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", MemoryType: application.AgentMemoryTypeEpisodic,
			Status: application.AgentMemoryStatusActive, ResourceType: "conversation", ResourceID: "group:G1", Content: "first", Priority: 10,
			Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M1"}, ValidFrom: now.Add(-time.Hour), CreatedAt: now.Add(-time.Minute)},
	}}
	task := &application.AgentTaskV1{TaskUUID: "TASK-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", CreatedAt: now}
	resolver, err := NewPersistentAgentMemoryResolverV1(store, agentMemoryInvocationResolverStub{invocation: application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions:    []string{application.AgentPermissionConversationRead},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "group:G1", Actions: []string{"read"}}},
	}}, agentMemoryTaskReaderStub{task: task}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	items, err := resolver.ResolveContextMemories(context.Background(), "TASK-1", "RUN-1", "conversation", "group:G1", 20)
	if err != nil {
		t.Fatalf("resolve Memories: %v", err)
	}
	if store.query.TenantID != "dipole" || store.query.PrincipalUUID != "U100" || store.query.AgentUUID != "UAI" || store.query.CreatedBefore != now || store.query.At != now {
		t.Fatalf("query did not use pinned identity: %+v", store.query)
	}
	if len(items) != 2 || items[0].MemoryUUID != "MEM-A" || items[1].MemoryUUID != "MEM-B" {
		t.Fatalf("Memories are not deterministically sorted: %+v", items)
	}
}

func TestPersistentAgentMemoryResolverRejectsResourceOutsideInvocationScope(t *testing.T) {
	resolver, _ := NewPersistentAgentMemoryResolverV1(&agentMemoryStoreStub{}, agentMemoryInvocationResolverStub{invocation: application.AgentInvocationV1{
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions:    []string{application.AgentPermissionConversationRead},
		ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "group:G1", Actions: []string{"read"}}},
	}}, agentMemoryTaskReaderStub{task: &application.AgentTaskV1{TaskUUID: "TASK-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", CreatedAt: time.Now()}}, time.Now)
	_, err := resolver.ResolveContextMemories(context.Background(), "TASK-1", "RUN-1", "conversation", "group:G2", 20)
	if !errors.Is(err, application.ErrAgentMemoryDenied) {
		t.Fatalf("outside-scope error = %v", err)
	}
}
