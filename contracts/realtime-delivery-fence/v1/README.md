# Realtime Delivery Authority Fence v1

The Redis value at the configured fencing key is one strict JSON object matching `authority.schema.json`. It grants one delivery mode for one deployment epoch until an absolute Unix-millisecond deadline.

- `epoch` is monotonic. A process accepts only the exact epoch supplied by its deployment configuration, preventing an old process from regaining authority after a later rollback to the same mode.
- `phase=active` permits the matching local authority. `phase=frozen` denies all client-write and checkpoint handlers so a controller can establish a no-authority transition window.
- Missing, malformed, expired, unknown-field, duplicate-field, authority-mismatched or epoch-mismatched values fail closed. Redis read failure has the same result.
- Readers revalidate before every message-created side effect. A denied Gateway handler waits on the current Kafka record until the lease becomes valid or the process context is cancelled; authority pauses do not enter the business retry/DLQ path.
- A future controller must renew leases, gather per-node observations and wait for in-flight work before changing phases.

The current Go Gateway reader is opt-in. No tracked writer, C++ reader, transition receipt or automatic rollback exists yet, so this contract alone does not close `AD-041`.
