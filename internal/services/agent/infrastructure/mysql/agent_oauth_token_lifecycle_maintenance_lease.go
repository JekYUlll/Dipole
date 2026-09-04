package agentmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type AgentOAuthTokenLifecycleMaintenanceLeaseQueries interface {
	GetAgentOAuthTokenLifecycle(context.Context, string) (generated.AgentOauthTokenLifecycle, error)
	InsertAgentOAuthTokenLifecycleMaintenanceLease(context.Context, generated.InsertAgentOAuthTokenLifecycleMaintenanceLeaseParams) (int64, error)
	ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLease(context.Context, generated.ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLeaseParams) (int64, error)
	GetAgentOAuthTokenLifecycleMaintenanceLease(context.Context, string) (generated.AgentOauthTokenLifecycleMaintenanceLease, error)
}

type AgentOAuthTokenLifecycleMaintenanceLeaseRepository struct {
	queries AgentOAuthTokenLifecycleMaintenanceLeaseQueries
}

var _ application.AgentOAuthTokenLifecycleMaintenanceLeaseStoreV1 = (*AgentOAuthTokenLifecycleMaintenanceLeaseRepository)(nil)

func NewAgentOAuthTokenLifecycleMaintenanceLeaseRepository(queries AgentOAuthTokenLifecycleMaintenanceLeaseQueries) (*AgentOAuthTokenLifecycleMaintenanceLeaseRepository, error) {
	if queries == nil {
		return nil, errors.New("Agent OAuth lifecycle maintenance lease queries are required")
	}
	return &AgentOAuthTokenLifecycleMaintenanceLeaseRepository{queries: queries}, nil
}

func (r *AgentOAuthTokenLifecycleMaintenanceLeaseRepository) ClaimAgentOAuthTokenLifecycleMaintenanceLease(ctx context.Context, handoffID, runtimeKeyID, owner string, now, expiresAt time.Time) (*application.AgentOAuthTokenLifecycleMaintenanceLeaseV1, error) {
	if !validHandoffIdentifier(handoffID) || !validHandoffLeaseOwner(runtimeKeyID) || !validHandoffLeaseOwner(owner) || now.IsZero() || expiresAt.IsZero() || !expiresAt.After(now) {
		return nil, application.ErrAgentOAuthTokenLifecycleInvalid
	}
	now, expiresAt = canonicalHandoffTime(now), canonicalHandoffTime(expiresAt)
	lifecycle, err := r.queries.GetAgentOAuthTokenLifecycle(ctx, handoffID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent OAuth token lifecycle for maintenance: %w", err)
	}
	if lifecycle.RuntimeKeyID != runtimeKeyID || (lifecycle.State != string(application.AgentOAuthTokenLifecycleActiveV1) && lifecycle.State != string(application.AgentOAuthTokenLifecycleRefreshedV1)) {
		return nil, nil
	}
	inserted, err := r.queries.InsertAgentOAuthTokenLifecycleMaintenanceLease(ctx, generated.InsertAgentOAuthTokenLifecycleMaintenanceLeaseParams{HandoffUuid: handoffID, RuntimeKeyID: runtimeKeyID, LeaseOwner: owner, LeaseExpiresAt: expiresAt})
	if err != nil {
		return nil, fmt.Errorf("insert Agent OAuth lifecycle maintenance lease: %w", err)
	}
	if inserted == 0 {
		if _, err := r.queries.ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLease(ctx, generated.ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLeaseParams{LeaseOwner: owner, LeaseExpiresAt: expiresAt, HandoffUuid: handoffID, RuntimeKeyID: runtimeKeyID, LeaseExpiresAt_2: now}); err != nil {
			return nil, fmt.Errorf("reclaim Agent OAuth lifecycle maintenance lease: %w", err)
		}
	}
	lease, err := r.queries.GetAgentOAuthTokenLifecycleMaintenanceLease(ctx, handoffID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Agent OAuth lifecycle maintenance lease: %w", err)
	}
	if lease.RuntimeKeyID != runtimeKeyID || lease.LeaseOwner != owner || !lease.LeaseExpiresAt.Equal(expiresAt) {
		return nil, nil
	}
	return &application.AgentOAuthTokenLifecycleMaintenanceLeaseV1{HandoffUUID: lease.HandoffUuid, RuntimeKeyID: lease.RuntimeKeyID, LeaseOwner: lease.LeaseOwner, LeaseGeneration: lease.LeaseGeneration, LeaseExpiresAt: lease.LeaseExpiresAt}, nil
}
