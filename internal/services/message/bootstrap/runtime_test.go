package bootstrap

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestValidateCassandraTimelineConfigRequiresExplicitCassandraEnablement(t *testing.T) {
	if err := validateCassandraTimelineConfig(config.Message{}, config.Cassandra{}); err != nil {
		t.Fatalf("disabled shadow read should preserve the default runtime: %v", err)
	}
	if err := validateCassandraTimelineConfig(
		config.Message{CassandraShadowReads: true},
		config.Cassandra{},
	); err == nil {
		t.Fatal("expected Cassandra shadow reads without Cassandra to fail")
	}
	if err := validateCassandraTimelineConfig(
		config.Message{CassandraShadowReads: true},
		config.Cassandra{Enabled: true},
	); err != nil {
		t.Fatalf("enabled Cassandra shadow reads should pass validation: %v", err)
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraReadPercent: 101}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected percentage above 100 to fail")
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraShadowReads: true, CassandraReadPercent: 1}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected shadow and primary Cassandra reads to be mutually exclusive")
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraReadPercent: 10}, config.Cassandra{}); err == nil {
		t.Fatal("expected Cassandra cohort without Cassandra to fail")
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraReadVerifyPercent: 101}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected verification percentage above 100 to fail")
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraReadVerifyPercent: 10}, config.Cassandra{Enabled: true}); err == nil {
		t.Fatal("expected verification without a Cassandra read cohort to fail")
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraReadPercent: 10, CassandraReadVerifyPercent: 100}, config.Cassandra{Enabled: true}); err != nil {
		t.Fatalf("expected Cassandra primary verification to pass: %v", err)
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraDuplicateHydration: true}, config.Cassandra{}); err == nil {
		t.Fatal("expected duplicate hydration without Cassandra to fail")
	}
	if err := validateCassandraTimelineConfig(config.Message{CassandraDuplicateHydration: true}, config.Cassandra{Enabled: true}); err != nil {
		t.Fatalf("expected Cassandra duplicate hydration to pass: %v", err)
	}
}
