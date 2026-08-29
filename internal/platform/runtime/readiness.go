package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	cassandradata "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	platformobservability "github.com/JekYUlll/Dipole/internal/platform/observability"
	realtimedelivery "github.com/JekYUlll/Dipole/internal/realtime/delivery"
	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type ServingController interface {
	SetServing(bool)
}

type ReadinessValidator interface {
	ValidateReadiness(context.Context) error
}

func ConfigureDependencyReadiness(
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

func BindRPCReadiness(server *platformobservability.MetricsServer, controller ServingController) {
	if server == nil || controller == nil {
		return
	}
	server.OnReadinessChange(controller.SetServing)
}

func MySQLReadinessProbe(name string, db *sql.DB) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: db.PingContext}
}

func RedisReadinessProbe(name string, client *redis.Client) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	}}
}

func KafkaReadinessProbe(name string, publisher *platformkafka.Publisher) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: publisher.Ping}
}

func KafkaConsumerReadinessProbe(name string, consumer *platformkafka.Consumer) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, RequireInitialSuccess: true, Check: consumer.ValidateReadiness}
}

func AuthorityFenceReadinessProbe(name string, fence realtimedelivery.AuthorityFence, authority realtimedelivery.Authority) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		if fence == nil {
			return fmt.Errorf("delivery authority fence is unavailable")
		}
		return fence.Assert(ctx, authority)
	}}
}

func GRPCReadinessProbe(name string, connection *grpc.ClientConn) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		response, err := grpc_health_v1.NewHealthClient(connection).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			return err
		}
		if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
			return fmt.Errorf("gRPC dependency %s is %s", name, response.GetStatus())
		}
		return nil
	}}
}

func ElasticsearchReadinessProbe(name string, validator ReadinessValidator) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: validator.ValidateReadiness}
}

func CassandraReadinessProbe(name string, session *gocql.Session, keyspace string) platformobservability.DependencyProbe {
	return platformobservability.DependencyProbe{Name: name, Check: func(ctx context.Context) error {
		return cassandradata.ValidateTimelineSchema(ctx, session, keyspace)
	}}
}
