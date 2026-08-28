# C3 Cutover Fault Drill Evidence

This isolated drill validates recoverable delivery-authority cutover at Git revision `6b6eef92e6a7f81806997cfcf57ba8311a23c896` on 2026-08-28. It did not use or mutate the shared `dipole-node1`, `dipole-node2`, or `dipole-node3` services.

## Topology

- Kafka: `apache/kafka:3.9.0`, image `sha256:2956061ba5d3388af8eb3148200f385ccba13ef75bc0eb6dcca5827ebd2d36d4`, one isolated KRaft broker on a random loopback port.
- Redis: `redis:7.4`, image `sha256:54fc9bbc80cdb3b3d8e3a5197732bff502650b62a8bc86f4c3c36e152db4e1af`, on a separate random loopback port.
- Two one-member kafka-go groups represented the compatibility and primary checkpoint positions over two one-partition fixture topics.
- A local TCP fault proxy sat between the production Redis writer/reader and the isolated Redis process. Node observations used the production Redis observation contract for both Gateway and realtime-delivery identities.

The canonical command was:

```bash
DIPOLE_CUTOVER_DRILL_REPORT=benchmarks/c3-cutover-faults-2026-08-28/report.json \
  ./scripts/drill-realtime-cutover-faults.sh
```

The script runs the integration test with the Go race detector, verifies the report with `jq`, and removes the Compose project and volumes on exit.

## Fault Results

- Controller crash: the source checkpoint action artifact was published without a journal event. A newly loaded orchestrator recovered artifact SHA-256 `dee48c0b15479c0574abab8e658e9ed394034107ce26616857a024397c873eab` and committed sequence 1 without repeating the Kafka capture.
- Redis outage: after target activation, the fault proxy closed active connections and rejected new connections. Lease renewal failed and left the journal sequence unchanged; restoring the proxy allowed the same durable renewal path to continue.
- Kafka rebalance/member loss: removing the primary group member made target checkpoint capture fail while the journal remained unchanged. Recreating the member and waiting for both groups to become `Stable` allowed checkpoint and completion.
- Expired freeze rollback: a second attempt used a 500 ms interruption budget. After source checkpoint and freeze, the next action automatically recorded rollback intent, collected a fresh source-node proof against the existing frozen lease, activated Go authority at epoch 2, captured the source checkpoint and ended `rolled_back` at sequence 7 with journal head `1dc51b345e2bbc2bf3a8e37a06b068cedee0ba3eadf91384633ec6ea425de1b7`.
- Forward result: state `completed`, sequence `7`, journal head `798d64cb72482baaf3cd7292953e5edfe30bd697d0d93e2f8c8ab06a6e16bf6d`.

`report.json` is low-sensitive, mode `0600`, and contains no message content, credentials, endpoints, ports, user IDs, or shared-environment identifiers. This drill establishes fail-closed forward recovery under the three named faults and real expired-freeze source rollback. It does not establish unattended lease scheduling or a C++ primary data-plane cutover; those gates remain open under `AD-041`.
