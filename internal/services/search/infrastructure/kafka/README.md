# Search Indexer Kafka Infrastructure

本目录归属 Search Indexer，负责消费 Message Service 的消息变更事件，并将版本化 mutation 写入 Search Index。

- `projector.go` 复用 Message domain 的事件解码与 Search mutation contract。
- Search 查询权限和 Elasticsearch 读模型由 Search application/platform 负责。
- 索引写入失败由 Kafka consumer 的 retry/DLQ 机制处理，回滚通过停止 Indexer 或切换 Alias 完成。
- 旧 `internal/projector/search` 路径已停止使用。
