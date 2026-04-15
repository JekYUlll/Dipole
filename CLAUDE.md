# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server (development with live reload)
air

# Build
go build -o ./tmp/server ./cmd/server

# Run all tests
go test ./...

# Run a single test
go test ./internal/service/... -run TestMessageService_SendMessage

# Run tests with verbose output
go test -v ./internal/service/...

# Start infrastructure (MySQL, Redis, Kafka, MinIO)
docker compose up -d
```

## Architecture

Dipole is a modular monolith IM backend. The layers are:

```
HTTP/WebSocket → Handler → Service → Repository → Store (MySQL + Redis)
                                  ↘ Platform (Kafka, MinIO, Cache, Presence, Bloom)
```

**Entry point:** `cmd/server/main.go` → `internal/bootstrap/runtime.go` initializes all dependencies and wires services together.

**Key packages:**
- `internal/bootstrap` — initialization orchestration; `runtime.go` is the composition root
- `internal/service` — all business logic; services are injected with repository interfaces
- `internal/repository` — GORM-based data access; all repos expose interfaces for testability
- `internal/handler/http` — Gin handlers; thin layer that calls services and writes responses
- `internal/transport/ws` — WebSocket hub, client lifecycle, message dispatcher, presence integration
- `internal/modules/ai` — Eino-based AI assistant; has its own DB user (`UserTypeAssistant`) and is initialized at bootstrap
- `internal/platform` — infrastructure abstractions: Kafka publisher, Redis cache, MinIO storage, bloom filters, rate limiter, presence tracker

## Non-Obvious Design Decisions

**Dual ID model:** Models have both an auto-increment `ID` (used for DB relations) and a `UUID` (exposed in APIs). Never expose the numeric ID externally.

**Kafka is optional:** When `kafka.enabled: false` in config, services operate synchronously. When enabled, message persistence and conversation updates are published as async events. See `internal/bootstrap/kafka.go` for handler registration.

**Bloom filters:** Redis-backed bloom filters (`internal/platform/bloom`) gate user/group existence checks before hitting MySQL. They're populated at startup and updated on create.

**WebSocket dispatcher:** `internal/transport/ws/dispatcher.go` routes incoming WS messages to services. It optionally publishes to Kafka for distributed fan-out.

**Rate limiting:** Applied at handler level via `internal/platform/ratelimit`. Limits are configured per operation (login, message send, file upload) in `configs/config.yaml`.

**AI module:** The AI assistant is a first-class participant in conversations — it has its own user account and context builder. It uses the Eino framework and supports OpenAI, Ollama, and DeepSeek providers, configured under `ai:` in config.

## Configuration

Main config: `configs/config.yaml`. Key sections: `app`, `server`, `auth` (JWT), `mysql`, `redis`, `kafka`, `storage` (MinIO), `ratelimit`, `presence`, `ai`.

## Code Style

- Avoid over-engineering upfront — leave room for incremental extension.
- Development is test-driven; write tests before or alongside implementation.
- Reference implementations are in `acc/`: `KamaChat` (learning project) and the two `im-*` projects (commercial). Architectural guidance derived from these is in `docs/architecture-reference.md`.

## Testing

Tests are co-located with source files. Repository tests use `miniredis` for Redis. Services use interface mocks. No integration test suite — unit tests only.
