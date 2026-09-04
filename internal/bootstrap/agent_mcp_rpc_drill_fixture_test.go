package bootstrap

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	corepolicy "github.com/JekYUlll/Dipole/internal/services/core/rpcpolicy"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcCredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const agentMCPRPCDrillSecret = "agent-mcp-rpc-drill-secret"

type agentMCPRPCDrillFixture struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
	mu                         sync.Mutex
	commands                   map[string]*agentv1.ResolveMcpToolCommandResponse
	rounds                     map[string]*agentv1.FinishMcpToolRoundRequest
	approvals                  map[string]*agentMCPRPCDrillApproval
	finishedStatuses           []string
	artifactCount              int
	memoryPromotionCommitCount int
	rpcCallCount               int
	statePath                  string
	stalePath                  string
}

type agentMCPRPCDrillApproval struct {
	request  *agentv1.RequestApprovalRequest
	status   string
	consumed bool
}

type agentMCPRPCDrillState struct {
	SchemaVersion              string   `json:"schema_version"`
	RPCType                    string   `json:"rpc_type"`
	RPCAuthenticated           bool     `json:"rpc_authenticated"`
	RPCCallCount               int      `json:"rpc_call_count"`
	ArtifactCount              int      `json:"artifact_count"`
	MemoryPromotionCommitCount int      `json:"memory_promotion_commit_count"`
	FinishedStatuses           []string `json:"finished_statuses"`
}

