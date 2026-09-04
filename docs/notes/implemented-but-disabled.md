# 已实现但未启用

整理日期：2026-09-03。只列**代码已具备、默认关闭**的能力；未实现的规划项不在此列。
默认拓扑是 Agent Shadow + Temporal 关 + 全部写能力关 + Cassandra/Web Sync/预签名直传关。
共享环境启用前仍要各自的观察窗口证据，本表不改变默认路径。

## 和「缺页面 / 重复实现」的边界

2026-09-03 台账勘误：waiting Vue、产物收件箱、记忆候选晋升已在 HEAD。CHANGELOG 同日更早的「前端后续接入」行是当时切片边界，被后文覆盖。

本表列的是 **flag-off**，不是缺实现。Gateway `agent_*_enabled` 与前端 `VITE_*` 是两层门禁，同一能力各守一层。Go/Eino 在 `internal/services/agent/legacy/`，只有 embedded Kafka composition 可引用，这是回滚面，不是第二套产品。

Pencil 侧栏「审批记录」没有路由和 API，已从任务/记忆/定义/订阅四页控制轨删除。审批走 `/approval` 单次决定。

## 怎么读

- **可 opt-in**：compose overlay / 显式 flag 就能跑隔离栈，缺的是共享环境证据。
- **缺接线**：实现齐，但生产启动链、Activity 或公开入口还没挂上。
- **缺环境**：实现齐，卡在浏览器二进制、真实 MinIO/CORS、共享 Kafka 等。

## Agent（优先整理）

| 能力 | 默认关开关 | 状态 | 卡在哪 |
| --- | --- | --- | --- |
| Subscription Auto-Reply | `DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED`；overlay `agent-subscription-autoreply.yml` | 可 opt-in | AD-034 观察窗口。执行、Core 铸 grant、smoke、activity 重试幂等已齐 |
| Subscription Active Read | `DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ENABLED`；`agent-subscription-active.yml` | 可 opt-in | AD-034 / AD-009 共享 Kafka/Temporal 窗口 |
| Subscription Shadow | `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED`；`agent-subscription-shadow.yml` | 可 opt-in | AD-034 reviewed Shadow |
| Interactive `/send` | `DIPOLE_AGENT_INTERACTIVE_MESSAGE_WRITE_ENABLED`；`agent-interactive-active.yml` | 可 opt-in | AD-009 浏览器 HITL、shared tenant、联合故障 |
| Active Read | `DIPOLE_AGENT_RUNTIME_MODE=remote` + Temporal；`agent-active.yml` | 可 opt-in | AD-009 |
| Temporal Worker | `DIPOLE_AGENT_TEMPORAL_ENABLED` | 可 opt-in | 默认 `foundation`，只在 overlay 开 |
| Task Control + 交互创建 + owner 收件箱 | `DIPOLE_AGENT_CONTROL_ENABLED` / Gateway 同名；前端 `VITE_AGENT_TIMELINE_ENABLED` | 可 opt-in | AD-009。收件箱随 Control 装配，页面随 Timeline 开关 |
| 多会话 `wait_input` | 无独立开关，随 Temporal read | 可 opt-in | AD-009 E2E |
| Memory 观察写入 | `DIPOLE_AGENT_MEMORY_ENABLED` | 可 opt-in | AD-009 / AD-061 |
| Memory Promotion Commit | Core `agent_memory_promotion_receipt_commit_enabled` + Runtime commit flag | 可 opt-in | AD-009 联合 revoke/rollback |
| Retrieval + Context Compiler | `DIPOLE_AGENT_RETRIEVAL_ENABLED` / `RETRIEVAL_CONTEXT_ENABLED` | 可 opt-in | 生产 ES + reviewed Shadow |
| 一等 MCP Server | Runtime/Gateway `*_MCP_*_ENABLED` | 可 opt-in | AD-037 OAuth/凭据/告警 |
| External MCP / Shadow | `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED` | 可 opt-in；误开 fail closed | AD-037 生产 I/O、密钥、Dispatcher |
| Gateway Definition/Subscription/Artifact/Memory HTTP | `gateway.agent_*_enabled` | 可 opt-in | AD-036 / AD-044 |
| Timeline Repair Worker | Compose profile `agent-timeline-repair` | 可 opt-in | AD-046 operator 灰度 |
| Workflow Repair Execute/Rollback | `internal_rpc.agent_workflow_repair_execute_enabled`；overlay `agent-workflow-repair-execute.yml` | 可 opt-in | AD-009 operator grant / 共享环境演练。启动链已挂，默认关 + mTLS |
| MCP Elicitation continuation | `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=external_mcp_shadow`；overlay `agent-external-mcp-shadow.yml` | 可 opt-in | AD-036 多轮 / 敏感授权 / 共享环境。单轮 continuation 已在该 Worker 的 `executeMcpDispatch` |
| 第一方 Elicitation Timeline 入口 | `VITE_AGENT_ELICITATION_ENABLED` | 可 opt-in | 投影已写 `input_request`，Timeline 已链接；页面默认关 |
| OAuth callback consume | `agent_oauth_authorization_transaction_consume_enabled` | 可 opt-in | mTLS + callback + 密钥评审 |
| OAuth callback durable handoff（Runtime 侧 provider + token lifecycle） | 无独立开关：`services/agent-runtime/src/index.ts` 完全不装配，`OAuthCallbackRuntimeConfig` 默认 `enabled=false` 且 `assertOAuthCallbackRuntimeUnavailable` 会拒绝启用 | 缺接线（组合闭环已实现） | 需真实 provider adapter + Core-owned token lifecycle SQLC 表 + Runtime 密钥轮换/retention + Gateway callback route。回滚方式：删除 feature branch，主 checkout 未受影响 |

