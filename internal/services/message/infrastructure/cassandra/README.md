# Message Cassandra Infrastructure

本目录归属 Message Service，负责将 Message created event 投影到 Cassandra Message Timeline。Cassandra 仍是可选的 shadow/primary 存储实现，默认运行路径保持关闭。

- `projector.go` 复用 Message domain 的事件解码 contract，只接受 direct/group created event。
- Cassandra Timeline 连接和 append adapter 位于 `internal/platform/cassandra/`。
- Projector runtime 位于 `internal/services/message/bootstrap/`；`cmd/tools/cassandra-projector` 作为独立可选运行入口保留，便于 smoke、暂停和回滚。
- 回滚通过停止 Projector、关闭 Cassandra primary/shadow 开关并恢复 MySQL 主路径完成。
