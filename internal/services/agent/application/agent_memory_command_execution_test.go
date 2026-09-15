package agentapplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type memoryCommandToolReaderStub struct {
	invocation *application.AgentToolInvocationV1
}

func (s memoryCommandToolReaderStub) GetToolInvocation(context.Context, string) (*application.AgentToolInvocationV1, error) {
	return s.invocation, nil
}

type memoryCommandApprovalReaderStub struct{ approval *application.AgentApprovalV1 }

func (s memoryCommandApprovalReaderStub) GetApproval(context.Context, string) (*application.AgentApprovalV1, error) {
	return s.approval, nil
}

func (memoryCommandApprovalReaderStub) GetRun(context.Context, string) (*application.AgentRunV1, error) {
	return nil, nil
}

type memoryCommandResolverStub struct{ invocation application.AgentInvocationV1 }

func (s memoryCommandResolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return s.invocation, nil
}

type memoryCommandStoreStub struct {
	items   map[string]application.AgentMemoryV1
	creates int
}

func (s *memoryCommandStoreStub) CreateMemory(_ context.Context, memory application.AgentMemoryV1) error {
	if _, exists := s.items[memory.MemoryUUID]; exists {
		return application.ErrAgentMemoryConflict
	}
	s.creates++
	s.items[memory.MemoryUUID] = memory
	return nil
}

func (s *memoryCommandStoreStub) ListContextMemories(context.Context, application.AgentMemoryQueryV1) ([]application.AgentMemoryV1, error) {
	return nil, nil
}

func (s *memoryCommandStoreStub) RevokeMemory(context.Context, string, time.Time) error { return nil }

func (s *memoryCommandStoreStub) ListOwnedMemories(context.Context, application.AgentMemoryOwnerListRequestV1) ([]application.AgentMemoryV1, error) {
	return nil, nil
}

func (s *memoryCommandStoreStub) GetOwnedMemory(_ context.Context, _, _ string, memoryUUID string) (*application.AgentMemoryV1, error) {
	memory, ok := s.items[memoryUUID]
	if !ok {
		return nil, nil
	}
	return &memory, nil
}

func (s *memoryCommandStoreStub) RevokeOwnedMemory(context.Context, string, string, string, string, string, time.Time) error {
	return nil
}

func (s *memoryCommandStoreStub) CorrectOwnedMemory(context.Context, application.AgentMemoryOwnerCorrectionWriteV1) (*application.AgentMemoryOwnerCorrectionResultV1, error) {
	return nil, nil
}

func (s *memoryCommandStoreStub) EraseOwnedMemoryRoot(context.Context, string, string, string, string, application.AgentMemoryErasureReasonV1, time.Time) (*application.AgentMemoryOwnerErasureReceiptV1, error) {
	return nil, nil
}

func TestAgentMemoryCommandExecutionBindsApprovedInvocationAndReplaysIdempotently(t *testing.T) {
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	request := application.AgentMemoryCommandExecutionRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-1", MemoryType: application.AgentMemoryTypeSemantic,
		Content: "Prefer concise Chinese replies", ConversationKey: "direct:U100:UAI",
	}
	argumentsSHA, err := memoryCommandArgumentsSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	tool := &application.AgentToolInvocationV1{
		InvocationUUID: request.InvocationUUID, TaskUUID: request.TaskUUID, RunUUID: request.RunUUID,
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Transport: application.AgentToolTransportMCP,
		CapabilityID: application.AgentCapabilityMemorySave, ArgumentsSHA256: argumentsSHA,
		Status: application.AgentToolInvocationStatusRunning, ApprovalUUID: "APR-1",
	}
	store := &memoryCommandStoreStub{items: map[string]application.AgentMemoryV1{}}
	service, err := NewAgentMemoryCommandExecutionV1(
		memoryCommandToolReaderStub{invocation: tool}, memoryCommandApprovalReaderStub{approval: &application.AgentApprovalV1{
			ApprovalUUID: "APR-1", TaskUUID: request.TaskUUID, CapabilityID: application.AgentCapabilityMemorySave,
			ResourceScope:   application.AgentResourceScopeV1{ResourceType: application.AgentResourceTypeConversation, ResourceID: request.ConversationKey, Actions: []string{application.AgentResourceActionWrite}},
			ArgumentsSHA256: argumentsSHA, Status: application.AgentApprovalStatusConsumed, ConsumedAt: &now,
		}}, memoryCommandResolverStub{invocation: application.AgentInvocationV1{
			TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Permissions: []string{application.AgentPermissionMemoryWrite},
			ResourceScopes:       []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: request.ConversationKey, Actions: []string{application.AgentResourceActionWrite}}},
			ApprovedCapabilities: []string{application.AgentCapabilityMemorySave},
		}}, store, store, func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.ExecuteMemory(context.Background(), request)
	if err != nil || created == nil || created.MemoryType != application.AgentMemoryTypeSemantic || store.creates != 1 {
		t.Fatalf("created=%+v creates=%d err=%v", created, store.creates, err)
	}
	tool.Status = application.AgentToolInvocationStatusCompleted
	replayed, err := service.ExecuteMemory(context.Background(), request)
	if err != nil || replayed == nil || replayed.MemoryUUID != created.MemoryUUID || store.creates != 1 {
		t.Fatalf("replayed=%+v creates=%d err=%v", replayed, store.creates, err)
	}
}

