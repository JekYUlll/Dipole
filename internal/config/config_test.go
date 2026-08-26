package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestSyncConfigLoadsCassandraShadowHydrationFromEnvironment(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	t.Setenv("DIPOLE_SYNC_CASSANDRA_SHADOW_HYDRATION", "true")
	if err := Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !SyncConfig().CassandraShadowHydration {
		t.Fatal("Sync Cassandra shadow hydration environment override was ignored")
	}
}

func TestConfigDistPlacesCassandraShadowHydrationUnderSync(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "configs", "config.dist.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read config.dist.yaml: %v", err)
	}
	if !v.IsSet("sync.cassandra_shadow_hydration") {
		t.Fatal("sync.cassandra_shadow_hydration is missing from config.dist.yaml")
	}
	if v.IsSet("message.cassandra_shadow_hydration") {
		t.Fatal("Sync hydration setting must not be declared under message")
	}
}

func TestMergeMySQLConfigKeepsGlobalDefaultsAndAppliesSyncCredentials(t *testing.T) {
	global := MySQL{Host: "mysql", Port: 3306, User: "dipole", Password: "global", DBName: "dipole"}
	got := mergeMySQLConfig(global, MySQL{User: "dipole_sync", Password: "sync"})
	want := MySQL{Host: "mysql", Port: 3306, User: "dipole_sync", Password: "sync", DBName: "dipole"}
	if got != want {
		t.Fatalf("merged MySQL config = %+v, want %+v", got, want)
	}
}

func TestMergeMySQLConfigIgnoresEmptySyncOverride(t *testing.T) {
	global := MySQL{Host: "mysql", Port: 3306, User: "dipole", Password: "global", DBName: "dipole"}
	if got := mergeMySQLConfig(global, MySQL{}); got != global {
		t.Fatalf("empty override changed config: %+v", got)
	}
}
