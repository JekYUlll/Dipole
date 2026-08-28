package bootstrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	agentgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/agent"
	grpcauth "github.com/JekYUlll/Dipole/internal/transport/grpc/auth"
	coregrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/core"
	deliverygrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/delivery"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	corev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/core/v1"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	messagev1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/message/v1"
	searchv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/search/v1"
	syncv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/sync/v1"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	searchgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/search"
	syncgrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/sync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcCredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const (
	gatewayServiceName  = "dipole-gateway"
	coreServiceName     = "dipole-core"
	messageServiceName  = "dipole-message"
	searchServiceName   = "dipole-search"
	syncServiceName     = "dipole-sync"
	agentServiceName    = "dipole-agent"
	realtimeServiceName = "dipole-realtime"
)

type InternalRPCServer struct {
	listener net.Listener
	server   *grpc.Server
	health   *health.Server
	done     chan struct{}
	stopOnce sync.Once
}

func NewDeliveryObservationRPCServer(cfg config.InternalRPC, adapter *deliverygrpc.ShadowServer) (*InternalRPCServer, error) {
	if adapter == nil {
		return nil, errors.New("delivery observation rpc adapter is required")
	}
	return newInternalRPCServer(cfg, cfg.DeliveryObservationListenAddress, []string{realtimeServiceName}, func(server *grpc.Server) {
		deliveryv1.RegisterNodeDeliveryServiceServer(server, adapter)
	})
}

func NewCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability) (*InternalRPCServer, error) {
	return newCoreRPCServer(cfg, capability, nil)
}

func NewCoreRPCServerWithAgent(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals ...application.AgentApprovalServiceV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServer(agentCapability, resolver, admission, approvals...)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func NewCoreRPCServerWithAgentControl(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServerWithControl(agentCapability, resolver, admission, approvals, controls)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func NewCoreRPCServerWithAgentControlAndProjection(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs ...application.AgentWorkflowRepairAuditServiceV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServerWithControlAndProjection(agentCapability, resolver, admission, approvals, controls, projections, repairs...)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func NewCoreRPCServerWithAgentArtifacts(cfg config.InternalRPC, capability application.CoreCapability, agentCapability application.AgentCapabilityV1, resolver application.AgentInvocationResolverV1, admission application.AgentRunAdmissionServiceV1, approvals application.AgentApprovalServiceV1, controls application.AgentTaskControlAuthorizerV1, projections application.AgentTaskWorkflowProjectionServiceV1, repairs application.AgentWorkflowRepairAuditServiceV1, subscriptions application.AgentEventSubscriptionResolverV1, subscriptionControls application.AgentEventSubscriptionControlServiceV1, artifacts application.AgentArtifactServiceV1, toolAudits application.AgentToolInvocationAuditServiceV1, messageCommands application.AgentMessageCommandExecutionV1, approvalGrants application.AgentApprovalGrantResolverV1, promotionControls application.AgentRuntimePromotionControlServiceV1, promotionEvidence application.AgentRuntimePromotionEvidenceReviewServiceV1, memories ...application.AgentMemoryContextResolverV1) (*InternalRPCServer, error) {
	agentAdapter, err := agentgrpc.NewServerWithControlAndProjection(agentCapability, resolver, admission, approvals, controls, projections, repairs)
	if err != nil {
		return nil, fmt.Errorf("create Agent Capability rpc adapter: %w", err)
	}
	if artifacts != nil {
		if _, err := agentAdapter.WithArtifacts(artifacts); err != nil {
			return nil, fmt.Errorf("configure Agent Artifact rpc adapter: %w", err)
		}
	}
	if toolAudits != nil {
		if _, err := agentAdapter.WithToolAudits(toolAudits); err != nil {
			return nil, fmt.Errorf("configure Agent Tool invocation audit rpc adapter: %w", err)
		}
	}
	if messageCommands != nil {
		if _, err := agentAdapter.WithMessageCommands(messageCommands); err != nil {
			return nil, fmt.Errorf("configure Agent Message Command rpc adapter: %w", err)
		}
	}
	if approvalGrants != nil {
		if _, err := agentAdapter.WithApprovalGrants(approvalGrants); err != nil {
			return nil, fmt.Errorf("configure Agent Approval grant rpc adapter: %w", err)
		}
	}
	if promotionControls != nil {
		if _, err := agentAdapter.WithPromotionControls(promotionControls); err != nil {
			return nil, fmt.Errorf("configure Agent Runtime promotion control rpc adapter: %w", err)
		}
	}
	if promotionEvidence != nil {
		if _, err := agentAdapter.WithPromotionEvidence(promotionEvidence); err != nil {
			return nil, fmt.Errorf("configure Agent Runtime promotion evidence review rpc adapter: %w", err)
		}
	}
	if subscriptions != nil {
		if _, err := agentAdapter.WithEventSubscriptions(subscriptions); err != nil {
			return nil, fmt.Errorf("configure Agent Event Subscription rpc adapter: %w", err)
		}
	}
	if subscriptionControls != nil {
		if _, err := agentAdapter.WithEventSubscriptionControls(subscriptionControls); err != nil {
			return nil, fmt.Errorf("configure Agent Event Subscription control rpc adapter: %w", err)
		}
	}
	if len(memories) > 1 {
		return nil, errors.New("at most one Agent Memory resolver may be configured")
	}
	if len(memories) == 1 && memories[0] != nil {
		if _, err := agentAdapter.WithMemories(memories[0]); err != nil {
			return nil, fmt.Errorf("configure Agent Memory rpc adapter: %w", err)
		}
	}
	return newCoreRPCServer(cfg, capability, agentAdapter)
}

func newCoreRPCServer(cfg config.InternalRPC, capability application.CoreCapability, agentAdapter *agentgrpc.Server) (*InternalRPCServer, error) {
	adapter, err := coregrpc.NewServer(capability)
	if err != nil {
		return nil, fmt.Errorf("create core rpc adapter: %w", err)
	}
	allowed := []string{messageServiceName, gatewayServiceName, searchServiceName, syncServiceName}
	if agentAdapter != nil {
		allowed = append(allowed, agentServiceName)
	}
	return newInternalRPCServer(cfg, cfg.CoreListenAddress, allowed, func(server *grpc.Server) {
		corev1.RegisterCoreCapabilityServiceServer(server, adapter)
		if agentAdapter != nil {
			agentv1.RegisterAgentCapabilityServiceServer(server, agentAdapter)
		}
	}, restrictCoreServiceMethods)
}

func DialSearchCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	return dialCoreCapabilityAs(ctx, cfg, searchServiceName)
}

func DialSyncCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	return dialCoreCapabilityAs(ctx, cfg, syncServiceName)
}

func DialCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	return dialCoreCapabilityAs(ctx, cfg, messageServiceName)
}

