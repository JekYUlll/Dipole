# Agent Capability RPC v1

该内部 RPC 按方法限制受认证服务身份。`dipole-agent` 负责 Run、只读 Capability、Workflow projection、Approval 与 Artifact 创建；`dipole-gateway` 负责用户 Task 控制、repair 审计和 Artifact 读取。业务 principal、permission 与 resource scope 由服务端根据 `task_id` 或 Gateway 认证上下文解析，模型请求无法覆盖。

Artifact v1 只接受当前 `dipole-agent/shadow` Run 创建，正文上限 1 MiB并保存到内容寻址 MinIO；Gateway 读取时由 Core 校验 Task principal 并复核正文大小与 SHA-256。RPC 没有 Artifact update/delete 或公开对象 URL。

`ResolveMcpContext` 仅允许受认证 `dipole-agent` 调用，transport context 不接受 principal。请求携带 Gateway 已认证 principal 与精确 Task/Run；Core 使用持久 Invocation resolver 校验 owner、运行中的 Run、固定 Definition、grant 和 resource scope。principal 不匹配统一返回 NotFound，响应只包含构造 ExecutionContext 所需的服务端权威字段。

MCP ToolCall 审计的读调用不携带 Approval 或 action reference。未来写调用必须在 Begin 绑定已消费 `approval_id`，成功 Finish 只提交 `message` 资源 UUID 与 Message Command kind/id；Core 会按 Task/Run、参数摘要、Approval 和 sender-scoped Message receipt 重新验证。RPC 字段可用不代表生产写 Tool 已开放，当前 MCP Server 仍只投影 read Capability。

`ExecuteMcpMessageCommand` 只接受受认证 Runtime 对 running、已审批 Tool Invocation 的引用。Command ID、Agent/principal 和 direct conversation 均由 Core 派生；Core 重算 canonical Tool 参数摘要，执行 Message Command 后返回可复算的 action reference。该方法不能脱离 Tool begin/finish 生命周期直接发送消息。

`ResolveApprovalGrant` 只接受认证 `dipole-agent`，Core 固定按 active Run 查询唯一 approved、未消费、未撤销、未过期的 exact binding。响应只含 Approval ID、Scope、scope/arguments/nonce SHA-256 和过期时间，不返回消息正文、审批说明或凭据。查询本身不消费审批，调用方随后仍需通过 `ConsumeApproval` 的原子条件更新。
