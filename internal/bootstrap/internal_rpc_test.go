package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	deliverygrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/delivery"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/common/v1"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type rpcCoreStub struct{}

type rpcDeliveryObservationSink struct{}

func (rpcDeliveryObservationSink) Observe(*deliveryv1.NodeDeliveryBatch) {}

type rpcBlockingDeliveryObservationSink struct {
	started chan struct{}
	release chan struct{}
}

func (s rpcBlockingDeliveryObservationSink) Observe(*deliveryv1.NodeDeliveryBatch) {
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
}

type rpcAgentCapabilityStub struct{ application.AgentCapabilityV1 }

func (rpcAgentCapabilityStub) ListConversations(context.Context, application.AgentInvocationV1, int) ([]*model.Conversation, error) {
	return []*model.Conversation{{ConversationKey: "direct:U100:UAI"}}, nil
}

type rpcAgentResolverStub struct{}

func (rpcAgentResolverStub) Resolve(context.Context, string, string) (application.AgentInvocationV1, error) {
	return application.AgentInvocationV1{PrincipalUUID: "U100", AgentUUID: "UAI"}, nil
}

type rpcAgentAdmissionStub struct{}
type rpcAgentApprovalStub struct{}
type rpcAgentTaskControlStub struct{}
type rpcAgentWorkflowProjectionStub struct{}
type rpcAgentWorkflowRepairStub struct{ operator string }
type rpcAgentSubscriptionStub struct{}
type rpcAgentMemoryStub struct{}
type rpcAgentArtifactStub struct {
	artifact *application.AgentArtifactV1
	body     []byte
}

func (rpcAgentSubscriptionStub) MatchEventSubscriptions(_ context.Context, request application.AgentEventSubscriptionMatchRequestV1) ([]application.AgentEventSubscriptionV1, error) {
	return []application.AgentEventSubscriptionV1{{
		SubscriptionUUID: "SUB-1", DefinitionUUID: "DEF-1", DefinitionVersion: 1,
		TenantID: request.TenantID, AgentUUID: request.AgentUUID, Status: application.AgentSubscriptionStatusActive,
		EventType: request.EventType, ResourceType: request.ResourceType, ResourceID: request.ResourceID,
		FilterKind: application.AgentSubscriptionFilterAll, FilterJSON: []byte(`{}`), CreatedByUUID: "U100",
	}}, nil
}

func (rpcAgentMemoryStub) ResolveContextMemories(_ context.Context, taskUUID, runUUID, resourceType, resourceID string, limit int) ([]application.AgentMemoryV1, error) {
	if taskUUID != "TASK-1" || runUUID != "RUN-1" || resourceType != "conversation" || resourceID != "group:G1" || limit != 20 {
		return nil, application.ErrAgentMemoryDenied
	}
	return []application.AgentMemoryV1{{
		MemoryUUID: "MEM-1", MemoryType: application.AgentMemoryTypeSemantic, Content: "Owner is Alice", Priority: 90,
		Provenance: application.AgentMemoryProvenanceV1{SourceType: "message", SourceID: "M1"},
	}}, nil
}

func (s *rpcAgentArtifactStub) Create(_ context.Context, input application.AgentArtifactCreateV1) (*application.AgentArtifactV1, error) {
	if input.TaskUUID != "TASK-1" || input.RunUUID != "RUN-1" {
		return nil, application.ErrAgentArtifactDenied
	}
	s.artifact = &application.AgentArtifactV1{ArtifactUUID: strings.Repeat("a", 64), SchemaVersion: application.AgentArtifactSchemaVersionV1, TaskUUID: input.TaskUUID, RunUUID: input.RunUUID, ArtifactType: input.ArtifactType, Version: input.Version, Title: input.Title, MediaType: input.MediaType, ContentSHA256: strings.Repeat("b", 64), SizeBytes: uint64(len(input.Content)), Metadata: input.Metadata, CreatedAt: time.Unix(1, 0)}
	s.body = append([]byte(nil), input.Content...)
	return s.artifact, nil
}
func (s *rpcAgentArtifactStub) GetForPrincipal(_ context.Context, principal, artifactUUID string) (*application.AgentArtifactV1, []byte, error) {
	if principal != "U100" || s.artifact == nil || artifactUUID != s.artifact.ArtifactUUID {
		return nil, nil, application.ErrAgentArtifactDenied
	}
	return s.artifact, append([]byte(nil), s.body...), nil
}

