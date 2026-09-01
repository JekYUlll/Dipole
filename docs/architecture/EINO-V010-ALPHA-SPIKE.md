# Eino v0.10 Alpha Spike

## Scope

本记录只评估 `github.com/cloudwego/eino v0.10.0-alpha.26`，不修改仓库默认 Go 依赖，也不改变 Go/Eino embedded rollback 路径。版本号来自本机 Go module proxy 的 `go list -m -versions`，源码已下载并完成只读 API 检查。

## Observed Changes

| Area | v0.10.0-alpha.26 observation | Dipole mapping |
| --- | --- | --- |
| ADK Session | typed `SessionEventStore`、append-only session events、event cursor、session lease/busy handling | 可映射到 Agent Task Timeline；当前 Core Timeline 与 TS Runtime audit 仍是项目权威 |
| Checkpoint/Resume | Runner/TurnLoop 支持 checkpoint、interrupt、resume 和部分上下文重建 | 可映射到 Temporal Activity checkpoint；不能替代 Temporal Workflow history |
| Background task | `adk/backgroundtask` 提供 TaskStore、TaskEventStore、lease、CAS version、retry/fail policy 和 durable resume payload | 与当前 AgentTask + Temporal + MySQL authority 重叠，暂不接入生产 |
| Notification | durable notification outbox、receipt、parent session notification 和 replay conflict 检查 | 可参考 Gateway/Agent Timeline notification contract，保留现有 Kafka/Timeline delivery boundary |
| Tool execution | background tool、sub-agent 和 interruption propagation | 可作为未来 Agent capability adapter 的行为参考，权限仍由 Dipole ExecutionContext/Policy Engine 决定 |

## Compatibility Decision

当前保持 `go.mod` 的 `eino v0.9.17`。`v0.10.0-alpha.26` 的 ADK 能力有价值，但 alpha API、持久化协议和任务生命周期仍可能变化；直接升级会同时影响 legacy Eino tool、checkpoint 语义和回滚边界。

后续若做 spike，应使用独立 module 或 build tag，仅验证以下映射：

1. Eino Session Event ID 与 Dipole Agent Task Timeline event ID 的确定性绑定。
2. Eino checkpoint payload 与 Temporal Activity checkpoint 的边界，禁止重复持久化模型状态。
3. Eino background task lease/CAS 与现有 Task/Run lease 的故障恢复对照。
4. Eino notification receipt 与 Kafka/Agent Timeline delivery receipt 的幂等及重放语义。

## Acceptance Boundary

- alpha 依赖不得进入默认 Go build、生产 Compose 或 Agent authority。
- 任何实验必须有独立 revision、明确的 provider/tool 行为测试和回滚说明。
- Temporal 继续负责跨进程 durable execution；Eino 只可作为模型/Agent execution primitive 或 adapter。
- Eino Memory/Session 数据不得绕过 Dipole Capability、owner scope、审计和 retention policy。

## Reproduction

```bash
GOTOOLCHAIN=local go list -m -versions github.com/cloudwego/eino
GOTOOLCHAIN=local go mod download -json github.com/cloudwego/eino@v0.10.0-alpha.26
```

Observed module checksum: `h1:snm9rhq/foSGrhTeDl7LKQSHzr+dVZhR2pLsY2PzEu8=`.
