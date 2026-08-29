# Gateway Kafka Infrastructure

本目录负责 Gateway 的 Kafka 热群通知聚合、realtime delivery authority fence、direct-conversation read receipt、contact deletion、session kick、群事件、direct-message 和 group-message handler。Gateway Kafka handler 实现已完成服务归属迁移；新增基础设施应优先归属本目录，并通过 contract test 验证回滚语义。
