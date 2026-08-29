# 兼容层

`internal/compat/` 收纳迁移期回归测试和历史包路径验证辅助。当前目录不承载生产实现；新代码应直接依赖对应的 `internal/services/<service>/` 包。

当前目录：

- `service/`：保留跨版本 domain-event decoder 的回归测试和 Go 包声明；Core、Message、Sync 及各领域 service contract 已在调用者迁移完成后移除。

兼容层的删除或缩减必须先完成调用方迁移，并保留可验证的回滚路径。