func TestAgentMCPRPCDrillFixtureAuthentication(t *testing.T) {
	certs := generateAgentMCPRPCDrillCertificates(t)
	fixture := newAgentMCPRPCDrillFixture("", "")
	server := startAgentMCPRPCDrillServer(t, certs, fixture)

	valid := dialAgentMCPRPCDrillClient(t, server.Address(), certs.ca, certs.agentCert, certs.agentKey, agentMCPRPCDrillSecret, agentServiceName)
	if response, err := valid.MatchEventSubscriptions(context.Background(), drillMatchRequest()); err != nil || len(response.GetSubscriptions()) != 1 || response.GetSubscriptions()[0].GetCreatedById() != "U100" {
		t.Fatalf("valid mTLS Agent call response=%+v err=%v", response, err)
	}
	admitRequest := &agentv1.AdmitRunRequest{
		TenantId: "dipole", PrincipalUserId: "U100", AgentId: "UAI-DRILL", TriggerType: "message.direct.created",
		TriggerRef: "MESSAGE-MCP-DRILL-1", EventId: "EVENT-MCP-DRILL-1", RuntimeId: agentServiceName, Mode: "shadow", SubscriptionId: " SUB-MCP-DRILL ",
	}
	if response, err := valid.AdmitRun(context.Background(), admitRequest); err != nil || response.GetTaskId() != drillIdentifier("task:", 59,
		"dipole.agent.policy.persistence.v1", "dipole", "UAI-DRILL", "message.direct.created", "MESSAGE-MCP-DRILL-1", "SUB-MCP-DRILL") {
		t.Fatalf("subscription-scoped mTLS Agent admission response=%+v err=%v", response, err)
	}
	if response, err := valid.CommitMemoryPromotionReceipt(context.Background(), drillMemoryPromotionCommitRequest()); err != nil || response.GetMemoryId() != "MEM-COMMIT-CAND-1" || response.GetMemoryType() != "semantic" {
		t.Fatalf("valid mTLS receipt commit response=%+v err=%v", response, err)
	}
	approvalRequest := drillApprovalRequest()
	if response, err := valid.RequestApproval(context.Background(), approvalRequest); err != nil || response.GetStatus() != "pending" {
		t.Fatalf("valid mTLS Approval request response=%+v err=%v", response, err)
	}
	if response, err := valid.ResolveApproval(context.Background(), &agentv1.ResolveApprovalRequest{
		TaskId: approvalRequest.GetTaskId(), RunId: approvalRequest.GetRunId(), ApprovalId: approvalRequest.GetApprovalId(), ActorUserId: "U100", Decision: "approved",
	}); err != nil || response.GetStatus() != "approved" {
		t.Fatalf("valid mTLS Approval resolution response=%+v err=%v", response, err)
	}
	grant, err := valid.ResolveApprovalGrant(context.Background(), &agentv1.ResolveApprovalGrantRequest{
		TaskId: approvalRequest.GetTaskId(), RunId: approvalRequest.GetRunId(), CapabilityId: approvalRequest.GetCapabilityId(),
		ResourceScope: approvalRequest.GetResourceScope(), ArgumentsSha256: approvalRequest.GetArgumentsSha256(),
	})
	if err != nil || grant.GetApprovalId() != approvalRequest.GetApprovalId() || grant.GetNonceSha256() != approvalRequest.GetNonceSha256() {
		t.Fatalf("valid mTLS Approval grant response=%+v err=%v", grant, err)
	}
	if response, err := valid.ConsumeApproval(context.Background(), &agentv1.ConsumeApprovalRequest{
		TaskId: approvalRequest.GetTaskId(), RunId: approvalRequest.GetRunId(), ApprovalId: approvalRequest.GetApprovalId(), CapabilityId: approvalRequest.GetCapabilityId(),
		ScopeSha256: approvalRequest.GetScopeSha256(), ArgumentsSha256: approvalRequest.GetArgumentsSha256(), NonceSha256: approvalRequest.GetNonceSha256(), Mode: "active",
	}); err != nil || response.GetStatus() != "consumed" {
		t.Fatalf("valid mTLS Approval consume response=%+v err=%v", response, err)
	}

	wrongSecret := dialAgentMCPRPCDrillClient(t, server.Address(), certs.ca, certs.agentCert, certs.agentKey, "wrong-secret", agentServiceName)
	if _, err := wrongSecret.MatchEventSubscriptions(context.Background(), drillMatchRequest()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong secret code=%s", status.Code(err))
	}
	if _, err := wrongSecret.CommitMemoryPromotionReceipt(context.Background(), drillMemoryPromotionCommitRequest()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong secret receipt commit code=%s", status.Code(err))
	}
	if _, err := wrongSecret.ResolveApprovalGrant(context.Background(), &agentv1.ResolveApprovalGrantRequest{
		TaskId: approvalRequest.GetTaskId(), RunId: approvalRequest.GetRunId(), CapabilityId: approvalRequest.GetCapabilityId(),
		ResourceScope: approvalRequest.GetResourceScope(), ArgumentsSha256: approvalRequest.GetArgumentsSha256(),
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong secret Approval grant code=%s", status.Code(err))
	}

	wrongIdentity := dialAgentMCPRPCDrillClient(t, server.Address(), certs.ca, certs.messageCert, certs.messageKey, agentMCPRPCDrillSecret, agentServiceName)
	if _, err := wrongIdentity.MatchEventSubscriptions(context.Background(), drillMatchRequest()); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong certificate identity code=%s", status.Code(err))
	}
	if _, err := wrongIdentity.CommitMemoryPromotionReceipt(context.Background(), drillMemoryPromotionCommitRequest()); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong certificate receipt commit code=%s", status.Code(err))
	}
	if _, err := wrongIdentity.ConsumeApproval(context.Background(), &agentv1.ConsumeApprovalRequest{
		TaskId: approvalRequest.GetTaskId(), RunId: approvalRequest.GetRunId(), ApprovalId: approvalRequest.GetApprovalId(), CapabilityId: approvalRequest.GetCapabilityId(),
		ScopeSha256: approvalRequest.GetScopeSha256(), ArgumentsSha256: approvalRequest.GetArgumentsSha256(), NonceSha256: approvalRequest.GetNonceSha256(), Mode: "active",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong certificate Approval consume code=%s", status.Code(err))
	}

	withoutCertificate := dialAgentMCPRPCDrillClient(t, server.Address(), certs.ca, "", "", agentMCPRPCDrillSecret, agentServiceName)
	if _, err := withoutCertificate.MatchEventSubscriptions(context.Background(), drillMatchRequest()); err == nil {
		t.Fatal("missing client certificate unexpectedly reached the fixture")
	}
}

