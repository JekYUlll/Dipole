package application

import (
	"testing"
	"time"
)

func TestAgentWorkflowRepairProposalMatchesTypeScriptCanonicalEvidence(t *testing.T) {
	proposal, err := NewAgentWorkflowRepairProposalV1(" U-OPS-1 ", AgentWorkflowRepairProposalRequestV1{
		TaskUUID: "TASK-1", Outcome: AgentWorkflowRepairOutcomeStale, TicketRef: "INC-2048",
		Reason:     "verified Temporal state after Worker recovery",
		Projected:  &AgentWorkflowEvidenceV1{WorkflowID: "dipole-agent-task/TASK-1", WorkflowRunID: "WR-1", Status: "running", Revision: 2},
		Temporal:   AgentWorkflowEvidenceV1{WorkflowID: "dipole-agent-task/TASK-1", WorkflowRunID: "WR-1", Status: "completed", Revision: 3},
		ProposedAt: mustRepairTimeV1(t, "2026-08-28T00:00:00Z"), ExpiresAt: mustRepairTimeV1(t, "2026-08-28T01:00:00Z"),
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	const expected = "bf5b8b49aa69444761642c4deb918c23360f3689e33a368f8ff0d1626ad4e700"
	if proposal.EvidenceSHA256 != expected || proposal.ProposalUUID != "repair:"+expected || proposal.RequiredApprovals != 2 {
		t.Fatalf("canonical proposal = %+v", proposal)
	}
}

func TestAgentWorkflowRepairProposalRejectsUnavailableEvidenceWindow(t *testing.T) {
	_, err := NewAgentWorkflowRepairProposalV1("U1", AgentWorkflowRepairProposalRequestV1{
		TaskUUID: "TASK-1", Outcome: AgentWorkflowRepairOutcomeStale, TicketRef: "INC-1", Reason: "repair",
		Temporal:   AgentWorkflowEvidenceV1{WorkflowID: "dipole-agent-task/TASK-1", WorkflowRunID: "WR-1", Status: "running", Revision: 1},
		ProposedAt: mustRepairTimeV1(t, "2026-08-28T00:00:00Z"), ExpiresAt: mustRepairTimeV1(t, "2026-08-28T02:00:00Z"),
	})
	if err == nil {
		t.Fatal("expected long-lived proposal rejection")
	}
}

func mustRepairTimeV1(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
