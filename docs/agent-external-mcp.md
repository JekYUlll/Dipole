# Agent 外部 MCP 连接边界

本文记录 Agent G4 外部 MCP Client 的配置、凭据和网络边界。当前仅交付默认关闭的 Profile 契约与 Transport 抽象，生产 Runtime 不会创建外部连接。

## 信任边界

```text
Agent ExecutionContext (tenant)
  -> ExternalMcpTransportRegistry (exact profile + tenant owner)
  -> Reloading Credential Catalog (exact version + active window + revocation)
  -> ExternalMcpTransportFactory (future production provider)
  -> Secret Provider (opaque ref + version)
  -> MCP AuthProvider (fresh bytes per request, bounded and redacted)
  -> DNS public-address validation on every connection
  -> TLS ServerName / CA validation
  -> AllowlistedMcpToolClient (Server identity + Tool + egress policy)
```

Profile 遵循 `contracts/agent-external-mcp/v1/profile.schema.json`，可以保存版本化 `credential.ref` 与 `ca_bundle_ref`。Token、密码、私钥、CA 正文和 OAuth refresh token 不得进入 Profile、Agent Definition、Temporal Workflow、Context、Tool 参数、审计或日志。

Credential Catalog 遵循 `contracts/agent-external-mcp/v1/credential-catalog.schema.json`，只保存 tenant、credential ref/version、生命周期和 opaque `provider_secret_ref`。Catalog loader 每次解析都重新读取完整 manifest，不缓存上一次成功或失败结果；跨租户、错版本、未生效、过期和 revoked binding 在 Factory 调用前拒绝。

## Profile 约束

- `tenant_id`、`profile_id` 与 `server_id` 使用有界稳定标识；Registry 按 profile 精确定位后复核执行 tenant。
- `endpoint` 只接受 HTTPS，禁止 username/password、query、fragment、IP 字面量、localhost、单标签及 `.local`/`.internal` 域名。
- endpoint hostname、effective port 与 TLS ServerName 必须精确命中 allowlist，禁止 wildcard 和大小写歧义。
- `dns_resolution=public_only` 要求未来 Factory 在每次连接时校验全部解析地址，拒绝 loopback、private、link-local、multicast 和其他非公网地址，重定向后重新执行同等校验。
- `allowed_tools` 继续由 MCP 握手 identity、discovery allowlist 和逐 Tool egress policy 复核，Profile 自身不会绕过现有 Client 门禁。

## 当前开关

Compose 固定：

```text
DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=false
```

关闭时忽略残留 Profile 文本，不解析凭据引用，也不创建连接。当前若显式开启，Runtime 在启动阶段返回错误，因为 credential-aware、public-DNS-only Transport Factory 尚未配置。这一行为用于避免配置人员把契约 foundation 误认为已可安全接入生产 Server。

## 轮换与吊销

Catalog 以 `(tenant_id, credential_ref, version)` 作为唯一 binding。轮换顺序为：Secret Provider 创建新 secret version，Catalog 发布新的 active binding，Profile 切换到新 version，验证新建连，最后把旧 binding 标记 revoked。每次建连重新解析 Catalog，因此后续旧版本调用立即被阻断；已经建立的连接仍需未来 Factory 支持 lease expiry 和主动关闭。

Catalog 提供受约束 file source，但尚未装配到 Runtime 启动链。路径必须为规范绝对路径，父目录不得经过 symlink，且由 root 或 Runtime UID 拥有、group/other 不可写；目标文件需要相同 owner/mode 边界，并且是 single-link regular file。默认上限 256 KiB，可在 32 B 至 1 MiB 间收紧。每次 resolve 都重新 `O_NOFOLLOW` 打开并有界读取，因此同目录原子 rename 后立即生效，读取或解析失败时不会回退旧内容。

默认 Kubernetes ConfigMap/Secret projected volume 依赖 symlink，会被该 source 拒绝。可以使用输出 regular file 的 CSI provider，或由受信 init/sidecar 把 lifecycle metadata 写入私有 tmpfs，并以同目录原子 rename 更新。Catalog 只含 opaque reference，仍需要部署层完整性、rollback revision 和可用性告警；不要降低 symlink、owner 或 mode 校验来适配挂载。

## Auth Provider 边界

`createExternalMcpAuthProvider` 适配官方 MCP `AuthProvider`，每次 `token()` 都按 Catalog 已授权的 exact binding 调用注入式 Secret Provider。Provider 返回独占、可写的 fresh `Uint8Array`；adapter 使用私有 AbortSignal 和默认 2 秒 deadline，严格限制大小、UTF-8 与 Bearer token 字符，随后覆盖源 buffer。Provider 原始错误、tenant、ref 和 secret 均不进入外部异常。

