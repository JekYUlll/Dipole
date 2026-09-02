# Agent 外部 MCP 连接边界

本文记录 Agent G4 外部 MCP Client 的配置、凭据和网络边界。当前已交付默认关闭的 Profile、Credential Catalog、Transport Factory、受控 Shadow Worker 装配与可注入网络策略；基础 Compose 不会创建外部连接，独立 Shadow overlay 仍需显式完整输入和共享环境证据。

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

关闭时忽略残留 Profile 和 I/O manifest 配置，不解析凭据引用，也不创建连接。`external_mcp_shadow` 已作为独占 Temporal activity mode 接入 Runtime 入口：只有 external Profile、Temporal、Kafka subscription trigger 和 Capability RPC 全部对齐时才会构造统一 process，任何部分启用或 mode 漂移都会在资源创建前拒绝。基础 Compose 继续固定关闭，避免配置人员把独立构件误认为已具备灰度上线条件。

## 受控 Shadow 部署

`deploy/microservices/agent-external-mcp-shadow.yml` 是唯一的 Compose 启用 overlay。它要求显式提供 Profile JSON、I/O manifest、route manifest、只读 secrets 目录、Kafka broker/独立 consumer group 与 Temporal address/namespace/task queue；渲染时会把 manifests 固定挂载到 `/run/dipole/external-mcp/`，并强制 `runtime_mode=shadow`、`trigger_mode=subscription`、`external_mcp_shadow`、metadata model、Memory/Control/first-party MCP server 关闭。

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-external-mcp-shadow.yml \
  config --quiet
```

`scripts/check-compose.sh` 会验证完整 overlay 的渲染语义，并确认缺少 Profile 时直接拒绝。该检查只证明部署契约，不能替代 Manifest owner/mode、fresh readiness、Core mTLS、Kafka/Temporal、真实公网 DNS/TLS、凭据 owner 或外部 Server 的联合演练。回滚仅需移除 overlay；基础 Compose 会恢复 `foundation` 与 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=false`。

## 轮换与吊销

Catalog 以 `(tenant_id, credential_ref, version)` 作为唯一 binding。轮换顺序为：Secret Provider 创建新 secret version，Catalog 发布新的 active binding，Profile 切换到新 version，验证新建连，关闭旧轮次，最后把旧 binding 标记 revoked。每次建连重新解析 Catalog，因此后续旧版本调用会在 Transport 构造前被阻断。Activity 每个 Tool round 都使用 fresh Client/Transport，并在完成、取消或失败后关闭；Catalog 吊销不具备中断已发出远端请求的 authority，在途窗口由 100 ms 至 60 秒 request timeout、取消传播和下游 Server 端吊销共同约束。

Catalog 提供受约束 file source，并由 `external_mcp_shadow` 的 deployment plan 在启用时装配到 Runtime 启动链。路径必须为规范绝对路径，父目录不得经过 symlink，且由 root 或 Runtime UID 拥有、group/other 不可写；目标文件需要相同 owner/mode 边界，并且是 single-link regular file。默认上限 256 KiB，可在 32 B 至 1 MiB 间收紧。每次 resolve 都重新 `O_NOFOLLOW` 打开并有界读取，因此同目录原子 rename 后立即生效，读取或解析失败时不会回退旧内容。

默认 Kubernetes ConfigMap/Secret projected volume 依赖 symlink，会被该 source 拒绝。可以使用输出 regular file 的 CSI provider，或由受信 init/sidecar 把 lifecycle metadata 写入私有 tmpfs，并以同目录原子 rename 更新。Catalog 只含 opaque reference，仍需要部署层完整性、rollback revision 和可用性告警；不要降低 symlink、owner 或 mode 校验来适配挂载。

`scripts/drill-agent-external-mcp-credential-lifecycle.sh` 提供默认不随单测执行的离线生命周期演练。它在临时 owner-only 目录创建两组独立 AES-256-GCM key/envelope，以同目录原子 rename 发布 Catalog，依次验证 v3 初始连接、v4 轮换连接、旧 v3 吊销拒绝、Runtime 重建后的 v4 连接和最终 v4 吊销拒绝。三次成功 Transport 均显式关闭，两次吊销均要求 Transport builder 调用数不增加。

演练证据写入 gitignored 的 `services/agent-runtime/.artifacts/external-mcp-credential-lifecycle.json`，mode 固定为 `0600`。`contracts/agent-external-mcp/v1/credential-lifecycle-drill-evidence.schema.json` 和 `npm run mcp:credential-drill:check` 共同固定 24 小时有效期、canonical SHA-256、三开三关与 fail-closed 门禁；证据不包含 tenant、Profile、credential/key/secret ref、路径、endpoint 或 Token，并显式声明 `inflight_revocation_authority=false`、`production_authority=false`。该演练使用注入式无网络 Transport，只证明本地生命周期组合，不能替代 provider owner、真实公网 TLS 或下游 Server 端吊销证据。

## Auth Provider 边界

`createExternalMcpAuthProvider` 适配官方 MCP `AuthProvider`，每次 `token()` 都按 Catalog 已授权的 exact binding 调用注入式 Secret Provider。Provider 返回独占、可写的 fresh `Uint8Array`；adapter 使用私有 AbortSignal 和默认 2 秒 deadline，严格限制大小、UTF-8 与 Bearer token 字符，随后覆盖源 buffer。Provider 原始错误、tenant、ref 和 secret 均不进入外部异常。

Adapter 没有 `onUnauthorized`，401 不会触发未经治理的自动刷新；轮换继续由 Catalog 与 Provider 控制。Adapter 也不缓存 token。MCP SDK 需要把 token 转换成 JavaScript string 并构造 Header，这些副本由 GC 管理，无法提供强零化保证；生产凭据必须短期、最小权限，并支持 Server 端快速吊销。

`createExternalMcpStreamableHttpTransportFactory` 已把 adapter 装配进官方 Streamable HTTP Transport，并且每次连接创建新的 AuthProvider。`createEncryptedFileExternalMcpSecretProvider` 提供本地 AES-256-GCM 静态加密实现：配置把 exact `provider_secret_ref` 映射到密文路径和 `key_ref`，再把 key ref 映射到独立 32 字节 key 文件。Provider 配置在构造时复制，未知 ref、重复路径、错 provider 和非规范路径直接拒绝。

envelope 固定为 `DPMCP01 | 12-byte nonce | 1..8192-byte ciphertext | 16-byte tag`。AAD 以 NUL 分隔绑定 schema、tenant、credential ref/version、provider ID、provider secret ref 和 key ref，因此复制密文到另一租户、版本、引用或 key 配置无法解密。每次 `token()` 都重新打开 key/envelope，成功返回独占可写 plaintext；key 总会擦除，失败或取消路径也擦除已产生的 plaintext。Node/OpenSSL 内部副本及最终 JavaScript token string 仍无法提供强零化保证。

key 文件必须是 root/Runtime UID 拥有的 single-link regular file，禁止 group/other 任意权限和执行位；密文文件允许 group/other 只读，但禁止写入和执行。两者父目录都必须 canonical、owner 正确且不可被 group/world 写，并通过 `O_NOFOLLOW` 打开。推荐把 key 放在独立 tmpfs/受控 CSI/KMS 解封目录，密文放在另一只读挂载；把 key 与密文放在同一持久卷只能抵御密文单独泄露，无法抵御完整主机或卷快照泄露。

