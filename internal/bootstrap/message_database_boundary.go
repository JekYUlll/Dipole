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
	"conversation_sequences",
	"group_sync_states",
	"user_sync_inbox",
	"user_sync_states",
	"outbox_events",
	"schema_migrations",
}

var coreOwnedTables = []string{
	"users",
	"contacts",
	"contact_applications",
	"groups",
	"group_members",
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

func verifyMessageDatabaseBoundary(ctx context.Context, database databasePermissionProbe) error {
	if database == nil {
		return errors.New("message database permission probe is required")
	}
	for _, table := range messageOwnedTables {
		if _, err := database.ExecContext(ctx, "SELECT 1 FROM `"+table+"` LIMIT 0"); err != nil {
			return fmt.Errorf("message database account cannot read owned table %s: %w", table, err)
		}
	}
	for _, table := range coreOwnedTables {
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
