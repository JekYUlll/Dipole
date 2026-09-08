package agentapplication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

type subscriptionActivationGrantStoreStub struct {
	grant *application.AgentRuntimePromotionGrantV1
	err   error
}

func (s subscriptionActivationGrantStoreStub) CreateRuntimePromotionGrant(context.Context, application.AgentRuntimePromotionGrantV1) (bool, error) {
	return false, nil
}
func (s subscriptionActivationGrantStoreStub) GetActiveRuntimePromotionGrant(context.Context, application.AgentRuntimePromotionGrantLookupV1) (*application.AgentRuntimePromotionGrantV1, error) {
	return s.grant, s.err
}
func (s subscriptionActivationGrantStoreStub) RevokeRuntimePromotionGrant(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

func TestAgentSubscriptionActivationResolver(t *testing.T) {
	now := time.Date(2026, time.September, 8, 0, 0, 0, 0, time.UTC)
	subscription := subscriptionFixture("SUB-1")
	grant := application.AgentRuntimePromotionGrantV1{GrantUUID: "GRANT-1", TenantID: subscription.TenantID, RuntimeID: "dipole-agent", CandidateVersion: "candidate-v1", DefinitionUUID: subscription.DefinitionUUID, DefinitionVersion: subscription.DefinitionVersion, PolicyVersion: application.AgentRuntimePromotionPolicyVersionV2, EvidenceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", EvalSuiteSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", GrantedByUUID: "U100", ReviewedByUUID: "U200", ValidFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	resolver, err := NewAgentSubscriptionActivationResolverV1(subscriptionActivationGrantStoreStub{grant: &grant}, "candidate-v1", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolver.Resolve(context.Background(), subscription)
	if err != nil || resolved.State != AgentSubscriptionActivationActiveV1 || !resolved.GrantExpiresAt.Equal(grant.ExpiresAt) {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	resolver, _ = NewAgentSubscriptionActivationResolverV1(subscriptionActivationGrantStoreStub{}, "candidate-v1", func() time.Time { return now })
	resolved, err = resolver.Resolve(context.Background(), subscription)
	if err != nil || resolved.State != AgentSubscriptionActivationPromotionRequiredV1 {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	subscription.Status = application.AgentSubscriptionStatusRevoked
	revokedAt := now
	subscription.RevokedAt = &revokedAt
	subscription.RevokedByUUID = "U200"
	subscription.RevokeReason = "owner revoked"
	resolved, err = resolver.Resolve(context.Background(), subscription)
	if err != nil || resolved.State != AgentSubscriptionActivationRevokedV1 {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	resolver, _ = NewAgentSubscriptionActivationResolverV1(subscriptionActivationGrantStoreStub{err: errors.New("db")}, "candidate-v1", func() time.Time { return now })
	if _, err = resolver.Resolve(context.Background(), subscriptionFixture("SUB-2")); err == nil {
		t.Fatal("expected store error")
	}
}