凭据轮换应创建新的 key ref、secret ref、credential version 和文件，再按 Catalog 流程切换 Profile；确认新连接且旧轮次关闭后 revoke 旧 binding，并在后续配置发布中移除旧映射。原地替换同 key 的密文适合短期 token 更新，key 与密文的双文件原地更新缺少原子性，不用于 key rotation。当前没有 Vault/KMS/Secret Manager adapter、key lease 或在途连接主动吊销；encrypted-file Provider 只会在受控 `external_mcp_shadow` deployment plan 启用后构造。

## Production I/O 组合

`createExternalMcpProductionIoRuntime` 是生产 adapters 的单一 construction authority。enabled 时它依次构造受约束文件 Catalog、encrypted-file Secret Provider、request-local Node DNS Resolver、文件 CA Provider、pinned TLS Dispatcher 和 Streamable HTTP Transport Factory，最终只公开 tenant-bound raw `registry`、local `preflight`、受约束 `shadowConnectivityDrill` 与组合后的 `readinessEvidence`。该 raw Registry 专供 readiness 采集及受控演练；MCP Worker 会在其外层强制构造 fresh-readiness gated Registry。兼容入口 `createExternalMcpProductionIoRegistry` 仍只返回 raw Registry；调用方无法取得裸 Secret Provider、Dispatcher 或 guarded fetch 来绕过 tenant Profile 与 Catalog 生命周期检查。

构造阶段只验证 ID、引用、绝对规范路径、映射唯一性和数值上限，不打开 Catalog/key/envelope/CA 文件，不创建 DNS client，也不建立 socket。`Registry.connect` 才重新读取 Catalog并检查 active/revoked；官方 Transport 随后按请求从 AuthProvider 读取 secret，并在 fetch 时解析 DNS、读取 CA 和建连。disabled 时组合器连残留 I/O 配置属性也不读取，保持 kill switch 的无副作用语义。

`production-io-manifest.schema.json` 定义 credential-free v1 配置：Catalog 路径及上限、单个 encrypted provider 的 ID/key/secret 路径映射、CA ref/path 映射和 TLS connect timeout。Schema 禁止附加字段，Runtime 进一步要求 ref 唯一、secret 引用已声明 key，并让 Catalog、key、secret 与 CA 的全部路径全局唯一。manifest 不得包含 token、password、32 字节 key、envelope 或 CA 正文。

`loadExternalMcpProductionIoManifest` 只在 Profile 开关 enabled 时读取 `DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST`。该路径和 manifest 内所有路径都必须是规范绝对路径；manifest 父目录必须 canonical、owner 正确且不可被 group/world 写，文件必须 owner-only、无执行位、regular/single-link，并通过 `O_NOFOLLOW` 有界读取 UTF-8 JSON。每次调用重新加载，读取或校验失败统一返回低敏错误且不会回退旧快照；disabled 时连残留环境变量 getter 都不触达。

loader 输出的 typed `io/options` 可直接传给 composition，并把同一 expected owner 与各项上限传递给下游文件 adapters。`maximum_secret_bytes` 会同时约束 encrypted Provider、请求期 AuthProvider 和 readiness preflight，避免部署上限与真实请求行为漂移。

`createExternalMcpReadCapabilityDefinitions` 提供代码拥有的外部只读 authority。当前唯一 definition 为 `repository.issue.read`：输入只接受 `owner`、`repo`、`issue_number`，仓库坐标规范化为小写，resource scope 固定为 `repository_issue/{owner}/{repo}#{issue_number}:read`，权限同 Capability ID，风险固定为 read。代码 egress ceiling 为 1 KiB 且只允许上述三个参数；route manifest 只能缩小字段集合或字节上限，无法改写 descriptor、schema 与 resource resolver。

factory 每次返回独立 Registry，并在注册后 seal；descriptor、ceiling 及参数名 snapshot 同时冻结。调用方不能在 deployment load 前后追加 write/destructive 或其他外部 definition。factory 本身没有环境、manifest、RPC、凭据或网络依赖；`external_mcp_shadow` 启用时由 production Worker bootstrap 调用，真实 Shadow route 仍需显式受控 manifest 与 Profile。

`loadExternalMcpDeploymentPlan` 在启动接线之前提供唯一的 default-off 部署组合边界。它先解析一次 Profile，再以同一 owner UID 和 AbortSignal 顺序加载 production I/O 与 deployment route manifest；两者全部通过后才构造 production I/O runtime。返回值只包含 exact config、route Registry/routes、production runtime、Worker external-MCP 依赖和低敏 Runtime binding。readiness collector 与 gated Worker 因而共享同一 I/O snapshot、raw Registry 和有效上限，装配调用方无需重复拼接 binding options。

deployment plan 构造不会打开 Catalog/key/envelope/CA，不执行 preflight、DNS、TLS、MCP discovery 或 RPC，也不创建 Temporal Worker；任一 manifest、Profile join、owner 或取消失败都会返回固定低敏错误且不暴露部分计划。external Profile disabled 时连 Profile JSON、I/O/route manifest 路径都不读取。该 plan 由 `external_mcp_shadow` Runtime mode 的 Worker bootstrap 调用，基础 Compose 仍不选择该 mode。

`runtime.preflight(signal?)` 在一次固定逻辑时间内解析所有 enabled Profile 的 Catalog binding，精确核对 tenant/ref/version，再按完整 binding 与 CA ref 去重。它通过正式 AuthProvider 验证 fresh encrypted Secret、Bearer 编码和源 buffer 擦除，并通过正式 CA Provider 验证文件证据及 PEM/X509 内容。成功收据只包含 schema version、enabled、checked-at 和 Profile/Credential/CA 聚合计数；路径、tenant、Profile、opaque ref、证书和 token 都不会进入收据。任何 revoke、binding 漂移、key/AAD/envelope/CA 损坏统一返回固定低敏错误，取消在依赖边界传播。

preflight 不调用 Registry、Transport Factory、DNS Resolver 或 TLS Dispatcher，因此不会创建 Transport、DNS client 或 socket。它证明本地策略与文件依赖可用，无法证明远端 DNS、证书链、网络路由或 MCP 协议响应。

`runtime.shadowConnectivityDrill({ profileId, tenantId }, signal?)` 是独立的只读在线证据边界。它重新通过 Registry 解析 exact Profile/Catalog，创建正式 guarded Transport 和 modern allowlisted Client，只执行协议 discovery/list；Server identity 必须匹配，全部 configured Tool 必须被发现。演练器不暴露 Client 的 Tool 调用方法，成功或失败都会收敛关闭连接。成功收据只保存 schema、checked-at 与 Tool 数量；Profile、tenant、Server、Tool、地址和证书信息留在受控运维输入/网络审计中。

本地组合测试覆盖 exact binding、Tool 缺失、连接/握手/时钟失败、取消和清理；官方 HTTP handler 协议测试确认 modern discovery 完成且 `tools/call` 次数为零。public-only Guard 会拒绝 loopback/private 地址，所以真实 DNS/TLS 成功证据必须来自隔离 Shadow 环境，不能用本地私网绕过策略模拟。loader、composition、preflight 与 drill 已通过 `external_mcp_shadow` Runtime mode 接入 Worker；上线前仍需 provider owner 授权、只读 Shadow tenant allowlist、真实公网故障演练和回滚证据。

