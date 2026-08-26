package syncprojector

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	sqlcrepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	"github.com/JekYUlll/Dipole/internal/service"
	_ "github.com/go-sql-driver/mysql"
)

func TestKafkaMySQLDualRunIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DIPOLE_TEST_SYNC_PROJECTOR_DSN"))
	brokersValue := strings.TrimSpace(os.Getenv("DIPOLE_TEST_SYNC_PROJECTOR_BROKERS"))
	if dsn == "" || brokersValue == "" {
		t.Skip("set DIPOLE_TEST_SYNC_PROJECTOR_DSN and DIPOLE_TEST_SYNC_PROJECTOR_BROKERS")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate MySQL: %v", err)
	}

	store, err := mysqldata.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	projectionStore, err := sqlcrepository.NewSyncProjectionRepository(store)
	if err != nil {
		t.Fatalf("create Sync projection store: %v", err)
	}
	projector, err := New(projectionStore)
	if err != nil {
		t.Fatalf("create Sync projector: %v", err)
	}

	prefix := fmt.Sprintf("syncsmoke%d", time.Now().UnixNano())
	kafkaCfg := config.Kafka{
		Enabled: true, Brokers: strings.Split(brokersValue, ","), ClientID: prefix,
		TopicPrefix: prefix, TopicPartitions: 1, TopicReplicationFactor: 3, TopicMinInSyncReplicas: 2,
		TopicRetentionHours: 1, RequiredAcks: "all", DialTimeoutSeconds: 10, WriteTimeoutSeconds: 10,
		ConsumeRetryMaxAttempts: 2, ConsumeRetryBackoffMS: 20, ConsumerGroupBalancer: "roundrobin",
		ConsumerHeartbeatSeconds: 1, ConsumerSessionTimeoutSeconds: 10, ConsumerRebalanceTimeoutSeconds: 10,
	}
	publisher, err := platformkafka.NewPublisher(kafkaCfg)
	if err != nil {
		t.Fatalf("create Kafka publisher: %v", err)
	}
	defer publisher.Close()
	if err := publisher.EnsureTopics(Topics()); err != nil {
		t.Fatalf("ensure Kafka topics: %v", err)
	}
	consumer, err := platformkafka.NewConsumerForService(kafkaCfg, prefix+"-consumer")
	if err != nil {
		t.Fatalf("create Kafka consumer: %v", err)
	}
	defer consumer.Close()
	for _, topic := range Topics() {
		consumer.Register(topic, projector.Handler())
	}
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start Kafka consumer: %v", err)
	}

	direct := &model.SyncProjection{
		EventID: "E-SYNC-DUAL", MessageUUID: "M-SYNC-DUAL", ConversationKey: "direct:U100:U200",
		MessageSeq: 17, RecipientUUIDs: []string{"U100", "U200"},
	}
	// Simulate the current Message-owned transaction before the independent projector sees the event.
	if err := projectionStore.Apply(direct); err != nil {
		t.Fatalf("seed Message-owned Inbox projection: %v", err)
	}
	fanout := true
	directPayload := service.MessageEventPayload{
		MessageID: direct.MessageUUID, ConversationKey: direct.ConversationKey, MessageSeq: direct.MessageSeq,
		SenderUUID: "U100", TargetUUID: "U200", TargetType: model.MessageTargetDirect,
		RecipientUUIDs: direct.RecipientUUIDs, SyncFanout: &fanout, SentAt: time.Now().UTC(),
	}
	publishUntilCommitted(t, ctx, publisher, consumer, topics[0], direct.MessageUUID, directPayload, 1)
	publishUntilCommitted(t, ctx, publisher, consumer, topics[0], direct.MessageUUID, directPayload, 2)
	if got := countProjectionRows(t, db, direct.MessageUUID); got != 2 {
		t.Fatalf("dual-run replay should retain one row per recipient, got %d", got)
	}
	rollbackRecipient := "U050"
	if err := projectionStore.Apply(&model.SyncProjection{
		EventID: "E-SYNC-CONFLICT", MessageUUID: direct.MessageUUID, ConversationKey: direct.ConversationKey,
		MessageSeq: direct.MessageSeq + 1, RecipientUUIDs: []string{rollbackRecipient, "U100"},
	}); err == nil {
		t.Fatal("conflicting locator should fail the recipient batch")
	}
	if got := countRecipientRows(t, db, rollbackRecipient); got != 0 {
		t.Fatalf("conflicting projection left %d partial recipient rows", got)
	}

	noFanout := false
	hotPayload := service.MessageEventPayload{
		MessageID: "M-SYNC-HOT", ConversationKey: "group:G-HOT", MessageSeq: 21,
		SenderUUID: "U100", TargetUUID: "G-HOT", TargetType: model.MessageTargetGroup,
		RecipientUUIDs: []string{"U100", "U200"}, SyncFanout: &noFanout, SentAt: time.Now().UTC(),
	}
	publishUntilCommitted(t, ctx, publisher, consumer, topics[1], hotPayload.MessageID, hotPayload, 3)
	if got := countProjectionRows(t, db, hotPayload.MessageID); got != 0 {
		t.Fatalf("hot-group event created %d Inbox rows", got)
	}
}

func publishUntilCommitted(t *testing.T, ctx context.Context, publisher *platformkafka.Publisher, consumer *platformkafka.Consumer, topic, key string, payload service.MessageEventPayload, minimum uint64) {
	t.Helper()
	deadline := time.NewTicker(400 * time.Millisecond)
	defer deadline.Stop()
	for {
		if err := publisher.PublishEvent(ctx, topic, key, topic, payload, nil); err != nil {
			t.Fatalf("publish %s: %v", topic, err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for committed Sync event: %v", ctx.Err())
		case <-deadline.C:
			if consumer.CollectStats().Committed >= minimum {
				return
			}
		}
	}
}

func countProjectionRows(t *testing.T, db *sql.DB, messageUUID string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM user_sync_inbox WHERE message_uuid = ?", messageUUID).Scan(&count); err != nil {
		t.Fatalf("count Sync projection rows: %v", err)
	}
	return count
}

func countRecipientRows(t *testing.T, db *sql.DB, userUUID string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM user_sync_inbox WHERE user_uuid = ?", userUUID).Scan(&count); err != nil {
		t.Fatalf("count Sync recipient rows: %v", err)
	}
	return count
}
