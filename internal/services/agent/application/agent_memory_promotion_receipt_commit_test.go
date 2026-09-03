package agentapplication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
)

func TestMemoryPromotionReceiptCommitDerivesOwnerFromActiveInvocation(t *testing.T) {
	now := time.Date(2026, 8, 30, 2, 0, 0, 0, time.UTC)
	invocation := activePromotionInvocation()
	resolver := &receiptInvocationResolver{invocation: invocation}
	promotions := &receiptPromotionService{}
	service, err := NewPersistentAgentMemoryPromotionReceiptCommitServiceV1(resolver, promotions, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	request := validReceiptCommitRequest(invocation, now)
	if request.ReceiptSHA256 != "0155c4d98ae86c7137a2babd593c0850fd93a02994e842865ca146afb2df525b" || request.ReceiptID != "MEM-PROMOTE-337a5d93826c8b2083f23bce81ff1756feeb53403618e2a080c3b13edfc63ed5" {
		t.Fatalf("receipt golden vector drifted: %+v", request)
	}
	memory, err := service.CommitMemoryPromotionReceipt(context.Background(), request)
	if err != nil || memory == nil {
		t.Fatalf("commit memory=%+v err=%v", memory, err)
	}
	if resolver.taskID != request.TaskUUID || resolver.runID != request.RunUUID || promotions.calls != 1 ||
		promotions.request.PrincipalUUID != invocation.PrincipalUUID || promotions.request.TenantID != invocation.TenantID ||
		promotions.request.TargetMemoryType != application.AgentMemoryTypeSemantic {
		t.Fatalf("resolver=%+v promotion=%+v calls=%d", resolver, promotions.request, promotions.calls)
	}
}

func TestMemoryPromotionReceiptCommitFailsClosedBeforePromotion(t *testing.T) {
	now := time.Date(2026, 8, 30, 2, 0, 0, 0, time.UTC)
	invocation := activePromotionInvocation()
	cases := []struct {
		name    string
		request func(application.AgentMemoryPromotionReceiptCommitRequestV1) application.AgentMemoryPromotionReceiptCommitRequestV1
		resolve application.AgentInvocationV1
		want    error
	}{
		{name: "receipt drift", request: func(r application.AgentMemoryPromotionReceiptCommitRequestV1) application.AgentMemoryPromotionReceiptCommitRequestV1 {
			r.CandidateSHA256 = "b" + r.CandidateSHA256[1:]
			return r
		}, resolve: invocation, want: application.ErrAgentMemoryCandidateConflict},
		{name: "working target", request: func(r application.AgentMemoryPromotionReceiptCommitRequestV1) application.AgentMemoryPromotionReceiptCommitRequestV1 {
			r.TargetMemoryType = application.AgentMemoryTypeWorking
			return r
		}, resolve: invocation, want: application.ErrAgentMemoryCandidateInvalid},
		{name: "shadow invocation", request: func(r application.AgentMemoryPromotionReceiptCommitRequestV1) application.AgentMemoryPromotionReceiptCommitRequestV1 {
			return r
		}, resolve: application.AgentInvocationV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", RuntimeID: "dipole-agent", Mode: "shadow"}, want: application.ErrAgentExecutionPolicyDenied},
		{name: "expired receipt", request: func(r application.AgentMemoryPromotionReceiptCommitRequestV1) application.AgentMemoryPromotionReceiptCommitRequestV1 {
			r.ExpiresAt = now
			return r
		}, resolve: invocation, want: application.ErrAgentMemoryCandidateInvalid},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			promotions := &receiptPromotionService{}
			service, _ := NewPersistentAgentMemoryPromotionReceiptCommitServiceV1(&receiptInvocationResolver{invocation: test.resolve}, promotions, func() time.Time { return now })
			_, err := service.CommitMemoryPromotionReceipt(context.Background(), test.request(validReceiptCommitRequest(invocation, now)))
			if !errors.Is(err, test.want) || promotions.calls != 0 {
				t.Fatalf("err=%v calls=%d want=%v", err, promotions.calls, test.want)
			}
		})
	}
}

