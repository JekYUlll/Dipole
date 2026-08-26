-- name: GetAdminOverviewCounts :one
SELECT
    (SELECT COUNT(*) FROM users AS all_users) AS user_total,
    (SELECT COUNT(*) FROM users AS admin_users WHERE admin_users.is_admin = TRUE) AS admin_user_total,
    (SELECT COUNT(*) FROM users AS disabled_users WHERE disabled_users.status = sqlc.arg(disabled_user_status)) AS disabled_user_total,
    (SELECT COUNT(*) FROM `groups` AS all_groups) AS group_total,
    (SELECT COUNT(*) FROM `groups` AS dismissed_groups WHERE dismissed_groups.status = sqlc.arg(dismissed_group_status)) AS dismissed_group_total,
    (SELECT COUNT(*) FROM messages AS all_messages) AS message_total,
    (SELECT COUNT(*) FROM conversations AS all_conversations) AS conversation_total,
    (SELECT COUNT(*) FROM contacts AS all_contacts) AS contact_total,
    (SELECT COUNT(*) FROM contact_applications AS pending_applications WHERE pending_applications.status = sqlc.arg(pending_application_status)) AS pending_contact_application_total;
