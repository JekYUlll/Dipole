# Core Bootstrap

本目录是 Core 服务的启动装配边界，并显式区分独立 Core 与 embedded 兼容模式。当前 facade 仍调用 `internal/bootstrap` 中的兼容实现，待共享 RPC、Kafka、metrics、storage 和 readiness 设施完成拆分后，再迁移底层运行时代码。

embedded 模式保留为本地开发和发布回滚路径；独立 Core 入口必须使用 `InitializeService`。