type receiptInvocationResolver struct {
	invocation    application.AgentInvocationV1
	err           error
	taskID, runID string
}

func (r *receiptInvocationResolver) Resolve(_ context.Context, taskID, runID string) (application.AgentInvocationV1, error) {
	r.taskID, r.runID = taskID, runID
	return r.invocation, r.err
}

type receiptPromotionService struct {
	request application.AgentMemoryCandidatePromotionRequestV1
	calls   int
}

func (s *receiptPromotionService) Promote(_ context.Context, request application.AgentMemoryCandidatePromotionRequestV1) (*application.AgentMemoryV1, error) {
	s.calls++
	s.request = request
	return &application.AgentMemoryV1{MemoryUUID: "MEM-CAND-1", TenantID: request.TenantID, PrincipalUUID: request.PrincipalUUID, AgentUUID: "UAI", MemoryType: request.TargetMemoryType, Status: application.AgentMemoryStatusActive, ResourceType: "conversation", ResourceID: "group:G1", Content: "Decision", CompactContent: "Decision", Priority: 60, Provenance: application.AgentMemoryProvenanceV1{SourceType: "memory_candidate", SourceID: request.CandidateUUID, Sequence: request.ReviewUUID}, ValidFrom: time.Unix(1, 0).UTC()}, nil
}

func (s *receiptPromotionService) Review(context.Context, application.AgentMemoryCandidateReviewRequestV1) (*application.AgentMemoryCandidateCatalogItemV1, error) {
	return nil, application.ErrAgentMemoryCandidateInvalid
}

func activePromotionInvocation() application.AgentInvocationV1 {
	return application.AgentInvocationV1{TenantID: "dipole", PrincipalUUID: "U100", AgentUUID: "UAI", RuntimeID: "dipole-agent", Mode: "active"}
}

func validReceiptCommitRequest(invocation application.AgentInvocationV1, now time.Time) application.AgentMemoryPromotionReceiptCommitRequestV1 {
	createdAt, expiresAt := now.Add(-time.Minute), now.Add(10*time.Minute)
	request := application.AgentMemoryPromotionReceiptCommitRequestV1{SchemaVersion: application.AgentMemoryPromotionReceiptSchemaV2, Status: application.AgentMemoryPromotionReceiptPrepared, TaskUUID: "TASK-1", RunUUID: "RUN-1", CandidateUUID: "CAND-1", CandidateSHA256: strings.Repeat("a", 64), ReviewUUID: "REV-1", PolicyVersion: "memory-v1", TargetMemoryType: application.AgentMemoryTypeSemantic, CreatedAt: createdAt, ExpiresAt: expiresAt}
	body := map[string]string{"schemaVersion": request.SchemaVersion, "status": request.Status, "tenantId": invocation.TenantID, "principalUserId": invocation.PrincipalUUID, "agentId": invocation.AgentUUID, "taskId": request.TaskUUID, "runId": request.RunUUID, "candidateId": request.CandidateUUID, "candidateSha256": request.CandidateSHA256, "reviewId": request.ReviewUUID, "policyVersion": request.PolicyVersion, "candidateMemoryType": "observational", "targetMemoryType": string(request.TargetMemoryType), "createdAt": createdAt.Format("2006-01-02T15:04:05.000Z07:00"), "expiresAt": expiresAt.Format("2006-01-02T15:04:05.000Z07:00")}
	encoded, _ := json.Marshal(body)
	digest := sha256.Sum256(encoded)
	request.ReceiptSHA256 = hex.EncodeToString(digest[:])
	body["receiptSha256"] = request.ReceiptSHA256
	encoded, _ = json.Marshal(body)
	digest = sha256.Sum256(encoded)
	request.ReceiptID = fmt.Sprintf("MEM-PROMOTE-%x", digest)
	return request
}
