# Dipole Agent Runtime

TypeScript Agent 执行面。当前 G2 foundation 固定可信 `ExecutionContext`、Capability Registry、resource-scope Policy Engine、Go 兼容 Task ID、Kafka consumer 和 Fastify 健康面。微服务 Compose 默认运行受限的 `remote + read_active` Durable Runtime，启用 Temporal、认证 Task Control 与只读 `conversation.list/read`；消息写入、MCP、Memory、检索和订阅触发仍保持显式关闭。完整部署输入、回滚和证据边界见 [Agent Active 部署运行手册](../../docs/agent/AGENT-ACTIVE-DEPLOYMENT.md)。

```bash
npm ci
npm test
npm run typecheck
npm run build
npm run generate:sql
```

运行独立 Kafka shadow consumer：

```bash
DIPOLE_AGENT_KAFKA_ENABLED=true \
DIPOLE_AGENT_KAFKA_BROKERS=127.0.0.1:9092 \
DIPOLE_AGENT_KAFKA_GROUP_ID=dipole-agent-shadow-v1 \
DIPOLE_AGENT_KAFKA_TOPIC_PREFIX=dipole \
DIPOLE_AGENT_KAFKA_FAILURE_MAX_ATTEMPTS=3 \
DIPOLE_AGENT_LEDGER_MODE=mysql \
DIPOLE_AGENT_MODEL_MODE=metadata \
DIPOLE_AGENT_MYSQL_HOST=127.0.0.1 \
DIPOLE_AGENT_MYSQL_USER=dipole_agent \
DIPOLE_AGENT_MYSQL_PASSWORD=change-me \
DIPOLE_AGENT_MYSQL_DATABASE=dipole \
npm start
```

未设置微服务 Compose 环境变量时，Runtime 可作为默认关闭 Kafka、Temporal、MCP 和 Task Control 的独立 HTTP 进程启动：`GET /livez` 检查进程存活，`GET /readyz` 检查生命周期初始化是否完成。安全的本地 smoke 可使用 `DIPOLE_AGENT_HOST=127.0.0.1` 和独立端口执行，随后以 SIGINT 验证优雅退出。

Runtime 只接受 `message.direct.created` 的兼容 v1 envelope，并在 consumer 启动完成后开放 `/readyz`。上面的独立示例使用 `dipole-agent-shadow-*` group；微服务 Compose 使用独立的 `dipole-agent-active-primary-v1` group。默认物理 topic 为 `dipole.message.direct.created`，启动时创建并校验 main、`.retry`、`.dead` 的分区与副本配置。冷启动时 topic metadata 尚未收敛会执行有界重连，每次失败均断开旧 consumer。

## Subscription prefilter rollout gate

`src/evals/subscription-runtime-gate.ts` 提供可复用的 `off/shadow/enforced` 运行时门禁。`off` 保持现有确定性规则路径，`shadow` 允许任务创建并记录观察结果，`enforced` 仅接受与候选、配置、语料、评审及 evidence 精确绑定且状态为 `eligible` 的 rollout decision。非 `off` 模式遇到哈希漂移或证据不完整会 fail closed。`buildKafkaShadowRuntime` 可注入该 gate，blocked 结果会在订阅匹配和 EventLedger claim 前停止；默认未注入，真实 reviewed corpus 和共享环境灰度完成后再启用。

## MCP G4 authenticated mount

MCP Server 网络入口默认关闭。受控环境需要同时启用 Runtime 与 Gateway：

```bash
DIPOLE_AGENT_MCP_SERVER_ENABLED=true
DIPOLE_AGENT_MCP_RESOURCE=https://dipole.example.com/api/v1/agent/mcp
DIPOLE_AGENT_MCP_TOOL_TIMEOUT_MS=5000
DIPOLE_GATEWAY_AGENT_MCP_ENABLED=true
```

