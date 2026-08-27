# Dipole Agent Runtime

TypeScript Agent 执行面。当前 G2 foundation 固定可信 `ExecutionContext`、Capability Registry、resource-scope Policy Engine、Go 兼容 Task ID、shadow 事件幂等和 Fastify 健康面。

```bash
npm ci
npm test
npm run typecheck
npm run build
```

Shadow 模式仅生成并审计计划，Policy Engine 拒绝 write/destructive capability。Kafka、AI SDK 模型路由与持久审计在后续切片通过 port 接入，领域内核不依赖具体 Agent 框架。
