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

func TestAgentMemoryV1ValidateCorrectionLineage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	valid := AgentMemoryV1{
		MemoryUUID: "MEM-CORR-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI",
		MemoryType: AgentMemoryTypeSemantic, Status: AgentMemoryStatusActive,
		ResourceType: "conversation", ResourceID: "group:G1", Content: "Owner is Bob", Priority: 80,
		Provenance: AgentMemoryProvenanceV1{SourceType: AgentMemorySourceOwnerCorrectionV1, SourceID: "MEM-1", Sequence: "2"},
		ValidFrom:  now, MemoryRootUUID: "MEM-1", MemoryVersion: 2, SupersedesMemoryUUID: "MEM-1",
		CorrectedByUUID: "U100", CorrectionReason: "owner changed",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid correction lineage: %v", err)
	}
	for name, mutate := range map[string]func(*AgentMemoryV1){
		"missing root":        func(item *AgentMemoryV1) { item.MemoryRootUUID = "" },
		"missing predecessor": func(item *AgentMemoryV1) { item.SupersedesMemoryUUID = "" },
		"missing corrector":   func(item *AgentMemoryV1) { item.CorrectedByUUID = "" },
		"missing reason":      func(item *AgentMemoryV1) { item.CorrectionReason = "" },
		"version one":         func(item *AgentMemoryV1) { item.MemoryVersion = 1 },
		"sequence drift":      func(item *AgentMemoryV1) { item.Provenance.Sequence = "3" },
		"reason control rune": func(item *AgentMemoryV1) { item.CorrectionReason = "owner\nchanged" },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			if !errors.Is(item.Validate(), ErrAgentMemoryInvalid) {
				t.Fatalf("expected invalid lineage: %+v", item)
			}
		})
	}
}