`runtime.readinessEvidence({ profileId, tenantId }, signal?)` 串行执行 preflight 和 drill，默认要求 5 分钟内完成，并校验 preflight 覆盖当前全部 Profile、Credential/CA 计数有界、在线 Tool 数等于目标 Profile allowlist、四个时间单调且均落在 collection window。任何收据重放、旧时间、计数漂移、取消或 cleanup 失败都不会生成 bundle；使用测试注入 `transportBuilder` 的 runtime 也固定拒绝出证。

`contracts/agent-external-mcp/v1/readiness-evidence.schema.json` 保留已发布的 v1 契约；`contracts/agent-external-mcp/v2/readiness-evidence.schema.json` 新增本次持久化要求的 Profile binding，避免在原版本上增加 required 字段。v2 的 `bindingSha256` 对排序后的所有 Profile 字段、Catalog/provider/key/secret/CA ref 与路径映射、owner UID 和 Catalog/Secret/CA/TLS/DNS/Auth/Shadow timeout 上限做 canonical SHA-256；`profileBindingSha256` 独立绑定本次 exact Profile 的 tenant、endpoint、credential version、网络策略与 Tool allowlist。bundle 本身只公开 hash、时间和聚合计数。路径存在于 Runtime binding preimage 中但不直接输出；token、key、envelope 和 CA 正文不参与摘要，避免把可离线猜测的凭据派生值写入运维记录。原地 token 更新会保留同一 binding，fresh preflight 负责证明采集时文件可解密且 Bearer 有效；推荐的版本化 ref/path 轮换会产生新 binding。

MySQL migration v37 新增独立的 `agent_mcp_readiness_evidence` 控制面表。Go Publisher 会严格解析低敏 schema、限制采集窗口最长 10 分钟、规范化毫秒时间、复算 content SHA-256，并把 tenant、Profile/Runtime binding、operator、request/trace、采集时间和最长一小时有效期一起派生为确定性 Evidence ID。表只支持追加：exact content/provenance 重放返回已有记录，binding、收据、窗口或 provenance 漂移生成新记录；历史过期行保留审计，但 fresh 查询必须同时命中 tenant、Profile binding、Runtime binding、`collected_at <= now` 与 `expires_at > now`。该表不依赖 Agent Task/Run，也不写 activation 状态。

Agent Capability 的 additive `PublishMcpReadinessEvidence` RPC 已连接上述 MySQL Publisher。它只允许 transport 认证的 `dipole-agent` 且 RequestContext principal 必须为空；operator 固定取认证 service identity，request/trace 从已验证上下文派生。请求不提供 Evidence ID、Runtime binding、content hash、status 或 activation 字段。Core 限制 evidence JSON 为 16 KiB 并严格解析 v2；TS adapter 会在发送前规范化字段与时间、复算 content SHA-256，并对响应的确定性 Evidence ID、双 binding、状态和时间逐项复核。exact replay 只改变 `created` 为 false。

`ResolveFreshMcpReadinessEvidence` 是对应的只读解析边界。调用方只能提交 tenant、Profile binding 与 Runtime binding，不能提交查询时间、Evidence ID 或 activation 意图；Core 使用服务端当前时间查询并重新验证记录。未找到返回严格空的 `found=false`，找到时只返回 Evidence ID、schema、双 binding、content hash、status 和 collection/expiry 时间。TS adapter 会拒绝带矛盾字段的空响应、binding/hash/schema/status 漂移及倒置时间。

默认关闭的 MCP Worker 在每次外部 `Registry.connect` 前消费该解析结果。Worker construction root 接受 host-owned Profile、production I/O、binding options 与 raw Registry，自行派生 exact Profile binding 和完整 Runtime binding；LLM、Workflow 与客户端均无法传入摘要或查询时间。每次连接都重新查询 Core且不缓存回执，随后再核对 raw Registry 返回的完整 Profile；缺失、双 binding/哈希/时间结构漂移、Profile 漂移、解析失败或取消都会在 raw `connect`、Catalog 与网络访问前停止。readiness collector 保留 raw Registry 以执行初次受控 discovery，避免形成“已有 readiness 才允许采集 readiness”的循环依赖。解析结果只授权本次 exact Profile egress，不改变 Run admission、Profile activation 或 Runtime promotion。

受控 Shadow 环境可通过独立单次命令采集并发布：

```bash
npm --prefix agent-runtime run mcp:readiness:publish -- \
  --tenant=TENANT-A \
  --profile=PROFILE-A \
  --valid-for-seconds=1800 \
  --request-id=CHANGE-REQUEST-123 \
  --trace-id=TRACE-123
```

执行前必须设置 enabled Profile、production I/O manifest 和 Agent Capability RPC/mTLS 环境。CLI 在任何文件或网络访问前严格校验五个参数；随后重新加载安全 manifest，构造 production adapters，串行执行全部 Profile local preflight 与 exact Profile 的只读 discovery，并在成功后调用一次 Publisher。有效期从 evidence `completedAt` 派生且限制为 60 至 3600 秒。采集、取消、清理、RPC 或收据校验失败只输出固定错误且不会自动重试。RPC 发出后若响应丢失，先按 request/trace 核对 Core 审计；重新运行命令会产生新的采集时间和 Evidence ID，作为追加历史保存。

该命令没有注册到常驻 `index.ts` 或 Compose，不会随 Agent 启动执行，也不读取 admission 或 activation 状态。MCP Worker 已具备 per-egress fresh evidence consumer，但 Worker、外部网络和自动采集调度仍未进入启动链；KMS 签名、可信时间戳和独立审计导出也尚未交付。bundle 作为可复算的运维完整性证据，需在隔离 Shadow tenant 与 trace/audit 联查。回滚 v37 前应先停止 Publisher 与 gated Worker，按保留策略导出证据；Down migration 会删除全部 readiness evidence 历史，之后所有 gated egress 都会 fail closed。

## Network Guard 边界

`createExternalMcpNetworkGuardedFetch` 可注入官方 SDK `StreamableHTTPClientTransport` 的 custom fetch。它在每个请求上重新校验 HTTPS、Host、Port、TLS ServerName 和无 query/credential 边界，再通过受信 Resolver 获取 1 至 32 个地址；全部答案都必须是格式、address family 和公网范围一致的唯一地址，混合私网答案或后续 rebinding 会在发起连接前被拒绝。

守卫把完整批准地址集合、TLS ServerName 和 opaque CA ref 交给 `ExternalMcpNetworkDispatcher`。Dispatcher 必须直接连接集合中的一个地址并返回 socket 实际 peer，守卫会再次核对；普通 hostname fetch 无法满足该接口。请求固定使用 `redirect=manual`，任何 `3xx`、`response.redirected` 或响应 URL 变化都会被拒绝并释放 body。

仓库已有 request-local Node DNS Resolver、文件 CA Provider 与 pinned TLS Dispatcher，并由 default-off production composition 统一持有；生产开关和启动接线仍然 fail closed。启用前还需通过隔离 Shadow 环境归档真实双栈 DNS、证书不匹配、连接 peer 偏移、超时与回滚故障演练。

