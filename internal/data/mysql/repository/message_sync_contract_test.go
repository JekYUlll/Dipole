package repository_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	mysqlData "github.com/JekYUlll/Dipole/internal/data/mysql"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
)

type messageSyncStores struct {
	message application.MessageStore
	sync    application.SyncStore
}

func TestMessageSyncRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	mysqlStore, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	sqlcMessage, err := sqlcRepository.NewMessageRepository(mysqlStore)
	if err != nil {
		t.Fatalf("create sqlc message repository: %v", err)
	}
	sqlcSync, err := sqlcRepository.NewSyncRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc sync repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runMessageSyncContract(t, db, messageSyncStores{message: sqlcMessage, sync: sqlcSync}, "sqlc")
	})
}

func runMessageSyncContract(t *testing.T, db *sql.DB, stores messageSyncStores, prefix string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	conversationKey := model.DirectConversationKey("U-"+prefix+"-sender", "U-"+prefix+"-target")
	first := contractStoredMessage(prefix+"-1", conversationKey, now)
	if err := stores.message.CreateWithSync(first, []string{" U-" + prefix + "-target ", "U-" + prefix + "-target", ""}); err != nil {
		t.Fatalf("create message with sync: %v", err)
	}
	if first.ID == 0 || first.Seq != 1 || first.CreatedAt.IsZero() || first.UpdatedAt.IsZero() {
		t.Fatalf("message fields were not hydrated: %+v", first)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", first.UUID); got != 1 {
		t.Fatalf("expected one deduplicated inbox row, got %d", got)
	}
	if err := stores.message.EnsureSyncInbox(first, []string{"U-" + prefix + "-target"}); err != nil {
		t.Fatalf("replay sync inbox: %v", err)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", first.UUID); got != 1 {
		t.Fatalf("inbox replay created duplicates: %d", got)
	}

	second := contractStoredMessage(prefix+"-2", conversationKey, now.Add(time.Second))
	event := contractOutboxEvent(second.UUID)
	if err := stores.message.StoreWithOutboxAndSync(second, staticOutboxBuilder(event), []string{"U-" + prefix + "-target"}); err != nil {
		t.Fatalf("store message, inbox, and outbox: %v", err)
	}
	if second.Seq != 2 {
		t.Fatalf("second conversation sequence = %d, want 2", second.Seq)
	}
	if got := countContractRows(t, db, "outbox_events", "aggregate_id = ?", second.UUID); got != 1 {
		t.Fatalf("expected one outbox event, got %d", got)
	}
	if err := stores.message.EnsureOutbox(event); err != nil {
		t.Fatalf("replay outbox event: %v", err)
	}
	if got := countContractRows(t, db, "outbox_events", "aggregate_id = ?", second.UUID); got != 1 {
		t.Fatalf("outbox replay created duplicates: %d", got)
	}

	byUUID, err := stores.message.GetByUUID(first.UUID)
	if err != nil || byUUID == nil || byUUID.ClientMessageID != first.ClientMessageID {
		t.Fatalf("get by UUID: message=%+v err=%v", byUUID, err)
	}
	byClient, err := stores.message.GetBySenderAndClientMessageID(first.SenderUUID, first.ClientMessageID)
	if err != nil || byClient == nil || byClient.UUID != first.UUID {
		t.Fatalf("get by client ID: message=%+v err=%v", byClient, err)
	}
	missing, err := stores.message.GetByUUID("missing")
	if err != nil || missing != nil {
		t.Fatalf("missing message: message=%+v err=%v", missing, err)
	}
	hasMessages, err := stores.message.HasConversationMessages(conversationKey)
	if err != nil || !hasMessages {
		t.Fatalf("conversation existence: has=%v err=%v", hasMessages, err)
	}
	history, err := stores.message.ListByConversationKey(conversationKey, 0, 10)
	if err != nil || len(history) != 2 || history[0].UUID != first.UUID || history[1].UUID != second.UUID {
		t.Fatalf("ordered history: messages=%+v err=%v", history, err)
	}
	before, err := stores.message.ListByConversationKey(conversationKey, second.ID, 10)
	if err != nil || len(before) != 1 || before[0].UUID != first.UUID {
		t.Fatalf("history before cursor: messages=%+v err=%v", before, err)
	}
	after, err := stores.message.ListByConversationKeyAfter(conversationKey, first.ID, 10)
	if err != nil || len(after) != 1 || after[0].UUID != second.UUID {
		t.Fatalf("history after cursor: messages=%+v err=%v", after, err)
	}
	groupUUID := "G-" + prefix
	groupRecipient := "U-" + prefix + "-target"
	if _, err := db.Exec("INSERT INTO `groups` (uuid, name, owner_uuid, member_count, status, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?, ?)", groupUUID, "Contract Group", first.SenderUUID, model.GroupStatusNormal, now, now); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if _, err := db.Exec("INSERT INTO group_members (group_uuid, user_uuid, role, joined_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", groupUUID, groupRecipient, model.GroupMemberRoleMember, now, now, now); err != nil {
		t.Fatalf("seed group member: %v", err)
	}
	groupMessage := contractStoredMessage(prefix+"-group", model.GroupConversationKey(groupUUID), now.Add(1500*time.Millisecond))
	groupMessage.TargetType = model.MessageTargetGroup
	groupMessage.TargetUUID = groupUUID
	if err := stores.message.CreateWithSync(groupMessage, nil); err != nil {
		t.Fatalf("create group message: %v", err)
	}
	if groupMessage.Seq != 1 {
		t.Fatalf("group conversation sequence = %d, want 1", groupMessage.Seq)
	}
	offline, err := stores.message.ListOfflineByUserUUID("U-"+prefix+"-target", 0, 10)
	if err != nil || len(offline) != 3 || offline[2].UUID != groupMessage.UUID {
		t.Fatalf("offline direct messages: messages=%+v err=%v", offline, err)
	}
	items, err := stores.sync.ListByUserAfter(" U-"+prefix+"-target ", 0, 10)
	if err != nil || len(items) != 2 || items[0].Message.UUID != first.UUID || items[1].Message.UUID != second.UUID || items[0].SyncSeq >= items[1].SyncSeq {
		t.Fatalf("sync timeline order: items=%+v err=%v", items, err)
	}
	latestSyncSeq, err := stores.sync.GetLatestUserSyncSequence("U-" + prefix + "-target")
	if err != nil || latestSyncSeq != items[len(items)-1].SyncSeq {
		t.Fatalf("latest sync sequence: seq=%d err=%v", latestSyncSeq, err)
	}
	checkpoint, err := stores.sync.GetDeviceCheckpoint("U-"+prefix+"-target", "web-a")
	if err != nil || checkpoint != nil {
		t.Fatalf("missing device checkpoint: checkpoint=%+v err=%v", checkpoint, err)
	}
	if err := stores.sync.AdvanceDeviceSyncCheckpoint("U-"+prefix+"-target", "web-a", items[1].SyncSeq); err != nil {
		t.Fatalf("advance device checkpoint: %v", err)
	}
	if err := stores.sync.AdvanceDeviceSyncCheckpoint("U-"+prefix+"-target", "web-a", items[0].SyncSeq); err != nil {
		t.Fatalf("repeat lower device checkpoint: %v", err)
	}
	checkpoint, err = stores.sync.GetDeviceCheckpoint("U-"+prefix+"-target", "web-a")
	if err != nil || checkpoint == nil || checkpoint.SyncSeq != items[1].SyncSeq {
		t.Fatalf("device checkpoint regressed: checkpoint=%+v err=%v", checkpoint, err)
	}
	otherCheckpoint, err := stores.sync.GetDeviceCheckpoint("U-"+prefix+"-target", "mobile-b")
	if err != nil || otherCheckpoint != nil {
		t.Fatalf("device checkpoints were not isolated: checkpoint=%+v err=%v", otherCheckpoint, err)
	}

	fileMessage := contractStoredMessage(prefix+"-file", conversationKey, now.Add(2*time.Second))
	fileMessage.MessageType = model.MessageTypeFile
	fileMessage.FileID = "F-" + prefix
	fileMessage.FileName = "report.pdf"
	if err := stores.message.CreateWithSync(fileMessage, nil); err != nil {
		t.Fatalf("create file message: %v", err)
	}
	accessible, err := stores.message.FindLatestAccessibleFileMessage(fileMessage.FileID, fileMessage.TargetUUID)
	if err != nil || accessible == nil || accessible.UUID != fileMessage.UUID {
		t.Fatalf("find accessible file: message=%+v err=%v", accessible, err)
	}
	denied, err := stores.message.FindLatestAccessibleFileMessage(fileMessage.FileID, "U-outsider")
	if err != nil || denied != nil {
		t.Fatalf("deny inaccessible file: message=%+v err=%v", denied, err)
	}

	rollback := contractStoredMessage(prefix+"-rollback", conversationKey, now.Add(3*time.Second))
	badEvent := contractOutboxEvent(rollback.UUID)
	badEvent.Topic = strings.Repeat("x", 200)
	if err := stores.message.StoreWithOutboxAndSync(rollback, staticOutboxBuilder(badEvent), []string{"U-" + prefix + "-target"}); err == nil {
		t.Fatal("expected invalid outbox event to roll back transaction")
	}
	if got := countContractRows(t, db, "messages", "uuid = ?", rollback.UUID); got != 0 {
		t.Fatalf("message survived outbox rollback: %d", got)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", rollback.UUID); got != 0 {
		t.Fatalf("inbox survived outbox rollback: %d", got)
	}
	if rollback.ID != 0 || rollback.Seq != 0 || !rollback.CreatedAt.IsZero() || !rollback.UpdatedAt.IsZero() {
		t.Fatalf("rolled-back message retained persisted fields: %+v", rollback)
	}
	afterRollback := contractStoredMessage(prefix+"-after-rollback", conversationKey, now.Add(4*time.Second))
	if err := stores.message.CreateWithSync(afterRollback, nil); err != nil {
		t.Fatalf("create message after rollback: %v", err)
	}
	if afterRollback.Seq != 4 {
		t.Fatalf("sequence after rollback = %d, want 4", afterRollback.Seq)
	}
}

func contractStoredMessage(suffix, conversationKey string, sentAt time.Time) *model.Message {
	return &model.Message{
		UUID: "M-" + suffix, ClientMessageID: "CM-" + suffix,
		ConversationKey: conversationKey, SenderUUID: "U-" + strings.Split(suffix, "-")[0] + "-sender",
		TargetType: model.MessageTargetDirect, TargetUUID: "U-" + strings.Split(suffix, "-")[0] + "-target",
		MessageType: model.MessageTypeText, Content: "hello", SentAt: sentAt,
	}
}

func contractOutboxEvent(messageUUID string) *model.OutboxEvent {
	return &model.OutboxEvent{AggregateType: "message", AggregateID: messageUUID, EventType: "message.created", Topic: "message.created", MessageKey: messageUUID, Value: []byte(`{"message_id":"` + messageUUID + `"}`), Status: model.OutboxStatusPending}
}

func staticOutboxBuilder(event *model.OutboxEvent) application.MessageOutboxBuilder {
	return func(*model.Message) (*model.OutboxEvent, error) { return event, nil }
}

func countContractRows(t *testing.T, db *sql.DB, table, where string, arg any) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+where, arg).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
