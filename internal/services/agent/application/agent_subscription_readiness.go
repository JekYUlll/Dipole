package agentapplication

import (
	"context"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

const (
	AgentSubscriptionActivationActiveV1            = "active"
	AgentSubscriptionActivationPromotionRequiredV1 = "promotion_required"
	AgentSubscriptionActivationRevokedV1           = "revoked"
)

type AgentSubscriptionActivationResolverV1 struct {
	grants           application.AgentRuntimePromotionGrantStoreV1
	candidateVersion string
	now              func() time.Time
}

func NewAgentSubscriptionActivationResolverV1(grants application.AgentRuntimePromotionGrantStoreV1, candidateVersion string, now func() time.Time) (*AgentSubscriptionActivationResolverV1, error) {
	if grants == nil || candidateVersion == "" || now == nil {
		return nil, application.ErrAgentSubscriptionInvalid
	}
	return &AgentSubscriptionActivationResolverV1{grants: grants, candidateVersion: candidateVersion, now: now}, nil
}

func (r *AgentSubscriptionActivationResolverV1) Resolve(ctx context.Context, subscription application.AgentEventSubscriptionV1) (application.AgentSubscriptionActivationV1, error) {
	if r == nil || subscription.Validate() != nil {
		return application.AgentSubscriptionActivationV1{}, application.ErrAgentSubscriptionInvalid
	}
	if subscription.Status == application.AgentSubscriptionStatusRevoked {
		return application.AgentSubscriptionActivationV1{State: AgentSubscriptionActivationRevokedV1}, nil
	}
	at := r.now().UTC()
	grant, err := r.grants.GetActiveRuntimePromotionGrant(ctx, application.AgentRuntimePromotionGrantLookupV1{TenantID: subscription.TenantID, RuntimeID: "dipole-agent", CandidateVersion: r.candidateVersion, DefinitionUUID: subscription.DefinitionUUID, DefinitionVersion: subscription.DefinitionVersion, At: at})
	if err != nil {
		return application.AgentSubscriptionActivationV1{}, err
	}
	if grant == nil || !grant.Active(at) {
		return application.AgentSubscriptionActivationV1{State: AgentSubscriptionActivationPromotionRequiredV1}, nil
	}
	return application.AgentSubscriptionActivationV1{State: AgentSubscriptionActivationActiveV1, GrantExpiresAt: grant.ExpiresAt.UTC()}, nil
}