## Result 信任边界

外部 MCP Server 返回的文本、结构化内容、Resource 和链接全部属于不可信数据。`externalMcpResultToContextFragment` 只接受成功、可 JSON 序列化且有界的 `CallToolResult`，并固定生成 `section=evidence`、`trust=untrusted` 的可选 fragment；provenance 同时绑定 Profile、Server、Tool 和 Invocation，原始对象在入口处转成不可变 JSON 快照。

预算不足时使用的 compact 内容只保留 Server、Tool、content type 与 structured-content 存在性，不摘要、不复述外部正文，避免未经审计的摘要把指令性文本隐式升级。完整结果进入模型时仍可能触发 Prompt Injection，`trust=untrusted` 需要与 system policy、Capability allowlist、Approval、trajectory Eval 和输出 lineage 一起使用。

当前外部 Client 尚未进入生产执行链。后续调用代码不得把原始 `CallToolResult` 直接拼接到 system/trusted prompt；结果转为 Artifact 或 Memory 时也必须保留 Invocation provenance 与 untrusted 来源，模型改写不能自动提升信任等级。

## Write Approval 边界

Core `ResolveApprovalGrant` 与 `ConsumeApproval` RPC 只接受认证的 `dipole-agent` 和 active Run。Core mTLS allowlist 显式包含这两个方法；其他 service identity、缺失或错误 shared secret 会在业务 handler 前拒绝。Resolve 使用 Task、Capability、Core 计算的 Resource Scope SHA-256 与 canonical Arguments SHA-256 查询最多两条 approved、未消费、未撤销且未过期记录，只在唯一 exact binding 时返回；查询不改变状态。Consume 随后通过 MySQL/sqlc 原子条件更新完成一次性消费，重放、吊销、过期和任一字段漂移都无法成功。

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

默认关闭的 Web Form 只消费 authenticated Task query/input/cancel API，并在查询失败时清空旧请求。浏览器验收覆盖 Chromium、Firefox、WebKit 的精确 Task/request 提交、untrusted Server/Tool/Invocation 来源披露、恢复重试、首个错误字段聚焦和 390x844 单列布局；字段错误通过 `aria-invalid` 与描述节点关联。该页面仍不接受密码、Token、支付信息或 URL mode 授权。

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

`deployment-route-manifest.schema.json` 将生产部署绑定固定为 credential-free v1 文件：每条 route 声明 route ID/version、Capability、Workflow step/ordinal、Profile/Server/Tool 和 effective egress policy。`ExternalMcpCapabilityDefinitionRegistry` 只保存代码拥有的 Capability descriptor、输入 schema、resource resolver 与不可突破的 egress ceiling；安全 loader 将两者与 enabled Profile allowlist 精确 join，生成同一个 `ExternalMcpCapabilityRouteRegistry` 和 `TemporalMcpDispatchRoute[]`。manifest 可以收窄参数名或字节上限，无法扩大代码 ceiling；重复 route、Capability、step/ordinal 坐标，以及未知 definition、Profile、Server 或 Tool 均拒绝。

route manifest 使用与 production I/O manifest 相同的 canonical parent、owner-only、`O_NOFOLLOW`、regular/single-link 和有界 UTF-8 JSON 证据；external Profile disabled 时不会读取残留路径。它不包含 credential、principal、Task/Run、arguments、Approval、模型、goal 或事件数据。loader 当前没有注册到 `index.ts` 或 Worker startup。

Invocation ID 由 tenant/principal/Agent/Task/Run/Workflow step/ordinal 的 canonical v1 绑定计算为 SHA-256。参数、Profile、Server、Tool、Approval 和 trace 不参与 ID 分叉；这些字段发生漂移时，相同 ID 会进入 Core exact begin 比较并 fail closed。这样 Activity retry 保持同一命令意图，显式增加 ordinal 才代表同一步内新的 Tool 调用。producer 可接收 Core 返回的 running/completed/failed 状态，为后续 receipt-only 恢复保留依据。

`FinishMcpToolInvocationFromRound` 提供专用 server-owned terminal API。Runtime 只提交 Task、Run、Invocation 和 Round ID；Core 重新加载两份持久记录，要求 exact binding、规范 terminal receipt 和已知 read-risk Capability，拒绝 `executing`、`input_required` 中间结果、write Capability 与任何漂移。首次完成由 Core 从 Invocation 开始时间和 Round 结果派生 latency、结果字节数、摘要或错误码，再调用既有审计 Finish；重试读取并核对已存 terminal Invocation，因此不会因重新计算 latency 产生冲突。默认关闭的 `createMcpTerminalWorkerRuntime` 在 complete 或稳定 failed Round 后调用该 API，ambiguous 和 waiting_input 不会提前收口。旧 Finish RPC 会先解析持久命令并拒绝任何带 Profile 的外部 Invocation，避免 Runtime 绕过 receipt 形成第二个终态所有者。

`TemporalMcpDispatchActivity` 提供独立、默认关闭的持久编排边界。begin 输入只含 host-owned route binding、Task/Run/principal、业务参数和低敏关联 ID；工厂配置固定 Capability、Workflow step 与 ordinal。deployment loader 先对 route/version、Capability descriptor、step/ordinal、Profile/Server/Tool 和排序后的 effective egress policy 生成 SHA-256；`temporalMcpDispatchRouteBinding` 再把该部署摘要与 route ID/version、Capability、step/ordinal 一起绑定，route ID/version 和最终摘要进入 Temporal Activity history。替换 Worker 的任意部署字段不匹配时都会在 Core 访问前拒绝，即使部署时遗漏 version 提升，也不会在 completion-loss retry 中生成第二个 Invocation 意图。

每次 begin、Activity retry 和 durable resume 都重新调用 Core context resolver，精确核对 Task/Run/principal，再以保存的 canonical 参数重放 `TrustedMcpInvocationProducer`。producer 返回同一 Invocation 后，Activity 仅向 terminal Worker 传递 Task、Run 和 Invocation ID。`wait_input` checkpoint 以 SHA-256 绑定完整 route manifest 摘要、Step 坐标、Invocation、参数、关联 ID 和内部 Worker checkpoint；恢复时任何漂移都会在 Worker 调用前失败。未来受信 Workflow 必须从版本化静态 route manifest 写入 binding，模型、goal、Kafka payload 和客户端参数不能提交或覆盖摘要。

terminal Worker 完成后，Activity 将不可信结果连同 Invocation/Round lineage 交给注入式 projector。`ExternalMcpArtifactProjector` 会从 Core 重新解析 completed Tool command，核对 tenant/principal/Agent/Task/Run 与 Profile/Server/Tool/Capability，调用 MCP 标准 schema 并要求 raw/parsed canonical JSON 完全一致，结果上限为 128 KiB。Artifact type 由稳定 Invocation 前缀派生，version 固定为 1，因此同一 Task 的不同调用各自拥有独立幂等键；metadata 只保存 untrusted 标记、命令 lineage、参数摘要和结果摘要，不复制参数或凭据。writer 返回的 Artifact ID、内容摘要、大小与 metadata 会再次复算。

