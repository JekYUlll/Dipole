# Agent Shadow Promotion Policy v1

`policy.json` 是 Agent Shadow 晋级证据的语言中立门槛。候选 Runtime 版本必须在同一 24 小时窗口提供至少 24 个观察点，任意相邻观察间隔不超过 90 分钟，累计比较至少 100 个 Task；所有 projection 对账必须为 `match`，六项 projection Eval 以及 outcome、trajectory、permission Eval 必须全部通过。

`eligible` 仅表示证据达到人工评审门槛，不会修改 `ai.runtime_mode`、Temporal Activity mode 或 Capability 写权限。候选版本、观察窗口、报告和 Eval 结果不能跨版本拼接。

Workflow repair CLI 只生成 `dipole.agent.workflow-repair-proposal.v1` Artifact。Core v25 审计入口随后从 Gateway 认证上下文绑定操作员身份，重新计算同一 canonical evidence SHA-256，并将提案和每位审批人的不可变决定持久化到 MySQL。授权表默认为空，提案人不能审批，批准需要两位不同的有效审批人；任一拒绝会终止提案。当前协议和服务均没有 apply/execute 方法，`approved` 只代表审计门槛满足，不能修改 Workflow projection；`unavailable` 只能先恢复 Worker/Temporal 后重新采证。
