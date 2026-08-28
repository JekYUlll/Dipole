package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/gateway"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	realtimeDelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	"github.com/JekYUlll/Dipole/internal/service"
	"github.com/JekYUlll/Dipole/internal/store"
	deliverygrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/delivery"
	agentv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/agent/v1"
	deliveryv1 "github.com/JekYUlll/Dipole/internal/transport/grpc/gen/delivery/v1"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type GatewayRuntime struct {
	server                 *gateway.Server
	router                 *wsTransport.PubSubRouter
	messageConn            *grpc.ClientConn
	coreConn               *grpc.ClientConn
	searchConn             *grpc.ClientConn
	redis                  *redis.Client
	metrics                *platformObservability.MetricsServer
	deliveryObservationRPC *InternalRPCServer
	deliveryObserver       *deliverygrpc.ShadowServer
	fenceHeartbeatCancel   context.CancelFunc
	fenceHeartbeatDone     chan struct{}
}

const (
	gatewayFenceObservationTTL = 15 * time.Second
	gatewayFenceHeartbeat      = 5 * time.Second
	gatewayFenceCheckTimeout   = 2 * time.Second
)

type gatewayObservationSink struct {
	batches     atomic.Uint64
	items       atomic.Uint64
	connections atomic.Uint64
}

type gatewayPrimaryDeliverySink struct {
	hub *wsTransport.Hub
}

func (s *gatewayPrimaryDeliverySink) Enqueue(ctx context.Context, request deliverygrpc.ConnectionDeliveryRequest) ([]deliverygrpc.ConnectionDeliveryOutcome, error) {
	results, err := s.hub.EnqueueEventToConnectionsContext(
		ctx, request.RecipientUserID, request.ConnectionIDs, request.DeliveryID,
		request.EventType, json.RawMessage(request.PayloadJSON),
	)
	if err != nil {
		return nil, err
	}
	outcomes := make([]deliverygrpc.ConnectionDeliveryOutcome, 0, len(results))
	for _, result := range results {
		status := deliverygrpc.ConnectionDeliveryStatusOffline
		switch result.Status {
		case wsTransport.ConnectionEnqueueStatusEnqueued:
			status = deliverygrpc.ConnectionDeliveryStatusEnqueued
		case wsTransport.ConnectionEnqueueStatusBackpressured:
			status = deliverygrpc.ConnectionDeliveryStatusBackpressured
		}
		outcomes = append(outcomes, deliverygrpc.ConnectionDeliveryOutcome{
			ConnectionID: result.ConnectionID, Status: status,
			QueueDepth: result.QueueDepth, QueueCapacity: result.QueueCapacity,
		})
	}
	return outcomes, nil
}

func (s *gatewayObservationSink) Observe(batch *deliveryv1.NodeDeliveryBatch) {
	s.batches.Add(1)
	s.items.Add(uint64(len(batch.GetItems())))
	for _, item := range batch.GetItems() {
		s.connections.Add(uint64(len(item.GetConnectionIds())))
	}
}

