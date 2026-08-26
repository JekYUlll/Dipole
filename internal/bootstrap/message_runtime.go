package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	appComposition "github.com/JekYUlll/Dipole/internal/app"
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformStorage "github.com/JekYUlll/Dipole/internal/platform/storage"
	"github.com/JekYUlll/Dipole/internal/store"
	"google.golang.org/grpc"
)

type MessageRuntime struct {
	rpc         *InternalRPCServer
	coreConn    *grpc.ClientConn
	outboxFlow  *outboxRelay
	shutdownSec int
}

func InitializeMessageService(ctx context.Context) (*MessageRuntime, error) {
	rpcCfg := config.InternalRPCConfig()
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("message service requires internal_rpc.enabled")
	}
	if err := store.InitMySQL(); err != nil {
		return nil, fmt.Errorf("message mysql init failed: %w", err)
	}
	if err := store.InitRedis(); err != nil {
		return nil, fmt.Errorf("message redis init failed: %w", err)
	}
	if err := platformKafka.Init(); err != nil {
		return nil, fmt.Errorf("message kafka publisher init failed: %w", err)
	}
	if err := platformKafka.InitConsumer(); err != nil {
		return nil, fmt.Errorf("message kafka consumer init failed: %w", err)
	}
	if err := platformStorage.Init(); err != nil {
		return nil, fmt.Errorf("message storage init failed: %w", err)
	}

	runner, err := migration.NewRunner(store.SQLDB, migrations.Files)
	if err != nil {
		return nil, fmt.Errorf("initialize message migration validation: %w", err)
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		return nil, fmt.Errorf("message database schema is not ready: %w", err)
	}
	repos, err := appComposition.NewRepositories(store.SQLDB)
	if err != nil {
		return nil, fmt.Errorf("compose message repositories: %w", err)
	}
	core, coreConn, err := DialCoreCapability(ctx, rpcCfg)
	if err != nil {
		return nil, err
	}
	runtime := &MessageRuntime{coreConn: coreConn, shutdownSec: rpcCfg.ShutdownTimeoutSeconds}

	var events applicationPort.EventPublisher
	if platformKafka.Client != nil {
		events = platformKafka.Client
	}
	messaging := appComposition.NewMessagingServices(repos, appComposition.MessagingDependencies{
		Core:      core,
		Events:    events,
		HotGroups: platformHotGroup.NewRedisDetector(),
		Storage:   platformStorage.Client,
	})
	RegisterMessageKafkaHandlers(messaging.Messages)
	if platformKafka.Client != nil {
		if err := platformKafka.Client.EnsureTopics(messageOwnedKafkaTopics()); err != nil {
			runtime.Close()
			return nil, fmt.Errorf("ensure message kafka topics: %w", err)
		}
	}
	if platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			runtime.Close()
			return nil, fmt.Errorf("start message kafka consumer: %w", err)
		}
	}
	if platformKafka.Client != nil {
		runtime.outboxFlow = newOutboxRelay(repos.Outbox)
		if runtime.outboxFlow != nil {
			runtime.outboxFlow.Start()
		}
	}
	runtime.rpc, err = NewMessageRPCServer(rpcCfg, messaging.Messages)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start message rpc server: %w", err)
	}
	return runtime, nil
}

func messageOwnedKafkaTopics() []string {
	return []string{
		"message.direct.send_requested",
		"message.direct.created",
		"message.group.send_requested",
		"message.group.created",
	}
}

func (r *MessageRuntime) Address() string {
	if r == nil || r.rpc == nil {
		return ""
	}
	return r.rpc.Address()
}

func (r *MessageRuntime) Close() {
	if r == nil {
		return
	}
	shutdownSec := r.shutdownSec
	if shutdownSec <= 0 {
		shutdownSec = 15
	}
	if r.rpc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSec)*time.Second)
		r.rpc.Close(ctx)
		cancel()
	}
	if err := platformKafka.CloseConsumer(); err != nil {
		_ = err
	}
	if r.outboxFlow != nil {
		r.outboxFlow.Stop()
	}
	if err := platformKafka.Close(); err != nil {
		_ = err
	}
	if r.coreConn != nil {
		_ = r.coreConn.Close()
	}
	if store.RDB != nil {
		_ = store.RDB.Close()
		store.RDB = nil
	}
	if store.SQLDB != nil {
		_ = store.SQLDB.Close()
		store.SQLDB = nil
	}
}
