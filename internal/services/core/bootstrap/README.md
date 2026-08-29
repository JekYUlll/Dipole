# Core Bootstrap

本目录是 Core 服务的启动装配边界，并显式区分独立 Core 与 embedded 兼容模式。独立 Core 的 runtime、消息 sender 和 RPC adapter 已在本目录组合；底层平台设施通过 `internal/platform` 复用，Core Kafka projection 与 AI assistant seed 暂通过兼容 bootstrap 的显式 facade 复用。

embedded 模式保留为本地开发和发布回滚路径；独立 Core 入口必须使用 `InitializeService`。
