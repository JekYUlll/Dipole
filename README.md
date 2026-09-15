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
- `conversation.list`, `conversation.read`, `conversation.search`,
  `user.profile.read`, and `contact.list` are read capabilities. Core derives
  the principal from the Task, bounds results, and excludes private profile fields.
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

Requirements: Docker Compose v2, Go (see `go.mod`), Node.js 22+, OpenSSL,
GNU Make and [just](https://github.com/casey/just) (`brew install just`).
`make` owns the build graph; `just --list` lists daily development commands.
The default build includes six Go services and the migration tool. Optional tools
use `make tool-<name>`; the historical benchmark image uses `make legacy-image`.
Run the following commands from the repository root. In your local `.env`, set
`DIPOLE_INTERNAL_RPC_SHARED_SECRET` to a random value generated with
`openssl rand -hex 32`. Keep `.env` private and pass it to Compose with `--env-file`.

Generate development certificates once and build the Go service images:

```bash
just certs
make images
```

The certificate generator replaces existing certificates when invoked directly;
reuse a valid set or deliberately renew the entire set when it expires.
Start the IM stack:

```bash
just up
```

Start the frontend during development:

```bash
just install
just web
```

Open `http://localhost:5173/app/` (or the port printed by Vite).

For the Agent demo, also set these values in `.env`. Use the exact model ID
accepted by your OpenAI-compatible provider; `DIPOLE_AGENT_MODEL_ROUTES` accepts
a comma-separated list of model IDs for fallback:

```dotenv
DIPOLE_AGENT_MODEL_ROUTES=your-model-id
DIPOLE_AGENT_MODEL_API_KEY=your-private-api-key
DIPOLE_AGENT_MODEL_BASE_URL=https://api.deepseek.com/v1
```

Enable Search, Indexer and Elasticsearch with the existing `search` profile.
The experience overlay enables the active TypeScript Runtime, Temporal and
Gateway search; Core uses remote AI execution. Agent and Gateway wait for Search:

```bash
just agent-up
```

The `migrate` service applies schema updates before application startup, including
`000052` for Agent model stages. After source updates, rebuild the Go binaries and
images above so migrations and services match the source revision. Existing data
volumes are reused; back them up before upgrading. This Compose setup is for local
development and uses single-node infrastructure and development database passwords.

Use `COMPOSE_PROJECT_NAME` and `DIPOLE_ENV_FILE` to select your project and private
environment file consistently. `just down` preserves volumes. Build one service
with `make image-core`; use `make image-agent` for the TypeScript image. Build
targets compile before packaging and accept `GO`, `NPM`, `DOCKER`, `IMAGE_TAG`
and existing `DIPOLE_*_IMAGE` overrides. Dependency installation is explicit.

Verify `http://localhost:8080/health`, register two users and send a message with a
distinctive phrase. Wait for it to appear in search, then ask AI to find and summarize
it. Direct AI chat and group `@AI` should reply once. In the AI direct conversation,
ask "Publish a system message here: deployment at 18:00" to propose a system
notice for approval. Reject to verify no write, or approve to send the approved
content once. `/system <text>` remains a deterministic shortcut. Proposals are
limited to the current direct conversation; ordinary AI replies use their
existing restricted reply authorization without an approval prompt.

Conversation Memory: in a direct AI conversation, send
`/remember semantic <fact>` or `/remember episodic <event>`. The Task enters
the existing approval flow before any record is written. Approving resumes the
same Temporal Workflow and stores one owner- and conversation-scoped Memory;
denying writes nothing. Active Memory is included by the Context Compiler only
for later Tasks in that authorized conversation. With
`VITE_AGENT_MEMORIES_ENABLED=true`, the **Agent Memories** page lets the owner
inspect, correct, or revoke saved records.

### Daily Agent Development

The native read tool `get_weather` accepts `{ "city": "Beijing", "countryCode": "CN" }`
and retrieves current conditions from [Open-Meteo](https://open-meteo.com/en/docs).
Ask the Agent "What is the current weather in Beijing?" after rebuilding Core,
Agent and the migration image and applying migration `000053`. No weather API key
is required for the provider's non-commercial endpoint; its usage limits and terms
still apply. The city is sent to Open-Meteo; identities and conversation history
are not included. The result identifies the matched city, timestamp and units.
This tool covers current conditions, not historical weather or multi-day forecasts.
Custom Agent policies need `weather.read` and a `weather/*` read scope; the migration
only updates the built-in Agent policies. Ambiguous cities can use a country code.

Collaboration report: open **Dipole AI** from the conversation list, then choose
**协作总结**. Enter a goal and an information deadline (within seven days). The
same Task retrieves authorized context, asks the owner one question if needed,
and continues with unknown facts marked when the deadline expires. Review or
replace the draft, then approve its exact text for publication. Denial sends
nothing. Input waits survive Worker restart; completed model stages and message
commands are reused. Group reports use the same panel in a group containing AI.
The command form is `/report <ISO deadline with timezone> <goal>`; prefix `@AI`
in groups. The deadline governs missing information, not scheduled publication.
Draft review lasts until one day after the later of its creation and that deadline;
publication approval lasts ten minutes. Task history in the panel follows loaded
conversation messages; task lookup requires HTTPS or localhost. This is a single
owner, single publication flow, without multi-person collection or recurring jobs.
Run `node scripts/smoke-agent-experience.mjs --report-only` for the real report
and deadline/restart scenarios against an already running local experience stack.

Scheduled digest: send
`/digest <ISO timestamp with timezone> <retrieval request>` to the AI user, for
example `/digest 2026-09-16T09:00:00+08:00 Find Cassandra discussions and summarize the decisions`.
Choose a future time within seven days. The task prepares a draft through the
existing context/tool path and shows its full text, destination and UTC publication
time for approval. Approval queues publication at that time; cancellation prevents
dispatch while waiting. Worker downtime beyond the ten-minute publication grace
period expires the task. During the timer wait, the task currently retains its
approval-wait display. In a group, send `@AI /digest <timestamp> <retrieval request>`
to publish the approved draft back to that group. Core rechecks the requesting user's
current group access before dispatch; the Agent must also be able to send to the group.
Direct requests publish to the owner's AI conversation. Natural-language time
clarification remains pending. The group scheduled path has been verified against
the real experience stack using DeepSeek, Temporal, MySQL, Elasticsearch, WebSocket
and Sync, including approval/denial, cancellation and restart during the timer wait.
Apply migrations through `000055` with the updated Core/Agent builds. Approved
message writes recover using the original invocation and committed message receipt.
The real smoke also interrupts completion auditing after message persistence and
restarts the Worker: the original task completes with one invocation and one message.
This verifies the tested failure window, without claiming universal exactly-once delivery.

The experience stack demonstrates conversation context, authorized retrieval,
explicit Memory writes, approval and durable execution. The public MCP server
and external MCP integrations are disabled in this configuration. Internal tool
support does not imply a configured third-party integration. The single-node
infrastructure is a development topology, not a cluster high-availability proof.

Use one running Agent Experience stack for development and demonstration. Model
credentials stay in your private environment file; keep the same Compose project
name on every invocation. Shadow, mock-provider and isolated Temporal tests are
optional regression tools, not prerequisites for demonstrating a feature.

For TypeScript-only changes with unchanged dependencies, update the existing
development container without rebuilding infrastructure:

```bash
export COMPOSE_PROJECT_NAME=dipole-agent-finalization
npm --prefix services/agent-runtime run build
docker cp services/agent-runtime/dist/. "${COMPOSE_PROJECT_NAME}-agent-1:/app/dist"
docker restart "${COMPOSE_PROJECT_NAME}-agent-1"
node scripts/smoke-agent-experience.mjs
```

The smoke uses the real configured model, HTTP/WebSocket, Temporal, Core, MySQL,
Kafka and Elasticsearch. It creates demo users/messages and restarts **only** the
named Agent container to verify approval recovery. Do not run it against a shared
production deployment. It prints IDs/counts, never login tokens or model keys.
Source-only container updates are local iteration aids: rebuild the image when
changing dependencies or recreating containers. Rebuild the migration binary and
image when adding a database migration; Go source changes require rebuilding the
affected service image too.

The single-node experience reserves 20/10/5GB for Elasticsearch disk watermarks.
Keep at least 20GB free. Its health check requires allocated primary shards;
HTTP reachability alone does not establish search readiness.

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
