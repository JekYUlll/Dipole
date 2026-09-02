package agentmysql_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	agentapplication "github.com/JekYUlll/Dipole/internal/services/agent/application"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/agent/infrastructure/mysql"
	corepolicy "github.com/JekYUlll/Dipole/internal/services/core/rpcpolicy"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	grpccommon "github.com/JekYUlll/Dipole/internal/transport/grpc/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAgentMemoryPromotionReceiptCommitMySQLContract(t *testing.T) {
	ctx := context.Background()
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}

	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create transaction store: %v", err)
	}
	policy, err := sqlcRepository.NewAgentPolicyRepositoryWithTransactions(store)
	if err != nil {
		t.Fatalf("create policy repository: %v", err)
	}
	memories, err := sqlcRepository.NewAgentMemoryRepositoryWithTransactions(store)
	if err != nil {
		t.Fatalf("create Memory repository: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	clock := now
	definition, grant := createReceiptContractPolicy(t, ctx, policy, now)
	authorizer, err := agentapplication.NewPersistentAgentActiveRunPromotionAuthorizerV1(policy)
	if err != nil {
		t.Fatalf("create promotion authorizer: %v", err)
	}
	admission, err := agentapplication.NewPersistentAgentRunAdmissionV1WithClock(policy, func() time.Time { return clock }, authorizer)
	if err != nil {
		t.Fatalf("create Run admission: %v", err)
	}
	admitted, err := admission.Admit(ctx, application.AgentRunAdmissionRequestV1{
		AgentExecutionPolicyStartV1: application.AgentExecutionPolicyStartV1{
			TenantID: definition.TenantID, PrincipalUUID: definition.OwnerUUID, AgentUUID: definition.AgentUUID,
			DelegatedByUUID: definition.OwnerUUID, TriggerType: "manual", TriggerRef: "receipt-mysql-contract",
		},
		RuntimeID: grant.RuntimeID, Mode: "active", CandidateVersion: grant.CandidateVersion,
	})
	if err != nil {
		t.Fatalf("admit active Run: %v", err)
	}

	candidateID, reviewID := "CAND-RECEIPT-MYSQL", "REVIEW-RECEIPT-MYSQL"
	candidateSHA256 := strings.Repeat("c", 64)
	insertAcceptedMemoryCandidate(t, ctx, db, candidateID, reviewID, candidateSHA256, now)

	resolver, err := agentapplication.NewPersistentAgentInvocationResolverV1WithClock(policy, func() time.Time { return clock }, authorizer)
	if err != nil {
		t.Fatalf("create Invocation resolver: %v", err)
	}
	promotions, err := agentapplication.NewPersistentAgentMemoryCandidatePromotionServiceV1(memories, func() time.Time { return clock })
	if err != nil {
		t.Fatalf("create Memory promotion service: %v", err)
	}
	commits, err := agentapplication.NewPersistentAgentMemoryPromotionReceiptCommitServiceV1(resolver, promotions, func() time.Time { return clock })
	if err != nil {
		t.Fatalf("create receipt commit service: %v", err)
	}

	invocation, err := resolver.Resolve(ctx, admitted.TaskUUID, admitted.RunUUID)
	if err != nil {
		t.Fatalf("resolve persisted invocation: %v", err)
	}
	receipt := receiptCommitRequest(t, invocation, admitted.TaskUUID, admitted.RunUUID, candidateID, candidateSHA256, reviewID, now)
	server, err := agentgrpc.NewMemoryPromotionReceiptServer(commits)
	if err != nil {
		t.Fatalf("create Core receipt adapter: %v", err)
	}
	certs := generateReceiptContractCertificates(t)
	rpcServer := startReceiptContractRPCServer(t, certs, server)
	connection := dialReceiptContractRPCClient(t, ctx, certs, rpcServer.Address())
	t.Cleanup(func() { _ = connection.Close() })
	client := agentv1.NewAgentCapabilityServiceClient(connection)
	request := receiptCommitRPCRequest(receipt)
	firstResponse, err := client.CommitMemoryPromotionReceipt(ctx, request)
	if err != nil {
		t.Fatalf("commit receipt through Core adapter: %v", err)
	}
	if firstResponse.GetMemoryType() != string(application.AgentMemoryTypeSemantic) || firstResponse.GetStatus() != string(application.AgentMemoryStatusActive) {
		t.Fatalf("committed receipt response=%+v", firstResponse)
	}
	assertReceiptPromotionStored(t, ctx, db, candidateID, firstResponse.GetMemoryId(), 1)

	clock = clock.Add(time.Second)
	replayed, err := client.CommitMemoryPromotionReceipt(ctx, request)
	if err != nil {
		t.Fatalf("replay receipt through Core adapter: %v", err)
	}
	if replayed.GetMemoryId() != firstResponse.GetMemoryId() || replayed.GetReceiptSha256() != request.GetReceiptSha256() {
		t.Fatalf("replayed receipt response=%+v, want Memory=%q receipt=%q", replayed, firstResponse.GetMemoryId(), request.GetReceiptSha256())
	}
	assertReceiptPromotionStored(t, ctx, db, candidateID, firstResponse.GetMemoryId(), 1)

	if revoked, revokeErr := policy.RevokeRuntimePromotionGrant(ctx, grant.GrantUUID, now); revokeErr != nil || !revoked {
		t.Fatalf("revoke promotion grant: revoked=%v err=%v", revoked, revokeErr)
	}
	if _, err := client.CommitMemoryPromotionReceipt(ctx, request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("revoked grant receipt code=%s err=%v, want permission denied", status.Code(err), err)
	}
	assertReceiptPromotionStored(t, ctx, db, candidateID, firstResponse.GetMemoryId(), 1)
}

func receiptCommitRPCRequest(receipt application.AgentMemoryPromotionReceiptCommitRequestV1) *agentv1.CommitMemoryPromotionReceiptRequest {
	return &agentv1.CommitMemoryPromotionReceiptRequest{
		Context: grpccommon.RequestContext("", "dipole-agent"), ReceiptId: receipt.ReceiptID, ReceiptSha256: receipt.ReceiptSHA256,
		SchemaVersion: receipt.SchemaVersion, Status: receipt.Status, TaskId: receipt.TaskUUID, RunId: receipt.RunUUID,
		CandidateId: receipt.CandidateUUID, CandidateSha256: receipt.CandidateSHA256, ReviewId: receipt.ReviewUUID,
		PolicyVersion: receipt.PolicyVersion, TargetMemoryType: string(receipt.TargetMemoryType),
		CreatedAtUnixMs: receipt.CreatedAt.UnixMilli(), ExpiresAtUnixMs: receipt.ExpiresAt.UnixMilli(),
	}
}

type receiptContractCertificates struct {
	ca, coreCert, coreKey, agentCert, agentKey string
}

func generateReceiptContractCertificates(t *testing.T) receiptContractCertificates {
	t.Helper()
	directory := t.TempDir()
	command := exec.Command("bash", filepath.Join("..", "..", "..", "..", "..", "scripts", "generate-internal-certs.sh"))
	command.Env = append(os.Environ(), "INTERNAL_CERT_DIR="+directory, "INTERNAL_CERT_VALID_DAYS=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate internal certificates: %v: %s", err, output)
	}
	return receiptContractCertificates{
		ca: filepath.Join(directory, "ca.pem"), coreCert: filepath.Join(directory, "core.pem"), coreKey: filepath.Join(directory, "core-key.pem"),
		agentCert: filepath.Join(directory, "agent.pem"), agentKey: filepath.Join(directory, "agent-key.pem"),
	}
}

