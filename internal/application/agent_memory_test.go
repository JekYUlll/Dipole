package application

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentMemoryV1Validate(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	valid := AgentMemoryV1{
		MemoryUUID: "MEM-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		MemoryType: AgentMemoryTypeSemantic, Status: AgentMemoryStatusActive,
		ResourceType: AgentResourceTypeConversation, ResourceID: "group:G1",
		Content: "The database migration owner is Alice.", CompactContent: "DB migration owner: Alice.", Priority: 80,
		Provenance: AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M100", Sequence: "42"},
		ValidFrom:  now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid Memory rejected: %v", err)
	}

	invalid := valid
	invalid.Content = strings.Repeat("x", AgentMemoryContentMaxBytesV1+1)
	if !errors.Is(invalid.Validate(), ErrAgentMemoryInvalid) {
		t.Fatal("oversized Memory content should be rejected")
	}

	invalid = valid
	invalid.Status = AgentMemoryStatusRevoked
	if !errors.Is(invalid.Validate(), ErrAgentMemoryInvalid) {
		t.Fatal("revoked Memory without revoked_at should be rejected")
	}
}
