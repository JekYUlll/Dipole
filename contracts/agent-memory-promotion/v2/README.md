# Agent Memory Promotion Receipt v2

v2 在 v1 的 tenant、owner、Agent、Task/Run、candidate/review 哈希和短时效绑定基础上，新增 `candidateMemoryType` 与显式 `targetMemoryType`。

当前 candidate 只允许 `observational`。目标类型只能是 `working`、`episodic`、`semantic`、`procedural` 或 `observational`。receipt 只证明经 review 准备的晋级意图；它不提供 Core 写入权限、Runtime authority 或自动 Memory 写入。

历史 v1 receipt 仍可按原 body/hash 读取，因其缺少类型绑定，不能作为 v2 target-aware replay 的输入。该行为要求重新生成带 review binding 的 v2 receipt，避免在恢复流程中推断目标类型。

`receiptSha256` 对规范 body 计算，`receiptId` 对 body 与 receipt hash 计算。candidate summary、evidence、消息正文、凭据和外部工具结果不得进入 receipt。

[Active Executor 设计](ACTIVE-EXECUTOR-DESIGN.md) 固定 receipt 到 Core 的后续提交边界：Runtime 只能提交低敏 receipt binding，Core 重新恢复 Task/Run 的权威身份、验证 active admission 与有效 promotion grant，并在已有 candidate/review 事务中执行幂等写入。
