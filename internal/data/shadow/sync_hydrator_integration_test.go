package shadow

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/config"
	cassandradata "github.com/JekYUlll/Dipole/internal/data/cassandra"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	mysqlrepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	_ "github.com/go-sql-driver/mysql"
)

func TestSyncCassandraHydrationShadowContract(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DIPOLE_TEST_MYSQL_DSN"))
	hosts := splitSyncShadowHosts(os.Getenv("DIPOLE_TEST_CASSANDRA_HOSTS"))
	if dsn == "" || len(hosts) == 0 {
		t.Skip("DIPOLE_TEST_MYSQL_DSN and DIPOLE_TEST_CASSANDRA_HOSTS are required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	queries := generated.New(db)
	mysqlHydrator, err := mysqlrepository.NewMySQLSyncMessageHydrator(queries)
	if err != nil {
		t.Fatalf("create MySQL hydrator: %v", err)
	}
	session, err := cassandradata.OpenSession(config.Cassandra{Enabled: true, Hosts: hosts, Keyspace: "dipole_message_shadow", LocalDatacenter: "datacenter1", TimelineBucketSize: 10_000, ConnectTimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("open Cassandra: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	timeline, err := cassandradata.NewTimelineStore(session, 10_000)
	if err != nil {
		t.Fatalf("create timeline: %v", err)
	}
	cassandraHydrator, err := cassandradata.NewSyncMessageHydrator(timeline)
	if err != nil {
		t.Fatalf("create Cassandra hydrator: %v", err)
	}

	suffix := time.Now().UnixNano() % 1_000_000_000
	userUUID := fmt.Sprintf("USH%09d", suffix)
	messageUUID := fmt.Sprintf("MSH%09d", suffix)
	conversationKey := "group:G-SYNC-SHADOW"
	sentAt := time.Date(2026, 8, 27, 13, 0, 0, 0, time.UTC)
	if _, err = db.Exec("INSERT INTO user_sync_states (user_uuid, created_at, updated_at) VALUES (?, NOW(3), NOW(3))", userUUID); err != nil {
		t.Fatalf("insert user state: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO messages (uuid, client_message_id, conversation_key, seq, sender_uuid, target_type, target_uuid, message_type, content, sent_at) VALUES (?, ?, ?, 1, 'U1', 1, 'G-SYNC-SHADOW', 0, 'shadow-body', ?)`, messageUUID, "C"+messageUUID, conversationKey, sentAt); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if _, err = db.Exec("INSERT INTO user_sync_inbox (user_uuid, message_uuid, conversation_key, message_seq, created_at) VALUES (?, ?, ?, 1, NOW(3))", userUUID, messageUUID, conversationKey); err != nil {
		t.Fatalf("insert Inbox: %v", err)
	}
	if _, err = timeline.Append(context.Background(), cassandradata.TimelineProjection{EventID: "E" + messageUUID, EventVersion: "v1", ConversationKey: conversationKey, MessageSeq: 1, MessageUUID: messageUUID, ClientMessageID: "C" + messageUUID, SenderUUID: "U1", TargetType: 1, TargetUUID: "G-SYNC-SHADOW", Content: "shadow-body", SentAt: sentAt}); err != nil {
		t.Fatalf("append timeline: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM user_sync_inbox WHERE user_uuid = ?", userUUID)
		_, _ = db.Exec("DELETE FROM user_sync_states WHERE user_uuid = ?", userUUID)
		_, _ = db.Exec("DELETE FROM messages WHERE uuid = ?", messageUUID)
		_ = session.Query("DELETE FROM timeline_by_conversation_bucket WHERE conversation_key = ? AND bucket = 0", conversationKey).Exec()
	})

	comparisons := make(chan SyncHydrationComparison, 3)
	shadowHydrator := NewSyncMessageHydrator(mysqlHydrator, cassandraHydrator, func(value SyncHydrationComparison) { comparisons <- value })
	syncStore, err := mysqlrepository.NewSyncRepositoryWithHydrator(queries, shadowHydrator)
	if err != nil {
		t.Fatalf("create Sync repository: %v", err)
	}
	assertPrimary := func() {
		items, listErr := syncStore.ListByUserAfter(userUUID, 0, 10)
		if listErr != nil || len(items) != 1 || items[0].Message == nil || items[0].Message.ID == 0 || items[0].Message.Content != "shadow-body" {
			t.Fatalf("primary items=%+v err=%v", items, listErr)
		}
		shadowHydrator.Wait()
	}
	assertPrimary()
	if comparison := <-comparisons; !comparison.Match {
		t.Fatalf("matching comparison=%+v", comparison)
	}
	if err = session.Query("UPDATE timeline_by_conversation_bucket SET content = 'corrupted' WHERE conversation_key = ? AND bucket = 0 AND message_seq = 1", conversationKey).Exec(); err != nil {
		t.Fatalf("corrupt timeline: %v", err)
	}
	assertPrimary()
	if comparison := <-comparisons; comparison.Match || comparison.ShadowError != "" {
		t.Fatalf("mismatch comparison=%+v", comparison)
	}
	if err = session.Query("DELETE FROM timeline_by_conversation_bucket WHERE conversation_key = ? AND bucket = 0 AND message_seq = 1", conversationKey).Exec(); err != nil {
		t.Fatalf("delete timeline: %v", err)
	}
	assertPrimary()
	if comparison := <-comparisons; comparison.Match || comparison.ShadowError == "" {
		t.Fatalf("missing comparison=%+v", comparison)
	}
}

func splitSyncShadowHosts(raw string) []string {
	var result []string
	for _, host := range strings.Split(raw, ",") {
		if host = strings.TrimSpace(host); host != "" {
			result = append(result, host)
		}
	}
	return result
}
