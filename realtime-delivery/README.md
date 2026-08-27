# Dipole Realtime Delivery

This directory contains the C++ realtime data-plane candidate. It generates C++ types from the canonical `dipole.delivery.v1` Protobuf schema, validates the same golden vectors as Go, and provides a deterministic Kafka-record-to-Delivery projection for message-created events.

The executable supports `contract_only` and an explicit `shadow` command. Shadow consumes Kafka into a low-sensitivity NDJSON evidence file and can query Redis Presence when explicitly enabled. It computes node batches without routing to Gateway nodes or writing clients and remains absent from production Compose.

```bash
./scripts/check-cpp-realtime.sh
```

The canonical gate also tests the language-neutral Go/C++ same-workload comparison report. The comparison folds retry attempts by Kafka coordinate, binds both input files and candidate revisions, and fails closed on final transport or workload drift. See `contracts/realtime-delivery-comparison/v1/`.

The current binary exposes offline validation and a contract-only health process:

```bash
dipole-realtime-delivery validate api/proto/dipole/delivery/v1/testdata
DIPOLE_REALTIME_HOST=127.0.0.1 DIPOLE_REALTIME_PORT=8092 \
  dipole-realtime-delivery serve api/proto/dipole/delivery/v1/testdata

DIPOLE_REALTIME_KAFKA_BROKERS=127.0.0.1:9094 \
DIPOLE_REALTIME_KAFKA_GROUP_ID=dipole-realtime-shadow-local-v1 \
DIPOLE_REALTIME_EVIDENCE_FILE=/tmp/dipole-shadow.ndjson \
DIPOLE_REALTIME_PRESENCE_MODE=shadow \
DIPOLE_REALTIME_REDIS_ENDPOINT=127.0.0.1:6379 \
  DIPOLE_REALTIME_NODE_TRANSPORT_MODE=shadow \
  DIPOLE_REALTIME_NODE_TARGETS=gateway-1=127.0.0.1:9095 \
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=replace-me \
  dipole-realtime-delivery shadow api/proto/dipole/delivery/v1/testdata
```

Node transport requires Presence shadow and remains opt-in. Plaintext targets must use loopback. Remote targets require `DIPOLE_REALTIME_NODE_TLS_ENABLED=true` together with `DIPOLE_REALTIME_NODE_TLS_CA_FILE`, `DIPOLE_REALTIME_NODE_TLS_CERT_FILE`, `DIPOLE_REALTIME_NODE_TLS_KEY_FILE`, and `DIPOLE_REALTIME_NODE_TLS_SERVER_NAME`.

`/livez`, `/readyz`, and `/health` return service identity after all golden contracts pass. Contract-only readiness is immediate. Shadow readiness requires a live Kafka partition assignment and a healthy latest poll/project/evidence/commit operation. Client-delivery `cpp` mode remains unavailable.

Requirements: CMake 3.21+, `/usr/bin/g++` with C++20, Ninja, clang-tidy, pkg-config, Protobuf compiler/C++ library 3.21+, gRPC C++ 1.51+, nlohmann/json 3.11+, hiredis 1.2+, and librdkafka 2.3+. `CXX`, `CLANG_TIDY_BIN`, `DIPOLE_CPP_COMPILER_PATH`, and `DIPOLE_CPP_BUILD_DIR` provide explicit toolchain overrides. Unpacked Debian package roots can be supplied through `DIPOLE_RDKAFKA_ROOT` and `DIPOLE_GRPC_ROOT` without installing host packages.

The independent image builds and runs all CTests on Ubuntu 24.04 before copying only the binary, runtime libraries and Delivery golden contracts:

```bash
docker build -f realtime-delivery/Dockerfile -t dipole-realtime-delivery:local .
```

The image is not referenced by production Compose until the remaining routing and comparison gates pass.
