# Search Bootstrap

本目录是 Search 服务的启动装配边界。当前 facade 仍调用 `internal/bootstrap` 中的兼容实现，方便在不改变运行时语义的情况下逐步迁移共享 gRPC、metrics 和 readiness 基础设施。

迁移完成前，Search 入口只能通过本目录初始化运行时；新的 Search 业务装配不得重新写入 `internal/bootstrap`。
