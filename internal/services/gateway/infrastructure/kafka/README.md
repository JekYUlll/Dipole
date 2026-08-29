# Gateway Kafka Infrastructure

本目录负责 Gateway 的 Kafka handler、realtime delivery authority factory、订阅注册、热群通知聚合、direct-conversation read receipt、contact deletion、session kick、群事件、direct-message 和 group-message 投递。Gateway Kafka 实现已完成服务归属迁移；新增基础设施应优先归属本目录，并通过 contract test 验证回滚语义。
