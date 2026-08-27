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

## 后续实现门槛

生产 Factory 至少需要：每租户 provider owner 授权、加密 Secret Provider、版本精确读取、lease/zeroization、DNS 全地址和重定向检查、TLS chain/ServerName 校验、有界连接超时、低敏审计及故障演练。Secret 只在 Factory 内短暂使用，接口只向 Runtime 返回已建立的 MCP Transport。

完成上述门槛后，先在独立 Shadow tenant 接入一个只读 Server，验证 Server identity、Tool allowlist、取消/超时、Prompt Injection provenance 和凭据轮换，再评估按租户灰度。回滚始终先关闭外部 MCP 开关并等待在途 Tool 调用收敛。
