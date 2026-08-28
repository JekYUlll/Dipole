package app

import (
	"context"
	"errors"

	"github.com/JekYUlll/Dipole/internal/application"
)

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