func DialGatewayCoreCapability(ctx context.Context, cfg config.InternalRPC) (*coregrpc.Client, *grpc.ClientConn, error) {
	return dialCoreCapabilityAs(ctx, cfg, gatewayServiceName)
}

func DialGatewayAgentCapability(ctx context.Context, cfg config.InternalRPC) (agentv1.AgentCapabilityServiceClient, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{Service: gatewayServiceName, Secret: cfg.SharedSecret})
	if err != nil {
		return nil, nil, fmt.Errorf("dial Gateway Agent capability: %w", err)
	}
	return agentv1.NewAgentCapabilityServiceClient(connection), connection, nil
}

func dialCoreCapabilityAs(ctx context.Context, cfg config.InternalRPC, callerService string) (*coregrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.CoreTarget, grpcauth.Credentials{
		Service: callerService,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial core rpc: %w", err)
	}
	client, err := coregrpc.NewClientForService(corev1.NewCoreCapabilityServiceClient(connection), callerService)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create core capability client: %w", err)
	}
	return client, connection, nil
}

func NewMessageRPCServer(cfg config.InternalRPC, messages application.MessageApplication) (*InternalRPCServer, error) {
	adapter, err := messagegrpc.NewServer(messages)
	if err != nil {
		return nil, fmt.Errorf("create message rpc adapter: %w", err)
	}
	return newInternalRPCServer(cfg, cfg.MessageListenAddress, []string{gatewayServiceName, coreServiceName}, func(server *grpc.Server) {
		messagev1.RegisterMessageServiceServer(server, adapter)
	})
}

func DialMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	return dialMessageApplicationAs(ctx, cfg, gatewayServiceName)
}

