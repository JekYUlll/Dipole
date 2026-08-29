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

---

# k6 分布式压测（2026-04-16）

## 环境

双节点 Docker Compose 部署（`deploy/compose/docker-compose.dist.yml`）：

- `dipole-node1` → 宿主机 `8081`
- `dipole-node2` → 宿主机 `8082`
- nginx 反向代理 → 宿主机 `80`（轮询两节点）
- 每节点资源限制：`8 CPU / 8 GiB`

k6 脚本：`scripts/bench/bench.js`（三场景）、`scripts/bench/bench_group.js`（500 人群专项）

---

## 问题排查过程

### 问题一：WS 消息投递率为 0

**现象**：k6 运行后 `msg_delivery_rate=0`，`msg_received_total=0`，但服务端日志显示消息正常写入 Kafka 并推送。

**排查过程**：
1. 用 Python `websockets` 脚本手动验证 → 收发正常，排除服务端问题。
2. 检查好友关系逻辑 → 正常。
3. 检查 Kafka 消费端 → 消息正常消费并调用 `SendEventToUser`。
4. 最终定位：k6 WS 回调内使用了 `sleep()`，**阻塞了整个 VU 事件循环**，导致 `message` 事件永远无法被处理。

**根本原因**：k6 的 WS VU 是单线程事件循环，`sleep()` 在回调内会挂起整个循环，所有入站消息在 sleep 期间被丢弃。

**修复**：将所有 WS 回调内的 `sleep()` 替换为 `socket.setInterval()` + `socket.setTimeout()`：

```js
// 错误写法
socket.on("open", () => {
  sleep(1);
  sendMsg(socket, peer, content);
});

// 正确写法
socket.on("open", () => {
  let i = 0;
  socket.setInterval(() => {
    if (i >= 5) return;  // k6 WS socket 无 clearInterval，用计数器守卫
    sendMsg(socket, peer, content);
    i++;
  }, 500);
});
socket.setTimeout(() => { socket.close(); }, 7000);
```

注意：k6 WS socket 没有 `clearInterval` 方法，不能用返回值取消定时器，只能在回调内用计数器提前 return。

---

### 问题二：`configs/config.docker.yaml` 被 sed 命令损坏

**现象**：节点启动后 rate_limit 配置解析失败，`rate_limit:` 段头被删除。

**原因**：`run_bench.sh` 中的清理命令 `sed -i 's/rate_limit.enabled: true/rate_limit.enabled: false/'` 误删了 `rate_limit:` 行（该行恰好匹配了宽松的正则）。

**修复**：手动恢复完整的 `rate_limit:` 配置段，并将 `enabled` 设为 `false`（压测期间禁用限流）。

---

### 问题三：500 人群广播延迟极高（avg 5m36s）

**现象**：`bench_group.js` 500 人群，发送者发 20 条消息，`msg_delivery_rate=100%` 但 avg 延迟 5m36s，p95=6m14s。

**根本原因**：`deliverGroupMessageHandler`（`internal/bootstrap/kafka.go`）对群成员的 WS 推送是**串行循环**：

```go
// 修复前：串行，500 成员 × 20 消息 = 10,000 次顺序调用
for _, recipientUUID := range payload.RecipientUUIDs {
    hub.SendEventToUser(recipientUUID, wsTransport.TypeChatMessage, eventData)
}
```

Kafka 消费者是单 goroutine，每条群消息需要等待所有成员推送完成才能处理下一条，形成严重队列积压。

**修复**：改为并发 fan-out，用 `sync.WaitGroup` 等待所有推送完成：

```go
// 修复后：并发 fan-out
var wg sync.WaitGroup
for _, recipientUUID := range payload.RecipientUUIDs {
    if recipientUUID == payload.SenderUUID {
        continue
    }
    wg.Add(1)
    go func(uuid string) {
        defer wg.Done()
        hub.SendEventToUser(uuid, wsTransport.TypeChatMessage, eventData)
    }(recipientUUID)
}
wg.Wait()
```

---

## 压测结果

### bench.js — 三场景综合（50 用户 / 20 人群）

修复 k6 WS sleep 问题后的结果：

| 指标 | 值 |
|---|---|
| msg_delivery_rate | 100% |
| msg_e2e_latency avg | 448 ms |
| msg_e2e_latency p95 | 884 ms |
| msg_e2e_latency p99 | 1.61 s |
| msg_sent_total | ~200 |
| msg_received_total | ~200 |

阈值 `p95<500ms` 和 `p99<1000ms` 未达标，原因是 Kafka 异步管道本身有约 400ms 的基础延迟（批量写入 + 消费等待）。

### bench_group.js — 500 人群广播专项

串行 fan-out 修复前：

| 指标 | 值 |
|---|---|
| msg_delivery_rate | 100% |
| msg_e2e_latency avg | 5m36s |
| msg_e2e_latency p95 | 6m14s |
| msg_sent_total | 39 |

串行 fan-out 修复后（并发 goroutine fan-out）：预期 avg 延迟降至秒级以内。

---

## 跨节点路由

本轮压测同步完成了 Redis Pub/Sub 跨节点 WS 路由（`internal/transport/ws/pubsub_router.go`）：

- 每个节点订阅 `ws:node:{nodeID}` channel
- `SendEventToUser` 先查 presence 获取目标用户所在节点，本节点直接推送，远端节点通过 Redis PUBLISH 转发
- 降级策略：presence 查询失败时回退到本地推送

---

## 结论与后续

1. **k6 WS 脚本规范**：WS 回调内禁止使用 `sleep()`，一律用 `socket.setInterval()` + 计数器守卫。
2. **群消息 fan-out**：已改为并发推送，消除了 Kafka 消费者的串行瓶颈。
3. **Kafka 延迟基线**：单聊 E2E 约 400ms（Kafka 批量写入开销），可通过进一步降低 `BatchTimeout` 优化。
4. **后续**：在并发 fan-out 修复后重跑 500 人群压测，记录修复后的实际延迟数据。
