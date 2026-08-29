# Agent Operations

本目录收纳 Agent 数据维护和一次性迁移操作。当前包含 Memory lineage backfill 的
manifest、审批、执行与回执装配；长期 Agent Runtime 实现位于
`internal/services/agent/` 和 `services/agent-runtime/`。

高风险操作默认 dry-run，执行必须绑定固定 manifest、独立审批身份和可验证回执。
