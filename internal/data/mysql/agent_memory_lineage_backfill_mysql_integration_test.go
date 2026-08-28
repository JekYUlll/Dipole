package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	memorylineage "github.com/JekYUlll/Dipole/internal/backfill/memorylineage"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlStore "github.com/JekYUlll/Dipole/internal/data/mysql"
)

func TestMemoryLineageBackfillSourceTargetAndCheckpointContract(t *testing.T) {
	db := openTemporaryDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate temporary database: %v", err)
	}

	insertMemoryLineageFixture(t, db, "MEM-LINEAGE-1", "T1", "U1")
	insertMemoryLineageFixture(t, db, "MEM-LINEAGE-2", "T1", "U2")
	insertTaskAndPlan(t, db, "TASK-LINEAGE-1", "T1", "U1", "MEM-LINEAGE-1", "E1")
	insertTaskAndPlan(t, db, "TASK-LINEAGE-2", "T1", "U1", "MEM-LINEAGE-2", "E2")

	store, err := mysqlStore.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	source, err := mysqlStore.NewMemoryLineageBackfillSource(store)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	target, err := mysqlStore.NewMemoryLineageBackfillTarget(store)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	checkpoints, err := mysqlStore.NewMemoryLineageBackfillCheckpointStore(store)
	if err != nil {
		t.Fatalf("create checkpoints: %v", err)
	}

	ctx := context.Background()
	highWatermark, err := source.HighWatermark(ctx)
	if err != nil || highWatermark != 2 {
		t.Fatalf("high watermark=%d err=%v", highWatermark, err)
	}
	items, err := source.ListAfter(ctx, 0, highWatermark, 10)
	if err != nil || len(items) != 2 || items[0].References[0].MemoryUUID != "MEM-LINEAGE-1" || items[1].References[0].MemoryUUID != "" {
		t.Fatalf("source items=%+v err=%v", items, err)
	}

	job, err := memorylineage.NewRunner(source, checkpoints, target, memorylineage.Config{
		JobName: "memory-lineage-contract", OwnerID: "owner-a", BatchSize: 10, LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}
	result, err := job.Run(ctx)
	if err == nil || result.LastProcessedID != 0 {
		t.Fatalf("expected owner-isolated invalid reference to fail before checkpoint advance, result=%+v err=%v", result, err)
	}
	var status string
	var lastProcessed uint64
	if err := db.QueryRow("SELECT status, last_processed_id FROM agent_memory_lineage_backfill_jobs WHERE job_name = ?", "memory-lineage-contract").Scan(&status, &lastProcessed); err != nil {
		t.Fatalf("read failed checkpoint: %v", err)
	}
	if status != memorylineage.StatusFailed || lastProcessed != 0 {
		t.Fatalf("checkpoint status=%s last_processed=%d", status, lastProcessed)
	}

	inserted, duplicate, err := target.Apply(ctx, memorylineage.Reference{MemoryUUID: "MEM-LINEAGE-1", TaskUUID: "TASK-LINEAGE-1", Representation: "full"})
	if err != nil || inserted || !duplicate {
		t.Fatalf("exact replay inserted=%v duplicate=%v err=%v", inserted, duplicate, err)
	}
	var lineageCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM agent_memory_task_lineage").Scan(&lineageCount); err != nil {
		t.Fatalf("count lineage: %v", err)
	}
	if lineageCount != 1 {
		t.Fatalf("valid reference should be replay-safe before foreign-owner failure, count=%d", lineageCount)
	}

	if _, err := checkpoints.Acquire(ctx, "memory-lineage-contract", "owner-b", highWatermark-1, time.Minute); !errors.Is(err, mysqlStore.ErrMemoryLineageBackfillSourceMismatch) {
		t.Fatalf("expected fixed high-water mismatch, got %v", err)
	}
}

func insertMemoryLineageFixture(t *testing.T, db *sql.DB, memoryUUID, tenantID, principalUUID string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO agent_memories (
        memory_uuid, tenant_id, principal_uuid, agent_uuid, memory_type, status,
        resource_type, resource_id, content, priority, source_type, source_id, valid_from,
        memory_root_uuid, memory_version, supersedes_memory_uuid, corrected_by_uuid, correction_reason
    ) VALUES (?, ?, ?, 'AGENT-1', 'semantic', 'active', 'conversation', 'C1', 'redacted-test-content', 1, 'test', ?, NOW(3), ?, 1, NULL, '', '')`, memoryUUID, tenantID, principalUUID, memoryUUID, memoryUUID)
	if err != nil {
		t.Fatalf("insert memory %s: %v", memoryUUID, err)
	}
}

func insertTaskAndPlan(t *testing.T, db *sql.DB, taskUUID, tenantID, principalUUID, memoryUUID, eventID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO agent_tasks (
        task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid,
        status, trigger_type, trigger_ref, goal
    ) VALUES (?, 'DEF-1', 1, ?, ?, 'AGENT-1', 'created', 'message', 'M1', 'test')`, taskUUID, tenantID, principalUUID); err != nil {
		t.Fatalf("insert task %s: %v", taskUUID, err)
	}
	if _, err := db.Exec(`INSERT INTO agent_shadow_plans (
        task_uuid, event_id, event_type, summary, plan_sha256, context_compiler_version,
        context_estimated_tokens, context_manifest_json
    ) VALUES (?, ?, 'message.direct.created', 'test', REPEAT('a', 64), 'v1', 1, ?)`, taskUUID, eventID,
		`{"selected":[{"id":"memory:`+memoryUUID+`","representation":"full"}],"omitted":[]}`); err != nil {
		t.Fatalf("insert plan %s: %v", taskUUID, err)
	}
}
