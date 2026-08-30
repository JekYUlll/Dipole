# Agent 文档索引

本目录维护 Dipole Agent Runtime 的设计、协议、运维和评估资料。阅读顺序建议从运行时设计开始，再阅读能力安全、任务与记忆，最后执行运维和发布门禁。跨服务 JSON Schema 和版本化证据契约统一收录于 [契约目录](../../contracts/README.md)。

## 快速导航

| 目标 | 文档 |
| --- | --- |
| 理解整体架构 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md) |
| 了解当前能力与边界 | [Agent 前置能力清单](ai-readiness-checklist.md) |
| 管理 Task、Run、Artifact | [Artifact 对账](agent-artifact-reconcile.md) |
| 理解 Memory 生命周期 | [Memory Observation](agent-memory-observation.md) |
| 配置事件订阅与 Shadow | [Subscription Shadow](agent-subscription-shadow.md) |
| 运维 Timeline repair | [Timeline Repair 运维](AGENT-TIMELINE-REPAIR-OPERATIONS.md) |
| 部署 user-gray Active Runtime | [Active 部署运行手册](AGENT-ACTIVE-DEPLOYMENT.md) |
| 接入 MCP 与权限控制 | [MCP 授权](agent-mcp-authorization.md)、[外部 MCP](agent-external-mcp.md) |
| 配置 Trace 与告警 | [OpenTelemetry 运维](agent-otel-operations.md) |

## 运行时边界

- Go IM Core 负责用户、会话、消息、Capability RPC 和权威数据状态。
- TypeScript Runtime 负责 Agent Task、Context Compiler、模型调用、Tool 编排、Memory 和审批等待。
- Kafka 负责事件触发与异步解耦；Temporal 负责可恢复的长任务执行。
- MCP 入口、外部 Server、write/destructive Capability 和 active Runtime 默认关闭。

## 开发与验收顺序

1. 先运行 `services/agent-runtime` 的 `npm test`、`npm run typecheck` 和 `npm run build`。
2. 再运行离线 Eval、promotion gate 和对应的安全回归测试。
3. 需要共享环境时，先完成 readiness、证据哈希、operator 审批和可执行回滚检查。
4. 只有证据覆盖真实窗口后，才评估从 `shadow` 提升到 `user_gray` 或 `active`。

## 证据边界

仓库中的 fixture、离线 Eval、隔离 Compose smoke 和 rollout CLI 只能证明契约或候选环境行为。真实语料、共享 Kafka/Temporal、外部 MCP、生产凭据、active authority 和用户灰度必须单独归档证据，不能由本地测试替代。

新增 Agent 文档时，请同时更新上级 [文档目录](../README.md)、必要时更新 `docs/architecture-docs.manifest`，并在 [CHANGELOG](../../CHANGELOG.md) 记录可验证的变更。
