package repository_test

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	loaded, err := store.GetToolInvocation(context.Background(), "INV-1")
	if err != nil || loaded == nil || loaded.InvocationUUID != "INV-1" || loaded.CapabilityID != application.AgentCapabilityConversationsList {
		t.Fatalf("get Tool invocation: loaded=%+v err=%v", loaded, err)
	}
	external := record
	external.InvocationUUID = "INV-EXT-1"
	external.ToolName = "calendar.create"
	external.ProfileID, external.ServerID, external.ArgumentsJSON = "calendar-prod", "calendar.example", `{"calendarId":"CAL-1"}`
	external.ArgumentsSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(external.ArgumentsJSON)))
	created, err = store.BeginToolInvocation(context.Background(), external)
	if err != nil || !created {
		t.Fatalf("begin external Tool command: created=%v err=%v", created, err)
	}
	loaded, err = store.GetToolInvocation(context.Background(), external.InvocationUUID)
	if err != nil || loaded == nil || loaded.ProfileID != external.ProfileID || loaded.ServerID != external.ServerID || loaded.ArgumentsJSON != external.ArgumentsJSON {
		t.Fatalf("get external Tool command: loaded=%+v err=%v", loaded, err)
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
	loaded, err = store.GetToolInvocation(context.Background(), "INV-1")
	if err != nil || loaded == nil || loaded.Status != application.AgentToolInvocationStatusCompleted || loaded.ResultSHA256 != testToolInvocationSHA ||
		loaded.ResultBytes != 128 || loaded.LatencyMS != 12 || loaded.FinishedAt == nil {
		t.Fatalf("get terminal Tool invocation evidence: loaded=%+v err=%v", loaded, err)
	}

	writeRecord := record
	writeRecord.InvocationUUID = "INV-W"
	writeRecord.ToolName = "dipole_message_send"
	writeRecord.CapabilityID = application.AgentCapabilitySystemMessageSend
	writeRecord.ApprovalUUID = "APR-1"
	created, err = store.BeginToolInvocation(context.Background(), writeRecord)
	if err != nil || !created {
		t.Fatalf("begin write Tool invocation: created=%v err=%v", created, err)
	}
	finished, err = store.FinishToolInvocation(context.Background(), application.AgentToolInvocationFinishV1{
		InvocationUUID: "INV-W", TaskUUID: "TASK-1", RunUUID: "RUN-1", Status: application.AgentToolInvocationStatusCompleted,
		ResultSHA256: testToolInvocationSHA, ResultBytes: 64, LatencyMS: 8,
		ActionReference: &application.AgentToolActionReferenceV1{
			ResourceType: application.AgentToolActionResourceMessage, ResourceUUID: "MSG-1",
			CommandKind: application.AgentMessageCommandSystemMessageV1, CommandID: "CMD-1",
		},
	})
	if err != nil || !finished {
		t.Fatalf("finish write Tool invocation: finished=%v err=%v", finished, err)
	}
	var approvalUUID, resourceType, resourceUUID, commandKind, commandID string
	if err := db.QueryRow(`SELECT approval_uuid, action_resource_type, action_resource_uuid, action_command_kind, action_command_id FROM agent_tool_invocations WHERE invocation_uuid = 'INV-W'`).Scan(&approvalUUID, &resourceType, &resourceUUID, &commandKind, &commandID); err != nil {
		t.Fatalf("read Tool action lineage: %v", err)
	}
	if approvalUUID != "APR-1" || resourceType != "message" || resourceUUID != "MSG-1" || commandKind != "system_message" || commandID != "CMD-1" {
		t.Fatalf("unexpected Tool action lineage: %q %q %q %q %q", approvalUUID, resourceType, resourceUUID, commandKind, commandID)
	}
}

const testToolInvocationSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