func (s *rpcAgentWorkflowRepairStub) Propose(_ context.Context, operator string, request application.AgentWorkflowRepairProposalRequestV1) (*application.AgentWorkflowRepairProposalV1, error) {
	s.operator = operator
	return &application.AgentWorkflowRepairProposalV1{ProposalUUID: "repair:" + strings.Repeat("a", 64), TaskUUID: request.TaskUUID, ProposerUUID: operator,
		Outcome: request.Outcome, Action: application.AgentWorkflowRepairActionV1, Temporal: request.Temporal, EvidenceSHA256: strings.Repeat("a", 64),
		Status: application.AgentWorkflowRepairStatusProposed, RequiredApprovals: 2, ProposedAt: request.ProposedAt, ExpiresAt: request.ExpiresAt}, nil
}
func (s *rpcAgentWorkflowRepairStub) Decide(context.Context, string, string, application.AgentWorkflowRepairDecisionV1) (*application.AgentWorkflowRepairProposalV1, error) {
	return nil, application.ErrAgentWorkflowRepairDenied
}
func (s *rpcAgentWorkflowRepairStub) Get(context.Context, string, string) (*application.AgentWorkflowRepairProposalV1, error) {
	return nil, application.ErrAgentWorkflowRepairDenied
}

func (rpcAgentTaskControlStub) AuthorizeTaskControl(_ context.Context, taskUUID, principalUUID string) (*application.AgentTaskControlAuthorizationV1, error) {
	if taskUUID != "TASK-1" || principalUUID != "U100" {
		return nil, application.ErrAgentExecutionPolicyDenied
	}
	return &application.AgentTaskControlAuthorizationV1{TaskUUID: taskUUID, Status: application.AgentTaskStatusWaitingApproval}, nil
}

func (rpcAgentWorkflowProjectionStub) Project(_ context.Context, request application.AgentTaskWorkflowProjectionRequestV1) (*application.AgentTaskWorkflowProjectionV1, error) {
	projection := request.Projection
	return &projection, nil
}

func (rpcAgentWorkflowProjectionStub) ListProjectionSnapshots(_ context.Context, _ string, _ int) (*application.AgentTaskWorkflowProjectionPageV1, error) {
	return &application.AgentTaskWorkflowProjectionPageV1{}, nil
}

func (rpcAgentApprovalStub) Request(_ context.Context, request application.AgentApprovalRequestV1) (*application.AgentApprovalV1, error) {
	approval := request.Approval
	return &approval, nil
}
func (rpcAgentApprovalStub) Resolve(_ context.Context, resolution application.AgentApprovalResolutionV1) (*application.AgentApprovalV1, error) {
	return &application.AgentApprovalV1{ApprovalUUID: resolution.ApprovalUUID, Status: application.AgentApprovalStatusApproved, ApprovedByUUID: resolution.ActorUUID}, nil
}
func (rpcAgentApprovalStub) Consume(context.Context, application.AgentApprovalConsumptionV1) error {
	return nil
}

func (rpcAgentAdmissionStub) Admit(context.Context, application.AgentRunAdmissionRequestV1) (*application.AgentRunAdmissionV1, error) {
	return &application.AgentRunAdmissionV1{TaskUUID: "TASK-1", RunUUID: "RUN-1", RunStatus: application.AgentRunStatusRunning}, nil
}

func (rpcAgentAdmissionStub) Complete(context.Context, string, string, string, string) error {
	return nil
}

func (rpcAgentAdmissionStub) Finish(context.Context, string, string, string, string, application.AgentRunStatusV1, string) error {
	return nil
}

func (rpcCoreStub) ListSearchConversationKeys(userUUID string) ([]string, error) {
	return []string{"direct:" + userUUID + ":U2"}, nil
}

type rpcSearchStub struct{}

type rpcSyncStub struct{}

