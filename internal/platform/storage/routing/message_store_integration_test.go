//go:build integration

package routing

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	cassandraData "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	timelineData "github.com/JekYUlll/Dipole/internal/platform/storage/timeline"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
	"github.com/apache/cassandra-gocql-driver/v2"
	_ "github.com/go-sql-driver/mysql"
)

const maxTimelineBenchmarkMessages = 10_000

type timelineFixture struct {
	key       string
	messageDB application.MessageStore
	highWater HighWatermarkReader
	timeline  application.ConversationTimelineReader
	session   *gocql.Session
}

func TestCassandraReadRouterMySQLFallbackContract(t *testing.T) {
	fixture := newTimelineFixture(t, 2)
	observations := make(chan ReadObservation, 5)
	router := NewMessageStoreWithVerification(fixture.messageDB, fixture.highWater, fixture.timeline, 100, 100, func(observation ReadObservation) { observations <- observation })

	page, err := router.ListByConversationSeqAfter(fixture.key, 0, 2)
	if err != nil || len(page) != 2 || page[0].ID != 0 || (<-observations).Route != "cassandra" {
		t.Fatalf("Cassandra primary page=%+v err=%v", page, err)
	}
	page, err = router.ListByConversationSeqBefore(fixture.key, 0, 2)
	if err != nil || len(page) != 2 || page[0].ID != 0 || (<-observations).Route != "cassandra" {
		t.Fatalf("Cassandra primary before page=%+v err=%v", page, err)
	}
	if err := fixture.session.Query("UPDATE timeline_by_conversation_bucket SET content = ? WHERE conversation_key = ? AND bucket = 0 AND message_seq = 2", "corrupted", fixture.key).Exec(); err != nil {
		t.Fatalf("corrupt Cassandra payload: %v", err)
	}
	page, err = router.ListByConversationSeqBefore(fixture.key, 0, 2)
	observation := <-observations
	if err != nil || len(page) != 2 || page[0].ID == 0 || observation.Route != "mysql_fallback" || observation.FallbackReason != "payload_mismatch" {
		t.Fatalf("payload mismatch fallback page=%+v observation=%+v err=%v", page, observation, err)
	}
	if err := fixture.session.Query("UPDATE timeline_by_conversation_bucket SET content = ? WHERE conversation_key = ? AND bucket = 0 AND message_seq = 2", "body-2", fixture.key).Exec(); err != nil {
		t.Fatalf("restore Cassandra payload: %v", err)
	}
	if err := fixture.session.Query("DELETE FROM timeline_by_conversation_bucket WHERE conversation_key = ? AND bucket = 0 AND message_seq = 2", fixture.key).Exec(); err != nil {
		t.Fatalf("delete Cassandra row: %v", err)
	}
	page, err = router.ListByConversationSeqAfter(fixture.key, 0, 2)
	observation = <-observations
	if err != nil || len(page) != 2 || page[0].ID == 0 || observation.Route != "mysql_fallback" || observation.FallbackReason != "incomplete_page" {
		t.Fatalf("missing-row fallback page=%+v observation=%+v err=%v", page, observation, err)
	}
	page, err = router.ListByConversationSeqBefore(fixture.key, 0, 2)
	observation = <-observations
	if err != nil || len(page) != 2 || page[0].ID == 0 || observation.Route != "mysql_fallback" || observation.FallbackReason != "incomplete_page" {
		t.Fatalf("missing-row before fallback page=%+v observation=%+v err=%v", page, observation, err)
	}
}

func BenchmarkConversationTimelineReaders(b *testing.B) {
	messageCount := timelineBenchmarkMessages(b)
	fixture := newTimelineFixture(b, messageCount)
	readers := []struct {
		name   string
		reader application.ConversationTimelineReader
	}{
		{name: "mysql_sqlc", reader: mustMessageStoreReader(b, fixture.messageDB)},
		{name: "cassandra", reader: fixture.timeline},
	}
	for _, candidate := range readers {
		b.Run(candidate.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				page, err := candidate.reader.ListConversationRange(context.Background(), fixture.key, 1, messageCount)
				if err != nil {
					b.Fatalf("read timeline: %v", err)
				}
				if len(page) != int(messageCount) {
					b.Fatalf("timeline page count=%d want=%d", len(page), messageCount)
				}
			}
		})
	}
}

