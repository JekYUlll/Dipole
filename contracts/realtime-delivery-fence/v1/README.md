# Realtime Delivery Authority Fence v1

The Redis value at the configured fencing key is one strict JSON object matching `authority.schema.json`. It grants one delivery mode for one deployment epoch until an absolute Unix-millisecond deadline.

- `epoch` is monotonic. A process accepts only the exact epoch supplied by its deployment configuration, preventing an old process from regaining authority after a later rollback to the same mode.
- `phase=active` permits the matching local authority. `phase=frozen` denies all client-write and checkpoint handlers so a controller can establish a no-authority transition window.
- Missing, malformed, expired, unknown-field, duplicate-field, authority-mismatched or epoch-mismatched values fail closed. Redis read failure has the same result.
- Readers revalidate before every message-created side effect. A denied Gateway handler waits on the current Kafka record until the lease becomes valid or the process context is cancelled; authority pauses do not enter the business retry/DLQ path.
- A controller must renew leases, gather live per-node observations and wait for in-flight work before changing phases.

The Go Gateway and C++ Delivery readers are opt-in and consume the shared vectors in `testdata/authority.v1.json`, including exact bounded reason codes. Both runtimes write `observation.schema.json` records at startup and on idle heartbeats. Each 15-second Redis record is keyed by component and stable instance ID, binds the expected mode/epoch to the exact observed lease SHA-256, and reports a bounded authorization reason. Observation persistence is fail-closed. Go message handlers keep the read-only lease reader, while the C++ observed reader throttles publication to five seconds; neither path adds per-message observation writes.

`expected-nodes.schema.json` freezes the exact deployment membership used for one proof. Each node may declare its local `expected_authority`; omission retains compatibility by using the transition authority. The aggregator reads each named Redis key directly, requires a live Redis TTL and an internally valid observation lifetime, and binds every record to the transition receipt's observed authority, epoch, phase, lease deadline and `next_sha256`. An active transition requires every local authority to equal the target and accepts only `authorized/authorized`. A frozen transition permits manifest-declared Go/shadow/C++ preparation states but accepts only `denied/frozen` against the same observed lease. The output follows `observation-aggregate.schema.json` and is eligible-only, with canonical node ordering and manifest SHA-256. Redis key discovery cannot replace the manifest because it cannot prove a missing intended node. The operator CLI below writes guarded transitions, while tracked automation and automatic rollback remain absent; this contract alone does not close `AD-041`.

`checkpoint-manifest.schema.json` names the compatibility and primary Kafka groups plus the exact message topics. A checkpoint collector requires both groups to be `Stable`, assigned to every named partition, committed at the read-committed log end, and in agreement on every log end. ConsumerProtocol assignment versions 0 through 3 retain a canonical topic/partition prefix; the collector parses that bounded prefix from the raw DescribeGroups response and ignores only the remaining client-owned opaque extension bytes. This supports kafka-go and librdkafka members without weakening topic/partition validation. `checkpoint-receipt.schema.json` binds this partition-level zero-lag snapshot to the observation aggregate SHA-256 and lease identity. `checkpoint-bundle.schema.json` stores both records together so the short-lived Redis proof remains independently reviewable.

`attempt-manifest.schema.json` starts one durable cutover attempt and binds its source/target authorities, initial epoch, interruption budget, three expected-node manifests and the dual-group checkpoint manifest by SHA-256. `attempt-event.schema.json` records each accepted state transition as an immutable, monotonically numbered event. The first event links to the canonical manifest SHA-256; every later event links to the canonical previous event. Strict decoding, sequence validation and the hash chain make an interrupted controller recover by replaying evidence instead of guessing which external action completed.

The forward path is `source_checkpointed -> freeze_applied -> frozen_confirmed -> target_activated -> target_checkpointed -> completed`. A rollback requested while the system is frozen may reactivate the source directly. A rollback requested after target activation first requires a second freeze and frozen-node confirmation, then source reactivation and a source-side rollback checkpoint. Each event's `artifact_sha256` must reference the immutable transition receipt or checkpoint bundle that proves the external action.

The orchestrator reloads this journal before every step and executes at most one deterministic action ID. An executor must make that ID idempotent and return the same immutable artifact after an ambiguous process or network failure. Executor failure leaves the journal unchanged so a replacement process retries the same action. Once the first freeze is recorded, exceeding `max_interruption_ms` changes the next forward action into `rollback_requested`; a source reactivation follows without target activation. Explicit rollback after target activation follows the second-freeze path. The generic executor seam is now present, while the production Redis/Kafka adapter, lease renewal and real automatic rollback drills remain pending.

`action-artifact.schema.json` is the local durable idempotency boundary for that executor. Its bounded action ID is derived from attempt ID, sequence and event type. The envelope includes the complete canonical action so an offline or cross-language verifier can recompute `action_sha256` without controller state; `payload_sha256` independently binds the strict JSON transition receipt, observation proof, checkpoint bundle or decision marker. Publication uses a new mode-`0600` file with file and directory fsync. Replaying the same action returns the original envelope; action or payload drift fails closed. The attempt event stores the canonical envelope SHA-256, creating `journal event -> action envelope -> external receipt/bundle` provenance.

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

This CLI relies on OS and Redis access controls; `operator_id` is an audit label and is not independently authenticated. Transition receipts remain in Redis for seven days. Per-node observations alone do not authorize a transition; the node-confirmed checkpoint below must succeed, and automatic rollback plus shared-topology interruption drills remain required before production cutover.

## Node-confirmed checkpoint

Prepare an expected-node manifest from the exact deployment inventory:

```json
{
  "schema_version": "dipole.realtime.delivery-fence-expected-nodes.v1",
  "manifest_id": "cutover-20260828-nodes",
  "nodes": [
    {"component": "gateway", "observer_id": "gateway-a", "expected_authority": "cpp"},
    {"component": "realtime-delivery", "observer_id": "cpp-a", "expected_authority": "cpp"}
  ]
}
```

Prepare the two group identities and fully qualified message topics:

```json
{
  "schema_version": "dipole.realtime.delivery-checkpoint-manifest.v1",
  "manifest_id": "cutover-20260828-checkpoint",
  "topics": [
    "dipole.message.direct.created",
    "dipole.message.group.created"
  ],
  "groups": [
    {"role": "compatibility", "group_id": "dipole-gateway-consumer"},
    {"role": "primary", "group_id": "dipole-realtime-primary-v1"}
  ]
}
```

Run the collector while the named topics are quiescent and every expected node is refreshing its observation:

```bash
DIPOLE_CONFIG_FILE=/path/to/config.yaml go run ./cmd/realtime-cutover-checkpoint \
  -transition-receipt /path/to/transition-receipt.json \
  -expected-nodes /path/to/expected-nodes.json \
  -checkpoint-manifest /path/to/checkpoint-manifest.json \
  -output /path/to/new-checkpoint-bundle.json \
  -confirm
```

The output path must be new. The command rejects missing nodes, stale proof, expired lease, rebalance, incomplete assignments, uncommitted non-empty partitions, nonzero lag, unequal group high water, and log-end movement during the capture window. An empty partition may retain Kafka's raw `committed_offset=-1` only when its read-committed log end is zero. The command never commits offsets or changes the authority lease. Preserve the bundle in the deployment evidence archive before the next transition.