func (rpcSyncStub) List(userUUID string, afterSeq uint64, limit int) (*application.SyncPage, error) {
	return &application.SyncPage{
		Items:   []*model.SyncMessage{{SyncSeq: afterSeq + 1, ConversationKey: "direct:" + userUUID + ":U2"}},
		NextSeq: afterSeq + 1,
	}, nil
}

func (rpcSyncStub) GetCheckpoint(userUUID, deviceID string) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: 9}, nil
}

func (rpcSyncStub) AdvanceCheckpoint(userUUID, deviceID string, syncSeq uint64) (*model.DeviceSyncCheckpoint, error) {
	return &model.DeviceSyncCheckpoint{UserUUID: userUUID, DeviceID: deviceID, SyncSeq: syncSeq}, nil
}

func (rpcSyncStub) ListGroupCheckpoints(string, string, []string) ([]*model.GroupSyncCheckpoint, error) {
	return []*model.GroupSyncCheckpoint{{GroupUUID: "G1", LatestMessageSeq: 12}}, nil
}

func (rpcSyncStub) AdvanceGroupCheckpoint(_, _, groupUUID string, messageSeq uint64) (*model.GroupSyncCheckpoint, error) {
	return &model.GroupSyncCheckpoint{GroupUUID: groupUUID, PulledMessageSeq: messageSeq}, nil
}

func (rpcSearchStub) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	return []*model.MessageSearchDocument{{
		MessageUUID: "M1", ConversationKey: "direct:" + principal + ":U2", MessageSeq: 7,
		Revision: 1, SenderUUID: "U2", MessageType: model.MessageTypeText, Content: text, SentAt: time.Unix(1, 0),
	}}, nil
}

func (rpcCoreStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID, Nickname: "RPC User"}, nil
}
func (rpcCoreStub) CanSendDirectMessage(string, string) (bool, error) { return true, nil }
func (rpcCoreStub) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return &model.Group{UUID: groupUUID, Name: "RPC Group"}, nil
}
func (rpcCoreStub) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID}, nil
}
func (rpcCoreStub) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return []*model.GroupMember{{GroupUUID: groupUUID, UserUUID: "U1"}}, nil
}
func (rpcCoreStub) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	return &model.UploadedFile{UUID: fileUUID, UploaderUUID: uploaderUUID, FileName: "rpc-file"}, nil
}

func TestCoreRPCServerAndClientUseAuthenticatedNetworkChannel(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})

	cfg.CoreTarget = server.Address()
	client, connection, err := DialCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	user, err := client.GetUserByUUID("U100")
	if err != nil {
		t.Fatalf("get user through core rpc: %v", err)
	}
	if user == nil || user.UUID != "U100" || user.Nickname != "RPC User" {
		t.Fatalf("unexpected user: %#v", user)
	}
	_, err = corev1.NewCoreCapabilityServiceClient(connection).GetUser(context.Background(), &corev1.GetUserRequest{
		Context: &commonv1.RequestContext{CallerService: "dipole-gateway"},
		UserId:  "U100",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected authenticated caller mismatch rejection, got %v", err)
	}
}