Artifact RPC 已提交后发生取消时，projector 会让当前 Activity 失败；下一次 Activity retry 重新解析命令并精确写入相同内容，现有 content-addressed Store 返回同一收据。Workflow 输出只保存 Invocation ID、Round ID、Artifact ID/version，不保存外部正文。当前普通 Artifact policy 只允许 `dipole-agent` 的 running shadow Run，active MCP 结果会在写入前 fail closed，后续需独立扩展 active Artifact admission 与对应审计。

`createTemporalMcpDispatchRuntime` 将上述边界组装为一个 route-scoped、default-off Runtime。输入只接受 host-owned Route Registry、Core RPC port、Artifact writer、Transport Registry 和有界 timeout/client seam；同一 Registry 既驱动 `TrustedMcpInvocationProducer`，也按当前 Capability 派生 Worker 使用的唯一 Profile/Tool egress policy，因此装配层无法传入第二份漂移策略。Core port 同时承担 Context、Invocation begin/resolve、Round receipt 和 terminal finish，Artifact projector 也通过该 port 重新读取同一命令。

factory 的公开结果只有 `routeBinding` 与 `activities.executeMcpDispatch`。producer、terminal Worker、projector、Profile/Tool policy 和 Transport session 均留在闭包内，调用方不能跳过三 ID handoff 或替换完成权威。组合测试已证明首次成功后 Activity completion 丢失只读取 durable Round 并重放同一 Artifact，`input_required` 使用新 Context 与同一 Invocation 继续第二轮，预取消在 Core/receipt/Transport/Artifact 之前结束。

`createTemporalMcpMultiRouteRuntime` 将 deployment plan 的全部 route-scoped runtime 收敛到唯一 `executeMcpDispatch` Activity 表面，避免多个路由以同名 Activity 覆盖注册。构造阶段先验证非空路由集合、每条完整 route binding 与 route ID 唯一性，再为每条 route 注入同一个 plan Registry、gated external-MCP snapshot、Core port 和 Artifact writer。begin 仅以 payload 的 route ID 选取 runtime；resume 仅以 durable checkpoint 内的 route ID 选取 runtime。dispatcher 不自行接受 Capability/Profile/Tool，也不替代 route-local version、manifest/deployment digest 和 checkpoint 完整性校验。

未知 route、残缺 selector 和重复 route 会在 Core 调用前拒绝；route-local 绑定失败与 Temporal cancellation 不被包装或降级。该组合只创建无启动副作用的闭包和映射，不读取凭据文件、不注册 Worker、不执行 RPC、preflight、DNS 或网络连接。后续 Worker 接线必须只注册这一份 Activity，并由受信版本化 Workflow 写入 plan 返回的 route binding。

`TemporalMcpWorkflowExecutionCatalog` 接受 deployment dispatcher 返回的 route bindings，并为专用 `TemporalMcpTaskClient` 生成 `external_mcp_v1` history envelope。启动调用方只提供受信业务选择的 route ID 与 16 KiB 内 JSON object；catalog 注入 route version 和 manifest digest、规范化参数并拒绝空集、重复/未知 route。启动接口不接受 Profile、Server、Tool、Capability、egress policy 或任意摘要，因此 goal、模型输出、Kafka payload 和客户端正文无法覆盖部署 authority。

通用 `agentTaskWorkflow` 对 envelope 采用 additive 分支。首次执行从持久 admission 和 Core admission 结果派生 Task、Run、principal、request/trace，再调用唯一 `executeMcpDispatch`；`wait_input` 后只用 Activity 返回的完整 checkpoint 和状态机验证过的 Signal value 构造 resume。没有 envelope 的现有任务继续调用 `executeAgentTaskStep`，`TemporalTaskClient` 的普通输入类型不包含 execution authority。直接构造缺少 admission 或带附加 authority 字段的 envelope 会 fail closed，route-local Activity 仍会重新验证完整 binding 与参数。

本地 Temporal Server 已验证 MCP begin、durable Elicitation、Worker replacement 和 resume，并回归现有 retry、approval、cancel、input expiry、step budget 与 read-shadow recovery。`external_mcp_shadow` 已通过受控启动 root 将 `executeMcpDispatch`、专用 MCP Workflow client 和统一 Kafka/Temporal process 接入 `index.ts`；只有 Profile、Temporal、Kafka subscription trigger 与 Capability RPC 同时显式启用时才会构造资源。基础配置仍选择 `foundation`，因此默认不注册 MCP Activity、不消费 Kafka 且不建立外部连接。共享 Shadow tenant、真实公网 DNS/TLS、凭据和回滚证据继续独立受控。

`createExternalMcpTemporalWorkerComposition` 进一步把 deployment plan、multi-route runtime、普通 lifecycle Activities 与 Workflow execution catalog 收敛成一个 default-off bundle。plan 为 undefined 时函数在检查 base Activities 或解析依赖前直接返回；enabled 时先复算 Profile/I/O/readiness options 的 Runtime binding，验证 route binding、Capability egress policy、重复 route 与 `executeMcpDispatch` 名称冲突，然后才调用一次 Core/Artifact 端口 provider。该 provider 只交付端口，composition 不拥有或创建 gRPC resource。

multi-route factory 返回后，composition 会把每个 route ID/version/manifest digest 与预先计算的部署 binding 逐项比较，再公开冻结的 `activities`、`routeBindings`、`workflowExecutions` 和 `runtimeBindingSha256`。因此 Worker 注册和专用 Workflow client 可以消费同一份 authority snapshot，无法分别拼接 route catalog 或替换摘要。构造过程只实例化闭包和内存映射；测试使用真实默认 factory 证明没有 Core、Artifact 或 raw Registry 调用。

`TemporalWorkerActivities` 现允许 additive `executeMcpDispatch`，现有 foundation/persistent/read-shadow Activities 仍可原样注册。生产 `index.ts` 继续只按原三种 mode 构造 Worker，没有加载 deployment plan、调用 composition、创建 MCP RPC/Client 或注册 MCP Activity。后续启动切片必须保证 disabled 路径在 RPC 创建前返回，并为 enabled Shadow deployment 提供启动失败清理、readiness preflight、真实公网证据和明确回滚。

`loadExternalMcpTemporalWorkerStartupPlan` 负责 deployment loader 与 Worker composition 之间的资源所有权。它把 caller 提供或内部创建的 AbortSignal 传入 manifest loader 和 resource factory，严格按 `load -> validate -> resource -> compose` 执行；disabled plan 和静态 composition 冲突都在 resource factory 前返回或拒绝。resource 暴露 Core/Artifact dependencies、可选 Worker Activity snapshot 与 `close()`，可由后续启动层封装一个认证 RPC channel，但 startup plan 不依赖具体 transport。

startup 的第一次 validation 使用 host base Activities，保证 disabled/静态错误在 RPC 前拒绝；resource 返回后，composition 改用 `workerActivities`（若存在）并重新执行 Runtime digest、route、egress、Workflow catalog 与 Activity collision 校验。旧的通用 resource 没有 Activity snapshot 时继续使用 host base，实现保持兼容。任何 post-resource Activity 冲突都会走既有 rollback close。