func startReceiptContractRPCServer(t *testing.T, certs receiptContractCertificates, adapter *agentgrpc.MemoryPromotionReceiptServer) *platformrpc.Server {
	t.Helper()
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "receipt-mysql-contract-secret", CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2,
		TLSEnabled: true, TLSCertFile: certs.coreCert, TLSKeyFile: certs.coreKey, TLSCAFile: certs.ca, TLSServerName: "core"}
	server, err := platformrpc.NewServer(cfg, cfg.CoreListenAddress, []string{"dipole-agent"}, func(server *grpc.Server) {
		agentv1.RegisterAgentCapabilityServiceServer(server, adapter)
	}, corepolicy.RestrictAgentServiceMethods)
	if err != nil {
		t.Fatalf("start receipt contract Core RPC server: %v", err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	return server
}

func dialReceiptContractRPCClient(t *testing.T, ctx context.Context, certs receiptContractCertificates, address string) *grpc.ClientConn {
	t.Helper()
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "receipt-mysql-contract-secret", DialTimeoutSeconds: 2,
		TLSEnabled: true, TLSCertFile: certs.agentCert, TLSKeyFile: certs.agentKey, TLSCAFile: certs.ca, TLSServerName: "core"}
	connection, err := platformrpc.Dial(ctx, cfg, address, grpcauth.Credentials{Service: "dipole-agent", Secret: cfg.SharedSecret})
	if err != nil {
		t.Fatalf("dial receipt contract Core RPC server: %v", err)
	}
	return connection
}

