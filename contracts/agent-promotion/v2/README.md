# Agent Shadow Promotion Policy v2

v2 保留 v1 的 24 小时连续观察、projection 对账和人工评审边界，并将 outcome、trajectory、permission、retrieval、cost 五类离线评测升级为候选版本绑定的 `dipole.agent.offline-eval-report.v1`。报告包含 canonical Suite SHA-256；任一类别缺少 case 或存在失败都会阻断晋级。

v1 解析和决策继续兼容已有证据。新候选使用 `dipole.agent.shadow-promotion-evidence.v2`，字段为 `projectionEvals` 和完整 `offlineEvalReport`，避免继续用无来源布尔值表达 Agent 效果。`eligible` 仍只进入人工评审，不会改变 Runtime mode、Capability 权限或生产流量。
