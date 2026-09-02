# Agent OAuth Callback Handoff Envelope v1

This contract carries an authorization code from Gateway to the Runtime key
boundary. It is an internal persistence format, never a browser, Kafka,
Temporal, audit, or log payload.

`sealed_authorization_code` has this ASCII form:

```text
v1.<nonce-base64url>.<ciphertext-base64url>.<tag-base64url>.<wrapped-dek-base64url>
```

- `nonce` is 12 bytes.
- `tag` is 16 bytes.
- `wrapped-dek` is a 32-byte AES data-encryption key wrapped by the Runtime
  RSA public key with RSA-OAEP-SHA256.
- `ciphertext` uses AES-256-GCM and contains UTF-8 authorization-code bytes.

The data key is randomly generated for each handoff and is zeroed by both
implementations after use. Gateway receives only the Runtime public key;
private key material is confined to the Runtime.

The AES-GCM AAD is UTF-8 fields joined by a single LF, in this exact order:

```text
dipole.agent.oauth-callback-handoff.v1
handoff_uuid
transaction_uuid
owner_user_uuid
issuer
redirect_uri
authorization_code_sha256
runtime_key_id
expires_at_rfc3339_millis
```

`authorization_code_sha256` is lowercase SHA-256 hex of the plaintext code.
The Runtime must recompute it after decryption and reject any mismatch. The
handoff record remains opaque to Core; a Runtime exchange seam is required
before this contract can be used by a callback route.