func InitializeGateway(ctx context.Context) (*GatewayRuntime, error) {
	rpcCfg := config.InternalRPCConfig()
	gatewayCfg := config.GatewayConfig()
	kafkaCfg := config.KafkaConfig()
	realtimeCfg := config.RealtimeConfig()
	deliveryAuthority, err := realtimeDelivery.ParseAuthority(realtimeCfg.Delivery)
	if err != nil {
		return nil, err
	}
	if err := deliveryAuthority.ValidateGatewayCapabilities(rpcCfg.DeliveryObservationEnabled, rpcCfg.DeliveryPrimaryEnabled); err != nil {
		return nil, err
	}
	if err := validateTimelineNotifyMode(config.MessageConfig()); err != nil {
		return nil, err
	}
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("gateway requires internal_rpc.enabled")
	}
	if gatewayCfg.Mode != "remote" {
		return nil, fmt.Errorf("gateway requires gateway.mode=remote")
	}
	if !kafkaCfg.Enabled {
		return nil, fmt.Errorf("gateway requires kafka.enabled for durable realtime delivery")
	}
	if err := store.InitRedis(); err != nil {
		return nil, fmt.Errorf("gateway redis init failed: %w", err)
	}
	runtime := &GatewayRuntime{redis: store.RDB}
	cleanup := func() { runtime.Close() }
	presence := platformPresence.NewRedisPresence()
	var deliveryFence realtimeDelivery.AuthorityFence
	var deliveryObservationFence realtimeDelivery.AuthorityFence
	if realtimeCfg.FencingEnabled {
		if presence == nil || presence.NodeID() == "" {
			cleanup()
			return nil, fmt.Errorf("realtime delivery fencing requires Redis Presence node identity")
		}
		reader, fenceErr := realtimeDelivery.NewRedisAuthorityFence(
			store.RDB, realtimeCfg.FencingKey, realtimeCfg.FencingEpoch, time.Now,
		)
		if fenceErr != nil {
			cleanup()
			return nil, fmt.Errorf("initialize realtime delivery authority fence: %w", fenceErr)
		}
		deliveryFence = reader
		deliveryObservationFence, fenceErr = realtimeDelivery.NewRedisObservedAuthorityFence(
			reader, store.RDB, realtimeCfg.FencingKey+":observation:", "gateway", presence.NodeID(),
			gatewayFenceObservationTTL, time.Now,
		)
		if fenceErr != nil {
			cleanup()
			return nil, fmt.Errorf("initialize realtime delivery authority observation: %w", fenceErr)
		}
		if err = deliveryObservationFence.Assert(ctx, deliveryAuthority); err != nil {
			cleanup()
			return nil, fmt.Errorf("verify realtime delivery authority fence: %w", err)
		}
		heartbeatCtx, heartbeatCancel := context.WithCancel(context.Background())
		runtime.fenceHeartbeatCancel = heartbeatCancel
		runtime.fenceHeartbeatDone = make(chan struct{})
		go func() {
			defer close(runtime.fenceHeartbeatDone)
			realtimeDelivery.RunAuthorityFenceHeartbeat(
				heartbeatCtx, deliveryObservationFence, deliveryAuthority,
				gatewayFenceHeartbeat, gatewayFenceCheckTimeout,
				func(err error) { logger.Warn("realtime delivery authority heartbeat denied", zap.Error(err)) },
			)
		}()
	}

	if err := platformKafka.Init(); err != nil {
		cleanup()
		return nil, fmt.Errorf("gateway kafka publisher init failed: %w", err)
	}
	if err := platformKafka.InitConsumerForService(gatewayServiceName); err != nil {
		cleanup()
		return nil, fmt.Errorf("gateway kafka consumer init failed: %w", err)
	}
	messages, messageConn, err := DialMessageApplication(ctx, rpcCfg)
	if err != nil {
		cleanup()
		return nil, err
	}
	runtime.messageConn = messageConn
	core, coreConn, err := DialGatewayCoreCapability(ctx, rpcCfg)
	if err != nil {
		cleanup()
		return nil, err
	}
	runtime.coreConn = coreConn
	var search application.SearchApplication
	if config.SearchConfig().Enabled {
		searchClient, searchConnection, err := DialSearchApplication(ctx, rpcCfg)
		if err != nil {
			cleanup()
			return nil, err
		}
		search = searchClient
		runtime.searchConn = searchConnection
	}
	var agentTasks gateway.AgentTaskControlApplication
	if gatewayCfg.AgentControlEnabled {
		agentTasks, err = gateway.NewAgentTaskControlClient(gatewayCfg.AgentControlTarget, rpcCfg.SharedSecret, time.Duration(rpcCfg.DialTimeoutSeconds)*time.Second)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("initialize Agent Task control client: %w", err)
		}
	}
	var agentMCP gateway.AgentMCPApplication
	if gatewayCfg.AgentMCPEnabled {
		agentMCP, err = gateway.NewAgentMCPProxy(gatewayCfg.AgentMCPTarget, rpcCfg.SharedSecret, service.AgentMCPResourceIdentifier())
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("initialize Agent MCP proxy: %w", err)
		}
	}
	var agentSubscriptions gateway.AgentSubscriptionControlApplication
	if gatewayCfg.AgentSubscriptionEnabled {
		agentSubscriptions, err = gateway.NewAgentSubscriptionControlClient(
			agentv1.NewAgentCapabilityServiceClient(coreConn), gatewayCfg.AgentSubscriptionTenantID,
			time.Duration(rpcCfg.DialTimeoutSeconds)*time.Second,
		)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("initialize Agent Subscription control client: %w", err)
		}
	}

	srv, err := gateway.NewServer(gatewayCfg.CoreHTTPTarget, gateway.Dependencies{
		Messages:           messages,
		Core:               core,
		Search:             search,
		AgentTasks:         agentTasks,
		AgentSubscriptions: agentSubscriptions,
		AgentMCP:           agentMCP,
		Presence:           wsTransport.NewRedisPresenceTracker(presence),
		Limiter:            platformRateLimit.NewLimiter(),
		AgentMCPLimiter:    platformRateLimit.NewLimiter(),
	})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize gateway server: %w", err)
	}
	runtime.server = srv
	if rpcCfg.DeliveryObservationEnabled || rpcCfg.DeliveryPrimaryEnabled {
		if presence == nil || presence.NodeID() == "" {
			cleanup()
			return nil, fmt.Errorf("delivery observation requires Redis Presence node identity")
		}
		runtime.deliveryObserver, err = deliverygrpc.NewShadowServer(
			presence.NodeID(), rpcCfg.DeliveryObservationCapacity,
			time.Duration(rpcCfg.DeliveryObservationRetryAfterMS)*time.Millisecond,
			&gatewayObservationSink{},
		)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("initialize delivery observation receiver: %w", err)
		}
		if rpcCfg.DeliveryPrimaryEnabled {
			primary, primaryErr := deliverygrpc.NewPrimaryDispatcher(
				presence.NodeID(), rpcCfg.DeliveryPrimaryReplayCapacity,
				time.Duration(rpcCfg.DeliveryObservationRetryAfterMS)*time.Millisecond,
				&gatewayPrimaryDeliverySink{hub: srv.WSHub()},
			)
			if primaryErr != nil {
				cleanup()
				return nil, fmt.Errorf("initialize primary delivery dispatcher: %w", primaryErr)
			}
			if primaryErr = runtime.deliveryObserver.EnablePrimary(primary); primaryErr != nil {
				cleanup()
				return nil, fmt.Errorf("enable primary delivery dispatcher: %w", primaryErr)
			}
		}
		runtime.deliveryObservationRPC, err = NewDeliveryObservationRPCServer(rpcCfg, runtime.deliveryObserver)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("start delivery observation rpc: %w", err)
		}
	}

	var eventSender kafkaWSEventSender = srv.WSHub()
	if config.PresenceConfig().Enabled && store.RDB != nil {
		runtime.router = wsTransport.NewPubSubRouter(srv.WSHub(), presence, store.RDB)
		if runtime.router != nil {
			runtime.router.Start()
			eventSender = runtime.router
		}
	}
	if err := RegisterGatewayKafkaHandlers(eventSender, deliveryAuthority, deliveryFence); err != nil {
		cleanup()
		return nil, fmt.Errorf("register gateway kafka handlers: %w", err)
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			cleanup()
			return nil, fmt.Errorf("start gateway kafka consumer: %w", err)
		}
	}
	authorityMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "dipole_realtime_delivery_authority",
		Help:        "Current Gateway realtime message delivery authority.",
		ConstLabels: prometheus.Labels{"authority": string(deliveryAuthority)},
	})
	authorityMetric.Set(1)
	runtime.metrics, err = startRuntimeMetrics(config.MetricsConfig(), gatewayServiceName, platformKafka.Subscriber, authorityMetric)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("start gateway metrics: %w", err)
	}
	readinessProbes := []platformObservability.DependencyProbe{
		redisReadinessProbe("redis", runtime.redis),
		grpcReadinessProbe("core-rpc", runtime.coreConn),
		grpcReadinessProbe("message-rpc", runtime.messageConn),
		kafkaReadinessProbe("kafka", platformKafka.Client),
		kafkaConsumerReadinessProbe("kafka-assignment", platformKafka.Subscriber),
	}
	if deliveryObservationFence != nil {
		readinessProbes = append(readinessProbes, authorityFenceReadinessProbe("delivery-authority", deliveryObservationFence, deliveryAuthority))
	}
	if err := configureRuntimeDependencyReadiness(runtime.metrics, config.MetricsConfig(), readinessProbes...); err != nil {
		cleanup()
		return nil, fmt.Errorf("configure Gateway dependency readiness: %w", err)
	}
	if runtime.metrics != nil {
		markRuntimeReady(runtime.metrics)
	}
	logger.Info("gateway runtime initialized",
		zap.String("core_http_target", gatewayCfg.CoreHTTPTarget),
		zap.Bool("kafka_enabled", kafkaCfg.Enabled),
		zap.String("kafka_consumer", gatewayServiceName),
		zap.String("realtime_delivery_authority", string(deliveryAuthority)),
	)
	return runtime, nil
}

