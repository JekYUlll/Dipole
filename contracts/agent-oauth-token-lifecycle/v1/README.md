# Agent OAuth Token Lifecycle Envelope v1

Runtime persists OAuth provider output through Core as an opaque record. This
format is internal-only: it must never be included in browser responses,
Kafka, Temporal history, logs, metrics, traces or audit payloads.

`sealed_token_bundle` is ASCII:

```text
v1.<nonce-base64url>.<ciphertext-base64url>.<tag-base64url>.<wrapped-dek-base64url>
```

- `nonce` is 12 random bytes; `tag` is 16 bytes.
- The plaintext is canonical UTF-8 JSON with ordered keys:
  `access_token`, `expires_at`, `refresh_token`, `scope`, `token_type`.
- A random 32-byte AES-256-GCM data key encrypts the plaintext.
- Runtime derives the public half of the configured `runtime_key_id` RSA key
  and wraps the data key using RSA-OAEP-SHA256. Core persists only the envelope
  and plaintext SHA-256; it has no decryption key.

The AES-GCM AAD is LF-joined UTF-8 fields in this exact order:

```text
dipole.agent.oauth-token-lifecycle.v1
handoff_uuid
runtime_key_id
state
token_bundle_sha256
access_token_expires_at_rfc3339_millis
scope
```

For the initial code exchange, `state=active`. Runtime must submit the envelope
through `PersistOAuthTokenLifecycle` while the callback handoff lease remains
valid, then complete the handoff only after Core acknowledges the same
`handoff_uuid` and state. `refreshed`, `revoked` and `expired` transitions need
their own long-lived authority and remain disabled.
