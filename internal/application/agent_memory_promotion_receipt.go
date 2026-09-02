package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	AgentMemoryPromotionReceiptSchemaV2 = "dipole.agent.memory-promotion-receipt.v2"
	AgentMemoryPromotionReceiptPrepared = "prepared"
)

// AgentMemoryPromotionReceiptCommitRequestV1 contains only the replay-safe
// receipt binding. Core derives the owner and tenant from the persisted Run.
type AgentMemoryPromotionReceiptCommitRequestV1 struct {
	ReceiptID, ReceiptSHA256, SchemaVersion, Status   string
	TaskUUID, RunUUID, CandidateUUID, CandidateSHA256 string
	ReviewUUID, PolicyVersion                         string
	TargetMemoryType                                  AgentMemoryTypeV1
	CreatedAt, ExpiresAt                              time.Time
}

type AgentMemoryPromotionReceiptCommitServiceV1 interface {
	CommitMemoryPromotionReceipt(ctx context.Context, request AgentMemoryPromotionReceiptCommitRequestV1) (*AgentMemoryV1, error)
}

func (r AgentMemoryPromotionReceiptCommitRequestV1) ValidateAt(now time.Time) error {
	if r.SchemaVersion != AgentMemoryPromotionReceiptSchemaV2 || r.Status != AgentMemoryPromotionReceiptPrepared ||
		anyBlank(r.ReceiptID, r.ReceiptSHA256, r.TaskUUID, r.RunUUID, r.CandidateUUID, r.CandidateSHA256, r.ReviewUUID, r.PolicyVersion) ||
		!validSHA256V1(r.ReceiptSHA256) || !validSHA256V1(r.CandidateSHA256) || !IsPersistentAgentMemoryTypeV1(r.TargetMemoryType) ||
		r.CreatedAt.IsZero() || r.ExpiresAt.IsZero() || r.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 || r.ExpiresAt.Nanosecond()%int(time.Millisecond) != 0 || !r.CreatedAt.Before(r.ExpiresAt) || now.IsZero() || !now.Before(r.ExpiresAt) || r.ExpiresAt.Sub(r.CreatedAt) > 15*time.Minute {
		return ErrAgentMemoryCandidateInvalid
	}
	return nil
}

func (r AgentMemoryPromotionReceiptCommitRequestV1) VerifyInvocation(invocation AgentInvocationV1) error {
	if invocation.Mode != "active" || strings.TrimSpace(invocation.RuntimeID) != "dipole-agent" ||
		anyBlank(invocation.TenantID, invocation.PrincipalUUID, invocation.AgentUUID) {
		return ErrAgentExecutionPolicyDenied
	}
	body := map[string]string{
		"schemaVersion":       AgentMemoryPromotionReceiptSchemaV2,
		"status":              AgentMemoryPromotionReceiptPrepared,
		"tenantId":            invocation.TenantID,
		"principalUserId":     invocation.PrincipalUUID,
		"agentId":             invocation.AgentUUID,
		"taskId":              r.TaskUUID,
		"runId":               r.RunUUID,
		"candidateId":         r.CandidateUUID,
		"candidateSha256":     r.CandidateSHA256,
		"reviewId":            r.ReviewUUID,
		"policyVersion":       r.PolicyVersion,
		"candidateMemoryType": string(AgentMemoryTypeObservational),
		"targetMemoryType":    string(r.TargetMemoryType),
		"createdAt":           receiptTimestampV1(r.CreatedAt),
		"expiresAt":           receiptTimestampV1(r.ExpiresAt),
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return ErrAgentMemoryCandidateInvalid
	}
	digest := sha256.Sum256(canonical)
	receiptSHA256 := hex.EncodeToString(digest[:])
	if r.ReceiptSHA256 != receiptSHA256 {
		return ErrAgentMemoryCandidateConflict
	}
	receiptBody := make(map[string]string, len(body)+1)
	for key, value := range body {
		receiptBody[key] = value
	}
	receiptBody["receiptSha256"] = receiptSHA256
	receiptCanonical, err := json.Marshal(receiptBody)
	if err != nil {
		return ErrAgentMemoryCandidateInvalid
	}
	receiptDigest := sha256.Sum256(receiptCanonical)
	if r.ReceiptID != fmt.Sprintf("MEM-PROMOTE-%x", receiptDigest) {
		return ErrAgentMemoryCandidateConflict
	}
	return nil
}

// receiptTimestampV1 matches JavaScript Date#toISOString, which is the v2
// receipt producer. Inputs are restricted to millisecond precision above.
func receiptTimestampV1(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
