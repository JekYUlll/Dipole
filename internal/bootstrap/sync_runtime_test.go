package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestInitializeSyncServiceRequiresInternalRPCBeforeOpeningDatabase(t *testing.T) {
	_, err := initializeSyncService(context.Background(), config.InternalRPC{}, config.MySQL{}, config.Metrics{}, config.Sync{}, config.Kafka{})
	if err == nil || !strings.Contains(err.Error(), "internal_rpc.enabled") {
		t.Fatalf("expected internal RPC validation, got %v", err)
	}
}

func TestValidateSyncProjectorConfig(t *testing.T) {
	if err := validateSyncProjectorConfig(config.Sync{}, config.Kafka{}); err != nil {
		t.Fatalf("disabled projector should not require Kafka: %v", err)
	}
	if err := validateSyncProjectorConfig(config.Sync{ProjectorEnabled: true}, config.Kafka{}); err == nil || !strings.Contains(err.Error(), "kafka.enabled") {
		t.Fatalf("expected enabled projector to require Kafka, got %v", err)
	}
	if err := validateSyncProjectorConfig(config.Sync{ProjectorEnabled: true}, config.Kafka{Enabled: true}); err != nil {
		t.Fatalf("enabled projector with Kafka should pass: %v", err)
	}
}
