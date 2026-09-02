# Dipole Agent Runtime 设计

本文档定义 Dipole 的 Agent 产品边界、TypeScript 技术栈、核心抽象和渐进迁移路线。实现状态以 [平台演进计划](PLATFORM-EVOLUTION-PLAN.md) 为准，发布变化记录在 [更新日志](../../CHANGELOG.md)。

## 1. 产品定位

Dipole Agent Runtime 是建立在 IM Event Bus 与业务 Capability 之上的事件驱动、可持久化执行平台。首个产品形态是 IM-native Project Guardian：持续观察指定会话，提取决策、任务和风险，在需要输入或高风险动作时等待用户确认。

运行形态分为三类：

- Interactive Agent：响应用户指令，检索会话并生成回复或 Artifact。
- Event-driven Agent：订阅消息、文件、成员变更等领域事件，按规则和语义过滤触发任务。
- Durable Task Agent：支持长时间运行、失败恢复、等待输入、等待审批、取消和继续执行。

Agent 只能通过版本化 Capability API 读取或修改 IM 数据，禁止直接访问 Message Store、Sync Store 和业务 Repository。

Run admission、completion 和 failure 使用同一组 `runtime_id + mode` 绑定。TS RPC client 默认使用 `shadow`，显式 active client 必须携带 candidate version；Core 继续执行 promotion grant、Definition、Task/Run 和权限复核，旧省略字段的调用保留 shadow 兼容窗口。

active 过渡阶段只注册 `read_active` Temporal Activity。Activity 会以同一 Task/Run/Event 绑定向 Core 解析权威 ExecutionContext，再执行 `conversation.list/read`；Artifact、消息发送和其他写 Capability 不会因为 active mode 自动开放。

## 2. 语言与技术栈

| 层 | 选择 | 职责 |
| --- | --- | --- |
| Language / Runtime | TypeScript + Node.js | Agent Runtime 与集成层 |
| HTTP | Fastify | 健康检查、管理 API、Webhook |
| Internal RPC | Connect 或 gRPC | 调用 Go Capability API |
| Event | Kafka | IM 事件触发和执行结果事件 |
| Agent primitives | Vercel AI SDK | 模型适配、流式输出、结构化输出、Tool Calling |
| Schema | Zod + JSON Schema | Capability 输入输出契约 |
| Durable execution | Temporal TypeScript SDK | Workflow、Signal、Retry、Timer 和恢复 |
| Tool protocol | MCP TypeScript SDK | 外部 Tool 接入与 Dipole 能力开放 |
| Metadata | MySQL | Agent、Task、Run、Approval 和 Artifact 元数据 |
| Ephemeral state | Redis | 限流、短期缓存和分布式协调 |
| Retrieval | Elasticsearch | 消息混合检索和语义召回 |
| Artifact | MinIO | 报告、结构化结果和大对象 |
| Observability | OpenTelemetry + Prometheus | Trace、Metric、审计和成本 |
| Test / Eval | Vitest + 自研 Eval Harness | 单测、轨迹、结果、权限和成本评测 |

Mastra、OpenAI Agents SDK 和 LangGraph.js 用作设计参考或可插拔 adapter。Runtime 的身份、权限、状态机、上下文、事件触发和评测由 Dipole 自己维护，避免核心语义被单一框架绑定。

## 3. 总体架构

```text
IM Gateway / Core ── gRPC/Connect ──► Capability API
        │                                  ▲
        └──────── Kafka Events ────────────┤
                                           │
                                 ┌─────────┴──────────┐
                                 │ TS Agent Runtime   │
                                 ├────────────────────┤
                                 │ Trigger Engine     │
                                 │ Agent Kernel       │
                                 │ Context Compiler   │
                                 │ Capability Registry│
                                 │ Memory Policy      │
                                 └─────────┬──────────┘
                                           │
                         ┌─────────────────┼───────────────┐
                         ▼                 ▼               ▼
                     Temporal          Model API        MCP Servers
                         │
                         ▼
                   Task / Approval / Artifact
```

Kafka 承担领域事件传播，Temporal 承担一次 Agent Task 的可靠执行。MCP Task 仅作为未来对外协议能力，不替代内部 Workflow 状态。

## 4. 核心抽象

### ExecutionContext

`ExecutionContext` 由认证与触发链生成，模型不能设置主体身份和权限。

