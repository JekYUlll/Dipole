package agentapplication

import (
	"context"
	"errors"
	"fmt"

	"github.com/JekYUlll/Dipole/internal/application"
)

type AgentRuntimePromotionEvidenceReviewServiceV1 struct {
	controls application.AgentRuntimePromotionControlServiceV1
	reader   application.AgentRuntimePromotionEvidenceReaderV1
}

func NewAgentRuntimePromotionEvidenceReviewServiceV1(controls application.AgentRuntimePromotionControlServiceV1, reader application.AgentRuntimePromotionEvidenceReaderV1) (*AgentRuntimePromotionEvidenceReviewServiceV1, error) {
	if controls == nil || reader == nil {
		return nil, errors.New("Agent Runtime promotion controls and evidence reader are required")
	}
	return &AgentRuntimePromotionEvidenceReviewServiceV1{controls: controls, reader: reader}, nil
}

func (s *AgentRuntimePromotionEvidenceReviewServiceV1) Get(ctx context.Context, operatorUUID, tenantID, proposalUUID string) (*application.AgentRuntimePromotionEvidenceReviewV1, error) {
	proposal, err := s.controls.Get(ctx, operatorUUID, tenantID, proposalUUID)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, fmt.Errorf("%w: promotion proposal is unavailable", application.ErrAgentRuntimePromotionControlDenied)
	}
	artifact, body, err := s.reader.ReadPromotionEvidence(ctx, proposal.EvidenceArtifactUUID, proposal.EvidenceSHA256)
	if err != nil {
		return nil, err
	}
	if artifact == nil || artifact.ArtifactUUID != proposal.EvidenceArtifactUUID || artifact.ContentSHA256 != proposal.EvidenceSHA256 {
		return nil, fmt.Errorf("%w: promotion evidence differs from the Proposal", application.ErrAgentRuntimePromotionControlConflict)
	}
	return &application.AgentRuntimePromotionEvidenceReviewV1{Proposal: proposal, Artifact: artifact, Content: body}, nil
}
