package bootstrap

import (
	"context"
	"fmt"

	"github.com/apache/cassandra-gocql-driver/v2"
	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/config"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	cassandraprojector "github.com/JekYUlll/Dipole/internal/projector/cassandra"
)

const cassandraProjectorServiceName = "dipole-cassandra-projector"

type CassandraProjectorRuntime struct {
	session *gocql.Session
	metrics *platformObservability.MetricsServer
}

func InitializeCassandraProjector(ctx context.Context) (*CassandraProjectorRuntime, error) {
	cassandraCfg := config.CassandraConfig()
	if !cassandraCfg.Enabled {
		return nil, fmt.Errorf("Cassandra projector requires cassandra.enabled")
	}
	kafkaCfg := config.KafkaConfig()
	if !kafkaCfg.Enabled {
		return nil, fmt.Errorf("Cassandra projector requires kafka.enabled")
	}

	session, err := cassandradata.OpenSession(cassandraCfg)
	if err != nil {
		return nil, err
	}
	runtime := &CassandraProjectorRuntime{session: session}
	cleanup := func() { runtime.Close() }
	if err := cassandradata.ValidateTimelineSchema(ctx, session, cassandraCfg.Keyspace); err != nil {
		cleanup()
		return nil, err
	}
	timeline, err := cassandradata.NewTimelineStore(session, cassandraCfg.TimelineBucketSize)
	if err != nil {
		cleanup()
		return nil, err
	}
	projector, err := cassandraprojector.New(timeline)
	if err != nil {
		cleanup()
		return nil, err
	}

	if err := platformKafka.Init(); err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize Cassandra projector Kafka publisher: %w", err)
	}
	if err := platformKafka.InitConsumerForService(cassandraProjectorServiceName); err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize Cassandra projector Kafka consumer: %w", err)
	}
	topics := []string{"message.direct.created", "message.group.created"}
	for _, topic := range topics {
		platformKafka.Subscriber.Register(topic, projector.Handler())
	}
	if err := platformKafka.Client.EnsureTopics(topics); err != nil {
		cleanup()
		return nil, fmt.Errorf("ensure Cassandra projector topics: %w", err)
	}
	if err := platformKafka.Subscriber.Start(ctx); err != nil {
		cleanup()
		return nil, fmt.Errorf("start Cassandra projector consumer: %w", err)
	}
	runtime.metrics, err = startRuntimeMetrics(config.MetricsConfig(), platformKafka.Subscriber)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("start Cassandra projector metrics: %w", err)
	}
	logger.Info("Cassandra projector runtime initialized",
		zap.String("consumer", cassandraProjectorServiceName),
		zap.String("keyspace", cassandraCfg.Keyspace),
		zap.Uint64("bucket_size", cassandraCfg.TimelineBucketSize),
	)
	return runtime, nil
}

func (r *CassandraProjectorRuntime) Close() {
	if r == nil {
		return
	}
	if err := closeRuntimeMetrics(r.metrics); err != nil {
		logger.Warn("Cassandra projector metrics close failed", zap.Error(err))
	}
	if err := platformKafka.CloseConsumer(); err != nil {
		logger.Warn("Cassandra projector Kafka consumer close failed", zap.Error(err))
	}
	if err := platformKafka.Close(); err != nil {
		logger.Warn("Cassandra projector Kafka publisher close failed", zap.Error(err))
	}
	if r.session != nil {
		r.session.Close()
		r.session = nil
	}
}
