package agentmysql

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
)

type maintenanceLeaseQueryStub struct{}

func (maintenanceLeaseQueryStub) GetAgentOAuthTokenLifecycle(context.Context, string) (generated.AgentOauthTokenLifecycle, error) {
	return generated.AgentOauthTokenLifecycle{}, nil
}
func (maintenanceLeaseQueryStub) InsertAgentOAuthTokenLifecycleMaintenanceLease(context.Context, generated.InsertAgentOAuthTokenLifecycleMaintenanceLeaseParams) (int64, error) {
	return 0, nil
}
func (maintenanceLeaseQueryStub) ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLease(context.Context, generated.ReclaimExpiredAgentOAuthTokenLifecycleMaintenanceLeaseParams) (int64, error) {
	return 0, nil
}
func (maintenanceLeaseQueryStub) GetAgentOAuthTokenLifecycleMaintenanceLease(context.Context, string) (generated.AgentOauthTokenLifecycleMaintenanceLease, error) {
	return generated.AgentOauthTokenLifecycleMaintenanceLease{}, nil
}

func TestAgentOAuthTokenLifecycleMaintenanceLeaseRejectsInvalidClaim(t *testing.T) {
	store, err := NewAgentOAuthTokenLifecycleMaintenanceLeaseRepository(maintenanceLeaseQueryStub{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.ClaimAgentOAuthTokenLifecycleMaintenanceLease(context.Background(), "short", "runtime-key", "worker", now, now.Add(time.Minute)); err != application.ErrAgentOAuthTokenLifecycleInvalid {
		t.Fatalf("invalid claim err=%v", err)
	}
}