func DialCoreMessageApplication(ctx context.Context, cfg config.InternalRPC) (*messagegrpc.Client, *grpc.ClientConn, error) {
	return dialMessageApplicationAs(ctx, cfg, coreServiceName)
}

func dialMessageApplicationAs(ctx context.Context, cfg config.InternalRPC, callerService string) (*messagegrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.MessageTarget, grpcauth.Credentials{
		Service: callerService,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial message rpc: %w", err)
	}
	client, err := messagegrpc.NewClientForService(messagev1.NewMessageServiceClient(connection), callerService)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create message application client: %w", err)
	}
	return client, connection, nil
}

func NewSearchRPCServer(cfg config.InternalRPC, search application.SearchApplication) (*InternalRPCServer, error) {
	adapter, err := searchgrpc.NewServer(search)
	if err != nil {
		return nil, fmt.Errorf("create Search rpc adapter: %w", err)
	}
	return newInternalRPCServer(cfg, cfg.SearchListenAddress, []string{gatewayServiceName}, func(server *grpc.Server) {
		searchv1.RegisterSearchServiceServer(server, adapter)
	})
}

func DialSearchApplication(ctx context.Context, cfg config.InternalRPC) (*searchgrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.SearchTarget, grpcauth.Credentials{
		Service: gatewayServiceName,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial Search rpc: %w", err)
	}
	client, err := searchgrpc.NewClientForService(searchv1.NewSearchServiceClient(connection), gatewayServiceName)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create Search application client: %w", err)
	}
	return client, connection, nil
}

func NewSyncRPCServer(cfg config.InternalRPC, syncApplication application.SyncApplication) (*InternalRPCServer, error) {
	adapter, err := syncgrpc.NewServer(syncApplication)
	if err != nil {
		return nil, fmt.Errorf("create sync rpc adapter: %w", err)
	}
	return newInternalRPCServer(cfg, cfg.SyncListenAddress, []string{gatewayServiceName, coreServiceName}, func(server *grpc.Server) {
		syncv1.RegisterSyncQueryServiceServer(server, adapter)
	})
}

func DialSyncApplication(ctx context.Context, cfg config.InternalRPC) (*syncgrpc.Client, *grpc.ClientConn, error) {
	return dialSyncApplicationAs(ctx, cfg, gatewayServiceName)
}

func DialCoreSyncApplication(ctx context.Context, cfg config.InternalRPC) (*syncgrpc.Client, *grpc.ClientConn, error) {
	return dialSyncApplicationAs(ctx, cfg, coreServiceName)
}

func dialSyncApplicationAs(ctx context.Context, cfg config.InternalRPC, callerService string) (*syncgrpc.Client, *grpc.ClientConn, error) {
	connection, err := dialInternalRPC(ctx, cfg, cfg.SyncTarget, grpcauth.Credentials{
		Service: callerService,
		Secret:  cfg.SharedSecret,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("dial sync rpc: %w", err)
	}
	client, err := syncgrpc.NewClientForService(syncv1.NewSyncQueryServiceClient(connection), callerService)
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("create sync application client: %w", err)
	}
	return client, connection, nil
}

func restrictCoreServiceMethods(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	caller, _ := grpcauth.CallerService(ctx)
	if caller == agentServiceName &&
		info.FullMethod != agentv1.AgentCapabilityService_AdmitRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CompleteRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishRun_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_RequestApproval_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveApproval_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListConversations_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_AuthorizeTaskControl_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ResolveMcpContext_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_BeginMcpToolInvocation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_FinishMcpToolInvocation_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ProjectTaskWorkflowState_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListTaskWorkflowProjectionSnapshots_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_CreateArtifact_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_MatchEventSubscriptions_FullMethodName &&
		info.FullMethod != agentv1.AgentCapabilityService_ListContextMemories_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Agent service is not allowed to call this Core capability")
	}
	if caller == searchServiceName &&
		info.FullMethod != corev1.CoreCapabilityService_ListSearchConversationKeys_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Search service is not allowed to call this Core capability")
	}
	if caller == syncServiceName &&
		info.FullMethod != corev1.CoreCapabilityService_GetGroupMember_FullMethodName &&
		info.FullMethod != healthv1.Health_Check_FullMethodName {
		return nil, status.Error(codes.PermissionDenied, "Sync service is not allowed to call this Core capability")
	}
	return handler(ctx, request)
}

