# C3 C++ Primary Cutover Evidence

This isolated drill validates the real C++ Realtime Delivery process as the target authority at Git revision `7057503472655780427679a302e45e3f88edb0ac` on 2026-08-28. It did not use or mutate the shared `dipole-node1`, `dipole-node2`, or `dipole-node3` services.

## Topology

- Kafka: `apache/kafka:3.9.0`, image `sha256:2956061ba5d3388af8eb3148200f385ccba13ef75bc0eb6dcca5827ebd2d36d4`, one isolated KRaft broker on a random loopback port.
- Redis: `redis:7.4`, image `sha256:54fc9bbc80cdb3b3d8e3a5197732bff502650b62a8bc86f4c3c36e152db4e1af`, behind the drill's local fault proxy.
- The compatibility group used kafka-go. The target group was deliberately emptied, then restored by the source-built `dipole-realtime-delivery primary` process using librdkafka and the canonical `dipole.message.*.created` topics.
- The target manifest required `gateway/gateway-a` and `realtime-delivery/cpp-a`. The fixture emitted only the Gateway observation; the C++ process had to validate the CPP active lease and emit its own short-TTL Redis observation.

The canonical command was:

```bash
DIPOLE_CUTOVER_DRILL_REPORT=benchmarks/c3-cutover-cpp-primary-2026-08-28/report.json \
  ./scripts/drill-realtime-cutover-faults.sh
```

The script builds and validates the current C++ source by default, runs the isolated integration test under the Go race detector, verifies the report with `jq`, and removes the Compose project and volumes on exit.

## Results

- C++ process: binary SHA-256 `9614ef018a4f293d7c622b243776f9355d4eaabb742957798e457aeb16f0fcb3`; instance `cpp-a`; readiness reached with a real Kafka assignment and the process stopped cleanly.
- Authority proof: C++ observation SHA-256 `8d942988c2c8c2bff433c49b9365ce7aa86d075145670270769ad0ef351c3ff7` was validated as `authorized`, `authority=cpp`, `epoch=2`, and `phase=active` before target checkpoint publication.
- Forward cutover: controller artifact recovery, Redis outage fail-closed behavior, primary-member loss, C++ member replacement, target checkpoint and completion converged at sequence 7 with journal head `0735c875b043c33c2fbfdb29a16dd3f7e916cdba8847c6c5942d35d3d48c1216`.
- Automatic rollback: the independent 500 ms expired-freeze attempt confirmed source nodes, restored Go active authority and converged at sequence 7 with journal head `915c59acbb2819e1981d25c17e62dfee34a11ecc3d6f999c1dd41b6647cf8c0e`.

`report.json` is low-sensitive, mode `0600`, and contains no message content, credentials, endpoints, ports, user IDs, or shared-environment identifiers. This evidence closes the real C++ target-process gate. A single continuously running controller that owns lease renewal, timeout rollback and leader replacement remains required before a user/node gray release under `AD-041`.
