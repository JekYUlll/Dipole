# 更新日志

本文档记录 Dipole 的重要功能、行为变化、兼容性说明和修复。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。项目引入正式版本后，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## 维护约定

- 日常开发统一追加到 `Unreleased`，不要直接创建临时版本章节。
- 条目描述用户或系统可观察到的变化，并标明影响范围；纯格式调整和无行为变化的重构通常无需记录。
- 分类按需使用：`新增`、`变更`、`修复`、`安全`、`弃用`、`移除`、`迁移说明`、`验证`、`已知问题`。
- 正式发布时，将 `Unreleased` 内容移动到 `## [X.Y.Z] - YYYY-MM-DD`，随后保留一个空的 `Unreleased` 章节继续滚动更新。
- 数据结构、接口兼容性或部署步骤发生变化时，必须补充 `迁移说明`；未解决但会影响开发或发布的问题写入 `已知问题`。
- 可在条目末尾附关联 Issue、PR 或提交，例如 `(#123)` 或 ``(`abc1234`)``。

## [Unreleased]

### 新增

- Agent G4 增加 active-only MCP Approval grant resolution：sqlc 按 Task/Capability/Scope/Arguments 查询最多两条 approved、未消费、未撤销且未过期记录，应用层要求 active `dipole-agent` Run、运行中 Task、principal 审批人与唯一 exact binding；零匹配和多匹配统一拒绝。认证 RPC 只返回 Approval ID、Resource Scope、三个 SHA-256 摘要与过期时间，不消费审批。TS client 复算 scope/arguments 并校验响应，grant adapter 直接连接现有 write gate。`nonce_sha256` 明确作为一次性绑定摘要，避免依赖无法持久恢复的原始 nonce。生产 MCP context 继续为 shadow。
- Agent G4 增加默认关闭的第一方 MCP Message write projection：显式 write executor 只接受 `mode=active`、`risk=write`、`approvalRequired=true` 且声明 Command kind 的 Capability。调用先校验 direct conversation，再由 Approval gate 消费精确绑定，Tool runner 生成同一个 Invocation ID 依次完成 audit begin、Message Command RPC 和带 action reference 的 audit finish；Command kind 或返回证据漂移会记录失败终态。现有生产 `index.ts` 未注册 write Capability、未注入 executor/grant resolver，Core MCP context 继续为 shadow。
- Agent G4 增加认证的 MCP Message Command RPC：`dipole-agent` 只能在已持久化、仍为 running、带已消费 Approval 且 Task/Run/Capability/身份完全匹配的 Tool Invocation 中请求消息动作。Core 从可信 Invocation 派生 sender/target，按标准化 `{content, conversationId}` canonical JSON 重算审批参数 SHA-256，并根据 `invocation_id + command kind` 生成稳定 Command ID；RPC 只返回 Message UUID、Command 引用和可复算 `client_message_id`。TS Runtime 独立验证返回证据，Tool runner 与 Approval gate 统一使用排序 canonical JSON。生产 MCP Server 仍只注册 read Tool。
- Agent MCP ToolCall 增加可验证的消息动作血缘：migration v31 与 sqlc 在 Begin 记录已消费 `approval_id`，在完成记录中只保存 `message` 资源 UUID、Command kind/id 与既有结果摘要，不复制消息正文。Core 对写 Capability 重新校验 active Agent Run、Task、参数摘要、审批终态与资源授权，再按权威 Agent sender 和稳定 Command ID 查询 Message receipt；只有 committed Message 的 UUID、sender、target、conversation 和 message type 全部一致时才能完成审计。protobuf 与 TS runner 支持可选 action reference，读 Tool 携带审批/动作引用、失败终态携带动作引用或引用漂移均 fail closed；生产 MCP Server 继续只投影 read Tool。
- Agent Message Command 增加 sender-scoped receipt/status 查询：Message v1 新增 `GetMessageCommandReceipt`，从认证 principal 派生 sender，并复用现有 sqlc `(sender_uuid, client_message_id)` 查询返回 `ABSENT|COMMITTED`，不新增 Command 表或 pending 状态。Agent 发送失败后使用脱离原取消信号、保留 trace values 的独立 2 秒窗口查询 receipt；仅 sender、target、conversation、message type、content 和稳定幂等键全部匹配时恢复成功，查询故障保留发送与查询两条错误因果链。
- Agent G4 增加默认关闭的 MCP durable Elicitation adapter：只接受 `elicitation/create` form mode 的严格子集，将最多 16 个 text/select/multiselect/boolean 字段转换为现有 `dipole.agent.elicitation.v1` 和 Temporal `wait_input` directive。checkpoint 绑定 host-owned Request、Server、Tool、Invocation、deadline、完整 Form 与 `trust=untrusted` SHA-256；仅精确且未过期的 durable resume 可转换为 MCP `accept`，`decline/cancel` 也要求同一 Request。URL、number/integer、default、format、description、自由扩展、敏感字段和 checkpoint 漂移均拒绝。当前外部 MCP Client 未声明 Elicitation capability，也未注册 request handler。
- Agent G4 增加默认关闭的 MCP write Approval gate：Core 新增 authenticated、active-only 的原子 `ConsumeApproval` RPC，使用现有 MySQL/sqlc 条件更新精确绑定 Task、Run、Capability、Resource Scope、canonical Arguments 与 nonce 摘要，并让重放、过期、吊销或任一哈希漂移返回拒绝。TS `McpWriteApprovalGate` 先通过 Capability Registry 完成 schema、Policy 和 Resource 校验，再按与 Go 一致的 scope v1/canonical JSON 计算 claim；只有原子消费成功才执行副作用。生产 active authority 尚未装配，启动链继续只投影 read Tool。
- Agent G4 增加外部 MCP Result-to-Context adapter：只接收成功、可 JSON 序列化且默认不超过 64 KiB（1 KiB 至 128 KiB 可配）的 Tool 结果，生成固定 `section=evidence`、`trust=untrusted` 的可选 Context fragment，并用 Profile/Server/Tool/Invocation 绑定 provenance。完整正文保留为不可变 JSON 快照；compact 版本只包含 Server、Tool、content type 集合和 structured-content 标记，不复制可能含 Prompt Injection 的外部文本。该 adapter 尚未接入生产外部调用链，外部网络继续关闭。
- Agent G4 增加默认关闭的外部 MCP Network Guard：每个 SDK HTTP 请求都重新校验 Profile 的 exact HTTPS Host/Port/TLS ServerName，使用 2 秒默认 DNS deadline（100 ms 至 60 秒可配），要求 1 至 32 个解析结果全部为合法公网地址且无重复。批准地址集合和 opaque CA ref 被交给 pinned Dispatcher，返回后复核实际 peer；混合私网答案、DNS rebinding、非批准 peer、跨边界 URL 和任何重定向均以固定脱敏错误 fail closed。当前只有 Resolver/Dispatcher 接口与测试实现，生产 DNS/TLS Dispatcher、CA Secret backend 和外部连接开关继续关闭。
- Agent G4 增加 provider-neutral 外部 MCP `AuthProvider` adapter：官方 SDK 每次请求前按 exact Credential binding 调用 Secret Provider，默认 2 秒超时（100 ms 至 60 秒可配）和 4 KiB 上限（16 B 至 8 KiB 可配），只接受严格 UTF-8 与 RFC 6750 Bearer 字符。Provider failure、timeout、非法内容分别收敛为 `secret_unavailable`、`secret_timeout`、`secret_invalid`，不携带底层异常或 credential 标识；成功、校验失败及超时后迟到的 fresh byte buffer 均被覆盖。Adapter 不缓存 token 且不实现 `onUnauthorized`，避免 SDK 自动刷新；JavaScript token/Header 字符串仍由 GC 管理，生产 Secret backend 与外部网络继续关闭。
- Agent G4 增加外部 MCP Credential Catalog 的受约束文件源：只接受规范绝对路径，要求 canonical、root/预期 Runtime owner 且 group/other 不可写的父目录，并在每次解析时以 `O_NOFOLLOW` 重新打开 single-link regular file；文件 owner/mode 同样受检，配置上限为 32 B 至 1 MiB（默认 256 KiB）。固定缓冲读取可阻断检查后增长绕过，原子 rename 后下一次建连立即使用新 lifecycle manifest；任意路径 symlink、目录、错误 owner/mode、超限和畸形 JSON 均 fail closed，旧 Catalog 不缓存。该 source 尚未装配到生产启动链，也不包含 secret material。
- Agent G4 增加外部 MCP Credential Catalog v1：语言中立 lifecycle manifest 仅保存 tenant、credential ref/version、生效/过期/吊销状态、provider ID 和 opaque provider secret ref，拒绝附加 secret 字段、重复绑定与非法时间状态。Catalog 在每次建连前重新加载，精确版本已吊销、未生效、过期或跨租户时均在 Transport Factory 前 fail closed；轮换可通过新增版本并更新 Profile 完成，Task、Workflow、Context 和审计不接触秘密正文。生产 Catalog source、Secret Provider 与真实外部连接继续关闭。
- Agent G4 增加默认关闭的外部 MCP 凭据与网络边界：语言中立 Profile v1 仅允许 tenant、HTTPS endpoint、Server/Tool/Host/Port allowlist、TLS ServerName、CA opaque ref 和版本化 credential opaque ref；拒绝 URL 凭据、query/fragment、IP/localhost/内部域名及附加 secret 字段。租户 Registry 只有在精确 owner 匹配后才调用注入式 Transport Factory，并要求 Factory 每次建连拒绝非公网 DNS 解析；当前生产 Provider 尚未实现，误开 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED` 会直接 fail closed，不产生外部连接。
- Agent G4 为 MCP 执行补充有界取消/超时：Runtime Tool invocation 默认 5 秒并限制在 100 ms 至 60 秒，超时触发 cooperative `AbortSignal`、持久化稳定 `tool_timeout` 且关闭 OTel span；外部 MCP Client 的 connect/list/call 默认使用 10 秒 request/total timeout并接受调用方取消信号。Gateway 断连经 Runtime Streamable HTTP Request signal 传播，DELETE Session 清理与长流入口不受统一代理超时破坏。配置保持默认关闭路径无行为变化。
- Agent G4 增加默认关闭的第一方 MCP 授权交换与资源边界：受认证 session 需对唯一 canonical resource 和 `dipole.agent.mcp.read` 显式 consent，才能取得 15 分钟专用 JWT；令牌以 `aud`、`scope`、`token_use` 防止跨资源和 session/MCP 混用。Gateway 验证后剥离客户端凭据，仅向 Runtime 传递可信 principal/resource/scope，Runtime 再构造只读 `AuthInfo`。Compose 支持统一覆盖 canonical URI，发布与回滚见 `docs/agent-mcp-authorization.md`；通用 OAuth 2.1 discovery/PKCE/客户端注册继续由 `AD-037` 跟踪。
- Agent G4 增加默认关闭的 OpenTelemetry 运维 profile：Collector `0.159.0` 通过 128 MiB memory limiter、有界 batch/queue 和重试向 Tempo `2.10.5` 写入 trace，Tempo local backend 固定 24 小时保留且端口只绑定 localhost；Prometheus 新增 Collector down、export failure 和 refused span 三类低基数告警。配置 gate 验证 Collector 与规则，真实 smoke 生成 Agent span、核对 accepted/sent 指标并按 trace ID 从 Tempo 查询；运维说明补充 Task/Run 审计联查和回滚。生产对象存储与通知链继续由 `AD-037` 跟踪。
- Agent G4 增加默认关闭的生产 OpenTelemetry 装配：Node trace SDK 通过 OTLP/HTTP protobuf 导出既有低敏 span，复用标准 `OTEL_*` endpoint、protocol、ParentBased trace-id ratio sampler 与超时参数，限制单 span 属性/事件/link 数量，并在 Runtime 逆序关闭末尾 flush。关闭 `DIPOLE_AGENT_OTEL_ENABLED` 时不实例化 SDK且忽略残留 OTel 配置；Collector、保留和告警继续由 `AD-037` 跟踪。
- Agent G4 增加统一低敏 OpenTelemetry 全链路：Foundation Event Processor 记录 Task/Run，Durable Activity 记录 Task admission/finish、Run、Approval 和 Artifact，Context Compiler 记录版本与预算统计，Model Router 为每个真实 provider attempt 记录 ModelCall，native Capability 与 MCP 均记录 ToolCall。Temporal Workflow 保持确定性；span 禁止 Prompt、消息、Memory、Tool/Artifact 正文和底层异常文本，SDK/exporter 装配由后一里程碑承接。
- Agent G4 增加真实 Shadow Task 离线评测 adapter：sqlc 与 TypeScript 从同一只读 SQL 源提取 Task/Run、Context provenance、Step、Artifact、ModelCall 和 ToolCall，结合版本化人工评审 manifest 生成五类 Suite；Task/Run 摘要进入 case ID，报告继续保持低敏。独立评测账号仅授予八张审计表 SELECT；非终态、capability 绑定漂移、缺失 Token/延迟或路由单价均 fail closed。当前仍需归档 Project Guardian 人工语料、reviewer agreement 与生产阈值后才能满足 `AD-038` 关闭条件。
- Agent G4 增加五路径 deterministic security suite：真实 ContextCompiler 验证 Prompt Injection 保持 `untrusted` provenance，Capability Registry 验证越权资源在执行前拒绝，EventLedger/lineage 验证重复事件单次规划与同源循环在 Ledger 前抑制，MCP Client/Server 验证敏感外发在网络调用前阻断。外部 MCP Tool 现在必须配置与 allowlist 完全匹配的 egress policy，限制顶层参数、JSON 大小和嵌套深度，并递归拒绝常见凭据字段；值级 DLP 与模型语义攻击继续作为 Shadow 切流门禁。
- Agent G4 增加 outcome、trajectory、permission、retrieval、cost 五类 deterministic 离线评测：严格 Suite/Report 契约要求五类 case、唯一 ID 和有界输入，以 canonical SHA-256 绑定候选数据集；CLI 输出不含消息正文的类别结果与 precision/recall、调用、Token、成本和延迟指标，并以 `0/2/1` 区分通过、评测失败和输入错误。Shadow promotion v2 强制携带同一候选版本的完整报告并逐类别阻断，v1 证据继续兼容；当前 synthetic fixture 只验证 Harness，真实 Task/corpus 由 `AD-038` 跟踪。
- Agent G4 为认证 MCP 入口增加 Redis principal 限流：Gateway 在 JWT principal 解析后对 GET/POST 使用统一 `rate:agent_mcp:{principal}` 固定窗口，跨 Task、Run、方法和 Gateway 实例共享额度；超限返回 429 与向上取整的 `Retry-After`。Redis 缺失、调用失败或额度配置非法时 fail closed，DELETE 继续允许释放 Streamable HTTP Session；旧登录、消息和文件限流的兼容语义不变。
- Agent G4 增加 migration v30 与 MCP ToolCall 持久审计：Core/sqlc 绑定权威 tenant、principal、Agent、Task 和 Run，仅保存参数/结果 SHA-256、结果字节数、耗时、稳定错误码与单次终态；TypeScript Runtime 只有在 durable begin 成功后才执行只读 Capability，并使用 `@opentelemetry/api` 创建不含正文的 ToolCall span。结果超过 64 KiB、工具异常或审计不可用均 fail closed；默认未装配 OTel SDK/exporter。
- Agent G4 增加默认关闭的认证 MCP 网络入口：Gateway 仅在 JWT 成功后转发 `GET/POST/DELETE` Streamable HTTP，并覆盖客户端伪造的服务身份与 principal；Runtime 通过 additive `ResolveMcpContext` RPC 让 Core 复核 Task、Run、固定 Definition、grant 和 resource scope，再构造只读 `conversation.list` ExecutionContext。服务密钥、Cookie 与公开 Authorization 不进入 MCP handler，SSE 和 Session header 保持流式转发。
- Agent G4 增加官方 MCP TypeScript SDK v2 foundation：MCP Server 将显式 allowlist 的只读 Capability 投影为 Tool，并由宿主注入经校验的 ExecutionContext；MCP 参数无法提供 principal。MCP Client 同时校验配置 allowlist、握手 Server identity、已发现 Tool、256 项发现上限及 128 KiB 响应上限。进程内与 Streamable HTTP 契约均通过，HTTP handler 要求宿主提供已验证 `AuthInfo`；write/destructive Tool 保持关闭。
- Agent G3 为 `WAITING_INPUT` 与 `WAITING_APPROVAL` 增加持久 deadline 和 Temporal Timer：Input Activity 与已持久 Approval binding 提供绝对截止时间，Workflow history 固定该值；到期且没有精确 Signal 时确定性进入 `cancelled`，分别记录 `input_expired` 或 `approval_expired`，投影终态并完成持久 Run，避免无限等待占用执行资源。
- Agent G3 增加语言中立 `dipole.agent.elicitation.v1` 与持久输入恢复链：Temporal `WAITING_INPUT` 固定受限 Form 和 request ID，Gateway 公开 JWT 认证输入 API，Runtime 经 Core 复核 Task principal 后校验精确字段、类型、选项和 16 KiB 上限，再发送 Signal。无效值、跨用户、旧 request 与终态请求 fail closed；Worker 替换后仍可从 Workflow history 恢复同一等待点。Pencil 客户端、敏感输入与 MCP Elicitation adapter 继续关闭。
- Agent G3 增加 migration v29 与默认关闭的 scoped Memory foundation：MySQL/sqlc 以 tenant、Task principal、Agent 和 conversation resource 保存不可变 Working/Episodic/Semantic/Procedural/Observational Memory，记录 full/compact content、priority、有效期和 provenance。受认证 `dipole-agent` 只能用运行中的 Task/Run 请求 Core；Core 从固定 Definition 解析 read permission/scope，并以 Task 创建时间固定可见记录上界，撤销/过期立即失效。显式启用后，TypeScript ModelShadowPlanner 为实际命中的 Memory 分配独立 500-token 预算并以 `untrusted` provenance fragment 注入 Context；空结果保留原 evidence 预算和路径。
- Agent G3 增加 migration v28 与 Event Subscription 确定性触发基础：Core/sqlc 按 Definition version、tenant、Agent、事件和 conversation resource 持久化 `all|message_contains_any` 订阅，受认证 `dipole-agent` RPC 仅返回当前有效 Definition 且具备 read scope 的候选。TypeScript Runtime 可显式选择 `subscription` 模式，在 EventLedger、Temporal 和模型调用前完成严格 schema、资源、事件及 Unicode 关键词过滤；零匹配直接退出，多匹配按 Subscription ID 稳定选择，并把 ID 固定到 Task 防止重放漂移。默认与 Compose 保持 `direct_target`，现有流量不切换。
- Agent G3 增加 Message v1 事件 lineage：可选 `origin`、`causation_event_id` 与 `agent_task_id` 从 Embedded Agent 命令经 Kafka `send_requested`、consumer context 和 Transactional Outbox 传播到 confirmed Message；Agent origin 强制绑定 Task，非法 lineage 在 Go/TS decoder 中 fail closed。TypeScript Shadow Runtime 在领取 EventLedger、启动 Temporal 或调用模型前返回 `suppressed`，阻断同一 Agent 因果链的循环触发；不带 lineage 的 legacy v1 事件保持兼容。
- Conversation State 增加低基数成功写 Counter 和 `dipole_conversation_projection_write_duration_seconds{projection,outcome}` Histogram，区分 direct message、group message 与 group init 的 Repository 调用耗时和错误；端到端基准升级为兼容 v1/v2 的 operations/baseline v3，逐节点保存前后 Prometheus 快照并拒绝 Counter 回退或成功次数漂移。20/100 人普通群和热群写放大证据位于 `benchmarks/ad005-2026-08-27/`，projection timing 与原始快照位于 `benchmarks/ad005-projection-timing-2026-08-27/`。
- Go 长运行服务增加缓存型动态依赖 readiness：按服务关键依赖矩阵周期探测 MySQL、Redis、Kafka、内部 gRPC、Elasticsearch 与 Cassandra，使用超时和失败/恢复双阈值避免瞬时抖动；HTTP readiness 与 gRPC health 同步摘流和恢复，Prometheus 可按服务/依赖告警。微服务 Compose 默认启用，Elasticsearch 隔离演练验证 Search 局部摘流且应用容器不发生级联重启。
- Agent Context 校准增加语言中立的 evidence/report v1 与离线 `context:calibrate` 命令：输入仅接受 synthetic corpus，要求每个 model route 覆盖中英文、代码、Emoji 与 Tool schema 五类语料，并绑定候选提交、provider/model/tokenizer revision 和 reference Token；报告逐 route 输出正文 SHA-256、估算误差、fallback 与 underestimate，原文不回显，evidence/report 均可复算哈希。完整 profile 且零低估返回 0，校准阻断返回 2，契约错误返回 1；命令不访问模型、网络或运行时配置。
- TypeScript Agent Context Compiler 增加显式启用的 route-aware Token estimator v1：候选模型 route 可声明 context window、UTF-8 bytes/token 校准值和 basis-point 安全余量，编译前按全部 fallback route 取最大 Token 估算与最小窗口；缺少声明时使用固定 `8192 / 2 bytes / 25%` 保守 fallback。Compiler v2 将配置导出的 SHA-256 estimator ID 写入不可变 Context manifest，窗口无法容纳固定 4096 输入预算与单次最大输出时在配置阶段 fail closed；默认与 Compose 继续固定 Compiler v1，避免在途不可变 Plan 重放发生哈希漂移，模型路由和流量权威不变。
- Agent Artifact 增加语言中立的 maintenance authorization/receipt v1 与离线命令：授权绑定 reconcile SHA-256、单个对象证据、两位独立审批人、独立 proposer/executor、grant version 和最长 15 分钟有效期；evaluate 重新执行对象 Stat 与 sqlc 元数据查询，输出 `would_delete` 或四类阻断 receipt。Schema 拒绝附加字段，授权固定 `delete_adapter_available=false`，receipt 固定 `delete_attempted=false`、`deleted=false`。
- Agent Artifact 增加 `dipole.agent.artifact.reconcile.v1` 离线 dry-run：独立只读 MinIO 身份固定列举 `agent-artifacts/v1/`，对象满 24 小时后才查询 sqlc 元数据并形成孤儿候选；异常键只产生不可清理告警。报告固定 `delete_authorized=false`、有界样例和可复算 SHA-256，命令不包含删除参数或删除 client。
- Agent G3 增加 `dipole.agent.artifact.v1` 与 migration v26：Temporal `read_shadow` 将持久模型摘要输出为确定性 Markdown Artifact，Core 校验 Task/Run、Shadow 模式、版本与跨语言 SHA-256 后把不可变元数据写入 MySQL、正文写入内容寻址 MinIO；精确重试重新验证对象并收敛到同一 Artifact。`dipole-agent` 仅能创建，Gateway 仅能按 Task principal 读取，协议不提供更新、删除、公开 URL、消息发送或 active 写权限。
- Agent G3 增加语言中立的 Workflow repair execution plan v1：当前只允许 `dry_run`，固定 approved Proposal、proposer、两位 approver、独立 executor grant version、当前/目标/回滚投影和三组 SHA-256 CAS 证据，并以 15 分钟有效期约束重新采证。生产 protobuf 继续没有 apply/execute/rollback 方法，实际写执行器需发布新契约版本并单独评审。
- Agent G3 增加 migration v25 与 Workflow repair 审计控制面：Core 以 Gateway 认证 principal 和默认空授权表接收服务端重算 SHA-256 的一小时修复提案，MySQL 不可变保存工单、投影/Temporal 证据和操作员身份；提案人无权审批，每位审批人仅能提交一票，至少两位独立授权操作员批准后才进入 `approved`，任一拒绝进入 `rejected`。API 未提供 apply/execute 方法，`dipole-agent` 服务身份被方法级拒绝，批准结果仍不能改变 Workflow projection。
- Agent G3 增加版本化 Shadow 晋级策略与修复提案 Artifact：候选版本需提供连续 24 小时、至少 24 个观察点、最大 90 分钟间隔、累计至少 100 个 Task 的零差异/零 unavailable 对账，并通过六项 projection Eval 与 outcome/trajectory/permission Eval；`promotion:check` 仅输出 eligible/blocked 证据。`repair:propose` 只生成绑定操作员声明、工单、Temporal 证据、一小时有效期和 SHA-256 的不可执行提案，不开放自动修复或 active 切换。
- Agent G3 增加只读 Workflow projection 离线对账：Core/sqlc 通过服务身份限定的 keyset RPC 枚举 `dipole-agent/shadow` Task 与可空投影，TS Runtime 同时读取 Temporal Query 和 Describe 的实际 Workflow/Run 绑定，输出版本化 `match|missing|stale|ahead|conflict|unavailable` JSON 报告；不一致时命令返回退出码 2，不执行自动修复。版本化 Eval 数据集固定六类结果，真实 MySQL 与 Temporal Server 覆盖分页和完成态对账。
- Agent G3 增加 migration v24 与 Temporal Workflow 状态投影：`agent_tasks` 以独立 nullable workflow binding/status/revision 保存 shadow 观察状态，保留原 Task `status` 权威语义；Core/sqlc 仅接受同一 Workflow/Run 的更高 revision，完全相同写入幂等，旧 revision、同 revision 漂移和身份漂移均 fail closed。Workflow 通过版本化 `patched` Activity 投影 running/waiting/terminal 状态，Gateway Task Query 返回 `match|missing|stale|ahead|conflict` 对账证据且不自动修复。
- Agent G3 增加默认关闭的 Gateway Task 控制桥：公开 `GET /api/v1/agent/tasks/:task_id`、取消与 Approval resolution API 由 Gateway JWT 派生 principal，经固定服务身份与共享密钥调用私有 Runtime；Runtime 每次操作都通过 additive `AuthorizeTaskControl` RPC 向 Go/sqlc Core 校验 Task 所有权，再执行 Temporal Query/Signal。审批同时绑定当前 pending request/approval ID，跨用户、旧审批、终态取消和模型/请求体伪造 principal 均 fail closed；TS 继续无权直接读取 `agent_tasks`。
- Temporal G3 增加默认关闭的 `read_shadow` 执行模式与 migration v23：Kafka Shadow 保留独立 consumer/EventLedger 和事件触发责任，成功启动稳定 Workflow 后由 Temporal Activity 执行 ContextCompiler、持久 ModelRouter、不可变 Plan 与首条只读 Capability 轨迹。成功模型调用保存 Zod 校验后的结构化输出，Activity 在 provider 返回、Plan 或 Step 完成后丢失 ACK 时可恢复同一输出和已完成 Step，不能重复付费调用或 Tool 副作用；Task/Run/admission/event 任一绑定漂移均 fail closed。
- Temporal G3 增加持久 Approval Signal：`wait_approval` 在进入等待前通过 additive `RequestApproval` RPC 保存 capability、resource scope、arguments、nonce 与过期时间绑定；Signal 必须同时匹配 request/approval ID，并由 `ResolveApproval` 校验运行中的 Task/Run 和持久 Task principal，Core 完成 approved/revoked 后 Workflow 才恢复。创建、批准、拒绝和并发网络重放均收敛，默认 `foundation` 与现有 Kafka/模型/Capability 流量保持不变。
- Temporal G3 增加默认关闭的持久 shadow 生命周期 Activity：Workflow 在 Step 前通过受认证 Capability RPC 建立确定性 Task/Run admission，并在 completed、failed、cancelled 后提交精确终态证据；Core 新增 additive `FinishRun` RPC，以 Task/Run/runtime/mode 绑定、compare-and-set 和终态证据实现网络重试幂等。`DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=foundation|persistent_shadow` 默认保持 `foundation`，Kafka、模型、Capability、Approval 与权威 Task 状态均未切流。
- TypeScript Agent Runtime 增加 Temporal G3 foundation：以持久 Agent Task ID 生成稳定 Workflow ID，运行中重复启动使用 `USE_EXISTING` 收敛、终态使用 `REJECT_DUPLICATE` 阻止重放；框架中立状态机覆盖 running、waiting_input、waiting_approval、completed、failed、cancelled，Workflow 提供输入/审批/取消 Signal 与状态 Query，模型、Capability、存储和副作用统一隔离到带三次有界重试的 Activity。Worker 配置默认关闭，Compose 显式保持零流量切换，foundation Activity 不产生外部副作用。
- TypeScript Agent Runtime 增加框架中立 `ContextCompiler` v1 与 migration v22：策略、身份、Task、事件证据和 Capability 以 system/trusted/untrusted JSON record 编译，按全局及 section Token 预算确定性选择 full/compact/omit，必需上下文超预算时 fail closed；Plan 审计持久化 compiler version、估算 Token、selected/omitted ID 和 provenance，不保存额外上下文正文。ModelShadowPlanner 只接收编译后的 prompt，模型 adapter 与上下文策略保持隔离。
- Agent Runtime 增加 migration v21、独立 `agent_runs` 生命周期与首个受认证远程 Capability：Core 通过 `dipole.agent.v1.AgentCapabilityService` 完成确定性 Task/Run admission、持久 Definition 快照解析和 `conversation.list` 授权，TS Runtime 使用静态生成的 protobuf gRPC client、mTLS `dipole-agent` 身份与 Capability Registry 执行只读 Step。Step 通过 lease/token 持久 claim/result/error，旧 owner 无法覆盖重领结果；Run 完成接口支持网络重试幂等，模型仍不能提交 principal，Agent 身份也无法调用未授权 Core RPC。
- Agent Runtime 增加 migration v20 与持久 Shadow Plan/Step 轨迹：模型输出升级为有序 `steps[]`，每步固定 capability ID 与结构化输入；Task Plan 以 SHA-256 不可变快照保存，Kafka 并发重放幂等收敛，计划漂移 fail closed。MySQL ledger 模式在 Kafka readiness 前探测轨迹表，最小账号仅获两表 `SELECT, INSERT`。
- Agent Runtime 增加 migration v19 与 MySQL ModelAuditStore：Task 唯一绑定持久 Run 和不可变预算快照，ModelRouter 在调用 provider 前事务预留 call slot，16 路并发及跨 Kafka 重投均严格受 `max_calls` 限制；调用完成/失败记录 route、Token、finish reason、latency 和错误，崩溃遗留 slot 在 Run 终止时收敛为 `abandoned`。AI SDK 模式强制持久 Store，Agent 最小账号仅新增两张审计表的 SELECT/INSERT/UPDATE。
- TypeScript Agent Runtime 增加 provider-neutral `ModelRouter` 与 AI SDK 结构化输出 adapter：按有序 route 降级，失败调用计入每 Run 调用上限，总 deadline 与单次输出 Token 上限由 Runtime 强制传递，AI SDK 隐式 retry 关闭；`metadata` 仍为默认模式，模型 plan 仅允许只读 capability 白名单并记录 route/attempt/token usage。
- Agent Runtime 增加有界 Kafka 失败转移：`dipole.<topic>.retry/.dead` 显式创建并校验拓扑，无效 envelope/tombstone 直接死信，处理错误最多尝试三次且保留原始 key/value/header；publisher 失败时 handler 拒绝完成。真实 Kafka 3.9 已验证 poison、retry→dead、offset LAG 归零和双副本 rebalance。
- Agent Runtime 增加 migration v18 与 MySQL EventLedger：Event ID/Task ID 双唯一、事务 claim、lease crash recovery、attempt 和精确 token 终态；微服务默认使用最小权限 `dipole_agent`，真实 MySQL 8.4 与 Kafka 3.9 验证并发单 owner、失败重领、旧 owner 拒绝及 Runtime 重启后重复事件收敛。
- TypeScript Agent Runtime 增加 `message.direct.created` v1 decoder、KafkaJS 独立 shadow consumer、稳定 Task ID 和 EventLedger port；冷启动 metadata 未收敛时会断开旧客户端并有界重连，微服务 Compose 可独立启动只读 Agent，真实 Kafka 3.9 重放同一事件只产生一条 metadata plan。
- 增加独立 `agent-runtime/` TypeScript foundation：Node 22+、Fastify 5、Zod 4、AI SDK 7、KafkaJS 2，提供 trusted ExecutionContext、Go 兼容 Task ID、Capability Registry、resource-scope Policy Engine、shadow 写隔离和 `/livez`/`/readyz`；模型路由与持久审计留待 G2 后续切片。
- Embedded Agent 增加持久执行策略：`ai.policy_mode=persistent` 默认从版本化 Definition 创建确定性 AgentTask、固定并重读精确 policy version，Invocation 携带 permission/resource scope；`static` 保留显式回滚，Task 以 compare-and-set 进入 completed/failed。
- 增加版本化 Web Sync 真实观察 Session/Evidence 与 `web_sync_observation.py`：`start/status/finalize` 将候选版本、完整 Git commit、实际发布 bundle SHA-256、初始/最终 Prometheus 原始响应和 24 小时门禁绑定为不可覆盖证据；窗口不足、候选漂移、告警、差异或溢出均 fail closed，blocked 窗口仍保留审计结果且不会自动切换客户端或 Cassandra 路由。
- 增加 `dipole.agent.policy.persistence.v1`、migration v16 与 sqlc `AgentPolicyStoreV1`：版本化 Definition 保存 permission/scope/有效期/撤销状态，AgentTask 固定 Definition version 与 principal 并以 compare-and-set 迁移状态，Approval 支持 pending→approved、撤销及绑定 capability/canonical scope hash/arguments hash/nonce/有效期的一次性消费；真实 MySQL 8.4 并发测试要求 16 个竞争者仅一个成功。
- 增加 `dipole.agent.command.v1` 跨语言消息写契约与本地 adapter：普通回复和系统 Tool 统一经过 AgentPolicy、Message Service、Kafka/Outbox；sender/target 取自可信 Invocation，Command ID 以固定 SHA-256 canonical form 映射到 64 字符 Message 幂等键，并以黄金向量约束未来 TypeScript 实现。
- 增加 `AgentPolicyV1` 与跨语言 capability descriptors：可信 Invocation 携带 tenant、principal、Agent、delegator、permissions、approvals 和 correlation IDs；read/write 需要显式 permission，destructive/敏感能力需要审批，Embedded Tool 与本地 Capability adapter 双层 fail closed，并以 `AD-027` 跟踪持久授权和审批状态。
- 增加 `ai.runtime_mode=off|embedded|shadow|remote`：未配置时兼容 `ai.enabled`，非法值 fail fast；shadow 保持 Go/Eino 权威并预留 TS 独立 consumer group，remote/off 在模型与 Capability 依赖构造前停止 Embedded consumer，微服务 Core 默认显式 `off`。
- 增加 `dipole.agent.capability.v1` 跨语言契约、Go application port 与本地 adapter；Agent ContextBuilder/Tool 停止持有 User/Message/Conversation repository-shaped 依赖，资料、上下文、会话读取和系统消息统一经过 Core/Conversation/Message 应用边界，写命令保留 correlation context。
- 增加 Embedded Agent `ExecutionContext`：由触发 Message 和 correlation context 注入 principal、Agent、会话及 request/trace/event ID；五个 Tool schema 移除模型可控 `user_uuid`，缺少可信上下文或发送 Agent 不匹配时拒绝执行，并以原 `AD-008` 越权输入持续回归。
- 增加语言中立 `dipole.agent.eval.v1` 与 Go/Eino baseline adapter 测试，覆盖 Agent 事件过滤/幂等、普通与 Tool 回复轨迹、会话读取授权，并将模型身份覆盖行为显式绑定到 `AD-008`。
- 增加 canonical 架构文档 manifest 与 Git 跟踪门禁，移除 `docs/*.md` 通配忽略；历史和本地参考资料改为显式单文件忽略，避免旧文档被误当作当前实现契约。
- 增加 G0 可复现端到端性能门禁与版本化报告，覆盖 direct、concurrent、普通群和热群的接受/持久化/投递率、P50/P95/P99、Kafka lag 及 Inbox 写放大；Compose 热群阈值可在基准期间受控覆盖，默认仍为 `200/50`。
- 增加统一服务健康面：Core、Gateway、Message、Sync、Search、Search Indexer 与 Cassandra Projector 通过 metrics listener 暴露 `/livez`、`/readyz`、兼容 `/health`、`dipole_service_info` 和 `dipole_service_ready`；微服务 Compose 使用 readiness 探针，Prometheus 增加必需服务 down/not-ready 告警及 promtool 时序测试。
- 增加统一关联上下文：HTTP Core/Gateway 生成并回传 `X-Request-ID`、`X-Trace-ID`，gRPC metadata/protobuf、WebSocket 命令与 ACK、Kafka Envelope/headers、consumer handler 和 Transactional Outbox 保持同一 request/trace 因果链；每个领域事件独立生成 `event_id`，旧接口和旧事件仍保持兼容。
- 增加 Group、Conversation Read、Contact 与 Session v1 语言中立事件 schema 和公共 decoder；Gateway 停止自行解码这些 payload，并新增门禁保证所有 `kafkaManagedTopics()` 都有唯一版本化契约。
- 增加 `contracts/events/message/v1` 语言中立事件契约，分别描述 pre-persistence `send_requested` 与 confirmed Message fact；统一 Gateway、Cassandra、Sync、Search、Backfill 和 Replay 的 v1 decoder，并验证 legacy created 默认值、minor additive 字段、事件通道与 producer schema drift。
- 增加真实 Vue 应用共享设备 E2E：在生产默认 IndexedDB 同时保存 U1/U2，通过 Axios 401 和 WebSocket `session.kicked` 两条生产链路终止 U1；Chromium、Firefox、WebKit 均验证凭据与 U1 被清理、U2 Message/Cursor 保留。
- 增加 `scripts/check-web-sync-real-quota.sh`：在无特权 128 MiB tmpfs 中运行独立 Chromium profile，以随机不可压缩正文触发真实 IndexedDB 容量拒绝；释放 reserve file 后验证失败页未推进安全 Cursor，Message 与 manifest 保持一致。
- Web IndexedDB 验收增加真实 Chromium 主进程强退场景：独立 persistent profile 在生产 `commitPage` 仍 pending 时触发 `Browser.crash`，以同一 profile 重启后验证 Message、manifest 与安全 Cursor 只能整页提交或整页回滚。
- 增加版本化 `sync.item.notify.v1` WebSocket 轻量通知，固定只携带 event、Message UUID、会话 key/Seq 和目标 locator；`message.timeline_notify_mode=off|shadow` 默认关闭，shadow 保留现有完整消息投递并附加通知，热群继续只使用聚合 notify + pull。
- Web 增加默认关闭的 Timeline notification shadow verifier，按会话串行使用 `after_seq` 补拉，覆盖通知丢失、重复、乱序、缺行和 UUID 冲突；仅上报 `match|missing|mismatch|error|invalid` 有界聚合指标，并配套 24 小时、至少 100 次 match、零失败的 Prometheus 晋级门禁。
- Direct Message Timeline 增加 `after_seq` 增量查询，HTTP、Message v1 gRPC、Local/Remote/Shadow adapters 与 Cassandra cohort 路由保持同一语义；字段以 protobuf 追加方式兼容旧调用方，为在线 `sync.item.notify` 拉取路径提供前置契约。
- 增加 Playwright Chromium/Firefox/WebKit IndexedDB 验收，直接运行生产 Store 与 Session Terminator，覆盖容量淘汰、关闭重开、账号隔离、延迟清理和页面中断事务原子性。
- Web Sync 聚合遥测增加 `dipole_web_sync_client_errors_total{outcome}`，仅允许 `storage_full|sync_error`，并新增浏览器存储不足、客户端恢复错误告警及 promtool 固定时序测试。
- 增加 `message.mysql.*` 专用数据库配置、atomic/projector 两套最小权限账号与操作级启动门禁；微服务 Compose 在 migration 后初始化授权，并默认使用专用 Message/Sync 凭据。
- 增加 `dipole-cassandra-archive` 与不可变完整消息快照：按固定 MySQL Message 高水位导出完整字段 NDJSON、SHA-256 manifest，并支持 MinIO object-lock 发布、固定 Version ID 恢复。
- Cassandra Backfill/Reconcile 增加 `mysql|archive` source selector；migration v15 将 Job 绑定到 source kind、snapshot ID 与 hash，同名 Job 续跑、完成后复核均拒绝换源或篡改。
- Storage 配置增加 `message_archive_bucket` 与 `message_archive_retention_days`，Compose 初始化独立 `dipole-message-archives` bucket、版本控制和 30 天默认 Governance 保留。
- 增加默认关闭的 `message.cassandra_duplicate_hydration`：重复发送先用 Metadata locator 从 Cassandra Timeline 恢复原响应，命中时不读取 MySQL 正文；缺行、查询错误、位置冲突或历史 Seq 缺失时回退 MySQL。
- 增加 `dipole_message_duplicate_hydration_total{outcome}`，预注册 `hit|fallback|skipped_no_seq` 三种有界结果，用于评估幂等响应正文退役条件。
- 增加重复消息 Cassandra hydration 的 24 小时 recording rules、fallback/no-seq 告警、promtool 固定时序测试与灰度手册；晋级要求至少 100 次 hit 且 fallback/no-seq 均为零。
- 增加 `dipole-search-outbox-cleanup` 受控清理命令：默认 dry-run，只接受已验证对象归档 receipt、一致 Reconcile 报告和匹配 Backfill Job；执行模式强制维护窗口确认与 operator，并输出对象版本、高水位和删除统计审计字段。
- 发布镜像增加 `dipole-search-archive` 与 `dipole-search-outbox-cleanup` 运维二进制，归档恢复和受控清理可直接在同一版本镜像中执行。
- 增加 `search.mysql.*` 专用维护连接与最小授权模板；账号仅可访问 migration ledger、Search Backfill Job 和 Outbox，Core 业务表保持拒绝。
- `dipole-search-archive` 增加 `publish|restore`：归档发布到启用 versioning/object lock 的独立 MinIO bucket，receipt 固定 object version ID、ETag 和 Governance 保留截止时间；恢复只读取指定版本并重新校验 hash。
- Storage 配置增加 `search_archive_bucket` 与 `search_archive_retention_days`，Compose 初始化独立 `dipole-search-archives` bucket、版本控制和 30 天默认保留。
- 增加 `dipole-search-archive` 与不可变 Search snapshot：按固定 Outbox mutation 高水位流式导出最终状态 NDJSON 和 SHA-256 manifest；Backfill、Reconcile、Alias 可统一选择 `mysql|archive` 源。
- 增加 migration v13，将 Search Backfill Job 绑定到 source kind、snapshot ID 与 hash；恢复、对账和 Alias 操作拒绝换源、篡改归档或高水位不一致。
- 增加 migration v12 `message_metadata`：在 Message、Inbox 与 Outbox 同一事务保存幂等 locator、会话 Seq、发送目标、文件绑定、过期时间和版本化 payload SHA-256；历史消息回填 locator，旧 payload hash 明确为空。
- 重复发送优先按 Metadata 的 `(sender_uuid, client_message_id)` 定位并校验目标与 payload hash；文件下载/内容授权改查 Metadata，完整 `messages` 行删除后仍可执行访问判断。
- Web 热群恢复增加独立 `GroupMessageSyncEngine` 与 IndexedDB v3 群位点：补拉消息和 `message_seq` 原子落库后才展示并 ACK 设备群 checkpoint；刷新后优先恢复本地群消息，并可补交丢失的 ACK。
- Web IndexedDB 升级到 v2，增加逐用户缓存 manifest、5000/4000 默认高低水位和按会话保底的最近消息淘汰；超量页面在同一事务内压缩到低水位，同时保留完整安全 `sync_seq`。
- Web Sync 增加 `storage_full` 状态和 Pencil 批准预览；浏览器拒绝持久化时提示释放空间后重试，未落库页面不会确认设备 Cursor。
- 增加默认关闭的 Web Sync Engine：按用户将 Sync 消息与安全 `sync_seq` 原子写入 IndexedDB，启动和 WebSocket 重连时从本地恢复并分页追平 `/sync`，仅在本地事务完成后确认设备 Cursor。
- 增加同步恢复的 Pencil 状态矩阵、desktop/mobile 恢复页面和 Vue 标题栏状态，覆盖恢复中、已同步、离线可读与可重试中断语义。
- 增加 Web `off|shadow|primary` 同步模式；shadow 保持旧 Offline 驱动界面，同时持久化 `/sync` 并按收到的私聊 Message UUID 做带宽限期的有界对照。
- 增加认证聚合遥测 `POST /api/v1/sync/comparison` 与 `dipole_web_sync_comparison_total{scope,outcome}`，只接收 baseline、match、pending、legacy_only、sync_only、overflow 计数，不接收消息 ID 或正文。
- 增加 Web Sync 24 小时 rolling recording rules、终态差异/比较器溢出 critical 告警和 promtool 固定时序测试；六种 outcome 启动时暴露零值并拒绝未知标签，晋级门槛为至少 100 个 match、零 `legacy_only/sync_only` 与零 overflow。
- 增加 Web Sync 灰度与观测手册，明确 Core metrics 抓取前提、停止条件、真实证据归档、primary 晋级和无数据回切步骤。
- 增加 Sync 专用 MySQL 配置覆盖、`dipole_sync` 最小授权模板和启动权限门禁，精确验证 Inbox/Checkpoint/恢复表读写及 Message/Core 越权拒绝。
- 增加 `message.inbox_write_mode=atomic|projector`；独立 Message owner 可停止 Inbox 写入并保留 Message、Conversation Seq、群高水位与 Transactional Outbox 原子提交，模块化单体继续固定使用 atomic 路径。
- 增加 `dipole_message_projector` 最小授权模板、Sync mTLS 证书身份和 Compose 运行时定义。
- 增加 Sync Projector 专用 Prometheus lag、retry、DLQ 告警及 promtool 时序测试，精确绑定 `dipole-sync-consumer` 与 created-event 失败 topic。
- 增加 `dipole-sync-baseline` 运维命令与 migration v11，按固定 Inbox `sync_seq` 高水位归档所有缺少 created Outbox 的历史 recipient/locator，保存规范化 SHA-256，并支持精确 Reconcile 与 missing-only Restore。
- Sync 历史基线恢复保留原始 `sync_seq`，发现新增 legacy 行、recipient/locator 偏移或 Cursor 序号冲突时拒绝自动修复，避免按当前群成员关系推断早期收件人。
- 增加真实 MySQL 8.4 Sync baseline smoke，覆盖重复 Capture、差异退出码 2、原 Cursor 恢复、冲突退出码 1 和最终收敛。
- 增加默认关闭的 `sync.projector_enabled`；`dipole-sync` 可使用独立 Kafka consumer group 将私聊和普通群 `message.created` 物化到 Durable Inbox，热群继续遵循 `sync_fanout=false` 的 notify + pull 路径。
- 增加 storage-neutral `SyncProjectionStore` 与 sqlc 事务适配器，按固定用户顺序锁定 Sync state，并以 `message_uuid + conversation_key + message_seq` 实现双运行精确重放和冲突回滚。
- 增加真实 Kafka 三节点与 MySQL 8.4 Sync Projector smoke，覆盖 Message 预写、重复事件收敛和热群无 Inbox 写扩散。
- 增加 `dipole-sync-replay` 与 `dipole-sync-reconcile` 运维命令，以 created Outbox 最大 ID 固定恢复快照，使用 lease/checkpoint 分批重放，并精确核对每条消息的收件人和 locator 集合。
- 增加 migration v10 `sync_replay_jobs`、机器可读一致性报告和退出码 2 差异门禁；真实 MySQL 8.4 演练覆盖部分状态补齐、completed no-op、删行检测和新 job 修复。
- 增加独立 `cmd/sync-service` 查询运行时、`dipole-sync` 内部身份和构建产物，进程只组合 sqlc Sync Store、MySQL schema readiness、Core 群成员授权与 Sync v1 gRPC。
- Core 内部 RPC 为 Sync 身份增加方法级最小权限，`dipole-sync` 只能读取群成员关系；Core/Gateway 可通过绑定自身身份的客户端调用 Sync API。
- 增加默认 `local` 的 `sync.transport=local|grpc` 切流开关；Core HTTP 可通过受认证 gRPC 使用独立 Sync Service，并保留进程内即时回滚路径。
- 增加默认关闭的 `sync.shadow_queries`，异步比较 Inbox、设备 Cursor 和群 checkpoint 只读结果；两类 checkpoint advance 始终只调用选定主实现一次。
- Sync Item 增加存储中立的 `conversation_key + message_uuid + message_seq` 定位契约，HTTP 与 Sync v1 gRPC 以追加字段暴露 locator，同时保留完整 Message 快照兼容旧客户端。
- 增加默认关闭的 `sync.cassandra_shadow_hydration`；Sync Service 继续由 MySQL 补全并返回消息正文，同时按 Inbox locator 异步读取 Cassandra Timeline，记录 match、mismatch、error 和容量跳过结果。
- 增加 `dipole_sync_hydration_shadow_total{outcome}` 与 `dipole_sync_hydration_shadow_duration_seconds{outcome}`，用于评估 Sync 正文补全切换条件；异步比较上限固定为 32，Cassandra 异常不增加主响应等待。
- `user_sync_inbox` 增加会话 Seq 回填和 `(user_uuid, conversation_key, message_seq)` 唯一约束；相同消息或位置的冲突重放会失败，正确重放保持幂等。
- 增加 Vue 消息搜索工作区，支持 desktop/mobile 的结果、加载、空态和局部故障态，以及会话入口、`Cmd/Ctrl+K`、300ms 防抖、乱序响应淘汰和重试。
- 增加 Vitest、Vue Test Utils 与 jsdom 前端测试基线，首批覆盖 Search 状态控制器和工作区交互。
- 增加 canonical Pencil 设计基线，包含消息搜索 desktop/mobile 的结果、加载、空态、错误态和可复用组件，并提供批准预览与持续维护说明。
- 增加 transport-neutral `MessageApplication`、`SyncApplication`、`CoreCapability` 和 `EventPublisher` 端口、单体 Local adapter 及数据层依赖架构测试。
- 增加用户同步 Inbox Timeline：通过 `user_sync_inbox` 按用户维护持久化 `sync_seq`，支持离线和多端增量同步。
- 增加 `GET /api/v1/sync` 接口，支持 `after_seq` 游标、分页上限、`next_seq` 和 `has_more`。
- 消息持久化、Inbox 写入与 Transactional Outbox 进入同一数据库事务，避免消息事实与同步状态分离提交。
- 增加同步链路的 repository、service 和 HTTP handler 测试，并覆盖私聊、普通群聊、热群和分页场景。
- 增加分阶段平台演进计划与架构债务台账，明确微服务、存储架构和 Agent Runtime 的分支、验收及回滚策略。
- 增加 TypeScript Agent Runtime 设计，明确 Durable Task、Capability、Context、Memory、MCP、评测和渐进迁移方案。
- 增加 GORM 到 sqlc 的渐进迁移计划，以及基于 Pencil `.pen` 的前端设计与视觉回归维护规范。
- 增加版本化 MySQL migration、独立 `cmd/migrate` runner、schema ledger 与真实 MySQL drift 测试。
- 增加固定 sqlc `v1.31.1` 的生成配置与漂移门禁、`database/sql` 事务 Store、错误映射及首组 AICallLog 类型安全查询。
- 增加 `AICallLogStore` application port、GORM 可注入 adapter、sqlc adapter 及共享 MySQL contract test。
- 增加 `AdminOverviewStore` application port 与 Admin sqlc adapter，以单条聚合查询替代九次独立统计查询，并通过 GORM/sqlc 共享契约验证。
- 增加 `FileMetadataStore` application port 与 File sqlc adapter，保持创建后的 ID/时间戳回填和缺失查询语义，并纳入统一回切开关。
- 增加 `UserStore` application port、共享 Redis/Bloom 缓存装饰器与 User sqlc adapter，覆盖创建、助手 upsert、资料更新、筛选和批量查询。
- 增加 `ContactStore` application port、共享关系缓存装饰器与 Contact sqlc adapter，覆盖双向好友关系和联系人申请生命周期。
- 增加 `GroupStore` application port、共享元数据/成员缓存装饰器与事务型 Group sqlc adapter。
- 增加 `ConversationStore` application port 与 Conversation sqlc adapter，覆盖投影 upsert、列表、初始化、备注和未读状态。
- 增加 `OutboxRelayStore` application port 与事务型 sqlc adapter，覆盖有序批量领取、过期租约回收、重试退避、发布终态和 Header 解码。
- 增加 `MessageStore`、`SyncStore` application port 与 sqlc adapters，完整覆盖 Message Store 查询、Inbox Timeline 读取和 Message/Inbox/Outbox 原子写入。
- 增加 `dipole.message.v1.MessageService` protobuf/gRPC 契约、固定版本生成门禁、结构化领域错误详情，以及实现同一 `MessageApplication` 的本地 server 与远程 client adapters。
- 增加共享 `dipole.common.v1.RequestContext`、Core Capability 与 Sync Query v1 契约，以及实现现有 application ports 的 Local server/Remote client adapters。
- 增加 Kafka schema version 兼容校验与事件契约，legacy 空版本和 v1 minor 保持兼容，未知主版本跳过业务 Handler 并直接进入 DLQ。
- 增加内部 gRPC 服务身份认证基元，客户端注入服务名与共享凭据，服务端以常量时间比较校验凭据并执行调用方 allowlist。
- 增加独立 `cmd/message-service` 运行时，拥有 Message RPC、消息持久化 Kafka consumer、Transactional Outbox Relay 和远程 Core Capability client。
- Core Capability v1 增加最小文件所有权快照，文件消息通过受认证 RPC 获取 File ID、名称、大小、类型和 URL，不暴露对象存储内部键。
- 增加仅组合 Message 与 Outbox adapters 的 `MessageProcessRepositories`，独立进程停止构造 User、Contact、Group、File、Conversation、Admin 和 AI Repository。
- 增加 `message.shadow_queries` 查询影子开关，可在 Local/Remote 任一方向异步比较历史、群增量和离线消息结果。
- 增加 `message.runtime_mode=shadow|owner`；shadow 进程拒绝全部命令且不启动 Kafka/Outbox，owner 进程承担完整消息写入职责。
- 内部 RPC 支持 TLS 1.3 双向证书认证，并将认证服务身份绑定到 protobuf `caller_service`；明文 listener/target 强制限制在 loopback。
- 增加 `message.enforce_db_permissions` 启动验收和 Message MySQL 最小授权模板，检查自有表可访问且 Core 表全部拒绝。
- 增加 Message Service 渐进部署手册，覆盖影子阶段、consumer group 交接、mTLS、验收和无数据回滚流程。
- 增加可滚动维护的性能基线，首组记录 Local 与 TLS 1.3 mTLS gRPC Message History adapter 的三轮 benchmark。
- 增加独立 `cmd/gateway`，承担公开 HTTP/WS、认证上下文、限流、连接管理、Redis Presence 与 Kafka Realtime Delivery，运行时不初始化 MySQL 或 Repository。
- 增加 IM Gateway 渐进部署手册，覆盖三进程边界、mTLS 身份、灰度验收和 `embedded` 无数据回滚。
- 增加最小微服务开发拓扑：统一镜像打包 Core、Message、Gateway 与 migration，Compose 默认启用内部 mTLS 和依赖健康门禁。
- 增加内部开发 CA/三服务证书生成脚本及可自动清理的微服务 cold-start smoke。
- 增加会话内单调 `message_seq` 与 `conversation_sequences` allocator；消息唯一 ID 继续承担全局身份，Seq 专门承担会话内排序。
- Message v1 protobuf、Kafka created event 和 WS chat payload 以追加字段暴露会话 Seq，旧 producer/client 缺字段时保持兼容。
- 增加 Conversation `last_message_seq/read_seq`、设备级 Sync checkpoint 查询与显式 ACK API，并将 checkpoint RPC 加入 Sync v1 契约。
- Web 客户端生成持久稳定的设备 ID，同时用于 HTTP `X-Device-ID` 与 WebSocket Presence 身份。
- 增加存储中立的会话 Seq 历史查询与 `SearchIndex` 契约，MySQL 搜索投影实现支持幂等更新、删除和限定会话范围检索。
- 增加热群持久高水位、设备群 checkpoint 和 `after_seq` 补拉；Web 重连后可发现并追平离线期间跳过用户 Inbox 的热群消息。
- 增加消息 mutation 事件契约；created Outbox 固化 `mutation_type/revision/actor_uuid`，并为未来 edited/recalled/deleted 预留单调 revision 语义。
- 增加三节点 Kafka KRaft cluster profile、显式 Topic min ISR/retention、可配置 producer ACK 策略与自动清理的 quorum 故障 smoke。
- 增加显式 Kafka consumer rebalance policy、处理/提交/retry/DLQ snapshot，以及双 member 到单 member 的 partition 接管故障 smoke。
- 增加进程级 Kafka Prometheus Collector、独立 metrics listener、Kafka exporter、Prometheus 告警规则与自动故障 smoke，覆盖 lag、ISR、retry 和 DLQ。
- 增加 MySQL 8.4 三成员 InnoDB Cluster、MySQL Router writer endpoint、AdminAPI 初始化/恢复脚本与连接池主切换故障 smoke。
- 增加 Redis Sentinel 连接模式、三节点 Redis/三 Sentinel 隔离拓扑与自动故障 smoke。
- 增加 Cassandra 5.0.9 与 Elasticsearch 9.5.2 隔离 Storage Lab、资源基线与自动 CRUD smoke。
- 增加 Cassandra Conversation Timeline 版本化 CQL、10,000 Seq bucket 规则和 LWT 幂等投影 primitive。
- 增加默认关闭的独立 Cassandra Projector Runtime、专属 Kafka consumer group、schema readiness 与端到端 smoke。
- 增加独立 Cassandra 历史 Backfill、固定 MySQL 高水位、owner lease、批次 checkpoint 和失败恢复烟测。
- 增加独立 Cassandra Reconciler，以 JSON 报告固定快照的数量、全量 payload hash、确定性字段样本和会话 Seq 连续性；确认差异时返回退出码 2。
- 增加默认关闭的 `message.cassandra_shadow_reads`，Message Service 可异步比较 MySQL 历史页与 Cassandra Timeline，客户端响应继续取自 MySQL。
- Cassandra Timeline 增加跨 bucket Seq 区间读取，并按升序合并结果供影子比较使用。
- 增加 `message.cassandra_read_percentage`，按会话稳定 cohort 将群 `after_seq` 增量读取灰度到 Cassandra，异常或 Seq 缺口自动回退 MySQL。
- 增加 Cassandra 读路由 Prometheus 请求计数与延迟直方图，以及真实 MySQL/Cassandra 缺行回退 smoke。
- Direct/Group 历史增加 `before_seq` HTTP 与 Message v1 RPC 游标，支持按会话 Seq 获取最新页和向前分页。
- Cassandra cohort 主读扩展到 Direct/Group `before_seq`，连续性校验失败时按同一游标整页回退 MySQL。
- 增加 `message.cassandra_read_verify_percentage` 主读抽样核验；按同一 Seq cursor 比较 MySQL 公开字段，payload mismatch 自动整页回退。
- 增加 `dipole_message_read_verification_total{operation,outcome}`，区分主读核验 match、mismatch 与 MySQL error。
- 增加 Cassandra 主读 Prometheus 告警与 promtool 规则测试，覆盖 payload mismatch、核验依赖失败和持续高 fallback 比例。
- 增加 MySQL 消息正文退役门禁，覆盖 Sync 补全、幂等回放、文件授权、搜索重建、备份与事件回放责任。
- 增加 Elasticsearch `dipole-messages-v1` strict mapping、read/write Alias、schema readiness 与原子 Alias 切换契约。
- 增加 Elasticsearch Search adapter，以 Message UUID、external revision 和 payload hash 识别幂等重放、旧事件与同版本冲突，并强制 conversation scope 查询。
- 增加版本化 `SearchIndex.Apply` 与持久 tombstone，MySQL/Elasticsearch 共享更高 revision 覆盖、旧 revision no-op、相同重放和同版本冲突语义。
- 增加默认关闭的独立 `cmd/search-indexer`、Elasticsearch 配置/认证、专属 Kafka consumer group 与八类 direct/group mutation Topic 投影。
- 增加 Outbox 固定快照 Search Backfill、目标索引绑定的 owner lease/checkpoint、最终 mutation 状态折叠与独立 Reconcile JSON 报告。
- 增加显式 Elasticsearch 物理构建目标；维护写入不绑定生产 Alias，在线 Indexer 继续强制 `require_alias=true`。
- Core Capability v1 增加由认证 principal 派生的 Search 会话范围；调用方无法提交 user ID 或 conversation keys，陈旧群会话投影无法绕过成员关系校验。
- 增加独立 `cmd/search-service`、`dipole.search.v1.SearchService` 与只读 Elasticsearch readiness；查询进程通过 Core scope 限定结果且不初始化 MySQL、Redis 或 Kafka。
- 微服务 Compose 增加可选 `search` profile、`dipole-search` mTLS 身份及 Search Service 隔离存储契约。
- Core 内部 RPC 增加 Search 身份的方法级最小权限，`dipole-search` 只能读取 Search scope，其他 Core capability 被拒绝。
- Storage Lab 架构门禁收窄为检查 Core、Message、Gateway 直连，允许独立 Search 运行时使用 Elasticsearch。
- 增加默认关闭的 Gateway 认证搜索 API `GET /api/v1/messages/search`；principal 来自 JWT 会话，查询参数只包含 `q` 与 1..100 的 `limit`。
- 增加 `dipole-search-alias` 受控切换/回滚命令，要求维护窗口确认、新鲜快照三重检查、现场 Reconcile、Alias owner CAS 与切换后自动补偿。

### 变更

- 前端开发工具链升级到 Vite 8.2.2、Vitest 4.1.11、plugin-vue 6.0.8 和 Rolldown 1.2.6，并固定 Node 22.12+ LTS；配置改用 `import.meta.dirname`，隔离测试可通过 `DIPOLE_WEB_PROXY_TARGET` 覆盖 HTTP/WS 代理目标。
- 用户状态固定为语言中立 `dipole.user.status.v1`：`normal=1`、`disabled=2`，Go 领域常量改为显式值，避免其他常量或多语言实现改变持久化语义。
- Web 会话终止统一覆盖显式退出、HTTP 401、WS kick 和账号切换：先撤销本地凭据，再等待在途 Sync 收敛并清理该用户 IndexedDB；并发终止复用 singleflight，快速重登等待旧清理完成。

- Web 同步以 `message_uuid + message_seq + sync_seq` 作为本地身份、排序和恢复契约；MySQL 自增 `id` 继续只服务旧 Offline 兼容路径。
- A6 将 Durable Inbox 新增写入责任切换到 Sync Projector；默认 `atomic` 仍是即时回滚路径，切换需同时启用 Sync Projector、最小权限账号和启动权限验收。
- `dipole-sync` 的新 Kafka consumer group 从 earliest retained offset 建立，已有 group 继续使用已提交 offset；Outbox Replay 与历史 baseline 负责覆盖 Kafka retention 之外的数据。
- A6 首个切片将 Durable Inbox、设备 Cursor 和群 checkpoint 的查询所有权抽入 Sync Service；Message 事务继续原子写入 Inbox，事件消费投影将在具备回放与一致性门禁后切换。
- 普通群消息按成员写入 Inbox；热群沿用 notify + pull，跳过成员级 Inbox 写扩散。
- HTTP、Kafka 与 Agent 启动路径通过统一 Composition Root 创建 Repository 与消息域 Service，消除进程内重复实例和分散的具体依赖构造。
- Runtime 在 HTTP、Kafka、Outbox 和 AI 助手初始化之间复用同一 sqlc Repository 集合，所有构造入口必须显式提供 `*sql.DB`。
- Kafka Topic 初始化显式覆盖主 Topic、`.retry` 与 `.dead`；Publisher 和 Transactional Outbox 同时写入 `version` 与 `schema_version` headers。
- Kafka 单节点默认继续使用 RF=1/min ISR=1/acks=one；cluster profile 使用 RF=3/min ISR=2/acks=all，低于 quorum 时拒绝确认写入。
- 增加 `message.transport=local|grpc` Composition Root 开关；默认 Local，grpc 模式通过进程内 channel 执行同一 MessageApplication 契约并支持安全回切。
- Messaging Composition Root 支持注入远程兼容的 `CoreCapability`，为独立 Message Service 停止直接读取 Core Repository 建立切换边界。
- `message.transport=grpc` 升级为受认证的真实 TCP channel；Core 先开放 Capability listener 再连接 Message，避免冷启动循环等待，`local` 继续作为默认回切路径。
- Kafka handler 按 Core 与 Message 进程拆分；远程模式下 Core 停止消费 `message.*.send_requested` 并停止运行 Outbox Relay。
- 本地与远程 MessageService 统一通过 Core Capability 校验文件所有权；非所有者与缺失文件返回相同不可用语义。
- Outbox Relay 停止时等待 worker 退出，避免 Kafka publisher 关闭后仍有并发发布。
- Shadow comparison 只比较 Message v1 对外字段，忽略 `CreatedAt/UpdatedAt` 等内部字段，并按时间瞬时语义处理时区差异。
- Cassandra 存储影子比较限制为 32 个并发任务；容量耗尽、空页、主查询失败或无效 Seq 仅记录跳过原因，不增加主查询等待时间。
- Cassandra 主读首批仅覆盖 Seq cursor；MySQL `id` cursor、Offline Inbox 和写链路保持原存储职责，百分比设为 0 可立即回切。
- MySQL 完整消息写入保留到 A5/A6 替代读契约双跑完成；`metadata_only` 写模式仅作为门禁后的未来开关，不在 A4 实现。
- Web 历史首屏和加载更多统一使用 `before_seq`，热群在线补拉使用 `after_seq`；消息 UUID 负责去重，Seq 负责排序和分页。
- Runtime 创建的同一套 Messaging Services 现在注入 Server 与 Kafka handlers，消除 grpc 模式下重复的 Local MessageService 和 singleflight 实例。
- 增加 `gateway.mode=embedded|remote`；默认保持单体行为，`remote` 时 Core 停止注册 WS 路由和实时投递 handler，其余 HTTP/Swagger/静态 Web 由 Gateway 代理到私网 Core。
- Kafka 实时投递 handler 与 Core 领域投影拆分，独立 Gateway 固定使用 `dipole-gateway-consumer`，避免与 Core 和 Message 的消费职责竞争。
- Core 与 Gateway 调用 Message RPC 时分别声明 `dipole-core`、`dipole-gateway`，内部证书和审计主体保持独立。
- 服务启动只读校验 migration 版本，已移除运行时 schema mutation 和 `AutoMigrate` 配置。
- Message、Sync Inbox、Conversation Seq allocator 与 Transactional Outbox 在同一 MySQL 事务中提交；Outbox payload 在 Seq 分配后构造，保证事件与消息事实一致。
- Migration Up/Down 使用 schema-scoped MySQL advisory lock，多个 migration owner 并发启动时按锁串行执行并重新读取 ledger。
- Redis 单节点配置保持兼容；Sentinel 模式通过 go-redis Failover Client 自动发现当前 master，业务组件继续共享同一客户端。
- Kafka Topic 创建后使用有界 metadata 收敛重试，避免冷启动期间短暂的 `Unknown Topic Or Partition` 造成服务退出。
- Conversation 投影按 Seq 拒绝重复和过期消息回退；`unread_count` 继续作为兼容投影，由 `last_message_seq - read_seq` 语义维护。
- Composition Root 统一使用 sqlc，已移除 `data.mysql_adapter` 兼容开关和 legacy GORM adapters。
- User Repository 的 Redis/Bloom 策略从数据库适配器中抽离，由 GORM 与 sqlc 后端共享同一缓存装饰器。
- Contact Repository 的 Redis 关系缓存从数据库适配器中抽离，由 GORM 与 sqlc 后端共享同一缓存装饰器。
- Group Repository 的 Redis/Bloom 与成员排序策略从数据库适配器中抽离，由 GORM 与 sqlc 后端共享同一缓存装饰器。
- Conversation 的消息预览规则收敛到 domain model，GORM 与 sqlc 投影复用同一文本、文件、AI 和系统消息摘要语义。
- Eino 从 `v0.8.8` 升级至 `v0.9.15`，`eino-ext/components/model/openai` 从 `v0.1.12` 升级至 `v0.1.13`。
- 更新 OpenAPI/Swagger 文档，加入同步接口及其请求、响应模型。

### 修复

- migration v17 将 Agent Definition、Task 与 Approval 的身份列从 20 字符 expand-only 扩至 24 字符，修复默认 21 字符 `assistant_uuid` 无法初始化 persistent policy 的启动失败。
- HTTP Handler 测试在包级 `TestMain` 统一初始化 Gin TestMode，移除并行测试中的重复全局写入，使整包 `go test -race ./internal/handler/http` 可作为稳定门禁。
- 修正 MySQL migration 集成测试仍将已存在的 v10 当作未来版本的问题，并将真实上下迁移与并发 owner 门禁推进到 v11。

- 修复 WS 暂态回声先占用 `message_id` 时，后续持久化历史消息无法回填正 `message_seq` 的问题；消息合并现在优先保留持久化等级更高的版本。
- 修复并发消息事务可能造成同一用户 Sync Cursor 永久跳过迟提交消息的问题，Inbox 写入现在按用户锁行串行化。
- 修复旧群消息 Kafka 事件缺少 `sync_fanout` 时被误判为热群的问题，滚动部署期间默认保留普通群 Inbox fanout。
- 修复幂等冲突复用已有消息时可能沿用新目标收件人的问题，路由身份不一致时拒绝 Outbox/Inbox 修复。
- 修复重复创建已存在好友关系时可能用新建默认值覆盖缓存、造成缓存状态与数据库状态暂时不一致的问题；建交成功后统一失效双向缓存。
- 修复群成员追加按输入切片长度递增 `member_count` 导致重复或空成员虚增的问题，改为按数据库实际插入行数计数。
- 修复 MySQL 8.4 下同一批次重复追加新成员可能触发 `Error 1869` 的问题，追加入口先按群和用户去重。
- 固定 Conversation upsert 的 SQL 赋值顺序，先基于旧 `last_message_uuid` 计算未读，再更新最新消息字段，避免依赖 GORM map 排序。

### 安全

- MCP request budget 独立于旧 `rate_limit.enabled`，防止关闭传统业务限流时同时移除模型驱动入口门禁；JWT principal 是唯一计数身份，客户端无法通过请求正文或更换 Task/Run 选择限流键。
- MCP Tool 参数、结果和内部异常正文禁止进入数据库审计及 span 属性；Core 重新解析运行中的 Task/Run 和固定 Definition，只允许 read-risk Capability，Runtime 无法提交 principal。重复 begin、重复终态、伪造 principal 与审计失效均拒绝执行或返回稳定泛化错误。
- MCP Gateway 代理移除公开 Authorization、Cookie、客户端注入的 `X-Dipole-*` 身份头和 hop-by-hop/body 长度头，再注入固定 `dipole-gateway` 服务凭据与 JWT principal；Core 对外部 principal 不匹配统一返回 unavailable，模型和 Tool 参数无法选择 tenant、Agent、Task 权限或 resource scope。
- 移除 Vite 5/esbuild 与 Vitest 2 开发依赖链中的 1 个 critical、1 个 high 和 3 个 moderate 公告；完整依赖及生产依赖 `npm audit` 均为零漏洞。
- 新增第三个离线 `dipoleartifactmaintenance` 检查身份，MinIO policy 仅允许固定 Artifact 前缀 GetObject 证据权限；Go adapter 只暴露 Stat，配置拒绝复用 Runtime 或 audit access key。该身份无法 List、Put、Delete，Core、Agent 与 Gateway 均不持有其凭据。
- 新增 `dipoleartifactaudit` 身份，policy 只有专用 bucket 的定位和 List 权限；CLI 显式拒绝复用 Artifact Runtime access key，Core、TS Agent 和公开 Gateway 均不接收 audit 凭据。候选发现不授予 Get/Put/Delete，未来清理身份继续与 audit/Runtime 分离。
- Agent Artifact 改用独立 `dipole-agent-artifacts` bucket 与专用 Core 身份，policy 仅允许 bucket 定位/列举和 `agent-artifacts/v1/*` 的 Get/Put，明确不授予删除权限；通用文件/归档身份同步从 MinIO root 降为限定三个既有 bucket 的 `dipoleplatform` 用户，两个运行时身份无法跨 bucket 写入。TS Agent Runtime 不接收对象存储凭据。
- Message atomic/projector 账号仅保留 sqlc 实际使用的表操作；显式拒绝 Message UPDATE/DELETE、Outbox DELETE、migration 写入、Core 表访问，以及 projector 模式的 Inbox 权限，关闭 AD-015。
- Sync 与 projector-mode Message 账号按表和操作拆分；启动时拒绝 Sync 修改 Message/Outbox/群高水位或读取 Core 数据，也拒绝 Message projector 访问 Inbox 状态。
- 更新前端生产依赖锁定版本，修复 Axios、form-data、nanoid 和 PostCSS 的高危公告；生产依赖审计恢复为零漏洞。

### 移除

- 移除 Message Service 测试替身中遗留的 `Create`、`StoreWithOutbox` 兼容写入口，并以方法集测试固定生产 `MessageRepository` 仅暴露 Sync-aware 原子写契约。
- 移除 legacy GORM repositories、model persistence tags、运行时 `AutoMigrate`、SQLite 方言测试以及 `gorm.io/*` 依赖。
- 移除 `data.mysql_adapter`、`mysql.auto_migrate` 和无依赖 Repository/Server/Kafka 便捷构造入口。

### 迁移说明

- `ResolveApprovalGrant` 是 additive Agent Capability RPC，无数据迁移；先滚动 Core 和 sqlc 查询，再发布 TS Runtime。查询只在持久 active Run 中返回唯一 exact grant，旧 Runtime 不调用。回滚先保持生产 write Tool 未注册，再回退 Runtime/Core。该 RPC 可用不代表 active context 已晋级。
- MCP Message write projection 没有数据迁移或生产开关。`createDipoleMcpServer` 只有同时收到 active ExecutionContext、审批必需的 write descriptor、Command kind 和显式 executor 才注册写 Tool；当前启动链不提供这些条件。未来启用前还需完成 active authority 晋级、UI 风险摘要，并演练 Command RPC 超时后通过 receipt 收敛 action lineage。
- `ExecuteMcpMessageCommand` 是 additive Agent Capability RPC，无数据迁移。先滚动 Core，再发布 TS Runtime；旧 Runtime 不调用该方法。回滚时先保持生产 MCP write Tool 未注册或关闭，再回退 Runtime/Core。该 RPC 依赖 migration v31 的 Tool Invocation Approval 字段，不能在 v31 之前启用。
- migration v31 为 `agent_tool_invocations` 增加 nullable Approval 与 action reference 字段及约束。先迁移 Core，再发布 additive Agent Proto 与 Runtime；旧只读调用继续写入空引用。回滚前必须关闭所有未来 write Tool、等待 `running` 调用收敛并确认不再需要 Approval/Command/Message 联查证据，v31 Down 会移除这些字段和索引。
- `ConsumeApproval` 是一次性、active-only 的安全门槛。审批在 Capability 执行前消费，执行失败后不会自动恢复或重放；调用方必须创建新审批。Message Command 已具备稳定业务幂等键和 sender-scoped receipt 查询，可收敛 RPC 超时后的提交状态；migration v31 已提供 Tool-to-Message lineage，生产 write projection 仍需 active authority、UI 风险摘要和故障演练。当前不要向 `createDipoleMcpServer` 注册 write/destructive Tool。
- 外部 MCP Client 的原始 `CallToolResult` 不得直接拼接到 system/trusted prompt、Memory 或 Agent instruction。未来接入必须先通过 `externalMcpResultToContextFragment`，保留 `trust=untrusted` 与 provenance；若结果要形成 Artifact 或 Memory，需继续携带原始 Invocation lineage，不能因人工摘要或模型改写提升信任等级。
- Network Guard 尚未装配生产 Resolver/Dispatcher。后续 Dispatcher 必须用守卫提供的某一个批准地址直接建连，使用 Profile `tlsServerName` 做 SNI/证书主机校验、通过 `caBundleRef` 从可信 Secret backend 取 CA，并返回 socket 实际 peer；禁止在 Dispatcher 内再次按 hostname 自由解析或自动跟随重定向。当前接口存在不代表可启用 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED`。
- Secret Provider adapter 没有生产 backend 或启动配置。未来 Provider 必须为每次 `read` 返回独占、可写的 fresh `Uint8Array`，遵守 AbortSignal 且不得把 secret 写入异常；复用共享 buffer 会在首次请求后被 adapter 清零。SDK 所需 token string 无法强零化，部署时应使用短期、最小权限凭据并依靠 Catalog 版本/吊销和 Server 端失效控制暴露窗口。
- Catalog file source 没有默认路径或启动装配。后续受控环境使用时必须挂载 root/Runtime UID 拥有、group/other 不可写的 regular file；默认 Kubernetes ConfigMap/Secret projected volume 使用 symlink，会被 `O_NOFOLLOW` 拒绝，可由受信 init/sidecar 写入私有 tmpfs regular file 并以同目录原子 rename 更新。不要降低 symlink/mode 校验来迁就挂载方式。
- Credential Catalog v1 没有数据库迁移或生产配置入口。后续 Catalog source 必须返回完整、受信的 `contracts/agent-external-mcp/v1/credential-catalog.schema.json` manifest，并在轮换时先发布新版本、更新 Profile、确认新建连成功，再吊销旧版本；Catalog 中只允许 opaque `provider_secret_ref`，禁止 Secret 值或 provider credential。
- 外部 MCP Profile foundation 没有数据迁移，Compose 固定 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=false`。现阶段不要在部署环境开启该变量；Runtime 会在发现启用配置后拒绝启动，直到后续里程碑注入具备加密凭据解析、轮换/吊销、逐次 DNS 地址检查和 TLS 验证的生产 Transport Factory。Profile JSON 禁止保存 Token、密码、私钥或 CA 正文。
- MCP 限流没有数据迁移。微服务 Gateway 默认配置为 60 次/60 秒；发布后可调整 `DIPOLE_RATE_LIMIT_AGENT_MCP_LIMIT` 与 `DIPOLE_RATE_LIMIT_AGENT_MCP_WINDOW_SECONDS`。两个值必须为正且 Redis 必须可用，否则认证 MCP GET/POST 返回 429。回滚先关闭 `DIPOLE_GATEWAY_AGENT_MCP_ENABLED`，保留限流配置不会影响其他入口。
- `BeginMcpToolInvocation` 与 `FinishMcpToolInvocation` 是 Agent Capability v1 的 additive RPC。先执行 migration v30 并滚动 Core，再滚动 Runtime；入口默认关闭，确认 Core 审计可用后再按既有双开关启用。回滚先关闭 Gateway/Runtime MCP 开关并等待 `running` 调用收敛，再回退 Runtime/Core；v30 down 会删除 `agent_tool_invocations` 审计历史，应先完成合规留存判断。
- `ResolveMcpContext` 是 Agent Capability v1 的 additive RPC。先滚动 Core 与 Runtime，再同时显式设置 `DIPOLE_AGENT_MCP_SERVER_ENABLED=true` 和 `DIPOLE_GATEWAY_AGENT_MCP_ENABLED=true`；任一开关保持 `false` 时公网链路不可用。回滚先关闭 Gateway 开关，再关闭 Runtime 开关，无数据迁移。共享环境仍需完成 `AD-037` 的 OAuth resource indicator、外部 Server 凭据和 OTel exporter/告警门禁。
- migration v29 创建 `agent_memories`。先迁移 Core，再滚动发布 additive Proto 两端，最后在受控 Shadow 环境设置 `DIPOLE_AGENT_MEMORY_ENABLED=true`；默认与 Compose 保持 `false`。回滚先关闭开关，Down 会删除全部 Memory 内容和来源记录，只适用于停止 Agent Runtime、确认无需保留记忆且已完成必要导出的环境。
- migration v28 创建 `agent_event_subscriptions` 并为 `agent_tasks` 增加 nullable `trigger_subscription_uuid` 外键。发布新 Runtime 前先迁移 Core，再滚动发布 additive Proto 两端；启用 `DIPOLE_AGENT_TRIGGER_MODE=subscription` 前必须通过受控 Store 写入绑定有效 Definition 的订阅。回滚先恢复 `direct_target` 并等待在途 Task 收敛，再执行 v28 Down；Down 会删除订阅表及 Task 订阅绑定，仅适用于确认不再需要相关审计数据的环境。
- migration v27 将历史 `users.status=0` 归一为 `normal=1`，把默认值改为 `1`，并增加仅允许 `1/2` 的数据库约束。Down 会移除约束并恢复默认值 `0`，已归一的行继续保留 `1`，避免破坏合法账号状态。
- `dipole-agent-artifact-maintenance -action authorize` 从已签名 reconcile JSON 生成短期授权；`-action evaluate` 需单独注入 `storage.artifact_maintenance_access_key/secret_key` 并连接只读 MySQL。`would_delete` 仅表示当前证据满足条件，不能转换为删除动作；其余阻断结果退出码为 2。
- 运行 `dipole-agent-artifact-reconcile` 前通过环境变量单独注入 `storage.artifact_audit_access_key/secret_key`，并保持最短 `-minimum-age=24h`；命令输出 JSON，发现候选或异常键时退出码为 2。当前报告仅用于审查和容量治理，不能直接作为删除授权。
- 通用存储默认凭据由 MinIO root 改为 `dipoleplatform`，Artifact 存储通过 `storage.artifact_*` 独立配置且默认关闭；更新后的 `minio-init` 会幂等创建两个受限用户、policy 和专用 bucket。微服务 Compose 显式启用 Artifact 存储；自定义部署需先创建等价 policy，再设置独立 access key/secret，回滚时关闭 `storage.artifact_enabled` 即可停止 Artifact blob 装配，现有元数据与对象保持不变。
- 启用 `read_shadow` 前先执行 migration v23，并同时配置 MySQL ledger、`ai_sdk` model routes、Agent Capability RPC 与 Temporal；启动顺序为 Store 探针、Worker、Temporal dispatcher、Kafka consumer。回滚先将 Activity mode 恢复为 `persistent_shadow`/`foundation` 或关闭 Temporal，再回滚 v23；v23 仅增加 nullable `agent_model_calls.output_json`，旧 Runtime 可继续运行。
- `FinishRun`、`RequestApproval` 和 `ResolveApproval` 是 Agent Capability v1 的 additive RPC，旧 Runtime 可继续调用 `CompleteRun`。持久 shadow Worker 需同时显式设置 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=persistent_shadow`、启用 Agent Capability RPC 并配置既有共享密钥或 mTLS；回滚时先恢复 `foundation` 或关闭 Temporal Worker，无需数据迁移。
- Agent 镜像新增精确锁定的 Temporal TypeScript SDK `1.23.0`，构建和运行基底由 Node 22 Alpine 切换为 Node 22 Bookworm slim，以满足 Temporal Native Core 的 glibc ABI。现有部署无需启动 Temporal Server；`DIPOLE_AGENT_TEMPORAL_ENABLED` 默认为 `false`，仅在独立 foundation 验证环境设置为 `true`，并配置 address、namespace 与 task queue。
- 发布 Context Compiler v1 前先执行 migration v22；该 migration 只向 `agent_shadow_plans` 追加 nullable compiler/Token/manifest 字段，旧 Runtime 可继续写入空值。回滚新 Runtime 后可执行 v22 down 删除 manifest 字段，不影响 Plan/Step 主数据。
- 启动 TS Agent Capability 执行前先应用 migration v21 和更新后的 `agent-service-grants.dist.sql`，再为 `dipole-agent` 配置 Core target、共享密钥与 mTLS CA/cert/key/server name。微服务证书脚本已生成 Agent 双用途证书；回滚时先关闭 `DIPOLE_AGENT_CAPABILITY_RPC_ENABLED` 或恢复 metadata-only Runtime，再执行 v21 down。v21 down 会删除 Run 审计及未完成 Step lease，只应在确认 Agent consumer 停止后执行。
- 启动前执行 migration v17。该变更仅扩宽 Agent policy 身份列；应用回滚时保留 24 字符宽度以兼容已有身份，因此 Down 为安全 no-op。需要临时回退策略来源时设置 `DIPOLE_AI_POLICY_MODE=static`。
- 独立 Message Service 部署先应用 `message-service-atomic-grants.dist.sql` 或 `message-service-projector-grants.dist.sql`，再配置 `message.mysql.*` 并启用 `message.enforce_db_permissions`；atomic/projector 模式与账号必须匹配。微服务 Compose 已通过一次性 `mysql-permissions` 服务自动执行开发授权。
- 发布 Cassandra 完整消息归档能力时先执行 migration v15；已有 Job 按固定高水位回填 `mysql-messages:<id>`。归档 Backfill/Reconcile 必须使用同一 manifest 和 Job 名；需要换源时创建新 Job，禁止覆盖归档或复用旧 checkpoint。
- 发布归档重建能力时先执行 migration v13；已有 Search Job 会按其固定高水位回填 `mysql-outbox:<id>` 身份。归档模式要求三条命令使用同一 manifest 和新 Job 名，回滚可显式恢复 `--source=mysql`，不得覆盖已存在归档文件。
- 发布 Message Metadata 时先执行 migration v12 并应用更新后的 Message atomic/projector GRANT，再滚动新 Message 节点；回滚先恢复旧节点，再执行 down migration。历史 `payload_sha256=''` 表示迁移前未记录指纹，不据此拒绝重复请求。
- migration v14 为 `message_metadata` 回填 `legacy_message_id`；先执行 migration，再灰度开启 `message.cassandra_duplicate_hydration`。回滚开关后可继续使用 MySQL 正文，随后再执行 v14 down migration。
- 写责任切换顺序固定为：完成 baseline/Replay Reconcile 和 lag/DLQ 门禁，应用两套最小授权，启用 `sync.enforce_db_permissions`，再设置 `message.inbox_write_mode=projector`；回滚时先恢复 `atomic` 及原 Message 授权，再停用 Projector。
- migration v9 会从 `messages.seq` 回填现有 Inbox，并创建位置唯一索引；存在无法关联 Message 的孤立 Inbox 时迁移会失败，部署前应先完成一致性检查。表级 DDL 建议在维护窗口执行。
- 切换 Sync 查询前先启动 `dipole-sync` 并验证 health/RPC；随后将 Core 的 `sync.transport` 改为 `grpc`。回滚只需恢复 `local` 并重启 Core，不涉及数据回滚。

- 消息搜索入口默认关闭；部署时需要同时设置 Gateway `search.enabled=true` 与前端构建变量 `VITE_SEARCH_ENABLED=true`，任一侧关闭都会保持现有聊天行为。
- Message、Inbox 与 Outbox Producer 已作为同一 sqlc 事务边界运行，避免跨连接提交。
- MySQL 运行时连接池由 `database/sql` 直接初始化，migration、sqlc repositories 与 Bloom Registry 共享同一连接池。
- Bloom Registry 改用 `database/sql` 读取用户和群 UUID，停止依赖全局 GORM 查询。
- 部署或本地启动服务前执行 `go run ./cmd/migrate -direction up`；`000001_baseline` 创建或接管基础业务表，`000002_conversation_sequence` 回填历史会话序号并建立 allocator。
- `000002` 按 `conversation_key + id` 为历史消息回填 `1..N`；先完成 migration，再滚动发布新 Message 节点。旧 `before_id`/`after_id` 接口在客户端迁移期间继续保留。
- `000003` 通过现有最后消息与未读计数回填会话读位置，并创建 `device_sync_checkpoints`；只有启用具备 IndexedDB 事务提交能力的 Web Sync Engine 后，客户端才会自动 ACK Sync 页面。
- Web Sync Engine 通过构建变量 `VITE_SYNC_ENGINE_MODE=off|shadow|primary` 灰度；默认 `off` 保持现有客户端行为，`shadow` 以旧 Offline 为界面主路径，`primary` 才由 `/sync` 恢复界面。旧布尔开关 `VITE_SYNC_ENGINE_ENABLED=true` 暂时映射到 primary 兼容已构建环境。
- `000004` 创建可重建的 `message_search_documents` 逻辑索引；当前不自动回填，A5 Search Indexer 上线前搜索入口保持关闭。
- `000005` 回填群消息高水位并创建设备群 checkpoint；Message 账号新增 `group_sync_states` 写权限，继续拒绝设备 checkpoint 与其他 Core 表。
- `000006` 创建 `cassandra_backfill_jobs`；先确认实时 Projector 已接管增量，再运行 `dipole-cassandra-backfill` 固定历史快照并补齐 Cassandra。
- `000007` 为 `message_search_documents` 增加 revision、searchable 与 payload hash，并允许最小 tombstone 使用空正文元数据；迁移前后搜索入口仍保持关闭。
- Cassandra 固定快照对账通过后，可同时启用 `cassandra.enabled` 与 `message.cassandra_shadow_reads`；回滚时关闭后者并滚动重启 Message Service，无需迁移数据。
- 启用 `message.cassandra_read_percentage` 前先发布支持 `before_seq` 的服务端；新版 Web 不再把 MySQL `id` 用作历史游标，旧客户端的 ID cursor 继续固定访问 MySQL。
- baseline migration 会创建 `user_sync_inbox` 与 `user_sync_states`；所有消息持久化节点完成升级后，并发提交顺序保证正式生效。
- Inbox 只覆盖升级后新产生的消息；升级前历史消息继续通过现有历史/离线消息接口读取。
- 现有 `/messages/offline` 接口继续保留，客户端可以渐进迁移到 `/sync`。
- 兼容缺少 `sync_fanout` 字段的旧私聊和群聊 Kafka 事件，避免滚动部署期间漏写 Inbox。
- 独立部署先启动启用 `internal_rpc` 的 Core，再启动 `cmd/message-service`；两端通过 `DIPOLE_INTERNAL_RPC_SHARED_SECRET` 注入同一服务凭据，Gateway/Core 节点随后将 `message.transport` 切换为 `grpc`。
- Gateway 独立部署要求 `gateway.mode=remote`、`message.transport=grpc`、Kafka 和内部 RPC 同时启用；Core 改为私网 HTTP 目标，公开流量只进入 Gateway。

### 验证

- Approval grant 测试覆盖 active Run/Task、Core scope hash、唯一 exact binding、零/多匹配、过期/消费/撤销过滤、RPC 服务身份与 NotFound 收敛、TS scope/arguments/nonce 摘要复核及 write gate adapter；sqlc 查询固定 `LIMIT 2`，不静默选择候选。
- MCP Message write projection 测试覆盖审批消费、Tool begin、同 Invocation ID 的 Command RPC、action finish 的严格顺序，以及 direct conversation、active mode、显式 executor、Command kind 和返回引用漂移的 fail-closed 行为；现有 read Tool 回归保持通过。
- MCP Message Command 测试覆盖 Core 派生 Command ID、canonical 参数黄金向量、Tool/Approval/Task/Run/Capability/身份漂移、Agent event lineage、RPC 服务身份与 TS 返回证据复算；Tool runner 和 Approval gate 对同一参数使用统一排序 canonical JSON。
- Tool action lineage 测试覆盖已消费审批、Task/Capability/参数摘要与资源 scope 漂移、read/write 引用隔离、Command kind/capability 对应、sender-scoped receipt、Message UUID/身份/会话/类型冲突、protobuf/TS 映射及 sqlc 持久化；生产 MCP Server 的 read-only 投影回归保持通过。
- MCP 限流测试覆盖 JWT principal 派生、跨 Task 共享额度、principal 隔离、429/`Retry-After` 向上取整、被限请求不进入代理、DELETE 清理旁路、Redis 缺失 fail closed，以及消息发送 fail-open 兼容性；真实 Redis 验证两个 Limiter 实例共享计数和 TTL 到期恢复。
- MCP ToolCall 测试覆盖 durable begin、权威身份绑定、只读风险门禁、参数/结果哈希、64 KiB 上限、异常脱敏、OTel span 终止、RPC principal 拒绝、重复 begin 与单次终态；真实 MySQL 8.4 验证 migration v30 全量 `up→down` 链、FK/CHECK 约束及 sqlc contract。Go 聚焦测试、128 项 TS 测试、typecheck/build 和双语言生成物通过。
- MCP 认证挂载测试覆盖默认零路由、错误服务密钥、JWT principal 覆盖、Task/Run 绑定、异步 Core 上下文解析、foreign principal NotFound、内部凭据与 body 长度头清理、SSE/Session header 透传及无效标识拒绝；Go 全量 test/vet、定向 race、125 项 TS 测试、typecheck/build、双语言 Proto 漂移、Compose、架构文档和生产依赖零漏洞门禁通过。
- Agent Memory 单元与传输测试覆盖内容/生命周期校验、Task 固定身份、conversation scope 越权、伪造 principal、provenance 映射、稳定排序和 `untrusted` Context 注入；真实 MySQL 8.4 验证主体隔离、sqlc 创建/查询/撤销、类型/状态/priority CHECK 及 migration v29 `up→down→up`。聚焦 Go/TS 测试、typecheck 和 build 通过。
- Agent Event Subscription 单元与契约测试覆盖未知过滤器字段、控制字符、Definition 撤销/过期/漂移、resource scope 越权、RPC 服务身份、零匹配零台账/零计划、多匹配稳定选择和 Task 绑定冲突；真实 MySQL 8.4 验证默认 21 字符 Agent ID、sqlc 创建/查询/撤销、外键/CHECK 约束及 migration v28 `up→down→up`。Go/TS Proto、sqlc 生成物、112 项 TS 测试、typecheck 和 build 通过。
- Node 22.12.0/npm 10.9.0 干净容器通过冷安装、3 项 Vite 工具链契约、53 项 Vitest、`vue-tsc`、Vite 8 生产构建和双重 audit；HTTP/WS 开发代理、`/app/` 静态资源路径及 Chromium/Firefox/WebKit 全部适用 E2E 场景通过。
- User status 契约测试固定 JSON Schema ID、版本、默认值、枚举与 Go 常量一致；真实 MySQL 8.4 migration 测试覆盖历史 `0` 回填、默认写入、非法值拒绝和保留归一数据的降级边界。
- maintenance 单元/契约测试覆盖双审批与职责分离、15 分钟有效期、候选绑定、24 小时语义复核、元数据回补、对象缺失/漂移/过期和可复算 authorization/receipt 示例；真实 tmpfs MinIO 验证 inspect 身份可 Stat 且无法 List/Put/Delete，生产 Agent protobuf 继续没有 Artifact 清理方法，第 21 个后端二进制及当前源码镜像构建通过。
- 真实 MySQL 8.4 验证对象键存在/缺失查询与 migration v26 回滚重建，真实 tmpfs MinIO 验证 audit 用户可列举固定前缀且无法 Get/Put/Delete；联合环境运行 CLI 后输出年轻对象隔离和有效 evidence SHA-256。单元测试覆盖过期孤儿、已引用对象、年轻对象、异常键、24 小时门槛、样例上限与报告复核，新增第 20 个后端二进制的当前源码镜像构建通过。
- 真实 tmpfs MinIO 验证 Artifact 专用身份可幂等 Put/Get，无法删除、写入通用文件 bucket 或逃逸对象前缀；`dipoleplatform` 可正常写入/清理文件对象且无法写 Artifact bucket。三份 Compose 配置渲染和两次连续 `minio-init` 均通过，Go 全量包 test/vet、定向 race、TS 90 项测试、typecheck/build、sqlc/Go+TS Proto 漂移及 19 个当前源码后端二进制镜像构建门禁通过。
- 真实 Temporal CLI 1.8.2 / Server 1.31.2 验证 read Activity 在模型、Plan 与 Capability 完成后丢失 ACK 并重试时，provider 和 Capability 均只执行一次；真实 MySQL 8.4 验证结构化输出恢复、持久预算漂移拒绝、已完成 Run 精确重放，以及完整 migration v23 `up→down` 链。TypeScript 绑定测试覆盖 Kafka 仅启动稳定 Workflow、inline planner 不再执行和伪造 Task/Run/event 拒绝。
- 真实 Temporal CLI 1.8.2 / Server 1.31.2 验证 Approval 在 Worker 替换前仅持久创建一次、恢复后保持同一等待点、精确 Signal 只解析一次且解析完成后才继续 Step；真实 MySQL 8.4 验证 pending 精确创建重放、绑定冲突拒绝、16 路并发批准收敛、approve/deny 竞争仅一个 CAS 获胜、终态重放和伪造 principal/cross-Task 拒绝。
- Temporal 状态机、稳定 Workflow ID、重复启动策略、默认关闭配置和 Worker 启停单测通过；真实 Temporal CLI 1.8.2 / Server 1.31.2 验证 admission 仅执行一次、Step 两次失败后第三次成功、替换 Worker 从历史恢复审批等待、Signal 后完成、terminal Activity 失败后重试，以及超步数精确写入 failed。真实 MySQL 8.4 验证 failed/cancelled Run 首次提交、精确重放、持久读取与冲突终态拒绝；Agent 全量为 62 passed / 13 skipped，TypeScript typecheck/build 和 Proto drift 门禁通过。
- 已通过 Context Compiler 的确定性顺序、section/global budget、full→compact、optional omit、required fail-closed、重复 ID 和不可信内容隔离测试；真实 MySQL 8.4 验证 v22 manifest 持久化与 `up→down→up`，最终 schema version 为 22。
- 已通过 Go/TypeScript Task/Run ID 黄金向量、Agent application/transport/bootstrap 测试、真实认证 gRPC 最小权限测试、真实 MySQL 8.4 Agent Policy Repository 与 Step claim 合同，以及 migration v21 `up→down→up`；最终 schema version 为 21，`agent_runs` 和 Step claim 字段均存在。真实 Go Core 与 Node grpc-js 完成 `AdmitRun→ListConversations→CompleteRun→replay`，重放返回同一 completed Run。
- 已通过真实 MySQL 8.4 Agent policy 合约：使用默认长度 Assistant/principal 初始化 Definition，持久 Task 固定版本并完成 `running→completed`；同时覆盖 Definition 撤销/过期、新版并发出现、重复触发和 resource scope 越权拒绝。
- 已通过 Chromium、Firefox、WebKit 共 12 项真实 IndexedDB/Session Playwright 验收；实验性 Chromium quota override 未拒绝 IndexedDB 写入并被明确标记为 skip，未作为 AD-025 完成证据。
- 已通过 MySQL 8.4/Cassandra 5.0.9 重复发送 hydration 演练：真实 Timeline 命中不读取 MySQL 正文，Metadata 恢复 legacy ID；缺失和历史无 Seq 回退，v14 历史回填成功；运行时要求显式启用 Cassandra，指标拒绝未定义标签。
- 已通过 MySQL 8.4、MinIO object lock 与 Elasticsearch 9.5.2 联合演练：专用账号按 2/2/1 清理 5 条水位内 Search mutation，保留无关 Outbox 并拒绝 Core 表访问；删除本地副本后按精确对象版本恢复，从空索引重建并完成 3/3 hash 对账、Alias 正向切换和回滚。
- 已通过 Search cleanup dry-run、安全证据、未发布事件阻断、维护窗口/operator 门禁和批次中断后可重入测试。

- 已通过 MinIO/MySQL 8.4/Elasticsearch 9.5.2 三存储恢复演练：对象发布返回固定版本，保留期内无 bypass 删除失败；删除本地归档后按 receipt 恢复，再删除历史 Message Outbox，最终重建、3/3 hash 对账、Alias 正反切换均通过。
- 已通过 MySQL 8.4/Elasticsearch 9.5.2 归档恢复演练：导出固定 mutation snapshot 后删除历史 Message Outbox，仍完成两个物理索引重建、3/3 hash 对账、Alias 正向切换和回滚；陈旧 MySQL snapshot 被阻断，目标篡改返回退出码 2。
- 已通过 MySQL 8.4 migration v12 空库、v11 历史回填、重复执行和回滚测试；Repository 合约覆盖 Metadata 双键定位、hash、Outbox 失败原子回滚，以及删除 Message 正文后的文件授权。最小权限 write-owner smoke 覆盖 projector/atomic 模式与 Metadata 表启动探针。
- 已通过 43 个前端测试；新增群同步引擎、IndexedDB v2→v3 升级、群位点单调性、逐账号清理和浏览器重开补交 ACK 契约。
- 已通过 29 个 Web Inbox Sync/Session 单元测试，覆盖本地优先恢复、Sync/Offline 多页推进、事务失败禁止 ACK、断点 ACK 补交、非推进页拒绝、分页上限、IndexedDB 重开与旧 schema 升级、账号隔离、游标单调性、高低水位淘汰、会话保底、大页面硬上限、quota 分类、首轮基线、宽限匹配、单边超时、状态溢出、损坏恢复、私聊语义过滤、终止 singleflight、先撤销后清理、存储/清理失败隔离、401 接线和跨账号登录。
- 已通过 Web Sync `off|shadow|primary` 三种生产构建、聚合遥测 Handler 边界测试、Prometheus Collector 测试、OpenAPI 生成和全量 Go 测试。
- 已通过 Web Sync Prometheus 七条规则静态检查与固定时序测试，覆盖窗口不足、样本不足、干净晋级、终态差异和比较器溢出。
- 已通过 Pencil 结构与截图检查，确认 Sync 状态矩阵、desktop 及 mobile frame 无 clipping 或残留 placeholder，并导出批准预览。
- 已通过 MySQL 8.4 与 Cassandra 5.0.9 真实 Sync hydration contract，覆盖一致结果、payload mismatch、缺少 Cassandra 投影及 MySQL 主结果隔离。
- 新增真实 MySQL 8.4 写责任烟测，覆盖 Sync/Message 最小权限启动探针、Message+Outbox 提交但不写 Inbox、Projector 收敛和 atomic 回退恢复。
- Sync Projector 三节点 Kafka/MySQL smoke 增加启动前 backlog、在线双运行、热群跳过、永久失败 retry/DLQ 和无脏行验证。
- 已通过锁文件冷安装、6 个前端 Search 单元/组件测试、`vue-tsc`、Search 开启/关闭两种 Vite 生产构建；生产依赖 `npm audit --omit=dev` 为零漏洞。
- 已通过 Pencil 全文档结构检查，确认 Search 八个 frame 无残留 placeholder、clipping 或未命名图层，并导出 desktop 1440x900 与 mobile 390x844 批准预览。
- 已通过 `go test ./...`、`go vet ./...` 和 `go mod verify`。
- 已通过新增同步 repository、service、handler 及消息 service 的定向 race 测试。
- 已通过 Kafka `sync_fanout` 新旧字段契约测试和幂等目标隔离测试。
- 已通过 MySQL 8.4 双事务提交顺序集成测试、`FOR UPDATE` 方言测试和 Sync 锁行回滚测试。
- 已通过 MySQL 8.4 空库升级、重复执行、未来 migration 兼容和 baseline 回滚测试。
- 已通过 sqlc Store 的 MySQL 8.4 提交、回滚与幂等插入集成测试。
- 已通过全部 sqlc Repository 的真实 MySQL 功能契约，覆盖幂等、状态转换、排序、权限、事务回滚、租约和同步顺序。
- 已通过 sqlc 同用户并发提交顺序测试，确认 Inbox `sync_seq` 与提交顺序一致。
- 已通过 Message gRPC bufconn 契约、结构化错误往返、全量 Go、vet、race 和模块完整性测试。
- 已通过 Core 五项能力与 Sync Timeline 页面的 bufconn 往返测试，覆盖调用身份、权限结果、成员快照和持久游标映射。
- 已通过 Cassandra MessageStore 影子读单元/race 测试和 Cassandra 5.0.9 跨 bucket Seq 范围真实 contract；差异、跳过与 Cassandra 错误均不改变 MySQL 响应。
- 已通过 MySQL 8.4 与 Cassandra 5.0.9 真实读路由 contract：before/after Seq 完整页由 Cassandra 返回，人工删除一行后按同一 Seq cursor 整页回退 MySQL。
- 已通过 Cassandra 主读 payload 篡改演练：Seq 保持连续时内容差异仍被抽样核验识别并返回 MySQL 完整页。
- 已通过 Kafka legacy/v1 minor/v2 兼容、永久 schema 错误隔离、DLQ 诊断 header 和 Outbox schema header 测试。
- 已对 Local 与 gRPC transport 运行同一套八项 MessageApplication 行为契约，覆盖文本/文件命令、历史、热群增量和离线查询。
- 已通过内部 gRPC 服务认证集成测试，覆盖合法凭据、缺失凭据、错误密钥、未授权调用方与无效启动配置。
- 已通过 Core File Capability 的 Local/Remote 契约、非所有者隐藏测试，以及 MessageService 远程文件消息测试。
- 已通过 shadow query 契约：primary 四类命令各执行一次、shadow 命令调用为零、四类查询均参与比较，差异不改变 primary 响应。
- 已通过 TLS 1.3 mTLS 真实网络测试、非 loopback 明文拒绝、认证 caller 与 payload caller 冲突拒绝测试。
- 已通过 MySQL 8.4 临时受限账号验证：五张 Message 所需表可读取，Core `users` 表返回权限拒绝，测试账号随后删除。
- Message History mTLS gRPC loopback 三轮为 `69,339-69,618 ns/op`，低于 M4 `<1 ms/op` adapter 门槛；完整端到端 P95/P99 继续由 G0 跟踪。
- 已通过 Gateway 本地 health、真实 HTTP 反向代理、依赖门禁、Gateway/Core 独立 RPC 身份和 WS Gin 全局状态 race 测试。
- 已通过本地 Core、Message、Gateway 三进程 smoke：Gateway 注入不可达 MySQL 仍正常启动，health 返回成功，Core HTTP 代理返回预期认证响应，Core remote WS 路由关闭。
- 已通过隔离 Compose 容器验收：migration 成功退出，Core/Message/Gateway 经 mTLS 冷启动并达到 healthy，公开代理与 WS 所有权检查通过。
- 已通过 MySQL 8.4 会话 Seq migration 回填/回滚、同会话事务锁顺序和 24 路并发连续性测试，并验证失败事务不消耗序号。
- 已通过 Kafka legacy payload、Message protobuf 往返、Sync RPC、WS 映射和 shadow comparison 的 Seq 兼容测试。
- 已通过 MySQL 8.4 部分已读、并发新消息隔离、乱序投影保护、24 路设备 ACK 单调性和多设备隔离测试。
- 已通过 MySQL 8.4 MessageStore/SyncStore/SearchIndex contract，覆盖会话 Seq cursor、索引幂等更新、范围隔离、排序和删除。
- 已通过 MySQL 8.4 热群高水位回填、消息事务原子回滚、设备群 ACK 单调性，以及 Sync 权限和 Message/Sync gRPC 契约测试。
- 已通过 created mutation 新旧 payload 归一化、event type 一致性、future mutation revision/actor 门禁和 direct/group 事件命名测试。
- 已通过三节点 Kafka 故障演练：单 broker 停止后继续确认写入，低于 min ISR 时拒绝 ACK，恢复 quorum 后消息完整可消费。
- 已通过 Kafka consumer group 演练：两个 member 各持有 3 个 partition，单 member 退出后剩余 member 接管 6 个 partition并将 lag 恢复为 0。
- 已通过 Kafka 可观测性演练：Prometheus 规则有效，consumer lag、retry/DLQ 增量和单 broker 故障造成的 ISR 缺口均可查询，broker 恢复后缺口归零。
- 已通过 MySQL Router writer 演练：停止 PRIMARY 后同一 `database/sql` 池约 4.1 秒内连接新 writer，切换前后已提交记录均可见，旧节点通过 AdminAPI 成功 rejoin。
- 已通过两个 migration runner 对空库并发执行测试，双方均成功且 migration ledger 保持唯一完整。
- 已通过 Redis Sentinel 演练：停止当前 master 后约 4 秒完成切换，同一客户端恢复读写与 Pub/Sub，Presence、热点和限流状态可用，旧 master 重新加入为 replica。
- 已通过隔离 Storage Lab 演练：Cassandra 与 Elasticsearch 健康启动并完成临时 CRUD，Core、Message、Gateway 和客户端生产读路径保持断开。
- 已通过 Elasticsearch 9.5.2 真实 contract：重复 Bootstrap 校验 mapping/Alias，external revision 支持重放、更新与旧事件 no-op，作用域搜索无隐藏会话泄漏。
- 已通过 MySQL 8.4 与 Elasticsearch 9.5.2 版本化 Search contract：同 revision 冲突可检测，tombstone 后旧正文事件无法恢复搜索结果。
- 已通过三节点 Kafka/Elasticsearch Search Indexer smoke：created r1、recalled r3 与迟到 edited r2 收敛为 revision 3 tombstone。
- 已通过 MySQL 8.4/Elasticsearch 9.5.2 Search 恢复演练：created/edited 与 created/recalled 折叠为 3 个最终状态，固定高水位对账一致，目标 hash 篡改返回退出码 2。
- 已通过 Elasticsearch Alias 正反切换演练：old→new 与 new→old 均保持双 Alias 原子所有权；新增 Outbox mutation 后陈旧快照被拒绝且 Alias 未漂移。
- 已通过 MySQL 8.4 Search 授权范围合约与 Core gRPC 往返测试：私聊和有效群成员关系可见，无成员关系或无效群状态均不可见，缺失 principal 被拒绝。
- 已通过 Search Application、内部 RPC 和 Composition Root 测试：空 scope 不访问 Elasticsearch，基础设施错误保持有界，启动 readiness 不产生索引写操作。
- 已通过 Elasticsearch 9.5.2 Search Service 真实契约：同关键词的授权与隐藏文档经 Core scope、内部 RPC 和 read Alias 查询后只返回授权结果。
- 已通过 Gateway Search 路由测试：缺失令牌返回 401，合法会话调用 Search Application，精确路由不进入 Core HTTP 反代，依赖错误不泄漏内部详情。
- 已通过 Cassandra 5.0.9 Timeline contract：bucket 边界与 Seq 倒序正确，重复 payload 安全重放，冲突 payload 拒绝覆盖。
- 已通过 Kafka/Cassandra projector 演练：独立 consumer group 获得 assignment 后消费两次相同 created event，最终只生成一条 Timeline 记录。
- 已通过 MySQL 8.4 Backfill lease 合约，以及 MySQL/Cassandra 恢复演练：失败批次 checkpoint 不前移，恢复时安全重放 duplicate，最终固定高水位全部完成。
- 已通过 Cassandra 对账演练：干净快照全量匹配；人工篡改后检测 hash 与样本差异并返回退出码 2，差异报告不包含消息正文。
- 已通过 MySQL 8.4、MinIO Object Lock 与 Cassandra 5.0.9 联合演练：发布并按固定对象版本恢复完整消息归档，删除 MySQL `messages` 正文后仍重建 3 条 Timeline 并 3/3 对账；换源、内容篡改和保留期内删除均被拒绝。
- 已通过真实 MySQL 8.4 Message/Sync 最小权限演练和完整微服务镜像 smoke：atomic/projector 写责任正确，禁止多余表操作与 Core 访问；migration 后自动授权，Message/Sync 权限门禁、mTLS、Gateway health 和 Core 代理全部通过。

### 已知问题

- Memory v1 仅提供受控 Store 和运行时读取链，尚无自动写入/纠正/删除 API、压缩反思 Worker、置信度与版本冲突策略、混合/向量召回和用户 UI；共享 Shadow 仅在已有受控记录时读取，详见 `AD-035`。
- Event Subscription v1 尚无用户/Gateway 管理 API，也未接入小模型、embedding 或向量召回；共享环境继续固定 `direct_target`。管理入口完成认证、版本化变更、撤销审计和 UI 前，不启用 `subscription` 流量模式，详见 `AD-034`。
- Sync Inbox、旧 Offline 与默认关闭的幂等 hydration 尚未完成替代链路观察；Cassandra 恢复工具已可独立使用不可变完整消息归档，正文退役其余条件继续由 AD-019 跟踪。
- `/messages/offline` 真实对照观察窗口仍待执行；Web 本地 Sync Engine 默认关闭，旧客户端继续使用数据库 ID cursor。
- Web IndexedDB 已统一会话清理和容量淘汰实现；真实浏览器配额、共享设备和进程强退验收仍是默认启用门禁，记录为 `AD-025`。
- Web 协议对照首批仅覆盖收到的私聊消息；群聊存在普通群 fanout 与热群 notify/pull 两套语义，需按群类型建立独立比较契约后再纳入。
- 独立 Message Service 已停止在代码中读取 Core Repository；当前仍与 Core 共用 MySQL schema 和数据库账号，最小数据库授权记录为 AD-015。
- M5 的非 WS HTTP、Swagger 与静态 Web 仍由私网 Core 提供，Gateway 通过反向代理暴露；后续服务拆分前必须保持 Core 不直接暴露公网。
- HTTP Handler 历史测试并行修改 Gin 全局模式，整包 race 门禁会产生数据竞争；新增 Sync Handler 定向 race 已通过，后续治理记录为 AD-016。

## 发布归档

当前尚未建立正式版本标签。首次发布时，从 `Unreleased` 下沉内容并使用以下格式：

```markdown
## [X.Y.Z] - YYYY-MM-DD

### 新增

- 描述新增能力及影响范围。

### 变更

- 描述行为、依赖或接口变化。

### 修复

- 描述已修复问题及触发条件。

### 安全

- 描述安全修复；敏感细节应链接到受控公告。

### 弃用

- 描述即将移除的能力、替代方案和计划移除版本。

### 移除

- 描述已移除能力及替代路径。

### 迁移说明

- 列出数据库、配置、API、消息协议或部署顺序要求。

### 验证

- 列出本次发布实际执行的测试和检查。

### 已知问题

- 列出尚未解决且可能影响使用、开发或发布的问题。
```
