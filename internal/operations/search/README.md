# Search Operations

本目录提供 Search 运维操作的装配实现，包括索引回填、归档、对账、Alias 切换和
Outbox 清理。它只由 `cmd/tools/search-*` 调用，不参与 Search Service 或 Search
Indexer 的长期运行时启动。

服务运行时实现位于 `internal/services/search/`，Elasticsearch 平台适配器位于
`internal/platform/elasticsearch/`。操作发生变化时，需要同步更新对应的契约、门禁和
`CHANGELOG.md`。