```ts
type ExecutionContext = {
  principalUserId: string;
  agentId: string;
  taskId: string;
  conversationId?: string;
  capabilities: ReadonlySet<string>;
  traceId: string;
  causationId: string;
};
```

### Capability Registry

Capability 是受控业务能力，Tool、MCP Tool 和 Agent-as-Tool 都是它的 adapter。

```ts
interface AgentCapability<I, O> {
  id: string;
  inputSchema: z.ZodType<I>;
  outputSchema: z.ZodType<O>;
  risk: "read" | "write" | "destructive";
  requiredPermissions: readonly string[];
  execute(input: I, context: ExecutionContext): Promise<O>;
}
```

Policy Engine 在执行前完成授权、预算、限流、审批和审计。写操作携带幂等键；破坏性操作默认要求人工审批。

G1 已实现 `AgentPolicyV1`：Invocation 携带 tenant、principal、Agent、delegator、permissions、resource scopes、approved capabilities 与 correlation IDs；descriptor 固定 capability ID、`read|write|destructive`、required permission 和 approval flag。Embedded Tool 先快速拒绝，本地 Capability/Command adapter 再按 `resource_type/resource_id/action` 执行同一策略，远程 server 必须复用该授权函数或等价 contract。Definition grant、Task snapshot 与 Approval 已通过 v16 Store 持久化，`AD-027` 已关闭。

G4 MCP foundation 使用官方拆分版 TypeScript SDK v2。Server 只将显式映射且 descriptor 为 `read` 的 Capability 注册为 Tool，Tool handler 复用 Capability Registry 与 Policy Engine；可信 ExecutionContext 由宿主根据已验证认证信息构建，Tool arguments 不包含身份。Client 在连接前校验 Server/Tool allowlist，握手后复核实际 Server identity，并限制发现数量和响应大小。

首个网络挂载由两个独立开关控制且默认关闭。Gateway 在 JWT 认证后将 principal、Task、Run 和 correlation 注入私有 Runtime 路由，移除公开凭据与可伪造内部头；Runtime 使用 `ResolveMcpContext` 让 Core 校验精确 Task/Run、固定 Definition、grant、scope 和 approvals，再为 `conversation.list` 创建请求级 MCP Server。Streamable HTTP 的 GET/POST/DELETE、SSE body 与 Session header 透传。

每次 Tool 调用先经 additive Core RPC 写入 migration v30 的 `agent_tool_invocations`，只有 durable begin 成功后才执行 Capability；完成或失败只能从 `running` 转换一次。审计只保存哈希、大小、耗时和稳定错误码，不保存参数、结果或内部异常正文。TS Runtime 同时通过 OpenTelemetry API 创建 ToolCall span；当前不装配 SDK/exporter，部署方启用观测后仍需遵守正文禁入 span 的约束。

Gateway 在 JWT principal 解析后、Runtime 代理前执行 MCP 专用 Redis 固定窗口限流。GET/POST 使用 `rate:agent_mcp:{principal}`，因此更换 Task、Run、方法或 Gateway 副本不会获得新额度；DELETE 不计数，确保超限后仍能释放 Streamable HTTP Session。该安全限流独立于旧 `rate_limit.enabled` 开关，Redis 不可用、额度或窗口非法时返回 429 并给出 `Retry-After`。第一方调用需先以 session 对 canonical resource 和只读 scope 显式 consent，取得 15 分钟 `aud/scope/token_use` 绑定的 MCP JWT；Gateway 与 Runtime 双重验证且不透传客户端令牌。Tool invocation 使用有界 timeout、cooperative AbortSignal 和稳定 `tool_timeout` 审计，外部 Client foundation 对 connect/list/call 使用 request/total timeout；Gateway 不设置会破坏 SSE 的统一代理超时，断连信号由 Runtime 继续传播。通用 OAuth 2.1 discovery/PKCE、外部 Server 凭据、生产 trace、write Tool 与 Elicitation adapter 继续由 `AD-037` 跟踪。

### Agent Task

```text
CREATED → RUNNING → WAITING_INPUT / WAITING_APPROVAL
                    ↓                 ↓
                 RUNNING ←────────────┘
                    ↓
          COMPLETED / FAILED / CANCELLED
```

