package store

import (
	"reflect"
	"testing"

	"github.com/JekYUlll/Dipole/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateCreatesCompositeIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:test_auto_migrate_indexes?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	oldDB := DB
	DB = db
	defer func() {
		DB = oldDB
	}()

	if err := AutoMigrate(); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if !db.Migrator().HasTable("user_sync_states") {
		t.Fatal("expected user_sync_states table to exist")
	}

	checks := []struct {
		model any
		index string
	}{
		{&model.Message{}, "idx_message_conversation_id"},
		{&model.Message{}, "idx_message_target_uuid_id"},
		{&model.Message{}, "idx_message_sender_id"},
		{&model.Message{}, "idx_message_sender_client"},
		{&model.Message{}, "idx_message_file_type_sent"},
		{&model.UserSyncInbox{}, "idx_sync_inbox_user_seq"},
		{&model.UserSyncInbox{}, "idx_sync_inbox_user_message"},
		{&model.Conversation{}, "idx_conversation_user_last_message_at"},
		{&model.ContactApplication{}, "idx_contact_applicant_created"},
		{&model.ContactApplication{}, "idx_contact_target_created"},
		{&model.GroupMember{}, "idx_user_group"},
	}
	for _, check := range checks {
		if !db.Migrator().HasIndex(check.model, check.index) {
			t.Fatalf("expected index %s to exist", check.index)
		}
	}

	var syncIndexColumns []string
	if err := db.Raw("SELECT name FROM pragma_index_info('idx_sync_inbox_user_seq') ORDER BY seqno").Scan(&syncIndexColumns).Error; err != nil {
		t.Fatalf("read sync inbox index columns: %v", err)
	}
	if !reflect.DeepEqual(syncIndexColumns, []string{"user_uuid", "sync_seq"}) {
		t.Fatalf("unexpected sync inbox index columns: %+v", syncIndexColumns)
	}
}
