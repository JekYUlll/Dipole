package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	routingData "github.com/JekYUlll/Dipole/internal/data/routing"
	shadowData "github.com/JekYUlll/Dipole/internal/data/shadow"
	cassandraData "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	platformHotGroup "github.com/JekYUlll/Dipole/internal/platform/hotgroup"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	messageapplication "github.com/JekYUlll/Dipole/internal/services/message/application"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
	"github.com/JekYUlll/Dipole/internal/store"
	"github.com/apache/cassandra-gocql-driver/v2"
	"google.golang.org/grpc"
)

type MessageRuntime struct {
	rpc                *InternalRPCServer
	coreConn           *grpc.ClientConn
	outboxFlow         *outboxRelay
	shutdownSec        int
	metrics            *platformObservability.MetricsServer
	shadowStore        *shadowData.MessageStore
	readRouter         *routingData.MessageStore
	duplicateHydration *platformObservability.DuplicateHydrationCollector
	cassandra          *gocql.Session
}

func InitializeMessageService(ctx context.Context) (*MessageRuntime, error) {
	rpcCfg := config.InternalRPCConfig()
	messageCfg := config.MessageConfig()
	kafkaCfg := config.KafkaConfig()
	cassandraCfg := config.CassandraConfig()
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("message service requires internal_rpc.enabled")
	}
	if err := validateCassandraShadowConfig(messageCfg, cassandraCfg); err != nil {
		return nil, err
	}
	if err := store.InitMySQLWithConfig(config.MessageMySQLConfig()); err != nil {
		return nil, fmt.Errorf("message mysql init failed: %w", err)
	}
	if err := store.InitRedis(); err != nil {
		return nil, fmt.Errorf("message redis init failed: %w", err)
	}
	if messageCfg.RuntimeMode != "owner" && messageCfg.RuntimeMode != "shadow" {
		return nil, fmt.Errorf("unsupported message.runtime_mode %q", messageCfg.RuntimeMode)
	}
	if err := validateMessageInboxWriteMode(messageCfg, kafkaCfg, config.SyncConfig()); err != nil {
		return nil, err
	}
	if messageCfg.RuntimeMode == "owner" {
		if err := platformKafka.Init(); err != nil {
			return nil, fmt.Errorf("message kafka publisher init failed: %w", err)
		}
		if err := platformKafka.InitConsumer(); err != nil {
			return nil, fmt.Errorf("message kafka consumer init failed: %w", err)
		}
	}
	runner, err := migration.NewRunner(store.SQLDB, migrations.Files)
	if err != nil {
		return nil, fmt.Errorf("initialize message migration validation: %w", err)
	}
	if err := runner.ValidateCurrent(ctx); err != nil {
		return nil, fmt.Errorf("message database schema is not ready: %w", err)
	}
	if messageCfg.EnforceDBPermissions {
		if err := verifyMessageDatabaseBoundary(ctx, store.SQLDB, messageCfg.InboxWriteMode == "atomic"); err != nil {
			return nil, fmt.Errorf("verify message database permissions: %w", err)
		}
	}
	repos, err := messagemysql.NewProcessRepositories(store.SQLDB, messageCfg.InboxWriteMode == "atomic")
	if err != nil {
		return nil, fmt.Errorf("compose message repositories: %w", err)
	}
	core, coreConn, err := DialCoreCapability(ctx, rpcCfg)
	if err != nil {
		return nil, err
	}
	runtime := &MessageRuntime{coreConn: coreConn, shutdownSec: rpcCfg.ShutdownTimeoutSeconds}
	var duplicateHydrator applicationPort.SyncMessageHydrator
	if messageCfg.CassandraShadowReads || messageCfg.CassandraReadPercent > 0 || messageCfg.CassandraDuplicateHydration {
		runtime.cassandra, err = cassandraData.OpenSession(cassandraCfg)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("open Cassandra shadow-read session: %w", err)
		}
		if err := cassandraData.ValidateTimelineSchema(ctx, runtime.cassandra, cassandraCfg.Keyspace); err != nil {
			runtime.Close()
			return nil, err
		}
		timeline, err := cassandraData.NewTimelineStore(runtime.cassandra, cassandraCfg.TimelineBucketSize)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("create Cassandra shadow timeline reader: %w", err)
		}
		if messageCfg.CassandraDuplicateHydration {
			runtime.duplicateHydration = platformObservability.NewDuplicateHydrationCollector()
			duplicateHydrator, err = cassandraData.NewSyncMessageHydrator(timeline)
			if err != nil {
				runtime.Close()
				return nil, fmt.Errorf("create Cassandra duplicate-message hydrator: %w", err)
			}
		}
		if messageCfg.CassandraShadowReads {
			runtime.shadowStore = shadowData.NewMessageStore(repos.Messages, timeline, nil)
			repos.Messages = runtime.shadowStore
		} else if messageCfg.CassandraReadPercent > 0 {
			runtime.readRouter = routingData.NewMessageStoreWithVerification(
				repos.Messages, repos.ConversationSequence, timeline,
				messageCfg.CassandraReadPercent, messageCfg.CassandraReadVerifyPercent, nil,
			)
			repos.Messages = runtime.readRouter
		}
	}

	var events applicationPort.EventPublisher
	if platformKafka.Client != nil {
		events = platformKafka.Client
	}
	var duplicateObserver func(string)
	if runtime.duplicateHydration != nil {
		duplicateObserver = runtime.duplicateHydration.Observe
	}
	messages := messageapplication.New(repos.Messages, core, messageapplication.Dependencies{
		Events: events, HotGroups: platformHotGroup.NewRedisDetector(), DuplicateHydrator: duplicateHydrator,
		DuplicateHydrationObserver: duplicateObserver,
	})
	servedMessages := applicationPort.MessageApplication(messages)
	if messageCfg.RuntimeMode == "shadow" {
		servedMessages = newQueryOnlyMessageApplication(messages)
	}
	if messageCfg.RuntimeMode == "owner" {
		RegisterMessageKafkaHandlers(messages)
	}
	if messageCfg.RuntimeMode == "owner" && platformKafka.Client != nil {
		if err := platformKafka.Client.EnsureTopics(messageOwnedKafkaTopics()); err != nil {
			runtime.Close()
			return nil, fmt.Errorf("ensure message kafka topics: %w", err)
		}
	}
	if messageCfg.RuntimeMode == "owner" && platformKafka.Subscriber != nil {
		if err := platformKafka.Subscriber.Start(ctx); err != nil {
			runtime.Close()
			return nil, fmt.Errorf("start message kafka consumer: %w", err)
		}
	}
	if messageCfg.RuntimeMode == "owner" && platformKafka.Client != nil {
		runtime.outboxFlow = newOutboxRelay(repos.Outbox)
		if runtime.outboxFlow != nil {
			runtime.outboxFlow.Start()
		}
	}
	runtime.metrics, err = startRuntimeMetrics(config.MetricsConfig(), messageServiceName, platformKafka.Subscriber, runtime.readRouter, runtime.duplicateHydration)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start message metrics: %w", err)
	}
	readinessProbes := []platformObservability.DependencyProbe{
		mysqlReadinessProbe("mysql", store.SQLDB),
		grpcReadinessProbe("core-rpc", runtime.coreConn),
	}
	if messageCfg.RuntimeMode == "owner" {
		readinessProbes = append(readinessProbes, kafkaReadinessProbe("kafka", platformKafka.Client))
	}
	if err := configureRuntimeDependencyReadiness(runtime.metrics, config.MetricsConfig(), readinessProbes...); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("configure Message dependency readiness: %w", err)
	}
	runtime.rpc, err = NewMessageRPCServer(rpcCfg, servedMessages)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start message rpc server: %w", err)
	}
	if runtime.metrics != nil {
		bindRPCReadiness(runtime.metrics, runtime.rpc)
		markRuntimeReady(runtime.metrics)
	}
	return runtime, nil
}

