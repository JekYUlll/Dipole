package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/data/mysql/repository"
	"github.com/JekYUlll/Dipole/internal/model"
)

func TestConversationRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	sqlcRepo, err := sqlcRepository.NewConversationRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc conversation repository: %v", err)
	}
	t.Run("sqlc", func(t *testing.T) {
		runConversationContract(t, sqlcRepo, "sqlc")
		userUUID := "U-sqlc-conversation"
		for _, group := range []struct {
			uuid   string
			status int8
		}{
			{uuid: "G-scope-normal", status: model.GroupStatusNormal},
			{uuid: "G-scope-dismissed", status: model.GroupStatusDismissed},
			{uuid: "G-scope-hidden", status: 2},
		} {
			if _, err := db.Exec("INSERT INTO `groups` (uuid, name, owner_uuid, member_count, status) VALUES (?, 'scope', ?, 1, ?)", group.uuid, userUUID, group.status); err != nil {
				t.Fatalf("insert scope group %s: %v", group.uuid, err)
			}
			if _, err := db.Exec("INSERT INTO group_members (group_uuid, user_uuid, role, joined_at) VALUES (?, ?, 0, NOW(3))", group.uuid, userUUID); err != nil {
				t.Fatalf("insert scope member %s: %v", group.uuid, err)
			}
		}
		keys, err := sqlcRepo.ListSearchConversationKeys(userUUID)
		want := []string{
			model.DirectConversationKey(userUUID, "U-sqlc-target"),
			model.GroupConversationKey("G-scope-dismissed"),
			model.GroupConversationKey("G-scope-normal"),
		}
		if err != nil || strings.Join(keys, ",") != strings.Join(want, ",") {
			t.Fatalf("Search membership scope: keys=%v want=%v err=%v", keys, want, err)
		}
	})
}

func TestConversationRepositoryBatchGroupMessageContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	repo, err := sqlcRepository.NewConversationRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc conversation repository: %v", err)
	}
	groupUUID := "G-batch-conversation"
	senderUUID, recipientUUID := "U-batch-sender", "U-batch-recipient"
	if _, err := db.Exec("INSERT INTO `groups` (uuid, name, owner_uuid, member_count, status) VALUES (?, 'batch', ?, 2, ?)", groupUUID, senderUUID, model.GroupStatusNormal); err != nil {
		t.Fatalf("insert batch group: %v", err)
	}
	for _, userUUID := range []string{senderUUID, recipientUUID} {
		if _, err := db.Exec("INSERT INTO group_members (group_uuid, user_uuid, role, joined_at) VALUES (?, ?, 0, NOW(3))", groupUUID, userUUID); err != nil {
			t.Fatalf("insert batch member %s: %v", userUUID, err)
		}
	}
	message := contractConversationMessage("M-batch-conversation", model.GroupConversationKey(groupUUID), groupUUID, 5, model.MessageTargetGroup, model.MessageTypeText, "batch", time.Now().UTC())
	if err := repo.UpsertGroupMessageBatch(groupUUID, message); err != nil {
		t.Fatalf("batch upsert: %v", err)
	}
	sender, err := repo.GetByUserAndConversationKey(senderUUID, message.ConversationKey)
	if err != nil || sender == nil || sender.ReadSeq != 5 || sender.UnreadCount != 0 {
		t.Fatalf("sender projection: conversation=%+v err=%v", sender, err)
	}
	recipient, err := repo.GetByUserAndConversationKey(recipientUUID, message.ConversationKey)
	if err != nil || recipient == nil || recipient.ReadSeq != 4 || recipient.UnreadCount != 1 {
		t.Fatalf("recipient projection: conversation=%+v err=%v", recipient, err)
	}
	if err := repo.UpsertGroupMessageBatch(groupUUID, message); err != nil {
		t.Fatalf("repeat batch upsert: %v", err)
	}
	recipient, err = repo.GetByUserAndConversationKey(recipientUUID, message.ConversationKey)
	if err != nil || recipient == nil || recipient.UnreadCount != 1 {
		t.Fatalf("repeat batch changed unread count: conversation=%+v err=%v", recipient, err)
	}
}

