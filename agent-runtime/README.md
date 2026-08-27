# Dipole Agent Runtime

TypeScript Agent 执行面。当前 G2 foundation 固定可信 `ExecutionContext`、Capability Registry、resource-scope Policy Engine、Go 兼容 Task ID、Kafka shadow consumer 和 Fastify 健康面。

```bash
npm ci
npm test
npm run typecheck
npm run build
```

运行独立 Kafka shadow consumer：

```bash
DIPOLE_AGENT_KAFKA_ENABLED=true \
DIPOLE_AGENT_KAFKA_BROKERS=127.0.0.1:9092 \
DIPOLE_AGENT_KAFKA_GROUP_ID=dipole-agent-shadow-v1 \
DIPOLE_AGENT_KAFKA_TOPIC_PREFIX=dipole \
DIPOLE_AGENT_KAFKA_FAILURE_MAX_ATTEMPTS=3 \
DIPOLE_AGENT_LEDGER_MODE=mysql \
DIPOLE_AGENT_MYSQL_HOST=127.0.0.1 \
DIPOLE_AGENT_MYSQL_USER=dipole_agent \
DIPOLE_AGENT_MYSQL_PASSWORD=change-me \
DIPOLE_AGENT_MYSQL_DATABASE=dipole \
npm start
```

Runtime 只接受 `message.direct.created` 的兼容 v1 envelope，使用独立 `dipole-agent-shadow-*` consumer group，并在 consumer 启动完成后开放 `/readyz`。默认物理 topic 为 `dipole.message.direct.created`，启动时创建并校验 main、`.retry`、`.dead` 的分区与副本配置。冷启动时 topic metadata 尚未收敛会执行有界重连，每次失败均断开旧 consumer。

Shadow 模式仅生成并审计 metadata plan，Policy Engine 拒绝 write/destructive capability。微服务默认使用 MySQL EventLedger，通过 Event ID/Task ID 唯一约束、claim token 与 lease 收敛重启和多副本重复投递；`memory` 只用于显式本地回滚。无效事件直接进入 dead，瞬时处理错误按 `retry_attempt` 有界重试；转移发布失败会让 handler 拒绝完成。AI SDK 模型路由与持久 Tool 轨迹审计留在后续切片。

微服务环境使用根目录 `docker-compose.microservices.yml` 的 `agent` 服务；容器固定 Node 22，只连接 Kafka 与 Agent 自有 MySQL ledger，不连接 Redis 或 Go 内部 RPC。
