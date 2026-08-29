package agentmysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	agenttimelinereconcile "github.com/JekYUlll/Dipole/internal/operations/agent/reconcile"
	sqlcrepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
)

type failingTimelineStore struct{ err error }

func (s failingTimelineStore) AppendAgentTaskTimelineEvent(context.Context, application.AgentTaskTimelineEventV1) (uint64, error) {
	return 0, s.err
}
func (s failingTimelineStore) ListAgentTaskTimelineEvents(context.Context, string, uint64, int) ([]application.AgentTaskTimelineEventV1, error) {
	return nil, nil
}

func TestAgentTaskTimelineRepairMySQLFaultRecoveryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	mysqlStore, err := mysqldata.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := sqlcrepository.NewAgentPolicyRepositoryWithTransactions(mysqlStore)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "DEF-REPAIR-" + now.Format("150405.000"), Version: 1, TenantID: "dipole", OwnerUUID: "U-REPAIR", AgentUUID: "A-REPAIR",
		Status: application.AgentDefinitionStatusActive, Permissions: []string{"conversation.read"},
		Scopes: []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"read"}}}, ValidFrom: now,
	}
	if err := policy.CreateDefinitionVersion(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	taskUUID := "TASK-REPAIR-" + now.Format("150405.000")
	task := application.AgentTaskV1{TaskUUID: taskUUID, DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: 1, TenantID: "dipole", PrincipalUUID: "U-REPAIR", AgentUUID: "A-REPAIR", Status: application.AgentTaskStatusRunning, TriggerType: "manual", TriggerRef: "repair-contract", Goal: "repair timeline"}
	if _, err := policy.CreateTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	runUUID := "RUN-REPAIR-" + now.Format("150405.000")
	if _, err := policy.CreateRun(context.Background(), application.AgentRunV1{RunUUID: runUUID, TaskUUID: taskUUID, RuntimeID: "dipole-agent", Mode: "shadow", Status: application.AgentRunStatusRunning}); err != nil {
		t.Fatal(err)
	}
	event := application.AgentTaskTimelineEventV1{EventUUID: "repair-contract:" + now.Format("150405.000000"), TaskUUID: taskUUID, RunUUID: runUUID, Kind: application.AgentTaskTimelineEventModelCall, Status: "completed", OccurredAt: now}
	if err := policy.EnqueueAgentTaskTimelineRepair(context.Background(), event, errors.New("injected timeline outage")); err != nil {
		t.Fatal(err)
	}
	var seededStatus string
	var seededNext time.Time
	if err := db.QueryRow("SELECT repair_status, next_retry_at FROM agent_task_timeline_repairs WHERE event_uuid = ?", event.EventUUID).Scan(&seededStatus, &seededNext); err != nil {
		t.Fatal(err)
	}
	claimNow := seededNext.Add(100 * time.Millisecond)
	if seededStatus != "pending" {
		t.Fatalf("seeded repair state status=%q next=%s", seededStatus, seededNext)
	}

	failingRepairer, err := agenttimelinereconcile.NewRepairer(policy, failingTimelineStore{err: errors.New("injected timeline outage")}, 10, time.Minute, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err := failingRepairer.RunOnce(context.Background(), claimNow)
	if err != nil || report.Retried != 1 {
		t.Fatalf("fault run report=%+v err=%v", report, err)
	}

	recoveryRepairer, err := agenttimelinereconcile.NewRepairer(policy, policy, 10, time.Minute, time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	report, err = recoveryRepairer.RunOnce(context.Background(), claimNow.Add(2*time.Second))
	if err != nil || report.Repaired != 1 {
		t.Fatalf("recovery run report=%+v err=%v", report, err)
	}
	var repairStatus string
	if err := db.QueryRow("SELECT repair_status FROM agent_task_timeline_repairs WHERE event_uuid = ?", event.EventUUID).Scan(&repairStatus); err != nil {
		t.Fatal(err)
	}
	if repairStatus != "completed" {
		t.Fatalf("repair_status=%q, want completed", repairStatus)
	}
	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM agent_task_timeline_events WHERE event_uuid = ?", event.EventUUID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("timeline event count=%d, want 1", eventCount)
	}
}
