package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentRuntimePromotionControlServiceV1 struct {
	policies  application.AgentPolicyStoreV1
	artifacts application.AgentArtifactStoreV1
	control   application.AgentRuntimePromotionControlStoreV1
	now       func() time.Time
}

func NewPersistentAgentRuntimePromotionControlServiceV1(policies application.AgentPolicyStoreV1, artifacts application.AgentArtifactStoreV1, control application.AgentRuntimePromotionControlStoreV1) (*PersistentAgentRuntimePromotionControlServiceV1, error) {
	return newPersistentAgentRuntimePromotionControlServiceV1(policies, artifacts, control, time.Now)
}

func newPersistentAgentRuntimePromotionControlServiceV1(policies application.AgentPolicyStoreV1, artifacts application.AgentArtifactStoreV1, control application.AgentRuntimePromotionControlStoreV1, now func() time.Time) (*PersistentAgentRuntimePromotionControlServiceV1, error) {
	if policies == nil || artifacts == nil || control == nil || now == nil {
		return nil, errors.New("Agent policy, Artifact, promotion control Store, and clock are required")
	}
	return &PersistentAgentRuntimePromotionControlServiceV1{policies: policies, artifacts: artifacts, control: control, now: now}, nil
}

func (s *PersistentAgentRuntimePromotionControlServiceV1) Propose(ctx context.Context, operatorUUID string, request application.AgentRuntimePromotionProposalRequestV1) (*application.AgentRuntimePromotionProposalV1, error) {
	if err := s.authorize(ctx, request.TenantID, operatorUUID, "propose"); err != nil {
		return nil, err
	}
	proposal, err := application.NewAgentRuntimePromotionProposalV1(operatorUUID, request)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if proposal.ProposedAt.After(now.Add(time.Minute)) || !proposal.ExpiresAt.After(now) {
		return nil, fmt.Errorf("%w: proposal is outside its active window", application.ErrAgentRuntimePromotionControlConflict)
	}
	artifact, err := s.artifacts.GetAgentArtifact(ctx, proposal.EvidenceArtifactUUID)
	if err != nil {
		return nil, fmt.Errorf("get promotion evidence Artifact: %w", err)
	}
	if artifact == nil || artifact.ArtifactType != "promotion_evaluation" || artifact.ContentSHA256 != proposal.EvidenceSHA256 {
		return nil, fmt.Errorf("%w: immutable evidence Artifact does not match", application.ErrAgentRuntimePromotionControlConflict)
	}
	var provenance struct {
		RuntimeID         string `json:"runtimeId"`
		CandidateVersion  string `json:"candidateVersion"`
		DefinitionID      string `json:"definitionId"`
		DefinitionVersion uint64 `json:"definitionVersion"`
		EvalSuiteSHA256   string `json:"evalSuiteSHA256"`
	}
	if err := json.Unmarshal(artifact.Metadata, &provenance); err != nil || provenance.RuntimeID != proposal.RuntimeID ||
		provenance.CandidateVersion != proposal.CandidateVersion || provenance.DefinitionID != proposal.DefinitionUUID ||
		provenance.DefinitionVersion != proposal.DefinitionVersion || strings.ToLower(provenance.EvalSuiteSHA256) != proposal.EvalSuiteSHA256 {
		return nil, fmt.Errorf("%w: immutable Artifact provenance does not match the promotion binding", application.ErrAgentRuntimePromotionControlConflict)
	}
	task, err := s.policies.GetTask(ctx, artifact.TaskUUID)
	if err != nil {
		return nil, fmt.Errorf("get promotion evidence Task: %w", err)
	}
	run, err := s.policies.GetRun(ctx, artifact.RunUUID)
	if err != nil {
		return nil, fmt.Errorf("get promotion evidence Run: %w", err)
	}
	definition, err := s.policies.GetDefinitionVersion(ctx, proposal.DefinitionUUID, proposal.DefinitionVersion)
	if err != nil {
		return nil, fmt.Errorf("get promotion Definition: %w", err)
	}
	if task == nil || run == nil || definition == nil || task.TenantID != proposal.TenantID || definition.TenantID != proposal.TenantID ||
		task.DefinitionUUID != proposal.DefinitionUUID || task.DefinitionVersion != proposal.DefinitionVersion ||
		run.TaskUUID != task.TaskUUID || run.RuntimeID != proposal.RuntimeID || run.Mode != "shadow" || run.CandidateVersion != "" ||
		run.Status != application.AgentRunStatusCompleted || definition.Status != application.AgentDefinitionStatusActive || definition.RevokedAt != nil {
		return nil, fmt.Errorf("%w: evidence provenance does not match the promotion binding", application.ErrAgentRuntimePromotionControlConflict)
	}
	if _, err := s.control.CreateRuntimePromotionProposal(ctx, *proposal); err != nil {
		return nil, err
	}
	return s.control.GetRuntimePromotionProposal(ctx, proposal.ProposalUUID)
}