Adapter 没有 `onUnauthorized`，401 不会触发未经治理的自动刷新；轮换继续由 Catalog 与 Provider 控制。Adapter 也不缓存 token。MCP SDK 需要把 token 转换成 JavaScript string 并构造 Header，这些副本由 GC 管理，无法提供强零化保证；生产凭据必须短期、最小权限，并支持 Server 端快速吊销。

当前没有 Vault/KMS/Secret Manager backend，也没有把 adapter 装配进 Transport Factory。测试 Provider 只能证明读取、timeout、redaction、validation 和 buffer wipe 语义，不能作为生产秘密管理能力。

## Network Guard 边界

`createExternalMcpNetworkGuardedFetch` 可注入官方 SDK `StreamableHTTPClientTransport` 的 custom fetch。它在每个请求上重新校验 HTTPS、Host、Port、TLS ServerName 和无 query/credential 边界，再通过受信 Resolver 获取 1 至 32 个地址；全部答案都必须是格式、address family 和公网范围一致的唯一地址，混合私网答案或后续 rebinding 会在发起连接前被拒绝。

守卫把完整批准地址集合、TLS ServerName 和 opaque CA ref 交给 `ExternalMcpNetworkDispatcher`。Dispatcher 必须直接连接集合中的一个地址并返回 socket 实际 peer，守卫会再次核对；普通 hostname fetch 无法满足该接口。请求固定使用 `redirect=manual`，任何 `3xx`、`response.redirected` 或响应 URL 变化都会被拒绝并释放 body。

当前仓库没有真实 DNS Resolver、TLS pinned Dispatcher 或 CA Secret backend。该模块只固定可测试的 SSRF/DNS rebinding 边界，生产开关仍然 fail closed。后续实现需要把 SDK request timeout/AbortSignal 传播到 DNS、socket connect、TLS handshake 和 response body，并通过真实双栈 DNS、证书不匹配、连接 peer 偏移及超时故障演练。

## Result 信任边界

外部 MCP Server 返回的文本、结构化内容、Resource 和链接全部属于不可信数据。`externalMcpResultToContextFragment` 只接受成功、可 JSON 序列化且有界的 `CallToolResult`，并固定生成 `section=evidence`、`trust=untrusted` 的可选 fragment；provenance 同时绑定 Profile、Server、Tool 和 Invocation，原始对象在入口处转成不可变 JSON 快照。

预算不足时使用的 compact 内容只保留 Server、Tool、content type 与 structured-content 存在性，不摘要、不复述外部正文，避免未经审计的摘要把指令性文本隐式升级。完整结果进入模型时仍可能触发 Prompt Injection，`trust=untrusted` 需要与 system policy、Capability allowlist、Approval、trajectory Eval 和输出 lineage 一起使用。

当前外部 Client 尚未进入生产执行链。后续调用代码不得把原始 `CallToolResult` 直接拼接到 system/trusted prompt；结果转为 Artifact 或 Memory 时也必须保留 Invocation provenance 与 untrusted 来源，模型改写不能自动提升信任等级。

## Write Approval 边界

Core `ResolveApprovalGrant` 与 `ConsumeApproval` RPC 只接受认证的 `dipole-agent` 和 active Run。Resolve 使用 Task、Capability、Core 计算的 Resource Scope SHA-256 与 canonical Arguments SHA-256 查询最多两条 approved、未消费、未撤销且未过期记录，只在唯一 exact binding 时返回；查询不改变状态。Consume 随后通过 MySQL/sqlc 原子条件更新完成一次性消费，重放、吊销、过期和任一字段漂移都无法成功。

Approval 创建时保存 `nonce_sha256`，Temporal durable binding 也只携带该摘要。它用于区分重复 Approval 并绑定原子 claim，不承担可恢复 Secret 的职责。Runtime 不再要求原始 nonce；认证 grant RPC 返回持久摘要，TS 仍会复核格式并在消费时原样提交。敏感凭据继续使用独立 Secret Provider 边界，不能借用 Approval nonce 字段。

`McpWriteApprovalGate` 持有 Capability Registry，先执行 schema parse、Policy authorize 与 Resource resolve，再从受信 grant resolver 读取当前 approval binding。scope hash 使用与 Go 相同的 `dipole.agent.scope.v1`，参数使用递归排序键的 canonical JSON。grant 精确匹配并且 Core 原子消费成功后，gate 返回只含 Approval、Capability、标准化输入和既有 operation 的授权句柄；兼容 `execute` 路径随后调用 operation。resolver、consume 或 binding 失败均不会触达副作用。

消费发生在 operation 前，因此语义为安全优先的 at-most-once。operation 失败后审批保持 consumed，重试需要新审批。Message Command 已使用稳定 `client_message_id` 并提供认证 sender 范围内的 `ABSENT|COMMITTED` receipt；不确定发送会在独立 2 秒窗口查询并核对完整消息绑定。

