package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrAgentWorkflowRepairPrecondition = errors.New("agent workflow repair precondition failed")

// AgentWorkflowRepairPreconditionV1 is the side-effect-free gate shared by a future executor.
// It keeps authorization and projection hashes outside the mutation operation.
type AgentWorkflowRepairPreconditionV1 struct {
	Execution AgentWorkflowRepairExecutionV1
	Grant     AgentWorkflowRepairOperatorGrantV1
	Current   *AgentTaskWorkflowProjectionV1
	Target    AgentTaskWorkflowProjectionV1
	At        time.Time
}

func (p AgentWorkflowRepairPreconditionV1) Validate() error {
	if p.At.IsZero() {
		return fmt.Errorf("%w: evaluation time is required", ErrAgentWorkflowRepairPrecondition)
	}
	if p.Execution.Status != AgentWorkflowRepairExecutionStatusPrepared && p.Execution.Status != AgentWorkflowRepairExecutionStatusExecuting {
		return fmt.Errorf("%w: execution is not claimable", ErrAgentWorkflowRepairPrecondition)
	}
	if p.Execution.ExecutorUUID != p.Grant.UserUUID || p.Execution.ExecutorGrantVersion != p.Grant.Version || !p.Grant.CanExecute || !p.Grant.Active(p.At) {
		return fmt.Errorf("%w: executor grant is unavailable", ErrAgentWorkflowRepairPrecondition)
	}
	if err := p.Target.Validate(); err != nil || p.Target.TaskUUID != p.Execution.TaskUUID {
		return fmt.Errorf("%w: target projection is invalid or unbound", ErrAgentWorkflowRepairPrecondition)
	}
	if digest := workflowProjectionSHA256(&p.Target); digest != p.Execution.TargetSHA256 {
		return fmt.Errorf("%w: target projection hash mismatch", ErrAgentWorkflowRepairPrecondition)
	}
	if p.Execution.ExpectedCurrentSHA256 == "" {
		if p.Current != nil {
			return fmt.Errorf("%w: expected missing current projection", ErrAgentWorkflowRepairPrecondition)
		}
	} else if p.Current == nil || p.Current.TaskUUID != p.Execution.TaskUUID || workflowProjectionSHA256(p.Current) != p.Execution.ExpectedCurrentSHA256 {
		return fmt.Errorf("%w: current projection hash mismatch", ErrAgentWorkflowRepairPrecondition)
	}
	if p.Current != nil && workflowProjectionSHA256(p.Current) == workflowProjectionSHA256(&p.Target) {
		return fmt.Errorf("%w: target projection is unchanged", ErrAgentWorkflowRepairPrecondition)
	}
	return nil
}

func workflowProjectionSHA256(projection *AgentTaskWorkflowProjectionV1) string {
	if projection == nil {
		return ""
	}
	payload, _ := json.Marshal(struct {
		Revision      uint64 `json:"revision"`
		Status        string `json:"status"`
		WorkflowID    string `json:"workflowId"`
		WorkflowRunID string `json:"workflowRunId"`
	}{projection.Revision, string(projection.Status), projection.WorkflowID, projection.RunID})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
