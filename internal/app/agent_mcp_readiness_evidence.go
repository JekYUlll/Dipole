package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentMCPReadinessEvidenceResolverV1 struct {
	store application.AgentMCPReadinessEvidenceStoreV1
	now   func() time.Time
}

var _ application.AgentMCPReadinessEvidenceResolverV1 = (*PersistentAgentMCPReadinessEvidenceResolverV1)(nil)

func NewPersistentAgentMCPReadinessEvidenceResolverV1(store application.AgentMCPReadinessEvidenceStoreV1, now func() time.Time) (*PersistentAgentMCPReadinessEvidenceResolverV1, error) {
	if store == nil {
		return nil, errors.New("Agent MCP readiness evidence Store is required")
	}
	if now == nil {
		now = time.Now
	}
	return &PersistentAgentMCPReadinessEvidenceResolverV1{store: store, now: now}, nil
}

func (resolver *PersistentAgentMCPReadinessEvidenceResolverV1) ResolveFreshAgentMCPReadinessEvidence(ctx context.Context, tenantID, profileBinding, runtimeBinding string) (*application.AgentMCPReadinessEvidenceRecordV1, error) {
	tenantID, profileBinding, runtimeBinding = strings.TrimSpace(tenantID), strings.TrimSpace(profileBinding), strings.TrimSpace(runtimeBinding)
	if tenantID == "" || len(tenantID) > 64 || !validReadinessSHA256(profileBinding) || !validReadinessSHA256(runtimeBinding) {
		return nil, application.ErrAgentMCPReadinessEvidenceInvalid
	}
	at := resolver.now().UTC().Truncate(time.Millisecond)
	if at.IsZero() {
		return nil, application.ErrAgentMCPReadinessEvidenceInvalid
	}
	record, err := resolver.store.GetFreshAgentMCPReadinessEvidence(ctx, application.AgentMCPReadinessEvidenceLookupV1{
		TenantID: tenantID, ProfileBindingSHA256: profileBinding, RuntimeBindingSHA256: runtimeBinding, At: at,
	})
	if err != nil || record == nil {
		return record, err
	}
	if record.Validate() != nil || record.TenantID != tenantID || record.ProfileBindingSHA256 != profileBinding ||
		record.RuntimeBindingSHA256 != runtimeBinding || record.CollectedAt.After(at) || !record.ExpiresAt.After(at) {
		return nil, application.ErrAgentMCPReadinessEvidenceInvalid
	}
	return record, nil
}

func validReadinessSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

type PersistentAgentMCPReadinessEvidencePublisherV1 struct {
	store application.AgentMCPReadinessEvidenceStoreV1
}

var _ application.AgentMCPReadinessEvidencePublisherV1 = (*PersistentAgentMCPReadinessEvidencePublisherV1)(nil)

func NewPersistentAgentMCPReadinessEvidencePublisherV1(store application.AgentMCPReadinessEvidenceStoreV1) (*PersistentAgentMCPReadinessEvidencePublisherV1, error) {
	if store == nil {
		return nil, errors.New("Agent MCP readiness evidence Store is required")
	}
	return &PersistentAgentMCPReadinessEvidencePublisherV1{store: store}, nil
}

func (publisher *PersistentAgentMCPReadinessEvidencePublisherV1) PublishAgentMCPReadinessEvidence(
	ctx context.Context,
	operatorUUID string,
	request application.AgentMCPReadinessEvidenceRequestV1,
) (*application.AgentMCPReadinessEvidenceRecordV1, bool, error) {
	record, err := application.NewAgentMCPReadinessEvidenceRecordV1(operatorUUID, request)
	if err != nil {
		return nil, false, err
	}
	created, err := publisher.store.AppendAgentMCPReadinessEvidence(ctx, record)
	if err != nil {
		return nil, false, err
	}
	return &record, created, nil
}
