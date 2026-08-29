package messagemysql_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
	mysqlData "github.com/JekYUlll/Dipole/internal/platform/mysql"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	messagemysql "github.com/JekYUlll/Dipole/internal/services/message/infrastructure/mysql"
)

type messageSyncStores struct {
	message  application.MessageStore
	metadata interface {
		GetMetadataByUUID(string) (*model.MessageMetadata, error)
		GetMetadataBySenderAndClientMessageID(string, string) (*model.MessageMetadata, error)
	}
	sync       application.SyncStore
	projection application.SyncProjectionStore
}

func TestMessageProjectorAccountWritesMessageAndOutbox(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_MESSAGE_PROJECTOR_MYSQL_DSN is required for Message permission integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open Message projector database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create Message projector store: %v", err)
	}
	repository, err := messagemysql.NewMessageRepositoryWithInboxWrites(store, false)
	if err != nil {
		t.Fatalf("create Message projector repository: %v", err)
	}
	message := contractStoredMessage("projector-smoke", model.DirectConversationKey("U-proj-sender", "U-proj-target"), time.Now().UTC())
	event := contractOutboxEvent(message.UUID)
	if err := repository.StoreWithOutboxAndSync(message, staticOutboxBuilder(event), []string{"U-proj-target"}); err != nil {
		t.Fatalf("store Message and Outbox with projector account: %v", err)
	}
	if _, err := repository.ListOfflineByUserUUID("U-proj-target", 0, 10); err != nil {
		t.Fatalf("query legacy offline compatibility with projector account: %v", err)
	}
	if message.Seq != 1 {
		t.Fatalf("message seq = %d, want 1", message.Seq)
	}
}

func TestMessageAtomicAccountWritesMessageOutboxAndInbox(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_MESSAGE_ATOMIC_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_MESSAGE_ATOMIC_MYSQL_DSN is required for Message permission integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open Message atomic database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create Message atomic store: %v", err)
	}
	repository, err := messagemysql.NewMessageRepositoryWithInboxWrites(store, true)
	if err != nil {
		t.Fatalf("create Message atomic repository: %v", err)
	}
	message := contractStoredMessage("atomic-smoke", model.DirectConversationKey("U-atomic-sender", "U-atomic-target"), time.Now().UTC())
	if err := repository.StoreWithOutboxAndSync(message, staticOutboxBuilder(contractOutboxEvent(message.UUID)), []string{"U-atomic-target"}); err != nil {
		t.Fatalf("store Message, Outbox, and Inbox with atomic account: %v", err)
	}
	if message.Seq != 1 {
		t.Fatalf("message seq = %d, want 1", message.Seq)
	}
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
	sqlcMessage, err := messagemysql.NewMessageRepository(mysqlStore)
	if err != nil {
		t.Fatalf("create sqlc message repository: %v", err)
	}
	sqlcSync, err := sqlcRepository.NewSyncRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc sync repository: %v", err)
	}
	sqlcProjection, err := sqlcRepository.NewSyncProjectionRepository(mysqlStore)
	if err != nil {
		t.Fatalf("create sqlc Sync projection repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runMessageSyncContract(t, db, messageSyncStores{message: sqlcMessage, metadata: sqlcMessage, sync: sqlcSync, projection: sqlcProjection}, "sqlc")
	})
}

