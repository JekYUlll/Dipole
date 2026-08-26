package bootstrap

import (
	"testing"

	appComposition "github.com/JekYUlll/Dipole/internal/app"
)

func TestLegacyGORMRequired(t *testing.T) {
	tests := []struct {
		name        string
		autoMigrate bool
		adapter     string
		want        bool
	}{
		{name: "sqlc default", adapter: appComposition.MySQLAdapterSQLC},
		{name: "gorm rollback", adapter: appComposition.MySQLAdapterGORM, want: true},
		{name: "auto migrate with sqlc", autoMigrate: true, adapter: appComposition.MySQLAdapterSQLC, want: true},
		{name: "auto migrate with gorm", autoMigrate: true, adapter: appComposition.MySQLAdapterGORM, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := legacyGORMRequired(test.autoMigrate, test.adapter); got != test.want {
				t.Fatalf("legacy GORM requirement: got=%v want=%v", got, test.want)
			}
		})
	}
}
