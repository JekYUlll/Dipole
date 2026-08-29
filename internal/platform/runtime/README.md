# Runtime Platform

本目录提供跨服务的运行时基础设施，例如 metrics 生命周期。它不承载业务编排、服务数据访问或服务间 RPC 语义。

服务 bootstrap 可以依赖本目录；旧 `internal/bootstrap` helper 仅作为兼容出口，便于渐进迁移和回滚。