func TestMessageInboxWriteOwnershipCanMoveToProjectorAndRollBack(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	store, err := mysqlData.NewStore(db)
	if err != nil {
		t.Fatalf("create MySQL store: %v", err)
	}
	projectorMode, err := messagemysql.NewMessageRepositoryWithInboxWrites(store, false)
	if err != nil {
		t.Fatalf("create projector-mode repository: %v", err)
	}
	projection, err := sqlcRepository.NewSyncProjectionRepository(store)
	if err != nil {
		t.Fatalf("create Sync projector: %v", err)
	}

	recipient := "U-owner-target"
	message := contractStoredMessage("owner-projector", model.DirectConversationKey("U-owner-sender", recipient), time.Now().UTC())
	event := contractOutboxEvent(message.UUID)
	if err := projectorMode.StoreWithOutboxAndSync(message, staticOutboxBuilder(event), []string{recipient}); err != nil {
		t.Fatalf("store in projector mode: %v", err)
	}
	if got := countContractRows(t, db, "messages", "uuid = ?", message.UUID); got != 1 {
		t.Fatalf("message rows = %d, want 1", got)
	}
	if got := countContractRows(t, db, "outbox_events", "aggregate_id = ?", message.UUID); got != 1 {
		t.Fatalf("outbox rows = %d, want 1", got)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", message.UUID); got != 0 {
		t.Fatalf("projector-mode inbox rows = %d, want 0", got)
	}
	if err := projectorMode.EnsureSyncInbox(message, []string{recipient}); err != nil {
		t.Fatalf("projector-mode duplicate repair: %v", err)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", message.UUID); got != 0 {
		t.Fatalf("projector-mode repair wrote %d rows", got)
	}

	if err := projection.Apply(&model.SyncProjection{EventID: "E-" + message.UUID, MessageUUID: message.UUID, ConversationKey: message.ConversationKey, MessageSeq: message.Seq, RecipientUUIDs: []string{recipient}}); err != nil {
		t.Fatalf("apply projector event: %v", err)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", message.UUID); got != 1 {
		t.Fatalf("projected inbox rows = %d, want 1", got)
	}

	atomicMode, err := messagemysql.NewMessageRepositoryWithInboxWrites(store, true)
	if err != nil {
		t.Fatalf("create atomic-mode repository: %v", err)
	}
	rollbackMessage := contractStoredMessage("owner-rollback", message.ConversationKey, time.Now().UTC().Add(time.Second))
	rollbackEvent := contractOutboxEvent(rollbackMessage.UUID)
	if err := atomicMode.StoreWithOutboxAndSync(rollbackMessage, staticOutboxBuilder(rollbackEvent), []string{recipient}); err != nil {
		t.Fatalf("store after rollback: %v", err)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", rollbackMessage.UUID); got != 1 {
		t.Fatalf("rollback inbox rows = %d, want 1", got)
	}
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
	if err := stores.projection.Apply(&model.SyncProjection{
		EventID: "E-replay", MessageUUID: first.UUID, ConversationKey: first.ConversationKey,
		MessageSeq: first.Seq, RecipientUUIDs: []string{"U-" + prefix + "-target", "U-" + prefix + "-target"},
	}); err != nil {
		t.Fatalf("replay Sync projector event: %v", err)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", first.UUID); got != 1 {
		t.Fatalf("projector replay created duplicates: %d", got)
	}
	var storedConversation string
	var storedSequence uint64
	if err := db.QueryRow("SELECT conversation_key, message_seq FROM user_sync_inbox WHERE message_uuid = ?", first.UUID).Scan(&storedConversation, &storedSequence); err != nil {
		t.Fatalf("read Sync locator: %v", err)
	}
	if storedConversation != first.ConversationKey || storedSequence != first.Seq {
		t.Fatalf("unexpected Sync locator: conversation=%q sequence=%d", storedConversation, storedSequence)
	}
	if err := stores.message.EnsureSyncInbox(first, []string{"U-" + prefix + "-target"}); err != nil {
		t.Fatalf("replay sync inbox: %v", err)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", first.UUID); got != 1 {
		t.Fatalf("inbox replay created duplicates: %d", got)
	}
	conflict := *first
	conflict.Seq++
	if err := stores.message.EnsureSyncInbox(&conflict, []string{"U-" + prefix + "-target"}); err == nil {
		t.Fatal("expected conflicting Sync locator replay to fail")
	}
	rollbackRecipient := "U-a-" + prefix
	if err := stores.projection.Apply(&model.SyncProjection{
		EventID: "E-conflict", MessageUUID: first.UUID, ConversationKey: first.ConversationKey,
		MessageSeq: first.Seq + 1, RecipientUUIDs: []string{rollbackRecipient, "U-" + prefix + "-target"},
	}); err == nil {
		t.Fatal("expected conflicting Sync projector event to fail")
	}
	if got := countContractRows(t, db, "user_sync_inbox", "user_uuid = ?", rollbackRecipient); got != 0 {
		t.Fatalf("failed projection left partial recipient rows: %d", got)
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
	metadataByUUID, err := stores.metadata.GetMetadataByUUID(first.UUID)
	if err != nil || metadataByUUID == nil || metadataByUUID.MessageSeq != first.Seq || len(metadataByUUID.PayloadSHA256) != 64 {
		t.Fatalf("get metadata by UUID: metadata=%+v err=%v", metadataByUUID, err)
	}
	metadataByClient, err := stores.metadata.GetMetadataBySenderAndClientMessageID(first.SenderUUID, first.ClientMessageID)
	if err != nil || metadataByClient == nil || metadataByClient.MessageUUID != first.UUID || metadataByClient.PayloadSHA256 != metadataByUUID.PayloadSHA256 {
		t.Fatalf("get metadata by client ID: metadata=%+v err=%v", metadataByClient, err)
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
	seqBefore, err := stores.message.ListByConversationSeqBefore(conversationKey, second.Seq, 10)
	if err != nil || len(seqBefore) != 1 || seqBefore[0].UUID != first.UUID {
		t.Fatalf("history before sequence: messages=%+v err=%v", seqBefore, err)
	}
	seqAfter, err := stores.message.ListByConversationSeqAfter(conversationKey, first.Seq, 10)
	if err != nil || len(seqAfter) != 1 || seqAfter[0].UUID != second.UUID {
		t.Fatalf("history after sequence: messages=%+v err=%v", seqAfter, err)
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
	groupState, err := stores.sync.GetGroupSyncState(groupUUID)
	if err != nil || groupState == nil || groupState.LatestMessageSeq != 1 || groupState.LatestMessageUUID != groupMessage.UUID {
		t.Fatalf("group sync high-water: state=%+v err=%v", groupState, err)
	}
	groupCheckpoints, err := stores.sync.ListGroupSyncCheckpoints(groupRecipient, "web-a", []string{groupUUID})
	if err != nil || len(groupCheckpoints) != 1 || groupCheckpoints[0].PulledMessageSeq != 0 {
		t.Fatalf("initial group checkpoint: checkpoints=%+v err=%v", groupCheckpoints, err)
	}
	if err := stores.sync.AdvanceDeviceGroupSyncCheckpoint(groupRecipient, "web-a", groupUUID, 1); err != nil {
		t.Fatalf("advance group checkpoint: %v", err)
	}
	if err := stores.sync.AdvanceDeviceGroupSyncCheckpoint(groupRecipient, "web-a", groupUUID, 0); err != nil {
		t.Fatalf("repeat lower group checkpoint: %v", err)
	}
	groupCheckpoints, err = stores.sync.ListGroupSyncCheckpoints(groupRecipient, "web-a", []string{groupUUID})
	if err != nil || len(groupCheckpoints) != 1 || groupCheckpoints[0].PulledMessageSeq != 1 {
		t.Fatalf("group checkpoint regressed: checkpoints=%+v err=%v", groupCheckpoints, err)
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
	if _, err := db.Exec("DELETE FROM messages WHERE uuid = ?", fileMessage.UUID); err != nil {
		t.Fatalf("remove full file message body: %v", err)
	}
	accessible, err = stores.message.FindLatestAccessibleFileMessage(fileMessage.FileID, fileMessage.TargetUUID)
	if err != nil || accessible == nil || accessible.UUID != fileMessage.UUID || accessible.FileID != fileMessage.FileID {
		t.Fatalf("metadata-only file authorization: message=%+v err=%v", accessible, err)
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
	if got := countContractRows(t, db, "message_metadata", "message_uuid = ?", rollback.UUID); got != 0 {
		t.Fatalf("metadata survived outbox rollback: %d", got)
	}
	if got := countContractRows(t, db, "user_sync_inbox", "message_uuid = ?", rollback.UUID); got != 0 {
		t.Fatalf("inbox survived outbox rollback: %d", got)
	}
	if rollback.ID != 0 || rollback.Seq != 0 || !rollback.CreatedAt.IsZero() || !rollback.UpdatedAt.IsZero() {
		t.Fatalf("rolled-back message retained persisted fields: %+v", rollback)
	}
	groupRollback := contractStoredMessage(prefix+"-group-rollback", model.GroupConversationKey(groupUUID), now.Add(3500*time.Millisecond))
	groupRollback.TargetType = model.MessageTargetGroup
	groupRollback.TargetUUID = groupUUID
	badGroupEvent := contractOutboxEvent(groupRollback.UUID)
	badGroupEvent.Topic = strings.Repeat("x", 200)
	if err := stores.message.StoreWithOutboxAndSync(groupRollback, staticOutboxBuilder(badGroupEvent), nil); err == nil {
		t.Fatal("expected invalid group outbox event to roll back transaction")
	}
	groupState, err = stores.sync.GetGroupSyncState(groupUUID)
	if err != nil || groupState == nil || groupState.LatestMessageSeq != 1 || groupState.LatestMessageUUID != groupMessage.UUID {
		t.Fatalf("group high-water survived rollback incorrectly: state=%+v err=%v", groupState, err)
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