Task 包含多个 Run，Run 包含 ContextCompile、ModelCall、ToolCall、Approval 和 ArtifactCreate 等 Step。Temporal Workflow ID 使用 Task ID，确保重复 Kafka 事件不会创建重复任务。交互式创建使用 `POST /api/v1/agent/tasks`，请求仅包含 `client_request_id` 与 `goal`；Gateway 从 JWT 派生 principal，再调用 Runtime 私有 `POST /internal/v1/agent/tasks`。Runtime 固定 tenant/Agent identity，并以 `client_request_id` 派生确定性 Task/Event ID。认证 Vue 入口为 `/agent/tasks/new`，需要 `VITE_AGENT_TASK_CREATE_ENABLED=true` 与 `VITE_AGENT_TIMELINE_ENABLED=true` 同时成立，并且仅在验证 `{taskId,status:"accepted"}` 后跳转只读 Timeline。该路由依赖既有 `gateway.agent_control_enabled` 与 `DIPOLE_AGENT_CONTROL_ENABLED` 双侧显式开启，基础 Compose 默认不注册它；请求体携带的身份字段不参与授权。

### Context Compiler

Context Compiler 根据 token 预算组合系统策略、Agent 身份、任务状态、相关会话、检索结果、Memory 和可用 Tool。它负责检索、裁剪、摘要、证据引用和预算分配，避免固定截取最近消息。

G2 已落地框架中立 v1：每个 fragment 固定 section、trust、priority、required、full/compact representation 与 provenance；编译器按语义 section 和稳定 ID 排序，在全局及 section 预算内选择内容。必需 fragment 无可用 representation 时 fail closed，不可信事件始终作为 JSON data record。migration v22 在 `agent_shadow_plans` 保存 compiler version、估算 Token 及 selected/omitted provenance manifest，Plan hash同时绑定编译结果；上下文正文不额外复制到审计列。当前 UTF-8 byte/4 估算限制由 `AD-031` 跟踪。

### Memory Policy

- Working Memory：当前任务计划、临时事实和执行进度。
- Episodic Memory：已完成任务及其结论、证据和反馈。
- Semantic Memory：项目、用户和资源的稳定事实。
- Procedural Memory：可复用工作流与 Skill。
- Observational Memory：将持续消息流压缩为 observation 和 reflection，降低长会话上下文衰减。

每次记忆写入需要来源、作用域、版本、置信度和过期策略；用户可查看、纠正和撤销长期记忆，物理删除需服从审计与隐私保留策略。

G3 v1 使用 migration v29 建立读取基础：`agent_memories` 按 tenant、principal、Agent 和精确 conversation scope 保存五类不可变记录、full/compact representation、priority、有效期与 provenance。Runtime 只提交 Task/Run 和资源，Core 从运行中的固定 Definition 解析身份、`conversation.read` permission 与 read scope；模型无法指定 principal。Task 创建时间固定可见记录上界，避免重试吸收后续新增记忆；撤销和过期立即移除，已存在的不可变 Plan 因此会在漂移时 fail closed。migration v38 进一步保存 revoker、撤销原因和时间；Gateway-only additive RPC 从认证 RequestContext 派生 owner，稳定分页和 owner-scoped revoke 均由 Core 二次约束，公开响应不携带内部 provenance URI。canonical Pencil 与默认关闭的 Vue 页面展示 `UNTRUSTED MEMORY`、来源和六类生命周期状态，并以权威撤销响应更新记录。v39-v42 依次增加 append-only correction、root 内容擦除、Task 直接 lineage 和模型前归因；derived-retention v1 将低敏影响报告转换为七域离线策略决策，固定无内容读取、删除执行或 Runtime 权威。`DIPOLE_AGENT_MEMORY_ENABLED` 默认与 Compose 固定为 `false`，受控 Shadow 显式启用后，Context Compiler 仅在命中记录时使用 Memory 独立预算，并将内容统一标记为 `untrusted` 数据。字段级派生擦除器、有界历史回填、自动写入、Observation/Reflection 压缩和 hybrid/vector retrieval 由 `AD-035` 跟踪。
历史 lineage 回填使用独立的 `dipole.agent.memory-lineage-backfill-manifest.v1` 与 receipt v1：游标绑定 `agent_shadow_plans.id` 的固定 high-water mark，批次和结果只保留低敏计数/hash。Go runner 要求 source ID 单调、引用身份与 representation 合法，目标写入成功后才推进 checkpoint，并允许 duplicate 精确重放；v43 MySQL checkpoint/source/target 接线完成前，该 runner 不由生产启动链调用。

