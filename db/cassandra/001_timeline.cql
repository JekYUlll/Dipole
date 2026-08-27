CREATE KEYSPACE IF NOT EXISTS dipole_message_shadow
WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 1}
AND durable_writes = true;

CREATE TABLE IF NOT EXISTS dipole_message_shadow.timeline_by_conversation_bucket (
    conversation_key text,
    bucket bigint,
    message_seq bigint,
    message_uuid text,
    client_message_id text,
    sender_uuid text,
    target_type tinyint,
    target_uuid text,
    message_type tinyint,
    content text,
    file_id text,
    file_name text,
    file_size bigint,
    file_url text,
    file_content_type text,
    file_expires_at timestamp,
    sent_at timestamp,
    event_id text,
    event_version text,
    payload_hash text,
    projected_at timestamp,
    PRIMARY KEY ((conversation_key, bucket), message_seq)
) WITH CLUSTERING ORDER BY (message_seq DESC);
