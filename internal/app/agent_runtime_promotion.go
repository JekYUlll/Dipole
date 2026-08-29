package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type PersistentAgentActiveRunPromotionAuthorizerV1 struct {
	store application.AgentRuntimePromotionGrantStoreV1
	now   func() time.Time
}

var _ application.AgentActiveRunPromotionAuthorizerV1 = (*PersistentAgentActiveRunPromotionAuthorizerV1)(nil)

func NewPersistentAgentActiveRunPromotionAuthorizerV1(store application.AgentRuntimePromotionGrantStoreV1) (*PersistentAgentActiveRunPromotionAuthorizerV1, error) {
	if store == nil {
		return nil, fmt.Errorf("persistent Agent active Run promotion authorizer requires store")
	}
	return &PersistentAgentActiveRunPromotionAuthorizerV1{store: store, now: time.Now}, nil
}

func (a *PersistentAgentActiveRunPromotionAuthorizerV1) AuthorizeActiveRun(ctx context.Context, request application.AgentActiveRunPromotionRequestV1) error {
	runtimeID, candidateVersion := strings.TrimSpace(request.RuntimeID), strings.TrimSpace(request.CandidateVersion)
	if runtimeID == "" || candidateVersion == "" || request.Task.Validate() != nil || request.Definition.Validate() != nil ||
		request.Task.DefinitionUUID != request.Definition.DefinitionUUID || request.Task.DefinitionVersion != request.Definition.Version ||
		request.Task.TenantID != request.Definition.TenantID || request.Task.AgentUUID != request.Definition.AgentUUID {
		return fmt.Errorf("%w: active Runtime promotion binding is invalid", application.ErrAgentExecutionPolicyDenied)
	}
	at := a.now().UTC()
	lookup := application.AgentRuntimePromotionGrantLookupV1{
		TenantID: request.Task.TenantID, RuntimeID: runtimeID, CandidateVersion: candidateVersion,
		DefinitionUUID: request.Definition.DefinitionUUID, DefinitionVersion: request.Definition.Version, At: at,
	}
	grant, err := a.store.GetActiveRuntimePromotionGrant(ctx, lookup)
	if err != nil {
		return fmt.Errorf("get active Runtime promotion grant: %w", err)
	}
	if grant == nil || !grant.Active(at) || grant.TenantID != lookup.TenantID || grant.RuntimeID != lookup.RuntimeID ||
		grant.CandidateVersion != lookup.CandidateVersion || grant.DefinitionUUID != lookup.DefinitionUUID ||
		grant.DefinitionVersion != lookup.DefinitionVersion || grant.PolicyVersion != application.AgentRuntimePromotionPolicyVersionV2 {
		return fmt.Errorf("%w: active Runtime promotion grant is unavailable", application.ErrAgentExecutionPolicyDenied)
	}
	return nil
}
