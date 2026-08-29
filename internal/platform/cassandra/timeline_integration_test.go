package cassandra

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"

	cassandraschema "github.com/JekYUlll/Dipole/db/cassandra"
)

func TestTimelineStoreContract(t *testing.T) {
	hosts := splitContactPoints(os.Getenv("DIPOLE_TEST_CASSANDRA_HOSTS"))
	if len(hosts) == 0 {
		t.Skip("DIPOLE_TEST_CASSANDRA_HOSTS is required for Cassandra integration tests")
	}

	systemSession := openCassandraSession(t, hosts, "system")
	statements, err := cassandraschema.Statements()
	if err != nil {
		t.Fatalf("load Cassandra timeline schema: %v", err)
	}
	for _, statement := range statements {
		if err := systemSession.Query(statement).Exec(); err != nil {
			t.Fatalf("apply Cassandra timeline schema: %v", err)
		}
	}
	systemSession.Close()

	session := openCassandraSession(t, hosts, "dipole_message_shadow")
	t.Cleanup(func() { session.Close() })
	if err := ValidateTimelineSchema(context.Background(), session, "dipole_message_shadow"); err != nil {
		t.Fatalf("validate Cassandra timeline schema: %v", err)
	}
	if err := session.Query("TRUNCATE timeline_by_conversation_bucket").Exec(); err != nil {
		t.Fatalf("truncate Cassandra timeline: %v", err)
	}

	store, err := NewTimelineStore(session, DefaultTimelineBucketSize)
	if err != nil {
		t.Fatalf("create Cassandra timeline store: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	first := validProjection()
	result, err := store.Append(ctx, first)
	if err != nil || !result.Inserted || result.Duplicate {
		t.Fatalf("append first projection: result=%+v err=%v", result, err)
	}
	record, found, err := store.Lookup(ctx, first.ConversationKey, first.MessageSeq)
	if err != nil || !found || record.Projection.Content != first.Content {
		t.Fatalf("lookup first projection: record=%+v found=%v err=%v", record, found, err)
	}
	if record.Projection.FileExpiresAt != nil {
		t.Fatalf("lookup nullable file expiry=%v", record.Projection.FileExpiresAt)
	}
	expectedHash, err := first.PayloadHash()
	if err != nil || record.PayloadHash != expectedHash {
		t.Fatalf("lookup payload hash=%s expected=%s err=%v", record.PayloadHash, expectedHash, err)
	}
	if _, found, err := store.Lookup(ctx, first.ConversationKey, 2); err != nil || found {
		t.Fatalf("lookup missing projection: found=%v err=%v", found, err)
	}

	replay := first
	replay.EventID = "E-REPLAY"
	result, err = store.Append(ctx, replay)
	if err != nil || result.Inserted || !result.Duplicate {
		t.Fatalf("append duplicate projection: result=%+v err=%v", result, err)
	}

	conflict := first
	conflict.EventID = "E-CONFLICT"
	conflict.Content = "conflicting payload"
	if _, err := store.Append(ctx, conflict); !errors.Is(err, ErrProjectionConflict) {
		t.Fatalf("expected projection conflict, got %v", err)
	}

	for _, sequence := range []uint64{10_000, 10_001} {
		projection := first
		projection.EventID = "E" + time.Now().Format("150405.000000000")
		projection.MessageSeq = sequence
		projection.MessageUUID = "M" + time.Now().Format("150405.000000000")
		if sequence == 10_001 {
			expiresAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
			projection.FileExpiresAt = &expiresAt
		}
		if _, err := store.Append(ctx, projection); err != nil {
			t.Fatalf("append sequence %d: %v", sequence, err)
		}
	}
	fileRecord, found, err := store.Lookup(ctx, first.ConversationKey, 10_001)
	if err != nil || !found || fileRecord.Projection.FileExpiresAt == nil || !fileRecord.Projection.FileExpiresAt.Equal(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("lookup file expiry: record=%+v found=%v err=%v", fileRecord, found, err)
	}
	rangeRecords, err := store.ListRange(ctx, first.ConversationKey, 10_000, 10_001)
	if err != nil {
		t.Fatalf("list cross-bucket range: %v", err)
	}
	if len(rangeRecords) != 2 || rangeRecords[0].Projection.MessageSeq != 10_000 || rangeRecords[1].Projection.MessageSeq != 10_001 {
		t.Fatalf("expected ascending cross-bucket range [10000 10001], got %+v", rangeRecords)
	}

	iter := session.Query(`
SELECT message_seq FROM timeline_by_conversation_bucket
WHERE conversation_key = ? AND bucket = ?`, first.ConversationKey, int64(0)).Iter()
	var sequences []int64
	var sequence int64
	for iter.Scan(&sequence) {
		sequences = append(sequences, sequence)
	}
	if err := iter.Close(); err != nil {
		t.Fatalf("read Cassandra timeline bucket: %v", err)
	}
	if len(sequences) != 2 || sequences[0] != 10_000 || sequences[1] != 1 {
		t.Fatalf("expected descending bucket sequences [10000 1], got %v", sequences)
	}
}

func openCassandraSession(t *testing.T, hosts []string, keyspace string) *gocql.Session {
	t.Helper()
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.LocalOne
	cluster.Timeout = 5 * time.Second
	cluster.ConnectTimeout = 5 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		t.Fatalf("connect Cassandra keyspace %s: %v", keyspace, err)
	}
	return session
}

func splitContactPoints(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