func TestDeliveryObservationRPCUsesRealtimeIdentity(t *testing.T) {
	receiver, err := deliverygrpc.NewShadowServer("gateway-1", 4, 25*time.Millisecond, rpcDeliveryObservationSink{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(receiver.Close)
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", DeliveryObservationListenAddress: "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewDeliveryObservationRPCServer(cfg, receiver)
	if err != nil {
		t.Fatalf("start delivery observation rpc: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})

	connection, err := dialInternalRPC(context.Background(), cfg, server.Address(), grpcauth.Credentials{
		Service: realtimeServiceName, Secret: cfg.SharedSecret,
	})
	if err != nil {
		t.Fatalf("dial delivery observation as Realtime: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	observation, err := deliveryv1.NewNodeDeliveryServiceClient(connection).ObserveNodeBatch(context.Background(), rpcObservationBatch("NB-bootstrap"))
	if err != nil || observation.GetStatus() != deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_OBSERVED {
		t.Fatalf("delivery observation=%+v err=%v", observation, err)
	}

	if _, err := dialInternalRPC(context.Background(), cfg, server.Address(), grpcauth.Credentials{
		Service: messageServiceName, Secret: cfg.SharedSecret,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected Message caller rejection, got %v", err)
	}
}

func TestDeliveryObservationRPCReportsBackpressureOverTCP(t *testing.T) {
	sink := rpcBlockingDeliveryObservationSink{started: make(chan struct{}, 1), release: make(chan struct{})}
	receiver, err := deliverygrpc.NewShadowServer("gateway-1", 1, 25*time.Millisecond, sink)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(receiver.Close)
	t.Cleanup(func() { close(sink.release) })
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", DeliveryObservationListenAddress: "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewDeliveryObservationRPCServer(cfg, receiver)
	if err != nil {
		t.Fatalf("start delivery observation rpc: %v", err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	connection, err := dialInternalRPC(context.Background(), cfg, server.Address(), grpcauth.Credentials{
		Service: realtimeServiceName, Secret: cfg.SharedSecret,
	})
	if err != nil {
		t.Fatalf("dial delivery observation: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := deliveryv1.NewNodeDeliveryServiceClient(connection)

	if _, err := client.ObserveNodeBatch(context.Background(), rpcObservationBatch("NB-pressure-1")); err != nil {
		t.Fatalf("observe first batch: %v", err)
	}
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("observation worker did not start")
	}
	if _, err := client.ObserveNodeBatch(context.Background(), rpcObservationBatch("NB-pressure-2")); err != nil {
		t.Fatalf("fill observation queue: %v", err)
	}
	observation, err := client.ObserveNodeBatch(context.Background(), rpcObservationBatch("NB-pressure-3"))
	if err != nil {
		t.Fatalf("observe saturated queue: %v", err)
	}
	if observation.GetStatus() != deliveryv1.NodeObservationStatus_NODE_OBSERVATION_STATUS_BACKPRESSURED ||
		observation.GetErrorCode() != deliveryv1.DeliveryErrorCode_DELIVERY_ERROR_CODE_QUEUE_FULL ||
		observation.GetPressure().GetDepth() != 1 || observation.GetPressure().GetCapacity() != 1 ||
		observation.GetPressure().GetRetryAfterMs() != 25 {
		t.Fatalf("unexpected backpressure observation: %+v", observation)
	}
}

func rpcObservationBatch(batchID string) *deliveryv1.NodeDeliveryBatch {
	return &deliveryv1.NodeDeliveryBatch{
		ContractVersion: "v1", BatchId: batchID, TargetNodeId: "gateway-1", SourceEventId: "E1",
		CreatedAt: timestamppb.New(time.Unix(1, 0).UTC()),
		Items: []*deliveryv1.NodeDeliveryItem{{
			DeliveryId: "D-" + batchID, RecipientUserId: "U1", ConnectionIds: []string{"C1"},
			EventType: "chat.message", PayloadJson: []byte(`{"message_id":"M1"}`), OrderingKey: "direct:U1:U2",
			Mode: deliveryv1.DeliveryMode_DELIVERY_MODE_FULL_EVENT,
		}},
	}
}

func TestAgentRPCUsesAuthenticatedLeastPrivilegeChannel(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
	server, err := NewCoreRPCServerWithAgent(cfg, rpcCoreStub{}, rpcAgentCapabilityStub{}, rpcAgentResolverStub{}, rpcAgentAdmissionStub{}, rpcAgentApprovalStub{})
	if err != nil {
		t.Fatalf("start Agent rpc server: %v", err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	interceptor, err := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: agentServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		t.Fatalf("create Agent rpc credentials: %v", err)
	}
	connection, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(interceptor))
	if err != nil {
		t.Fatalf("dial Agent rpc: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	agentClient := agentv1.NewAgentCapabilityServiceClient(connection)
	response, err := agentClient.AdmitRun(context.Background(), &agentv1.AdmitRunRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TenantId: "dipole", PrincipalUserId: "U100",
		AgentId: "UAI", TriggerType: "message.direct.created", TriggerRef: "M100", RuntimeId: agentServiceName, Mode: "shadow",
	})
	if err != nil || response.GetRunId() != "RUN-1" {
		t.Fatalf("admit Agent Run through authenticated channel: response=%+v err=%v", response, err)
	}
	finished, err := agentClient.FinishRun(context.Background(), &agentv1.FinishRunRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", RunId: "RUN-1", RunStatus: "cancelled",
	})
	if err != nil || finished.GetRunStatus() != "cancelled" {
		t.Fatalf("finish Agent Run through authenticated channel: response=%+v err=%v", finished, err)
	}
	expiresAt := time.Now().Add(time.Hour).UnixMilli()
	requested, err := agentClient.RequestApproval(context.Background(), &agentv1.RequestApprovalRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1",
		CapabilityId: "message.bulk.send", ResourceScope: &agentv1.AgentResourceScope{ResourceType: "conversation", ResourceId: "G1", Actions: []string{"write"}},
		ScopeSha256: strings.Repeat("a", 64), ArgumentsSha256: strings.Repeat("b", 64), NonceSha256: strings.Repeat("c", 64), ExpiresAtUnixMs: expiresAt,
	})
	if err != nil || requested.GetStatus() != "pending" {
		t.Fatalf("request Approval through authenticated channel: response=%+v err=%v", requested, err)
	}
	resolved, err := agentClient.ResolveApproval(context.Background(), &agentv1.ResolveApprovalRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", RunId: "RUN-1", ApprovalId: "APR-1", ActorUserId: "U100", Decision: "approved",
	})
	if err != nil || resolved.GetStatus() != "approved" {
		t.Fatalf("resolve Approval through authenticated channel: response=%+v err=%v", resolved, err)
	}
	_, err = corev1.NewCoreCapabilityServiceClient(connection).GetUser(context.Background(), &corev1.GetUserRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, UserId: "U100",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent Core method code = %s, want permission denied", status.Code(err))
	}
}

func TestAgentRPCControlAuthorizationUsesLeastPrivilegeChannel(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
	server, err := NewCoreRPCServerWithAgentControlAndProjection(
		cfg, rpcCoreStub{}, rpcAgentCapabilityStub{}, rpcAgentResolverStub{}, rpcAgentAdmissionStub{}, rpcAgentApprovalStub{}, rpcAgentTaskControlStub{}, rpcAgentWorkflowProjectionStub{},
	)
	if err != nil {
		t.Fatalf("start Agent control rpc server: %v", err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	interceptor, err := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: agentServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		t.Fatalf("create Agent rpc credentials: %v", err)
	}
	connection, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(interceptor))
	if err != nil {
		t.Fatalf("dial Agent rpc: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	client := agentv1.NewAgentCapabilityServiceClient(connection)
	response, err := client.AuthorizeTaskControl(context.Background(), &agentv1.AuthorizeTaskControlRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", PrincipalUserId: "U100",
	})
	if err != nil || response.GetTaskStatus() != "waiting_approval" {
		t.Fatalf("authorize Agent Task control: response=%+v err=%v", response, err)
	}
	projected, err := client.ProjectTaskWorkflowState(context.Background(), &agentv1.ProjectTaskWorkflowStateRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", RunId: "RUN-1",
		WorkflowId: "dipole-agent-task/TASK-1", WorkflowRunId: "temporal-run-1", WorkflowStatus: "running", WorkflowRevision: 1,
	})
	if err != nil || projected.GetWorkflowRevision() != 1 || projected.GetWorkflowId() != "dipole-agent-task/TASK-1" {
		t.Fatalf("project Agent Task Workflow state: response=%+v err=%v", projected, err)
	}
	projectionPage, err := client.ListTaskWorkflowProjectionSnapshots(context.Background(), &agentv1.ListTaskWorkflowProjectionSnapshotsRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, PageSize: 100,
	})
	if err != nil || len(projectionPage.GetTasks()) != 0 {
		t.Fatalf("list Agent Task Workflow projections: response=%+v err=%v", projectionPage, err)
	}
	if _, err := corev1.NewCoreCapabilityServiceClient(connection).GetUser(context.Background(), &corev1.GetUserRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, UserId: "U100",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent control channel accessed unrelated Core capability: %v", err)
	}
}

func TestGatewayUsesItsOwnAuthenticatedCoreIdentity(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})
	cfg.CoreTarget = server.Address()
	client, connection, err := DialGatewayCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial gateway core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	user, err := client.GetUserByUUID("U-GATEWAY")
	if err != nil || user == nil || user.UUID != "U-GATEWAY" {
		t.Fatalf("gateway core query failed: user=%+v err=%v", user, err)
	}
}

func TestWorkflowRepairRPCRequiresAuthenticatedGatewayIdentity(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
	repairs := &rpcAgentWorkflowRepairStub{}
	server, err := NewCoreRPCServerWithAgentControlAndProjection(cfg, rpcCoreStub{}, rpcAgentCapabilityStub{}, rpcAgentResolverStub{}, rpcAgentAdmissionStub{}, rpcAgentApprovalStub{}, rpcAgentTaskControlStub{}, rpcAgentWorkflowProjectionStub{}, repairs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })
	cfg.CoreTarget = server.Address()
	gatewayClient, gatewayConnection, err := DialGatewayAgentCapability(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gatewayConnection.Close() })
	response, err := gatewayClient.ProposeWorkflowRepair(context.Background(), &agentv1.ProposeWorkflowRepairRequest{Context: &commonv1.RequestContext{CallerService: gatewayServiceName, PrincipalUserId: "U-OPS"}, TaskId: "TASK-1", Outcome: "stale", TicketRef: "INC-1", Reason: "verified", Temporal: &agentv1.WorkflowRepairEvidence{WorkflowId: "dipole-agent-task/TASK-1", WorkflowRunId: "WR-1", Status: "completed", Revision: 3}, ProposedAtUnixMs: 1000, ExpiresAtUnixMs: 2000})
	if err != nil || response.GetProposerId() != "U-OPS" || repairs.operator != "U-OPS" {
		t.Fatalf("Gateway repair response=%+v operator=%s err=%v", response, repairs.operator, err)
	}
	agentInterceptor, _ := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: agentServiceName, Secret: cfg.SharedSecret})
	agentConnection, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(agentInterceptor))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentConnection.Close() })
	_, err = agentv1.NewAgentCapabilityServiceClient(agentConnection).GetWorkflowRepair(context.Background(), &agentv1.GetWorkflowRepairRequest{Context: &commonv1.RequestContext{CallerService: agentServiceName, PrincipalUserId: "U-OPS"}, ProposalId: response.GetProposalId()})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent repair code=%s", status.Code(err))
	}
}

