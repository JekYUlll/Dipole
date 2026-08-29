# Generated API Contracts

`api/gen/go/` 是由 `api/proto/` 生成的 Go RPC 客户端、服务端和消息类型目录。
生成代码不属于某个具体服务的内部 transport；各服务通过版本化 protobuf 契约共享它。

使用 `scripts/proto.sh` 生成，使用 `scripts/check-proto.sh` 检查生成漂移。手工修改生成
文件会在下一次生成时被覆盖。
