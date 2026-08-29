# Message Bootstrap

本目录是 Message 服务的启动装配边界。runtime 与配置校验测试已迁入本目录，直接组合 Message application、SQLC repository、Outbox、Kafka、Cassandra 和平台能力；少量旧 helper 与 Internal RPC 暂通过兼容入口接入。

Message 入口必须通过本目录初始化；旧共享 runtime 路径已移除。兼容入口保留为下一阶段进一步拆分 Kafka handler、Outbox 和 RPC transport 的回滚边界。
