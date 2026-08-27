package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/gateway"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformPresence "github.com/JekYUlll/Dipole/internal/platform/presence"
	platformRateLimit "github.com/JekYUlll/Dipole/internal/platform/ratelimit"
	"github.com/JekYUlll/Dipole/internal/service"
	"github.com/JekYUlll/Dipole/internal/store"
	wsTransport "github.com/JekYUlll/Dipole/internal/transport/ws"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type GatewayRuntime struct {
	server      *gateway.Server
	router      *wsTransport.PubSubRouter
	messageConn *grpc.ClientConn
	coreConn    *grpc.ClientConn
	searchConn  *grpc.ClientConn
	redis       *redis.Client
	metrics     *platformObservability.MetricsServer
}

func InitializeGateway(ctx context.Context) (*GatewayRuntime, error) {
	rpcCfg := config.InternalRPCConfig()
	gatewayCfg := config.GatewayConfig()
	kafkaCfg := config.KafkaConfig()
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

	presence := platformPresence.NewRedisPresence()
	srv, err := gateway.NewServer(gatewayCfg.CoreHTTPTarget, gateway.Dependencies{
		Messages:        messages,
		Core:            core,
		Search:          search,
		AgentTasks:      agentTasks,
		AgentMCP:        agentMCP,
		Presence:        wsTransport.NewRedisPresenceTracker(presence),
		Limiter:         platformRateLimit.NewLimiter(),
		AgentMCPLimiter: platformRateLimit.NewLimiter(),
	})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize gateway server: %w", err)
	}
	runtime.server = srv

	var eventSender kafkaWSEventSender = srv.WSHub()
	if config.PresenceConfig().Enabled && store.RDB != nil {
		runtime.router = wsTransport.NewPubSubRouter(srv.WSHub(), presence, store.RDB)
		if runtime.router != nil {
			runtime.router.Start()
			eventSender = runtime.router
		}
	}
	if err := RegisterGatewayKafkaHandlers(eventSender); err != nil {
		cleanup()
		return nil, fmt.Errorf("register gateway kafka handlers: %w", err)
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			cleanup()
			return nil, fmt.Errorf("start gateway kafka consumer: %w", err)
		}
	}
	runtime.metrics, err = startRuntimeMetrics(config.MetricsConfig(), gatewayServiceName, platformKafka.Subscriber)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("start gateway metrics: %w", err)
	}
	if err := configureRuntimeDependencyReadiness(runtime.metrics, config.MetricsConfig(),
		redisReadinessProbe("redis", runtime.redis),
		grpcReadinessProbe("core-rpc", runtime.coreConn),
		grpcReadinessProbe("message-rpc", runtime.messageConn),
		kafkaReadinessProbe("kafka", platformKafka.Client),
	); err != nil {
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
