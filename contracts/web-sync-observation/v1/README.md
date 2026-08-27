# Web Sync Observation Evidence v1

该契约把 A6 Web Sync 的 24 小时真实客户端观察从人工截图升级为候选版本绑定的不可变证据。

`session.schema.json` 固定候选版本、完整 Git commit、实际 Web bundle SHA-256、Prometheus 地址、开始时间和初始原始响应。Session ID 覆盖这些字段，修改初始快照会导致身份校验失败。

`evidence.schema.json` 保存满窗后的 recording rule、告警原始响应、判定原因和三层 SHA-256：Session、最终快照以及完整 Evidence。`eligible` 仍只代表 A6 手册中的人工晋级门槛满足，不会修改 Web 构建模式或 Cassandra hydration 路由。

运行证据通常包含环境地址和运行指标，不直接提交到 Git。应归档到受控对象存储并记录 object version、ETag 和保留期；文件不得包含 Prometheus 凭据、Message UUID 或消息正文。
