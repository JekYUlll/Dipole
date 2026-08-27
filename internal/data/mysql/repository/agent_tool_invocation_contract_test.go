package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestAgentToolInvocationRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, _ := migration.NewRunner(db, migrations.Files)
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := db.Exec(`INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES ('TASK-1', 'DEF-1', 1, 'dipole', 'U100', 'UAI', 'running', 'message.created', 'M1', 'audit')`); err != nil {
		t.Fatalf("insert Agent Task fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO agent_runs (run_uuid, task_uuid, runtime_id, mode, status, started_at) VALUES ('RUN-1', 'TASK-1', 'dipole-agent', 'shadow', 'running', ?)`, now); err != nil {
		t.Fatalf("insert Agent Run fixture: %v", err)
	}
	store, err := sqlcRepository.NewAgentToolInvocationRepository(generated.New(db))
	if err != nil {
		t.Fatalf("new Tool invocation repository: %v", err)
	}
	record := application.AgentToolInvocationV1{
		InvocationUUID: "INV-1", TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", TaskUUID: "TASK-1", RunUUID: "RUN-1",
		Transport: application.AgentToolTransportMCP, ToolName: "dipole_conversation_list", CapabilityID: application.AgentCapabilityConversationsList,
		ArgumentsSHA256: testToolInvocationSHA, Status: application.AgentToolInvocationStatusRunning, RequestID: "REQ-1", TraceID: "TRACE-1", StartedAt: now,
	}
	created, err := store.BeginToolInvocation(context.Background(), record)
	if err != nil || !created {
		t.Fatalf("begin Tool invocation: created=%v err=%v", created, err)
	}
	created, err = store.BeginToolInvocation(context.Background(), record)
	if err != nil || created {
		t.Fatalf("duplicate begin must be idempotent: created=%v err=%v", created, err)
	}
	finished, err := store.FinishToolInvocation(context.Background(), application.AgentToolInvocationFinishV1{
		InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Status: application.AgentToolInvocationStatusCompleted,
		ResultSHA256: testToolInvocationSHA, ResultBytes: 128, LatencyMS: 12,
	})
	if err != nil || !finished {
		t.Fatalf("finish Tool invocation: finished=%v err=%v", finished, err)
	}
	finished, err = store.FinishToolInvocation(context.Background(), application.AgentToolInvocationFinishV1{
		InvocationUUID: "INV-1", TaskUUID: "TASK-1", RunUUID: "RUN-1", Status: application.AgentToolInvocationStatusFailed,
		ErrorCode: "tool_execution_failed", LatencyMS: 13,
	})
	if err != nil || finished {
		t.Fatalf("terminal invocation must not transition twice: finished=%v err=%v", finished, err)
	}
	var status, argumentsSHA, resultSHA string
	var resultBytes, latencyMS uint64
	if err := db.QueryRow(`SELECT status, arguments_sha256, result_sha256, result_bytes, latency_ms FROM agent_tool_invocations WHERE invocation_uuid = 'INV-1'`).Scan(&status, &argumentsSHA, &resultSHA, &resultBytes, &latencyMS); err != nil {
		t.Fatalf("read Tool invocation evidence: %v", err)
	}
	if status != "completed" || argumentsSHA != testToolInvocationSHA || resultSHA != testToolInvocationSHA || resultBytes != 128 || latencyMS != 12 {
		t.Fatalf("unexpected Tool invocation evidence: %s %s %s %d %d", status, argumentsSHA, resultSHA, resultBytes, latencyMS)
	}
}

const testToolInvocationSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
