# Agent Temporal Fault v1

本目录定义 `worker_replacement_approval_resume` 的低敏故障演练证据。

- `observation.schema.json`：由受控演练采集的状态修订、注入故障与副作用基数。
- `receipt.schema.json`：由 Runtime 复算并以 canonical SHA-256 绑定的 `eligible|ineligible` 决策。

v1 覆盖隔离 Temporal 中的 Worker replacement、审批恢复、Elicitation input resume 和一次终态写入重试。它不证明共享环境的 Core restart、lease expiry 或 active authority。运行步骤和不变量见 [Agent Temporal Fault Receipts](../../../docs/agent/agent-temporal-fault-receipts.md)。
