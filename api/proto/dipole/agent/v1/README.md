# Agent Capability RPC v1

该内部 RPC 只接受受认证的 `dipole-agent` 服务身份。业务 principal、permission 与 resource scope 由服务端根据 `task_id` 读取固定 Agent Task/Definition，客户端请求无法覆盖。
