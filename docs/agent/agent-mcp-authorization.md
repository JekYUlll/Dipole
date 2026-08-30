# Agent MCP 授权边界

本文记录 Agent G4 第一方 MCP 网络入口的令牌、资源和部署边界。当前实现用于受控 Shadow 与第一方客户端接入；第三方 MCP Host 所需的 OAuth 2.1 discovery、PKCE、客户端注册和 refresh token 尚未开放。

## 信任链

```text
User session JWT
  -> POST /api/v1/auth/agent-mcp/token (explicit consent)
  -> 15 minute MCP access JWT (aud + scope + token_use)
  -> Gateway audience/scope validation
  -> trusted internal resource/scope attestation
  -> TypeScript Runtime AuthInfo
  -> read-only Capability Policy
```

普通登录令牌的 `token_use=session`，MCP 访问令牌的 `token_use=agent_mcp_access`。两类解析器互相拒绝。MCP 令牌只包含一个 `aud`，Scope 固定为 `dipole.agent.mcp.read`；Gateway 删除外部 `Authorization`、Cookie 和所有内部身份头，再注入已验证的 principal、resource 与 scope，Runtime 逐项复核后才建立 `AuthInfo`。

单次 Runtime Tool invocation 默认最长 5 秒，可通过 `DIPOLE_AGENT_MCP_TOOL_TIMEOUT_MS` 在 100 ms 至 60 秒之间调整。超时触发 cooperative `AbortSignal`，审计以 `tool_timeout` 收敛；外部 MCP Client foundation 的 connect、Tool discovery 和 call 默认使用 10 秒 request/total timeout，也接受调用方 AbortSignal。Gateway 保留 Streamable HTTP 长流和 DELETE 清理，不设置会截断 SSE 的全局代理超时；客户端断连由 Runtime 转换为 Request signal。

## 第一方授权交换

调用方先使用普通登录令牌请求：

```http
POST /api/v1/auth/agent-mcp/token
Authorization: Bearer <session-token>
Content-Type: application/json

{
  "resource": "https://dipole.local/api/v1/agent/mcp",
  "scopes": ["dipole.agent.mcp.read"],
  "consent": true
}
```

主体由认证上下文派生，请求中的额外用户字段不会参与授权。resource、scope 或 consent 不精确匹配时返回 `400`；生成的 Bearer token 有效期为 900 秒，不能调用普通 Dipole API。

## 配置与发布

共享环境必须把 `DIPOLE_AGENT_MCP_RESOURCE` 设置为公开 MCP 入口的 canonical HTTP(S) URI。Compose 会把该值分别映射给 Core/Gateway 的 `DIPOLE_AUTH_AGENT_MCP_RESOURCE` 和 Runtime；URI 禁止凭据、query 与 fragment。Core、Gateway 和 Runtime 必须使用完全相同的值。

建议发布顺序：

1. 保持 `DIPOLE_AGENT_MCP_SERVER_ENABLED=false` 和 `DIPOLE_GATEWAY_AGENT_MCP_ENABLED=false`，先滚动 Core、Runtime、Gateway。
2. 固定公开 URI并验证三个进程的 resource 配置一致。
3. 根据依赖延迟设置 `DIPOLE_AGENT_MCP_TOOL_TIMEOUT_MS`，先保持默认 5000 ms。
4. 启用 Runtime，再启用 Gateway，使用第一方交换取得短期令牌执行只读 `tools/list`。
5. 验证普通 session token、错误 audience、错误 scope 均返回 `401`，客户端令牌未到达 Runtime，并验证超时审计收敛为 `tool_timeout`。

回滚时先关闭 Gateway MCP 开关，再关闭 Runtime MCP 开关。该变更没有数据迁移；既有 session JWT 仍兼容，已签发的短期 MCP token 最多 15 分钟后自然失效。

## 后续门槛

面向通用 MCP Host 前仍需实现 RFC 9728 Protected Resource Metadata、OAuth 2.1 Authorization Code + PKCE 和客户端注册策略。`oauth-discovery-pkce.ts` 已提供默认关闭的基础：按 RFC 8414 派生 authorization-server metadata URI，要求 issuer 精确匹配、HTTPS 与 `S256`，并只生成 Authorization Code + PKCE 的 verifier/challenge/state 材料。`discoverAuthorizationServerMetadata` 仅通过调用方显式注入的 fetch 访问该精确 URL，固定禁止重定向，限制超时、64 KiB JSON 响应和错误状态；它没有接入 Runtime 默认 composition。

`oauth-authorization-transaction.ts` 已定义后续持久化的短时事务记录：state 仅保留 SHA-256，verifier 使用 AES-256-GCM 密封，AAD 绑定 transaction、owner、issuer、redirect URI、state digest 与绝对 expiry。Store 必须按 transaction、owner、state digest、未过期和未消费条件原子 consume，再允许解封 verifier；禁止内存 fallback。不得将 state、verifier、authorization code 或 token 写入 Profile、Temporal history、Context、Tool 参数、审计或日志。

Core Agent Capability 已预留 `ConsumeOAuthAuthorizationTransaction` RPC。它仅接受 `dipole-gateway` 服务身份，owner 只从认证 `RequestContext` 恢复；Gateway 只能传递 transaction ID 与 state SHA-256。Core 在返回固定 issuer、callback、expiry 和 sealed verifier 前完成条件消费。Core standalone bootstrap 仅在 `internal_rpc.agent_oauth_authorization_transaction_consume_enabled=true` 与内部 RPC mTLS 同时满足时注入 SQLC Store；默认仍拒绝为 `Unavailable`，因此现有部署不会意外启用 OAuth callback。Gateway 仍不得解封 verifier，也不得将返回载荷写入日志。

Gateway 内部已提供与该 RPC 对应的未装配 client。它仅使用已有 Core mTLS 通道，并校验返回 transaction、HTTPS issuer/callback、expiry 与 base64url 密封 verifier；client result 只能用于后续 Runtime handoff，禁止进入浏览器响应、审计或日志。当前没有 Gateway Dependency、bootstrap 配置或 HTTP route 使用该 client。

当前仍缺少 Gateway 到 Runtime 的短时受认证 handoff、callback HTTP、RFC 9728 Protected Resource Metadata、Runtime 解封后的 token code exchange、客户端注册、refresh 与撤销流程。外部 MCP Server 的 Profile/凭据边界见 `docs/agent/agent-external-mcp.md`；生产 Secret Provider、write/destructive Capability、Elicitation URL mode 继续由 `AD-037` 管理。
