# Agent Capability API v1

该契约固定 Embedded Go/Eino 与未来 TypeScript Runtime 共享的最小能力集合。`principal_uuid`、`agent_uuid`、request 和 trace 信息由可信执行链注入，不能来自模型 Tool 参数。

当前 Go port 为 `application.AgentCapabilityV1`，本地实现为 `app.LocalAgentCapabilityV1`。后续远程实现应复用相同 operation 语义，并通过 contract test 后再进入 shadow 流量。
