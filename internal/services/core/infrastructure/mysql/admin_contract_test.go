package coremysql_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/generated"
	"github.com/JekYUlll/Dipole/internal/platform/mysql/migration"
	sqlcRepository "github.com/JekYUlll/Dipole/internal/services/core/infrastructure/mysql"
)

func TestAdminRepositoryContract(t *testing.T) {
	db, _ := openContractDatabase(t)
	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate contract database: %v", err)
	}
	seedAdminOverview(t, db)

	sqlcRepo, err := sqlcRepository.NewAdminRepository(generated.New(db))
	if err != nil {
		t.Fatalf("create sqlc admin repository: %v", err)
	}

	want := &application.AdminOverviewCounts{
		UserTotal:                      3,
		AdminUserTotal:                 1,
		DisabledUserTotal:              1,
		GroupTotal:                     2,
		DismissedGroupTotal:            1,
		MessageTotal:                   2,
		ConversationTotal:              1,
		ContactTotal:                   2,
		PendingContactApplicationTotal: 1,
	}
	t.Run("sqlc", func(t *testing.T) {
		got, err := sqlcRepo.OverviewCounts()
		if err != nil {
			t.Fatalf("overview counts: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected counts: got %+v want %+v", got, want)
		}
	})
}

func seedAdminOverview(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO users (uuid, nickname, telephone, password_hash, is_admin, status)
VALUES ('U-user-2', 'disabled', '18800000002', 'hash', FALSE, ?)`, model.UserStatusDisabled); err != nil {
		t.Fatalf("seed disabled user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (uuid, nickname, telephone, password_hash, is_admin, status) VALUES
('U-admin-1', 'admin', '18800000001', 'hash', TRUE, ?),
('U-user-3', 'normal', '18800000003', 'hash', FALSE, ?)`, model.UserStatusNormal, model.UserStatusNormal); err != nil {
		t.Fatalf("seed active users: %v", err)
	}
	_, err := db.Exec(`INSERT INTO ` + "`groups`" + ` (uuid, name, owner_uuid, status) VALUES
('G-admin-1', 'active', 'U-admin-1', 0),
('G-admin-2', 'dismissed', 'U-admin-1', 1);
INSERT INTO messages (uuid, client_message_id, conversation_key, seq, sender_uuid, target_type, target_uuid, message_type, content, sent_at) VALUES
('M-admin-1', 'C-admin-1', 'direct:1', 1, 'U-admin-1', 0, 'U-user-3', 0, 'one', NOW(3)),
('M-admin-2', 'C-admin-2', 'direct:1', 2, 'U-user-3', 0, 'U-admin-1', 0, 'two', NOW(3));
INSERT INTO conversations (user_uuid, target_type, target_uuid, conversation_key, last_message_uuid, last_message_at) VALUES
('U-admin-1', 0, 'U-user-3', 'direct:1', 'M-admin-2', NOW(3));
INSERT INTO contacts (user_uuid, friend_uuid, status) VALUES
('U-admin-1', 'U-user-3', 0),
('U-user-3', 'U-admin-1', 0);
INSERT INTO contact_applications (applicant_uuid, target_uuid, status) VALUES
('U-user-2', 'U-admin-1', 0),
('U-user-3', 'U-admin-1', 1);`)
	if err != nil {
		t.Fatalf("seed admin overview: %v", err)
	}
}
