# C2 Gateway Kafka Assignment Readiness Evidence

## Scope

This archive validates the Go Gateway cold-start readiness gate introduced for `AD-039`. It uses an isolated microservices Compose project and the clean image revision recorded in `runtime-provenance.json`. The existing shared three-node Go stack was left running.

The drill contains no message payloads, credentials, certificates, or user identifiers. Kafka evidence is limited to service-owned topic names, partitions, offsets, and generated consumer member IDs.

## Result

- Before startup, `dipole-gateway-consumer` was `Empty` with zero members.
- The first running Gateway sample reported `/readyz=not-ready`, `dipole_service_ready=0`, and `kafka-assignment=0`.
- The final sample reported `/readyz=ready`, `dipole_service_ready=1`, and `kafka-assignment=1`.
- Kafka then reported `Stable`, round-robin assignment, and 20 members covering every registered base/retry topic.
- The transition required 32 samples over about 10.2 seconds, including the configured two-success readiness hysteresis.
- The complete runtime dependency smoke passed, including Elasticsearch failure isolation and recovery without restarting application services.

## Files

- `drill-summary.json`: low-sensitivity drill result and revision binding.
- `startup-timeline.tsv`: high-frequency cold-start readiness samples.
- `group-before-start.txt` and `group-after-start.txt`: initial and final group state.
- `assignments-after-start.txt`: final topic/partition assignment evidence.
- `gateway-metrics-after.prom`: final dependency and service metrics snapshot.
- `gateway.log`: Gateway startup log from the isolated drill.
- `runtime-provenance.json`: image ID, revision, build time, and clean-source label.
- `SHA256SUMS`: archive integrity checksums, excluding this explanatory README.

## Verification

```bash
sha256sum -c SHA256SUMS
awk 'NR == 2 || NR == 33 {print}' startup-timeline.tsv
grep -F 'dipole_dependency_ready{dependency="kafka-assignment",service="dipole-gateway"} 1' gateway-metrics-after.prom
```

The isolated containers, network, and volumes were removed after capture. The temporary certificate directory was removed separately.