func createReceiptContractPolicy(t *testing.T, ctx context.Context, policy *sqlcRepository.AgentPolicyRepository, now time.Time) (application.AgentDefinitionVersionV1, application.AgentRuntimePromotionGrantV1) {
	t.Helper()
	definition := application.AgentDefinitionVersionV1{
		DefinitionUUID: "DEF-RECEIPT-MYSQL", Version: 1, TenantID: "dipole", OwnerUUID: "U100", AgentUUID: "UAI",
		Status: application.AgentDefinitionStatusActive, Permissions: []string{application.AgentPermissionMessageWrite},
		Scopes:    []application.AgentResourceScopeV1{{ResourceType: "conversation", ResourceID: "*", Actions: []string{"write"}}},
		ValidFrom: now.Add(-time.Hour),
	}
	if err := policy.CreateDefinitionVersion(ctx, definition); err != nil {
		t.Fatalf("create Definition: %v", err)
	}
	grant := application.AgentRuntimePromotionGrantV1{
		GrantUUID: "PROMOTION-RECEIPT-MYSQL", TenantID: definition.TenantID, RuntimeID: "dipole-agent", CandidateVersion: "receipt-mysql-v1",
		DefinitionUUID: definition.DefinitionUUID, DefinitionVersion: definition.Version, PolicyVersion: application.AgentRuntimePromotionPolicyVersionV2,
		EvidenceSHA256: strings.Repeat("a", 64), EvalSuiteSHA256: strings.Repeat("b", 64), GrantedByUUID: "OPERATOR-1", ReviewedByUUID: "OPERATOR-2",
		ValidFrom: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	created, err := policy.CreateRuntimePromotionGrant(ctx, grant)
	if err != nil || !created {
		t.Fatalf("create promotion grant: created=%v err=%v", created, err)
	}
	return definition, grant
}

func insertAcceptedMemoryCandidate(t *testing.T, ctx context.Context, db *sql.DB, candidateID, reviewID, candidateSHA256 string, now time.Time) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_memory_candidates
        (candidate_uuid, tenant_id, principal_uuid, agent_uuid, resource_type, resource_id, candidate_type, source_id, evidence_ids_json, summary, policy_version, candidate_sha256, status, observed_at)
        VALUES (?, 'dipole', 'U100', 'UAI', 'conversation', 'group:G1', 'message', 'MESSAGE-RECEIPT-MYSQL', ?, 'reviewed project decision', 'memory-v1', ?, 'accepted', ?)`,
		candidateID, `["MESSAGE-RECEIPT-MYSQL"]`, candidateSHA256, now.Add(-time.Minute)); err != nil {
		t.Fatalf("insert accepted candidate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO agent_memory_candidate_reviews
        (review_uuid, candidate_uuid, candidate_sha256, reviewer_uuid, decision, reason, review_sha256, reviewed_at)
        VALUES (?, ?, ?, 'U100', 'accepted', 'owner reviewed', ?, ?)`,
		reviewID, candidateID, candidateSHA256, strings.Repeat("d", 64), now); err != nil {
		t.Fatalf("insert accepted review: %v", err)
	}
}

func receiptCommitRequest(t *testing.T, invocation application.AgentInvocationV1, taskID, runID, candidateID, candidateSHA256, reviewID string, now time.Time) application.AgentMemoryPromotionReceiptCommitRequestV1 {
	t.Helper()
	request := application.AgentMemoryPromotionReceiptCommitRequestV1{
		SchemaVersion: application.AgentMemoryPromotionReceiptSchemaV2, Status: application.AgentMemoryPromotionReceiptPrepared,
		TaskUUID: taskID, RunUUID: runID, CandidateUUID: candidateID, CandidateSHA256: candidateSHA256, ReviewUUID: reviewID,
		PolicyVersion: "memory-v1", TargetMemoryType: application.AgentMemoryTypeSemantic,
		CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(10 * time.Minute),
	}
	body := map[string]string{
		"schemaVersion": request.SchemaVersion, "status": request.Status, "tenantId": invocation.TenantID,
		"principalUserId": invocation.PrincipalUUID, "agentId": invocation.AgentUUID, "taskId": request.TaskUUID, "runId": request.RunUUID,
		"candidateId": request.CandidateUUID, "candidateSha256": request.CandidateSHA256, "reviewId": request.ReviewUUID,
		"policyVersion": request.PolicyVersion, "candidateMemoryType": string(application.AgentMemoryTypeObservational),
		"targetMemoryType": string(request.TargetMemoryType), "createdAt": receiptContractTimestamp(request.CreatedAt), "expiresAt": receiptContractTimestamp(request.ExpiresAt),
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal receipt body: %v", err)
	}
	digest := sha256.Sum256(canonical)
	request.ReceiptSHA256 = hex.EncodeToString(digest[:])
	body["receiptSha256"] = request.ReceiptSHA256
	canonical, err = json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal receipt ID body: %v", err)
	}
	digest = sha256.Sum256(canonical)
	request.ReceiptID = fmt.Sprintf("MEM-PROMOTE-%x", digest)
	return request
}

func receiptContractTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func assertReceiptPromotionStored(t *testing.T, ctx context.Context, db *sql.DB, candidateID, memoryID string, wantMemoryRows int) {
	t.Helper()
	var memoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_memories WHERE memory_uuid = ?`, memoryID).Scan(&memoryRows); err != nil || memoryRows != wantMemoryRows {
		t.Fatalf("stored Memory rows=%d err=%v, want %d", memoryRows, err, wantMemoryRows)
	}
	var promoted sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT promoted_memory_uuid FROM agent_memory_candidates WHERE candidate_uuid = ?`, candidateID).Scan(&promoted); err != nil || !promoted.Valid || promoted.String != memoryID {
		t.Fatalf("promoted candidate Memory=%q valid=%v err=%v, want %q", promoted.String, promoted.Valid, err, memoryID)
	}
}
