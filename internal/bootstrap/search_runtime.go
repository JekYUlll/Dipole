package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/logger"
	elasticsearchdata "github.com/JekYUlll/Dipole/internal/platform/elasticsearch"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	platformruntime "github.com/JekYUlll/Dipole/internal/platform/runtime"
	searchapplication "github.com/JekYUlll/Dipole/internal/services/search/application"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type SearchRuntime struct {
	rpc         *InternalRPCServer
	coreConn    *grpc.ClientConn
	httpClient  *http.Client
	metrics     *platformobservability.MetricsServer
	shutdownSec int
}

func InitializeSearchService(ctx context.Context) (*SearchRuntime, error) {
	return initializeSearchService(ctx, config.InternalRPCConfig(), config.ElasticsearchConfig(), config.MetricsConfig())
}

func initializeSearchService(ctx context.Context, rpcCfg config.InternalRPC, elasticsearchCfg config.Elasticsearch, metricsCfg config.Metrics) (*SearchRuntime, error) {
	if !rpcCfg.Enabled {
		return nil, fmt.Errorf("Search Service requires internal_rpc.enabled")
	}
	if !elasticsearchCfg.Enabled {
		return nil, fmt.Errorf("Search Service requires elasticsearch.enabled")
	}
	if elasticsearchCfg.RequestTimeoutSeconds <= 0 {
		return nil, fmt.Errorf("Elasticsearch request timeout must be positive")
	}
	httpClient := &http.Client{Timeout: time.Duration(elasticsearchCfg.RequestTimeoutSeconds) * time.Second}
	runtime := &SearchRuntime{httpClient: httpClient, shutdownSec: rpcCfg.ShutdownTimeoutSeconds}
	index, err := elasticsearchdata.NewIndex(elasticsearchdata.Config{
		Address: elasticsearchCfg.Address, IndexPrefix: elasticsearchCfg.IndexPrefix,
		Shards: elasticsearchCfg.Shards, Replicas: elasticsearchCfg.Replicas,
		Username: elasticsearchCfg.Username, Password: elasticsearchCfg.Password, APIKey: elasticsearchCfg.APIKey,
		HTTPClient: httpClient,
	})
	if err != nil {
		runtime.Close()
		return nil, err
	}
	if err := index.ValidateReadiness(ctx); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("validate Elasticsearch Search readiness: %w", err)
	}
	core, coreConnection, err := DialSearchCoreCapability(ctx, rpcCfg)
	if err != nil {
		runtime.Close()
		return nil, err
	}
	runtime.coreConn = coreConnection
	search, err := searchapplication.NewSearchApplication(core, index)
	if err != nil {
		runtime.Close()
		return nil, err
	}
	runtime.metrics, err = platformruntime.StartMetrics(metricsCfg, searchServiceName, nil)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Search Service metrics: %w", err)
	}
	if err := configureRuntimeDependencyReadiness(runtime.metrics, metricsCfg,
		elasticsearchReadinessProbe("elasticsearch", index), grpcReadinessProbe("core-rpc", runtime.coreConn),
	); err != nil {
		runtime.Close()
		return nil, fmt.Errorf("configure Search dependency readiness: %w", err)
	}
	runtime.rpc, err = NewSearchRPCServer(rpcCfg, search)
	if err != nil {
		runtime.Close()
		return nil, fmt.Errorf("start Search rpc server: %w", err)
	}
	if runtime.metrics != nil {
		bindRPCReadiness(runtime.metrics, runtime.rpc)
		platformruntime.MarkReady(runtime.metrics)
	}
	logger.Info("Search Service runtime initialized", zap.String("read_alias", index.ReadAlias()))
	return runtime, nil
}

func (r *SearchRuntime) Address() string {
	if r == nil || r.rpc == nil {
		return ""
	}
	return r.rpc.Address()
}

func (r *SearchRuntime) Close() {
	if r == nil {
		return
	}
	if err := platformruntime.CloseMetrics(r.metrics); err != nil {
		logger.Warn("Search Service metrics close failed", zap.Error(err))
	}
	r.metrics = nil
	shutdownSec := r.shutdownSec
	if shutdownSec <= 0 {
		shutdownSec = 15
	}
	if r.rpc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSec)*time.Second)
		r.rpc.Close(ctx)
		cancel()
		r.rpc = nil
	}
	if r.coreConn != nil {
		_ = r.coreConn.Close()
		r.coreConn = nil
	}
	if r.httpClient != nil {
		r.httpClient.CloseIdleConnections()
		r.httpClient = nil
	}
}
