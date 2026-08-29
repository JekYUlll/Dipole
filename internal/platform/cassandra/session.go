package cassandra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"

	"github.com/JekYUlll/Dipole/internal/config"
)

func OpenSession(cfg config.Cassandra) (*gocql.Session, error) {
	hosts := normalizedHosts(cfg.Hosts)
	keyspace := strings.TrimSpace(cfg.Keyspace)
	localDC := strings.TrimSpace(cfg.LocalDatacenter)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("Cassandra hosts are required")
	}
	if keyspace == "" {
		return nil, fmt.Errorf("Cassandra keyspace is required")
	}
	if localDC == "" {
		return nil, fmt.Errorf("Cassandra local datacenter is required")
	}

	timeout := time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.LocalQuorum
	cluster.SerialConsistency = gocql.LocalSerial
	cluster.Timeout = timeout
	cluster.ConnectTimeout = timeout
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.DCAwareRoundRobinPolicy(localDC))

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect Cassandra keyspace %s: %w", keyspace, err)
	}
	return session, nil
}

func ValidateTimelineSchema(ctx context.Context, session *gocql.Session, keyspace string) error {
	if session == nil {
		return fmt.Errorf("Cassandra session is required")
	}
	keyspace = strings.TrimSpace(keyspace)
	var tableName string
	err := session.Query(`
SELECT table_name FROM system_schema.tables
WHERE keyspace_name = ? AND table_name = ?`, keyspace, TimelineTableName).WithContext(ctx).Scan(&tableName)
	if err != nil {
		return fmt.Errorf("Cassandra timeline schema is not ready: %w", err)
	}
	if tableName != TimelineTableName {
		return fmt.Errorf("Cassandra timeline schema is not ready: table %s is missing", TimelineTableName)
	}
	return nil
}

func normalizedHosts(hosts []string) []string {
	normalized := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host = strings.TrimSpace(host); host != "" {
			normalized = append(normalized, host)
		}
	}
	return normalized
}
