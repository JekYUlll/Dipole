package agentapplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

// AgentMemoryCommandExecutionServiceV1 is the Core-side authority for Memory
// writes. The Runtime only supplies a Tool Invocation ID and untrusted input.
type AgentMemoryCommandExecutionServiceV1 struct {
	tools     application.AgentToolInvocationReaderV1
	approvals application.AgentToolApprovalReaderV1
	resolver  application.AgentInvocationResolverV1
	memories  application.AgentMemoryStoreV1
	owners    application.AgentMemoryOwnerStoreV1
	now       func() time.Time
}

var _ application.AgentMemoryCommandExecutionV1 = (*AgentMemoryCommandExecutionServiceV1)(nil)

func NewAgentMemoryCommandExecutionV1(tools application.AgentToolInvocationReaderV1, approvals application.AgentToolApprovalReaderV1, resolver application.AgentInvocationResolverV1, memories application.AgentMemoryStoreV1, owners application.AgentMemoryOwnerStoreV1, now func() time.Time) (*AgentMemoryCommandExecutionServiceV1, error) {
	if tools == nil || approvals == nil || resolver == nil || memories == nil || owners == nil {
		return nil, errors.New("Agent Memory Command execution dependencies are required")
	}
	if now == nil {
		now = time.Now
	}
	return &AgentMemoryCommandExecutionServiceV1{tools: tools, approvals: approvals, resolver: resolver, memories: memories, owners: owners, now: now}, nil
}

func (s *AgentMemoryCommandExecutionServiceV1) ExecuteMemory(ctx context.Context, request application.AgentMemoryCommandExecutionRequestV1) (*application.AgentMemoryV1, error) {
	request.TaskUUID, request.RunUUID, request.InvocationUUID = strings.TrimSpace(request.TaskUUID), strings.TrimSpace(request.RunUUID), strings.TrimSpace(request.InvocationUUID)
	request.Content, request.CompactContent, request.ConversationKey = strings.TrimSpace(request.Content), strings.TrimSpace(request.CompactContent), strings.TrimSpace(request.ConversationKey)
	if request.TaskUUID == "" || request.RunUUID == "" || request.InvocationUUID == "" || request.Content == "" || request.ConversationKey == "" ||
		(request.MemoryType != application.AgentMemoryTypeSemantic && request.MemoryType != application.AgentMemoryTypeEpisodic) ||
		len(request.Content) > 1000 || len(request.CompactContent) > application.AgentMemoryCompactContentMaxBytesV1 {
		return nil, application.ErrAgentMemoryInvalid
	}
	tool, err := s.tools.GetToolInvocation(ctx, request.InvocationUUID)
	if err != nil {
		return nil, fmt.Errorf("load Agent Tool invocation for Memory Command: %w", err)
	}
	if tool == nil || tool.InvocationUUID != request.InvocationUUID || tool.TaskUUID != request.TaskUUID || tool.RunUUID != request.RunUUID ||
		tool.Transport != application.AgentToolTransportMCP || (tool.Status != application.AgentToolInvocationStatusRunning && tool.Status != application.AgentToolInvocationStatusCompleted) ||
		tool.CapabilityID != application.AgentCapabilityMemorySave || strings.TrimSpace(tool.ApprovalUUID) == "" {
		return nil, application.ErrAgentMemoryDenied
	}
	invocation, err := s.resolver.Resolve(ctx, request.TaskUUID, request.RunUUID)
	if err != nil || invocation.TenantID != tool.TenantID || invocation.PrincipalUUID != tool.PrincipalUUID || invocation.AgentUUID != tool.AgentUUID {
		return nil, application.ErrAgentMemoryDenied
	}
	approval, err := s.approvals.GetApproval(ctx, tool.ApprovalUUID)
	if err != nil || approval == nil || approval.TaskUUID != request.TaskUUID || approval.CapabilityID != application.AgentCapabilityMemorySave ||
		approval.Status != application.AgentApprovalStatusConsumed || approval.ConsumedAt == nil || approval.RevokedAt != nil ||
		approval.ResourceScope.ResourceType != application.AgentResourceTypeConversation || approval.ResourceScope.ResourceID != request.ConversationKey ||
		len(approval.ResourceScope.Actions) != 1 || approval.ResourceScope.Actions[0] != application.AgentResourceActionWrite {
		return nil, application.ErrAgentMemoryDenied
	}
	if err := application.AuthorizeAgentCapabilityForResourceV1(invocation, application.AgentCapabilityDescriptorV1{
		ID: application.AgentCapabilityMemorySave, Risk: application.AgentCapabilityRiskWrite, RequiredPermission: application.AgentPermissionMemoryWrite, ApprovalRequired: true,
	}, application.AgentResourceTypeConversation, request.ConversationKey, application.AgentResourceActionWrite); err != nil {
		return nil, application.ErrAgentMemoryDenied
	}
	argumentsSHA, err := memoryCommandArgumentsSHA256(request)
	if err != nil || tool.ArgumentsSHA256 != argumentsSHA || approval.ArgumentsSHA256 != argumentsSHA {
		return nil, application.ErrAgentMemoryDenied
	}
	memory := application.AgentMemoryV1{
		MemoryUUID: stableAgentMemoryCommandUUID(request), TenantID: invocation.TenantID, PrincipalUUID: invocation.PrincipalUUID, AgentUUID: invocation.AgentUUID,
		MemoryType: request.MemoryType, Status: application.AgentMemoryStatusActive, ResourceType: application.AgentResourceTypeConversation, ResourceID: request.ConversationKey,
		Content: request.Content, CompactContent: request.CompactContent, Priority: 500,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "agent_task", SourceID: request.TaskUUID, Sequence: request.RunUUID}, ValidFrom: s.now().UTC(),
	}
	memory = application.CanonicalAgentMemoryLineageV1(memory)
	if memory.Validate() != nil {
		return nil, application.ErrAgentMemoryInvalid
	}
	if tool.Status == application.AgentToolInvocationStatusCompleted {
		return s.existing(ctx, memory)
	}
	if err := s.memories.CreateMemory(ctx, memory); err != nil {
		return s.existing(ctx, memory)
	}
	return s.existing(ctx, memory)
}

