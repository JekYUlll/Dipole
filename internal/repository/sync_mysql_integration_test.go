package repository

import (
	"os"
	"testing"
	"time"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMySQLSyncSequenceFollowsCommitOrder(t *testing.T) {
	dsn := os.Getenv("DIPOLE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DIPOLE_TEST_MYSQL_DSN is required for MySQL commit-order verification")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open mysql integration database: %v", err)
	}
	models := []any{&model.OutboxEvent{}, &model.UserSyncInbox{}, &model.UserSyncState{}, &model.Message{}}
	if err := db.Migrator().DropTable(models...); err != nil {
		t.Fatalf("reset mysql integration tables: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.UserSyncState{}, &model.UserSyncInbox{}); err != nil {
		t.Fatalf("migrate mysql integration tables: %v", err)
	}
	defer db.Migrator().DropTable(models...)

	firstInboxInserted := make(chan struct{})
	secondMessageInserted := make(chan struct{})
	releaseFirst := make(chan struct{})
	callbackName := "test:sync_commit_order"
	if err := db.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		switch values := tx.Statement.Dest.(type) {
		case *model.Message:
			if values.UUID == "M-commit-2" {
				closeOnce(secondMessageInserted)
			}
		case *[]*model.UserSyncInbox:
			for _, row := range *values {
				if row != nil && row.MessageUUID == "M-commit-1" {
					closeOnce(firstInboxInserted)
					<-releaseFirst
					return
				}
			}
		}
	}); err != nil {
		t.Fatalf("register commit-order callback: %v", err)
	}
	defer db.Callback().Create().Remove(callbackName)

	oldDB := store.DB
	store.DB = db
	defer func() { store.DB = oldDB }()

	repo := NewMessageRepository()
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		firstDone <- repo.CreateWithSync(syncTestMessage("M-commit-1", "cmid-commit-1"), []string{"U200"})
	}()
	waitForSignal(t, firstInboxInserted, "first inbox insert")

	go func() {
		secondDone <- repo.CreateWithSync(syncTestMessage("M-commit-2", "cmid-commit-2"), []string{"U200"})
	}()
	waitForSignal(t, secondMessageInserted, "second message insert")

	select {
	case err := <-secondDone:
		t.Fatalf("expected second transaction to wait for the user sync lock, got %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("commit first message: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("commit second message: %v", err)
	}

	var rows []*model.UserSyncInbox
	if err := db.Where("user_uuid = ?", "U200").Order("sync_seq ASC").Find(&rows).Error; err != nil {
		t.Fatalf("list committed inbox rows: %v", err)
	}
	if len(rows) != 2 || rows[0].MessageUUID != "M-commit-1" || rows[1].MessageUUID != "M-commit-2" {
		t.Fatalf("expected commit-ordered inbox rows, got %+v", rows)
	}
}

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}
