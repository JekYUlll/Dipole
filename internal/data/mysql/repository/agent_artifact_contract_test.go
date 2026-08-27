package repository_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestAgentArtifactMySQLImmutableConcurrencyContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	policy, _ := sqlcRepository.NewAgentPolicyRepository(generated.New(db))
	artifacts, _ := sqlcRepository.NewAgentArtifactRepository(generated.New(db))
	now := time.Now().UTC().Truncate(time.Millisecond)
	definition := application.AgentDefinitionVersionV1{DefinitionUUID: "DEF-ART", Version: 1, TenantID: "dipole", OwnerUUID: "U1", AgentUUID: "UAI", Status: application.AgentDefinitionStatusActive, Permissions: []string{"conversation.read"}, Scopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"read"}}}, ValidFrom: now}
	if err := policy.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	task := application.AgentTaskV1{TaskUUID: "TASK-ART", DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: 1, TenantID: "dipole", PrincipalUUID: "U1", AgentUUID: "UAI", Status: application.AgentTaskStatusRunning, TriggerType: "manual", TriggerRef: "ART-1", Goal: "create report"}
	if _, err := policy.CreateTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	run := application.AgentRunV1{RunUUID: "RUN-ART", TaskUUID: task.TaskUUID, RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}
	if _, err := policy.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	artifact := application.AgentArtifactV1{ArtifactUUID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SchemaVersion: application.AgentArtifactSchemaVersionV1, TaskUUID: task.TaskUUID, RunUUID: run.RunUUID, ArtifactType: "project_report", Version: 1, Title: "Report", MediaType: "text/markdown", ObjectBucket: "dipole-agent-artifacts", ObjectKey: "agent-artifacts/v1/object", ContentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SizeBytes: 6, Metadata: json.RawMessage(`{"source":"G1"}`)}
	var inserted atomic.Int64
	var workers sync.WaitGroup
	errorsCh := make(chan error, 16)
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			created, createErr := artifacts.CreateAgentArtifact(context.Background(), artifact)
			if createErr != nil {
				errorsCh <- createErr
				return
			}
			if created {
				inserted.Add(1)
			}
		}()
	}
	workers.Wait()
	close(errorsCh)
	for createErr := range errorsCh {
		t.Fatal(createErr)
	}
	if inserted.Load() != 1 {
		t.Fatalf("inserted=%d, want 1", inserted.Load())
	}
	loaded, err := artifacts.GetAgentArtifactByTaskTypeVersion(context.Background(), task.TaskUUID, artifact.ArtifactType, 1)
	if err != nil || loaded == nil || loaded.ArtifactUUID != artifact.ArtifactUUID || string(loaded.Metadata) != string(artifact.Metadata) {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	if exists, err := artifacts.ExistsByObjectKey(context.Background(), artifact.ObjectBucket, artifact.ObjectKey); err != nil || !exists {
		t.Fatalf("existing Artifact object lookup=%t err=%v", exists, err)
	}
	if exists, err := artifacts.ExistsByObjectKey(context.Background(), artifact.ObjectBucket, artifact.ObjectKey+"-missing"); err != nil || exists {
		t.Fatalf("missing Artifact object lookup=%t err=%v", exists, err)
	}
	currentVersion, err := runner.CurrentVersion(context.Background())
	if err != nil {
		t.Fatalf("read current migration: %v", err)
	}
	if err := runner.Down(context.Background(), int(currentVersion-25)); err != nil {
		t.Fatalf("rollback migration v26: %v", err)
	}
	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'agent_artifacts'`).Scan(&tableCount); err != nil || tableCount != 0 {
		t.Fatalf("Artifact table after rollback: count=%d err=%v", tableCount, err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("reapply migration v26: %v", err)
	}
	if version, err := runner.CurrentVersion(context.Background()); err != nil || version != currentVersion {
		t.Fatalf("migration version after reapply: version=%d err=%v", version, err)
	}
}
