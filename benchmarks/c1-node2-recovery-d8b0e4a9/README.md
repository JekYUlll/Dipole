# C1 Node Recovery Evidence

## Scope

- Revision: `d8b0e4a9ac902c747e7d4259fd5cedccef439d02`
- Isolated Compose project: `dipole-c1`
- Fault: stop/start `dipole-node2`
- Runtime: CPU/container only; two unrelated GPU tasks remained running

## Result

- Recovery report: `passed=true`
- Fault to unavailable: `518ms`
- Restart to ready: `16093ms`
- Consumer group stable members: `72`
- Post-recovery workload: `40/40` accepted, persisted, and received
- Expected receipts: `40/40`
- HTTP failure rate: `0%`
- Kafka lag: peak `0`, settled `0`
- Container image revision and source cleanliness remained aligned before and after recovery

## Evidence

- `report.json`: validated recovery report
- `evidence.json`: low-sensitivity fault timeline and runtime snapshots

The candidate topology was removed after the run. No unrelated process, container, volume, network, or GPU task was changed.
