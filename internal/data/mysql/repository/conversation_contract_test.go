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
	gormRepository "github.com/JekYUlll/Dipole/internal/repository"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestConversationRepositoryContract(t *testing.T) {
	db, dsn := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	gormDB, err := gorm.Open(gormMySQL.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open GORM contract database: %v", err)
	}
	sqlcRepo, err := sqlcRepository.NewConversationRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc conversation repository: %v", err)
	}
	stores := map[string]application.ConversationStore{
		"gorm": gormRepository.NewConversationRepositoryWithDB(gormDB),
		"sqlc": sqlcRepo,
	}
	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			runConversationContract(t, store, name)
		})
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
	first := contractConversationMessage("M-"+prefix+"-direct-1", directKey, targetUUID, model.MessageTargetDirect, model.MessageTypeText, longContent, now)
	if err := store.UpsertDirectMessage(userUUID, targetUUID, first, 1); err != nil {
		t.Fatalf("insert direct conversation: %v", err)
	}
	direct, err := store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil {
		t.Fatalf("get direct conversation: conversation=%+v err=%v", direct, err)
	}
	if direct.UnreadCount != 1 || direct.LastMessageUUID != first.UUID || utf8.RuneCountInString(direct.LastMessagePreview) != 100 {
		t.Fatalf("unexpected first direct projection: %+v", direct)
	}

	if err := store.UpsertDirectMessage(userUUID, targetUUID, first, 1); err != nil {
		t.Fatalf("repeat direct message: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 1 {
		t.Fatalf("duplicate message changed unread count: conversation=%+v err=%v", direct, err)
	}

	second := contractConversationMessage("M-"+prefix+"-direct-2", directKey, targetUUID, model.MessageTargetDirect, model.MessageTypeFile, "", now.Add(time.Second))
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
	if err := store.ClearUnreadByConversationKey(userUUID, directKey); err != nil {
		t.Fatalf("clear unread: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 0 || direct.Remark != "teammate" {
		t.Fatalf("remark or unread update: conversation=%+v err=%v", direct, err)
	}

	third := contractConversationMessage("M-"+prefix+"-direct-3", directKey, targetUUID, model.MessageTargetDirect, model.MessageTypeAIText, "answer", now.Add(2*time.Second))
	if err := store.UpsertDirectMessage(userUUID, targetUUID, third, 1); err != nil {
		t.Fatalf("upsert third direct message: %v", err)
	}
	fourth := contractConversationMessage("M-"+prefix+"-direct-4", directKey, targetUUID, model.MessageTargetDirect, model.MessageTypeText, "sent by current user", now.Add(3*time.Second))
	if err := store.UpsertDirectMessage(userUUID, targetUUID, fourth, 0); err != nil {
		t.Fatalf("upsert sender direct message: %v", err)
	}
	direct, err = store.GetByUserAndConversationKey(userUUID, directKey)
	if err != nil || direct == nil || direct.UnreadCount != 0 || direct.LastMessageUUID != fourth.UUID {
		t.Fatalf("zero increment did not reset unread: conversation=%+v err=%v", direct, err)
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

	groupMessage := contractConversationMessage("M-"+prefix+"-group-1", groupKey, groupUUID, model.MessageTargetGroup, model.MessageTypeSystem, "group updated", now.Add(4*time.Second))
	if err := store.UpsertGroupMessage(userUUID, groupUUID, groupMessage, 1); err != nil {
		t.Fatalf("upsert group message: %v", err)
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
	if err := store.ClearUnreadByConversationKey(userUUID, "missing"); err != nil {
		t.Fatalf("clear missing unread: %v", err)
	}
}

func contractConversationMessage(uuid, conversationKey, targetUUID string, targetType, messageType int8, content string, sentAt time.Time) *model.Message {
	return &model.Message{
		UUID:            uuid,
		ConversationKey: conversationKey,
		SenderUUID:      "U-sender",
		TargetType:      targetType,
		TargetUUID:      targetUUID,
		MessageType:     messageType,
		Content:         content,
		SentAt:          sentAt,
	}
}
