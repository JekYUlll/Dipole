# Sync Query RPC v1

`SyncQueryService` provides the durable per-user Inbox Timeline page used for device synchronization.

- `principal_user_id` selects and authorizes the Inbox; clients cannot request another user's timeline.
- `after_seq` is exclusive. Responses return `next_seq` and `has_more` for checkpoint persistence.
- Device checkpoints advance only through explicit acknowledgement and never regress. `RequestContext.device_id` is required for checkpoint operations.
- Group checkpoints combine a durable group Timeline high-water mark with a per-user/device pulled sequence. Every requested group is authorized against Core membership before state is returned or advanced.
- Clients pull stale groups through Message v1 `after_sequence`, then acknowledge only after local durable persistence.
- `page_size=0` uses the application default of 100; values above 200 are capped, and negative values are rejected.
- Each item carries the Message v1 snapshot required for local reconciliation.
- Offline history compatibility remains in Message v1 until clients complete the Sync migration.