func TestAgentArtifactRPCSeparatesRuntimeCreateAndPrincipalRead(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2}
	artifacts := &rpcAgentArtifactStub{}
	server, err := NewCoreRPCServerWithAgentArtifacts(cfg, rpcCoreStub{}, rpcAgentCapabilityStub{}, rpcAgentResolverStub{}, rpcAgentAdmissionStub{}, rpcAgentApprovalStub{}, rpcAgentTaskControlStub{}, rpcAgentWorkflowProjectionStub{}, &rpcAgentWorkflowRepairStub{}, rpcAgentSubscriptionStub{}, nil, artifacts, nil, nil, nil, nil, nil, nil, nil, rpcAgentMemoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close(context.Background()) })

	agentInterceptor, _ := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: agentServiceName, Secret: cfg.SharedSecret})
	agentConnection, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(agentInterceptor))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentConnection.Close() })
	agentClient := agentv1.NewAgentCapabilityServiceClient(agentConnection)
	matched, err := agentClient.MatchEventSubscriptions(context.Background(), &agentv1.MatchEventSubscriptionsRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TenantId: "dipole", AgentId: "UAI",
		EventType: "message.direct.created", ResourceType: "conversation", ResourceId: "group:G1",
	})
	if err != nil || len(matched.GetSubscriptions()) != 1 || matched.GetSubscriptions()[0].GetSubscriptionId() != "SUB-1" {
		t.Fatalf("Agent Event Subscription match=%+v err=%v", matched, err)
	}
	memories, err := agentClient.ListContextMemories(context.Background(), &agentv1.ListContextMemoriesRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TaskId: "TASK-1", RunId: "RUN-1",
		ResourceType: "conversation", ResourceId: "group:G1", Limit: 20,
	})
	if err != nil || len(memories.GetMemories()) != 1 || memories.GetMemories()[0].GetMemoryId() != "MEM-1" {
		t.Fatalf("Agent Context Memories=%+v err=%v", memories, err)
	}
	created, err := agentClient.CreateArtifact(context.Background(), &agentv1.CreateArtifactRequest{
		Context: &commonv1.RequestContext{CallerService: agentServiceName}, TenantId: "dipole", TaskId: "TASK-1", RunId: "RUN-1",
		ArtifactType: "conversation_digest", Version: 1, Title: "Digest", MediaType: "text/markdown", Content: []byte("digest"), MetadataJson: []byte(`{}`),
	})
	if err != nil || created.GetArtifact().GetArtifactId() != strings.Repeat("a", 64) {
		t.Fatalf("Agent Artifact create=%+v err=%v", created, err)
	}
	if _, err := agentClient.GetArtifact(context.Background(), &agentv1.GetArtifactRequest{Context: &commonv1.RequestContext{CallerService: agentServiceName, PrincipalUserId: "U100"}, ArtifactId: created.GetArtifact().GetArtifactId()}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Agent Artifact read code=%s", status.Code(err))
	}

	cfg.CoreTarget = server.Address()
	gatewayClient, gatewayConnection, err := DialGatewayAgentCapability(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gatewayConnection.Close() })
	read, err := gatewayClient.GetArtifact(context.Background(), &agentv1.GetArtifactRequest{Context: &commonv1.RequestContext{CallerService: gatewayServiceName, PrincipalUserId: "U100"}, ArtifactId: created.GetArtifact().GetArtifactId()})
	if err != nil || string(read.GetContent()) != "digest" {
		t.Fatalf("Gateway Artifact read=%+v err=%v", read, err)
	}
	if _, err := gatewayClient.GetArtifact(context.Background(), &agentv1.GetArtifactRequest{Context: &commonv1.RequestContext{CallerService: gatewayServiceName, PrincipalUserId: "U999"}, ArtifactId: created.GetArtifact().GetArtifactId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("cross-principal Artifact read code=%s", status.Code(err))
	}
}

