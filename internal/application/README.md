# Application Contracts

`internal/application/` 保存跨服务复用的 Go application port、版本化请求/响应类型和协议校验。该目录不拥有具体服务的数据库、业务编排或一次性运维实现。

服务实现进入 `internal/services/<service>/application`、`domain` 或 `infrastructure`；平台能力进入 `internal/platform`。如需跨语言复用，优先同步 `api/proto/` 或 `contracts/`，再由本包提供 Go adapter。

架构测试会阻止本包依赖服务实现、旧数据层和运维目录，避免共享契约层反向形成新的业务共享区。
