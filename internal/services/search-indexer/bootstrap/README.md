# Search Indexer Bootstrap

本目录是 Search Indexer 服务的启动装配边界。当前 facade 仍调用 `internal/bootstrap` 中的兼容实现，待共享 Kafka、Elasticsearch、metrics 和 readiness 设施完成拆分后，再迁移底层运行时代码。

Search Indexer 入口必须通过本目录初始化；旧 bootstrap 实现暂时保留，作为发布回滚路径。
