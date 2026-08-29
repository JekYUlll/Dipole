package routing

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	mysqlRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
	cassandraData "github.com/JekYUlll/Dipole/internal/platform/cassandra"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	_ "github.com/go-sql-driver/mysql"
)

func TestCassandraReadRouterMySQLFallbackContract(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MYSQL_DSN"))
	hosts := splitHosts(os.Getenv("DIPOLE_TEST_CASSANDRA_HOSTS"))
	if dsn == "" || len(hosts) == 0 {
		t.Skip("DIPOLE_TEST_MYSQL_DSN and DIPOLE_TEST_CASSANDRA_HOSTS are required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	primary, err := mysqlRepository.NewMessageRepository(mysqlStore)
	if err != nil {
		t.Fatalf("create MySQL message repository: %v", err)
	}
	highWater := mysqlRepository.NewConversationSequenceRepository(generated.New(db))

	session, err := cassandraData.OpenSession(config.Cassandra{
		Enabled: true, Hosts: hosts, Keyspace: "dipole_message_shadow",
		LocalDatacenter: "datacenter1", TimelineBucketSize: 10_000, ConnectTimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("open Cassandra: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	timeline, err := cassandraData.NewTimelineStore(session, 10_000)
	if err != nil {
		t.Fatalf("create timeline: %v", err)
	}

	key := fmt.Sprintf("group:ROUTE-%d", time.Now().UnixNano())
	if _, err := db.Exec("INSERT INTO conversation_sequences (conversation_key, last_seq) VALUES (?, 2)", key); err != nil {
		t.Fatalf("insert sequence head: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM conversation_sequences WHERE conversation_key = ?", key) })
	for seq := uint64(1); seq <= 2; seq++ {
		uuid := fmt.Sprintf("MR%015d%02d", time.Now().UnixNano()%1_000_000_000_000_000, seq)
		message := model.Message{
			UUID: uuid, ClientMessageID: "C-" + uuid, ConversationKey: key, Seq: seq,
			SenderUUID: "U1", TargetType: model.MessageTargetGroup, TargetUUID: "ROUTE",
			MessageType: model.MessageTypeText, Content: fmt.Sprintf("body-%d", seq),
			SentAt: time.Date(2026, 8, 27, 12, int(seq), 0, 0, time.UTC),
		}
		if _, err := db.Exec(`INSERT INTO messages
(uuid, client_message_id, conversation_key, seq, sender_uuid, target_type, target_uuid, message_type, content, sent_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, message.UUID, message.ClientMessageID, key, seq, message.SenderUUID, message.TargetType, message.TargetUUID, message.MessageType, message.Content, message.SentAt); err != nil {
			t.Fatalf("insert MySQL message %d: %v", seq, err)
		}
		if _, err := timeline.Append(context.Background(), cassandraData.TimelineProjection{
			EventID: "integration:" + uuid, EventVersion: "v1", ConversationKey: key,
			MessageSeq: seq, MessageUUID: uuid, ClientMessageID: message.ClientMessageID,
			SenderUUID: message.SenderUUID, TargetType: message.TargetType, TargetUUID: message.TargetUUID,
			MessageType: message.MessageType, Content: message.Content, SentAt: message.SentAt,
		}); err != nil {
			t.Fatalf("append Cassandra message %d: %v", seq, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM messages WHERE conversation_key = ?", key)
		_ = session.Query("DELETE FROM timeline_by_conversation_bucket WHERE conversation_key = ? AND bucket = 0", key).Exec()
	})

	observations := make(chan ReadObservation, 5)
	router := NewMessageStoreWithVerification(primary, highWater, timeline, 100, 100, func(observation ReadObservation) { observations <- observation })
	page, err := router.ListByConversationSeqAfter(key, 0, 2)
	if err != nil || len(page) != 2 || page[0].ID != 0 || (<-observations).Route != "cassandra" {
		t.Fatalf("Cassandra primary page=%+v err=%v", page, err)
	}
	page, err = router.ListByConversationSeqBefore(key, 0, 2)
	if err != nil || len(page) != 2 || page[0].ID != 0 || (<-observations).Route != "cassandra" {
		t.Fatalf("Cassandra primary before page=%+v err=%v", page, err)
	}
	if err := session.Query("UPDATE timeline_by_conversation_bucket SET content = ? WHERE conversation_key = ? AND bucket = 0 AND message_seq = 2", "corrupted", key).Exec(); err != nil {
		t.Fatalf("corrupt Cassandra payload: %v", err)
	}
	page, err = router.ListByConversationSeqBefore(key, 0, 2)
	observation := <-observations
	if err != nil || len(page) != 2 || page[0].ID == 0 || observation.Route != "mysql_fallback" || observation.FallbackReason != "payload_mismatch" {
		t.Fatalf("payload mismatch fallback page=%+v observation=%+v err=%v", page, observation, err)
	}
	if err := session.Query("UPDATE timeline_by_conversation_bucket SET content = ? WHERE conversation_key = ? AND bucket = 0 AND message_seq = 2", "body-2", key).Exec(); err != nil {
		t.Fatalf("restore Cassandra payload: %v", err)
	}
	if err := session.Query("DELETE FROM timeline_by_conversation_bucket WHERE conversation_key = ? AND bucket = 0 AND message_seq = 2", key).Exec(); err != nil {
		t.Fatalf("delete Cassandra row: %v", err)
	}
	page, err = router.ListByConversationSeqAfter(key, 0, 2)
	observation = <-observations
	if err != nil || len(page) != 2 || page[0].ID == 0 || observation.Route != "mysql_fallback" || observation.FallbackReason != "incomplete_page" {
		t.Fatalf("MySQL fallback page=%+v observation=%+v err=%v", page, observation, err)
	}
	page, err = router.ListByConversationSeqBefore(key, 0, 2)
	observation = <-observations
	if err != nil || len(page) != 2 || page[0].ID == 0 || observation.Route != "mysql_fallback" || observation.FallbackReason != "incomplete_page" {
		t.Fatalf("MySQL fallback before page=%+v observation=%+v err=%v", page, observation, err)
	}
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
