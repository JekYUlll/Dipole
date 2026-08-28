# Agent 外部 MCP 连接边界

本文记录 Agent G4 外部 MCP Client 的配置、凭据和网络边界。当前已交付默认关闭的 Profile、Credential Catalog、Transport Factory 与可注入网络策略，生产 Runtime 仍不会创建外部连接。

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

关闭时忽略残留 Profile 文本，不解析凭据引用，也不创建连接。当前若显式开启，Runtime 仍在启动阶段返回错误，因为生产 Secret Provider、DNS Resolver 与 TLS pinned Dispatcher 尚未配置。这一行为用于避免配置人员把策略完整的 Factory 误认为已具备生产 I/O backend。

## 轮换与吊销

Catalog 以 `(tenant_id, credential_ref, version)` 作为唯一 binding。轮换顺序为：Secret Provider 创建新 secret version，Catalog 发布新的 active binding，Profile 切换到新 version，验证新建连，最后把旧 binding 标记 revoked。每次建连重新解析 Catalog，因此后续旧版本调用立即被阻断；已经建立的连接仍需未来 Factory 支持 lease expiry 和主动关闭。

Catalog 提供受约束 file source，但尚未装配到 Runtime 启动链。路径必须为规范绝对路径，父目录不得经过 symlink，且由 root 或 Runtime UID 拥有、group/other 不可写；目标文件需要相同 owner/mode 边界，并且是 single-link regular file。默认上限 256 KiB，可在 32 B 至 1 MiB 间收紧。每次 resolve 都重新 `O_NOFOLLOW` 打开并有界读取，因此同目录原子 rename 后立即生效，读取或解析失败时不会回退旧内容。

默认 Kubernetes ConfigMap/Secret projected volume 依赖 symlink，会被该 source 拒绝。可以使用输出 regular file 的 CSI provider，或由受信 init/sidecar 把 lifecycle metadata 写入私有 tmpfs，并以同目录原子 rename 更新。Catalog 只含 opaque reference，仍需要部署层完整性、rollback revision 和可用性告警；不要降低 symlink、owner 或 mode 校验来适配挂载。

## Auth Provider 边界

`createExternalMcpAuthProvider` 适配官方 MCP `AuthProvider`，每次 `token()` 都按 Catalog 已授权的 exact binding 调用注入式 Secret Provider。Provider 返回独占、可写的 fresh `Uint8Array`；adapter 使用私有 AbortSignal 和默认 2 秒 deadline，严格限制大小、UTF-8 与 Bearer token 字符，随后覆盖源 buffer。Provider 原始错误、tenant、ref 和 secret 均不进入外部异常。

Adapter 没有 `onUnauthorized`，401 不会触发未经治理的自动刷新；轮换继续由 Catalog 与 Provider 控制。Adapter 也不缓存 token。MCP SDK 需要把 token 转换成 JavaScript string 并构造 Header，这些副本由 GC 管理，无法提供强零化保证；生产凭据必须短期、最小权限，并支持 Server 端快速吊销。

`createExternalMcpStreamableHttpTransportFactory` 已把 adapter 装配进官方 Streamable HTTP Transport，并且每次连接创建新的 AuthProvider。当前没有 Vault/KMS/Secret Manager backend；测试 Provider 只能证明读取、timeout、redaction、validation 和 buffer wipe 语义，不能作为生产秘密管理能力。

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

`McpMessageWriteProjection` 将上述边界组合成默认关闭的第一方写路径。Server 仅在 ExecutionContext 为 active、Capability 明确要求审批并注入 write executor 时接受写投影；projection 在消费审批前限制目标为当前 principal 与 Agent 的 direct conversation，随后把 Tool runner 生成的 Invocation ID 传入 Command RPC，并将返回引用绑定到成功终态。Core grant resolver 与 TS adapter 已具备。migration v32 的 Runtime promotion grant 绑定 tenant、candidate、pinned Definition、promotion v2 evidence 与 Eval Suite，并要求不同 grantor/reviewer；active Run 持久 candidate，admission 和每次 MCP context resolve 都重查有效期与撤销状态。migration v33 进一步提供只允许认证 Gateway 调用的提案、复核、查询和撤销控制面；提案绑定不可变 Artifact provenance，复核与 Grant 签发、撤销与追加审计分别在单个事务完成，Runtime 数据面无法调用。真实 v2 证据通过独立 `promotion:publish` CLI 写入 completed Shadow Run 绑定的 content-addressed Artifact；Core 交叉校验 candidate、pinned Definition、Suite hash、eligible 决策和 metadata，CLI 只返回低敏收据且不触发提案。Gateway-only review RPC 先复用 operator/tenant/Proposal 授权，再从专用 Artifact 存储读取并复算 exact content hash；普通 Task-principal 下载授权不扩展，公共 HTTP 继续关闭。active context 现仅从 pinned Definition 的 `message.write` 与 conversation/write scope 投影显式 allowlist 中的 `message.system.send`；Core 与 TS 同时拒绝 shadow、未知和重复投影，Registry 新增 Tool 不会自动扩权。生产未注入 authorizer，`index.ts` 仍只注册 `conversation.list` 且没有 write executor。上线前还需 operator Grant 安全配置、UI 风险摘要，以及 RPC deadline/cancellation 发生在服务端提交之后的 receipt 与 action-lineage 收敛演练。