func (r *GatewayRuntime) Server() *gateway.Server {
	if r == nil {
		return nil
	}
	return r.server
}

func (r *GatewayRuntime) Close() {
	if r == nil {
		return
	}
	if r.fenceHeartbeatCancel != nil {
		r.fenceHeartbeatCancel()
		if r.fenceHeartbeatDone != nil {
			<-r.fenceHeartbeatDone
		}
		r.fenceHeartbeatCancel = nil
		r.fenceHeartbeatDone = nil
	}
	if r.deliveryObservationRPC != nil {
		ctx, cancel := context.WithTimeout(context.Background(), GatewayShutdownTimeout())
		r.deliveryObservationRPC.Close(ctx)
		cancel()
		r.deliveryObservationRPC = nil
	}
	if r.deliveryObserver != nil {
		r.deliveryObserver.Close()
		r.deliveryObserver = nil
	}
	if err := closeRuntimeMetrics(r.metrics); err != nil {
		logger.Warn("gateway metrics close failed", zap.Error(err))
	}
	if err := platformKafka.CloseConsumer(); err != nil {
		logger.Warn("gateway kafka consumer close failed", zap.Error(err))
	}
	if err := platformKafka.Close(); err != nil {
		logger.Warn("gateway kafka publisher close failed", zap.Error(err))
	}
	if r.router != nil {
		r.router.Stop()
		r.router = nil
	}
	if r.messageConn != nil {
		_ = r.messageConn.Close()
		r.messageConn = nil
	}
	if r.coreConn != nil {
		_ = r.coreConn.Close()
		r.coreConn = nil
	}
	if r.searchConn != nil {
		_ = r.searchConn.Close()
		r.searchConn = nil
	}
	if r.redis != nil {
		_ = r.redis.Close()
		if store.RDB == r.redis {
			store.RDB = nil
		}
		r.redis = nil
	}
}

func RunGatewayServer(srv *gateway.Server, tlsCfg config.TLS) error {
	if !tlsCfg.Enabled {
		return srv.Run(config.Addr())
	}
	if err := ensureTLSFiles(tlsCfg); err != nil {
		return err
	}
	return srv.RunTLS(config.Addr(), tlsCfg.CertFile, tlsCfg.KeyFile)
}

func GatewayShutdownTimeout() time.Duration {
	seconds := config.InternalRPCConfig().ShutdownTimeoutSeconds
	if seconds <= 0 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}
