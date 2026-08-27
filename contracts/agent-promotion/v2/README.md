# Agent Shadow Promotion Policy v2

v2 保留 v1 的 24 小时连续观察、projection 对账和人工评审边界，并将 outcome、trajectory、permission、retrieval、cost 五类离线评测升级为候选版本绑定的 `dipole.agent.offline-eval-report.v1`。报告包含 canonical Suite SHA-256；任一类别缺少 case 或存在失败都会阻断晋级。

v1 解析和决策继续兼容已有证据。新候选使用 `dipole.agent.shadow-promotion-evidence.v2`，字段为 `projectionEvals` 和完整 `offlineEvalReport`，避免继续用无来源布尔值表达 Agent 效果。`eligible` 仍只进入人工评审，不会改变 Runtime mode、Capability 权限或生产流量。

通过门槛后，`publication.schema.json` 定义受控发布输入。`dipole-agent` CLI 将完整证据与 eligible 决策写入 completed Shadow Run 绑定的不可变 `promotion_evaluation` Artifact，控制面只接收 Artifact ID、正文 SHA-256 和 Suite SHA-256。`receipt.schema.json` 定义低敏发布收据；该操作不会自动创建晋升提案、Grant、active Run 或写 Capability。
