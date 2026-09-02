package coremysql_test

import (
	"context"
	"testing"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
)

func TestFileRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	sqlcRepo, err := sqlcRepository.NewFileRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc file repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runFileContract(t, sqlcRepo, "sqlc")
	})
}

func runFileContract(t *testing.T, store application.FileMetadataStore, prefix string) {
	t.Helper()
	if err := store.Create(nil); err == nil {
		t.Fatal("expected nil file creation to fail")
	}
	file := &model.UploadedFile{
		UUID:         "F-" + prefix,
		UploaderUUID: "U100",
		Bucket:       "files",
		ObjectKey:    prefix + "/object.txt",
		FileName:     "object.txt",
		FileSize:     42,
		ContentType:  "text/plain",
		URL:          "https://files.local/" + prefix,
	}
	if err := store.Create(file); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if file.ID == 0 || file.CreatedAt.IsZero() || file.UpdatedAt.IsZero() {
		t.Fatalf("expected generated fields to be populated: %+v", file)
	}
	got, err := store.GetByUUID(file.UUID)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if got == nil || got.ObjectKey != file.ObjectKey || got.FileSize != 42 || got.ContentType != "text/plain" {
		t.Fatalf("unexpected file: %+v", got)
	}
	missing, err := store.GetByUUID("F-missing-" + prefix)
	if err != nil || missing != nil {
		t.Fatalf("expected missing file to return nil, got file=%+v err=%v", missing, err)
	}
}