## Durable Elicitation 边界

`McpDurableElicitationAdapter` 将 MCP `elicitation/create` 的受限 form mode 转为现有 Temporal `wait_input` directive。它只支持 text、无标题值映射的 select/multiselect 和 boolean，最多 16 个字段、32 个选项及 16 KiB 请求；URL mode、number/integer、default、format、description、自由扩展和密码/Token 等敏感字段全部拒绝。外部 message、label 和选项保持 `trust=untrusted`，UI 后续必须明确显示来源 Server/Tool。

checkpoint 使用 SHA-256 绑定 host-owned Request ID、Server、Tool、Invocation、deadline、完整 Form 和信任级别。返回 MCP `accept` 前必须收到同一 Request 的有效 durable input resume，并再次执行 Form response 校验；`decline/cancel` 同样要求精确 Request，过期或 checkpoint 漂移 fail closed。

当前 adapter 与单轮 MRTR continuation 已进入默认关闭的 Activity-safe runner：首次调用可返回 `wait_input` checkpoint，恢复后使用新 Client/Transport 精确回传原参数、用户输入和 opaque request state。生产 Worker 尚未调度这类权威命令；多轮、URL mode、敏感输入和 Server 不支持恢复时的产品策略继续关闭。

## Durable Round Receipt 边界

migration v36 为每个外部 Tool Invocation 保存最多两个 round。Round ID 由 Invocation、轮次和 canonical 请求 SHA-256 确定，表同时绑定 Task、Run、请求摘要和随机 owner token 摘要；`INSERT IGNORE` 只允许首次调用原子取得 `executing`，没有 lease、超时回收或 owner 接管路径。

Activity 取得 `claimed` 后才建立全新 Client/Transport。远端返回结果后，Runtime 先把最多 128 KiB 的 canonical JSON、摘要和字节数写成 `completed`，随后才向 Temporal 返回；Activity completion 丢失时，新尝试读取 `replay_completed` 并跳过网络。已知失败以稳定错误码重放。已有 `executing` 一律返回 `ambiguous`，即使新尝试持有相同参数也不会自动重发。传输异常保存为 `remote_outcome_unknown`，将不确定调用收敛为 at-most-once 失败。

该收据仍无法证明远端已执行、但响应尚未到达 Runtime 或本地终态尚未提交的极小窗口。未来只有 Profile 显式声明且验证了服务端幂等键或查询收据协议时，才能对这类调用增加恢复策略。当前 Worker 与外部网络开关保持关闭，禁止通过缩短 lease 或手工修改 `executing` 记录来重试。

Worker command dispatcher 只接受 Task、Run 和 Invocation ID。它通过认证 Core RPC 重新取得持久 Profile、Server、Tool、Capability、canonical 参数摘要和 Invocation 开始时间；稳定 input request ID 与绝对截止时间均由这些权威字段派生。`wait_input` 外层 checkpoint 绑定完整命令摘要和 Activity checkpoint，进程替换后先重新解析并比较，任何参数或 authority 漂移都会在建连前拒绝。连接 Session Factory 只得到 tenant/profile/server/tool 四字段，不接收 Task/Run/Invocation、参数或 principal。

`createMcpWorkerRuntime` 将上述 dispatcher 与 Core round receipt、外部 Transport Registry、allowlisted modern Client 和 Activity continuation 组合为专用可注入单元。取消信号会在 Core resolve 前及 resolve 后再次检查，避免尚未发网的取消认领 `executing` receipt；本地 completed receipt 在替换 Runtime 后直接重放，`ambiguous` 在创建 Client/Transport 前终止。

Invocation begin/finish 现支持精确重放。Begin 的稳定 ID 已存在时，Core 重新授权 Task/Run/Capability/Approval，并逐项比较 tenant、principal、Agent、Tool、Profile/Server、canonical 参数、request/trace；只有完全一致才返回原 running/completed/failed 记录。Finish 对 terminal 记录逐项比较结果摘要、字节数、延迟、错误码与 action reference，精确重放不再次更新数据库。

