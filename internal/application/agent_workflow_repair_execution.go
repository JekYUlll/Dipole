package application

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const AgentWorkflowRepairExecutionVersionV1 = "dipole.agent.workflow-repair-execution.v1"

var (
	repairExecutionIDPattern = regexp.MustCompile(`^repair-execution:[a-f0-9]{64}$`)
	repairPlanIDPattern      = regexp.MustCompile(`^repair-plan:[a-f0-9]{64}$`)
	repairProposalIDPattern  = regexp.MustCompile(`^repair:[a-f0-9]{64}$`)
	repairHashPattern        = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type AgentWorkflowRepairExecutionStatusV1 string

const (
	AgentWorkflowRepairExecutionStatusPrepared   AgentWorkflowRepairExecutionStatusV1 = "prepared"
	AgentWorkflowRepairExecutionStatusExecuting  AgentWorkflowRepairExecutionStatusV1 = "executing"
	AgentWorkflowRepairExecutionStatusCommitted  AgentWorkflowRepairExecutionStatusV1 = "committed"
	AgentWorkflowRepairExecutionStatusFailed     AgentWorkflowRepairExecutionStatusV1 = "failed"
	AgentWorkflowRepairExecutionStatusRolledBack AgentWorkflowRepairExecutionStatusV1 = "rolled_back"
)

type AgentWorkflowRepairExecutionV1 struct {
	ExecutionUUID, PlanID, ProposalUUID, TaskUUID, ExecutorUUID string
	ExecutorGrantVersion                                        uint64
	ExpectedCurrentSHA256, TargetSHA256, RollbackSHA256         string
	Status                                                      AgentWorkflowRepairExecutionStatusV1
}

func (e AgentWorkflowRepairExecutionV1) Validate() error {
	if !repairExecutionIDPattern.MatchString(strings.TrimSpace(e.ExecutionUUID)) ||
		!repairPlanIDPattern.MatchString(strings.TrimSpace(e.PlanID)) ||
		!repairProposalIDPattern.MatchString(strings.TrimSpace(e.ProposalUUID)) ||
		strings.TrimSpace(e.TaskUUID) == "" || strings.TrimSpace(e.ExecutorUUID) == "" {
		return errors.New("repair execution identity or binding is invalid")
	}
	if !repairHashPattern.MatchString(e.TargetSHA256) {
		return errors.New("repair execution target hash is invalid")
	}
	for _, value := range []string{e.ExpectedCurrentSHA256, e.RollbackSHA256} {
		if value != "" && !repairHashPattern.MatchString(value) {
			return errors.New("repair execution optional hash is invalid")
		}
	}
	if e.ExecutorGrantVersion == 0 || e.Status != AgentWorkflowRepairExecutionStatusPrepared {
		return errors.New("repair execution must be prepared")
	}
	return nil
}

type AgentWorkflowRepairExecutionStoreV1 interface {
	CreateWorkflowRepairExecution(context.Context, AgentWorkflowRepairExecutionV1) (bool, error)
	GetWorkflowRepairExecution(context.Context, string) (*AgentWorkflowRepairExecutionV1, error)
	ClaimWorkflowRepairExecution(context.Context, string, string, uint64, time.Time) (bool, error)
	FailWorkflowRepairExecution(context.Context, string, string, string, time.Time) (bool, error)
}

type AgentWorkflowRepairPrepareRequestV1 struct {
	Execution AgentWorkflowRepairExecutionV1
}

type AgentWorkflowRepairPrepareServiceV1 interface {
	Prepare(context.Context, AgentWorkflowRepairPrepareRequestV1) (*AgentWorkflowRepairExecutionV1, error)
}
