# 存储平台适配器

`internal/platform/storage/` 收纳跨服务复用的存储适配器与迁移装饰器：

- 根包：对象存储和 Artifact/Search Archive 适配器。
- `routing/`：MySQL 与 Cassandra Timeline 的灰度读取、验证和回退。
- `shadow/`：消息页与 Sync hydration 的异步对照、指标和 fallback。

这些包只包装 application port，不改变 Message、Sync 的业务协议。关闭对应配置或移除装饰器即可回到 MySQL 主路径。
