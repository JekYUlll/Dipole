package application

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewAgentMCPReadinessEvidenceRecordV1CanonicalReplayAndDrift(t *testing.T) {
	request := validAgentMCPReadinessEvidenceRequestV1()
	first, err := NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.EvidenceUUID != second.EvidenceUUID || first.ContentSHA256 != second.ContentSHA256 || string(first.ContentJSON) != string(second.ContentJSON) {
		t.Fatalf("exact replay changed identity: first=%+v second=%+v", first, second)
	}
	if first.TenantID != "dipole" || first.ProfileBindingSHA256 != strings.Repeat("b", 64) || first.RuntimeBindingSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("unexpected binding: %+v", first)
	}
	if strings.Contains(string(first.ContentJSON), "OPERATOR") || strings.Contains(string(first.ContentJSON), "REQ-1") {
		t.Fatalf("operator provenance leaked into evidence content: %s", first.ContentJSON)
	}

	drift := request
	drift.Evidence.ToolCount++
	changed, err := NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", drift)
	if err != nil {
		t.Fatal(err)
	}
	if changed.EvidenceUUID == first.EvidenceUUID || changed.ContentSHA256 == first.ContentSHA256 {
		t.Fatal("receipt drift must create distinct immutable evidence")
	}
}

func TestNewAgentMCPReadinessEvidenceRecordV1RejectsInvalidOrStaleInput(t *testing.T) {
	base := validAgentMCPReadinessEvidenceRequestV1()
	cases := []AgentMCPReadinessEvidenceRequestV1{
		func() AgentMCPReadinessEvidenceRequestV1 { value := base; value.TenantID = " other "; return value }(),
		func() AgentMCPReadinessEvidenceRequestV1 {
			value := base
			value.ProfileBindingSHA256 = strings.Repeat("A", 64)
			return value
		}(),
		func() AgentMCPReadinessEvidenceRequestV1 {
			value := base
			value.ExpiresAt = value.Evidence.CompletedAt
			return value
		}(),
		func() AgentMCPReadinessEvidenceRequestV1 {
			value := base
			value.ExpiresAt = value.Evidence.CompletedAt.Add(time.Hour + time.Millisecond)
			return value
		}(),
		func() AgentMCPReadinessEvidenceRequestV1 { value := base; value.Evidence.ToolCount = 0; return value }(),
		func() AgentMCPReadinessEvidenceRequestV1 {
			value := base
			value.Evidence.ProfileBindingSHA256 = strings.Repeat("c", 64)
			return value
		}(),
		func() AgentMCPReadinessEvidenceRequestV1 {
			value := base
			value.Evidence.StartedAt = value.Evidence.CompletedAt.Add(time.Millisecond)
			return value
		}(),
		func() AgentMCPReadinessEvidenceRequestV1 {
			value := base
			value.Evidence.CompletedAt = value.Evidence.StartedAt.Add(10*time.Minute + time.Millisecond)
			value.Evidence.ConnectivityCheckedAt = value.Evidence.CompletedAt
			value.ExpiresAt = value.Evidence.CompletedAt.Add(time.Minute)
			return value
		}(),
	}
	for index, candidate := range cases {
		if _, err := NewAgentMCPReadinessEvidenceRecordV1("OPERATOR", candidate); !errors.Is(err, ErrAgentMCPReadinessEvidenceInvalid) {
			t.Fatalf("case %d: err=%v", index, err)
		}
	}
}

func TestParseAgentMCPReadinessEvidenceV1RejectsAdditionalOrSensitiveFields(t *testing.T) {
	valid := validAgentMCPReadinessEvidenceRequestV1().Evidence
	payload, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAgentMCPReadinessEvidenceV1(payload); err != nil {
		t.Fatalf("parse canonical evidence: %v", err)
	}
	for _, candidate := range [][]byte{
		append(payload[:len(payload)-1], []byte(`,"token":"secret"}`)...),
		[]byte(`{"schemaVersion":"dipole.agent.external-mcp-readiness-evidence.v1"}`),
		append(payload, []byte(` trailing`)...),
	} {
		if _, err := ParseAgentMCPReadinessEvidenceV1(candidate); !errors.Is(err, ErrAgentMCPReadinessEvidenceInvalid) {
			t.Fatalf("unexpected parse error: %v", err)
		}
	}
}

func validAgentMCPReadinessEvidenceRequestV1() AgentMCPReadinessEvidenceRequestV1 {
	started := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	return AgentMCPReadinessEvidenceRequestV1{
		TenantID:             "dipole",
		ProfileBindingSHA256: strings.Repeat("b", 64),
		RequestID:            "REQ-1",
		TraceID:              "TRACE-1",
		ExpiresAt:            started.Add(30 * time.Minute),
		Evidence: AgentMCPReadinessEvidenceV1{
			SchemaVersion: AgentMCPReadinessEvidenceSchemaVersionV2,
			BindingSHA256: strings.Repeat("a", 64), ProfileBindingSHA256: strings.Repeat("b", 64),
			StartedAt: started, CompletedAt: started.Add(3 * time.Second),
			PreflightCheckedAt: started.Add(time.Second), ConnectivityCheckedAt: started.Add(2 * time.Second),
			ProfileCount: 2, CredentialCount: 2, CABundleCount: 2, ToolCount: 2,
		},
	}
}