`createExternalMcpAgentCapabilityRPCResourceFactory` 提供该认证 RPC resource 的生产 adapter。factory 构造本身没有 I/O；startup 真正请求 resource 时，它要求 Agent Capability RPC enabled、deployment 至少包含一个 Profile，并逐项确认 Profile tenant 等于 Shadow Runtime tenant。随后只创建一个 `AgentCapabilityRPCClient`，将同一实例同时作为 MCP Context/Invocation/Round/readiness/terminal Core port 与 Artifact writer，防止两个连接看到不同的授权或持久状态。

resource 还从该 client 派生 persistent `admitAgentTask`、`finishAgentTask`、Workflow projection 与 Approval Activities，并与 host 的 `executeAgentTaskStep` 合成冻结 `workerActivities` snapshot。这样 MCP Workflow 的 Run admission、Context resolve、Invocation、readiness 与 Artifact 都观察同一认证 Core transport；未来 `index.ts` 接线无需为 lifecycle 另建 `temporalRPC`。host 若提供 read-shadow Step Activity，该 Step 仍保持原实现，五个 lifecycle Activity 则始终由 resource client 覆盖。

RPC 构造后若 AbortSignal 已取消，factory 会先关闭 transport 再传播取消；构造错误固定为 unavailable，回滚或显式 close 错误固定为 cleanup failed。成功 resource 的 dependencies snapshot 冻结，close Promise 对成功和失败都只执行一次。该 adapter 没有修改 Proto，也不自行加载 deployment、启动 Worker、执行 readiness 或访问外部 MCP 网络；`external_mcp_shadow` 启用时由受控启动 root 按同一 deployment snapshot 调用它。

resource 创建后若取消、composition 抛错或返回空结果，startup plan 会先调用一次 rollback close。清理成功时取消保留原 Abort reason，其他构造错误固定为低敏 unavailable；清理本身失败统一报告固定 cleanup failure，避免隐藏潜在资源泄漏或暴露 RPC target。成功返回的 `close()` 缓存首次 Promise，重复关闭以及首次关闭失败后的重试都不会再次触达底层 resource。

成功结果同时保存 exact `deployment` 与 `worker` composition，受控 Shadow 启动链以同一 snapshot 编排 Worker、专用 Workflow client 和停止顺序。该层本身不创建 Temporal Worker/Client、启动轮询、执行 readiness preflight/drill 或访问 raw Registry；只有 `external_mcp_shadow` mode 的启动 root 消费其结果。代码拥有的只读 Capability definition、受控 route manifest、RPC resource factory 与“先停 Worker/Client、后关 resource”的集成测试均已纳入该链路；共享环境的真实 manifest、readiness 和回滚演练仍待完成。

`startExternalMcpTemporalWorkerLifecycle` 将 managed startup snapshot 与现有 `TemporalWorkerRuntime` 收敛为一个 owner。undefined snapshot 在读取 Temporal config 状态或创建 Worker 前返回；enabled snapshot 要求 Temporal config 同时 enabled，并把 composition 的 exact Activities 交给 Runtime。Runtime factory 同步失败、Worker 未进入 RUNNING 或后续启动失败都会先停止已创建的 Runtime，再关闭 startup resource；若 Runtime 尚未构造，则直接归还 resource。

成功 lifecycle 的 `stop()` 固定先停止 Worker polling 并关闭 Temporal connection，再关闭 startup 持有的 Core/Artifact resource。前一阶段失败不会阻断后一阶段，最终只返回固定低敏 shutdown error；首次成功或失败 Promise 都会缓存，重复 stop 不会再次触达任一 owner。该层仍不加载 manifest、创建 RPC、发布 readiness、启动 Workflow client 或修改生产进程，`index.ts`/Compose 与外部网络继续关闭。

`startExternalMcpShadowWorkerBootstrap` 是当前完整但默认关闭的 Worker startup root。它先创建 seal 的 `repository.issue.read` definition Registry，再把 environment、base Activities 与 lazy RPC resource callback 交给 managed startup plan；只有 enabled deployment 通过 Profile/I/O/route/static composition 校验后，callback 才构造 RPC factory 和 transport。随后 exact startup snapshot 只交给 Temporal lifecycle 一次，成功结果直接公开同一 deployment、Worker composition、host Workflow route catalog 与 stop handle。

bootstrap 在 lifecycle 调用前拥有 startup：load 完成后的取消会先关闭 RPC resource，再传播 Abort reason；关闭失败返回固定 cleanup error。调用 lifecycle 后 ownership 完全转移，bootstrap 不捕获并二次关闭，启动失败由 lifecycle 按 Worker/connection/resource 顺序回滚。disabled deployment 不构造 RPC factory、RPC transport 或 Worker。该 root 由 `external_mcp_shadow` 的 process owner 调用，并会组合 `TemporalMcpTaskClient`；基础 Compose 不启用该 mode，也不自动发布共享环境 readiness。受控 Shadow 启用后，无 fresh evidence 的外部 egress 仍逐请求 fail closed。

`TemporalMcpShadowTaskDispatcher` 提供事件驱动路径的可信 Workflow start boundary。它重新解析 `AgentEvent` 与 `AgentIdentity`，按 tenant、Agent、event type 和 aggregate 复算确定性 Task ID；不匹配会在 route selector 与 Temporal Client 前拒绝。admission 与固定 goal 在 selector 调用前由 Runtime 固化，事件和身份快照也会冻结，因此 selector 只能根据受信宿主逻辑返回 strict `{routeId, arguments}`，不能覆盖 tenant、principal、Agent、trigger、request/trace 或 goal。

dispatcher 随后调用专用 `TemporalMcpTaskClient`，由 matching host catalog 注入 route version 与 manifest digest，再写入 `external_mcp_v1` history。业务参数仍会经过 16 KiB canonical JSON、route-local Capability schema、egress policy、Core Context 与资源权限复核。该类由 `external_mcp_shadow` process 在受管 Client 生命周期内使用；它自身不创建 Temporal connection，基础 Compose 也不会选择该 mode。即使 mode 已接线，也不得直接以消息正文、模型输出或事件 payload 选择 route。

subscription mode 现在将 Core 返回且经本地 filter 选中的 subscription 固化为 `subscriptionBinding`：其中只含 subscription ID、definition ID/version、tenant 与 Agent。事件 schema 要求 binding 的 subscription ID 与 admission 使用的顶层 ID 一致；旧 direct-target 和只有顶层 ID 的事件仍可解析。`TemporalMcpSubscriptionRouteSelector` 进一步把 exact definition ID/version 映射到代码注册的 route 和参数 resolver，并在 resolver 前核对 tenant/Agent。definition 版本升级不会沿用旧映射，重复 binding、未知版本和非对象参数均固定拒绝。

deployment route manifest 的可选 `subscription_trigger` 现在提供首个 production-ready registration：同一 host-owned route 绑定 exact Definition ID/version 与静态 JSON 参数。加载器先执行代码 Capability input schema，再要求全部参数名落在 route egress allowlist 且 canonical JSON 不超过 route 上限；重复 Definition binding、schema 失败和扩权全部使完整 manifest 失效。Definition 与参数同时进入 deployment binding SHA-256，配置变化会形成新的 Temporal route history authority。

