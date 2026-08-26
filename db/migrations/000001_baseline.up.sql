CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    uuid VARCHAR(24) NOT NULL,
    nickname VARCHAR(20) NOT NULL,
    telephone VARCHAR(11) NOT NULL,
    email VARCHAR(64) NULL,
    avatar VARCHAR(255) NOT NULL DEFAULT '',
    avatar_file_uuid VARCHAR(24) NULL,
    signature VARCHAR(255) NOT NULL DEFAULT '',
    password_hash VARCHAR(255) NOT NULL,
    is_admin TINYINT(1) NOT NULL DEFAULT 0,
    user_type TINYINT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_users_uuid (uuid),
    UNIQUE KEY idx_users_telephone (telephone),
    KEY idx_users_avatar_file_uuid (avatar_file_uuid),
    KEY idx_users_user_type (user_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS messages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    uuid VARCHAR(24) NOT NULL,
    client_message_id VARCHAR(64) NOT NULL,
    conversation_key VARCHAR(64) NOT NULL,
    sender_uuid VARCHAR(24) NOT NULL,
    target_type TINYINT NOT NULL DEFAULT 0,
    target_uuid VARCHAR(24) NOT NULL,
    message_type TINYINT NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    file_id VARCHAR(24) NOT NULL DEFAULT '',
    file_name VARCHAR(255) NOT NULL DEFAULT '',
    file_size BIGINT NOT NULL DEFAULT 0,
    file_url VARCHAR(512) NOT NULL DEFAULT '',
    file_content_type VARCHAR(255) NOT NULL DEFAULT '',
    file_expires_at DATETIME(3) NULL,
    sent_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_messages_uuid (uuid),
    UNIQUE KEY idx_message_sender_client (sender_uuid, client_message_id),
    KEY idx_messages_conversation_key (conversation_key),
    KEY idx_messages_sender_uuid (sender_uuid),
    KEY idx_messages_target_uuid (target_uuid),
    KEY idx_messages_file_id (file_id),
    KEY idx_messages_file_expires_at (file_expires_at),
    KEY idx_messages_sent_at (sent_at),
    KEY idx_message_conversation_id (conversation_key, id),
    KEY idx_message_target_uuid_id (target_type, target_uuid, id),
    KEY idx_message_sender_id (target_type, sender_uuid, id),
    KEY idx_message_file_type_sent (file_id, message_type, sent_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_sync_states (
    user_uuid VARCHAR(24) NOT NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (user_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_sync_inbox (
    sync_seq BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_uuid VARCHAR(24) NOT NULL,
    message_uuid VARCHAR(24) NOT NULL,
    conversation_key VARCHAR(64) NOT NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (sync_seq),
    UNIQUE KEY idx_sync_inbox_user_message (user_uuid, message_uuid),
    KEY idx_sync_inbox_user_seq (user_uuid, sync_seq),
    KEY idx_user_sync_inbox_message_uuid (message_uuid),
    KEY idx_user_sync_inbox_conversation_key (conversation_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS uploaded_files (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    uuid VARCHAR(24) NOT NULL,
    uploader_uuid VARCHAR(24) NOT NULL,
    bucket VARCHAR(128) NOT NULL,
    object_key VARCHAR(255) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    url VARCHAR(512) NOT NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_uploaded_files_uuid (uuid),
    UNIQUE KEY idx_uploaded_files_object_key (object_key),
    KEY idx_uploaded_files_uploader_uuid (uploader_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS conversations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_uuid VARCHAR(24) NOT NULL,
    target_type TINYINT NOT NULL DEFAULT 0,
    target_uuid VARCHAR(24) NOT NULL,
    conversation_key VARCHAR(64) NOT NULL,
    last_message_uuid VARCHAR(24) NOT NULL,
    last_message_type TINYINT NOT NULL DEFAULT 0,
    last_message_preview VARCHAR(255) NOT NULL DEFAULT '',
    last_message_at DATETIME(3) NOT NULL,
    last_message_sender_uuid VARCHAR(24) NOT NULL DEFAULT '',
    unread_count BIGINT NOT NULL DEFAULT 0,
    remark VARCHAR(50) NOT NULL DEFAULT '',
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_user_conversation (user_uuid, conversation_key),
    KEY idx_conversations_user_uuid (user_uuid),
    KEY idx_conversations_target_uuid (target_uuid),
    KEY idx_conversations_last_message_at (last_message_at),
    KEY idx_conversation_user_last_message_at (user_uuid, last_message_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS contacts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_uuid VARCHAR(24) NOT NULL,
    friend_uuid VARCHAR(24) NOT NULL,
    remark VARCHAR(50) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_user_friend (user_uuid, friend_uuid),
    KEY idx_contacts_friend_uuid (friend_uuid),
    KEY idx_contacts_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS contact_applications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    applicant_uuid VARCHAR(24) NOT NULL,
    target_uuid VARCHAR(24) NOT NULL,
    message VARCHAR(255) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 0,
    expires_at DATETIME(3) NULL,
    handled_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_applicant_target (applicant_uuid, target_uuid),
    KEY idx_contact_applications_target_uuid (target_uuid),
    KEY idx_contact_applications_status (status),
    KEY idx_contact_applications_expires_at (expires_at),
    KEY idx_contact_applicant_created (applicant_uuid, created_at),
    KEY idx_contact_target_created (target_uuid, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `groups` (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    uuid VARCHAR(24) NOT NULL,
    name VARCHAR(50) NOT NULL,
    notice VARCHAR(500) NOT NULL DEFAULT '',
    avatar VARCHAR(255) NOT NULL DEFAULT '',
    avatar_file_uuid VARCHAR(24) NULL,
    owner_uuid VARCHAR(24) NOT NULL,
    member_count BIGINT NOT NULL DEFAULT 1,
    status TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_groups_uuid (uuid),
    KEY idx_groups_avatar_file_uuid (avatar_file_uuid),
    KEY idx_groups_owner_uuid (owner_uuid),
    KEY idx_groups_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS group_members (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    group_uuid VARCHAR(24) NOT NULL,
    user_uuid VARCHAR(24) NOT NULL,
    role TINYINT NOT NULL,
    joined_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_group_user (group_uuid, user_uuid),
    KEY idx_group_members_group_uuid (group_uuid),
    KEY idx_group_members_user_uuid (user_uuid),
    KEY idx_user_group (user_uuid, group_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ai_call_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    trigger_message_uuid VARCHAR(24) NOT NULL,
    response_message_uuid VARCHAR(24) NOT NULL DEFAULT '',
    conversation_key VARCHAR(64) NOT NULL,
    user_uuid VARCHAR(24) NOT NULL,
    assistant_uuid VARCHAR(24) NOT NULL,
    provider VARCHAR(32) NOT NULL DEFAULT '',
    model VARCHAR(64) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 0,
    error_message VARCHAR(512) NOT NULL DEFAULT '',
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_ai_call_logs_trigger_message_uuid (trigger_message_uuid),
    KEY idx_ai_call_logs_response_message_uuid (response_message_uuid),
    KEY idx_ai_call_logs_conversation_key (conversation_key),
    KEY idx_ai_call_logs_user_uuid (user_uuid),
    KEY idx_ai_call_logs_assistant_uuid (assistant_uuid),
    KEY idx_ai_call_logs_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    aggregate_type VARCHAR(32) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    topic VARCHAR(128) NOT NULL,
    message_key VARCHAR(128) NOT NULL,
    value LONGBLOB NOT NULL,
    headers_json LONGBLOB NULL,
    status VARCHAR(16) NOT NULL,
    retry_count BIGINT NOT NULL DEFAULT 0,
    last_error VARCHAR(512) NULL,
    next_retry_at DATETIME(3) NULL,
    locked_at DATETIME(3) NULL,
    published_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_outbox_aggregate_event (aggregate_type, aggregate_id, event_type),
    KEY idx_outbox_status_next_retry (status, event_type, next_retry_at),
    KEY idx_outbox_events_locked_at (locked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
