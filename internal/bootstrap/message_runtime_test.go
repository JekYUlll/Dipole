package bootstrap

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestValidateCassandraShadowConfigRequiresExplicitCassandraEnablement(t *testing.T) {
	if err := validateCassandraShadowConfig(config.Message{}, config.Cassandra{}); err != nil {
		t.Fatalf("disabled shadow read should preserve the default runtime: %v", err)
	}
	if err := validateCassandraShadowConfig(
		config.Message{CassandraShadowReads: true},
		config.Cassandra{},
	); err == nil {
		t.Fatal("expected Cassandra shadow reads without Cassandra to fail")
	}
	if err := validateCassandraShadowConfig(
		config.Message{CassandraShadowReads: true},
		config.Cassandra{Enabled: true},
	); err != nil {
		t.Fatalf("enabled Cassandra shadow reads should pass validation: %v", err)
	}
}