func newInternalRPCServer(cfg config.InternalRPC, address string, allowedCallers []string, register func(*grpc.Server), additionalInterceptors ...grpc.UnaryServerInterceptor) (*InternalRPCServer, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("internal rpc listen address is required")
	}
	interceptor, err := grpcauth.NewUnaryServerInterceptor(cfg.SharedSecret, allowedCallers...)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}
	interceptors := append([]grpc.UnaryServerInterceptor{interceptor}, additionalInterceptors...)
	options := []grpc.ServerOption{grpc.ChainUnaryInterceptor(interceptors...)}
	transportCredentials, err := internalRPCServerCredentials(cfg, address)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	if transportCredentials != nil {
		options = append(options, grpc.Creds(transportCredentials))
	}
	server := grpc.NewServer(options...)
	register(server)
	healthServer := newServingHealthServer()
	healthv1.RegisterHealthServer(server, healthServer)
	runtime := &InternalRPCServer{listener: listener, server: server, health: healthServer, done: make(chan struct{})}
	go func() {
		defer close(runtime.done)
		_ = server.Serve(listener)
	}()
	return runtime, nil
}

func dialInternalRPC(ctx context.Context, cfg config.InternalRPC, target string, serviceCredentials grpcauth.Credentials) (*grpc.ClientConn, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("internal rpc target is required")
	}
	interceptor, err := grpcauth.NewUnaryClientInterceptor(serviceCredentials)
	if err != nil {
		return nil, err
	}
	if cfg.DialTimeoutSeconds <= 0 {
		cfg.DialTimeoutSeconds = 5
	}
	transportCredentials, err := internalRPCClientCredentials(cfg, target)
	if err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.DialTimeoutSeconds)*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(
		dialCtx,
		target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithUnaryInterceptor(interceptor),
	)
	if err != nil {
		return nil, err
	}
	if _, err := healthv1.NewHealthClient(connection).Check(dialCtx, &healthv1.HealthCheckRequest{}); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("internal rpc health check failed: %w", err)
	}
	return connection, nil
}

func internalRPCServerCredentials(cfg config.InternalRPC, address string) (grpcCredentials.TransportCredentials, error) {
	if !cfg.TLSEnabled {
		if !isLoopbackAddress(address) {
			return nil, fmt.Errorf("plaintext internal rpc listener must use loopback address: %s", address)
		}
		return nil, nil
	}
	certificate, roots, err := loadMutualTLSIdentity(cfg)
	if err != nil {
		return nil, err
	}
	return grpcCredentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    roots,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

func internalRPCClientCredentials(cfg config.InternalRPC, target string) (grpcCredentials.TransportCredentials, error) {
	if !cfg.TLSEnabled {
		if !isLoopbackAddress(target) {
			return nil, fmt.Errorf("plaintext internal rpc target must use loopback address: %s", target)
		}
		return insecure.NewCredentials(), nil
	}
	certificate, roots, err := loadMutualTLSIdentity(cfg)
	if err != nil {
		return nil, err
	}
	return grpcCredentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      roots,
		ServerName:   cfg.TLSServerName,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

func loadMutualTLSIdentity(cfg config.InternalRPC) (tls.Certificate, *x509.CertPool, error) {
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load internal rpc certificate: %w", err)
	}
	caPEM, err := os.ReadFile(cfg.TLSCAFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read internal rpc ca: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, errors.New("internal rpc ca contains no certificates")
	}
	return certificate, roots, nil
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newServingHealthServer() *health.Server {
	server := health.NewServer()
	server.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	return server
}

func (s *InternalRPCServer) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *InternalRPCServer) SetServing(serving bool) {
	if s == nil || s.health == nil {
		return
	}
	status := healthv1.HealthCheckResponse_NOT_SERVING
	if serving {
		status = healthv1.HealthCheckResponse_SERVING
	}
	s.health.SetServingStatus("", status)
}

func (s *InternalRPCServer) Close(ctx context.Context) {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		stopped := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-ctx.Done():
			s.server.Stop()
			<-stopped
		}
		_ = s.listener.Close()
		<-s.done
	})
}
