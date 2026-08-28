package repository_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
)

func TestAgentMCPToolRoundRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, _ := migration.NewRunner(db, migrations.Files)
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := db.Exec(`INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES ('TASK-ROUND', 'DEF-1', 1, 'dipole', 'U100', 'UAI', 'running', 'message.created', 'M1', 'round')`); err != nil {
		t.Fatalf("insert Agent Task fixture: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO agent_runs (run_uuid, task_uuid, runtime_id, mode, status, started_at) VALUES ('RUN-ROUND', 'TASK-ROUND', 'dipole-agent', 'active', 'running', ?)`, now); err != nil {
		t.Fatalf("insert Agent Run fixture: %v", err)
	}
	arguments := `{"calendarId":"CAL-1"}`
	argumentsSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(arguments)))
	if _, err := db.Exec(`INSERT INTO agent_tool_invocations (invocation_uuid, tenant_id, principal_uuid, agent_uuid, task_uuid, run_uuid, transport, tool_name, capability_id, arguments_sha256, profile_id, server_id, arguments_json, status, started_at) VALUES ('INV-ROUND', 'dipole', 'U100', 'UAI', 'TASK-ROUND', 'RUN-ROUND', 'mcp', 'calendar.create', 'conversation.list', ?, 'calendar-prod', 'calendar.example', ?, 'running', ?)`, argumentsSHA, arguments, now); err != nil {
		t.Fatalf("insert Tool invocation fixture: %v", err)
	}
	store, err := sqlcRepository.NewAgentMCPToolRoundRepository(generated.New(db))
	if err != nil {
		t.Fatalf("new Tool round repository: %v", err)
	}
	claim := application.AgentMCPToolRoundClaimV1{
		RoundUUID: strings.Repeat("a", 64), InvocationUUID: "INV-ROUND", TaskUUID: "TASK-ROUND", RunUUID: "RUN-ROUND",
		RoundNumber: 0, RequestSHA256: strings.Repeat("b", 64), OwnerTokenSHA256: strings.Repeat("c", 64),
	}
	created, err := store.ClaimMCPToolRound(context.Background(), claim)
	if err != nil || !created {
		t.Fatalf("claim Tool round: created=%v err=%v", created, err)
	}
	created, err = store.ClaimMCPToolRound(context.Background(), claim)
	if err != nil || created {
		t.Fatalf("executing round must not be reclaimed: created=%v err=%v", created, err)
	}
	resultJSON := `{"content":[]}`
	resultSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(resultJSON)))
	wrongOwner := application.AgentMCPToolRoundFinishV1{
		RoundUUID: claim.RoundUUID, OwnerTokenSHA256: strings.Repeat("d", 64), Status: application.AgentMCPToolRoundStatusCompleted,
		ResultJSON: resultJSON, ResultSHA256: resultSHA,
	}
	finished, err := store.FinishMCPToolRound(context.Background(), wrongOwner)
	if err != nil || finished {
		t.Fatalf("wrong owner must not finish: finished=%v err=%v", finished, err)
	}
	finish := wrongOwner
	finish.OwnerTokenSHA256 = claim.OwnerTokenSHA256
	finished, err = store.FinishMCPToolRound(context.Background(), finish)
	if err != nil || !finished {
		t.Fatalf("finish Tool round: finished=%v err=%v", finished, err)
	}
	finished, err = store.FinishMCPToolRound(context.Background(), finish)
	if err != nil || finished {
		t.Fatalf("terminal round must not transition twice: finished=%v err=%v", finished, err)
	}
	loaded, err := store.GetMCPToolRound(context.Background(), claim.RoundUUID)
	if err != nil || loaded == nil || loaded.Status != application.AgentMCPToolRoundStatusCompleted || loaded.ResultJSON != resultJSON || loaded.ResultSHA256 != resultSHA {
		t.Fatalf("load completed Tool round: loaded=%+v err=%v", loaded, err)
	}
}