func TestSearchServiceUsesAuthenticatedCoreAndGatewayChannels(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0",
		SearchListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2,
	}
	coreServer, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start Core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		coreServer.Close(ctx)
	})
	cfg.CoreTarget = coreServer.Address()
	core, coreConnection, err := DialSearchCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Core as Search service: %v", err)
	}
	t.Cleanup(func() { _ = coreConnection.Close() })
	keys, err := core.ListSearchConversationKeys("U1")
	if err != nil || len(keys) != 1 || keys[0] != "direct:U1:U2" {
		t.Fatalf("Search Core scope: keys=%v err=%v", keys, err)
	}
	if _, err := core.GetUserByUUID("U1"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected Search identity to be denied unrelated Core capability, got %v", err)
	}

	searchServer, err := NewSearchRPCServer(cfg, rpcSearchStub{})
	if err != nil {
		t.Fatalf("start Search rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		searchServer.Close(ctx)
	})
	cfg.SearchTarget = searchServer.Address()
	search, searchConnection, err := DialSearchApplication(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Search application: %v", err)
	}
	t.Cleanup(func() { _ = searchConnection.Close() })
	documents, err := search.Search("U1", "migration", 10)
	if err != nil || len(documents) != 1 || documents[0].Content != "migration" {
		t.Fatalf("Search rpc result: documents=%+v err=%v", documents, err)
	}
}

