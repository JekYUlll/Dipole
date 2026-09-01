package agentapplication_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
)

func TestRuntimePromotionControlRequiresEvidenceAndDistinctReviewer(t *testing.T) {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	fixture := newPromotionControlFixtureV1(now)
	service, err := agentapplication.NewPersistentAgentRuntimePromotionControlServiceV1WithClock(fixture.policies, fixture.artifacts, fixture.control, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	request := promotionProposalRequestV1(now)
	if _, err := service.Propose(context.Background(), "UNGRANTED", request); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("ungranted propose: %v", err)
	}
	bad := request
	bad.EvidenceSHA256 = strings.Repeat("b", 64)
	if _, err := service.Propose(context.Background(), "PROPOSER", bad); !errors.Is(err, application.ErrAgentRuntimePromotionControlConflict) {
		t.Fatalf("mismatched evidence: %v", err)
	}
	proposal, err := service.Propose(context.Background(), "PROPOSER", request)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, err := service.Review(context.Background(), "PROPOSER", proposal.ProposalUUID, application.AgentRuntimePromotionReviewApproved); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("self review: %v", err)
	}
	approved, err := service.Review(context.Background(), "REVIEWER", proposal.ProposalUUID, application.AgentRuntimePromotionReviewApproved)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if approved.Status != application.AgentRuntimePromotionProposalApproved || approved.GrantUUID == "" {
		t.Fatalf("approved proposal = %+v", approved)
	}
	grant := fixture.control.grants[approved.GrantUUID]
	if grant == nil || grant.GrantedByUUID != "PROPOSER" || grant.ReviewedByUUID != "REVIEWER" || grant.EvidenceSHA256 != fixture.artifact.ContentSHA256 {
		t.Fatalf("grant = %+v", grant)
	}
}

func TestRuntimePromotionControlRejectsCrossTenantEvidenceAndAuditsRevocation(t *testing.T) {
	now := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	fixture := newPromotionControlFixtureV1(now)
	service, _ := agentapplication.NewPersistentAgentRuntimePromotionControlServiceV1WithClock(fixture.policies, fixture.artifacts, fixture.control, func() time.Time { return now })
	crossTenant := promotionProposalRequestV1(now)
	crossTenant.TenantID = "TENANT-B"
	if _, err := service.Propose(context.Background(), "PROPOSER", crossTenant); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("cross-tenant proposal: %v", err)
	}
	proposal, _ := service.Propose(context.Background(), "PROPOSER", promotionProposalRequestV1(now))
	approved, _ := service.Review(context.Background(), "REVIEWER", proposal.ProposalUUID, application.AgentRuntimePromotionReviewApproved)
	if _, err := service.Revoke(context.Background(), "REVIEWER", approved.GrantUUID, "INC-42", "candidate rollback"); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("reviewer revoke: %v", err)
	}
	revoked, err := service.Revoke(context.Background(), "REVOKER", approved.GrantUUID, "INC-42", "candidate rollback")
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked.RevokedAt == nil || fixture.control.revocations[approved.GrantUUID].RevokedByUUID != "REVOKER" {
		t.Fatalf("revocation = %+v audit=%+v", revoked, fixture.control.revocations[approved.GrantUUID])
	}
}

type promotionControlFixtureV1 struct {
	policies  *agentPolicyStoreStub
	artifacts *promotionArtifactStoreStubV1
	control   *promotionControlStoreStubV1
	artifact  application.AgentArtifactV1
}

func newPromotionControlFixtureV1(now time.Time) promotionControlFixtureV1 {
	task := &application.AgentTaskV1{TaskUUID: "TASK-1", DefinitionUUID: "DEF-1", DefinitionVersion: 1, TenantID: "TENANT-A", PrincipalUUID: "U-1", AgentUUID: "AGENT-1", Status: application.AgentTaskStatusCompleted, TriggerType: "interactive", TriggerRef: "MSG-1", Goal: "evaluate candidate"}
	definition := &application.AgentDefinitionVersionV1{DefinitionUUID: "DEF-1", Version: 1, TenantID: "TENANT-A", OwnerUUID: "U-1", AgentUUID: "AGENT-1", Status: application.AgentDefinitionStatusActive, Permissions: []string{"conversation.read"}, Scopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "C-1", Actions: []string{"read"}}}, ValidFrom: now.Add(-time.Hour)}
	artifact := application.AgentArtifactV1{ArtifactUUID: strings.Repeat("1", 64), SchemaVersion: application.AgentArtifactSchemaVersionV1, TaskUUID: "TASK-1", RunUUID: "RUN-1", ArtifactType: "promotion_evaluation", Version: 1, Title: "Candidate evaluation", MediaType: "application/json", ObjectBucket: "agent", ObjectKey: "evidence", ContentSHA256: strings.Repeat("a", 64), SizeBytes: 100, Metadata: []byte(`{"runtimeId":"dipole-agent","candidateVersion":"sha256:candidate","definitionId":"DEF-1","definitionVersion":1,"evalSuiteSHA256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}`), CreatedAt: now.Add(-time.Minute)}
	policies := &agentPolicyStoreStub{tasks: map[string]*application.AgentTaskV1{"TASK-1": task}, definitions: map[string]*application.AgentDefinitionVersionV1{"DEF-1:1": definition}, runs: map[string]*application.AgentRunV1{"RUN-1": {RunUUID: "RUN-1", TaskUUID: "TASK-1", RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusCompleted}}}
	control := newPromotionControlStoreStubV1(now)
	return promotionControlFixtureV1{policies: policies, artifacts: &promotionArtifactStoreStubV1{artifact: artifact}, control: control, artifact: artifact}
}

