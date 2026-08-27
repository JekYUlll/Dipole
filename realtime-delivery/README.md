# Dipole Realtime Delivery

This directory contains the C++ realtime data-plane candidate. It generates C++ types from the canonical `dipole.delivery.v1` Protobuf schema, validates the same golden vectors as Go, and provides a deterministic Kafka-record-to-Delivery projection for message-created events.

The executable supports `contract_only` and an explicit `shadow` command. Shadow consumes Kafka into a low-sensitivity NDJSON evidence file and never queries Redis, routes to Gateway nodes, or writes clients. It remains absent from production Compose.

```bash
./scripts/check-cpp-realtime.sh
```

The current binary exposes offline validation and a contract-only health process:

```bash
dipole-realtime-delivery validate api/proto/dipole/delivery/v1/testdata
DIPOLE_REALTIME_HOST=127.0.0.1 DIPOLE_REALTIME_PORT=8092 \
  dipole-realtime-delivery serve api/proto/dipole/delivery/v1/testdata

DIPOLE_REALTIME_KAFKA_BROKERS=127.0.0.1:9094 \
DIPOLE_REALTIME_KAFKA_GROUP_ID=dipole-realtime-shadow-local-v1 \
DIPOLE_REALTIME_EVIDENCE_FILE=/tmp/dipole-shadow.ndjson \
  dipole-realtime-delivery shadow api/proto/dipole/delivery/v1/testdata
```

`/livez`, `/readyz`, and `/health` return service identity after all golden contracts pass. Contract-only readiness is immediate. Shadow readiness requires a live Kafka partition assignment and a healthy latest poll/project/evidence/commit operation. Client-delivery `cpp` mode remains unavailable.

Requirements: CMake 3.21+, `/usr/bin/g++` with C++20, Ninja, clang-tidy, pkg-config, Protobuf compiler/C++ library 3.21+, nlohmann/json 3.11+, hiredis 1.2+, and librdkafka 2.3+. `CXX`, `CLANG_TIDY_BIN`, `DIPOLE_CPP_COMPILER_PATH`, and `DIPOLE_CPP_BUILD_DIR` provide explicit toolchain overrides. An unpacked Debian package root can be supplied through `DIPOLE_RDKAFKA_ROOT` without installing host packages.

The independent image builds and runs all CTests on Ubuntu 24.04 before copying only the binary, runtime libraries and Delivery golden contracts:

```bash
docker build -f realtime-delivery/Dockerfile -t dipole-realtime-delivery:local .
```

The image is not referenced by production Compose until the remaining routing and comparison gates pass.
