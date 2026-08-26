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
	if err := validateCassandraShadowConfig(config.Message{CassandraReadPercent: 101}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected percentage above 100 to fail")
	}
	if err := validateCassandraShadowConfig(config.Message{CassandraShadowReads: true, CassandraReadPercent: 1}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected shadow and primary Cassandra reads to be mutually exclusive")
	}
	if err := validateCassandraShadowConfig(config.Message{CassandraReadPercent: 10}, config.Cassandra{}); err == nil {
		t.Fatal("expected Cassandra cohort without Cassandra to fail")
	}
	if err := validateCassandraShadowConfig(config.Message{CassandraReadVerifyPercent: 101}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected verification percentage above 100 to fail")
	}
	if err := validateCassandraShadowConfig(config.Message{CassandraReadVerifyPercent: 10}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected verification without a Cassandra read cohort to fail")
	}
	if err := validateCassandraShadowConfig(config.Message{CassandraReadPercent: 10, CassandraReadVerifyPercent: 100}, config.Cassandra{Enabled: true}); err != nil {
		t.Fatalf("expected Cassandra primary verification to pass: %v", err)
	}
}
