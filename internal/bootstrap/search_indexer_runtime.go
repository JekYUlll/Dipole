package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/JekYUlll/Dipole/internal/config"
	elasticsearchdata "github.com/JekYUlll/Dipole/internal/data/elasticsearch"
	"github.com/JekYUlll/Dipole/internal/logger"
	platformKafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformObservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	searchprojector "github.com/JekYUlll/Dipole/internal/projector/search"
)

const searchIndexerServiceName = "dipole-search-indexer"

type SearchIndexerRuntime struct {
	client  *http.Client
	metrics *platformObservability.MetricsServer
}

func InitializeSearchIndexer(ctx context.Context) (*SearchIndexerRuntime, error) {
	elasticsearchCfg := config.ElasticsearchConfig()
	if !elasticsearchCfg.Enabled {
		return nil, fmt.Errorf("Search Indexer requires elasticsearch.enabled")
	}
	if !config.KafkaConfig().Enabled {
		return nil, fmt.Errorf("Search Indexer requires kafka.enabled")
	}
	if elasticsearchCfg.RequestTimeoutSeconds <= 0 {
		return nil, fmt.Errorf("Elasticsearch request timeout must be positive")
	}
	httpClient := &http.Client{Timeout: time.Duration(elasticsearchCfg.RequestTimeoutSeconds) * time.Second}
	runtime := &SearchIndexerRuntime{client: httpClient}
	cleanup := func() { runtime.Close() }
	index, err := elasticsearchdata.NewIndex(elasticsearchdata.Config{
		Address: elasticsearchCfg.Address, IndexPrefix: elasticsearchCfg.IndexPrefix,
		Shards: elasticsearchCfg.Shards, Replicas: elasticsearchCfg.Replicas,
		Username: elasticsearchCfg.Username, Password: elasticsearchCfg.Password, APIKey: elasticsearchCfg.APIKey,
		HTTPClient: httpClient,
	})
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := index.Bootstrap(ctx); err != nil {
		cleanup()
		return nil, fmt.Errorf("bootstrap Elasticsearch Search index: %w", err)
	}
	projector, err := searchprojector.New(index)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := platformKafka.Init(); err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize Search Indexer Kafka publisher: %w", err)
	}
	if err := platformKafka.InitConsumerForService(searchIndexerServiceName); err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize Search Indexer Kafka consumer: %w", err)
	}
	topics := searchprojector.Topics()
	for _, topic := range topics {
		platformKafka.Subscriber.Register(topic, projector.Handler())
	}
	if err := platformKafka.Client.EnsureTopics(topics); err != nil {
		cleanup()
		return nil, fmt.Errorf("ensure Search Indexer topics: %w", err)
	}
	if err := platformKafka.Subscriber.Start(ctx); err != nil {
		cleanup()
		return nil, fmt.Errorf("start Search Indexer consumer: %w", err)
	}
	runtime.metrics, err = startRuntimeMetrics(config.MetricsConfig(), searchIndexerServiceName, platformKafka.Subscriber)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("start Search Indexer metrics: %w", err)
	}
	if err := configureRuntimeDependencyReadiness(runtime.metrics, config.MetricsConfig(),
		elasticsearchReadinessProbe("elasticsearch", index), kafkaReadinessProbe("kafka", platformKafka.Client),
	); err != nil {
		cleanup()
		return nil, fmt.Errorf("configure Search Indexer dependency readiness: %w", err)
	}
	if runtime.metrics != nil {
		markRuntimeReady(runtime.metrics)
	}
	logger.Info("Search Indexer runtime initialized",
		zap.String("consumer", searchIndexerServiceName),
		zap.String("index", index.PhysicalIndex()),
		zap.Int("topics", len(topics)),
	)
	return runtime, nil
}

func (r *SearchIndexerRuntime) Close() {
	if r == nil {
		return
	}
	if err := closeRuntimeMetrics(r.metrics); err != nil {
		logger.Warn("Search Indexer metrics close failed", zap.Error(err))
	}
	if err := platformKafka.CloseConsumer(); err != nil {
		logger.Warn("Search Indexer Kafka consumer close failed", zap.Error(err))
	}
	if err := platformKafka.Close(); err != nil {
		logger.Warn("Search Indexer Kafka publisher close failed", zap.Error(err))
	}
	if r.client != nil {
		r.client.CloseIdleConnections()
		r.client = nil
	}
}
