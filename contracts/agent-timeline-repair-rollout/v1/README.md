# Agent Timeline Repair Rollout v1

该契约把 Repair Worker 的隔离/共享观察窗口转换为低敏 `eligible|blocked` 报告。CLI 只读证据和策略文件，不启动、停止或切换 worker；退出码 `0` 表示满足策略，`2` 表示证据有效但仍 blocked，`1` 表示输入无效。

证据必须绑定服务 revision、时间窗口、worker readiness、operator、告警状态、回滚演练和 outcome 聚合。报告只包含哈希、原因和聚合指标，不包含 task/event/message 标识。`eligible` 只允许人工评审下一步灰度，不能自动打开默认生产开关。
