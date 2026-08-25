package repository

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMessageRepositoryCreateWithSyncStoresReplaySafeInboxRows(t *testing.T) {
	db, cleanup := setupSyncRepositoryTest(t)
	defer cleanup()

	message := syncTestMessage("M100", "cmid-100")
	repo := NewMessageRepository()
	if err := repo.CreateWithSync(message, []string{"U100", "U200", "U200", ""}); err != nil {
		t.Fatalf("create message with sync inbox: %v", err)
	}

	var rows []*model.UserSyncInbox
	if err := db.Order("sync_seq ASC").Find(&rows).Error; err != nil {
		t.Fatalf("list sync inbox rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two deduplicated inbox rows, got %d", len(rows))
	}
	if rows[0].UserUUID != "U100" || rows[1].UserUUID != "U200" {
		t.Fatalf("unexpected inbox users: %+v", rows)
	}

	if err := repo.EnsureSyncInbox(message, []string{"U100", "U200"}); err != nil {
		t.Fatalf("replay sync inbox projection: %v", err)
	}
	var count int64
	if err := db.Model(&model.UserSyncInbox{}).Count(&count).Error; err != nil {
		t.Fatalf("count sync inbox rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected replay to remain at two rows, got %d", count)
	}
}

func TestSyncRepositoryListsItemsAfterCursorInSequenceOrder(t *testing.T) {
	db, cleanup := setupSyncRepositoryTest(t)
	defer cleanup()

	messages := []*model.Message{
		syncTestMessage("M101", "cmid-101"),
		syncTestMessage("M102", "cmid-102"),
		syncTestMessage("M103", "cmid-103"),
	}
	for _, message := range messages {
		if err := db.Create(message).Error; err != nil {
			t.Fatalf("seed message %s: %v", message.UUID, err)
		}
		if err := db.Create(&model.UserSyncInbox{
			UserUUID:        "U200",
			MessageUUID:     message.UUID,
			ConversationKey: message.ConversationKey,
		}).Error; err != nil {
			t.Fatalf("seed sync row %s: %v", message.UUID, err)
		}
	}

	items, err := NewSyncRepository().ListByUserAfter("U200", 1, 2)
	if err != nil {
		t.Fatalf("list sync items: %v", err)
	}
	if len(items) != 2 || items[0].SyncSeq != 2 || items[1].SyncSeq != 3 {
		t.Fatalf("unexpected sync sequence page: %+v", items)
	}
	if items[0].Message == nil || items[0].Message.UUID != "M102" {
		t.Fatalf("expected joined message M102, got %+v", items[0].Message)
	}
}

func TestMessageRepositoryStoreWithOutboxAndSyncRollsBackTogether(t *testing.T) {
	db, cleanup := setupSyncRepositoryTest(t)
	defer cleanup()

	callbackName := "test:fail_outbox_create"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*model.OutboxEvent); ok {
			tx.AddError(errors.New("forced outbox failure"))
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	defer db.Callback().Create().Remove(callbackName)

	message := syncTestMessage("M200", "cmid-200")
	err := NewMessageRepository().StoreWithOutboxAndSync(message, &model.OutboxEvent{
		AggregateType: "message",
		AggregateID:   message.UUID,
		EventType:     "message.direct.created",
		Topic:         "message.direct.created",
		MessageKey:    message.UUID,
		Value:         []byte(`{}`),
		Status:        model.OutboxStatusPending,
	}, []string{"U100", "U200"})
	if err == nil {
		t.Fatal("expected forced outbox failure")
	}

	for table, value := range map[string]any{
		"messages":        &model.Message{},
		"user_sync_inbox": &model.UserSyncInbox{},
		"outbox_events":   &model.OutboxEvent{},
	} {
		var count int64
		if err := db.Model(value).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("expected %s rollback, got %d rows", table, count)
		}
	}
}

func setupSyncRepositoryTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.UserSyncInbox{}, &model.OutboxEvent{}); err != nil {
		t.Fatalf("auto migrate sqlite: %v", err)
	}

	oldDB := store.DB
	store.DB = db
	return db, func() {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
		store.DB = oldDB
	}
}

func syncTestMessage(uuid, clientMessageID string) *model.Message {
	return &model.Message{
		UUID:            uuid,
		ClientMessageID: clientMessageID,
		ConversationKey: model.DirectConversationKey("U100", "U200"),
		SenderUUID:      "U100",
		TargetType:      model.MessageTargetDirect,
		TargetUUID:      "U200",
		MessageType:     model.MessageTypeText,
		Content:         "hello",
		SentAt:          time.Now().UTC(),
	}
}
