# 兼容层

`internal/compat/` 收纳为渐进式迁移保留的旧包路径适配器。兼容层只负责仍需兼容的 domain-event decoder，不新增业务实现；新代码应直接依赖对应的 `internal/services/<service>/` 包。

当前目录：

- `service/`：保留历史包路径下的 domain-event decoder 测试与兼容辅助；Core、Message、Sync 及各领域 service contract 已在调用者迁移完成后移除。

兼容层的删除或缩减必须先完成调用方迁移，并保留可验证的回滚路径。