migration v31 把已消费 Approval 绑定到 Tool Invocation Begin，并在成功终态保存有界 `message` action reference。Core 先确认 Run 属于当前 Task、`dipole-agent/active` 且仍在运行，再从持久 Invocation 获取 Agent/principal，按 Command kind/id 计算稳定 `client_message_id`，回查 sender-scoped receipt，并核对 Message UUID、sender、target、direct conversation 与 message type。审计表只保存 Approval、Command 和 Message 标识及摘要，不保存消息正文；读取 Tool 和失败终态不能携带 action reference。当前 `createDipoleMcpServer` 继续硬性拒绝 write/destructive descriptor，MCP context 仍为 shadow，生产 write Tool 和 active authority 没有启用。

`ExecuteMcpMessageCommand` 也只允许认证 `dipole-agent` 调用，且请求必须引用上述 running Tool Invocation。Runtime 不提供 Command ID；Core 使用 `invocation_id + command_kind` 派生稳定 ID，并从权威 Invocation 派生 sender/target。审批参数固定为排序 canonical JSON `{"content":...,"conversationId":...}`，Core 会重算 SHA-256 并与 ToolCall/Approval 摘要比较。返回只含 Message action reference 与 `client_message_id`，TS 还会按 Command v1 公式复算后再接受。该 RPC 没有绕过 Approval/Tool audit 的裸发送路径。

`McpMessageWriteProjection` 将上述边界组合成默认关闭的第一方写路径。Server 仅在 ExecutionContext 为 active、Capability 明确要求审批并注入 write executor 时接受写投影；projection 在消费审批前限制目标为当前 principal 与 Agent 的 direct conversation，随后把 Tool runner 生成的 Invocation ID 传入 Command RPC，并将返回引用绑定到成功终态。Core grant resolver 与 TS adapter 已具备。migration v32 的 Runtime promotion grant 绑定 tenant、candidate、pinned Definition、promotion v2 evidence 与 Eval Suite，并要求不同 grantor/reviewer；active Run 持久 candidate，admission 和每次 MCP context resolve 都重查有效期与撤销状态。migration v33 进一步提供只允许认证 Gateway 调用的提案、复核、查询和撤销控制面；提案绑定不可变 Artifact provenance，复核与 Grant 签发、撤销与追加审计分别在单个事务完成，Runtime 数据面无法调用。active context 现仅从 pinned Definition 的 `message.write` 与 conversation/write scope 投影显式 allowlist 中的 `message.system.send`；Core 与 TS 同时拒绝 shadow、未知和重复投影，Registry 新增 Tool 不会自动扩权。生产未注入 authorizer，`index.ts` 仍只注册 `conversation.list` 且没有 write executor。上线前还需 operator Grant 安全配置、UI 风险摘要，以及 RPC deadline/cancellation 发生在服务端提交之后的 receipt 与 action-lineage 收敛演练。

## Durable Elicitation 边界

`McpDurableElicitationAdapter` 将 MCP `elicitation/create` 的受限 form mode 转为现有 Temporal `wait_input` directive。它只支持 text、无标题值映射的 select/multiselect 和 boolean，最多 16 个字段、32 个选项及 16 KiB 请求；URL mode、number/integer、default、format、description、自由扩展和密码/Token 等敏感字段全部拒绝。外部 message、label 和选项保持 `trust=untrusted`，UI 后续必须明确显示来源 Server/Tool。

checkpoint 使用 SHA-256 绑定 host-owned Request ID、Server、Tool、Invocation、deadline、完整 Form 和信任级别。返回 MCP `accept` 前必须收到同一 Request 的有效 durable input resume，并再次执行 Form response 校验；`decline/cancel` 同样要求精确 Request，过期或 checkpoint 漂移 fail closed。

当前 adapter 是协议纯函数边界。`AllowlistedMcpToolClient` 没有声明 Elicitation capability，也没有 request handler；Temporal Activity 不会阻塞等待用户。后续接线需要把 MCP input-required continuation 保存为 checkpoint，在 Workflow 恢复后的新 Activity 中继续协议交互，并定义 Server 不支持恢复、连接丢失和用户取消时的稳定结果。

## 后续实现门槛

生产 Factory 至少需要：每租户 provider owner 授权、加密 Secret Provider、版本精确读取、lease/zeroization、DNS 全地址和重定向检查、TLS chain/ServerName 校验、有界连接超时、低敏审计及故障演练。Secret 只在 Factory 内短暂使用，接口只向 Runtime 返回已建立的 MCP Transport。

完成上述门槛后，先在独立 Shadow tenant 接入一个只读 Server，验证 Server identity、Tool allowlist、取消/超时、Prompt Injection provenance 和凭据轮换，再评估按租户灰度。回滚始终先关闭外部 MCP 开关并等待在途 Tool 调用收敛。
