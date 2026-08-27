package application

import (
	"context"
	"strings"
	"time"
)

const AgentRuntimePromotionPolicyVersionV2 = "dipole.agent.shadow-promotion-policy.v2"

type AgentRuntimePromotionGrantV1 struct {
	GrantUUID         string     `json:"grant_uuid"`
	TenantID          string     `json:"tenant_id"`
	RuntimeID         string     `json:"runtime_id"`
	CandidateVersion  string     `json:"candidate_version"`
	DefinitionUUID    string     `json:"definition_uuid"`
	DefinitionVersion uint64     `json:"definition_version"`
	PolicyVersion     string     `json:"policy_version"`
	EvidenceSHA256    string     `json:"evidence_sha256"`
	EvalSuiteSHA256   string     `json:"eval_suite_sha256"`
	GrantedByUUID     string     `json:"granted_by_uuid"`
	ReviewedByUUID    string     `json:"reviewed_by_uuid"`
	ValidFrom         time.Time  `json:"valid_from"`
	ExpiresAt         time.Time  `json:"expires_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at,omitempty"`
}

func (g AgentRuntimePromotionGrantV1) Validate() error {
	if !validPromotionIdentifierV1(g.GrantUUID, 64) || !validPromotionIdentifierV1(g.TenantID, 64) ||
		!validPromotionIdentifierV1(g.RuntimeID, 64) || !validPromotionIdentifierV1(g.CandidateVersion, 128) ||
		!validPromotionIdentifierV1(g.DefinitionUUID, 64) || g.DefinitionVersion == 0 ||
		g.PolicyVersion != AgentRuntimePromotionPolicyVersionV2 || !validSHA256V1(g.EvidenceSHA256) ||
		!validSHA256V1(g.EvalSuiteSHA256) || !validPromotionIdentifierV1(g.GrantedByUUID, 24) ||
		!validPromotionIdentifierV1(g.ReviewedByUUID, 24) || g.GrantedByUUID == g.ReviewedByUUID ||
		g.ValidFrom.IsZero() || g.ExpiresAt.IsZero() || !g.ValidFrom.Before(g.ExpiresAt) {
		return ErrAgentPolicyInvalid
	}
	return nil
}

func (g AgentRuntimePromotionGrantV1) Active(at time.Time) bool {
	return g.Validate() == nil && !at.Before(g.ValidFrom) && at.Before(g.ExpiresAt) && g.RevokedAt == nil
}

type AgentRuntimePromotionGrantLookupV1 struct {
	TenantID          string
	RuntimeID         string
	CandidateVersion  string
	DefinitionUUID    string
	DefinitionVersion uint64
	At                time.Time
}

type AgentRuntimePromotionGrantStoreV1 interface {
	CreateRuntimePromotionGrant(ctx context.Context, grant AgentRuntimePromotionGrantV1) (bool, error)
	GetActiveRuntimePromotionGrant(ctx context.Context, lookup AgentRuntimePromotionGrantLookupV1) (*AgentRuntimePromotionGrantV1, error)
	RevokeRuntimePromotionGrant(ctx context.Context, grantUUID string, revokedAt time.Time) (bool, error)
}

func validPromotionIdentifierV1(value string, limit int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= limit
}
