# C3 Continuous Cutover Controller Evidence

This isolated drill validates controller ownership and process replacement at Git revision `0941f03b58fd21f0c33733af0b255f768f946508` on 2026-08-28. It used a random loopback Redis 7.4 container and did not use or mutate the shared `dipole-node1`, `dipole-node2`, or `dipole-node3` services.

## Scenario

1. Controller process A acquired an attempt-scoped Redis ownership key with a 5-second TTL.
2. A durably committed journal sequence 1, then exited with code 91 without running deferred ownership release.
3. Controller B attempted acquisition before expiry and was rejected.
4. After the real Redis TTL elapsed, B acquired with a different owner token, replayed the same workspace and completed at sequence 6.

The canonical command was:

```bash
DIPOLE_CUTOVER_DRILL_REPORT=/tmp/dipole-c3-controller-final-fault.json \
DIPOLE_CUTOVER_CONTROLLER_DRILL_REPORT=benchmarks/c3-cutover-controller-2026-08-28/report.json \
  ./scripts/drill-realtime-cutover-faults.sh
```

The same race-enabled run also repeated the C++ Primary cutover, controller artifact recovery, Redis outage, Kafka member loss and expired-freeze rollback gates. The Compose project and volumes were removed on exit.

## Result

- Process A exit code: `91`; durable sequence before loss: `1`.
- Pre-expiry replacement: blocked by Redis owner-token comparison.
- Post-expiry replacement: completed from the same journal at sequence `6`.
- Final journal head: `16b7ffd223c1be760b45d1c9273c4a8c2d89300b3ac3c47ca5a576ff3abad0e9`.

`report.json` is low-sensitive, mode `0600`, and contains no credentials, endpoints, ports, user IDs or message content. This evidence closes the process-replacement gate for `AD-041`; tracked deployment remains default-Go and requires an explicit gray-release decision to enable C++ authority.
