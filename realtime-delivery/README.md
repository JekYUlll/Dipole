# Dipole Realtime Delivery

This directory contains the C++ realtime data-plane candidate. It generates C++ types from the canonical `dipole.delivery.v1` Protobuf schema, validates the same golden vectors as Go, and provides a deterministic Kafka-record-to-Delivery projection for message-created events.

The executable remains contract-only. It does not consume Kafka, query Redis, route to Gateway nodes, or appear in production Compose. Library targets now include a librdkafka consumer boundary, deterministic projection, low-sensitivity NDJSON evidence sink and evidence-before-commit runner; no executable composes these pieces yet.

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

Requirements: CMake 3.21+, `/usr/bin/g++` with C++20, Ninja, clang-tidy, pkg-config, Protobuf compiler/C++ library 3.21+, nlohmann/json 3.11+, and librdkafka 2.3+. `CXX`, `CLANG_TIDY_BIN`, `DIPOLE_CPP_COMPILER_PATH`, and `DIPOLE_CPP_BUILD_DIR` provide explicit toolchain overrides. An unpacked Debian package root can be supplied through `DIPOLE_RDKAFKA_ROOT` without installing host packages.