func TestAgentMemoryCommandExecutionRejectsApprovalScopeDrift(t *testing.T) {
	now := time.Now().UTC()
	request := application.AgentMemoryCommandExecutionRequestV1{
		TaskUUID: "TASK-1", RunUUID: "RUN-1", InvocationUUID: "INV-1", MemoryType: application.AgentMemoryTypeEpisodic,
		Content: "Cassandra discussion ended with MySQL staying primary", ConversationKey: "direct:U100:UAI",
	}
	argumentsSHA, _ := memoryCommandArgumentsSHA256(request)
	tool := &application.AgentToolInvocationV1{InvocationUUID: request.InvocationUUID, TaskUUID: request.TaskUUID, RunUUID: request.RunUUID,
		TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", Transport: application.AgentToolTransportMCP,
		CapabilityID: application.AgentCapabilityMemorySave, ArgumentsSHA256: argumentsSHA, Status: application.AgentToolInvocationStatusRunning, ApprovalUUID: "APR-1"}
	store := &memoryCommandStoreStub{items: map[string]application.AgentMemoryV1{}}
	service, _ := NewAgentMemoryCommandExecutionV1(memoryCommandToolReaderStub{invocation: tool}, memoryCommandApprovalReaderStub{approval: &application.AgentApprovalV1{
		ApprovalUUID: "APR-1", TaskUUID: request.TaskUUID, CapabilityID: application.AgentCapabilityMemorySave,
		ResourceScope:   application.AgentResourceScopeV1{ResourceType: application.AgentResourceTypeConversation, ResourceID: "group:G1", Actions: []string{application.AgentResourceActionWrite}},
		ArgumentsSHA256: argumentsSHA, Status: application.AgentApprovalStatusConsumed, ConsumedAt: &now,
	}}, memoryCommandResolverStub{invocation: application.AgentInvocationV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		Permissions: []string{application.AgentPermissionMemoryWrite}, ResourceScopes: []application.AgentResourceScopeV1{{ResourceType: application.AgentResourceTypeConversation, ResourceID: "*", Actions: []string{application.AgentResourceActionWrite}}},
		ApprovedCapabilities: []string{application.AgentCapabilityMemorySave},
	}}, store, store, func() time.Time { return now })
	if _, err := service.ExecuteMemory(context.Background(), request); !errors.Is(err, application.ErrAgentMemoryDenied) || store.creates != 0 {
		t.Fatalf("err=%v creates=%d", err, store.creates)
	}
}

func TestMemoryCommandArgumentsSHA256UsesCanonicalMcpObjectOrder(t *testing.T) {
	request := application.AgentMemoryCommandExecutionRequestV1{MemoryType: application.AgentMemoryTypeSemantic, Content: "fact", ConversationKey: "direct:U100:UAI"}
	got, err := memoryCommandArgumentsSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(`{"content":"fact","conversationId":"direct:U100:UAI","memoryType":"semantic"}`))
	if got != hex.EncodeToString(want[:]) {
		t.Fatalf("digest=%s want=%x", got, want)
	}
}
