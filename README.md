<p align="center">
  <a href="docs/images/README.md"><img src="docs/images/dipole-v3-brand-lockup.svg" width="760" alt="Dipole IM and Dipole Agent brand lockup: navy and red conversation poles, gold Agent orbit" /></a>
</p>

<p align="center">
  A modern, event-driven collaboration platform for real-time messaging and governed Agent workflows.
</p>

<p align="center">
  <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26" /></a>
  <a href="services/agent-runtime"><img src="https://img.shields.io/badge/TypeScript-Agent_Runtime-3178C6?logo=typescript&logoColor=white" alt="TypeScript Agent Runtime" /></a>
  <a href="frontend"><img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white" alt="Vue 3 web client" /></a>
  <a href="frontend/.nvmrc"><img src="https://img.shields.io/badge/Node-22.12-5FA04E?logo=nodedotjs&logoColor=white" alt="Node 22.12" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue" alt="MIT License" /></a>
</p>

<p align="center">
  <a href="https://kafka.apache.org/"><img src="https://img.shields.io/badge/Kafka-Event_Driven-231F20?logo=apachekafka&logoColor=white" alt="Kafka event driven" /></a>
  <a href="https://temporal.io/"><img src="https://img.shields.io/badge/Temporal-Durable_Tasks-000000?logo=temporal&logoColor=white" alt="Temporal durable tasks" /></a>
  <a href="https://sqlc.dev/"><img src="https://img.shields.io/badge/MySQL-sqlc-4479A1?logo=mysql&logoColor=white" alt="MySQL with sqlc generated queries" /></a>
  <a href="docs/README.md"><img src="https://img.shields.io/badge/Redis-Realtime_State-DC382D?logo=redis&logoColor=white" alt="Redis realtime state" /></a>
  <a href="docs/README.md"><img src="https://img.shields.io/badge/MinIO-Multipart_Objects-C72E49?logo=minio&logoColor=white" alt="MinIO multipart object storage" /></a>
</p>

<p align="center">
  <a href="#quick-start">Quick start</a> &middot;
  <a href="docs/README.md">Documentation</a> &middot;
  <a href="services/README.md">Services</a> &middot;
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

## Overview

Dipole provides a real-time IM core with a gradual path to independently deployable services. Go owns IM domain consistency, Kafka carries domain events and projections, and SQLC keeps the MySQL access layer explicit. A TypeScript Agent Runtime integrates through versioned capability contracts and remains fail-closed for privileged paths.

| Area | Current responsibility |
| --- | --- |
| **IM Core** | Authentication, users, contacts, groups, messages and conversations in Go. |
| **Gateway and delivery** | HTTP/WebSocket access, presence, routing and realtime delivery boundaries. |
| **Event and data** | Kafka events, MySQL metadata, Redis realtime state, MinIO objects, with Cassandra and Elasticsearch rollout gates. |
| **Agent Runtime** | TypeScript tasks, ExecutionContext, Capability Policy, Context Compiler and Temporal workflows. |

## Architecture

```text
Client -- HTTP / WebSocket --> Gateway --> Core / Message
                                         |        |
                                         |        +--> MySQL + transactional outbox
                                         v
                                       Kafka
                         +---------------+---------------+
                         |               |               |
                       Sync            Search       Agent Runtime
                         |               |               |
                   Timeline store   Elasticsearch   Temporal / MCP
```

The repository evolves through reversible slices: service contracts precede deployment extraction, and storage migrations use verification and rollback gates. See the [architecture overview](docs/architecture/PLATFORM-EVOLUTION-PLAN.md) for scope, evidence and deferred work.

## Highlights

- **Ordered sync model:** conversation sequence, read sequence and per-device cursor contracts support incremental synchronization.
- **Reliable event boundary:** transactional outbox, idempotent message handling and explicit Kafka projection ownership.
- **Portable data access:** versioned migrations and `database/sql + sqlc`; GORM is retired from the production data path.
- **Large-object workflow:** MinIO multipart upload supports resumable sessions, reconciliation and lifecycle cleanup.
- **Governed Agent execution:** trusted execution context, capability policy, durable Temporal tasks and owner-reviewed Memory promotion.

## Quick Start

### Prerequisites

- Docker Engine with Compose v2
- Go version declared in [`go.mod`](go.mod)
- Node.js version declared by [`frontend/.nvmrc`](frontend/.nvmrc) for the web client

### Run the local stack

```bash
docker compose up -d mysql redis kafka
go run ./cmd/tools/migrate -direction up
go run ./cmd/services/core
```

Run the web client in another terminal:

```bash
cd frontend
npm ci
npm run dev
```

For the isolated microservice smoke topology, use:

```bash
scripts/smoke-microservices-lite.sh
```

The script creates and removes an isolated Compose project. Remote deployment and load-testing procedures live in the [operations documentation](docs/operations/REMOTE-DEV-DEPLOYMENT.md).

## Verify

```bash
scripts/check-go.sh
scripts/check-sqlc.sh
scripts/check-proto.sh
scripts/check-compose.sh
scripts/check-architecture-docs.sh
scripts/check-service-layout.sh
```

For frontend checks:

```bash
cd frontend
npm run test:toolchain
npm run typecheck
npm test
npm run test:design   # Pencil design source matches the shipped UI
npm run test:brand    # docs/images SVGs match their generator
npm run build
```

## Documentation

- [Documentation index](docs/README.md): architecture, data, operations and frontend design.
- [Service catalog](services/README.md): Go, TypeScript and C++ service boundaries.
- [Contracts](contracts/README.md): versioned inter-service protocols.
- [Learning and interview index](docs/guides/PROJECT-LEARNING-AND-INTERVIEW.md): choose the IM or Agent project narrative.
- [IM project material](docs/guides/DIPOLE-IM-LEARNING-AND-INTERVIEW.md): IM system design and interview evidence.
- [Agent project material](docs/guides/DIPOLE-AGENT-LEARNING-AND-INTERVIEW.md): Agent Runtime design and interview evidence.
- [Brand assets](docs/images/README.md): the V3 marks, palette and their generator.
- [Design source](design/README.md): the canonical Pencil document and design changelog.
- [Changelog](CHANGELOG.md): rolling project updates.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a change. Each capability slice should include focused tests, documentation updates, a changelog entry and a rollback boundary where it changes runtime behavior.

## License

Dipole is released under the [MIT License](LICENSE).
