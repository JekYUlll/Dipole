# Dipole Load Test Report

Date: 2026-04-14

## Scope

This round focused on three goals:

1. Verify the frontend group invitation path works for invited members.
2. Package the server as a Docker image for constrained-resource testing.
3. Run a small set of IM-oriented benchmarks on the constrained container.

## Runtime Setup

Dependencies were provided by the existing local `docker-compose.yml`:

- MySQL
- Redis
- Kafka
- MinIO

The application server was packaged into Docker and started with explicit limits:

```bash
docker run -d --rm \
  --name dipole-app-bench \
  --network host \
  --cpus 1 \
  --memory 768m \
  -v /home/horeb/_code/_go/Dipole/configs:/app/configs:ro \
  -v /home/horeb/_code/_go/Dipole/logs:/app/logs \
  dipole-app:bench
```

Confirmed container limits:

- CPU: `1` core (`NanoCpus=1000000000`)
- Memory: `768 MiB` (`805306368`)
- Network mode: `host`

## Functional Verification

### Group invitation visibility

Problem before fix:

- invited members did not receive a usable group event on initial group creation
- frontend only refreshed conversations for member-change events, so the group list stayed stale

Fixes applied:

- `group.created` is now broadcast to initial members
- frontend now handles:
  - `group.created`
  - `group.members_added`
  - `group.members_removed`
  - `group.dismissed`
- invited members automatically fetch group detail and member list after receiving the event

Validation result:

- invited member successfully received `group.created`
- event `group_uuid` matched the newly created group

## Benchmark Items

### 1. Health endpoint

Command:

```bash
hey -n 500 -c 50 http://127.0.0.1:8080/health
```

Result:

- Requests/sec: `14419.29`
- Average latency: `2.2 ms`
- P95 latency: `11.8 ms`
- Status: `500 x 200`

### 2. Login endpoint

Command:

```bash
hey -n 200 -c 20 -m POST \
  -H 'Content-Type: application/json' \
  -d '{"telephone":"13104202221","password":"pass1234"}' \
  'http://127.0.0.1:8080/api/v1/auth/login'
```

Observed result:

- Requests/sec: `209.71`
- Average latency: `93.9 ms`
- Status:
  - `9 x 200`
  - `191 x 429`

Notes:

- this result is dominated by the current login rate limiter
- it reflects protection behavior more than raw login throughput

### 3. Conversations list

Command:

```bash
hey -n 300 -c 30 \
  -H 'Authorization: Bearer <token>' \
  'http://127.0.0.1:8080/api/v1/conversations?limit=50'
```

Result:

- Requests/sec: `1018.15`
- Average latency: `24.7 ms`
- P95 latency: `68.3 ms`
- Status: `300 x 200`

### 4. Direct message history

Command:

```bash
hey -n 300 -c 30 \
  -H 'Authorization: Bearer <token>' \
  'http://127.0.0.1:8080/api/v1/messages/direct/<target_uuid>?limit=30'
```

Result:

- Requests/sec: `1077.05`
- Average latency: `25.7 ms`
- P95 latency: `71.0 ms`
- Status: `300 x 200`

### 5. WebSocket connect concurrency

Test:

- 100 concurrent WebSocket connections
- same authenticated user
- each connection opened and closed once

Result:

- Count: `100`
- Min: `76.41 ms`
- Avg: `86.67 ms`
- P95: `94.70 ms`
- Max: `150.82 ms`

### 6. WebSocket direct message latency

Test:

- one sender and one receiver
- both connected by WebSocket
- 30 direct text messages
- measured:
  - sender to `chat.sent`
  - sender to receiver `chat.message`

Result:

- ACK latency:
  - Min: `1.57 ms`
  - Avg: `2.00 ms`
  - P95: `2.41 ms`
  - Max: `6.68 ms`

- Receiver push latency:
  - Min: `20.97 ms`
  - Avg: `28.09 ms`
  - P95: `39.65 ms`
  - Max: `41.54 ms`

## Key Findings

### Kafka latency issue was fixed

Before tuning:

- sender ACK was around `1000 ms`
- receiver delivery was around `2000 ms`

Root cause:

- Kafka writer was using the default `1s` batch timeout
- consumer `MaxWait` was too high for IM text messaging

After tuning:

- writer batch size set to `1`
- writer batch timeout set to `5ms`
- consumer max wait set to `10ms`

Impact:

- ACK latency dropped from about `1006 ms` to about `2 ms`
- receiver latency dropped from about `2042 ms` to about `28 ms`

## Conclusion

Under a constrained `1 CPU / 768 MiB` Docker container:

- HTTP read APIs stayed in the `20 ms ~ 30 ms` average range
- WebSocket connect latency stayed under `100 ms` on average
- direct-message ACK stayed around `2 ms`
- direct-message receiver push stayed around `28 ms`

For the current project stage, the single-node IM chain is now in a healthy range for functional testing and further feature development.

## Follow-ups

- add a reusable benchmark command to `cmd/wscli` or a dedicated `cmd/imbench`
- add multi-pair concurrent message benchmarks
- add group-message benchmark samples
- add containerized benchmark scripts for one-click rerun
