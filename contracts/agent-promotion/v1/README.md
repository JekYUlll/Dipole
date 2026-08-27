# Agent Shadow Promotion Policy v1

`policy.json` 是 Agent Shadow 晋级证据的语言中立门槛。候选 Runtime 版本必须在同一 24 小时窗口提供至少 24 个观察点，任意相邻观察间隔不超过 90 分钟，累计比较至少 100 个 Task；所有 projection 对账必须为 `match`，六项 projection Eval 以及 outcome、trajectory、permission Eval 必须全部通过。

`eligible` 仅表示证据达到人工评审门槛，不会修改 `ai.runtime_mode`、Temporal Activity mode 或 Capability 写权限。候选版本、观察窗口、报告和 Eval 结果不能跨版本拼接。

Workflow repair 当前只允许生成 `dipole.agent.workflow-repair-proposal.v1` Artifact。提案必须绑定 Task、操作员声明、工单、Temporal 证据、原因和不超过一小时的有效期，并用 canonical evidence SHA-256 形成稳定 ID。该 Artifact 尚未经过服务端身份签名或持久审批，不能执行修复；`unavailable` 只能先恢复 Worker/Temporal 后重新采证。
