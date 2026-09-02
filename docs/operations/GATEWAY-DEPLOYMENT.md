# IM Gateway 渐进部署手册

本文档描述 M5 独立 Gateway 的部署、验收与回滚。默认 `gateway.mode=embedded` 保持模块化单体路径；`remote` 将公开 HTTP/WS 和实时投递交给 `cmd/services/gateway`。

## 职责边界

| 进程 | 公开入口 | 持久化依赖 | 主要职责 |
| --- | --- | --- | --- |
| `cmd/services/gateway` | HTTP/HTTPS、WebSocket | 无 MySQL | WS 认证、限流、连接、Redis Presence、Kafka 实时投递、Core HTTP 代理 |
| `cmd/services/core` | 仅私网 HTTP 与 Core RPC | MySQL、Redis | Auth、User、Group、Contact、File、Conversation 与领域投影 |
| `cmd/services/message` | 仅内部 Message RPC | MySQL、Redis | 消息命令、历史、幂等、Inbox、Outbox 与消息持久化 |

Gateway 自己处理 `/health` 和 `/api/v1/ws`。其余 HTTP、Swagger 与静态 Web 在 M5 期间代理到 Core，后续可按流量和安全需求继续抽离。

## 配置门禁

独立模式要求：

```text
DIPOLE_GATEWAY_MODE=remote
DIPOLE_MESSAGE_TRANSPORT=grpc
DIPOLE_INTERNAL_RPC_ENABLED=true
DIPOLE_KAFKA_ENABLED=true
DIPOLE_GATEWAY_CORE_HTTP_TARGET=http://127.0.0.1:8081
```

Core 监听私网地址或 loopback，公开负载均衡器只连接 Gateway。`remote` 模式下 Core 不注册 `/api/v1/ws`，也不消费实时投递 handler；它继续消费领域投影。Gateway consumer group 固定为 `dipole-gateway-consumer`，不与 Core 投影或 Message persistence 竞争分区。

跨主机或容器网段必须启用 TLS 1.3 mTLS。三个进程分别使用 CN 为 `dipole-gateway`、`dipole-core`、`dipole-message` 的证书，共享同一内部 CA，但不共享私钥。共享 RPC secret 通过运行时 secret 注入。

Gateway 不需要 MySQL 环境变量或数据库网络权限。它需要 Redis、Kafka、Core HTTP、Core RPC 和 Message RPC 的网络访问。

## 访问日志安全

Gateway 与 embedded Core 共用 Gin 结构化访问日志。日志中的 `path` 会保留普通 query 诊断信息，并对 token、access/refresh/id token、Authorization、API key、client secret、密码和签名类参数进行大小写无关脱敏；同一键的多个值分别替换为 `REDACTED`。无法安全解析的 query 只记录固定脱敏值，不回退原始 URI。

WebSocket 当前继续兼容 `token`、`access_token` query 和 Bearer Header。新增短期 ticket、签名参数或其他 query credential 时，必须先扩展 `internal/logger` 的敏感键集合和真实 WebSocket 日志 capture 测试。公开 Nginx、Ingress、CDN 和日志 Agent 位于应用脱敏边界之外，需要分别关闭原始 URI、Authorization 与 Cookie 记录，验收日志正文不得出现可重放凭据。

## 切换顺序

1. 按 [Message Service 手册](MESSAGE-SERVICE-DEPLOYMENT.md) 完成 owner 模式和受限数据库账号验收。
2. 保持 `gateway.mode=embedded`，确认 Core/Message 的 Remote 契约、Kafka 和 Redis Presence 正常。
3. 准备 Core 私网监听端口，例如 `8081`，并配置 `gateway.mode=remote`、`message.transport=grpc`。
4. 并行启动 Core 与 Message；Core 会先开放 Capability listener，再连接 Message。编排器应配置健康检查和失败重启。
5. 启动 `go run ./cmd/services/gateway`，确认 `/health`、Core HTTP 代理和两个内部 RPC 健康。
6. 将少量测试流量切到 Gateway，验证登录、WS 重连、私聊、群聊、文件消息、踢下线与跨节点投递。
7. 完成全流量切换后，移除公开网络到 Core 的直连规则；保留 `embedded` 配置和单体制品作为回滚路径。

## 验收

- Gateway 运行账号无法连接 MySQL，HTTP/WS 职责仍正常。
- Core 的 `/api/v1/ws` 在 `remote` 模式返回 404，公开入口的同路径可以升级连接。
- 每条 `message.created` 由 Gateway delivery group 消费并路由到目标 Presence 节点。
- Core 继续更新 Conversation，Message 继续持久化消息和 Outbox，三类 consumer group 互不竞争。
- Gateway、Core 和 Message 的 mTLS caller 与证书 CN 一致，伪造 caller 被拒绝。
- 使用 query token 和 Bearer Header 分别建立 WebSocket，采集应用及前置代理访问日志，确认只出现 `REDACTED` 且无 JWT、共享密钥或 Cookie 正文。
- SIGTERM 先停止公开 HTTP/WS，再关闭 Kafka、Pub/Sub、RPC 和 Redis 连接。

## 回滚

1. 从公开负载均衡器摘除独立 Gateway，停止新建 WS 连接。
2. 将 Core 改回 `gateway.mode=embedded` 并重启；若同时回滚 Message，再按 Message 手册切回 `message.transport=local`。
3. 将公开流量恢复到 Core，验证 HTTP、WS、Kafka 投递和 Presence。
4. 停止独立 Gateway。该过程不修改 schema，也不需要回滚消息数据。

如果故障仅发生在 Core HTTP 代理，可先停止流量切换并保持独立 Message owner；不要临时公开 Core RPC 或放宽非 loopback 明文限制。
