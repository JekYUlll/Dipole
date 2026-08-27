package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentMemoryResolverV1 struct {
	store       application.AgentMemoryStoreV1
	invocations application.AgentInvocationResolverV1
	tasks       agentMemoryTaskReaderV1
	now         func() time.Time
}

type agentMemoryTaskReaderV1 interface {
	GetTask(ctx context.Context, taskUUID string) (*application.AgentTaskV1, error)
}

func NewPersistentAgentMemoryResolverV1(store application.AgentMemoryStoreV1, invocations application.AgentInvocationResolverV1, tasks agentMemoryTaskReaderV1, now func() time.Time) (*PersistentAgentMemoryResolverV1, error) {
	if store == nil || invocations == nil || tasks == nil {
		return nil, errors.New("Agent Memory store, Invocation resolver, and Task reader are required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentMemoryResolverV1{store: store, invocations: invocations, tasks: tasks, now: now}, nil
}

func (r *PersistentAgentMemoryResolverV1) ResolveContextMemories(ctx context.Context, taskUUID, runUUID, resourceType, resourceID string, limit int) ([]application.AgentMemoryV1, error) {
	taskUUID, runUUID = strings.TrimSpace(taskUUID), strings.TrimSpace(runUUID)
	resourceType, resourceID = strings.TrimSpace(resourceType), strings.TrimSpace(resourceID)
	if taskUUID == "" || runUUID == "" || resourceType == "" || resourceID == "" || limit < 1 || limit > 100 {
		return nil, application.ErrAgentMemoryInvalid
	}
	invocation, err := r.invocations.Resolve(ctx, taskUUID, runUUID)
	if err != nil {
		return nil, fmt.Errorf("resolve Agent Memory Invocation: %w", err)
	}
	task, err := r.tasks.GetTask(ctx, taskUUID)
	if err != nil || task == nil || task.TaskUUID != taskUUID || task.CreatedAt.IsZero() || task.TenantID != invocation.TenantID ||
		task.PrincipalUUID != invocation.PrincipalUUID || task.AgentUUID != invocation.AgentUUID {
		return nil, fmt.Errorf("%w: Agent Memory Task snapshot is unavailable", application.ErrAgentMemoryDenied)
	}
	descriptor := application.AgentCapabilityDescriptorV1{
		ID: application.AgentCapabilityConversationRead, Risk: application.AgentCapabilityRiskRead,
		RequiredPermission: application.AgentPermissionConversationRead,
	}
	if resourceType != application.AgentResourceTypeConversation ||
		application.AuthorizeAgentCapabilityForResourceV1(invocation, descriptor, resourceType, resourceID, application.AgentResourceActionRead) != nil {
		return nil, application.ErrAgentMemoryDenied
	}
	query := application.AgentMemoryQueryV1{
		TenantID: invocation.TenantID, PrincipalUUID: invocation.PrincipalUUID, AgentUUID: invocation.AgentUUID,
		ResourceType: resourceType, ResourceID: resourceID, CreatedBefore: task.CreatedAt.UTC(), At: r.now().UTC(), Limit: limit,
	}
	items, err := r.store.ListContextMemories(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list Agent Context Memories: %w", err)
	}
	for _, item := range items {
		if item.Validate() != nil || item.Status != application.AgentMemoryStatusActive || item.RevokedAt != nil ||
			item.TenantID != query.TenantID || item.PrincipalUUID != query.PrincipalUUID || item.AgentUUID != query.AgentUUID ||
			item.ResourceType != query.ResourceType || item.ResourceID != query.ResourceID || item.CreatedAt.After(query.CreatedBefore) || query.At.Before(item.ValidFrom) ||
			(item.ExpiresAt != nil && !query.At.Before(*item.ExpiresAt)) {
			return nil, application.ErrAgentMemoryInvalid
		}
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Priority != items[right].Priority {
			return items[left].Priority > items[right].Priority
		}
		return items[left].MemoryUUID < items[right].MemoryUUID
	})
	return items, nil
}