func runConversationContract(t *testing.T, store application.ConversationStore, prefix string) {
	t.Helper()
	if err := store.UpsertDirectMessage("U-any", "U-target", nil, 1); err == nil {
		t.Fatal("expected nil direct message upsert to fail")
	}
	if err := store.UpsertGroupMessage("U-any", "G-target", nil, 1); err == nil {
		t.Fatal("expected nil group message upsert to fail")
	}

	userUUID := "U-" + prefix + "-conversation"
	targetUUID := "U-" + prefix + "-target"
	directKey := model.DirectConversationKey(userUUID, targetUUID)
	now := time.Now().UTC().Truncate(time.Millisecond)
	longContent := strings.Repeat("消息", 60)
	first := contractConversationMessage("M-"+prefix+"-direct-1", directKey, targetUUID, 1, model.MessageTargetDirect, model.MessageTypeText, longContent, now)
	if err := store.UpsertDirectMessage(userUUID, targetUUID, first, 1); err != nil {
		t.Fatalf("insert direct conversation: %v", err)
	}
	direct, err := store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil {
		t.Fatalf("get direct conversation: conversation=%+v err=%v", direct, err)
	}
	if direct.UnreadCount != 1 || direct.LastMessageSeq != 1 || direct.ReadSeq != 0 || direct.LastMessageUUID != first.UUID || utf8.RuneCountInString(direct.LastMessagePreview) != 100 {
		t.Fatalf("unexpected first direct projection: %+v", direct)
	}

	if err := store.UpsertDirectMessage(userUUID, targetUUID, first, 1); err != nil {
		t.Fatalf("repeat direct message: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 1 {
		t.Fatalf("duplicate message changed unread count: conversation=%+v err=%v", direct, err)
	}

	second := contractConversationMessage("M-"+prefix+"-direct-2", directKey, targetUUID, 2, model.MessageTargetDirect, model.MessageTypeFile, "", now.Add(time.Second))
	second.FileName = "report.pdf"
	if err := store.UpsertDirectMessage(userUUID, targetUUID, second, 1); err != nil {
		t.Fatalf("upsert second direct message: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 2 || direct.LastMessagePreview != "[file] report.pdf" {
		t.Fatalf("new message did not increment unread: conversation=%+v err=%v", direct, err)
	}

	if err := store.UpdateRemarkByConversationKey(userUUID, directKey, "teammate"); err != nil {
		t.Fatalf("update remark: %v", err)
	}
	if err := store.MarkReadThroughByConversationKey(userUUID, directKey, 1); err != nil {
		t.Fatalf("partially mark read: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 1 || direct.ReadSeq != 1 {
		t.Fatalf("partial read position: conversation=%+v err=%v", direct, err)
	}
	if err := store.MarkReadThroughByConversationKey(userUUID, directKey, 2); err != nil {
		t.Fatalf("clear unread: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 0 || direct.ReadSeq != 2 || direct.Remark != "teammate" {
		t.Fatalf("remark or unread update: conversation=%+v err=%v", direct, err)
	}

	third := contractConversationMessage("M-"+prefix+"-direct-3", directKey, targetUUID, 3, model.MessageTargetDirect, model.MessageTypeAIText, "answer", now.Add(2*time.Second))
	if err := store.UpsertDirectMessage(userUUID, targetUUID, third, 1); err != nil {
		t.Fatalf("upsert third direct message: %v", err)
	}
	if err := store.MarkReadThroughByConversationKey(userUUID, directKey, 2); err != nil {
		t.Fatalf("mark through stale visible sequence: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 1 || direct.ReadSeq != 2 || direct.LastMessageSeq != 3 {
		t.Fatalf("concurrent newer message was incorrectly marked read: conversation=%+v err=%v", direct, err)
	}
	fourth := contractConversationMessage("M-"+prefix+"-direct-4", directKey, targetUUID, 4, model.MessageTargetDirect, model.MessageTypeText, "sent by current user", now.Add(3*time.Second))
	if err := store.UpsertDirectMessage(userUUID, targetUUID, fourth, 0); err != nil {
		t.Fatalf("upsert sender direct message: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 0 || direct.ReadSeq != 4 || direct.LastMessageUUID != fourth.UUID {
		t.Fatalf("zero increment did not reset unread: conversation=%+v err=%v", direct, err)
	}
	if err := store.UpsertDirectMessage(userUUID, targetUUID, third, 1); err != nil {
		t.Fatalf("replay older direct message: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 0 || direct.ReadSeq != 4 || direct.LastMessageUUID != fourth.UUID {
		t.Fatalf("older projection regressed conversation: conversation=%+v err=%v", direct, err)
	}

	groupUUID := "G-" + prefix + "-conversation"
	groupKey := model.GroupConversationKey(groupUUID)
	if err := store.InitGroupConversation(userUUID, groupUUID, groupKey, now.Add(-time.Hour)); err != nil {
		t.Fatalf("init group conversation: %v", err)
	}
	if err := store.InitGroupConversation(userUUID, groupUUID, groupKey, now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("repeat group init: %v", err)
	}
	initialized, err := store.GetByUserAndConversationKey(userUUID, groupKey)
	if err != nil || initialized == nil || initialized.TargetType != model.MessageTargetGroup || !initialized.LastMessageAt.Equal(now.Add(-time.Hour)) {
		t.Fatalf("idempotent group init: conversation=%+v err=%v", initialized, err)
	}

	groupMessage := contractConversationMessage("M-"+prefix+"-group-1", groupKey, groupUUID, 1, model.MessageTargetGroup, model.MessageTypeSystem, "group updated", now.Add(4*time.Second))
	if err := store.UpsertGroupMessage(userUUID, groupUUID, groupMessage, 1); err != nil {
		t.Fatalf("upsert group message: %v", err)
	}
	lateJoinUser := "U-" + prefix + "-late-join"
	lateJoinMessage := contractConversationMessage("M-"+prefix+"-late-join", groupKey, groupUUID, 100, model.MessageTargetGroup, model.MessageTypeText, "first visible", now.Add(5*time.Second))
	if err := store.UpsertGroupMessage(lateJoinUser, groupUUID, lateJoinMessage, 1); err != nil {
		t.Fatalf("upsert first late-join message: %v", err)
	}
	lateJoinConversation, err := store.GetByUserAndConversationKey(lateJoinUser, groupKey)
	if err != nil || lateJoinConversation == nil || lateJoinConversation.ReadSeq != 99 || lateJoinConversation.UnreadCount != 1 {
		t.Fatalf("late-join read baseline: conversation=%+v err=%v", lateJoinConversation, err)
	}
	if err := store.InitGroupConversation(userUUID, groupUUID, groupKey, now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("init existing group projection: %v", err)
	}
	groupConversation, err := store.GetByUserAndConversationKey(userUUID, groupKey)
	if err != nil || groupConversation == nil || groupConversation.LastMessageUUID != groupMessage.UUID || groupConversation.UnreadCount != 1 {
		t.Fatalf("group init overwrote projection: conversation=%+v err=%v", groupConversation, err)
	}
	if groupConversation.LastMessagePreview != "[system] group updated" {
		t.Fatalf("unexpected system preview: %q", groupConversation.LastMessagePreview)
	}

	listed, err := store.ListByUserUUID(userUUID, 10)
	if err != nil || len(listed) != 2 || listed[0].ConversationKey != groupKey || listed[1].ConversationKey != directKey {
		t.Fatalf("ordered conversation list: conversations=%+v err=%v", listed, err)
	}
	missing, err := store.GetByUserAndConversationKey(userUUID, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing conversation: conversation=%+v err=%v", missing, err)
	}
	if err := store.UpdateRemarkByConversationKey(userUUID, "missing", "ignored"); err != nil {
		t.Fatalf("update missing remark: %v", err)
	}
	if err := store.MarkReadThroughByConversationKey(userUUID, "missing", 1); err != nil {
		t.Fatalf("clear missing unread: %v", err)
	}
	searchScope, err := store.ListSearchConversationKeys(userUUID)
	if err != nil || len(searchScope) != 1 || searchScope[0] != directKey {
		t.Fatalf("direct-only Search scope without group membership: keys=%v err=%v", searchScope, err)
	}
}

func contractConversationMessage(uuid, conversationKey, targetUUID string, seq uint64, targetType, messageType int8, content string, sentAt time.Time) *model.Message {
	return &model.Message{
		UUID:            uuid,
		ConversationKey: conversationKey,
		Seq:             seq,
		SenderUUID:      "U-sender",
		TargetType:      targetType,
		TargetUUID:      targetUUID,
		MessageType:     messageType,
		Content:         content,
		SentAt:          sentAt,
	}
}
