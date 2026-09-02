# Multipart Presigned Rollout Evidence v1

This contract decides whether a future release may change the Multipart policy default from `relay` to `presigned`. It does not change runtime routing and it cannot authorize a deployment by itself.

The operator exports one bounded observation window, records the policy SHA-256 produced from the exact policy JSON accepted by the tool, verifies rollback to `relay`, and obtains review from a different operator. First obtain the hash, then bind it in the evidence record:

```bash
scripts/check-multipart-presigned-rollout.sh \
  -policy contracts/multipart-presigned-rollout/v1/default-policy.json \
  -print-policy-sha
```

Evaluate the completed record:

```bash
scripts/check-multipart-presigned-rollout.sh \
  -evidence evidence.json \
  -policy contracts/multipart-presigned-rollout/v1/default-policy.json
```

The command prints an immutable receipt with evidence and policy hashes. It exits `0` only for `eligible`, `2` for a valid but blocked window, and `1` for malformed input. An eligible receipt is a release prerequisite; the release still needs the versioned Multipart policy manifest, a reviewed change and an immediate `relay` rollback path.

`attempted` must equal the sum of terminal outcomes (`directCompleted`, `relayFallback`, `failed`, `expired`, `aborted`). `checksumMismatch` is a subset of `failed`; retries are attempts at individual parts and may exceed terminal upload counts.