func promotionProposalRequestV1(now time.Time) application.AgentRuntimePromotionProposalRequestV1 {
	return application.AgentRuntimePromotionProposalRequestV1{TenantID: "TENANT-A", RuntimeID: "dipole-agent", CandidateVersion: "sha256:candidate", DefinitionUUID: "DEF-1", DefinitionVersion: 1, EvidenceArtifactUUID: strings.Repeat("1", 64), EvidenceSHA256: strings.Repeat("a", 64), EvalSuiteSHA256: strings.Repeat("e", 64), TicketRef: "REL-42", Reason: "offline and shadow gates passed", ProposedAt: now, ExpiresAt: now.Add(time.Hour), GrantValidFrom: now, GrantExpiresAt: now.Add(24 * time.Hour)}
}

type promotionArtifactStoreStubV1 struct{ artifact application.AgentArtifactV1 }

func (s *promotionArtifactStoreStubV1) CreateAgentArtifact(context.Context, application.AgentArtifactV1) (bool, error) {
	return false, nil
}
func (s *promotionArtifactStoreStubV1) GetAgentArtifact(_ context.Context, id string) (*application.AgentArtifactV1, error) {
	if id != s.artifact.ArtifactUUID {
		return nil, nil
	}
	value := s.artifact
	return &value, nil
}
func (s *promotionArtifactStoreStubV1) GetAgentArtifactByTaskTypeVersion(context.Context, string, string, uint32) (*application.AgentArtifactV1, error) {
	return nil, nil
}

type promotionControlStoreStubV1 struct {
	grants         map[string]*application.AgentRuntimePromotionGrantV1
	operatorGrants map[string]*application.AgentRuntimePromotionOperatorGrantV1
	proposals      map[string]*application.AgentRuntimePromotionProposalV1
	revocations    map[string]application.AgentRuntimePromotionRevocationV1
}

func newPromotionControlStoreStubV1(now time.Time) *promotionControlStoreStubV1 {
	return &promotionControlStoreStubV1{grants: map[string]*application.AgentRuntimePromotionGrantV1{}, proposals: map[string]*application.AgentRuntimePromotionProposalV1{}, revocations: map[string]application.AgentRuntimePromotionRevocationV1{}, operatorGrants: map[string]*application.AgentRuntimePromotionOperatorGrantV1{
		"PROPOSER": {TenantID: "TENANT-A", UserUUID: "PROPOSER", CanPropose: true, ValidFrom: now.Add(-time.Hour)},
		"REVIEWER": {TenantID: "TENANT-A", UserUUID: "REVIEWER", CanReview: true, ValidFrom: now.Add(-time.Hour)},
		"REVOKER":  {TenantID: "TENANT-A", UserUUID: "REVOKER", CanRevoke: true, ValidFrom: now.Add(-time.Hour)},
	}}
}

func (s *promotionControlStoreStubV1) GetRuntimePromotionOperatorGrant(_ context.Context, tenant, user string) (*application.AgentRuntimePromotionOperatorGrantV1, error) {
	return s.operatorGrants[user], nil
}
func (s *promotionControlStoreStubV1) CreateRuntimePromotionProposal(_ context.Context, p application.AgentRuntimePromotionProposalV1) (bool, error) {
	copy := p
	s.proposals[p.ProposalUUID] = &copy
	return true, nil
}
func (s *promotionControlStoreStubV1) GetRuntimePromotionProposal(_ context.Context, id string) (*application.AgentRuntimePromotionProposalV1, error) {
	if p := s.proposals[id]; p != nil {
		copy := *p
		return &copy, nil
	}
	return nil, nil
}
func (s *promotionControlStoreStubV1) ReviewRuntimePromotionProposal(_ context.Context, id, reviewer string, decision application.AgentRuntimePromotionReviewDecisionV1, at time.Time) (*application.AgentRuntimePromotionProposalV1, error) {
	p := s.proposals[id]
	if p == nil || p.ProposerUUID == reviewer {
		return nil, application.ErrAgentRuntimePromotionControlDenied
	}
	if decision == application.AgentRuntimePromotionReviewRejected {
		p.Status = application.AgentRuntimePromotionProposalRejected
		decided := at
		p.DecidedAt = &decided
		return p, nil
	}
	grant := p.Grant(reviewer)
	s.grants[grant.GrantUUID] = &grant
	p.Status, p.GrantUUID = application.AgentRuntimePromotionProposalApproved, grant.GrantUUID
	decided := at
	p.DecidedAt = &decided
	return p, nil
}
func (s *promotionControlStoreStubV1) RevokeRuntimePromotionGrantAudited(_ context.Context, r application.AgentRuntimePromotionRevocationV1) (*application.AgentRuntimePromotionGrantV1, error) {
	grant := s.grants[r.GrantUUID]
	operator := s.operatorGrants[r.RevokedByUUID]
	if grant == nil || operator == nil || !operator.Active(grant.TenantID, r.RevokedAt) || !operator.CanRevoke {
		return nil, application.ErrAgentRuntimePromotionControlDenied
	}
	at := r.RevokedAt
	grant.RevokedAt = &at
	r.TenantID = grant.TenantID
	s.revocations[r.GrantUUID] = r
	return grant, nil
}