开发期可叠加 `deploy/microservices/agent-mcp-server-shadow.yml`，它只开启
第一方只读 MCP Server 和 Gateway 代理，保持外部 MCP、Task Control、Memory、
retrieval、Artifact 与消息写入关闭：

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-mcp-server-shadow.yml \
  config --quiet
```

客户端访问 `/api/v1/agent/tasks/{task_id}/runs/{run_id}/mcp`。Gateway 支持 Streamable HTTP 的 GET/POST/DELETE，先验证现有 JWT，再以内部服务身份调用 Runtime；Runtime 通过 Core `ResolveMcpContext` 复核 Task owner、运行中的 Run、固定 Definition、权限和 scope。当前只注册 `dipole_conversation_list`，不开启外部 Server、write/destructive Tool。

启用入口前先应用 migration v30 并滚动 Core，再滚动 Runtime/Gateway。每次 Tool 调用必须先通过 Core 持久化 begin；审计只记录参数/结果 SHA-256、结果大小、耗时、终态和稳定错误码。`@opentelemetry/api` 会创建低敏 ToolCall span；Exporter 总开关默认关闭。回滚先关闭 Gateway 开关，再关闭 Runtime 开关；确认没有 `running` 调用后才可执行 v30 down，降级会删除 Tool 审计历史。

Gateway 对 MCP GET/POST 使用 Redis principal 额度，默认每 60 秒 60 次，可通过 `DIPOLE_RATE_LIMIT_AGENT_MCP_LIMIT` 和 `DIPOLE_RATE_LIMIT_AGENT_MCP_WINDOW_SECONDS` 调整。该门禁独立于旧 `DIPOLE_RATE_LIMIT_ENABLED`；Redis 不可用或值小于等于零时返回 429/`Retry-After`。DELETE 始终允许认证用户清理 Session。需要紧急回滚时关闭 `DIPOLE_GATEWAY_AGENT_MCP_ENABLED`，不要通过关闭限流开放无界入口。

外部 MCP Client foundation 对每个 allowlisted Tool 强制配置 egress policy：策略必须与 Tool allowlist 完全匹配，声明允许的顶层参数名和 2 B 到 64 KiB 的请求上限；调用前先规范化为 JSON，拒绝未声明参数、超过 16 层的对象以及 password、token、authorization、cookie、credential、private key 等常见凭据字段。该门禁用于阻止常见误传和无界请求；内容值级 DLP、租户凭据托管与完整 egress 审计完成前，外部 Server 继续保持未启用。

## OpenTelemetry

Agent trace exporter 默认关闭。受控环境可显式启用 OTLP/HTTP protobuf，并使用标准 `OTEL_*` 参数：

```bash
DIPOLE_AGENT_OTEL_ENABLED=true \
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://127.0.0.1:4318/v1/traces \
OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=http/protobuf \
OTEL_TRACES_SAMPLER=parentbased_traceidratio \
OTEL_TRACES_SAMPLER_ARG=0.1 \
npm start
```

Runtime 拒绝非 HTTP endpoint、嵌入 URL 的凭据、非 protobuf 协议和越界采样/超时。SDK 在业务 Runtime 创建前注册，并在进程逆序关闭的最后阶段 flush；span 仅包含设计文档列出的低敏属性。默认关闭的 `observability` profile 提供 24 小时 local Tempo、Collector 指标和 Prometheus 告警，配置与真实 trace 可分别通过 `scripts/check-agent-otel-observability.sh`、`scripts/smoke-agent-otel.sh` 验证。完整启用、联查与回滚步骤见 `docs/agent/agent-otel-operations.md`；生产对象存储和通知链仍由 `AD-037` 跟踪。

触发模式默认是 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`。应用 migration v28 并通过受控 Core Store 配置有效订阅后，可显式设置 `subscription`：Runtime 先经受认证 Capability RPC 获取 Definition/resource scope 授权后的候选，再以 `all` 或 `message_contains_any` 做本地确定性过滤。零匹配不会领取 EventLedger、启动 Temporal 或调用模型；匹配 Task 固定稳定排序后的 Subscription ID。Gateway/Vue 已提供默认关闭的 owner list/create/revoke 管理入口；共享环境仍保持 `direct_target`，Runtime subscription 消费和语义预筛需先通过独立证据门禁。

