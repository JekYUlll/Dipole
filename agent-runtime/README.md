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
npm start
```

Runtime 只接受 `message.direct.created` 的兼容 v1 envelope，使用独立 `dipole-agent-shadow-*` consumer group，并在 consumer 启动完成后开放 `/readyz`。冷启动时 topic metadata 尚未收敛会执行有界重连，每次失败均断开旧 consumer。

Shadow 模式仅生成并审计 metadata plan，Policy Engine 拒绝 write/destructive capability。当前 EventLedger 位于进程内，可收敛单进程重复投递；重启和多副本持久幂等、AI SDK 模型路由与持久审计留在后续切片，详见 `AD-028`。

微服务环境使用根目录 `docker-compose.microservices.yml` 的 `agent` 服务；容器固定 Node 22，且不连接 MySQL、Redis 或 Go 内部 RPC。