func TestSyncServiceUsesAuthenticatedCoreAndCoreChannels(t *testing.T) {
	cfg := config.InternalRPC{
		Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "127.0.0.1:0",
		SyncListenAddress: "127.0.0.1:0", DialTimeoutSeconds: 2,
	}
	coreServer, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start Core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		coreServer.Close(ctx)
	})
	cfg.CoreTarget = coreServer.Address()
	core, coreConnection, err := DialSyncCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Core as Sync service: %v", err)
	}
	t.Cleanup(func() { _ = coreConnection.Close() })
	member, err := core.GetGroupMember("G1", "U1")
	if err != nil || member == nil || member.GroupUUID != "G1" {
		t.Fatalf("Sync Core membership scope: member=%+v err=%v", member, err)
	}
	if _, err := core.GetUserByUUID("U1"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected Sync identity to be denied unrelated Core capability, got %v", err)
	}

	syncServer, err := NewSyncRPCServer(cfg, rpcSyncStub{})
	if err != nil {
		t.Fatalf("start Sync rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		syncServer.Close(ctx)
	})
	cfg.SyncTarget = syncServer.Address()
	syncApplication, syncConnection, err := DialCoreSyncApplication(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial Sync application as Core: %v", err)
	}
	t.Cleanup(func() { _ = syncConnection.Close() })
	page, err := syncApplication.List("U1", 7, 20)
	if err != nil || page == nil || page.NextSeq != 8 || len(page.Items) != 1 {
		t.Fatalf("Sync rpc result: page=%+v err=%v", page, err)
	}
	checkpoint, err := syncApplication.GetCheckpoint("U1", "web-1")
	if err != nil || checkpoint == nil || checkpoint.SyncSeq != 9 || checkpoint.DeviceID != "web-1" {
		t.Fatalf("Sync checkpoint result: checkpoint=%+v err=%v", checkpoint, err)
	}
}

