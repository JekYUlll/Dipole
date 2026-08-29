package bootstrap

import (
	"strings"
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestValidateSyncHydrationConfig(t *testing.T) {
	if err := validateSyncHydrationConfig(config.Sync{}, config.Cassandra{}); err != nil {
		t.Fatalf("disabled shadow: %v", err)
	}
	if err := validateSyncHydrationConfig(config.Sync{CassandraShadowHydration: true}, config.Cassandra{}); err == nil || !strings.Contains(err.Error(), "cassandra.enabled") {
		t.Fatalf("missing Cassandra error=%v", err)
	}
	if err := validateSyncHydrationConfig(config.Sync{CassandraShadowHydration: true}, config.Cassandra{Enabled: true}); err != nil {
		t.Fatalf("enabled shadow: %v", err)
	}
	if err := validateSyncHydrationConfig(config.Sync{CassandraPrimaryHydration: true}, config.Cassandra{Enabled: true}); err != nil {
		t.Fatalf("enabled primary: %v", err)
	}
	if err := validateSyncHydrationConfig(config.Sync{CassandraShadowHydration: true, CassandraPrimaryHydration: true}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected shadow and primary hydration conflict")
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