## Temporal runtime modes

微服务 Compose 默认启用 Temporal，并以 `read_active` 运行受控只读 Task。下面的 `foundation` 配置只用于独立进程或本地恢复诊断，不接管 Kafka 流量：

```bash
DIPOLE_AGENT_TEMPORAL_ENABLED=true \
DIPOLE_AGENT_TEMPORAL_ADDRESS=127.0.0.1:7233 \
DIPOLE_AGENT_TEMPORAL_NAMESPACE=default \
DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-task-v1 \
DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=foundation \
npm start
```

Workflow ID 固定为 `dipole-agent-task/{task_id}`。运行中重复启动复用现有 Workflow，终态 Task 拒绝重复启动。模型调用、Capability RPC、持久化和副作用重试均须通过 Activity；foundation 的 Step Activity 只返回受控失败，用于验证 Worker 部署、恢复和运维边界。

显式设置 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=persistent_shadow` 后，Worker 使用既有 Agent Capability RPC 执行 Task/Run admission，并在 Workflow 终止前精确提交 completed、failed 或 cancelled Run。`wait_approval` 会先持久化 capability/scope/arguments/nonce 绑定；只有 request/approval ID 匹配且 Core 确认 actor 为 Task principal 的 Signal 才能完成 approved/revoked 并恢复 Workflow。该模式要求 `DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true` 及对应 target、共享密钥或 mTLS 配置。Workflow starter 和未来 Signal bridge 必须来自可信认证入口，模型无权设置 principal。当前 Kafka consumer 不启动 Workflow，模型、Capability Step 和权威 Task 状态继续由既有路径持有。

显式设置 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=read_shadow` 后，Kafka consumer 只负责 EventLedger claim 和稳定 Workflow 启动，ContextCompiler、ModelRouter、Plan/Step 持久化与 `conversation.list/read` 在 Temporal Activity 中执行。该模式同时要求 migration v26、`LEDGER_MODE=mysql`、`MODEL_MODE=ai_sdk`、模型 routes、Capability RPC、Core MinIO 和 Temporal；Task、Run、admission 与原始事件必须精确绑定。成功模型输出写入 `agent_model_calls.output_json`，随后经 Core 创建版本化 `conversation_digest` Artifact；Activity 重试先恢复模型与已完成 Step，并复用和复核同一内容寻址对象。Shadow 保留 `agent_tasks.status` 的策略生命周期，受 CAS 保护的 `workflow_status` 与 Run 共同表示本次 Durable 执行终态；评测只能接受两者中的有效终态记录，不能以仍在运行的策略 Task 伪造结果。`read_shadow` 是显式回退与测试 profile；主 Compose 保持 Temporal enabled + `read_active`。

`DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=external_mcp_shadow` 是外部 MCP 的独占常驻模式。它要求 external Profile、Temporal、Kafka、subscription trigger 与 Capability RPC 全部显式启用，并加载受约束 I/O/deployment route manifests；入口会跳过旧 Kafka runtime 和旧 Temporal Worker，使用统一 process 按 Worker/Client/Kafka 启动、Kafka/Client/Worker/Core 停止。Compose 默认不启用该模式。回滚先关闭 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED`，并将 activity mode 恢复 `foundation`；任何真实外部连接前仍要求 fresh readiness evidence。

Active Runtime 要求 `DIPOLE_AGENT_RUNTIME_MODE=remote`、`DIPOLE_AGENT_CANDIDATE_VERSION` 和 `DIPOLE_AGENT_RELEASE_MANIFEST`。微服务 Compose 默认提供 `remote + read_active` 所需的 Temporal、认证 Capability RPC、独立 `DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID`、OpenAI-compatible Provider 输入与 v2 Context profile；`deploy/microservices/agent-active.yml` 只保留旧部署命令兼容。任一必填输入缺失都会在 Compose 渲染时拒绝。启动时会读取 release manifest，只有 candidate 一致且 manifest 已推进到 `user_gray` 才允许进入 `read_active`；`offline`/`shadow` 清单会 fail closed。回滚时加载 `deploy/microservices/agent-temporal-read-shadow.yml` 或停止该 Runtime，release manifest 不会被启动过程改写。

Agent 镜像使用 Node 22 Bookworm slim。Temporal Native Core 发布为 GNU libc 二进制，Alpine/musl 镜像无法启用 Worker。

真实 Temporal dev server 契约默认不进入快速测试，可显式运行：

```bash
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm test -- --run src/temporal/agent-task-workflow.integration.test.ts
```

Worker 与 Core RPC 在线时可执行只读 Workflow projection 对账：

```bash
DIPOLE_AGENT_TEMPORAL_ENABLED=true \
DIPOLE_AGENT_TEMPORAL_ADDRESS=127.0.0.1:7233 \
DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true \
DIPOLE_AGENT_CAPABILITY_RPC_TARGET=127.0.0.1:9090 \
DIPOLE_INTERNAL_RPC_SHARED_SECRET=change-me \
npm run reconcile:projection -- --page-size=100 --max-examples=100
```

命令输出 `dipole.agent.projection-reconcile.v1` JSON。全部 Task 为 `match` 时退出 0，发现 missing/stale/ahead/conflict/unavailable 时退出 2。它通过 Core 私有 RPC 分页读取固定 shadow cohort，只读 Temporal Query/Describe，不修改 Task、Run 或 Workflow。

将同一候选版本的对账观察和 Eval 结果整理为 `dipole.agent.shadow-promotion-evidence.v1` 后，可执行：

```bash
npm run promotion:check -- --evidence=/path/to/evidence.json
```

策略要求连续 24 小时、至少 24 个观察点、最大间隔 90 分钟、累计至少 100 个 Task、零 projection 异常与 unavailable，并要求 projection/outcome/trajectory/permission Eval 全通过。eligible 只用于人工评审，命令不修改配置或运行时权威。

G4 五类 deterministic 离线评测使用语言中立 Suite：

```bash
npm run eval:offline -- --suite=../../contracts/agent-evals/v1/offline-suite.json
```

报告绑定 candidate version 与 canonical Suite SHA-256，按 outcome、trajectory、permission、retrieval、cost 输出低敏结果。合法且全部通过返回 0，合法但有失败返回 2，输入错误返回 1。新候选应把完整报告写入 `dipole.agent.shadow-promotion-evidence.v2`；`promotion:check` 自动分派 v1/v2，v2 要求五类报告全部通过。当前样例属于 synthetic Harness 证据，不能代表真实 Agent 效果。

真实 Shadow Run 与其对应的 Durable Task 执行均已终态，且 `promotion:check` 返回 eligible 后，可按 `contracts/agent-promotion/v2/publication.schema.json` 准备发布输入并执行：

```bash
DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true \
DIPOLE_AGENT_CAPABILITY_RPC_TARGET=127.0.0.1:9090 \
DIPOLE_INTERNAL_RPC_SHARED_SECRET=change-me \
npm run promotion:publish -- --input=/path/to/publication.json
```

命令复用 Agent Runtime 的受认证 Capability RPC/mTLS 配置，将完整证据和决策写入 content-addressed `promotion_evaluation` Artifact，只向标准输出返回 `contracts/agent-promotion/v2/receipt.schema.json` 定义的低敏收据。普通 Artifact 仍要求 running Shadow Run；该类型只能在 completed Shadow Run 上首次发布。收据可供 Gateway 控制面提案引用，发布本身不会创建 Proposal、Grant、active Run 或注册 write Tool。提案创建后，已获 tenant-scoped operator Grant 的 reviewer 可通过 Gateway-only `GetRuntimePromotionEvidence` 读取 Proposal 精确绑定且重新验哈希的 Artifact 正文；该方法未挂载公共 HTTP，也不复用普通 Task-principal 下载授权。

结构性安全回归位于 `src/evals/agent-security-regression.test.ts`，使用 `contracts/agent-evals/v1/security-suite.json` 串联真实 ContextCompiler、Capability Registry、EventLedger/Shadow Processor 和 MCP Client/Server。测试要求 Prompt Injection 内容保留 `untrusted` provenance、越权和敏感外发在副作用前拒绝、重复事件只规划一次、同源循环在 Ledger 前抑制。

确认 Temporal 证据后，操作员可生成短时效修复候选 Artifact：

```bash
npm run repair:propose -- --input=/path/to/repair-input.json
```

提案绑定 Task、操作员声明、工单、原因、投影/Temporal 证据和一小时有效期，并生成稳定 SHA-256。当前没有 apply 命令，提案也尚未经过服务端签名和持久审批，不能作为已授权修复。

Shadow 模式仅生成并审计 plan，Policy Engine 拒绝 write/destructive capability。微服务默认使用 MySQL EventLedger，通过 Event ID/Task ID 唯一约束、claim token 与 lease 收敛重启和多副本重复投递；`memory` 只用于显式本地回滚。无效事件直接进入 dead，瞬时处理错误按 `retry_attempt` 有界重试；转移发布失败会让 handler 拒绝完成。migration v20 将 Plan 保存为不可变 Task 快照，并按顺序保存处于 `planned` 状态的结构化 capability Step；远程只读执行与 Step 终态将在 Agent Capability RPC 接入后启用。

模型调用默认关闭。显式开启 AI SDK shadow planner 时，必须配置单一 OpenAI-compatible Provider、有序 route 与预算：

```bash
DIPOLE_AGENT_MODEL_MODE=ai_sdk \
DIPOLE_AGENT_MODEL_PROVIDER=openai_compatible \
DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.com/v1 \
DIPOLE_AGENT_MODEL_API_KEY=... \
DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS=false \
DIPOLE_AGENT_MODEL_OUTPUT_MODE=json_schema \
DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini,openai/gpt-5-nano \
DIPOLE_AGENT_CONTEXT_COMPILER_VERSION=v2 \
DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
DIPOLE_AGENT_MODEL_MAX_CALLS=2 \
DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS=15000 \
DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS=512 \
npm start
```

Provider name 是 route 的稳定前缀，所有 route 必须使用相同前缀，例如 `openai/<model-id>`；Runtime 拒绝跨 Provider route、空密钥、无效 Provider name 和包含凭据/query/fragment 的 base URL，HTTP 仅允许 loopback 开发端点。密钥只从进程环境或部署 Secret 注入，不写入 Compose、Artifact、审计或日志。

`DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS` 默认 `false`。只有 Provider 已验证支持 OpenAI JSON Schema response format 时才设为 `true`；该声明决定 AI SDK 是否为 Zod plan schema 请求结构化输出，避免向通用兼容网关发送不支持的字段。

`DIPOLE_AGENT_MODEL_OUTPUT_MODE` 默认 `json_schema`。Provider 不支持该 response format 时可显式设置 `json_text`：Runtime 在同一次、无内部重试的调用中要求纯 JSON，再用同一份 Zod schema 在本地验证。无效 JSON 或 schema 不匹配照常使该次调用失败并由既有 ModelRouter 记录与预算控制。

`deploy/microservices/agent-ai-sdk-shadow.yml` 是独立的 Shadow/测试 overlay。主 Compose 已固定 `ai_sdk` 与 `openai_compatible`，Provider、预算与 Context 输入由受忽略的 `.env` 提供；移除该 overlay 后仍回到默认的只读 `remote + read_active` Runtime。

Runtime 按 route 顺序降级，失败调用同样消耗 `MAX_CALLS`；AI SDK 内部 retry 固定为 0。模型输出经过 Zod 校验，只能规划显式允许的只读 capability，并输出有序 `steps[]`。`ai_sdk` 模式强制使用 MySQL：ModelRouter 在每次 provider 调用前通过 ModelAuditStore 预留 Task slot，持久化 route、attempt、input/output Token、结构化输出、latency 与终态；Kafka 或 Temporal 重投不能刷新预算。

`DIPOLE_AGENT_RETRIEVAL_ENABLED` 默认 `false`。仅在 `MODEL_MODE=ai_sdk` 且 Capability RPC 已启用时，Runtime 才向模型公开并注册 `conversation.search`；每次调用仍由 Core 从 Task/Run 恢复 principal，复核独立 permission、`conversation/*/read` scope、query/结果/正文上限，并将命中作为有界 `untrusted` evidence。关闭该开关时，模型、Registry 和 Shadow 执行 Context 都只包含 `conversation.list/read`。

`DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED` 也默认 `false`，且要求先开启 retrieval。启用后 Planner 仅从当前事件的 `payload.content` 提取最多 256 个 Unicode 字符作为查询，经 Core 受权检索最多 8 条结果，并在模型调用前按 Context budget 编译为带 `messageId`、conversation、sequence 和 query hash provenance 的 `untrusted` evidence。检索失败会阻断本次模型调用；缺少正文、关闭开关或预算不足时保持既有路径。该配置不启用 Elasticsearch、跨会话检索、共享环境流量或生产默认路径。

独立进程的 `CONTEXT_COMPILER_VERSION` 默认 `v1`，微服务 Compose 固定为 `v2`。切换候选前应等待旧候选 Task 收敛或使用新的 Task cohort；回滚到 `v1` 仅适用于显式 Shadow/本地 profile。

v2 的 `MODEL_CONTEXT_PROFILES` 是可选严格 JSON 数组。每个 route 可声明 `contextWindowTokens`、`utf8BytesPerToken` 与 `safetyMarginBps`；Runtime 对全部候选 route 取最大估算和最小窗口，确保同一 prompt 可安全进入后续 fallback。未声明 route 固定采用 8192 Token 窗口、2 UTF-8 bytes/token 与 25% 余量，并把最终 profile 集合的 SHA-256 estimator ID 写入 Context manifest。v1 拒绝 profile，v2 中任一 profile 引用未知 route，或最小窗口容纳不了 4096 输入预算与 `MAX_OUTPUT_TOKENS` 时，配置解析直接失败。

`src/context/token-estimator.test.ts` 中的中英文、代码、Emoji 与 Tool schema 语料用于确定性回归，只代表工程夹具。生产 profile 需要使用对应 route 的可复现 tokenizer 或 provider usage 单独校准；Runtime 不会根据单次调用结果自动调整估算值。

离线校准证据遵循 `contracts/agent-context-calibration/v1`。evidence 必须标记 `dataClassification=synthetic`，每个 route 都要包含中英文、代码、Emoji 与 Tool schema 五类 case，并记录 reference Token 的 provider、model 和 tokenizer/usage revision。执行：

```bash
npm run context:calibrate -- --evidence=../../contracts/agent-context-calibration/v1/examples/eligible-evidence.json
```

命令不访问 provider 或网络。报告不回显 case 正文，只保存正文 SHA-256、UTF-8 字节数、逐 route 估算误差、fallback route 和双重证据哈希；零低估且所有 route 均有显式 profile 时退出 0，合法但不足的校准退出 2，输入错误退出 1。`eligible` 只用于候选评审，不能修改 Runtime 配置或自动启用 Compiler v2。

微服务环境使用根目录 `deploy/compose/docker-compose.microservices.yml` 的 `agent` 服务；容器固定 Node 22，默认启用 Kafka、Agent 自有 MySQL ledger、认证 Capability RPC 与 Temporal。只读范围之外的 Capability 仍需单独的 profile、凭据和验收。
