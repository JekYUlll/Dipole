package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/JekYUlll/Dipole/internal/config"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
)

type readinessValidator interface {
	ValidateReadiness(context.Context) error
}

func configureRuntimeDependencyReadiness(
	server *platformobservability.MetricsServer,
	cfg config.Metrics,
	probes ...platformobservability.DependencyProbe,
) error {
	if !cfg.DependencyProbesEnabled {
		return nil
	}
	if server == nil {
		return fmt.Errorf("dependency readiness requires metrics.enabled")
	}
	policy := platformobservability.DependencyReadinessPolicy{
		Interval:         time.Duration(cfg.DependencyProbeIntervalSeconds) * time.Second,
		Timeout:          time.Duration(cfg.DependencyProbeTimeoutMS) * time.Millisecond,
		FailureThreshold: cfg.DependencyFailureThreshold,
		SuccessThreshold: cfg.DependencySuccessThreshold,
	}
	return server.MonitorDependencies(probes, policy)
}

func bindRPCReadiness(server *platformobservability.MetricsServer, rpc *InternalRPCServer) {
	if server == nil || rpc == nil {
		return
	}
	server.OnReadinessChange(rpc.SetServing)
}

func mysqlReadinessProbe(name string, db *sql.DB) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: db.PingContext}
}

func redisReadinessProbe(name string, client *redis.Client) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	}}
}

func kafkaReadinessProbe(name string, publisher *platformkafka.Publisher) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: publisher.Ping}
}

func grpcReadinessProbe(name string, connection *grpc.ClientConn) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		response, err := healthv1.NewHealthClient(connection).Check(ctx, &healthv1.HealthCheckRequest{})
		if err != nil {
			return err
		}
		if response.GetStatus() != healthv1.HealthCheckResponse_SERVING {
			return fmt.Errorf("gRPC dependency %s is %s", name, response.GetStatus())
		}
		return nil
	}}
}

func elasticsearchReadinessProbe(name string, validator readinessValidator) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: validator.ValidateReadiness}
}

func cassandraReadinessProbe(name string, session *gocql.Session, keyspace string) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		return cassandradata.ValidateTimelineSchema(ctx, session, keyspace)
	}}
}