`ResolveMcpToolCommand` 同时返回 Invocation 状态。对于 completed/failed Invocation，Round Service 在任何 claim 写入前读取确定性 Round ID 对应的既有 receipt；completed/failed 结果可重放，executing 返回 ambiguous，缺失或绑定漂移直接拒绝。terminal Invocation 永远不能创建新 round，因此 Activity completion 丢失不会转化为第二次远端调用。

该组合器当前没有进入 `index.ts` 或 Temporal Worker Activity mode。现有系统也没有“查找下一条外部 Invocation”的轮询入口；后续应由受信 Agent Step 在同一持久 Run 内先创建 exact Invocation，再把三 ID 交给组合器。禁止从 Task goal、模型输出、Kafka payload 或客户端参数直接选择 Profile/Server/Tool，也不为此增加重复命令权威的 dispatch 表。

`TrustedMcpInvocationProducer` 提供上述受信创建边界。运行时输入是 strict `{workflowStep, ordinal, capabilityId, arguments, approvalId?}`；ExecutionContext 提供身份与 Task/Run，`ExternalMcpCapabilityRouteRegistry` 按 Capability 固定 Profile、Server、Tool、输入 schema、Resource resolver 和 egress policy。模型无法提交或覆盖 authority 字段。

Invocation ID 由 tenant/principal/Agent/Task/Run/Workflow step/ordinal 的 canonical v1 绑定计算为 SHA-256。参数、Profile、Server、Tool、Approval 和 trace 不参与 ID 分叉；这些字段发生漂移时，相同 ID 会进入 Core exact begin 比较并 fail closed。这样 Activity retry 保持同一命令意图，显式增加 ordinal 才代表同一步内新的 Tool 调用。producer 可接收 Core 返回的 running/completed/failed 状态，为后续 receipt-only 恢复保留依据。

`FinishMcpToolInvocationFromRound` 提供专用 server-owned terminal API。Runtime 只提交 Task、Run、Invocation 和 Round ID；Core 重新加载两份持久记录，要求 exact binding、规范 terminal receipt 和已知 read-risk Capability，拒绝 `executing`、`input_required` 中间结果、write Capability 与任何漂移。首次完成由 Core 从 Invocation 开始时间和 Round 结果派生 latency、结果字节数、摘要或错误码，再调用既有审计 Finish；重试读取并核对已存 terminal Invocation，因此不会因重新计算 latency 产生冲突。默认关闭的 `createMcpTerminalWorkerRuntime` 在 complete 或稳定 failed Round 后调用该 API，ambiguous 和 waiting_input 不会提前收口。旧 Finish RPC 会先解析持久命令并拒绝任何带 Profile 的外部 Invocation，避免 Runtime 绕过 receipt 形成第二个终态所有者。

`TemporalMcpDispatchActivity` 提供独立、默认关闭的持久编排边界。begin 输入只含 host-owned route binding、Task/Run/principal、业务参数和低敏关联 ID；工厂配置固定 Capability、Workflow step 与 ordinal。`temporalMcpDispatchRouteBinding` 对 route ID/version、Capability、step 与 ordinal 的 canonical manifest 生成 SHA-256，route ID/version 和摘要一起进入 Temporal Activity history。替换 Worker 的任意 manifest 字段不匹配时都会在 Core 访问前拒绝，即使部署时遗漏 version 提升，也不会在 completion-loss retry 中生成第二个 Invocation 意图。

每次 begin、Activity retry 和 durable resume 都重新调用 Core context resolver，精确核对 Task/Run/principal，再以保存的 canonical 参数重放 `TrustedMcpInvocationProducer`。producer 返回同一 Invocation 后，Activity 仅向 terminal Worker 传递 Task、Run 和 Invocation ID。`wait_input` checkpoint 以 SHA-256 绑定完整 route manifest 摘要、Step 坐标、Invocation、参数、关联 ID 和内部 Worker checkpoint；恢复时任何漂移都会在 Worker 调用前失败。未来受信 Workflow 必须从版本化静态 route manifest 写入 binding，模型、goal、Kafka payload 和客户端参数不能提交或覆盖摘要。

terminal Worker 完成后，Activity 将不可信结果连同 Invocation/Round lineage 交给注入式 projector。`ExternalMcpArtifactProjector` 会从 Core 重新解析 completed Tool command，核对 tenant/principal/Agent/Task/Run 与 Profile/Server/Tool/Capability，调用 MCP 标准 schema 并要求 raw/parsed canonical JSON 完全一致，结果上限为 128 KiB。Artifact type 由稳定 Invocation 前缀派生，version 固定为 1，因此同一 Task 的不同调用各自拥有独立幂等键；metadata 只保存 untrusted 标记、命令 lineage、参数摘要和结果摘要，不复制参数或凭据。writer 返回的 Artifact ID、内容摘要、大小与 metadata 会再次复算。

