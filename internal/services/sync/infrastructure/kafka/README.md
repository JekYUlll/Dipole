# Sync Kafka Infrastructure

本目录归属 Sync Service，负责消费 Message Service 发布的 created event，并将其投影为 User Inbox locator。

- `projector.go` 只处理 Kafka envelope、Message domain event contract 和 Sync projection store。
- Inbox 写入策略、设备 Cursor 和历史 hydration 由 Sync application/infrastructure 负责。
- 热群事件的 `sync_fanout=false` 语义在 Message domain contract 中定义，本目录不重复实现业务规则。
- 旧 `internal/projector/sync` 路径已停止使用；回滚通过关闭 projector、恢复 Message atomic 写入完成。