func TestInternalRPCRejectsMissingRuntimeCredentials(t *testing.T) {
	if _, err := NewCoreRPCServer(config.InternalRPC{Enabled: true, CoreListenAddress: "127.0.0.1:0"}, rpcCoreStub{}); err == nil {
		t.Fatal("expected core rpc server without shared secret to fail")
	}
	if _, _, err := DialCoreCapability(context.Background(), config.InternalRPC{Enabled: true, CoreTarget: "127.0.0.1:1"}); err == nil {
		t.Fatal("expected core rpc client without shared secret to fail")
	}
}

func TestCoreRPCServerAndClientUseMutualTLS(t *testing.T) {
	certFile, keyFile, caFile := writeRPCIdentity(t, messageServiceName)
	cfg := config.InternalRPC{
		Enabled:            true,
		SharedSecret:       "test-secret",
		CoreListenAddress:  "127.0.0.1:0",
		DialTimeoutSeconds: 2,
		TLSEnabled:         true,
		TLSCertFile:        certFile,
		TLSKeyFile:         keyFile,
		TLSCAFile:          caFile,
		TLSServerName:      "localhost",
	}
	server, err := NewCoreRPCServer(cfg, rpcCoreStub{})
	if err != nil {
		t.Fatalf("start mtls core rpc server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Close(ctx)
	})
	cfg.CoreTarget = server.Address()
	client, connection, err := DialCoreCapability(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dial mtls core capability: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if user, err := client.GetUserByUUID("U-TLS"); err != nil || user == nil || user.UUID != "U-TLS" {
		t.Fatalf("mtls core call failed: user=%+v err=%v", user, err)
	}
}

func TestInternalRPCRejectsPlaintextOutsideLoopback(t *testing.T) {
	cfg := config.InternalRPC{Enabled: true, SharedSecret: "test-secret", CoreListenAddress: "0.0.0.0:0"}
	if _, err := NewCoreRPCServer(cfg, rpcCoreStub{}); err == nil {
		t.Fatal("expected non-loopback plaintext listener to fail")
	}
	cfg.CoreTarget = "10.0.0.1:9091"
	if _, _, err := DialCoreCapability(context.Background(), cfg); err == nil {
		t.Fatal("expected non-loopback plaintext target to fail")
	}
}

func writeRPCIdentity(t testing.TB, serviceName string) (string, string, string) {
	t.Helper()
	directory := t.TempDir()
	now := time.Now().UTC()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Dipole Test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create ca certificate: %v", err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: serviceName},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	certFile := filepath.Join(directory, "identity.pem")
	keyFile := filepath.Join(directory, "identity-key.pem")
	caFile := filepath.Join(directory, "ca.pem")
	writePEM(t, certFile, "CERTIFICATE", leafDER, 0o644)
	writePEM(t, keyFile, "EC PRIVATE KEY", keyDER, 0o600)
	writePEM(t, caFile, "CERTIFICATE", caDER, 0o644)
	return certFile, keyFile, caFile
}

func writePEM(t testing.TB, path, blockType string, data []byte, mode os.FileMode) {
	t.Helper()
	encoded := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data})
	if err := os.WriteFile(path, encoded, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
