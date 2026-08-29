package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestInitializeSyncServiceRequiresInternalRPCBeforeOpeningDatabase(t *testing.T) {
	_, err := InitializeWithConfig(context.Background(), config.InternalRPC{}, config.MySQL{}, config.Metrics{}, config.Sync{}, config.Kafka{}, config.Cassandra{})
	if err == nil || !strings.Contains(err.Error(), "internal_rpc.enabled") {
		t.Fatalf("expected internal RPC validation, got %v", err)
	}
}
