package messagemysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type messagePermissionProbe struct {
	table string
	verb  string
	query string
}

var messageRequiredPermissionProbes = []messagePermissionProbe{
	{table: "messages", verb: "SELECT", query: "SELECT 1 FROM `messages` LIMIT 0"},
	{table: "messages", verb: "INSERT", query: "INSERT INTO `messages` SELECT * FROM `messages` WHERE 1=0"},
	{table: "message_metadata", verb: "SELECT", query: "SELECT 1 FROM `message_metadata` LIMIT 0"},
	{table: "message_metadata", verb: "INSERT", query: "INSERT INTO `message_metadata` SELECT * FROM `message_metadata` WHERE 1=0"},
	{table: "conversation_sequences", verb: "SELECT", query: "SELECT 1 FROM `conversation_sequences` LIMIT 0"},
	{table: "conversation_sequences", verb: "INSERT", query: "INSERT INTO `conversation_sequences` SELECT * FROM `conversation_sequences` WHERE 1=0"},
	{table: "conversation_sequences", verb: "UPDATE", query: "UPDATE `conversation_sequences` SET `last_seq`=`last_seq` WHERE 1=0"},
	{table: "group_sync_states", verb: "SELECT", query: "SELECT 1 FROM `group_sync_states` LIMIT 0"},
	{table: "group_sync_states", verb: "INSERT", query: "INSERT INTO `group_sync_states` SELECT * FROM `group_sync_states` WHERE 1=0"},
	{table: "group_sync_states", verb: "UPDATE", query: "UPDATE `group_sync_states` SET `latest_message_seq`=`latest_message_seq` WHERE 1=0"},
	{table: "outbox_events", verb: "SELECT", query: "SELECT 1 FROM `outbox_events` LIMIT 0"},
	{table: "outbox_events", verb: "INSERT", query: "INSERT INTO `outbox_events` SELECT * FROM `outbox_events` WHERE 1=0"},
	{table: "outbox_events", verb: "UPDATE", query: "UPDATE `outbox_events` SET `status`=`status` WHERE 1=0"},
	{table: "schema_migrations", verb: "SELECT", query: "SELECT 1 FROM `schema_migrations` LIMIT 0"},
	{table: "groups", verb: "SELECT", query: "SELECT 1 FROM `groups` LIMIT 0"},
	{table: "group_members", verb: "SELECT", query: "SELECT 1 FROM `group_members` LIMIT 0"},
}

var messageAtomicInboxTables = []string{"user_sync_inbox", "user_sync_states"}

var messageAtomicInboxPermissionProbes = []messagePermissionProbe{
	{table: "user_sync_inbox", verb: "SELECT", query: "SELECT 1 FROM `user_sync_inbox` LIMIT 0"},
	{table: "user_sync_inbox", verb: "INSERT", query: "INSERT INTO `user_sync_inbox` SELECT * FROM `user_sync_inbox` WHERE 1=0"},
	{table: "user_sync_inbox", verb: "UPDATE", query: "UPDATE `user_sync_inbox` SET `message_uuid`=`message_uuid` WHERE 1=0"},
	{table: "user_sync_states", verb: "SELECT", query: "SELECT 1 FROM `user_sync_states` LIMIT 0"},
	{table: "user_sync_states", verb: "INSERT", query: "INSERT INTO `user_sync_states` SELECT * FROM `user_sync_states` WHERE 1=0"},
	{table: "user_sync_states", verb: "UPDATE", query: "UPDATE `user_sync_states` SET `user_uuid`=`user_uuid` WHERE 1=0"},
}

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

var messageDeniedPermissionProbes = []messagePermissionProbe{
	{table: "users", verb: "SELECT", query: "SELECT 1 FROM `users` LIMIT 0"},
	{table: "contacts", verb: "SELECT", query: "SELECT 1 FROM `contacts` LIMIT 0"},
	{table: "contact_applications", verb: "SELECT", query: "SELECT 1 FROM `contact_applications` LIMIT 0"},
	{table: "uploaded_files", verb: "SELECT", query: "SELECT 1 FROM `uploaded_files` LIMIT 0"},
	{table: "conversations", verb: "SELECT", query: "SELECT 1 FROM `conversations` LIMIT 0"},
	{table: "device_sync_checkpoints", verb: "SELECT", query: "SELECT 1 FROM `device_sync_checkpoints` LIMIT 0"},
	{table: "device_group_sync_checkpoints", verb: "SELECT", query: "SELECT 1 FROM `device_group_sync_checkpoints` LIMIT 0"},
	{table: "message_search_documents", verb: "SELECT", query: "SELECT 1 FROM `message_search_documents` LIMIT 0"},
	{table: "ai_call_logs", verb: "SELECT", query: "SELECT 1 FROM `ai_call_logs` LIMIT 0"},
}