func validateMessageInboxWriteMode(messageCfg config.Message, kafkaCfg config.Kafka, syncCfg config.Sync) error {
	if messageCfg.InboxWriteMode != "atomic" && messageCfg.InboxWriteMode != "projector" {
		return fmt.Errorf("unsupported message.inbox_write_mode %q", messageCfg.InboxWriteMode)
	}
	if messageCfg.InboxWriteMode == "projector" && messageCfg.RuntimeMode != "owner" {
		return fmt.Errorf("message.inbox_write_mode projector requires message.runtime_mode owner")
	}
	if messageCfg.InboxWriteMode == "projector" && !kafkaCfg.Enabled {
		return fmt.Errorf("message.inbox_write_mode projector requires kafka.enabled")
	}
	if messageCfg.InboxWriteMode == "projector" && !syncCfg.ProjectorEnabled {
		return fmt.Errorf("message.inbox_write_mode projector requires sync.projector_enabled")
	}
	return nil
}

func messageOwnedKafkaTopics() []string {
	return []string{
		"message.direct.send_requested",
		"message.direct.created",
		"message.group.send_requested",
		"message.group.created",
	}
}

func validateCassandraShadowConfig(messageCfg config.Message, cassandraCfg config.Cassandra) error {
	if messageCfg.CassandraReadPercent < 0 || messageCfg.CassandraReadPercent > 100 {
		return fmt.Errorf("message.cassandra_read_percentage must be between 0 and 100")
	}
	if messageCfg.CassandraReadVerifyPercent < 0 || messageCfg.CassandraReadVerifyPercent > 100 {
		return fmt.Errorf("message.cassandra_read_verify_percentage must be between 0 and 100")
	}
	if messageCfg.CassandraReadVerifyPercent > 0 && messageCfg.CassandraReadPercent == 0 {
		return fmt.Errorf("Cassandra read verification requires a positive primary-read cohort")
	}
	if messageCfg.CassandraShadowReads && messageCfg.CassandraReadPercent > 0 {
		return fmt.Errorf("Cassandra shadow reads and primary-read cohorts cannot be enabled together")
	}
	if (messageCfg.CassandraShadowReads || messageCfg.CassandraReadPercent > 0 || messageCfg.CassandraDuplicateHydration) && !cassandraCfg.Enabled {
		return fmt.Errorf("Cassandra Message reads require cassandra.enabled")
	}
	return nil
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
	_ = closeRuntimeMetrics(r.metrics)
	shutdownSec := r.shutdownSec
	if shutdownSec <= 0 {
		shutdownSec = 15
	}
	if r.rpc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSec)*time.Second)
		r.rpc.Close(ctx)
		cancel()
	}
	if r.shadowStore != nil {
		r.shadowStore.Wait()
	}
	if r.cassandra != nil {
		r.cassandra.Close()
		r.cassandra = nil
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
