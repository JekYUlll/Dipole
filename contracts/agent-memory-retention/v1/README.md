# Agent Memory Retention Contract v1

该契约区分检索过期、owner 撤销和内容擦除。v1 内容擦除固定覆盖完整 `memory_root`，保留版本链与最小审计字段，将正文、压缩正文、URI 和自由文本原因替换为固定 tombstone。

`policy.example.json` 只描述约束，没有自动执行或公开 API 权威。`receipt.schema.json` 用 root SHA-256 表示对象，不包含 principal、资源标识或正文；当前内部 Store/Core 收据尚未发布为外部 evidence。

自动 retention worker、公开 owner API、账号级派生数据清理和 Artifact/Message 擦除均不在 v1 切片内，继续由 `AD-035` 阻断。
