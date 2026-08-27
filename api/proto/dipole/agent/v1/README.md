# Agent Capability RPC v1

该内部 RPC 按方法限制受认证服务身份。`dipole-agent` 负责 Run、只读 Capability、Workflow projection、Approval 与 Artifact 创建；`dipole-gateway` 负责用户 Task 控制、repair 审计和 Artifact 读取。业务 principal、permission 与 resource scope 由服务端根据 `task_id` 或 Gateway 认证上下文解析，模型请求无法覆盖。

Artifact v1 只接受当前 `dipole-agent/shadow` Run 创建，正文上限 1 MiB并保存到内容寻址 MinIO；Gateway 读取时由 Core 校验 Task principal 并复核正文大小与 SHA-256。RPC 没有 Artifact update/delete 或公开对象 URL。

`ResolveMcpContext` 仅允许受认证 `dipole-agent` 调用，transport context 不接受 principal。请求携带 Gateway 已认证 principal 与精确 Task/Run；Core 使用持久 Invocation resolver 校验 owner、运行中的 Run、固定 Definition、grant 和 resource scope。principal 不匹配统一返回 NotFound，响应只包含构造 ExecutionContext 所需的服务端权威字段。
