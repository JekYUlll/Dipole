# Elasticsearch 存储适配器

该目录提供 Search 与 Search Indexer 共用的 Elasticsearch 连接、版本化 Alias、strict mapping、mutation apply 和查询适配器。Search/Indexer 的权限范围、事件消费和运行时编排仍由各自服务负责。

约束：

- `schema/` 保存版本化索引契约，变更必须配套 contract test。
- 通过 read/write Alias 和 external revision 支持重建、切换与回滚。
- Elasticsearch 只保存可重建的搜索投影，不承担消息事实存储或成员权限事实。