func TestAgentMCPRPCDrillFixtureProcess(t *testing.T) {
	if os.Getenv("DIPOLE_AGENT_RPC_DRILL_FIXTURE") != "true" {
		t.Skip("external Agent RPC drill fixture is disabled")
	}
	readyPath := requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_READY")
	stopPath := requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_STOP")
	statePath := requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_STATE")
	stalePath := requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_STALE")
	fixture := newAgentMCPRPCDrillFixture(statePath, stalePath)
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_SECRET"),
		CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2, TLSEnabled: true,
		TLSCertFile: requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_SERVER_CERT"),
		TLSKeyFile:  requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_SERVER_KEY"),
		TLSCAFile:   requiredAgentMCPRPCDrillEnv(t, "DIPOLE_AGENT_RPC_DRILL_CA"), TLSServerName: "core",
	}
	server, err := platformrpc.NewServer(cfg, cfg.CoreListenAddress, []string{agentServiceName}, func(server *grpc.Server) {
		agentv1.RegisterAgentCapabilityServiceServer(server, fixture)
	}, corepolicy.RestrictAgentServiceMethods)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	fixture.writeState()
	writeAgentMCPRPCDrillJSON(t, readyPath, map[string]string{"address": server.Address()})

	deadline := time.Now().Add(3 * time.Minute)
	for {
		if _, err := os.Stat(stopPath); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("Agent RPC drill fixture timed out")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func newAgentMCPRPCDrillFixture(statePath, stalePath string) *agentMCPRPCDrillFixture {
	return &agentMCPRPCDrillFixture{
		commands:  make(map[string]*agentv1.ResolveMcpToolCommandResponse),
		rounds:    make(map[string]*agentv1.FinishMcpToolRoundRequest),
		approvals: make(map[string]*agentMCPRPCDrillApproval),
		statePath: statePath, stalePath: stalePath,
	}
}

func (f *agentMCPRPCDrillFixture) RequestApproval(_ context.Context, request *agentv1.RequestApprovalRequest) (*agentv1.ApprovalResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if request.GetApprovalId() == "" || request.GetTaskId() == "" || request.GetRunId() == "" || request.GetCapabilityId() == "" || request.GetResourceScope() == nil {
		return nil, status.Error(codes.InvalidArgument, "drill Approval request is invalid")
	}
	if _, exists := f.approvals[request.GetApprovalId()]; !exists {
		f.approvals[request.GetApprovalId()] = &agentMCPRPCDrillApproval{request: proto.Clone(request).(*agentv1.RequestApprovalRequest), status: "pending"}
	}
	f.rpcCallCount++
	f.writeStateLocked()
	return &agentv1.ApprovalResponse{ApprovalId: request.GetApprovalId(), Status: f.approvals[request.GetApprovalId()].status}, nil
}

func (f *agentMCPRPCDrillFixture) ResolveApproval(_ context.Context, request *agentv1.ResolveApprovalRequest) (*agentv1.ApprovalResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	approval := f.approvals[request.GetApprovalId()]
	if approval == nil || approval.request.GetTaskId() != request.GetTaskId() || approval.request.GetRunId() != request.GetRunId() {
		return nil, status.Error(codes.NotFound, "drill Approval is unavailable")
	}
	switch request.GetDecision() {
	case "approved":
		approval.status = "approved"
	case "denied":
		approval.status = "revoked"
	default:
		return nil, status.Error(codes.InvalidArgument, "drill Approval decision is invalid")
	}
	f.rpcCallCount++
	f.writeStateLocked()
	return &agentv1.ApprovalResponse{ApprovalId: request.GetApprovalId(), Status: approval.status, ApprovedByUserId: request.GetActorUserId()}, nil
}

func (f *agentMCPRPCDrillFixture) ResolveApprovalGrant(_ context.Context, request *agentv1.ResolveApprovalGrantRequest) (*agentv1.ResolveApprovalGrantResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, approval := range f.approvals {
		stored := approval.request
		if approval.status != "approved" || approval.consumed || stored.GetTaskId() != request.GetTaskId() || stored.GetRunId() != request.GetRunId() ||
			stored.GetCapabilityId() != request.GetCapabilityId() || stored.GetScopeSha256() == "" ||
			stored.GetArgumentsSha256() != request.GetArgumentsSha256() || !proto.Equal(stored.GetResourceScope(), request.GetResourceScope()) {
			continue
		}
		f.rpcCallCount++
		f.writeStateLocked()
		return &agentv1.ResolveApprovalGrantResponse{ApprovalId: stored.GetApprovalId(), CapabilityId: stored.GetCapabilityId(), ResourceScope: proto.Clone(stored.GetResourceScope()).(*agentv1.AgentResourceScope),
			ScopeSha256: stored.GetScopeSha256(), ArgumentsSha256: stored.GetArgumentsSha256(), NonceSha256: stored.GetNonceSha256(), ExpiresAtUnixMs: stored.GetExpiresAtUnixMs()}, nil
	}
	return nil, status.Error(codes.NotFound, "drill Approval grant is unavailable")
}

func (f *agentMCPRPCDrillFixture) ConsumeApproval(_ context.Context, request *agentv1.ConsumeApprovalRequest) (*agentv1.ConsumeApprovalResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	approval := f.approvals[request.GetApprovalId()]
	if approval == nil || approval.status != "approved" || approval.consumed || approval.request.GetTaskId() != request.GetTaskId() || approval.request.GetRunId() != request.GetRunId() ||
		approval.request.GetCapabilityId() != request.GetCapabilityId() || approval.request.GetScopeSha256() != request.GetScopeSha256() ||
		approval.request.GetArgumentsSha256() != request.GetArgumentsSha256() || approval.request.GetNonceSha256() != request.GetNonceSha256() || request.GetMode() != "active" {
		return nil, status.Error(codes.PermissionDenied, "drill Approval consumption is denied")
	}
	approval.consumed = true
	f.rpcCallCount++
	f.writeStateLocked()
	return &agentv1.ConsumeApprovalResponse{ApprovalId: request.GetApprovalId(), Status: "consumed"}, nil
}

func (f *agentMCPRPCDrillFixture) MatchEventSubscriptions(_ context.Context, request *agentv1.MatchEventSubscriptionsRequest) (*agentv1.MatchEventSubscriptionsResponse, error) {
	f.recordCall()
	return &agentv1.MatchEventSubscriptionsResponse{Subscriptions: []*agentv1.AgentEventSubscription{{
		SubscriptionId: "SUB-MCP-DRILL", DefinitionId: "DEF-REPOSITORY-GUARDIAN", DefinitionVersion: 1,
		TenantId: request.GetTenantId(), AgentId: request.GetAgentId(), CreatedById: "U100", EventType: request.GetEventType(),
		ResourceType: request.GetResourceType(), ResourceId: request.GetResourceId(), FilterKind: "all", FilterJson: []byte(`{}`), Status: "active",
	}}}, nil
}

func (f *agentMCPRPCDrillFixture) AdmitRun(_ context.Context, request *agentv1.AdmitRunRequest) (*agentv1.AdmitRunResponse, error) {
	f.recordCall()
	taskParts := []string{"dipole.agent.policy.persistence.v1", request.GetTenantId(), request.GetAgentId(), request.GetTriggerType(), request.GetTriggerRef()}
	if subscriptionID := strings.TrimSpace(request.GetSubscriptionId()); subscriptionID != "" {
		taskParts = append(taskParts, subscriptionID)
	}
	taskID := drillIdentifier("task:", 59, taskParts...)
	runID := drillIdentifier("run:", 60, "dipole.agent.run.v1", taskID, request.GetRuntimeId(), request.GetMode())
	return &agentv1.AdmitRunResponse{TaskId: taskID, RunId: runID, RunStatus: "running"}, nil
}

func (f *agentMCPRPCDrillFixture) FinishRun(_ context.Context, request *agentv1.FinishRunRequest) (*agentv1.FinishRunResponse, error) {
	f.mu.Lock()
	f.rpcCallCount++
	f.finishedStatuses = append(f.finishedStatuses, request.GetRunStatus())
	f.writeStateLocked()
	f.mu.Unlock()
	return &agentv1.FinishRunResponse{RunStatus: request.GetRunStatus()}, nil
}

func (f *agentMCPRPCDrillFixture) ProjectTaskWorkflowState(_ context.Context, request *agentv1.ProjectTaskWorkflowStateRequest) (*agentv1.ProjectTaskWorkflowStateResponse, error) {
	f.recordCall()
	return &agentv1.ProjectTaskWorkflowStateResponse{TaskId: request.GetTaskId(), WorkflowId: request.GetWorkflowId(),
		WorkflowRunId: request.GetWorkflowRunId(), WorkflowStatus: request.GetWorkflowStatus(), WorkflowRevision: request.GetWorkflowRevision()}, nil
}

func (f *agentMCPRPCDrillFixture) ResolveMcpContext(_ context.Context, request *agentv1.ResolveMcpContextRequest) (*agentv1.ResolveMcpContextResponse, error) {
	f.recordCall()
	return &agentv1.ResolveMcpContextResponse{
		TenantId: "dipole", PrincipalUserId: request.GetPrincipalUserId(), AgentId: "UAI-DRILL", RuntimeId: agentServiceName, Mode: "shadow",
		Permissions: []string{"repository.issue.read"}, ResourceScopes: []*agentv1.AgentResourceScope{{
			ResourceType: "repository_issue", ResourceId: "dipole/dipole#1", Actions: []string{"read"},
		}},
	}, nil
}

func (f *agentMCPRPCDrillFixture) BeginMcpToolInvocation(_ context.Context, request *agentv1.BeginMcpToolInvocationRequest) (*agentv1.BeginMcpToolInvocationResponse, error) {
	f.mu.Lock()
	f.rpcCallCount++
	if _, exists := f.commands[request.GetInvocationId()]; !exists {
		f.commands[request.GetInvocationId()] = &agentv1.ResolveMcpToolCommandResponse{
			InvocationId: request.GetInvocationId(), TenantId: "dipole", PrincipalUserId: "U100", AgentId: "UAI-DRILL",
			TaskId: request.GetTaskId(), RunId: request.GetRunId(), ProfileId: request.GetProfileId(), ServerId: request.GetServerId(),
			ToolName: request.GetToolName(), CapabilityId: request.GetCapabilityId(), ArgumentsJson: append([]byte(nil), request.GetArgumentsJson()...),
			ArgumentsSha256: request.GetArgumentsSha256(), StartedAtUnixMs: time.Now().UnixMilli(), Status: "running",
		}
	}
	statusValue := f.commands[request.GetInvocationId()].GetStatus()
	f.writeStateLocked()
	f.mu.Unlock()
	return &agentv1.BeginMcpToolInvocationResponse{InvocationId: request.GetInvocationId(), Status: statusValue}, nil
}

func (f *agentMCPRPCDrillFixture) ResolveMcpToolCommand(_ context.Context, request *agentv1.ResolveMcpToolCommandRequest) (*agentv1.ResolveMcpToolCommandResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rpcCallCount++
	command := f.commands[request.GetInvocationId()]
	if command == nil || command.GetTaskId() != request.GetTaskId() || command.GetRunId() != request.GetRunId() {
		return nil, status.Error(codes.NotFound, "drill command is unavailable")
	}
	copy := proto.Clone(command).(*agentv1.ResolveMcpToolCommandResponse)
	f.writeStateLocked()
	return copy, nil
}

func (f *agentMCPRPCDrillFixture) ClaimMcpToolRound(_ context.Context, request *agentv1.ClaimMcpToolRoundRequest) (*agentv1.ClaimMcpToolRoundResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rpcCallCount++
	receipt := f.rounds[request.GetRoundId()]
	f.writeStateLocked()
	if receipt == nil {
		return &agentv1.ClaimMcpToolRoundResponse{RoundId: request.GetRoundId(), Outcome: "claimed"}, nil
	}
	if receipt.GetStatus() == "failed" {
		return &agentv1.ClaimMcpToolRoundResponse{RoundId: request.GetRoundId(), Outcome: "replay_failed", ErrorCode: receipt.GetErrorCode()}, nil
	}
	return &agentv1.ClaimMcpToolRoundResponse{RoundId: request.GetRoundId(), Outcome: "replay_completed",
		ResultJson: append([]byte(nil), receipt.GetResultJson()...), ResultSha256: receipt.GetResultSha256()}, nil
}

func (f *agentMCPRPCDrillFixture) FinishMcpToolRound(_ context.Context, request *agentv1.FinishMcpToolRoundRequest) (*agentv1.FinishMcpToolRoundResponse, error) {
	f.mu.Lock()
	f.rpcCallCount++
	copy := proto.Clone(request).(*agentv1.FinishMcpToolRoundRequest)
	f.rounds[request.GetRoundId()] = copy
	f.writeStateLocked()
	f.mu.Unlock()
	return &agentv1.FinishMcpToolRoundResponse{RoundId: request.GetRoundId(), Status: request.GetStatus()}, nil
}

func (f *agentMCPRPCDrillFixture) FinishMcpToolInvocationFromRound(_ context.Context, request *agentv1.FinishMcpToolInvocationFromRoundRequest) (*agentv1.FinishMcpToolInvocationFromRoundResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rpcCallCount++
	command := f.commands[request.GetInvocationId()]
	round := f.rounds[request.GetRoundId()]
	if command == nil || round == nil {
		return nil, status.Error(codes.NotFound, "drill invocation receipt is unavailable")
	}
	terminal := "completed"
	if round.GetStatus() == "failed" {
		terminal = "failed"
	}
	command.Status = terminal
	f.writeStateLocked()
	return &agentv1.FinishMcpToolInvocationFromRoundResponse{InvocationId: request.GetInvocationId(), Status: terminal}, nil
}

func (f *agentMCPRPCDrillFixture) ResolveFreshMcpReadinessEvidence(_ context.Context, request *agentv1.ResolveFreshMcpReadinessEvidenceRequest) (*agentv1.ResolveFreshMcpReadinessEvidenceResponse, error) {
	f.recordCall()
	now := time.Now()
	expiresAt := now.Add(time.Minute)
	if _, err := os.Stat(f.stalePath); err == nil {
		expiresAt = now.Add(-time.Millisecond)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, status.Error(codes.Internal, "drill readiness state is unavailable")
	}
	return &agentv1.ResolveFreshMcpReadinessEvidenceResponse{
		Found: true, EvidenceId: strings.Repeat("e", 64), SchemaVersion: "dipole.agent.external-mcp-readiness-evidence-record.v1",
		ProfileBindingSha256: request.GetProfileBindingSha256(), RuntimeBindingSha256: request.GetRuntimeBindingSha256(),
		ContentSha256: strings.Repeat("c", 64), Status: "recorded", CollectedAtUnixMs: now.Add(-time.Second).UnixMilli(), ExpiresAtUnixMs: expiresAt.UnixMilli(),
	}, nil
}

func (f *agentMCPRPCDrillFixture) CreateArtifact(_ context.Context, request *agentv1.CreateArtifactRequest) (*agentv1.CreateArtifactResponse, error) {
	contentDigest := sha256.Sum256(request.GetContent())
	contentSHA := hex.EncodeToString(contentDigest[:])
	identity := strings.Join([]string{"dipole.agent.artifact.v1", strings.TrimSpace(request.GetTaskId()), strings.TrimSpace(request.GetRunId()),
		strings.TrimSpace(request.GetArtifactType()), strconv.Itoa(int(request.GetVersion())), contentSHA}, "\n")
	artifactDigest := sha256.Sum256([]byte(identity))
	f.mu.Lock()
	f.rpcCallCount++
	f.artifactCount++
	f.writeStateLocked()
	f.mu.Unlock()
	return &agentv1.CreateArtifactResponse{Artifact: &agentv1.AgentArtifact{
		SchemaVersion: "dipole.agent.artifact.v1", ArtifactId: hex.EncodeToString(artifactDigest[:]), TaskId: request.GetTaskId(), RunId: request.GetRunId(),
		ArtifactType: request.GetArtifactType(), Version: request.GetVersion(), Title: strings.TrimSpace(request.GetTitle()), MediaType: strings.TrimSpace(request.GetMediaType()),
		ContentSha256: contentSHA, SizeBytes: uint64(len(request.GetContent())), MetadataJson: append([]byte(nil), request.GetMetadataJson()...), CreatedAtUnixMs: time.Now().UnixMilli(),
	}}, nil
}

func (f *agentMCPRPCDrillFixture) CommitMemoryPromotionReceipt(_ context.Context, request *agentv1.CommitMemoryPromotionReceiptRequest) (*agentv1.CommitMemoryPromotionReceiptResponse, error) {
	if request.GetSchemaVersion() != "dipole.agent.memory-promotion-receipt.v2" || request.GetStatus() != "prepared" ||
		request.GetCandidateId() == "" || request.GetReviewId() == "" || request.GetReceiptSha256() == "" {
		return nil, status.Error(codes.InvalidArgument, "drill Memory promotion receipt is invalid")
	}
	f.mu.Lock()
	f.rpcCallCount++
	f.memoryPromotionCommitCount++
	f.writeStateLocked()
	f.mu.Unlock()
	return &agentv1.CommitMemoryPromotionReceiptResponse{
		MemoryId: "MEM-COMMIT-" + request.GetCandidateId(), MemoryType: request.GetTargetMemoryType(), Status: "active", ReceiptSha256: request.GetReceiptSha256(),
		Provenance: &agentv1.AgentMemoryProvenance{SourceType: "memory_candidate", SourceId: request.GetCandidateId(), Sequence: request.GetReviewId()},
	}, nil
}

func (f *agentMCPRPCDrillFixture) recordCall() {
	f.mu.Lock()
	f.rpcCallCount++
	f.writeStateLocked()
	f.mu.Unlock()
}

func (f *agentMCPRPCDrillFixture) writeState() {
	f.mu.Lock()
	f.writeStateLocked()
	f.mu.Unlock()
}

func (f *agentMCPRPCDrillFixture) writeStateLocked() {
	if f.statePath == "" {
		return
	}
	if err := writeAgentMCPRPCDrillJSONRaw(f.statePath, agentMCPRPCDrillState{
		SchemaVersion: "dipole.agent.mcp-rpc-drill-state.v1", RPCType: "go_internal_grpc_mtls",
		RPCAuthenticated: true, RPCCallCount: f.rpcCallCount, ArtifactCount: f.artifactCount,
		MemoryPromotionCommitCount: f.memoryPromotionCommitCount,
		FinishedStatuses:           append([]string(nil), f.finishedStatuses...),
	}); err != nil {
		panic("Agent MCP RPC drill state write failed")
	}
}

type agentMCPRPCDrillCertificates struct {
	ca, coreCert, coreKey, agentCert, agentKey, messageCert, messageKey string
}

func generateAgentMCPRPCDrillCertificates(t *testing.T) agentMCPRPCDrillCertificates {
	t.Helper()
	directory := t.TempDir()
	command := exec.Command("bash", filepath.Join("..", "..", "scripts", "generate-internal-certs.sh"))
	command.Env = append(os.Environ(), "INTERNAL_CERT_DIR="+directory, "INTERNAL_CERT_VALID_DAYS=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate internal certificates: %v: %s", err, output)
	}
	return agentMCPRPCDrillCertificates{
		ca: filepath.Join(directory, "ca.pem"), coreCert: filepath.Join(directory, "core.pem"), coreKey: filepath.Join(directory, "core-key.pem"),
		agentCert: filepath.Join(directory, "agent.pem"), agentKey: filepath.Join(directory, "agent-key.pem"),
		messageCert: filepath.Join(directory, "message.pem"), messageKey: filepath.Join(directory, "message-key.pem"),
	}
}