## 5. Event Trigger

`AgentSubscription` 描述 Agent、资源范围、事件类型、过滤策略和 Capability 授权。事件先经过规则、小模型或向量召回做低成本筛选，相关事件才创建 Durable Task。

G3 v1 使用 migration v28 的 `agent_event_subscriptions` 保存 Subscription 与精确 Definition version。Core 通过受认证的 additive RPC 按 tenant、Agent、event type 和 conversation resource 查询候选，并重新校验 Definition 有效期、撤销状态、`conversation.read` permission 和 read scope。G4 migration v34 增加 creator/revoker、撤销原因和更新时间；Gateway-only 控制 RPC 从认证 principal 派生 owner，创建时再次复核固定 Definition 和 scope，并将大小写无关关键词集合规范化为稳定 SHA-256 Subscription ID。等价创建与同原因撤销可重放，payload 或撤销原因漂移冲突。migration v58 将 Definition 的版本唯一性收敛到 `(tenant, owner, agent, version)`；订阅执行按创建者查询 Definition，因此多位用户可为同一 Agent 保存相同版本号且互不覆盖。认证 `POST /api/v1/agent/definitions` 现生成固定 Assistant、固定 `conversation.read` wildcard 的 owner Definition，客户端无法提供 tenant、owner、Agent 或写权限；重复创建返回相同记录。TS Runtime 的 `DIPOLE_AGENT_TRIGGER_MODE=subscription` 只执行严格 `all|message_contains_any` 规则，零匹配时在 EventLedger、Temporal 和模型之前返回；每个匹配按 Subscription ID 排序并独立派发，Task 与 EventLedger key 都绑定 Subscription identity。默认 `direct_target` 保留既有行为。公开 Definition 创建、目录和 Subscription 页面需要 Gateway Subscription 控制开关；当前仅嵌入式 Core 完整装配该控制面，standalone Core 接线、Shadow 观察和受控 Runtime 启用继续由 `AD-034` 跟踪。

G4 预筛评测使用独立 `dipole.agent.subscription-prefilter-*.v1` 合同。受控 corpus 保存最多 10000 条事件与人工相关性标签；candidate evidence 将 `rule|embedding|small_model` 的 revision/configuration SHA-256 绑定到逐 case 决策、basis-point 分数、微秒耗时和微美元成本。纯 evaluator 不访问模型、数据库或网络，输出 corpus/evidence SHA-256、混淆矩阵、向下取整的 precision/recall bps、nearest-rank p95、向上取整平均成本和误判 case ID，不回显消息正文。首个 rule adapter 直接复用生产 `matchEventSubscriptions`，防止测试基线与部署语义分叉。synthetic 示例只验证合同和规则链；真实 Project Guardian corpus 及 embedding/小模型 evidence 达标前，生产仍固定 `direct_target`。

Corpus review v1 将同一 corpus SHA-256 绑定到两个独立 reviewer 的完整逐 case 标签。两个标签集有分歧时，第三个独立 adjudicator 必须精确裁决全部分歧 case；身份复用、缺失/多余 case、无分歧时的多余裁决和最终标签漂移均 fail closed。离线 CLI 输出 review/final-label SHA-256、向下取整 agreement bps、计数与异常 case ID，不回显事件正文或 reviewer 身份。该合同只提供评审 provenance；真实事件收集、脱敏和 reviewer 操作仍由受控流程负责。

Subscription rollout gate 不信任调用方预先聚合的报告，而是从 corpus、review 与 candidate evidence 重新执行两个 evaluator。只有同一 corpus 的 review 和 candidate 均通过才输出 `eligible`，决策绑定 corpus/review/final-label/candidate evidence/configuration 哈希及 agreement、precision、recall、p95、成本指标。`eligible` 只进入 operator review，不修改 Runtime mode、Trigger mode 或 Capability authority。

