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
	"github.com/JekYUlll/Dipole/internal/compat/service"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/model"
	platformkafka "github.com/JekYUlll/Dipole/internal/platform/kafka"
	mysqldata "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	messagedomain "github.com/JekYUlll/Dipole/internal/services/message/domain"
	syncmysql "github.com/JekYUlll/Dipole/internal/services/sync/infrastructure/mysql"
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
	projectionStore, err := syncmysql.NewSyncProjectionRepository(store)
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
	catchupFanout := true
	catchupPayload := messagedomain.MessageEventPayload{
		MessageID: "M-SYNC-CATCHUP", ConversationKey: "direct:U300:U400", MessageSeq: 1,
		SenderUUID: "U300", TargetUUID: "U400", TargetType: model.MessageTargetDirect,
		RecipientUUIDs: []string{"U300", "U400"}, SyncFanout: &catchupFanout, SentAt: time.Now().UTC(),
	}
	publishEventEventually(t, ctx, publisher, topics[0], catchupPayload.MessageID, catchupPayload)
	consumer, err := platformkafka.NewReplayableConsumerForService(kafkaCfg, prefix+"-consumer")
	if err != nil {
		t.Fatalf("create Kafka consumer: %v", err)
	}
	defer consumer.Close()
	consumer.UseFailurePublisher(publisher)
	for _, topic := range Topics() {
		consumer.Register(topic, projector.Handler())
	}
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start Kafka consumer: %v", err)
	}
	waitForConsumerStats(t, ctx, consumer, func(stats platformkafka.ConsumerStats) bool { return stats.Committed >= 1 })
	if got := countProjectionRows(t, db, catchupPayload.MessageID); got != 2 {
		t.Fatalf("new Sync group projected %d pre-existing recipient rows, want 2", got)
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
	publishUntilCommitted(t, ctx, publisher, consumer, topics[0], direct.MessageUUID, directPayload, 2)
	publishUntilCommitted(t, ctx, publisher, consumer, topics[0], direct.MessageUUID, directPayload, 3)
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
	publishUntilCommitted(t, ctx, publisher, consumer, topics[1], hotPayload.MessageID, hotPayload, 4)
	if got := countProjectionRows(t, db, hotPayload.MessageID); got != 0 {
		t.Fatalf("hot-group event created %d Inbox rows", got)
	}
	poisonPayload := service.MessageEventPayload{
		MessageID: "M-SYNC-POISON", ConversationKey: "group:G-POISON", MessageSeq: 22,
		SenderUUID: "U100", TargetUUID: "G-POISON", TargetType: model.MessageTargetGroup,
		SyncFanout: &catchupFanout, SentAt: time.Now().UTC(),
	}
	if err := publisher.PublishEvent(ctx, topics[1], poisonPayload.MessageID, topics[1], poisonPayload, nil); err != nil {
		t.Fatalf("publish poison Sync event: %v", err)
	}
	waitForConsumerStats(t, ctx, consumer, func(stats platformkafka.ConsumerStats) bool {
		return stats.RetryPublished >= 1 && stats.DeadPublished >= 1 && stats.Committed >= 6
	})
	if got := countProjectionRows(t, db, poisonPayload.MessageID); got != 0 {
		t.Fatalf("poison event created %d Inbox rows", got)
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

func publishEventEventually(t *testing.T, ctx context.Context, publisher *platformkafka.Publisher, topic, key string, payload service.MessageEventPayload) {
	t.Helper()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := publisher.PublishEvent(ctx, topic, key, topic, payload, nil); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("publish pre-group Sync event: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForConsumerStats(t *testing.T, ctx context.Context, consumer *platformkafka.Consumer, ready func(platformkafka.ConsumerStats) bool) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		stats := consumer.CollectStats()
		if ready(stats) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Sync consumer outcome: stats=%+v err=%v", stats, ctx.Err())
		case <-ticker.C:
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