Artifact RPC 已提交后发生取消时，projector 会让当前 Activity 失败；下一次 Activity retry 重新解析命令并精确写入相同内容，现有 content-addressed Store 返回同一收据。Workflow 输出只保存 Invocation ID、Round ID、Artifact ID/version，不保存外部正文。当前普通 Artifact policy 只允许 `dipole-agent` 的 running shadow Run，active MCP 结果会在写入前 fail closed，后续需独立扩展 active Artifact admission 与对应审计。

`createTemporalMcpDispatchRuntime` 将上述边界组装为一个 route-scoped、default-off Runtime。输入只接受 host-owned Route Registry、Core RPC port、Artifact writer、Transport Registry 和有界 timeout/client seam；同一 Registry 既驱动 `TrustedMcpInvocationProducer`，也按当前 Capability 派生 Worker 使用的唯一 Profile/Tool egress policy，因此装配层无法传入第二份漂移策略。Core port 同时承担 Context、Invocation begin/resolve、Round receipt 和 terminal finish，Artifact projector 也通过该 port 重新读取同一命令。

factory 的公开结果只有 `routeBinding` 与 `activities.executeMcpDispatch`。producer、terminal Worker、projector、Profile/Tool policy 和 Transport session 均留在闭包内，调用方不能跳过三 ID handoff 或替换完成权威。组合测试已证明首次成功后 Activity completion 丢失只读取 durable Round 并重放同一 Artifact，`input_required` 使用新 Context 与同一 Invocation 继续第二轮，预取消在 Core/receipt/Transport/Artifact 之前结束。

该 Activity 没有注册到通用 `agentTaskWorkflow`、`index.ts` 或现有 Activity mode。当前启动链也没有外部 Capability route；第一方 Message write 继续使用带 action reference 的现有 Finish 路径，外部 write Capability 尚无通用可验证 action receipt。在真实路由注册、受控调度、active Artifact policy 和生产 I/O 完成前，生产 Worker 与外部网络开关继续关闭。

## 后续实现门槛

Transport Factory 已完成版本精确绑定、每请求 fresh Secret、公共 DNS 全地址、重定向拒绝、peer 复核及 SDK 隐式重试关闭的组合。`NodeExternalMcpDnsResolver` 现提供真实 Node DNS 适配器：每个 fetch 使用独立 Resolver 并行请求 A/AAAA，不保留跨请求缓存；`ENODATA/ENOTFOUND` 只表示当前 family 无记录，任何其他单族错误会使完整解析失败。调用取消会执行 request-local `Resolver.cancel()`，不会影响其他并发请求。Network Guard 随后继续执行公网、family、重复和数量复核。

`NodeExternalMcpPinnedTlsDispatcher` 接收 Network Guard 当前请求批准的完整地址集合，通过 `https.request` 的自定义 lookup 只返回该集合；`agent=false` 阻止连接复用绕过 fresh DNS，Node 直连路径不会采用环境代理。TLS 固定 ServerName、CA、最低 TLS 1.2 和证书链校验，响应建立后还会把 socket `remoteAddress` 与批准集合再次比较；注入式 client 的 peer 证据也由 Dispatcher 外层复核。请求和响应 body 保持流式，100 ms 至 60 秒的 connect timeout 只覆盖 TLS `secureConnect` 前窗口，取消会销毁当前 request。3xx 不在该层跟随，由 Network Guard 统一拒绝。

文件 CA provider 将 opaque ref 映射到规范绝对路径，每次 dispatch 都重新打开并加载，以支持受控原子轮换。父目录必须 canonical 且不可被 group/world 写，文件使用 `O_NOFOLLOW` 并要求 regular、single-link、root/expected-owner、不可被 group/world 写、256 KiB 默认上限；内容只允许 1 至 32 个可解析 PEM certificate。该 provider 适合静态 CA bundle，私钥和 Bearer secret 不得进入此映射。

生产接入仍至少需要：每租户 provider owner 授权、加密 Secret Provider、secret lease/吊销告警、低敏审计及真实公网故障演练。Secret 只在 Factory 内短暂使用，接口只向 Runtime 返回 MCP Transport。当前 DNS Resolver、CA provider 和 pinned Dispatcher 均未注册到 `index.ts` 或 Worker startup，单独存在不会发起查询或建立连接。

完成上述门槛后，先在独立 Shadow tenant 接入一个只读 Server，验证 Server identity、Tool allowlist、取消/超时、Prompt Injection provenance 和凭据轮换，再评估按租户灰度。回滚始终先关闭外部 MCP 开关并等待在途 Tool 调用收敛。
