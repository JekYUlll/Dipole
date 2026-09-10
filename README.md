<p align="center">
  <img src="docs/images/dipole-wordmark.svg" width="560" alt="Dipole" />
</p>

<p align="center">
  面向多端实时协作的即时通信与 Durable Agent 平台。
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Kafka-event%20bus-231F20?logo=apachekafka" alt="Kafka" />
  <img src="https://img.shields.io/badge/Temporal-durable%20workflow-1D4ED8" alt="Temporal" />
</p>

## Overview

Dipole combines a Go IM backend with a TypeScript Agent Runtime. The IM side
supports authenticated direct and group messaging, real-time WebSocket delivery,
multi-device synchronization, history, search, files, and hot-group pull.
The Agent side turns IM events into durable Temporal workflows with bounded
conversation retrieval, server-authorized tools, and human approval for writes.

```text
Client
  | HTTP / WebSocket
  v
Gateway -> Core / Message -> MySQL + Transactional Outbox -> Kafka
              |                                      |          |
              v                                      v          +-> Agent Runtime -> Temporal
         Conversation Timeline                  Sync / Search             |
              |                                      |                       v
              +---------------- Redis Presence / Elasticsearch      Core Message Command
```

## Core Capabilities

### Modern IM

- JWT authentication, contacts, direct messages, groups, conversations, and files.
- WebSocket delivery with Redis-backed presence and cross-node routing.
- Transactional Outbox and idempotent consumers for reliable message events.
- Dual Timeline model: conversation sequence for history, user sync timeline and
  device cursor for offline catch-up and multi-device incremental sync.
- Read sequence, bounded cursor pagination, and hot-group `notify + pull`.
- Asynchronous Elasticsearch indexing with permission-aware conversation search.
- S3-compatible MinIO multipart upload for large files.

### Durable Agent

- Direct AI chat and group `@AI` tasks share one TypeScript Agent Runtime.
- ExecutionContext derives principal and resource scope from trusted server state.
- `conversation.list`, `conversation.read`, and `conversation.search` are
  read capabilities; retrieved evidence is bounded before entering context.
- Temporal persists task state, retries Activities, waits for approval, and
  resumes the same workflow after a worker restart.
- Write tools require approval and execute through an idempotent Core Message
  Command, keeping the business side effect authoritative in the IM domain.

## Service Boundaries

| Component | Responsibility |
| --- | --- |
| Gateway | HTTP/WebSocket access, authentication, connection lifecycle, realtime routing |
| Core | Users, groups, contacts, authorization, conversation state, Agent authority |
| Message | Message persistence, sequence allocation, outbox, history |
| Sync | User inbox timeline, device cursors, incremental synchronization |
| Search / Indexer | Kafka-driven indexing and authorized message search |
| Agent Runtime | Task orchestration, context compilation, capabilities, approvals, Temporal workflows |

## Quick Start

Start the IM stack:

```bash
docker compose -f deploy/compose/docker-compose.microservices.yml up -d
```

Start the frontend during development:

```bash
cd frontend
npm ci
npm run dev
```

To run the complete Agent demo, provide a compatible model API key in `.env` and
add the experience profile. It enables the active TypeScript Runtime and Temporal
while keeping the legacy responder disabled:

```bash
docker compose --env-file .env \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-experience.yml up -d --build
```

## Verification

```bash
scripts/check-go.sh
scripts/check-sqlc.sh
scripts/check-proto.sh
scripts/check-compose.sh

npm --prefix frontend test
npm --prefix frontend run build
npm --prefix services/agent-runtime run typecheck
npm --prefix services/agent-runtime test
```

## Documentation

- [Project overview and demo narrative](docs/guides/PROJECT-LEARNING-AND-INTERVIEW.md)
- [Dipole IM interview guide](docs/guides/INTERVIEW-IM.md)
- [Dipole Agent interview guide](docs/guides/INTERVIEW-AGENT.md)
- [Architecture and operations](docs/README.md)
- [Changelog](CHANGELOG.md)