## 存储 / Sync / 文件

| 能力 | 默认 | 卡在哪 |
| --- | --- | --- |
| Cassandra 平台 / shadow 读 / 读灰度 / duplicate hydration | `cassandra.enabled=false`，百分比 `0` | 观察窗口 + 回切演练 |
| Sync Cassandra shadow/primary hydration | 默认 `false` | 双跑证据 |
| Inbox projector 接管 | `inbox_write_mode=atomic` | AD-048 |
| Web Sync Engine | `VITE_SYNC_ENGINE_MODE=off` | A6 24h 真客户端窗口 |
| Timeline notify shadow/primary | 默认 `off` | A6 |
| MinIO 预签名直传 | `multipart_mode=relay` | AD-055 真 MinIO/CORS |
| 预签名同域代理 | 默认 `false` | 故障矩阵 |

## 前端（全部 `VITE_*=false`）

Search、预签名、Sync、Elicitation、Approval、Timeline、Task Create、Artifacts、Subscriptions、Definitions、Memories、Memory correction。
页面与契约已在。走查用 `frontend/.env.agent-experience` / `npm run dev:agent`，默认生产路径不变。入口已挂到 Settings / Chat / 侧栏。按页缺口见 `docs/notes/agent-frontend-experience-gaps.md`。

## C++ 数据面（暂缓）

`DIPOLE_REALTIME_DELIVERY=go`。C++ primary、observation RPC、fencing 均默认关；microbenchmark 未过晋级门槛，不作为本轮整理对象。

## 本轮开发顺序

1. ~~Workflow Repair Execute~~：已挂到默认关的 Core 启动链。
2. ~~MCP Elicitation continuation~~：已随 `external_mcp_shadow` 进入生产 Worker；第一方 Timeline 入口已挂。
3. 可 opt-in 项只补门禁/证据，不改默认。AD-036 剩余多轮与敏感授权。
4. ~~Owner 任务收件箱~~：`GET /api/v1/agent/tasks` + Vue `/agent/tasks` 已齐，随 Task Control / Timeline 开关。
5. ~~Owner 记忆候选列表~~：Gateway API 已齐；记忆页晋升入口已挂，随 `VITE_AGENT_MEMORIES_ENABLED`。
6. ~~Waiting 任务 Chat 提示~~：`agent_task_waiting` 已在 Chat 去重并链到收件箱，重连以任务列表为准。
7. ~~产物收件箱页~~：`GET /api/v1/agent/artifacts` 已挂到 Vue `/agent/artifacts`，随 `VITE_AGENT_ARTIFACTS_ENABLED`。列表只展示 metadata，正文仍走既有 digest 详情页。
8. ~~Owner 记忆候选审核~~：`POST /api/v1/agent/memory-candidates/:id/review` + 记忆页通过/拒绝已齐，随 `VITE_AGENT_MEMORIES_ENABLED`。
