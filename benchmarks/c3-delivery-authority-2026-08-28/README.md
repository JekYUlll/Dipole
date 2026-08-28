# C3 Local Delivery Authority Evidence

This archive binds the first C3 delivery-authority drill to binary source revision `7d079f659e2113197b7f07ae0427af77e6597277`. The certificate-permission regression discovered while preparing the drill was fixed separately at archive revision `bbea83f13ca8a6f010c787baf22e70f4e92989c0`.

## Result

- In isolated `go` mode, one direct message produced exactly one matching client frame without a C++ delivery ID. The Gateway exposed `dipole_realtime_delivery_authority{authority="go"} 1` and its consumer reached log end with zero lag.
- In isolated `cpp` mode, the same probe contract produced exactly one matching client frame with a stable C++ delivery ID. The Go compatibility consumer ran checkpoint-only, the C++ primary group committed terminal delivery evidence, and both groups reached log end with zero lag.
- The C++ application readiness endpoint returned `status=ok, mode=primary`. Docker marked the temporary container unhealthy because Compose executed `/dev/tcp` through `/bin/sh`, which lacks that Bash extension; `raw/cpp-container-health.json` preserves the diagnostic.
- Both isolated Compose projects and their volumes were removed. Shared `dipole-node1/2/3` remained running before and after cleanup.

Run `python3 benchmarks/c3-delivery-authority-2026-08-28/verify.py` to regenerate `report.json`. After the report is stable, run `sha256sum --check benchmarks/c3-delivery-authority-2026-08-28/SHA256SUMS`.

## Boundary

This evidence closes the local single-authority frame gate. It does not close `AD-041`: shared dynamic fencing, coordinated dual-group checkpoint receipts, crash/rebalance/Redis-failure cutover drills and automatic rollback remain required before production or user/node gray release.

The archive excludes credentials, message bodies, certificates and private keys. IDs belong to isolated synthetic users and events.
