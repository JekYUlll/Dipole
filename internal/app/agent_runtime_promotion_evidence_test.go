package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestRuntimePromotionEvidenceReviewUsesOperatorAuthorizationAndExactArtifact(t *testing.T) {
	proposal := &application.AgentRuntimePromotionProposalV1{
		ProposalUUID: strings.Repeat("a", 64), TenantID: "TENANT-A",
		EvidenceArtifactUUID: strings.Repeat("1", 64), EvidenceSHA256: strings.Repeat("2", 64),
	}
	controls := &promotionEvidenceControlStubV1{proposal: proposal}
	reader := &promotionEvidenceReaderStubV1{artifact: &application.AgentArtifactV1{
		ArtifactUUID: proposal.EvidenceArtifactUUID, ArtifactType: "promotion_evaluation", ContentSHA256: proposal.EvidenceSHA256,
	}, body: []byte(`{"schemaVersion":"dipole.agent.promotion-evaluation.v1"}`)}
	service, err := NewAgentRuntimePromotionEvidenceReviewServiceV1(controls, reader)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.Get(context.Background(), "REVIEWER", "TENANT-A", proposal.ProposalUUID)
	if err != nil || review.Proposal != proposal || review.Artifact != reader.artifact || string(review.Content) != string(reader.body) {
		t.Fatalf("review=%+v err=%v", review, err)
	}
	if controls.operator != "REVIEWER" || reader.artifactID != proposal.EvidenceArtifactUUID || reader.sha256 != proposal.EvidenceSHA256 {
		t.Fatalf("authorization/binding operator=%s artifact=%s hash=%s", controls.operator, reader.artifactID, reader.sha256)
	}

	reader.artifact.ContentSHA256 = strings.Repeat("3", 64)
	if _, err := service.Get(context.Background(), "REVIEWER", "TENANT-A", proposal.ProposalUUID); !errors.Is(err, application.ErrAgentRuntimePromotionControlConflict) {
		t.Fatalf("drifted review evidence: %v", err)
	}
}

func TestRuntimePromotionEvidenceReviewFailsBeforeArtifactReadWhenUnauthorized(t *testing.T) {
	controls := &promotionEvidenceControlStubV1{err: application.ErrAgentRuntimePromotionControlDenied}
	reader := &promotionEvidenceReaderStubV1{}
	service, _ := NewAgentRuntimePromotionEvidenceReviewServiceV1(controls, reader)
	if _, err := service.Get(context.Background(), "OTHER", "TENANT-A", strings.Repeat("a", 64)); !errors.Is(err, application.ErrAgentRuntimePromotionControlDenied) {
		t.Fatalf("unauthorized review: %v", err)
	}
	if reader.artifactID != "" {
		t.Fatalf("unauthorized review reached Artifact reader: %s", reader.artifactID)
	}
}

type promotionEvidenceControlStubV1 struct {
	proposal *application.AgentRuntimePromotionProposalV1
	err      error
	operator string
}

func (s *promotionEvidenceControlStubV1) Propose(context.Context, string, application.AgentRuntimePromotionProposalRequestV1) (*application.AgentRuntimePromotionProposalV1, error) {
	return nil, nil
}
func (s *promotionEvidenceControlStubV1) Review(context.Context, string, string, application.AgentRuntimePromotionReviewDecisionV1) (*application.AgentRuntimePromotionProposalV1, error) {
	return nil, nil
}
func (s *promotionEvidenceControlStubV1) Get(_ context.Context, operator, _, _ string) (*application.AgentRuntimePromotionProposalV1, error) {
	s.operator = operator
	return s.proposal, s.err
}
func (s *promotionEvidenceControlStubV1) Revoke(context.Context, string, string, string, string) (*application.AgentRuntimePromotionGrantV1, error) {
	return nil, nil
}

type promotionEvidenceReaderStubV1 struct {
	artifact           *application.AgentArtifactV1
	body               []byte
	err                error
	artifactID, sha256 string
}

func (s *promotionEvidenceReaderStubV1) ReadPromotionEvidence(_ context.Context, artifactID, sha256 string) (*application.AgentArtifactV1, []byte, error) {
	s.artifactID, s.sha256 = artifactID, sha256
	return s.artifact, s.body, s.err
}
