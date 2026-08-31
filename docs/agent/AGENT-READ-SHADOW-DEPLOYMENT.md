# Agent Read Shadow 部署运行手册

`read_shadow` 将 Kafka 触发的 Agent Task 交给 Temporal 执行。它只允许
`conversation.list/read` 和版本化 `conversation_digest` Artifact；消息发送、
Memory 写入、检索、MCP、Control API 与 OAuth callback 均保持关闭。

## 启动

开发环境必须同时加载 AI SDK Shadow 与 Temporal overlay：

```bash
docker compose --env-file .env -p dipole-dev \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-ai-sdk-shadow.yml \
  -f deploy/microservices/agent-temporal-read-shadow.yml up -d --build
```

`.env` 负责 Provider 的 base URL、API key、route、预算和 Context profile。它不
应提交到仓库。Temporal gRPC 只通过 Compose 网络的 `temporal:7233` 提供，overlay
没有公开端口。

## 验证

```bash
docker compose -p dipole-dev ps
docker compose -p dipole-dev exec agent \
  node -e 'fetch("http://127.0.0.1:8091/readyz").then(async r => { console.log(r.status, await r.text()); process.exit(r.ok ? 0 : 1) })'
```

发送一条指向受控 Agent 的私聊后，应确认 Task/Run、`agent_model_calls`、Shadow
Plan 与 `conversation_digest` Artifact 具有同一 Task/Run 绑定。只读 Shadow 出现
失败时，先保留数据库和 Temporal evidence，再移除本 overlay；基础 Compose 会回到
`foundation` Temporal disabled 的 Shadow Planner。

## 回滚

停止并移除 overlay 启动的 Temporal 服务，再用基础 Compose 加 AI SDK Shadow overlay
重新启动 Agent。不要在没有运行记录的情况下删除 `agent_temporal_postgresql_data`。
