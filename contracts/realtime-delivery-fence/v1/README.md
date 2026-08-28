# Realtime Delivery Authority Fence v1

The Redis value at the configured fencing key is one strict JSON object matching `authority.schema.json`. It grants one delivery mode for one deployment epoch until an absolute Unix-millisecond deadline.

- `epoch` is monotonic. A process accepts only the exact epoch supplied by its deployment configuration, preventing an old process from regaining authority after a later rollback to the same mode.
- `phase=active` permits the matching local authority. `phase=frozen` denies all client-write and checkpoint handlers so a controller can establish a no-authority transition window.
- Missing, malformed, expired, unknown-field, duplicate-field, authority-mismatched or epoch-mismatched values fail closed. Redis read failure has the same result.
- Readers revalidate before every message-created side effect. A denied Gateway handler waits on the current Kafka record until the lease becomes valid or the process context is cancelled; authority pauses do not enter the business retry/DLQ path.
- A controller must renew leases, gather live per-node observations and wait for in-flight work before changing phases.

The Go Gateway and C++ Delivery readers are opt-in and consume the shared vectors in `testdata/authority.v1.json`. An enabled Go Gateway also writes `observation.schema.json` records at startup and on an idle heartbeat. Each 15-second Redis record is keyed by component and stable Presence node ID, binds the expected mode/epoch to the exact observed lease SHA-256, and reports a bounded authorization reason. Observation persistence is fail-closed; message handlers still use the read-only lease reader to avoid per-message observation writes. The operator CLI below writes guarded transitions, while C++ observations, tracked automation, node-confirmed checkpoint receipts and automatic rollback remain absent; this contract alone does not close `AD-041`.

## Operator transition state machine

`dipole-realtime-authority` is a local operations CLI backed by a Redis Lua compare-and-set. Every non-bootstrap request binds the SHA-256 of the exact current raw lease. The allowed transitions are:

```text
absent -> active(go, epoch=1)
active(A, epoch=N) -> frozen(A, epoch=N+1)
frozen(A, epoch=N) -> active(target, epoch=N)
active|frozen(A, epoch=N) -> same state with a later lease deadline
```

The Lua operation writes the lease with `PEXPIREAT` and an idempotent receipt key in one script. A repeated `transition_id` returns the original receipt only when its canonical request hash matches. Receipts retain operator label, action, state, epoch and hashes; the free-text reason is represented only by SHA-256.

The CLI requires `-confirm`, a fixed absolute `lease_until_unix_ms` that is 5 seconds to 1 hour in the future on first apply, bounded transition/operator IDs and a reason. The absolute deadline is part of the request hash, so a command retry can reproduce the exact request. `freeze`, `activate`, and `renew` require `-expected-sha256`; `bootstrap` only accepts Go; `activate` is the only action that can change authority. Example:

```bash
DIPOLE_CONFIG_FILE=/path/to/config.yaml go run ./cmd/realtime-authority \
  -action freeze \
  -transition-id cutover-20260828-freeze \
  -operator operator-a \
  -reason 'prepare C++ delivery cutover' \
  -expected-sha256 <current-lease-sha256> \
  -lease-until-unix-ms <fixed-absolute-deadline> \
  -confirm
```

This CLI relies on OS and Redis access controls; `operator_id` is an audit label and is not independently authenticated. Receipts remain in Redis for seven days. Go observations alone do not authorize a transition; C++ observations, durable checkpoint receipts, expected-node aggregation and automatic rollback are still required before shared cutover.
