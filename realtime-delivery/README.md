# Dipole Realtime Delivery

This directory contains the C++ realtime data-plane candidate. The foundation milestone is contract-only: it generates C++ types from the canonical `dipole.delivery.v1` Protobuf schema and validates the same golden vectors as Go.

It does not consume Kafka, query Redis, route to Gateway nodes, or appear in production Compose. Those responsibilities are introduced behind independent shadow gates.

```bash
./scripts/check-cpp-realtime.sh
```

The current binary exposes offline validation and a contract-only health process:

```bash
dipole-realtime-delivery validate api/proto/dipole/delivery/v1/testdata
DIPOLE_REALTIME_HOST=127.0.0.1 DIPOLE_REALTIME_PORT=8092 \
  dipole-realtime-delivery serve api/proto/dipole/delivery/v1/testdata
```

`/livez`, `/readyz`, and `/health` return service identity after all golden contracts pass. The only accepted mode is `contract_only`; `shadow` and `cpp` fail closed until their traffic dependencies and evidence gates exist.

Requirements: CMake 3.21+, `/usr/bin/g++` with C++20, Ninja, clang-tidy, pkg-config, and Protobuf compiler/C++ library 3.21+. `CXX`, `CLANG_TIDY_BIN`, and `DIPOLE_CPP_BUILD_DIR` provide explicit toolchain overrides.
