# Agent Memory Derived Retention Contract v1

该契约把已验证的 `dipole.agent.memory-derived-lineage-report.v1` 转换为逐持久域的离线治理决策。策略必须完整覆盖 Model Call、Shadow Plan/Step、Artifact、Tool Invocation、Message Action 与 Temporal potential Task；动作词汇固定为擦除派生正文、保留最小审计、按天到期或人工复核。

`policyComplete` 只表示 lineage 完整且所有实际受影响域均有非人工动作。它不表示数据已被删除，也不授予执行权限。决策固定 `contentRead=false`、`deletionExecuted=false`、`deletionAuthority=false` 与 `runtimeAuthority=false`，CLI 只读取两个最大 64 KiB 的本地 JSON 文件，不连接数据库或网络。

`policySha256`、`lineageReportSha256` 与 `decisionSha256` 绑定输入和输出。parser 会根据 `lineageComplete` 与实际受影响域重新推导阻断原因；任何报告 hash 漂移、缺失 lineage、未知字段或 authority 提升都 fail closed。字段级擦除器、retention worker、公开 API、owner 外部收据和生产 Runtime 接线继续由 `AD-035` 阻断。
