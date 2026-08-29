# Gateway Kafka Infrastructure

本目录负责 Gateway 的 Kafka 热群通知聚合、realtime delivery authority fence、direct-conversation read receipt、contact deletion、session kick 和群事件 handler。其余消息 delivery handler 仍处于渐进迁移阶段，暂由 embedded 兼容装配复用；新增 Gateway Kafka 基础设施应优先归属本目录，并通过 contract test 验证回滚语义。
