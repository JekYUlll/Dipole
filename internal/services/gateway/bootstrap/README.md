# Gateway Bootstrap

本目录是 Gateway 服务的启动装配边界。runtime 已迁入本目录，直接组合 Gateway HTTP/WS、Redis Presence/限流、Kafka、实时投递 authority 和平台 readiness；共享 Internal RPC、TLS 与 Kafka handler 暂通过兼容入口接入。

Gateway 入口必须通过本目录初始化；旧共享 runtime 路径已移除。兼容入口保留为后续拆分 RPC、TLS 和 Kafka handler 的回滚边界。
