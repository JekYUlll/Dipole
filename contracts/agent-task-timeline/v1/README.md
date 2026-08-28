# Agent Task Timeline v1

该契约定义 Gateway 向任务所有者提供的只读执行时间线。它只允许返回 Core 已按认证 principal 复核的 Task/Run 范围；Gateway 不直接访问 Agent 数据库，也不从 Temporal 历史自行拼接事件。

## 目标

- 用稳定的 `event_seq` 表达同一 Task 内的可观察执行顺序。
- 统一展示 Task、Run、Context 编译、Model Call、Tool Invocation、Approval、Input、Artifact 和终态事件。
- 事件正文、模型 prompt/completion、凭据、参数原文和外部不可信内容均不进入公开时间线。
- 允许增量读取和重复请求；游标只在 Core 返回完整页后推进。

## v1 response

```json
{
  "schema_version": "dipole.agent.task_timeline.v1",
  "task_id": "TASK-1",
  "revision": 12,
  "events": [
    {
      "event_seq": 4,
      "event_id": "EVT-4",
      "kind": "tool_invocation",
      "status": "waiting_approval",
      "occurred_at_unix_ms": 1788000000000,
      "run_id": "RUN-1",
      "capability_id": "conversation.read",
      "approval_id": "APR-1"
    }
  ],
  "next_cursor": "4"
}
```

允许的 `kind` 为 `task`、`run`、`context_compile`、`model_call`、`tool_invocation`、`approval`、`input_request`、`artifact` 和 `terminal`。`status` 使用对应持久化域的枚举值，未知值必须 fail closed。

## 读取与权限

- 请求只提交 `task_id`、不透明 `after_cursor` 和 1..100 的 `limit`。
- `tenant_id`、principal、Task owner、Run 绑定和事件范围由认证 Gateway/Core 上下文派生。
- foreign/missing Task 使用统一 NotFound 语义；不得泄露资源存在性。
- Core 必须在同一一致性读取中确认 Task 与 Run 绑定，事件页按 `event_seq ASC` 返回。
- 不完整页、revision 漂移、重复 `event_seq`、跨 Run 事件或游标格式错误均拒绝响应。

## 当前边界

当前仓库已持久化多个执行域，但尚未提供按 Task 聚合并输出本契约的 Core RPC/Gateway adapter。现有 Task Query 继续作为兼容入口；前端只有在 `VITE_AGENT_TASK_TIMELINE_ENABLED=true` 且收到本契约响应后才可展示时间线。该契约本身不开放写操作、Temporal 查询代理或 Artifact 正文下载。
