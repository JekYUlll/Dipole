package store

import (
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

	checks := []struct {
		model any
		index string
	}{
		{&model.Message{}, "idx_message_conversation_id"},
		{&model.Message{}, "idx_message_target_uuid_id"},
		{&model.Message{}, "idx_message_sender_id"},
		{&model.Message{}, "idx_message_sender_client"},
		{&model.Message{}, "idx_message_file_type_sent"},
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
}
