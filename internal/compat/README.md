# 兼容层

`internal/compat/` 收纳为渐进式迁移保留的旧包路径适配器。兼容层只负责类型别名、错误别名和构造转发，不新增业务实现；新代码应直接依赖对应的 `internal/services/<service>/` 包。

当前目录：

- `service/`：旧 `internal/service` 包的 Core 和 Message 兼容入口；Sync、Core Auth Token、Core Admin、Core Session 与 Core User 兼容入口已在调用者迁移完成后移除。

兼容层的删除或缩减必须先完成调用方迁移，并保留可验证的回滚路径。
