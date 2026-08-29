# Runtime Platform

本目录提供跨服务的运行时基础设施，例如 metrics 生命周期、TLS 文件校验和 Internal RPC transport。它不承载业务编排、服务数据访问或具体服务的 RPC 方法语义。

`internal/platform/rpc` 统一负责 gRPC listener、服务认证、TLS 1.3 mTLS、health check、拨号超时和优雅关闭。服务 bootstrap 只负责注册自己的协议 adapter 与方法权限。

服务 bootstrap 可以依赖本目录；旧 `internal/bootstrap` helper 仅作为兼容出口，便于渐进迁移和回滚。
