# Search Indexer Bootstrap

本目录是 Search Indexer 服务的启动装配边界。runtime 已迁入本目录，直接组合 Search Indexer 自有 Kafka projector 与共享 Kafka、Elasticsearch、metrics、readiness 平台能力。

Search Indexer 入口必须通过本目录初始化；旧共享 bootstrap 路径已移除，服务制品和 Kafka/Elasticsearch 回滚开关保持不变。
