# Agent Memory Derived Lineage Contract v1

该契约用于在执行 Memory root 内容擦除前，保守统计可能继承该 Memory 内容的 Task 及持久化域。任何 Task 的 Context 选择过目标 root 的任一版本后，其 Model Call、Shadow Plan/Step、Artifact、Tool Invocation、Message Action 与 Temporal History 都被视为可能派生，不进行字段级内容判定。

`manifest.example.json` 含 tenant、owner 和 Memory 标识，只允许在 owner 授权的本地审计路径中使用。`report.example.json` 只保留 root SHA-256、有界计数和完整性标志，不读取或输出 Memory、Context、模型结果、Artifact、Tool 参数或 Message 正文。

新增 Task 在 Shadow Plan 事务中写入规范化 `Memory -> Task` 直接引用。历史 Plan 继续通过 `context_manifest_json` 只做引用探测；存在尚未建立规范化索引的引用时，报告必须返回 `lineageComplete=false`。模型成功结果早于 Plan 提交，审计会将当前 tenant/owner 下已完成调用且尚无 Plan 的 Task 计入 `unattributedModelTasks`，该值非零时也必须声明不完整。派生域计数只表示已规范索引的下界；Temporal 潜在 Task 额外包含已识别的历史引用。v1 不执行历史回填、内容删除、Artifact/Message 擦除或 Temporal 清理。

`contentRead=false`、`deletionAuthority=false` 和 `runtimeAuthority=false` 是固定安全边界。`reportSha256` 对移除该字段后的规范对象执行 SHA-256，Runtime 解析时必须复算。报告仅作为后续人工治理与删除策略设计的影响证据，不能触发运行时行为或自动 retention。
