# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server (development with live reload)
air

# Build
go build -o ./tmp/server ./cmd/services/core

# Run all tests
go test ./...

# Run a single test
go test ./internal/services/message/... -run TestMessageService_SendMessage

# Run tests with verbose output
go test -v ./internal/services/message/...

# Start infrastructure (MySQL, Redis, Kafka, MinIO)
docker compose up -d
```

## Architecture

Dipole is a service-oriented IM monorepo with an embedded compatibility path. The layers are:

```
HTTP/WebSocket → Gateway → Service Application → Domain → SQLC Repository
                                  ↘ Kafka → Sync/Search/Agent projections
                                  ↘ Platform (Redis, MinIO, Presence, Bloom)
```

**Entry points:** Go services live under `cmd/services/`; TypeScript Agent Runtime and C++ Realtime Delivery live under `services/`. Embedded rollback composition is owned by `internal/bootstrap/embedded/`; service runtimes live under their corresponding `internal/services/<service>/bootstrap/` packages.

**Key packages:**
- `internal/bootstrap/embedded` — embedded aggregate initialization and rollback composition
- `internal/services/<service>` — service-owned application, domain, and infrastructure implementations
- `internal/compat/service` — legacy package-path aliases and construction forwards kept for rollback
- `internal/data/mysql/repository` — compatibility adapters only; new SQLC implementations belong to service infrastructure
- `internal/gateway/http` — Gateway-owned Gin edge handlers; thin adapters that call application ports and write responses
- `internal/transport/ws` — WebSocket hub, client lifecycle, message dispatcher, presence integration
- `internal/services/agent/legacy` — Go/Eino compatibility baseline; TypeScript Agent Runtime is the target runtime
- `internal/platform` — infrastructure abstractions: Kafka publisher, Redis cache, MinIO storage, bloom filters, rate limiter, presence tracker

## Non-Obvious Design Decisions

**Dual ID model:** Models have both an auto-increment `ID` (used for DB relations) and a `UUID` (exposed in APIs). Never expose the numeric ID externally.

**Kafka is optional:** When `kafka.enabled: false` in config, services operate synchronously. When enabled, message persistence and conversation updates are published as async events. Embedded handler registration is owned by `internal/bootstrap/embedded/kafka.go`.

**Bloom filters:** Redis-backed bloom filters (`internal/platform/bloom`) gate user/group existence checks before hitting MySQL. They're populated at startup and updated on create.

**WebSocket dispatcher:** `internal/transport/ws/dispatcher.go` routes incoming WS messages to services. It optionally publishes to Kafka for distributed fan-out.

**Rate limiting:** Applied at handler level via `internal/platform/ratelimit`. Limits are configured per operation (login, message send, file upload) in `configs/config.yaml`.

**AI module:** The AI assistant is a first-class participant in conversations — it has its own user account and context builder. It uses the Eino framework and supports OpenAI, Ollama, and DeepSeek providers, configured under `ai:` in config.

## Configuration

Main config: `configs/config.yaml`. Key sections: `app`, `server`, `auth` (JWT), `mysql`, `redis`, `kafka`, `storage` (MinIO), `ratelimit`, `presence`, `ai`.

## Code Style

- Avoid over-engineering upfront — leave room for incremental extension.
- Development is test-driven; write tests before or alongside implementation.
- Reference implementations are in `acc/`: `KamaChat` (learning project) and the two `im-*` projects (commercial). Architectural guidance derived from these is in `docs/architecture/architecture-reference.md`.

## Testing

Tests are co-located with source files. Repository tests use `miniredis` for Redis. Services use interface mocks. No integration test suite — unit tests only.