func (s *AgentMemoryCommandExecutionServiceV1) existing(ctx context.Context, expected application.AgentMemoryV1) (*application.AgentMemoryV1, error) {
	stored, err := s.owners.GetOwnedMemory(ctx, expected.TenantID, expected.PrincipalUUID, expected.MemoryUUID)
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.Validate() != nil || stored.MemoryUUID != expected.MemoryUUID || stored.TenantID != expected.TenantID ||
		stored.PrincipalUUID != expected.PrincipalUUID || stored.AgentUUID != expected.AgentUUID || stored.MemoryType != expected.MemoryType ||
		stored.Status != application.AgentMemoryStatusActive || stored.ResourceType != expected.ResourceType || stored.ResourceID != expected.ResourceID ||
		stored.Content != expected.Content || stored.CompactContent != expected.CompactContent || stored.Provenance != expected.Provenance {
		return nil, application.ErrAgentMemoryConflict
	}
	return stored, nil
}

func stableAgentMemoryCommandUUID(request application.AgentMemoryCommandExecutionRequestV1) string {
	value := strings.Join([]string{"dipole.agent.memory-command.v1", request.TaskUUID, request.RunUUID, request.InvocationUUID, string(request.MemoryType), request.ConversationKey, request.Content, request.CompactContent}, "\n")
	sum := sha256.Sum256([]byte(value))
	return "MEM-" + hex.EncodeToString(sum[:16])
}

func memoryCommandArgumentsSHA256(request application.AgentMemoryCommandExecutionRequestV1) (string, error) {
	payload, err := json.Marshal(struct {
		Content        string `json:"content"`
		ConversationID string `json:"conversationId"`
		MemoryType     string `json:"memoryType"`
	}{Content: request.Content, ConversationID: request.ConversationKey, MemoryType: string(request.MemoryType)})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
