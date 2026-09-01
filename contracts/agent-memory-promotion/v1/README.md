# Agent Memory Promotion Receipt v1/v2

该 receipt 绑定 tenant、owner principal、Agent、Temporal Task/Run、候选 ID、候选 SHA-256、review ID、策略版本和短时效窗口。

receipt 不携带 candidate summary、evidence ID 列表、消息正文、凭据或外部工具结果。`receiptSha256` 对规范 body 计算，`receiptId` 对 body 与 receipt hash 计算，因此相同绑定生成相同 ID。

当前 `prepared` receipt 只表示已形成可恢复的晋级意图，不代表 Memory 已写入。过期、取消、绑定漂移或重复请求必须 fail closed。Core 的 accepted candidate promotion 仍要求 Gateway owner 控制入口重新读取 candidate/review，并在 v47 事务中完成实际写入。

## 持久化目标类型

`PromoteMemoryCandidateRequest.target_memory_type` 允许 owner 在已接受 review 的基础上指定 `episodic`、`semantic`、`procedural` 或 `observational`。空值保持 v1 的 `observational` 兼容语义。Core 会在写入事务前再次校验候选、owner、review、目标类型和既有 promotion，并对重复调用返回同一条已写入 Memory。

`working` 是任务级短期状态，不能通过该持久化入口创建。该 RPC 当前使用 Gateway 的 owner 控制边界；Temporal receipt v2 已绑定目标类型，但 receipt-to-executor 的 active-authority 接线仍为后续工作，不能将本契约表述为 active Runtime 自动写入。
