package config

import "testing"

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