func newTimelineFixture(tb testing.TB, messageCount uint64) timelineFixture {
	tb.Helper()
	if messageCount == 0 || messageCount > maxTimelineBenchmarkMessages {
		tb.Fatalf("timeline fixture message count must be between 1 and %d", maxTimelineBenchmarkMessages)
	}
	dsn := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MYSQL_DSN"))
	hosts := splitHosts(os.Getenv("DIPOLE_TEST_CASSANDRA_HOSTS"))
	if dsn == "" || len(hosts) == 0 {
		tb.Skip("DIPOLE_TEST_MYSQL_DSN and DIPOLE_TEST_CASSANDRA_HOSTS are required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		tb.Fatalf("open MySQL: %v", err)
	}
	tb.Cleanup(func() { _ = db.Close() })
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		tb.Fatalf("create MySQL store: %v", err)
	}
	primary, err := messagemysql.NewMessageRepository(mysqlStore)
	if err != nil {
		tb.Fatalf("create MySQL message repository: %v", err)
	}
	session, err := cassandraData.OpenSession(config.Cassandra{
		Enabled: true, Hosts: hosts, Keyspace: "dipole_message_shadow",
		LocalDatacenter: "datacenter1", TimelineBucketSize: maxTimelineBenchmarkMessages, ConnectTimeoutSeconds: 5,
	})
	if err != nil {
		tb.Fatalf("open Cassandra: %v", err)
	}
	tb.Cleanup(func() { session.Close() })
	timeline, err := cassandraData.NewTimelineStore(session, maxTimelineBenchmarkMessages)
	if err != nil {
		tb.Fatalf("create timeline: %v", err)
	}

	key := fmt.Sprintf("group:ROUTE-%d", time.Now().UnixNano())
	if _, err := db.Exec("INSERT INTO conversation_sequences (conversation_key, last_seq) VALUES (?, ?)", key, messageCount); err != nil {
		tb.Fatalf("insert sequence head: %v", err)
	}
	tb.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM conversation_sequences WHERE conversation_key = ?", key)
		_, _ = db.Exec("DELETE FROM messages WHERE conversation_key = ?", key)
		_ = session.Query("DELETE FROM timeline_by_conversation_bucket WHERE conversation_key = ? AND bucket = 0", key).Exec()
	})
	for seq := uint64(1); seq <= messageCount; seq++ {
		uuid := fmt.Sprintf("MR%015d%04d", time.Now().UnixNano()%1_000_000_000_000_000, seq)
		message := model.Message{
			UUID: uuid, ClientMessageID: "C-" + uuid, ConversationKey: key, Seq: seq,
			SenderUUID: "U1", TargetType: model.MessageTargetGroup, TargetUUID: "ROUTE",
			MessageType: model.MessageTypeText, Content: fmt.Sprintf("body-%d", seq),
			SentAt: time.Date(2026, 8, 27, 12, int(seq%60), 0, 0, time.UTC),
		}
		if _, err := db.Exec(`INSERT INTO messages
(uuid, client_message_id, conversation_key, seq, sender_uuid, target_type, target_uuid, message_type, content, sent_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, message.UUID, message.ClientMessageID, key, seq, message.SenderUUID, message.TargetType, message.TargetUUID, message.MessageType, message.Content, message.SentAt); err != nil {
			tb.Fatalf("insert MySQL message %d: %v", seq, err)
		}
		if _, err := timeline.Append(context.Background(), cassandraData.TimelineProjection{
			EventID: "integration:" + uuid, EventVersion: "v1", ConversationKey: key,
			MessageSeq: seq, MessageUUID: uuid, ClientMessageID: message.ClientMessageID,
			SenderUUID: message.SenderUUID, TargetType: message.TargetType, TargetUUID: message.TargetUUID,
			MessageType: message.MessageType, Content: message.Content, SentAt: message.SentAt,
		}); err != nil {
			tb.Fatalf("append Cassandra message %d: %v", seq, err)
		}
	}
	return timelineFixture{
		key: key, messageDB: primary, highWater: messagemysql.NewConversationSequenceRepository(generated.New(db)), timeline: timeline, session: session,
	}
}

func mustMessageStoreReader(tb testing.TB, store application.MessageStore) application.ConversationTimelineReader {
	tb.Helper()
	reader, err := timelineData.NewMessageStoreReader(store)
	if err != nil {
		tb.Fatalf("create MySQL Timeline reader: %v", err)
	}
	return reader
}

func timelineBenchmarkMessages(tb testing.TB) uint64 {
	tb.Helper()
	value, err := strconv.ParseUint(strings.TrimSpace(os.Getenv("DIPOLE_TIMELINE_BENCH_MESSAGES")), 10, 64)
	if err != nil || value == 0 {
		tb.Fatal("DIPOLE_TIMELINE_BENCH_MESSAGES must be a positive integer")
	}
	return value
}

func splitHosts(raw string) []string {
	var hosts []string
	for _, host := range strings.Split(raw, ",") {
		if host = strings.TrimSpace(host); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}
