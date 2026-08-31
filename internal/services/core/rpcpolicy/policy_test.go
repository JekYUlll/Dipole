package rpcpolicy

import (
	"context"
	"net"
	"testing"

	agentv1 "github.com/JekYUlll/Dipole/api/gen/go/agent/v1"
	commonv1 "github.com/JekYUlll/Dipole/api/gen/go/common/v1"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const policyTestAgentService = "dipole-agent"

func TestRestrictAgentServiceMethodsAllowsTaskTimelineRead(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	serverAuth, err := grpcauth.NewUnaryServerInterceptor("test-secret", policyTestAgentService)
	if err != nil {
		t.Fatalf("create server auth: %v", err)
	}
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(serverAuth, RestrictAgentServiceMethods))
	agentv1.RegisterAgentCapabilityServiceServer(server, policyTestTimelineServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	clientAuth, err := grpcauth.NewUnaryClientInterceptor(grpcauth.Credentials{Service: policyTestAgentService, Secret: "test-secret"})
	if err != nil {
		t.Fatalf("create client auth: %v", err)
	}
	connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(clientAuth))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	response, err := agentv1.NewAgentCapabilityServiceClient(connection).ListAgentTaskTimeline(context.Background(), &agentv1.ListAgentTaskTimelineRequest{
		Context:         &commonv1.RequestContext{CallerService: policyTestAgentService},
		TaskId:          "task:policy-timeline",
		PrincipalUserId: "U-policy",
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("list Agent Task Timeline: %v", err)
	}
	if response.GetTaskId() != "task:policy-timeline" {
		t.Fatalf("task id = %q, want task:policy-timeline", response.GetTaskId())
	}
}

type policyTestTimelineServer struct {
	agentv1.UnimplementedAgentCapabilityServiceServer
}

func (policyTestTimelineServer) ListAgentTaskTimeline(_ context.Context, request *agentv1.ListAgentTaskTimelineRequest) (*agentv1.ListAgentTaskTimelineResponse, error) {
	return &agentv1.ListAgentTaskTimelineResponse{SchemaVersion: "dipole.agent.task-timeline.v1", TaskId: request.GetTaskId()}, nil
}