Worker composition 在资源创建前再次验证全部 trigger route 属于 exact Workflow catalog，并复制冻结 route snapshot。`createExternalMcpSubscriptionRouteSelector` 只从该 snapshot 构造 selector，空 mapping 或 catalog drift 固定拒绝。selector 仍不接受 Profile、Server、Tool、manifest digest、admission 或 goal；静态参数随后继续经过 route-local schema、egress、Core Context 与 resource scope。旧 manifest 可以不含 `subscription_trigger`，但无法进入后续 production subscription process。模型输出和消息字段不能解释为 route ID 或覆盖静态参数。

`startExternalMcpTemporalClientLifecycle` 提供受管 Workflow start connection。它只接受已启动的 `ExternalMcpTemporalWorkerLifecycle`，因此直接复用 Worker owner 冻结的 address、namespace、task queue 和 `workflowExecutions`；调用方没有第二份 Temporal config 或 route catalog 输入。Worker disabled 时 selector factory 与 Client resource factory 均不会调用；enabled 时先构造无网络 selector，再连接 Temporal。连接期间取消或后续构造失败会回滚 resource，错误只暴露固定 startup/cleanup 分类。

Client lifecycle 只实现受信 `ShadowTaskDispatcher` 与 `stop()`。stop 立即关闭新 dispatch admission，等待已接受的 Workflow start 全部收敛后关闭独立 Client connection，并对成功或失败缓存同一 Promise。它不停止 Worker，也不关闭 Worker 持有的 Core/Artifact RPC；process owner 先停止 Kafka consumer，再停止该 Client，最后停止 Worker lifecycle。该 owner 已由 `external_mcp_shadow` mode 组合到 Runtime 入口，但只在完整显式配置下启动；默认配置没有生产 route registration，也不会建立外部 MCP 网络连接。

`startExternalMcpShadowTemporalRuntime` 是 Worker 与 Client 的单一 Temporal process owner。它先调用完整 Shadow Worker bootstrap，disabled deployment 直接返回；enabled 时把同一 Worker owner、route selector factory 和 AbortSignal 交给 managed Client。Client 构造失败会停止 Worker，Client 已交接后的取消会依次停止 Client 与 Worker。任何 rollback 阶段失败统一报告 cleanup failure，避免把半关闭状态误报为普通 startup failure。

成功结果只公开 exact deployment、Worker composition、冻结 Temporal config、可信 `dispatch` 与幂等 `stop`。stop 固定先让 Client 拒绝新请求并 drain 已接受 Workflow start，再停止 Worker polling、Temporal connections 和同一 RPC resource；Client 或 Worker 失败都不会阻断后续清理。上层 process owner 负责 Kafka consumer，并已由 `external_mcp_shadow` mode 接入 `index.ts`；基础 Compose 继续关闭。受控 production route/resolver 与真实 readiness/Shadow 证据仍是启用前置条件。

`startExternalMcpShadowProcess` 进一步拥有 Kafka consumer 与上述 Temporal process。它只接受已启用的 subscription trigger：先启动 Temporal Worker/Client，再创建并启动 Kafka；disabled Kafka 或 Temporal deployment 保持零 Kafka 副作用。Kafka 构造、启动或交接后取消会先回收任何已创建的 Kafka runtime，再回收 Temporal owner。正常 stop 同样先停止 Kafka 接收新事件，再 drain Workflow Client 并关闭 Worker/Core resource，成功或失败均幂等。

subscription matcher 由 Worker 的 Agent Capability RPC resource 从同一个认证 client 投影，并沿 startup plan、Worker lifecycle 和 Temporal owner 保持引用一致；Kafka runtime 只借用该 matcher，不拥有或关闭 transport。这样 subscription 授权、persistent Workflow Activities、MCP Core 与 Artifact writer 共享一条身份和连接视图，同时 Temporal resource 仍是唯一关闭权威。matcher 缺失会在 Kafka 构造前 fail closed 并回收 Temporal。该 process 已由后述独占 mode 接入 `index.ts`，Compose 与生产开关继续关闭，只有显式完整配置才会建立资源。

常驻入口现提供显式 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=external_mcp_shadow`。该 mode 只有在 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=true`、Temporal enabled、Kafka enabled、`DIPOLE_AGENT_TRIGGER_MODE=subscription` 和 Capability RPC enabled 同时成立时才可启动；外部 Profile 开关与 activity mode 只启用一侧也会固定拒绝。入口在该 mode 下不构造原有 Kafka Shadow runtime 或通用 Temporal Worker，完整生命周期只交给 `startExternalMcpProductionShadow` 一次，因此同一 task queue 不会注册两份不同 Activity catalog。