func startAgentMCPRPCDrillServer(t *testing.T, certs agentMCPRPCDrillCertificates, fixture *agentMCPRPCDrillFixture) *platformrpc.Server {
	t.Helper()
	cfg := config.InternalRPC{Enabled: true, SharedSecret: agentMCPRPCDrillSecret, CoreListenAddress: "127.0.0.1:0", TLSEnabled: true,
		TLSCertFile: certs.coreCert, TLSKeyFile: certs.coreKey, TLSCAFile: certs.ca, TLSServerName: "core"}
	server, err := platformrpc.NewServer(cfg, cfg.CoreListenAddress, []string{agentServiceName}, func(server *grpc.Server) {
		agentv1.RegisterAgentCapabilityServiceServer(server, fixture)
	}, corepolicy.RestrictAgentServiceMethods)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	return server
}

func dialAgentMCPRPCDrillClient(t *testing.T, address, caPath, certPath, keyPath, secret, caller string) agentv1.AgentCapabilityServiceClient {
	t.Helper()
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("drill CA is invalid")
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: "core", MinVersion: tls.VersionTLS13}
	if certPath != "" {
		certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	interceptor, err := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: caller, Secret: secret})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(grpcCredentials.NewTLS(tlsConfig)), grpc.WithUnaryInterceptor(interceptor))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return agentv1.NewAgentCapabilityServiceClient(connection)
}