func (s *PersistentAgentRuntimePromotionControlServiceV1) Review(ctx context.Context, operatorUUID, proposalUUID string, decision application.AgentRuntimePromotionReviewDecisionV1) (*application.AgentRuntimePromotionProposalV1, error) {
	proposal, err := s.control.GetRuntimePromotionProposal(ctx, strings.TrimSpace(proposalUUID))
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, fmt.Errorf("%w: promotion proposal is unavailable", application.ErrAgentRuntimePromotionControlDenied)
	}
	if err := s.authorize(ctx, proposal.TenantID, operatorUUID, "review"); err != nil {
		return nil, err
	}
	if proposal.ProposerUUID == strings.TrimSpace(operatorUUID) {
		return nil, fmt.Errorf("%w: proposer cannot review their proposal", application.ErrAgentRuntimePromotionControlDenied)
	}
	if decision != application.AgentRuntimePromotionReviewApproved && decision != application.AgentRuntimePromotionReviewRejected {
		return nil, fmt.Errorf("%w: review decision is invalid", application.ErrAgentRuntimePromotionControlConflict)
	}
	return s.control.ReviewRuntimePromotionProposal(ctx, proposal.ProposalUUID, strings.TrimSpace(operatorUUID), decision, s.now().UTC())
}

func (s *PersistentAgentRuntimePromotionControlServiceV1) Get(ctx context.Context, operatorUUID, tenantID, proposalUUID string) (*application.AgentRuntimePromotionProposalV1, error) {
	if err := s.authorize(ctx, tenantID, operatorUUID, "read"); err != nil {
		return nil, err
	}
	proposal, err := s.control.GetRuntimePromotionProposal(ctx, strings.TrimSpace(proposalUUID))
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.TenantID != strings.TrimSpace(tenantID) {
		return nil, fmt.Errorf("%w: promotion proposal is unavailable", application.ErrAgentRuntimePromotionControlDenied)
	}
	return proposal, nil
}

func (s *PersistentAgentRuntimePromotionControlServiceV1) Revoke(ctx context.Context, operatorUUID, grantUUID, ticketRef, reason string) (*application.AgentRuntimePromotionGrantV1, error) {
	operatorUUID, grantUUID, ticketRef, reason = strings.TrimSpace(operatorUUID), strings.TrimSpace(grantUUID), strings.TrimSpace(ticketRef), strings.TrimSpace(reason)
	if operatorUUID == "" || grantUUID == "" || ticketRef == "" || reason == "" || len(reason) > 1000 {
		return nil, fmt.Errorf("%w: promotion revocation is invalid", application.ErrAgentRuntimePromotionControlConflict)
	}
	// Tenant ownership and the revoker role are checked atomically by the Store.
	grant, err := s.control.RevokeRuntimePromotionGrantAudited(ctx, application.AgentRuntimePromotionRevocationV1{GrantUUID: grantUUID, RevokedByUUID: operatorUUID, TicketRef: ticketRef, Reason: reason, RevokedAt: s.now().UTC()})
	if err != nil {
		return nil, err
	}
	if grant == nil {
		return nil, fmt.Errorf("%w: promotion grant is unavailable", application.ErrAgentRuntimePromotionControlDenied)
	}
	return grant, nil
}

func (s *PersistentAgentRuntimePromotionControlServiceV1) authorize(ctx context.Context, tenantID, operatorUUID, action string) error {
	tenantID, operatorUUID = strings.TrimSpace(tenantID), strings.TrimSpace(operatorUUID)
	grant, err := s.control.GetRuntimePromotionOperatorGrant(ctx, tenantID, operatorUUID)
	if err != nil {
		return fmt.Errorf("get Runtime promotion operator grant: %w", err)
	}
	allowed := grant != nil && grant.Active(tenantID, s.now().UTC())
	if allowed {
		switch action {
		case "propose":
			allowed = grant.CanPropose
		case "review":
			allowed = grant.CanReview
		case "read":
			allowed = grant.CanPropose || grant.CanReview || grant.CanRevoke
		default:
			allowed = false
		}
	}
	if !allowed {
		return fmt.Errorf("%w: inactive or insufficient Runtime promotion operator grant", application.ErrAgentRuntimePromotionControlDenied)
	}
	return nil
}