Compose 仍保留 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=false`、Temporal disabled 与 `foundation`，所以发布后没有默认网络或消费行为。隔离验收组合使用临时 in-memory Temporal Server 与独立 task queue 验证恢复、替换、取消和 MCP history，同时用本地 modern Streamable HTTP Client/Server 只执行 initialize 和 Tool discovery，断言 `tools/call=0`。该证据验证入口策略、Workflow 与协议只读面；真实 Profile/I/O/route manifest、Core/MySQL、Kafka、凭据、public DNS、pinned TLS 与 fresh readiness 仍需独立 Shadow tenant 联合演练。

### 隔离全栈 Shadow 演练

`scripts/drill-agent-external-mcp-shadow.sh` 提供默认不随单测执行的 owner-only 联合证据入口。脚本使用随机 Compose project、随机 loopback 端口和临时卷启动独立 MySQL 8.4 与 Kafka 3.9；随后生成临时 CA、`dipole-core` 服务证书和 `dipole-agent` 客户端证书，启动环境门控的 Go test Core RPC fixture；Vitest 再启动临时 Temporal Dev Server 和本地 modern MCP Server。退出时始终停止 fixture 并删除证书、容器、网络与卷，不读取或重启共享 `dipole-node*` 服务。

演练加载 mode `0600` 的 production route manifest，并复用正式 Capability definition、route/egress policy、Kafka consumer、MySQL EventLedger、Temporal Workflow、MCP Client 与 Artifact projector。Core fixture 仅在 Go test binary 中编译，通过生产 `newInternalRPCServer` 强制 TLS 1.3、客户端证书验证、metadata secret、caller allowlist 及证书 CN 一致性，并实现受信 subscription、Run/Invocation/Round/readiness/Artifact 隔离状态；TS 侧统一使用正式 `AgentCapabilityRPCClient`。MCP 传输仍通过显式本地测试边界注入，因此不会降低 production `public_only DNS + pinned TLS + encrypted secret` 防线，也不会获得 production authority。

同一入口也执行 Approval gate 场景。它经 mTLS RPC 创建并批准 exact binding，验证一次受限 write operation；随后验证 denied binding、已消费 binding 与首次 operation failure 都不会触发第二次 operation。该测试只调用内存 fixture 的受控 operation，不发送 IM 消息，也不构成 active write、共享 Shadow 或 service-side commit 后不确定性证据。

成功后，`approval:drill:check` 会验证 `contracts/agent-approval/v1/approval-gate-drill-evidence.schema.json` 对应的短期 receipt。它只记录固定 effect 基数、认证类型、时间窗、`production_authority=false` 与 canonical SHA-256；artifact 写入 gitignored `.artifacts/` 且默认 24 小时过期。它与 External MCP read receipt 独立，不能用于推断真实消息写入、共享环境状态或提交后的回滚语义。

成功路径发送一个 subscription event，要求 exactly one allowlisted `read_issue` Tool 调用及一个 untrusted Artifact。随后以同 consumer group 和持久 ledger 重启 Runtime并重发同 Event ID，确认不启动第二个 Workflow或 Tool；最后让 readiness receipt 过期后发送新事件，要求 Workflow 收敛为 failed 且 Tool count 不增加。fresh gate 现在使用 Worker 当前时钟要求 `expiresAt > now`，因此历史回执会在 raw Registry、Catalog 与 Transport 之前拒绝。

证据默认写入 gitignored 的 `services/agent-runtime/.artifacts/external-mcp-shadow-drill.json`，文件 mode 为 `0600`，只包含 schema、通过状态、隔离类型、采集/失效时间、聚合计数、Core RPC 类型/认证门禁、布尔结果和 canonical `content_sha256`，不包含 tenant、Profile、Task、Event、Tool、路径、端口、消息正文、Token 或底层错误。v1 Schema 保留用于历史解释，`contracts/agent-external-mcp/v2/shadow-drill-evidence.schema.json` 固定当前语言中立结构；Runtime Zod parser 校验 canonical hash、最多 24 小时有效期及当前时钟，脚本末尾通过 `npm run mcp:shadow-drill:check -- --evidence=<path>` 复核完整证据。可用 `DIPOLE_AGENT_MCP_DRILL_EVIDENCE` 指向受控归档路径。内容 hash 用于发现文件漂移，没有签名身份或 production authority；当前 mTLS 证据覆盖隔离 Core composition，真实共享 Core 身份、公共 DNS/证书链/peer pinning、凭据轮换/吊销和 provider owner 仍需独立 Shadow tenant 演练。

2026-08-30 已在 Remote GPU 一次性 worktree 上复核运行该脚本：隔离 MySQL、Kafka、Temporal、Go Core mTLS fixture 与本地 MCP 共同通过。证据为 `outcome=passed`、`event_count=2`、`ledger_completed_event_count=2`、`tool_call_count=1`、`artifact_count=1`；同 consumer group 重启重放被抑制，过期 readiness 被拒绝，Core RPC 的 mTLS 身份拒绝检查通过。该运行使用 disposable 资源并明确返回 `production_authority=false`，没有连接共享 Core、Kafka、Temporal、Provider 或外部 MCP Server。

2026-09-01 在候选 revision `3c1f3eba87921419ff7186b1ea7ff09d1a7206f9` 复跑通过。该 Remote GPU 的 disposable 环境受宿主 Linux AIO 配额影响，drill 专用 MySQL 已显式关闭 native AIO；基础微服务 Compose 未改动。低敏 MCP 和 approval receipt 已归档于 [agent-mcp-approval-shadow-2026-09-01](../../benchmarks/agent-mcp-approval-shadow-2026-09-01/)，确认单次本地 Tool/Artifact、重启去重、过期 readiness 拒绝、mTLS identity denial，以及一次已批准 fixture operation 的精确副作用基数。该证据继续限定为本地 fixture；approval 的 `denied_effect_count=0` 只表示零拒绝副作用，尚未覆盖审批 UI 或共享环境的 deny 流程。

Approval receipt v2 进一步将拒绝语义与 effect count 分开绑定：`denied_authorization_rejected`、`consumed_replay_rejected` 和 `failed_replay_rejected` 都必须为 `true`，对应的 effect count 保持零。v1 receipt 继续作为历史归档；下一次 disposable drill 会产生 v2 receipt。该版本化不会把 fixture 结论扩展到审批 UI、IM 写入、真实外部 MCP 或共享环境。

2026-09-01 在候选 revision `f0dcf98a0b366031f7097cfd331318d39a9cf7a6` 完成 v2 drill。归档的 [v2 receipt](../../benchmarks/agent-mcp-approval-shadow-2026-09-01-v2/) 已同时验证 deny、consumed replay 和 failed-operation replay 都被拒绝且没有新增 effect；同一运行仍通过本地 MCP、EventLedger 重启去重、过期 readiness 与 mTLS identity denial。该结论保持在 disposable fixture 范围内。

该 Activity 已由通用 `agentTaskWorkflow` 的 `external_mcp_v1` 分支引用，但没有注册到生产 Worker、`index.ts` 或现有 Activity mode。当前启动链也没有外部 Capability route；第一方 Message write 继续使用带 action reference 的现有 Finish 路径，外部 write Capability 尚无通用可验证 action receipt。在真实路由注册、受控调度、active Artifact policy 和生产 I/O 完成前，生产 Worker 与外部网络开关继续关闭。

## 后续实现门槛

Transport Factory 已完成版本精确绑定、每请求 fresh Secret、公共 DNS 全地址、重定向拒绝、peer 复核及 SDK 隐式重试关闭的组合。`NodeExternalMcpDnsResolver` 现提供真实 Node DNS 适配器：每个 fetch 使用独立 Resolver 并行请求 A/AAAA，不保留跨请求缓存；`ENODATA/ENOTFOUND` 只表示当前 family 无记录，任何其他单族错误会使完整解析失败。调用取消会执行 request-local `Resolver.cancel()`，不会影响其他并发请求。Network Guard 随后继续执行公网、family、重复和数量复核。

`NodeExternalMcpPinnedTlsDispatcher` 接收 Network Guard 当前请求批准的完整地址集合，通过 `https.request` 的自定义 lookup 只返回该集合；`agent=false` 阻止连接复用绕过 fresh DNS，Node 直连路径不会采用环境代理。TLS 固定 ServerName、CA、最低 TLS 1.2 和证书链校验，响应建立后还会把 socket `remoteAddress` 与批准集合再次比较；注入式 client 的 peer 证据也由 Dispatcher 外层复核。请求和响应 body 保持流式，100 ms 至 60 秒的 connect timeout 只覆盖 TLS `secureConnect` 前窗口，取消会销毁当前 request。3xx 不在该层跟随，由 Network Guard 统一拒绝。

文件 CA provider 将 opaque ref 映射到规范绝对路径，每次 dispatch 都重新打开并加载，以支持受控原子轮换。父目录必须 canonical 且不可被 group/world 写，文件使用 `O_NOFOLLOW` 并要求 regular、single-link、root/expected-owner、不可被 group/world 写、256 KiB 默认上限；内容只允许 1 至 32 个可解析 PEM certificate。该 provider 适合静态 CA bundle，私钥和 Bearer secret 不得进入此映射。

生产接入仍至少需要：启动前下游文件 preflight、每租户 provider owner 授权、secret lease/吊销告警、低敏审计及真实公网故障演练；更高安全级别部署还需 Vault/KMS/Secret Manager adapter。Secret 只在 Factory 内短暂使用，组合接口只向 Runtime 返回 tenant-bound Registry。当前 loader/composition 未注册到 `index.ts` 或 Worker startup，不会读取 manifest/凭据、发起查询或建立连接。

完成上述门槛后，先在独立 Shadow tenant 接入一个只读 Server，验证 Server identity、Tool allowlist、取消/超时、Prompt Injection provenance 和凭据轮换，再评估按租户灰度。回滚始终先关闭外部 MCP 开关并等待在途 Tool 调用收敛。
