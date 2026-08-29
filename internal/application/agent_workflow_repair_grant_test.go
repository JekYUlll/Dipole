package application

import (
	"testing"
	"time"
)

func TestAgentWorkflowRepairGrantSeparatesVersionAndExecutionAuthority(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	grant := AgentWorkflowRepairOperatorGrantV1{
		UserUUID: "EXECUTOR-1", GrantedByUUID: "ADMIN-1", Version: 7,
		CanExecute: true, ValidFrom: now.Add(-time.Minute), ExpiresAt: ptrTime(now.Add(time.Minute)),
	}
	if !grant.Active(now) || grant.Version != 7 || !grant.CanExecute {
		t.Fatalf("executor grant = %+v", grant)
	}
	legacy := grant
	legacy.Version = 6
	legacy.CanExecute = false
	if !legacy.Active(now) || legacy.CanExecute {
		t.Fatalf("legacy grant must remain active without execution authority: %+v", legacy)
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