默认关闭的 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED` 为 `direct_target` 增加在线确定性对照：同一 Kafka 事件在 EventLedger 前调用 Core matcher，只累计固定 `accepted|ignored × match|miss|error` 指标和候选总数，随后仍按 direct-target 结果执行。matcher 失败只记录 error，不能阻断主路径，也不能创建第二个 Task、Workflow 或模型调用。运维与回滚见 `docs/agent/agent-subscription-shadow.md`；该观察证据不能替代 reviewed corpus 与离线 precision/recall 门槛。

在线对照的 `dipole.agent.subscription-shadow-evidence.v1` 只接受 24 小时以上的 Prometheus 起止快照，绑定 Runtime/config SHA-256、query revision、抓取覆盖率和 `resets()` 结果；至少 100 个事件、95% 抓取、零 reset、零 matcher error 才能形成最多有效 24 小时的 passing evidence。Schema 与 CLI 固定双 authority 为 false，Runtime 启动链不读取该证据。

只读 `dipole.agent.subscription-shadow-collection.v1` Collector 固定历史查询集合并生成上述 input：单 Agent series、全窗口 Shadow enabled、Prometheus envelope 与安全整数均 fail closed，URL 不得携带凭据。Collector 不写远端状态，也无法从现有指标证明部署 artifact revision；Runtime/config SHA-256 继续由受控发布记录提供。

Message v1 Envelope 可选携带 `lineage`：`origin.type/id` 标记自动化根来源，`causation_event_id` 指向直接父事件，`agent_task_id` 固定根 Agent Task。Kafka consumer 在进入业务 handler 时将 causation 滚动为当前 `event_id`；Agent 动作保留已有 Agent 根来源，Transactional Outbox 因此可将同一因果链写入 confirmed Message fact。TypeScript Trigger Engine 在领取 EventLedger、创建 Temporal Workflow 或调用模型前抑制 `origin.type=agent` 且 `origin.id` 等于当前 Agent 的事件。旧 v1 事件缺少 `lineage` 时继续按原路径处理；Agent origin 缺少 Task、未知 origin type 或非法标识符时 fail closed。

## 6. Human-in-the-loop 与 Artifact

高风险动作进入 `WAITING_APPROVAL`，通过 Temporal Signal 接收批准或拒绝。缺少结构化输入时进入 `WAITING_INPUT`。G3 v1 已固定 `dipole.agent.elicitation.v1`：Workflow 持久保存受限 Form、request ID 与绝对截止时间，Gateway 从 JWT 派生 principal，Runtime 经 Core 复核 Task 所有权并按当前 Form 校验响应后发送 Signal。Approval 使用持久 binding 的 expiry；Input 使用 Activity 记录的 deadline。旧 request、跨用户、未知字段与非法选项均 fail closed；Timer 到期后以 `input_expired|approval_expired` 取消并完成持久 Run，Worker 替换后由 Workflow history 恢复等待点和同一 Timer。canonical Pencil 和默认关闭的 Vue 普通 Form 已完成；MCP 单轮 Form continuation 使用新 Client/Transport 恢复并拒绝敏感字段、URL mode 与第二轮请求，详见 `AD-036`。

外部 MCP Activity 通过 migration v35 的权威 Tool command 和 migration v36 的 durable round receipt 恢复。确定性 Round ID 绑定 Invocation、轮次和 canonical 请求摘要，首次 `claimed` 才允许连接远端；结果先写入 MySQL 终态再返回 Temporal。`replay_completed|replay_failed` 不触达网络，遗留 `executing` 固定为 `ambiguous` 且没有 lease reclaim。远端执行后、本地收据前的窗口采用 `remote_outcome_unknown` at-most-once 失败策略；生产 Worker、Transport Factory 和外部开关继续关闭。

MCP Worker command dispatcher 的不可信输入面只包含 Task/Run/Invocation ID。Core `ResolveMcpToolCommand` 返回持久开始时间和完整命令，TS 复算参数摘要并以全部 authority 字段生成 binding；request ID 和输入截止时间由 Invocation 固定派生。恢复前使用同三 ID 再次解析 Core，并在建立新 Session 前比较命令与 Activity checkpoint。Session Factory 只接收 tenant/profile/server/tool，命令参数与 Task 身份不会传播到连接工厂。

任务输出同时支持 Message 和 Artifact。报告、任务清单、事故分析和会话摘要保存为版本化 Artifact，元数据进入 MySQL，大对象进入 MinIO。

G3 v1 已实现 `conversation_digest` 产物：Artifact ID 绑定 Task、Run、类型、版本和正文 SHA-256，正文限制 1 MiB，元数据限制 16 KiB。`dipole-agent` 只能为当前运行中的 Shadow Run 创建产物，Gateway 只能以 Task principal 读取；读取和精确重试都会验证对象大小与哈希。默认关闭的 Gateway metadata seam 只返回 Artifact 身份、Task/Run、类型、版本、标题、媒体类型、摘要、大小和创建时间，并在返回前复核正文长度与 SHA-256；正文、对象键与 metadata JSON 仍不进入浏览器 API。Timeline 的 `artifact` 事件现可选携带经过 SHA-256 校验的 `artifact_id`，用于关联同一 Task 内的 owner-scoped metadata；主投影和失败修复队列共同持久化该关联，历史事件缺失该字段时保持可读。当前没有更新、删除、公开 URL、下载、消息转换和 active 模式写入，Pencil 页面与正文披露策略等待独立契约。

## 7. 数据模型

首期包含：

- `agent_definition_versions`：所有者、Agent、permission、resource scope、有效期、版本和撤销状态；grant 内容按版本追加，Task 始终固定精确版本。
- `agent_event_subscriptions`：v28 保存事件、资源、确定性过滤器和固定 Definition version，v34 增加 owner/revocation 审计与稳定管理重放；Task 可空绑定触发 Subscription ID。
- `agent_tasks`：目标、触发来源、主体、状态和固定 Definition version；v16 先提供 compare-and-set 状态迁移，后续追加 Temporal Workflow ID。
- `agent_runs`：v21 按 Task、runtime 和 mode 保存独立 `running/completed/failed/cancelled` 生命周期；同一 Task 可同时拥有 Embedded、Shadow 和未来 Active Run。
- `agent_shadow_steps`：v20-v21 保存不可变 capability/input、attempt、lease/token 和 result/error；Step owner 只能用精确 token 在租约内完成，重领后旧 owner 无法覆盖。
- `tool_invocations` / `agent_approvals`：参数、结果、风险、授权依据和审批状态；v16 Approval 已绑定 capability、canonical scope hash、arguments hash、nonce 和有效期，并通过条件更新完成一次性消费。
- `agent_artifacts`：类型、URI、版本、来源和元数据。
- `agent_memories`：作用域、类型、来源、置信度和过期时间。

敏感输入输出采用脱敏摘要和受控对象存储，审计记录避免保存明文凭据。

当前 `dipole.agent.policy.persistence.v1`、migration v16-v34 和 sqlc Store 已落地 Definition/Task/Approval/Run/Artifact/Subscription 的持久边界。Embedded Go/Eino 默认使用 persistent policy：触发事件创建确定性 Task、固定并重新读取精确 Definition version、校验有效期与撤销状态、恢复 permission/resource scope，再以 CAS 进入终态。TS Runtime 使用同一 Task ID，经受认证 admission 创建独立 Shadow Run；已完成 Run 的 admission 与 completion 均幂等收敛。`ai.policy_mode=static` 仅作为显式回滚；v17 以 expand-only 方式将 policy 身份列扩到 24 字符。

## 8. 可观测性与评测

Trace 层级采用 `Task → Run → ContextCompile / ModelCall / ToolCall / Approval / ArtifactCreate`。指标至少覆盖任务完成率、Tool 成功率、审批拒绝率、Token 与成本、上下文大小、检索命中、模型回退和端到端延迟。

G4 已通过统一 `AgentTelemetry` 将上述阶段接入 `@opentelemetry/api`。Kafka Foundation 在 Event Processor 建立 Task/Run 父子边界；Temporal 的 Workflow 代码保持确定性，Task admission/finish、Run 执行、Approval 和 Artifact 仅在 Activity 进程创建 span。ContextCompile 记录版本、估算 Token 和选取数量；ModelRouter 为每个真实 provider attempt 建立 ModelCall；native Capability 与 MCP 调用均使用 ToolCall。属性只包含稳定 ID、阶段、route、计数、Token、大小和状态，异常统一写稳定阶段错误，不记录 Prompt、消息、Memory、Tool 参数/结果、Artifact 正文或底层异常文本。Runtime 已增加默认关闭的 Node trace SDK + OTLP/HTTP protobuf exporter，使用标准 endpoint/protocol/sampler 环境变量和 ParentBased trace-id ratio 采样，并在逆序关闭末尾 flush；关闭总开关时不创建 SDK。独立 `observability` profile 以 Collector memory limiter/batch 将 trace 写入 24 小时 local Tempo，并提供 Collector down、export failure 和 refused span 告警；真实 smoke 按 trace ID 验证写入和查询。local backend 只用于 Shadow/验收，生产对象存储、通知路由和长期保留证据仍由 `AD-037` 管理。

Eval Harness 同时评估：

- Outcome：结果是否满足目标并包含必要证据。
- Trajectory：Tool 选择、顺序和调用次数是否合理。
- Permission：是否访问越权资源或尝试绕过审批。
- Retrieval：关键证据召回率和引用正确性。
- Cost：模型调用、Token、延迟和失败重试是否在预算内。

模型、Prompt、Tool Schema 和 Memory Policy 升级先跑离线数据集，再进入 shadow，最后按 Agent 或用户灰度。

G4 使用 `dipole.agent.offline-eval-suite.v1` 固定五类 deterministic case，并生成绑定 candidate version 与 canonical Suite SHA-256 的低敏报告。Outcome 检查必要/禁止输出 ID，Trajectory 检查精确 Step 与禁止动作，Permission 比较 capability/resource/action 决策，Retrieval 计算 precision/recall，Cost 对模型调用、Tool 调用、Token、微美元与延迟执行硬预算。Harness 不调用 LLM judge，避免评测自身产生随机性和未审计成本。

Shadow 晋级 v2 将完整五类报告作为证据，任一类别缺失或失败均阻断；v1 保留历史兼容。G4 真实 Task adapter 使用 sqlc/TS 共享查询只读提取 Task/Run、Context provenance、Step、Artifact、ModelCall 与 ToolCall，并与人工评审 manifest 合成五类 Suite；Task/Run 摘要进入 case ID，Suite SHA-256 因此绑定来源执行且报告不暴露内部 ID。模型路由单价必须显式版本化，缺失终态或指标时 fail closed。Subscription corpus 已有双评审 agreement 合同和低敏报告，但真实 Project Guardian corpus、生产 retrieval relevance 和成本分位阈值仍需在切流前独立采证（`AD-038`）。

G4 security suite 复用五类 Harness 串联实际 Runtime 边界：ContextCompiler 保留系统策略顺序并标记外部事件为 `untrusted`，Capability Registry 在执行前拒绝越权资源，EventLedger 收敛重复事件，lineage 在 Ledger/模型前抑制同源循环，MCP Client 在网络发送前执行 Tool 级 egress policy。Egress policy 采用显式顶层参数 allowlist、请求大小/深度上限和常见凭据字段拒绝；值级 DLP、外部凭据托管和模型语义攻击仍属于切流前门禁。

G3 Shadow 晋级使用 `contracts/agent-promotion/v1/policy.json`：同一候选版本连续观察至少 24 小时，至少 24 个观察点且最大间隔 90 分钟，累计比较至少 100 个 Task，Workflow projection 六类对账中只能出现 `match`；projection、outcome、trajectory、permission Eval 必须全部通过。策略评估只产出 `eligible|blocked` Artifact，不修改 Runtime mode。Workflow repair 也只生成一小时内有效、绑定操作员声明/工单/Temporal 证据与 SHA-256 的 proposal；服务端认证、持久审计和审批链完成前没有执行入口。

当前 Embedded Go/Eino baseline 位于 `contracts/agent-evals/v1/go-eino-baseline.json`。它通过真实 Service/Tool adapter 测试固定 direct trigger 过滤与幂等、普通回复、Tool 回复去重、会话授权和消息读取轨迹。两个原 `AD-008` case 持续提交恶意身份参数，并要求资料读取和系统消息目标使用服务端派生 principal；TypeScript Runtime 必须通过同一契约后才能获得流量。

Embedded Runtime 的 `ExecutionContext` 由 Service 从触发 Message、持久 Task policy snapshot 与 correlation context 共同派生 principal、Agent、会话、permission/resource scope 和 request/trace/event ID。Tool schema 不暴露身份字段，缺少可信上下文或资源授权时拒绝执行。

进程内 Capability 基线位于 `contracts/agent-capabilities/v1/schema.json` 与 `application.AgentCapabilityV1`。五项 operation 覆盖受限用户资料、Agent 直聊消息、会话列表、授权会话读取和系统消息命令；`app.LocalAgentCapabilityV1` 组合 Core Capability、Conversation Service 与 Message Application。Embedded ContextBuilder/Tool 仅依赖该端口，远程 gRPC/Connect adapter 后续复用同一 contract。

G1 使用 `ai.runtime_mode` 控制迁移：

| 模式 | Go/Eino consumer | TS Runtime | 写入权 |
| --- | --- | --- | --- |
| `off` | 关闭 | 关闭 | 无 |
| `embedded` | 权威执行 | 关闭 | Go |
| `shadow` | 权威执行 | 独立 consumer group 旁路评测 | 仅 Go |
| `remote` | 关闭 | 权威执行 | TS 经 Capability/Command API |

未配置 mode 时，`ai.enabled=true|false` 兼容映射为 `embedded|off`。显式 mode 优先，非法值阻止 Kafka handler 注册。TS Runtime、远程 Capability 与写入审批门禁完成前，生产环境不能切换 `remote`。

Embedded policy 另由 `ai.policy_mode=persistent|static` 控制。默认 `persistent` 使用 MySQL Task snapshot；`static` 使用同一 Invocation/resource scope 授权链回滚到代码内基线，不改变 Runtime 流量模式。

## 9. 渐进路线

1. 固化 Go/Eino 行为基线、可信 ExecutionContext、事件契约和评测集，建立 Capability API 与 Agent Command API。
2. 建立 TS Runtime 骨架，实现 ExecutionContext、Capability Registry、Kafka shadow consumer 和执行审计。
3. 引入 Temporal AgentTask，支持等待输入、审批、重试、取消和恢复。
4. 实现 Context Compiler、分层 Memory、Event Subscription 和 Artifact。
5. 接入 MCP client/server、OpenTelemetry 和完整 Eval Harness。
6. shadow 结果达到门禁后逐步切换 `agent.mode=remote`，保留 Eino 回滚窗口。
7. 后期按明确场景评估多 Agent、A2A Agent Card 和 MCP experimental Tasks。

## 10. 首个演示验收

Project Guardian 订阅一个项目群，每日维护决策、任务和风险；发现缺失负责人时向用户索取输入；准备向群内发送提醒时等待审批；进程重启后继续原 Task；所有读取、Tool、审批、Artifact 和模型调用均可追踪。

首期避免引入无明确职责的多 Agent 编排，也避免每条消息直接调用高成本模型。

G2 foundation 已建立在 `services/agent-runtime/`：Node 22+、Fastify 5、Zod 4、AI SDK 7、KafkaJS 2 与 mysql2 由独立 package 管理；领域内核已实现严格 ExecutionContext、resource-scope Policy Engine、Capability Registry、Go 兼容 Task/Run ID 和只读 shadow processor。KafkaJS adapter 使用独立 `dipole-agent-shadow-*` group 消费兼容 v1 Message 事件，冷启动执行有界重连。migration v18 与 MySQL EventLedger 通过 Event/Task 双唯一、事务 claim、lease 和精确 token 提供跨进程幂等，Compose 使用 Agent 专用最小权限账号。物理 main/retry/dead topic 在 readiness 前显式创建并校验；永久错误直达 dead，瞬时错误有界重试，失败转移成功后才完成源 handler（`AD-028` 已关闭）。provider-neutral ModelRouter 已支持有序降级、总 deadline 与单次输出 Token 上限；AI SDK adapter 使用 Zod structured output 并关闭内部 retry。`ai_sdk` 模式通过单一 OpenAI-compatible adapter 注入真实 LanguageModel，Provider name 绑定全部 route 前缀，base URL 与密钥在配置解析阶段校验，默认 `metadata` 不创建 Provider。migration v19 与 MySQL ModelAuditStore 固定预算快照、原子 call slot 和 completed/failed/abandoned 轨迹，Router 每次 provider 调用均先占用 slot，跨 Kafka 重投共享上限（`AD-029` 已关闭）。migration v20-v21 保存不可变 Shadow Plan、有序 Step、独立 Runtime Run 与 lease/token 终态。受认证 `dipole.agent.v1` gRPC 通过 mTLS `dipole-agent` 身份完成 Task admission、持久 policy 解析和首个 `conversation.list` 执行，principal 只来自服务端 Task，静态 protobuf client 与最小 RPC allowlist 已关闭 `AD-030`。模型模式默认 `metadata`，显式 `ai_sdk` 强制 MySQL Store、Capability RPC 和只读 shadow；write capability 继续关闭。
