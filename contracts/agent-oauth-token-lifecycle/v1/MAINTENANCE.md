# OAuth Token Lifecycle Maintenance Contract v1

This contract defines the future long-lived authority for lifecycle transitions
after the initial callback handoff. It is a design and release gate only; no
maintenance worker or public route is enabled by this document.

## Authority Separation

The callback lease authorizes exactly one initial `active` or `revoked` write.
It expires with the callback handoff and cannot authorize refresh, revoke, or
expiry maintenance.

Each later operation must hold a distinct maintenance lease bound to:

```text
handoff_uuid
runtime_key_id
maintenance_owner
lease_generation
lease_expires_at
```

Core grants at most one live lease per lifecycle. A replacement worker may
claim only after expiry or explicit release. The worker identity is the mTLS
`dipole-agent` caller plus an allowlisted Runtime deployment identity; model
input, Gateway and browser data cannot set it.

## Opaque Retrieval And CAS

Core may return a lifecycle envelope only after the maintenance lease is
validated and only to the matching `runtime_key_id`. Core never decrypts it.
The response includes the current `token_bundle_sha256` and a lease generation.

A refresh write must provide both values as compare-and-swap preconditions:

```text
expected_token_bundle_sha256
lease_generation
```

Core accepts the update only while the maintenance lease is live, the runtime
key matches and the stored digest equals the expected digest. A successful
refresh replaces the opaque envelope, increments `refresh_count`, writes an
append-only lifecycle event and returns the new digest. Deadline uncertainty
must be reconciled by querying the event or current digest before retrying.

## Allowed Transitions

| Current | Operation | Next | Material |
| --- | --- | --- | --- |
| `active`, `refreshed` | refresh CAS | `refreshed` | replace envelope |
| `active`, `refreshed` | provider permanent failure | `revoked` | clear envelope |
| `active`, `refreshed` | expiry sweep at or after expiry | `expired` | clear envelope |
| `revoked`, `expired` | any maintenance write | rejected | unchanged |

The existing Core expiry primitive may perform the last transition without
returning a lifecycle row. It is intentionally independent from token
retrieval and remains default-unmounted.

## Audit And Retention

Every accepted maintenance transition appends an event containing only:

```text
handoff_uuid, runtime_key_id, operation, prior_state, next_state,
prior_digest, next_digest, lease_generation, maintenance_owner, occurred_at
```

Token plaintext, envelope ciphertext, scope, provider response, request body
and provider error detail are forbidden from events, logs, Kafka, Temporal and
metrics. `revoked` and `expired` clear envelope, digest, expiry and scope in
the same transaction. Retention later removes terminal metadata only after an
audited policy window.

## Enablement Gate

Before a deployment profile enables maintenance, it must provide a Runtime key
allowlist, mTLS identity binding, bounded worker concurrency, lease release on
shutdown, event retention policy, restart/CAS/revocation drills and an
operator-approved rollback. Default Runtime, Compose and Gateway profiles stay
disabled until those artifacts exist.