func drillMatchRequest() *agentv1.MatchEventSubscriptionsRequest {
	return &agentv1.MatchEventSubscriptionsRequest{TenantId: "dipole", AgentId: "UAI-DRILL", EventType: "message.direct.created",
		ResourceType: "conversation", ResourceId: "direct:U100:U200"}
}

func drillMemoryPromotionCommitRequest() *agentv1.CommitMemoryPromotionReceiptRequest {
	createdAt := time.UnixMilli(1_700_000_000_000).UTC()
	return &agentv1.CommitMemoryPromotionReceiptRequest{
		ReceiptId: "MEM-PROMOTE-" + strings.Repeat("a", 64), ReceiptSha256: strings.Repeat("a", 64),
		SchemaVersion: "dipole.agent.memory-promotion-receipt.v2", Status: "prepared", TaskId: "TASK-1", RunId: "RUN-1",
		CandidateId: "CAND-1", CandidateSha256: strings.Repeat("b", 64), ReviewId: "REV-1", PolicyVersion: "memory-v1", TargetMemoryType: "semantic",
		CreatedAtUnixMs: createdAt.UnixMilli(), ExpiresAtUnixMs: createdAt.Add(time.Minute).UnixMilli(),
	}
}

func drillApprovalRequest() *agentv1.RequestApprovalRequest {
	return &agentv1.RequestApprovalRequest{
		TaskId: "TASK-APPROVAL-1", RunId: "RUN-APPROVAL-1", ApprovalId: "APR-APPROVAL-1", CapabilityId: "message.system.send",
		ResourceScope: &agentv1.AgentResourceScope{ResourceType: "conversation", ResourceId: "group:G1", Actions: []string{"write"}},
		ScopeSha256:   strings.Repeat("a", 64), ArgumentsSha256: strings.Repeat("b", 64), NonceSha256: strings.Repeat("c", 64),
		ExpiresAtUnixMs: time.Now().Add(time.Minute).UnixMilli(),
	}
}

func drillIdentifier(prefix string, length int, parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return prefix + hex.EncodeToString(digest[:])[:length]
}

func requiredAgentMCPRPCDrillEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

func writeAgentMCPRPCDrillJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := writeAgentMCPRPCDrillJSONRaw(path, value); err != nil {
		t.Fatal(err)
	}
}

func writeAgentMCPRPCDrillJSONRaw(path string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
