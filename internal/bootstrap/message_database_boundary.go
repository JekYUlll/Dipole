package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

var messageOwnedTables = []string{
	"messages",
	"message_metadata",
	"conversation_sequences",
	"group_sync_states",
	"outbox_events",
	"schema_migrations",
	"groups",
	"group_members",
}

var messageAtomicInboxTables = []string{"user_sync_inbox", "user_sync_states"}

var coreOwnedTables = []string{
	"users",
	"contacts",
	"contact_applications",
	"uploaded_files",
	"conversations",
	"device_sync_checkpoints",
	"device_group_sync_checkpoints",
	"message_search_documents",
	"ai_call_logs",
}

type databasePermissionProbe interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func verifyMessageDatabaseBoundary(ctx context.Context, database databasePermissionProbe, inboxWrites bool) error {
	if database == nil {
		return errors.New("message database permission probe is required")
	}
	ownedTables := append([]string(nil), messageOwnedTables...)
	deniedTables := append([]string(nil), coreOwnedTables...)
	if inboxWrites {
		ownedTables = append(ownedTables, messageAtomicInboxTables...)
	} else {
		deniedTables = append(deniedTables, messageAtomicInboxTables...)
	}
	for _, table := range ownedTables {
		if _, err := database.ExecContext(ctx, "SELECT 1 FROM `"+table+"` LIMIT 0"); err != nil {
			return fmt.Errorf("message database account cannot read owned table %s: %w", table, err)
		}
	}
	for _, table := range deniedTables {
		_, err := database.ExecContext(ctx, "SELECT 1 FROM `"+table+"` LIMIT 0")
		if err == nil {
			return fmt.Errorf("message database account can read core-owned table %s", table)
		}
		var mysqlError *mysqlDriver.MySQLError
		if !errors.As(err, &mysqlError) || (mysqlError.Number != 1142 && mysqlError.Number != 1143) {
			return fmt.Errorf("probe core-owned table %s: %w", table, err)
		}
	}
	return nil
}
