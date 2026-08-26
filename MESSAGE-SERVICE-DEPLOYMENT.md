# Message Service 渐进部署手册

本文档描述 M4 Message Service 的影子验证、流量切换、验收和回滚顺序。`message.transport=local` 始终是 Core/Gateway 的快速回切路径。

## 运行模式

| 进程 | 配置 | 职责 |
| --- | --- | --- |
| Core/Gateway | `message.transport=local` | 本地 Message primary，继续拥有 Kafka persistence 与 Outbox |
| Core/Gateway | `message.transport=grpc` | 通过受认证 RPC 调用独立 Message，不再消费 send-request 或运行 Outbox |
| Message | `message.runtime_mode=shadow` | 只提供四类查询，拒绝全部发送命令，不启动 Kafka consumer/Outbox |
| Message | `message.runtime_mode=owner` | 提供命令和查询，拥有 send-request consumer、Message Store 与 Outbox |

## 安全前置

跨主机、容器网段或监听 `0.0.0.0` 时必须启用 mTLS。明文 RPC 只允许 `127.0.0.1`、`::1` 或 `localhost`。

每个进程使用由同一内部 CA 签发的独立证书，证书 CN 必须等于调用身份（`dipole-gateway` 或 `dipole-message`），并包含 `serverAuth` 和 `clientAuth` 用途；服务端 DNS/IP 必须进入 SAN。通过环境变量注入：

```text
DIPOLE_INTERNAL_RPC_ENABLED=true
DIPOLE_INTERNAL_RPC_SHARED_SECRET=<service-secret>
DIPOLE_INTERNAL_RPC_TLS_ENABLED=true
DIPOLE_INTERNAL_RPC_TLS_CERT_FILE=/run/secrets/service.crt
DIPOLE_INTERNAL_RPC_TLS_KEY_FILE=/run/secrets/service.key
DIPOLE_INTERNAL_RPC_TLS_CA_FILE=/run/secrets/internal-ca.crt
DIPOLE_INTERNAL_RPC_TLS_SERVER_NAME=<peer-certificate-san>
```

Message 进程使用独立 MySQL 账号。以 [message-service-grants.dist.sql](configs/mysql/message-service-grants.dist.sql) 为模板创建账号，并设置：

```text
DIPOLE_MESSAGE_ENFORCE_DB_PERMISSIONS=true
```

启动验收要求账号可读取 Message/Sync/Outbox/migration 表，同时无法读取任何 Core 表。

## 渐进切换

1. 执行 `go run ./cmd/migrate -direction up`，确认全部节点 schema 版本一致。
2. 保持 Core `message.transport=local`，启用 Core RPC listener 并先启动 Core。
3. 以 `message.runtime_mode=shadow` 启动 `go run ./cmd/message-service`。
4. 在 Core 开启 `message.shadow_queries=true`，检查 `message shadow query mismatch` 日志和查询错误率；shadow 进程不会执行命令或后台写入。
5. 关闭 shadow 进程，以 `message.runtime_mode=owner` 和受限数据库账号重新启动。切换窗口内 Core 与 Message 使用相同 `kafka.client_id`，由同一 consumer group 保证 send-request 只交给一个实例。
6. 重启 Core，设置 `message.transport=grpc`；此时 Core 停止 Message persistence handler 和 Outbox，Message 成为唯一所有者。
7. 可暂时保留 `message.shadow_queries=true`，以 Remote 为 primary、Local 为查询 shadow；指标稳定后关闭 shadow。

## 验收

- 文本与文件私聊、群聊均只生成一条 server message 和一组 Outbox 事件。
- 重复 `client_message_id` 返回同一消息，目标身份冲突继续拒绝。
- 历史、群增量和离线查询的 shadow comparison 持续匹配。
- Core 停止后 Message RPC 的 Core Capability 调用按超时失败，不产生半写消息。
- Message 停止后 Core/Gateway 快速失败，切回 Local 无需数据回滚。
- SIGTERM 后 RPC 停止接流量，Kafka consumer、Outbox worker、连接池依次退出。

## 回滚

1. 保持 Message owner 在线并使用与 Core 相同的 Kafka consumer group。
2. 将 Core 改回 `message.transport=local` 并重启；Core 恢复 persistence handler 与 Outbox。
3. 验证 Local 发送、历史和 Outbox 后停止独立 Message owner。
4. 回滚期间保留数据库数据与 migration，不执行逆向数据迁移。

若问题仅发生在 Remote 查询，可先关闭 `message.shadow_queries`；若证书或网络配置失败，修复前保持 Local，不放宽明文非 loopback 限制。
