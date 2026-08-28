# Agent Memory Promotion Receipt v1

该 receipt 绑定 tenant、owner principal、Agent、Temporal Task/Run、候选 ID、候选 SHA-256、review ID、策略版本和短时效窗口。

receipt 不携带 candidate summary、evidence ID 列表、消息正文、凭据或外部工具结果。`receiptSha256` 对规范 body 计算，`receiptId` 对 body 与 receipt hash 计算，因此相同绑定生成相同 ID。

当前 `prepared` receipt 只表示已形成可恢复的晋级意图，不代表 Memory 已写入。过期、取消、绑定漂移或重复请求必须 fail closed。Core 的 accepted candidate promotion 仍要求 Gateway owner 控制入口重新读取 candidate/review，并在 v47 事务中完成实际写入。
