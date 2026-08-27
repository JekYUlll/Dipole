# Message Event v1

`message-event.schema.json` is the language-neutral producer contract for
confirmed Message facts. `message-send-requested.schema.json` describes the
pre-persistence command consumed by Message Service. Go, TypeScript, and C++
consumers should generate or validate their boundary types from these artifacts
instead of copying Go model definitions.

## Versioning

- Producers emit `version: v1` and all fields listed as required by the schema.
- Consumers accept a missing envelope version as legacy `v1` and accept `v1.x`
  additive changes. Unknown payload fields must be ignored.
- Removing a field, changing its meaning or type, or changing a required field
  requires a new major contract and a dual-read migration window.
- A future major version must use a separate schema directory and remain
  rejected until every active consumer declares support.

The send-requested command intentionally has no mutation revision or
conversation sequence. Confirmed facts add `mutation_type`, `revision`,
`actor_uuid`, and the allocated `message_seq` after durable persistence.

## Legacy created events

Events written before the current producer contract may omit
`mutation_type`, `revision`, `actor_uuid`, `message_seq`, `recipient_uuids`, or
`sync_fanout`. The shared consumer normalizes a created mutation to revision 1
and derives `actor_uuid` from `sender_uuid`. Storage projections may apply
stricter requirements: Sync and Cassandra Timeline require a positive
`message_seq`, and group Inbox projection requires the event-time recipient
snapshot unless `sync_fanout` is explicitly false.
