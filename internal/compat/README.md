# 兼容层

`internal/compat/` 收纳迁移期回归测试和历史包路径验证辅助。当前目录不承载生产实现；新代码应直接依赖对应的 `internal/services/<service>/` 包。

当前目录不再保留 service 子目录。跨版本 domain-event decoder 回归测试已归档到
`internal/platform/events/contract/`，由平台事件契约包统一维护。

兼容层的删除或缩减必须先完成调用方迁移，并保留可验证的回滚路径。
