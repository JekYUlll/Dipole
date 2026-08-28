package bootstrap

import (
	"context"
	"errors"
	"fmt"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type syncPermissionProbe struct {
	table string
	verb  string
	query string
}

var syncRequiredPermissionProbes = []syncPermissionProbe{
	{table: "messages", verb: "SELECT", query: "SELECT 1 FROM `messages` LIMIT 0"},
	{table: "outbox_events", verb: "SELECT", query: "SELECT 1 FROM `outbox_events` LIMIT 0"},
	{table: "group_sync_states", verb: "SELECT", query: "SELECT 1 FROM `group_sync_states` LIMIT 0"},
	{table: "schema_migrations", verb: "SELECT", query: "SELECT 1 FROM `schema_migrations` LIMIT 0"},
	{table: "user_sync_inbox", verb: "SELECT", query: "SELECT 1 FROM `user_sync_inbox` LIMIT 0"},
	{table: "user_sync_inbox", verb: "INSERT", query: "INSERT INTO `user_sync_inbox` (`user_uuid`,`message_uuid`,`conversation_key`,`message_seq`) SELECT `user_uuid`,`message_uuid`,`conversation_key`,`message_seq` FROM `user_sync_inbox` WHERE 1=0"},
	{table: "user_sync_inbox", verb: "UPDATE", query: "UPDATE `user_sync_inbox` SET `message_uuid`=`message_uuid` WHERE 1=0"},
	{table: "user_sync_states", verb: "SELECT", query: "SELECT 1 FROM `user_sync_states` LIMIT 0"},
	{table: "user_sync_states", verb: "INSERT", query: "INSERT INTO `user_sync_states` (`user_uuid`) SELECT `user_uuid` FROM `user_sync_states` WHERE 1=0"},
	{table: "user_sync_states", verb: "UPDATE", query: "UPDATE `user_sync_states` SET `user_uuid`=`user_uuid` WHERE 1=0"},
	{table: "device_sync_checkpoints", verb: "SELECT", query: "SELECT 1 FROM `device_sync_checkpoints` LIMIT 0"},
	{table: "device_sync_checkpoints", verb: "INSERT", query: "INSERT INTO `device_sync_checkpoints` (`user_uuid`,`device_id`,`sync_seq`) SELECT `user_uuid`,`device_id`,`sync_seq` FROM `device_sync_checkpoints` WHERE 1=0"},
	{table: "device_sync_checkpoints", verb: "UPDATE", query: "UPDATE `device_sync_checkpoints` SET `sync_seq`=`sync_seq` WHERE 1=0"},
	{table: "device_group_sync_checkpoints", verb: "SELECT", query: "SELECT 1 FROM `device_group_sync_checkpoints` LIMIT 0"},
	{table: "device_group_sync_checkpoints", verb: "INSERT", query: "INSERT INTO `device_group_sync_checkpoints` (`user_uuid`,`device_id`,`group_uuid`,`pulled_message_seq`) SELECT `user_uuid`,`device_id`,`group_uuid`,`pulled_message_seq` FROM `device_group_sync_checkpoints` WHERE 1=0"},
	{table: "device_group_sync_checkpoints", verb: "UPDATE", query: "UPDATE `device_group_sync_checkpoints` SET `pulled_message_seq`=`pulled_message_seq` WHERE 1=0"},
	{table: "sync_replay_jobs", verb: "SELECT", query: "SELECT 1 FROM `sync_replay_jobs` LIMIT 0"},
	{table: "sync_replay_jobs", verb: "INSERT", query: "INSERT INTO `sync_replay_jobs` (`job_name`,`last_error`) SELECT `job_name`,`last_error` FROM `sync_replay_jobs` WHERE 1=0"},
	{table: "sync_replay_jobs", verb: "UPDATE", query: "UPDATE `sync_replay_jobs` SET `status`=`status` WHERE 1=0"},
	{table: "sync_inbox_baseline_jobs", verb: "SELECT", query: "SELECT 1 FROM `sync_inbox_baseline_jobs` LIMIT 0"},
	{table: "sync_inbox_baseline_jobs", verb: "INSERT", query: "INSERT INTO `sync_inbox_baseline_jobs` SELECT * FROM `sync_inbox_baseline_jobs` WHERE 1=0"},
	{table: "sync_inbox_baseline_entries", verb: "SELECT", query: "SELECT 1 FROM `sync_inbox_baseline_entries` LIMIT 0"},
	{table: "sync_inbox_baseline_entries", verb: "INSERT", query: "INSERT INTO `sync_inbox_baseline_entries` SELECT * FROM `sync_inbox_baseline_entries` WHERE 1=0"},
}

var syncDeniedPermissionProbes = []syncPermissionProbe{
	{table: "messages", verb: "INSERT", query: "INSERT INTO `messages` SELECT * FROM `messages` WHERE 1=0"},
	{table: "outbox_events", verb: "UPDATE", query: "UPDATE `outbox_events` SET `status`=`status` WHERE 1=0"},
	{table: "group_sync_states", verb: "UPDATE", query: "UPDATE `group_sync_states` SET `latest_message_seq`=`latest_message_seq` WHERE 1=0"},
	{table: "conversation_sequences", verb: "SELECT", query: "SELECT 1 FROM `conversation_sequences` LIMIT 0"},
	{table: "users", verb: "SELECT", query: "SELECT 1 FROM `users` LIMIT 0"},
	{table: "groups", verb: "SELECT", query: "SELECT 1 FROM `groups` LIMIT 0"},
	{table: "conversations", verb: "SELECT", query: "SELECT 1 FROM `conversations` LIMIT 0"},
	{table: "message_search_documents", verb: "SELECT", query: "SELECT 1 FROM `message_search_documents` LIMIT 0"},
	{table: "ai_call_logs", verb: "SELECT", query: "SELECT 1 FROM `ai_call_logs` LIMIT 0"},
}

func verifySyncDatabaseBoundary(ctx context.Context, database databasePermissionProbe) error {
	if database == nil {
		return errors.New("Sync database permission probe is required")
	}
	for _, probe := range syncRequiredPermissionProbes {
		if _, err := database.ExecContext(ctx, probe.query); err != nil {
			return fmt.Errorf("Sync database account lacks %s on %s: %w", probe.verb, probe.table, err)
		}
	}
	for _, probe := range syncDeniedPermissionProbes {
		_, err := database.ExecContext(ctx, probe.query)
		if err == nil {
			return fmt.Errorf("Sync database account has forbidden %s on %s", probe.verb, probe.table)
		}
		var mysqlError *mysqlDriver.MySQLError
		if !errors.As(err, &mysqlError) || (mysqlError.Number != 1142 && mysqlError.Number != 1143) {
			return fmt.Errorf("probe forbidden %s on %s: %w", probe.verb, probe.table, err)
		}
	}
	return nil
}
