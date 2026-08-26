package migration

import (
	"testing"
	"testing/fstest"
)

func TestLoadOrdersPairedMigrations(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"000002_second.up.sql":     {Data: []byte("SELECT 2")},
		"000002_second.down.sql":   {Data: []byte("SELECT -2")},
		"000001_baseline.up.sql":   {Data: []byte("SELECT 1")},
		"000001_baseline.down.sql": {Data: []byte("SELECT -1")},
		"README.md":                {Data: []byte("ignored")},
	}

	migrations, err := load(files)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected two migrations, got %d", len(migrations))
	}
	if migrations[0].Version != 1 || migrations[0].Name != "baseline" {
		t.Fatalf("unexpected first migration: %+v", migrations[0])
	}
	if migrations[1].Version != 2 || migrations[1].Name != "second" {
		t.Fatalf("unexpected second migration: %+v", migrations[1])
	}
}

func TestLoadRejectsUnpairedMigration(t *testing.T) {
	t.Parallel()

	_, err := load(fstest.MapFS{
		"000001_baseline.up.sql": {Data: []byte("SELECT 1")},
	})
	if err == nil {
		t.Fatal("expected unpaired migration to fail")
	}
}
