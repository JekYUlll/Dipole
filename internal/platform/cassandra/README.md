# Cassandra 存储适配器

该目录提供跨服务复用的 Cassandra 连接、Timeline 存储和 Sync hydration 适配器。它只负责存储访问与数据映射，Message、Sync 及维护工具的业务编排仍由各自服务或 Bootstrap 负责。

迁移规则：

- 通过接口向服务暴露 Timeline 查询、写入和 hydration 能力。
- 不在此目录放置服务业务策略或投影编排。
- 新的 Cassandra 数据模型应先明确数据所有权，再决定进入服务 infrastructure 或继续作为平台适配器。