var messageForbiddenPermissionProbes = []messagePermissionProbe{
	{table: "messages", verb: "UPDATE", query: "UPDATE `messages` SET `uuid`=`uuid` WHERE 1=0"},
	{table: "messages", verb: "DELETE", query: "DELETE FROM `messages` WHERE 1=0"},
	{table: "outbox_events", verb: "DELETE", query: "DELETE FROM `outbox_events` WHERE 1=0"},
	{table: "schema_migrations", verb: "INSERT", query: "INSERT INTO `schema_migrations` SELECT * FROM `schema_migrations` WHERE 1=0"},
	{table: "message_metadata", verb: "UPDATE", query: "UPDATE `message_metadata` SET `message_uuid`=`message_uuid` WHERE 1=0"},
	{table: "message_metadata", verb: "DELETE", query: "DELETE FROM `message_metadata` WHERE 1=0"},
	{table: "conversation_sequences", verb: "DELETE", query: "DELETE FROM `conversation_sequences` WHERE 1=0"},
	{table: "group_sync_states", verb: "DELETE", query: "DELETE FROM `group_sync_states` WHERE 1=0"},
	{table: "groups", verb: "INSERT", query: "INSERT INTO `groups` SELECT * FROM `groups` WHERE 1=0"},
	{table: "group_members", verb: "INSERT", query: "INSERT INTO `group_members` SELECT * FROM `group_members` WHERE 1=0"},
}

type databasePermissionProbe interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// VerifyDatabaseBoundary validates the least-privilege account used by the
// Message service before it starts serving traffic.
func VerifyDatabaseBoundary(ctx context.Context, database databasePermissionProbe, inboxWrites bool) error {
	if database == nil {
		return errors.New("message database permission probe is required")
	}
	required := append([]messagePermissionProbe(nil), messageRequiredPermissionProbes...)
	denied := append([]messagePermissionProbe(nil), messageDeniedPermissionProbes...)
	forbidden := append([]messagePermissionProbe(nil), messageForbiddenPermissionProbes...)
	if inboxWrites {
		required = append(required, messageAtomicInboxPermissionProbes...)
		forbidden = append(forbidden,
			messagePermissionProbe{table: "user_sync_inbox", verb: "DELETE", query: "DELETE FROM `user_sync_inbox` WHERE 1=0"},
			messagePermissionProbe{table: "user_sync_states", verb: "DELETE", query: "DELETE FROM `user_sync_states` WHERE 1=0"},
		)
	} else {
		for _, table := range messageAtomicInboxTables {
			denied = append(denied,
				messagePermissionProbe{table: table, verb: "SELECT", query: "SELECT 1 FROM `" + table + "` LIMIT 0"},
				messagePermissionProbe{table: table, verb: "INSERT", query: "INSERT INTO `" + table + "` SELECT * FROM `" + table + "` WHERE 1=0"},
				messagePermissionProbe{table: table, verb: "UPDATE", query: "UPDATE `" + table + "` SET `user_uuid`=`user_uuid` WHERE 1=0"},
				messagePermissionProbe{table: table, verb: "DELETE", query: "DELETE FROM `" + table + "` WHERE 1=0"},
			)
		}
	}
	for _, probe := range required {
		if _, err := database.ExecContext(ctx, probe.query); err != nil {
			return fmt.Errorf("message database account lacks %s on %s: %w", probe.verb, probe.table, err)
		}
	}
	for _, probe := range append(denied, forbidden...) {
		_, err := database.ExecContext(ctx, probe.query)
		if err == nil {
			return fmt.Errorf("message database account has forbidden %s on %s", probe.verb, probe.table)
		}
		var mysqlError *mysqlDriver.MySQLError
		if !errors.As(err, &mysqlError) || (mysqlError.Number != 1142 && mysqlError.Number != 1143) {
			return fmt.Errorf("probe forbidden %s on %s: %w", probe.verb, probe.table, err)
		}
	}
	return nil
}
