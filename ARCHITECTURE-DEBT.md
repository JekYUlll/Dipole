# 架构债务台账

本文档记录已确认但暂缓处理的架构风险、兼容性缺口和可清理冗余，便于后续按优先级滚动治理。

## 维护约定

- 状态使用：`暂缓`、`处理中`、`已解决`、`接受风险`。
- 优先级使用：`P0` 阻断发布、`P1` 应在正式启用相关能力前解决、`P2` 进入后续迭代、`P3` 按需清理。
- 新问题使用连续编号 `AD-NNN`，保留历史编号，不复用已关闭条目。
- 开始处理时补充负责人或关联 Issue/PR；解决后记录提交、验证方式和完成日期。
- 本台账描述风险和演进方向，不代表当前迭代立即修改对应实现。

### AD-042：正式技术架构图与已发布分层拓扑发生漂移

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **完成日期：** 2026-08-28
- **影响范围：** `docs/technical-architecture.svg`、微服务边界、Timeline/Projection 说明、Agent Runtime 迁移叙事
- **解决方式：** 将架构图更新为当前 Core/Message/Gateway/Sync/Search/Agent Runtime 分层，补充 sqlc、`user_sync_inbox`、Cassandra/Elasticsearch 影子投影和回滚门禁，并移除 `AutoMigrate`、无 Inbox 单体及旧 Eino 主链路的过时表述。图中仍明确标注本地合并启动、影子能力和默认关闭边界。
- **验证：** `scripts/check-architecture-docs.sh`、SVG XML 解析和 `git diff --check` 通过；本次只修改文档，不改变运行配置或服务权限。
- **长期约束：** 服务拓扑、数据 ownership、默认开关或语言职责变化时，必须同步更新架构图、对应正式文档、更新日志和台账；架构图不得把 shadow、fallback 或离线契约描述成生产主路径。

## 待处理

### AD-045：Agent Task Timeline 缺少 Core 聚合只读适配器

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** Agent Task UI、Core/Gateway 只读 API、Run/Step/Tool/Artifact 审计
- **现状：** Task、Run、Shadow Step、Model Call、Tool Invocation、Approval 和 Artifact 已分别持久化；Gateway 当前只提供权威 Task 当前状态、输入和审批控制。已建立 `contracts/agent-task-timeline/v1/`，规定 Core principal 复核、稳定 `event_seq`、增量游标和低敏事件 DTO。
- **风险：** 若由 Gateway 直接拼接多张 Agent 表或读取 Temporal 历史，会绕过服务 ownership、产生跨 Run 顺序歧义并泄露 prompt、参数或外部结果；当前前端不能声称展示完整执行历史。
- **本轮进展：** migration v48 新增 append-only `agent_task_timeline_events`，以数据库生成的 `event_seq` 保存低敏 Task/Run/Capability/Approval 元数据，并通过 sqlc 提供 append/list 查询与领域校验。生产事务装配现在让 Task/Run 创建和状态迁移与 Timeline append 一起提交；Core 已提供 owner-scoped `ListAgentTaskTimeline` v1 增量 RPC，并接入生产仓储；Runtime/Gateway 已贯通认证只读代理；前端已在 `VITE_AGENT_TIMELINE_ENABLED` flag 下支持低敏展示和 cursor 分页，失败清空并回退。其余 Step/Model/Tool/Artifact 写入口、默认生产开关和视觉评审仍未开放。
- **建议方向：** 先补齐 Step/Model/Tool/Artifact 事件写入和真实数据闭环，再以共享环境证据开启前端 flag；继续只返回低敏元数据，随后按证据逐步加入 Artifact 引用与 Pencil/视觉回归。
- **处理门槛：** Core/Gateway 契约测试覆盖 foreign Task、游标重复/漂移、跨 Run 事件、事件缺失和字段脱敏；前端默认关闭，未收到 v1 response 时保持当前 Task Query 页面。

### AD-043：Sync Cassandra hydration 缺少共享环境运行时证据闭环

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** Sync Service、Cassandra primary/fallback、Prometheus、灰度与回切门禁
- **现状：** Sync Service 已提供默认关闭的 Cassandra primary 路径和 MySQL 即时回退；离线 evidence evaluator 可消费 hit、fallback、missing、conflict、error 与 p95 聚合。运行时现在按低基数 outcome 暴露 `dipole_sync_hydration_route_total` 与 `dipole_sync_hydration_route_duration_seconds`，并保留旧日志观测。
- **风险：** 当前仍缺少真实客户端窗口、共享 Cassandra/Sync 环境采集、missing/conflict 细分的端到端归因、责任人批准、自动停止门禁与可执行回切演练；collector 只能证明进程内路由结果，不能单独证明生产 eligible。
- **本轮进展：** Prometheus snapshot adapter 现拒绝重复 outcome/family、错误类型、额外标签、未知 outcome 和非单调 histogram，并要求起止快照差分；它仍只提供受校验的低敏输入，不替代共享环境身份、客户端窗口和人工批准。
- **建议方向：** 将 Prometheus snapshot 与脱敏客户端/服务 revision、配置比例、窗口和回切演练 ID 合成为 evidence，再交给既有 evaluator；缺少完整窗口或观测断层时保持 blocked，并持续保留 MySQL 完整消息。
- **处理门槛：** 任何提高 `sync.cassandra_primary_hydration` 比例前，必须归档共享环境 evidence、复核人批准和自动回切记录；未满足前保持默认关闭或人工小比例运行。

### AD-044：Pencil 增量设计任务缺少稳定的 CLI 执行闭环

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** `design/dipole-ui.pen`、设计导出、F3 Agent Task Timeline、前端视觉回归
- **现状：** Pencil CLI `0.3.5` 认证和版本检查正常，canonical `.pen` 与既有导出资产可读取；本轮 Agent Task Timeline 增量任务在重复画布调用阶段长时间无输出，未生成新 `.pen` 或导出图，原设计文件保持不变。
- **风险：** 没有稳定的增量执行与导出结果时，无法把 Agent Task Timeline 设计资产纳入评审，也不能宣称 F3/F4 视觉基线已完成。
- **建议方向：** 将 Pencil CLI 调用拆成小批次、设置任务超时并在每次调用后校验输出文件、节点命名和导出图；失败时保留原文件并记录 CLI/skill 版本，必要时使用已批准 frame 作为回滚点。
- **处理门槛：** 新设计必须同时提交 canonical `.pen`、导出预览、`DESIGN-CHANGELOG.md` 条目和结构/视觉检查结果；未满足前不修改现有设计基线。

### AD-040：WebSocket 查询令牌进入 HTTP 访问日志

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **解决日期：** 2026-08-28
- **影响范围：** Gateway HTTP 访问日志、日志聚合与保留、WebSocket session JWT
- **解决方式：** Gin 统一访问日志在 handler 执行前解析 query，对 token、Authorization、API key、client secret、密码和签名类键进行大小写无关匹配，并把每个重复值替换为固定 `REDACTED`；非法 query 不回退原文，整段记录为固定脱敏值。普通参数规范化后继续提供路由诊断，现有 WebSocket query/Bearer 认证协议保持兼容。
- **验证：** 单元测试覆盖普通参数、百分号编码键、大小写变体、重复凭据和非法分隔符；Zap observer 经真实 Gorilla WebSocket upgrade 捕获访问日志，确认 query token、编码 access token 与 Authorization Header 均未进入结构化字段。logger/server/WS 相关包 race 测试通过。
- **长期约束：** 新增任何 query credential、短期 WS ticket 或签名参数时，必须同步更新敏感键集合和真实日志 capture 测试。反向代理与外部日志采集器仍需独立确认不记录脱敏前的原始 URI；认证传输方案变化需保留客户端兼容窗口和重放威胁测试。

### AD-038：Agent 离线评测缺少真实 Task adapter 与生产语料

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Eval、Shadow 晋级、Memory/Retrieval、模型与 Prompt 发布
- **现状：** TypeScript Runtime 已提供严格的 outcome、trajectory、permission、retrieval、cost deterministic Harness、语言中立 Suite/Report schema、canonical SHA-256 和三态 CLI；promotion v2 强制绑定同一候选版本的完整五类报告并逐类别阻断。security suite 串联真实结构边界。真实 Shadow adapter 现通过 sqlc/TS 共享只读查询提取 Task/Run/Context/Step/Artifact/ModelCall/ToolCall，将数据库 observation 与版本化评审 manifest 合成五类 Suite；Task/Run 摘要绑定 case ID，独立 MySQL 账号仅具八张审计表 SELECT。通过门槛的 v2 证据可发布为不可变 `promotion_evaluation` Artifact，并通过 Gateway-only projection 审阅。Subscription corpus review v1 另以 corpus SHA-256 绑定双 reviewer 完整标签和第三方分歧裁决，输出不含正文/身份的 agreement 报告。migration v32/v33 已建立 durable grant 与双人控制面，active context 会逐次重查有效期和撤销状态。
- **风险：** 当前证据可证明 Harness、结构性门禁、评审一致性合同和真实持久执行转换语义。缺少实际归档的 Project Guardian outcome/evidence 与 review 报告、模型语义攻击 corpus、检索相关性集合和按模型/场景校准的成本分位阈值时，`eligible` 仍无法证明产品效果或生产成本满足目标。Step 表仅保存最后一次 attempt 的时间，真实 adapter 会拒绝 `attempt_count != 1`，逐 attempt 成本审计仍待补充。
- **建议方向：** 建立版本化 Project Guardian corpus 和双评审 agreement，使用真实 adapter 按场景统计 precision/recall、trajectory 差异和成本分位数；报告仅引用受控 evidence ID。候选模型、Prompt、Tool Schema 和 Memory Policy 必须先离线，再 shadow，最后灰度。
- **处理门槛：** 任何 Agent active authority、自动 Memory 写入、语义检索切流或面向用户的主动消息发送前，至少归档一份真实候选五类报告及对应 Suite hash；当前 promotion v2 只可作为 Harness/Shadow 工程门禁。

### AD-037：MCP 网络入口尚缺 OAuth、外部连接与写能力门禁

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Runtime、MCP Client/Server、Gateway/OAuth、Capability Policy、外部数据流
- **现状：** 官方 MCP TS SDK v2 Client/Server foundation 与默认关闭的 Gateway/Runtime 网络入口已完成，当前生产只投影 `conversation.list`。第一方授权交换要求 session principal 对 canonical resource 和只读 scope 显式 consent，签发 15 分钟且绑定 `aud/scope/token_use` 的 MCP JWT；普通 session 与 MCP token 互相拒绝。Gateway 剥离外部凭据并向 Runtime 证明已验证 principal/resource/scope。单次 Tool invocation 有 100 ms 至 60 秒有界 timeout、cooperative cancellation 和 `tool_timeout` 审计；外部 Client foundation 的 connect/list/call 也使用 request/total timeout，Runtime 传播连接断开信号。migration v30、统一低敏 OTel、默认关闭的 Collector/Tempo profile、共享 Redis principal 限流与真实 trace smoke 已完成。外部连接 Profile v1 现以严格契约绑定 tenant、HTTPS endpoint、Server identity、Tool/Host/Port allowlist、TLS ServerName、CA 与版本化 credential opaque ref。Credential Catalog v1 进一步保存 tenant/ref/version、生效窗口、active/revoked 状态及 opaque provider secret ref，每次建连前重新加载并精确授权，轮换/吊销无需进入 Task 或 Workflow 状态；受约束文件 source 使用规范绝对路径、canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode 和 256 KiB 默认上限，并支持原子替换。Provider-neutral MCP `AuthProvider` adapter 每次请求读取 fresh bytes，使用独立 timeout/AbortSignal、大小和 Bearer 字符校验、固定脱敏错误与源 buffer 擦除，同时不暴露自动 401 refresh。可注入 Network Guard 对每个 SDK 请求重新校验 exact HTTPS Host/Port/TLS identity，要求全部 DNS 答案为公网地址，把批准地址交给 pinned Dispatcher，并核对实际 peer；重定向、混合/重复/超量答案和 rebinding 均 fail closed。外部 Tool 成功结果可通过有界 adapter 转换为 `section=evidence`、`trust=untrusted` 且绑定 Profile/Server/Tool/Invocation provenance 的 Context fragment；compact 记录不复制外部正文。Core 现提供 active-only Approval grant resolution 与原子 consumption RPC：sqlc 精确查询最多两条候选，应用层要求 active Run、运行中 Task、principal 审批人和唯一未消费 binding；TS 独立复算 scope/arguments 并连接 write gate。`nonce_sha256` 明确作为持久的一次性绑定摘要。认证 Message Command RPC 要求 running Tool Invocation，Core 复算 canonical 参数摘要、派生 Command ID/身份并返回可验证 Message action reference。默认关闭的第一方 Message projection 已能按 `consume -> begin -> command -> finish(action)` 顺序组合这些边界，并要求 active context、显式 executor 与精确 direct conversation。active Run admission 必须经过注入式 promotion authorizer，MCP context 使用持久 Run 的权威 `runtime_id/mode` 并由 Go/TS 双重校验；migration v33 增加仅认证 Gateway 可调用的 Runtime promotion 提案、复核、查询和撤销控制面，Runtime 数据面不能签发 Grant。migration v35 保存可恢复的权威外部 Tool command；migration v36 进一步按确定性 Round ID 原子认领最多两个 Tool round，仅原 owner 可提交终态，已完成/失败结果可重放，任何遗留 `executing` 都返回 `ambiguous` 且没有 lease reclaim/retry 路径。Activity 在发起远端调用前 Claim，并在返回 Temporal 前持久化规范结果。生产未注入 authorizer，Registry、write executor 和 active context 继续关闭。真实 DNS Resolver、pinned TLS Dispatcher 与文件 CA provider 已独立实现但尚未装配到生产启动链，进程继续 fail closed。外部 MCP Server、加密 Secret backend 和 write/destructive Tool 均未启用。
- **本轮进展：** Streamable HTTP Transport Factory 已精确复核 Profile/Catalog 的 tenant-ref-version，为每次连接创建独立 AuthProvider、Network Guard 与官方 SDK Transport，并关闭 401 自动刷新、403 扩权和 SSE 自动重连。该组合只完成策略层，未提供生产 Secret/DNS/TLS I/O backend。
- **本轮进展：** 新增 production DNS Resolver、pinned TLS Dispatcher 与受约束文件 CA provider。Dispatcher 每次重载 CA，只允许自定义 lookup 返回当前批准地址，禁用代理/连接复用，并验证 chain、ServerName、remote peer、connect timeout、取消和流式 body；启动链与外部网络开关未接线。
- **本轮进展：** 新增本地 AES-256-GCM encrypted-file Secret Provider。二进制 envelope 的 AAD 绑定 tenant、credential ref/version、provider、secret ref 与 key ref；key 和密文文件独立映射、每次读取、权限约束且错误脱敏，版本轮换可移除旧映射而不 fallback。该适配器尚未接入启动链，也不替代独立 KMS、lease 和吊销告警。
- **本轮进展：** 新增 default-off production I/O composition，把文件 Catalog、encrypted Secret、Node DNS、文件 CA、pinned TLS 与 Transport Factory 收敛到只公开 Registry 的单一构造边界。disabled 不读取残留配置，enabled 构造也无文件/DNS/socket side effect；`index.ts`、环境路径加载和外部 Worker startup 仍未接线。
- **本轮进展：** 新增语言中立 production I/O manifest v1 与安全 loader。manifest 只保存 opaque ref、规范路径和有界参数，运行时复核全局唯一路径、key 关联及 owner-only/canonical/`O_NOFOLLOW` 文件证据；disabled 完全不读，重载失败不复用旧配置。`index.ts` 注册、下游文件 preflight、tenant 灰度仍待完成。
- **本轮进展：** production I/O runtime 现只公开 Registry 与 readiness preflight。预检在单一逻辑时间解析全部 Profile，去重后验证 Catalog active binding、encrypted Secret/Bearer 与 CA 文件，固定低敏收据和错误，并复用 Provider/Transport 的同一 secret 大小上限；真实文件测试覆盖 revoke、错 key、envelope/CA 损坏和恢复。该路径不创建 Transport、DNS 或 socket；`index.ts` 注册、tenant allowlist、隔离 Shadow 连通与回滚演练仍待完成。
- **本轮进展：** production runtime 增加显式 Profile/tenant 的只读 Shadow connectivity drill。它复用正式 Registry 和 modern allowlisted Client，仅执行连接、Server identity 与 Tool discovery，要求完整 allowlist 后关闭资源；协议级测试证明 `tools/call` 为零，收据及失败固定低敏。`index.ts`、自动调度与生产网络保持关闭，仍需在隔离 Shadow tenant 归档真实公网 DNS/TLS/协议、超时和回滚证据。
- **本轮进展：** readiness evidence v1 保持兼容，新增 v2 把 local preflight 与 exact Profile online drill 约束在有界时间窗，并以 canonical SHA-256 分别绑定 exact Profile 与完整 production I/O topology。migration v37 和 Go Publisher 以确定性 ID 追加保存 canonical 低敏 bundle、tenant、双 binding、operator、request/trace 与最长一小时有效期；exact replay 幂等，漂移留新历史，fresh reader 强制 tenant、双 binding 和 expiry。证据仍未签名，真实 Shadow 归档、自动 admission、独立审计导出和回滚演练待完成。
- **本轮进展：** additive readiness Publisher RPC 只允许认证且无 principal 的 `dipole-agent`，服务端派生 operator/request/trace 并严格解析 v2；TS adapter 复算 content hash 与确定性 Evidence ID，响应漂移 fail closed。RPC 已进入 Core composition，但自动采集/调度、admission consumer 和外部网络 startup 继续关闭。
- **本轮进展：** 显式单次 readiness 发布器把 production collector 与认证 RPC 串联，要求受控 tenant/Profile、60 至 3600 秒有效期和非空 request/trace；证据完成后只调用一次 Publisher，失败或取消不会重试。独立 CLI 才会读取 manifest 并执行只读公网 discovery，常驻 `index.ts`、Compose、自动调度与 admission consumer 继续关闭。
- **本轮进展：** fresh readiness Resolver 以 Core 服务端时钟执行 tenant/双 binding 精确查询，并在 Store 返回后复核 canonical 内容、确定性身份和 freshness；只读 RPC 仅向认证 `dipole-agent` 返回低敏收据或 `found=false`。自动 admission、activation、签名和独立审计导出仍待完成。
- **本轮进展：** MCP Worker construction root 已强制接收 host-owned Profile、production I/O 与 raw Registry，并在每次外部连接前自行派生 exact Profile/Runtime binding、调用 fresh Resolver、复核低敏回执和 underlying Profile。证据不缓存，缺失、漂移、取消与解析失败均在 raw Registry/Catalog/网络前关闭；readiness 采集继续使用独立 raw Registry以避免循环门禁。该控制只约束单次 exact Profile egress，不改变 Run admission、promotion 或 activation；Worker startup 与真实外部网络仍关闭。
- **本轮进展：** 新增 credential-free deployment route manifest v1 与安全 loader，把部署拥有的 route/version、Workflow 坐标、Profile/Server/Tool/egress policy 同代码拥有的 Capability schema、resource resolver 和 egress ceiling 精确 join；重复坐标、Profile allowlist 漂移和扩权均拒绝。完整部署摘要纳入 Temporal history/checkpoint，覆盖漏升版本时的 Profile/Tool/egress 漂移。loader 尚未进入 `index.ts` 或 Worker registration，外部网络继续关闭。
- **本轮进展：** 新增 default-off deployment plan，将 Profile、I/O manifest、route manifest 与 production runtime 收敛为一次加载和一个失败边界，并复用 exact config/I/O/options/raw Registry 给 readiness collection 与 gated Worker。构造不读取运行期凭据文件、不建 RPC/Worker/网络状态；`index.ts`、Compose、自动 preflight/drill 和路由调度仍关闭。
- **本轮进展：** 默认关闭的 MCP Worker Runtime 已组合 Core command resolver/round receipt、Transport Registry、Activity continuation 和三 ID dispatcher；替换实例可回放 completed receipt，ambiguous/cancellation 在网络边界前 fail closed。Temporal 调度入口仍等待受信 Agent Step 创建持久 Invocation。
- **本轮进展：** Invocation begin/finish 已支持全字段精确重放，Repository 读取终态摘要与 action reference；Command RPC additive 返回 Invocation 状态。terminal Invocation 的 Round Claim 只允许读取已有 receipt，缺失或漂移时拒绝且不会创建新执行记录，关闭 Invocation finish 后 Activity completion 丢失的重复远端调用路径。
- **本轮进展：** 默认关闭的 `TrustedMcpInvocationProducer` 使用 host-owned Workflow step/ordinal 生成稳定 Invocation ID，通过 Capability route、PolicyEngine、schema 与 egress policy 派生并验证 Profile/Server/Tool/参数；输入无法携带 authority 字段。
- **本轮进展：** additive terminal RPC 只接受 Task/Run/Invocation/Round ID，Core 从 durable receipt 派生 read-risk Invocation 的结果、字节数、错误码和首次 latency；terminal 重放核对已存证据，`executing`、`input_required`、write Capability 与绑定漂移均拒绝。默认关闭的 terminal Worker composition 已连接成功/稳定失败路径，生产仍缺真实路由、Temporal 调度与 I/O backend。
- **本轮进展：** 独立且默认关闭的 Temporal MCP dispatch Activity 已固定 route ID/version、Workflow step/ordinal、canonical 参数和完整性 checkpoint；每次 begin/retry/resume 都重新解析 Core ExecutionContext、精确重放 producer，并只向 terminal Worker 下发 Task/Run/Invocation 三个 ID。完成结果经注入式幂等 projector 收敛为 Artifact 收据，原始外部结果不进入 Workflow 输出；生产 Worker、启动入口、Activity mode 与外部网络保持未接线。
- **本轮进展：** 默认关闭的 `ExternalMcpArtifactProjector` 会重新解析 completed Invocation 并核对完整身份与 Profile/Server/Tool/Capability，从 terminal 结果生成 128 KiB 内的 canonical JSON Artifact；Invocation-derived type 隔离同一 Task 的多次调用，metadata 固定 untrusted lineage，精确重放和 Artifact 提交后取消均返回同一 content-addressed 收据。现有 Artifact policy 只允许 running shadow Run，active 结果仍 fail closed。
- **本轮进展：** Temporal dispatch route manifest 现对 route ID/version、Capability、Workflow step/ordinal 生成 canonical SHA-256，并同时绑定 begin history 与 wait checkpoint；同 route version 下的配置漂移也会在 Core 访问前拒绝。生产调度仍需确保该摘要来自版本化 host manifest，不能接受模型、事件或客户端注入。
- **本轮进展：** `createTemporalMcpDispatchRuntime` 已将 route-bound Activity、fresh Core Context、stable producer、terminal Worker 和 Artifact projector 收敛为单一 default-off construction boundary。Worker egress policy 直接从指定 Capability route 派生，删除装配调用方的重复 Profile/Tool policy 输入；公开结果只含 route binding 与专用 Activity。完成丢失、durable resume 和取消已通过端到端组合测试，生产启动仍关闭。
- **本轮进展：** `createTemporalMcpMultiRouteRuntime` 已把 deployment plan 中全部 route-scoped runtime 收敛到唯一 Activity 表面，构造时拒绝空集/重复 route，begin 与 resume 分别按输入或 durable checkpoint 的 route ID 选取 runtime，随后继续执行 route-local 全量绑定校验。该层无 I/O 或启动副作用；Workflow 已引用该 Activity，生产 Worker registration、`index.ts`、真实路由调度与外部网络仍关闭。
- **本轮进展：** `external_mcp_v1` Workflow history envelope 与专用 Temporal client 已隔离普通 goal/event 路径。route version/manifest digest 由 host catalog 注入，Workflow 首次派生 Task/Run/principal，恢复仅传 durable checkpoint 与验证后的 Signal；真实 Temporal Server 通过 Worker replacement/Elicitation resume 及现有 Workflow 回归。生产 Worker 尚未注册 MCP Activity，`index.ts`、真实 Capability route 与外部网络继续关闭。
- **本轮进展：** default-off Temporal Worker composition 已将 deployment plan、multi-route Activity、普通 lifecycle Activities 和 matching Workflow catalog 组成同一不可变 authority snapshot。disabled 不解析端口；enabled 在端口 provider 前复算 Runtime binding 并验证 route/egress/collision，随后拒绝工厂 binding 漂移。Worker 类型已支持 additive Activity，但生产 `index.ts` 尚未加载 plan、创建 RPC/Client 或实际注册，外部网络继续关闭。
- **本轮进展：** managed Worker startup plan 已固定 `load -> validate -> resource -> compose` 生命周期。disabled 与静态 composition 冲突都不创建 resource；资源后的取消/组合失败先 rollback close，成功句柄幂等关闭并对构造/清理错误固定脱敏。该层仍不创建 Temporal Worker/Client 或执行 readiness/network；生产接线、真实 read-only Capability definition、Shadow route 与停止顺序集成证据仍缺失。
- **本轮进展：** default-off Worker lifecycle owner 已组合现有 Temporal Runtime 与 managed startup snapshot。disabled 零 Worker 调用；构造/启动失败回收 resource；正常关闭严格执行 Worker/connection stop 后再关 Core/Artifact resource，并在任一失败下继续清理且幂等脱敏。该层尚未接入 `index.ts`/Compose 或真实 RPC；生产 Shadow 配置、readiness publication 与进程级信号集成证据仍缺失。
- **本轮进展：** 首个代码拥有的外部只读 definition `repository.issue.read` 已固定严格参数、规范化单 Issue resource scope、read 权限与 1 KiB egress ceiling。Definition Registry 可 seal 且冻结 authority snapshot，deployment 无法追加定义或扩权。生产 startup 尚未使用该 factory；受控 Profile/route manifest、RPC resource factory 与 Shadow 运行证据仍缺失。
- **本轮进展：** 默认关闭的 Agent Capability RPC resource factory 已将一个认证 client 精确复用为 MCP Core/Artifact ports，并在 transport 前拒绝 disabled RPC、空 Profile 和跨 Shadow tenant deployment。取消回滚与显式 close 均幂等脱敏。生产 startup 尚未组合 definition/resource/lifecycle，`index.ts`、Compose、readiness publication 和外部网络仍关闭。
- **本轮进展：** default-off Shadow Worker bootstrap 已组合 seal definitions、deployment loader、lazy RPC resource、managed startup 与 Temporal lifecycle，固定 disabled 零 RPC/Worker、副作用交接和单 owner 回滚。它已是可调用的完整 Worker root，但生产 `index.ts`/Compose 未接线，Workflow client、受控 manifests/readiness 发布、Shadow 演练和进程信号集成证据仍缺失。
- **本轮进展：** RPC resource 现用同一认证 client 同时派生 persistent Workflow lifecycle Activities、MCP Core 与 Artifact writer，并保留 host Step Activity；startup 在 RPC 前校验 base snapshot、RPC 后重新校验实际冻结 snapshot，消除未来 `index.ts` 另建 lifecycle RPC transport 的需求。生产 activity-mode 约束、Workflow client authority 与真实进程接线仍缺失。
- **本轮进展：** 新增可信外部 MCP Shadow dispatcher 原语。它复算确定性 Task ID，从已验证事件/身份固化 admission 与 goal，并把冻结快照交给宿主 selector；selector 的严格输出只能含 route ID 和业务参数，版本/manifest authority 继续由 Workflow catalog 注入。当前 subscription match 仍只投影 `subscriptionId`，尚未保存或装配 definition-to-route 映射；生产 Workflow Client、`index.ts`、Compose 与真实 Shadow 运行继续关闭。
- **本轮进展：** subscription match 现将 Core 已授权的 winning definition ID/version、tenant 与 Agent 连同 subscription ID 固化到可选事件 binding；代码拥有的 route selector 只按 exact definition version 路由，并在参数 resolver 前复核 subscription/tenant/Agent。旧事件保持兼容，生产尚无 route/resolver 注册、受管 Workflow Client 或进程接线，外部 Worker 与网络继续关闭。
- **本轮进展：** 新增受管外部 MCP Temporal Client lifecycle。Worker owner 冻结其实际 Temporal config，Client 复用同一地址、namespace、task queue 与 exact route catalog；disabled 零副作用，取消/构造失败回滚，停止时 drain 已接受 start 后幂等关连接且不越权关闭 Worker。生产 route registration、Kafka/Client/Worker 进程级装配、`index.ts`、Compose 与真实 Shadow 证据仍缺失。
- **本轮进展：** 新增外部 MCP Shadow Temporal process owner，将 Worker bootstrap 与 managed Client 收敛到一次所有权交接和同一取消边界。Client 失败回收 Worker；停止严格执行 Client drain/close 后再停 Worker/Core，任一失败仍继续且幂等。Kafka consumer 与该 owner 的进程级装配、生产 route/resolver、`index.ts`、Compose、readiness 发布和真实 Shadow 证据仍未接线。
- **本轮进展：** 新增 Kafka + 外部 MCP Temporal Shadow process owner，只允许 subscription trigger，按 Temporal 后 Kafka 启动、Kafka 后 Temporal 停止，并覆盖取消和部分启动回滚。subscription matcher 从 Worker 持有的同一认证 RPC resource 逐层借用，Kafka 不再创建重复 RPC transport；matcher 缺失会在 Kafka 构造前回收 Temporal。生产 route/resolver、`index.ts`、Compose、readiness 发布与真实 Shadow 证据仍未接线。
- **本轮进展：** deployment route manifest 现可选绑定 exact Agent Definition ID/version 与经 Capability schema/egress 校验的静态参数，并把完整 trigger mapping 纳入 route digest。Worker composition 在 RPC 前拒绝重复或 catalog 外 mapping，冻结快照后由 production selector factory 使用。旧 manifest 保持可读但没有 trigger authority；独占 Temporal activity mode、`index.ts` 生命周期、Compose 与真实只读 Shadow 演练仍待接线。
- **本轮进展：** `external_mcp_shadow` 已作为独占 Temporal activity mode 接入 `index.ts`：Profile/Temporal/Kafka subscription/RPC 必须完整对齐，旧 Kafka 与旧 Worker 在该 mode 下不构造，统一 process 负责启动回滚和 Kafka-first 停止。Compose 默认仍关闭。隔离 in-memory Temporal 与现代 MCP Client/Server 演练通过 17 项，覆盖 Worker replacement/resume/cancel 和 `tools/call=0` discovery；真实 production manifests、Core/MySQL、Kafka、凭据、DNS/TLS 与公网 readiness evidence 联合演练仍缺失。
- **本轮进展：** 增加可重复的隔离全栈 Shadow 演练：独立 MySQL/Kafka、临时 Temporal、owner-only route manifest、可信 Core 夹具和本地只读 MCP 串联同一 subscription event；持久 ledger 重启重放保持一次 Tool/Artifact，过期 readiness 在 raw Registry 前阻断第二次 Tool。低敏证据明确 `production_authority=false`，本地 transport boundary 不替代真实公网 DNS/TLS、凭据和 Core mTLS 演练。
- **本轮进展：** 隔离演练证据已固定为语言中立 v1 Schema，并由 Runtime 创建器与独立 CLI 共同校验 strict 字段、成功不变量、canonical content SHA-256 和最多 24 小时有效期；未同步更新 hash 的内容漂移及自然过期文件无法通过脚本。content hash 不提供签名来源、防伪造能力或生产授权。
- **本轮进展：** 全栈演练 Core 边界已从进程内夹具替换为测试专用 Go mTLS RPC fixture，复用生产 TLS 1.3、客户端证书验证、secret metadata、caller allowlist/CN 绑定和正式 TS RPC Client。错误 secret、错误 CN、缺失客户端证书及完整 12 类 RPC 链均有证据；v2 仍固定 `production_authority=false`，共享 Core 与真实外部身份尚未覆盖。
- **本轮进展：** 增加离线 encrypted-file Credential 生命周期演练与严格低敏 v1 证据。真实临时 key/envelope、Catalog 原子替换和 production I/O Registry 证明 v3 到 v4 的版本切换、重建恢复、旧/当前版本吊销均在新 Transport 构造前 fail closed，三次成功连接全部关闭；证据明确没有在途请求主动吊销或生产 authority。共享 provider owner、真实 Server 端失效和公网 TLS 联合演练仍缺失。
- **本轮进展：** `NodeExternalMcpDnsResolver` 已提供每请求独立、无缓存的 production DNS 实现，并行解析 A/AAAA、接受单族无记录、对部分瞬时失败和畸形证据 fail closed，AbortSignal 仅取消请求本地 Resolver。Network Guard 仍负责公网地址、去重和 32 条上限复核；真实 pinned TLS Dispatcher 与加密 Secret backend 尚未实现。
- **风险：** 第一方交换尚未提供 RFC 9728 Protected Resource Metadata、Authorization Server Metadata、OAuth 2.1 Authorization Code + PKCE 和第三方客户端注册，因此还不能声明为通用 MCP OAuth Server。encrypted-file Secret、Node DNS、pinned TLS Dispatcher、文件 CA adapter、隔离 Core mTLS composition 与离线凭据生命周期演练已实现；共享环境仍缺 provider owner 授权、KMS/Secret Manager、真实公网 DNS/证书链/peer pinning、下游 Server 吊销及真实 Core 身份联合证据。Catalog 只能阻断新连接，无法主动中断已发出的远端请求。fresh byte buffer 可覆盖，MCP SDK 所需的 JavaScript token 字符串与 Header copy 仍由 GC 管理，无法证明强零化。Catalog 文件依赖 OS mount/owner 信任，没有签名、rollback revision 或可用性告警；拒绝 symlink 也意味着 Kubernetes 默认 projected volume 不能直接使用。Tempo local backend 只适合单机 Shadow/验收，尚无生产对象存储生命周期、Alertmanager 通知链和长期 trace/audit 联查证据。结构化 egress guard 无法识别被改名、编码或嵌入普通文本的敏感值；`trust=untrusted` 提供上下文隔离语义，模型仍可能受 Prompt Injection 影响，接入时还需 trajectory Eval、Tool Policy 和输出 lineage 共同约束。Round receipt 可恢复本地已接收并持久化的成功结果；远端完成副作用后、Runtime 收到结果前或本地收据提交前仍存在无法由本地数据库证明的极小窗口。当前策略把传输异常持久为 `remote_outcome_unknown`，后续重放失败关闭，牺牲自动恢复来避免重复副作用；具备服务端幂等键或查询收据的 Profile 才能进一步收敛。Approval consumption 提供 at-most-once 副作用门槛；Message Command receipt 与 migration v31 action reference 已连接 Approval、Command 和权威 Message UUID，消费后 operation 失败仍需新审批。Command RPC 在服务端提交后遇到客户端 deadline/cancellation 时仍需 receipt-driven action-lineage 收敛演练。promotion operator Grant 仍依赖受控运维预置，生产 authorizer 仍未注入；approved Capability 已有显式 allowlist 投影与 Go/TS 双重校验，UI 风险摘要和真实故障演练仍缺失。
- **建议方向：** 接入真实 OAuth 2.1 Authorization Server 后发布 discovery/PKCE；为现有 Factory 实现加密 Secret Provider、真实 DNS Resolver 和按批准地址建连的 TLS pinned Dispatcher，绝不把 secret 返回 Runtime。外部 Tool 结果作为 untrusted Context fragment。生产 trace 使用对象存储与通知链；write Tool 必须绑定现有 Approval 与 Agent lineage。手工 MRTR continuation、round receipt 和三 ID Worker dispatcher 已固定恢复与命令权威边界，后续仍需专用默认关闭 Worker 装配、多轮策略与敏感授权隔离。
- **处理门槛：** 任何共享环境 MCP 开关启用、外部 Server 连接或 write/destructive Tool 上线前完成。当前网络入口仅用于受控认证和授权边界验证。

### AD-036：Elicitation 缺少 MCP continuation 与敏感授权隔离

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Human-in-the-loop、Web 客户端、MCP 集成、凭据与第三方授权
- **现状：** `dipole.agent.elicitation.v1` 已固定 text/select/multiselect/boolean Form、动态响应校验、大小上限和绝对截止时间；Gateway JWT API 经 Core Task owner 复核后发送精确 request ID 的 Temporal Signal，Worker 替换可恢复同一等待点和 Timer，到期自动以 `input_expired` 取消。默认关闭的 MCP adapter 已将受限 form mode 映射为 `wait_input`，以 checkpoint 绑定 untrusted Server/Tool/Invocation/Form/deadline，并拒绝 URL、敏感字段与有损 schema。MCP `2026-07-28` Client seam 显式声明 Form Elicitation 并关闭进程内自动 fulfilment；手工 MRTR continuation 只接受一个 input request，将原 Tool 参数、请求键、可选 opaque `requestState` 和 lineage 绑定到完整性 checkpoint，并可在新连接中精确生成下一次 `tools/call`。真实 SDK Streamable HTTP 双轮契约已通过。canonical Pencil 与默认关闭的 Vue 页面覆盖 desktop/mobile 普通 Form、来源披露和七态，经 authenticated Task query/input/cancel API 精确提交当前 Task/request；Runtime 与 Web 均拒绝凭据类字段。Chromium、Firefox、WebKit 已验证精确请求绑定、首次失败后的 stale Form 清理与恢复、键盘错误聚焦、ARIA 关系和移动端单列布局。当前尚未把 continuation 装配进生产 Temporal Activity/外部 Transport Factory，也未交付多轮、敏感授权 URL mode、产品入口编排和视觉回归基线。
- **风险：** 浏览器闭环只能恢复已进入 `waiting_input` 的 Task；MCP Server 仍无法在 durable input 完成后恢复原 Tool 调用。将密码、Token 或 OAuth 信息放入普通 Form 会进入 HTTP、日志或 Workflow history，扩大敏感数据暴露面；未来生产接线仍需处理连接丢失、用户取消和 Server 无恢复能力等差异。
- **建议方向：** 保持普通 Form 的字段白名单和默认关闭灰度，后续补充 Pencil 视觉回归与产品入口编排。Activity-safe runner 已能跨实例重开现代 Client、校验 tenant-owned Profile 并关闭失败资源；下一步将其接入独立默认关闭的 Worker mode，并固定持久 Tool invocation、progress/cancel 和审计映射。第三方授权继续采用独立 URL mode、短期 challenge 与回调绑定。
- **处理门槛：** Project Guardian 的普通 Form UI 已完成并保持默认关闭；任何凭据、支付、OAuth 或外部 MCP Elicitation 上线前完成独立敏感输入隔离、continuation 和威胁建模。

### AD-035：Memory foundation 缺少受控写入、版本纠正与压缩治理

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Memory、Context Compiler、隐私删除、长期事实质量、Project Guardian 演示
- **现状：** migration v29 与 sqlc Store 已保存五类不可变 scoped Memory、full/compact content、priority、有效期和 provenance；Core 根据运行中的 Task/Run 固定 principal、tenant、Agent 与 conversation read scope，并使用 Task 创建时间阻止后续新增记录进入重放，撤销/过期立即 fail closed。v38 已交付默认关闭的 owner list/revoke，v39 已交付 append-only correction 与五类离线结构 Eval，v40 提供 root-wide 内容擦除收据。v41 建立 `Memory -> Task` 直接引用与低敏影响审计。v42 将 lineage 生命周期绑定到权威 `agent_tasks`，`ModelShadowPlanner` 在 Context 编译后、任何模型调用前原子保存所选 Memory；写入失败时模型零调用，Plan-time 写入继续作为幂等修复且不会降级 `context_pre_model` 来源。历史 Context 只探测引用，真正缺少 Plan 和 lineage 的 owner 模型结果继续计入 `unattributedModelTasks` 并阻断完整声明。语言中立 derived-retention v1 现已完整覆盖七个持久域，离线决策绑定 policy/report/decision SHA-256，并从 lineage 完整性与人工复核域重新推导阻断原因；固定不读取正文、不执行删除且不授予删除或 Runtime 权威。历史回填已具备固定 high-water mark manifest/receipt、v43 MySQL checkpoint、sqlc source/target、owner-scoped adapter、可重放 Go runner、默认 dry-run/独立 operator/approver 审批 CLI、只读 rollout review CLI、OCI/config provenance collector、deployment evidence contract/CLI 和隔离 MySQL resume/owner/up-down 测试，仍未接入共享环境生产启动链或自动执行。Core 擦除、TS 审计、owner 页面和 correction 各自保持独立开关，自动写入继续关闭。
- **风险：** v42 已消除受管 Model planner 的 Context 到模型调用归因窗口，逐域策略也已可离线判定，但尚未证明字段级副本或实现 Shadow plan summary、Step input/output、Artifact body/metadata、Agent Message 与 Temporal history 的擦除/到期执行器。历史回填虽已具备可验证的有界索引链路，仍未获得共享环境 rollout 证据，任何旁路/旧 Runtime 产生的无 lineage 模型结果仍会阻断完整声明。真实多次纠正、语义冲突和 retrieval ranking 标注语料仍缺失，仅按 priority 的精确 scope 检索无法衡量生产 recall、precision 和 context 成本。
- **建议方向：** 下一步定义 owner 可见的派生治理收据和有界历史 lineage 回填，再按逐域策略分别设计字段级执行器与故障恢复；在执行能力启用前归档真实 correction/retrieval corpus并增加离线 Observation/Reflection Worker。写入策略要求来源证据、置信度、TTL、幂等键和冲突合并；基于 retrieval Eval 比较 MySQL 精确检索、Elasticsearch hybrid/vector 与 reranker。
- **处理门槛：** 在共享环境自动写入消息 Memory、启用跨 Task 长期召回、开放 owner 擦除 API或根据 Memory 自动执行动作前，完成派生域删除语义、历史索引完整性、Temporal/对象存储治理、真实 owner correction/retrieval 验收及安全评审；当前仅允许受控 seed、Shadow 读取、只读影响审计及默认关闭的 owner 查看、撤销和追加纠正，内部擦除方法没有外部调用路径。
- **本轮进展：** 新增默认 shadow-only 的 Observation/Reflection worker 与 `memory-candidate.v1` 严格契约。Observation 以事件 ID 幂等提取决定、任务和风险片段，Reflection 仅聚合同租户/主体/Agent/资源范围内的唯一 evidence window；两者都不访问模型、数据库、Kafka 或 Memory sink。超限、凭据模式、跨范围和重复窗口均 fail closed，后续仍需 candidate ledger、人工评审、Temporal durable 编排和真实 reviewed corpus。
- **本轮进展：** migration v45 与 TS MySQL ledger 持久化候选摘要、来源/证据 ID、策略版本、规范哈希和 `pending|accepted|rejected` 状态；重复候选必须通过 exact hash，冲突 fail closed，完整 candidate content 不进入 SQL 参数。ledger 仍不授予 `agent_memories` 写权限，人工评审、accepted 投影、Temporal receipt 和真实 corpus 继续待完成。
- **本轮进展：** migration v46 增加 append-only review ledger；`accepted|rejected` 记录绑定 candidate hash、reviewer、有限理由、时间和 review hash，并与候选状态在同一事务内更新。精确重复审查可重放，candidate/hash/status 漂移回滚；v46 回滚保留 v45 候选且不删除 Memory。accepted 到 `agent_memories` 的 Core 投影、Temporal receipt、双人/owner 策略和真实 reviewed corpus 仍未接线。
- **本轮进展：** migration v47 增加 promotion receipt，Core-owned service 与 sqlc Repository 在同一事务中锁定 accepted candidate/review、写入摘要型 observational Memory 并记录 `promoted_memory_uuid`；稳定重试返回既有 Memory，候选/审核/状态漂移回滚。公开 RPC、Temporal receipt、双人审批策略、真实 corpus 与自动写入开关仍未接线。
- **本轮进展：** 增加 `PromoteMemoryCandidate` additive gRPC 与 Gateway HTTP 控制入口。Gateway 只提交候选 ID、候选哈希和 review ID，Core 从认证 principal 派生 owner 并调用 v47 promotion service；Gateway/TS client 对返回 Memory 的来源、review 绑定和 active 状态进行复核。Temporal 自动触发、Runtime 旁路、双人审批和真实 reviewed corpus 继续关闭。
- **本轮进展：** 增加 `agent-memory-promotion-receipt.v1` 语言中立契约和 Temporal preparation Activity。receipt 仅绑定 Task/Run、owner、候选/审核摘要与短时效窗口，支持确定性重放和 fail-closed 过期检查；当前只形成 durable intent，未开放 Runtime 直接写 Memory、自动晋级或生产灰度。
- **本轮进展：** 增加 Memory reviewed corpus v1、双 reviewer/独立 adjudicator 门禁和离线 CLI。输入不含消息正文，只绑定候选类型、资源、证据数量与内容哈希；报告不输出 case/reviewer 标识，分歧、覆盖不完整、gold drift 或 corpus hash 漂移均 fail closed。当前仓库仅有脱敏测试夹具，真实 owner-approved corpus、retrieval ranking 标注和灰度证据仍待完成。
- **本轮进展：** 增加 source manifest v1 与安全加载器，要求 owner UID、绝对规范路径、无符号链接、严格文件权限/大小、批准时间窗口以及 corpus/review 双哈希一致；该边界允许后续接入真实脱敏文件，同时阻断任意本地文件冒充已批准语料。当前仍缺真实 owner-approved corpus、发布签名和共享环境评测证据。
- **本轮进展：** 增加 Memory prefilter evidence v1 与 `eval:memory-prefilter` 离线 CLI，embedding/small_model 候选必须完整绑定 reviewed corpus、配置哈希和 score/threshold；报告仅含聚合分类、延迟、成本指标与门禁原因，缺失 case、哈希漂移或阈值漂移均 fail closed。当前仍缺真实 owner-approved corpus、embedding/小模型采集、retrieval ranking 标注和在线灰度证据。
- **本轮进展：** 增加 Memory prefilter rollout decision v1 与 `eval:memory-prefilter-rollout`，在发布判定前重新计算双 reviewer/gold 与 candidate evidence，绑定 corpus/review/final-label/evidence 哈希并以 `eligible|blocked` 输出。该门禁仍为离线证据，真实语料、候选采集、审批授权和在线灰度继续关闭。
- **本轮进展：** 增加 `runtime-binding.v1` 与 provider-neutral 三态 gate：`shadow` 只观察、`enforced` 仅接受精确绑定的 `eligible` 决策，所有模式固定无 Memory 写权。该 gate 尚未接入真实模型、Kafka subscription 或自动晋级，仍需真实语料和在线回切证据。
- **本轮进展：** 增加 Cassandra read rollout evidence v1 与 Go CLI，校验窗口/部署标识、路由计数不变量，并按样本量、观察比例、fallback、verification 和 Cassandra p95 门槛重算 `eligible|blocked`。当前仍缺真实共享环境 Prometheus 快照、责任人批准、快照/回放证据和生产回切窗口。

### AD-034：Event Subscription 缺少用户界面与语义预筛

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Trigger Engine、Definition 授权、模型成本、Gateway/前端配置与 Project Guardian 演示
- **现状：** migration v28 与 v34、sqlc Store、Core resolver 和受认证 RPC 已持久化精确 Definition version 订阅，并提供 Gateway principal 派生 owner 的创建、历史分页与可审计撤销。TS Runtime 可在 EventLedger、Temporal 和模型前确定性过滤。canonical Pencil 和默认关闭的 Gateway/Vue 页面已交付 owner list/create/revoke：创建候选由 Core 对 authenticated readable conversation 与 Definition scope 求交集，Gateway 从会话派生 principal/tenant 并从 conversation key 派生 event/resource，Core 在写入前再次复核；前端严格解析候选和权威结果，拒绝静默截断关键词。默认关闭的在线 Shadow 对照可在 direct-target 主路径进入 EventLedger 前调用同一 matcher，仅记录六种低基数结果和候选总数；matcher error 不阻断主路径，Prometheus 已提供 error/drift 告警。语言中立 prefilter Eval 已支持有界标签 corpus、三类 candidate evidence、分类/延迟/成本指标和生产规则基线；corpus review v1 要求双 reviewer 完整标签与第三方分歧裁决。Compose 与默认配置继续使用 `direct_target` 且 Shadow 关闭。
- **风险：** 控制面已可安全创建确定性订阅，但 Runtime 仍未消费共享 subscription 流量。在线 Shadow 已有 24 小时窗口、抓取覆盖、counter reset 和零 error 的低敏证据合同，并用只读 Collector 固定查询/单 series/持续启用检查，但尚未归档真实共享环境 evidence；当前指标也无法独立证明部署 artifact revision，仍需发布记录交叉核对。确定性关键词无法覆盖语义等价表达；公开候选接口当前返回有界 Definition scope 的完整交集，尚无超大 scope 分页协议。直接启用共享环境订阅模式仍会造成难以运维的策略或相关事件漏触发。
- **建议方向：** 使用真实 Project Guardian reviewed corpus 采集 embedding 与小模型 candidate evidence，并与规则基线比较；随后设计 subscription Runtime 的分批灰度、漏触发/成本告警和回切证据。若单 Definition scope 扩展到当前上限之外，再增加稳定 cursor 的候选分页。高成本 Agent 只接收预筛后的事件。
- **处理门槛：** Project Guardian 或共享环境启用 `subscription` 前完成用户管理界面，归档真实事件 corpus、reviewer agreement 和至少一个候选 evidence/report；synthetic 规则示例只证明 Harness。语义预筛需先离线达标，不能直接对每条消息调用大模型。

### AD-032：Artifact 对象写入后缺少孤儿清扫证据

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Artifact、MinIO 容量、MySQL 元数据、故障恢复与审计
- **现状：** migration v26 保存不可变 Artifact 元数据，正文使用 Task/Run/版本/内容哈希导出的确定性对象键。已增加固定前缀、24 小时门槛、sqlc 二次存在性查询和 SHA-256 报告的只读 dry-run；maintenance authorization/receipt 再绑定双审批、职责分离、15 分钟有效期、对象 Stat 与执行前元数据复核。三个离线/运行时身份均无删除权限。
- **风险：** MinIO 写入成功后若 MySQL 持续失败且任务不再重试，会留下无法从用户 API 引用的内容寻址对象。该对象不会覆盖其他版本，也不会获得读取授权，但会长期占用容量。
- **建议方向：** 在真实 Shadow 观察窗口持续归档 dry-run 报告和 receipt；是否增加 DeleteObject-capable 执行器需单独评审，并要求新的不可回退契约版本、独立删除身份、对象版本/保留策略、审批持久化和删除后 receipt，现有 Runtime/Core/audit/inspect 账号继续没有删除权限。
- **处理门槛：** Artifact 进入 active 模式或配置自动保留期限前完成；Shadow 阶段以容量指标和人工审计接受该风险。

### AD-031：Context Token 预算使用确定性近似估算

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** `agent-runtime`、Context Compiler、多模型路由、长上下文与成本门禁
- **现状：** 显式启用的 Context Compiler v2 已支持 route 声明 context window、UTF-8 bytes/token 校准值与安全余量，并对所有候选 route 取最大估算和最小窗口；未声明 route 使用固定保守 fallback，配置 SHA-256 estimator ID 随 Plan manifest 持久化。语言中立 evidence/report 与离线 CLI 已要求每个 route 覆盖中英文、代码、Emoji、Tool schema，逐项记录 reference/estimate/error、正文哈希及 provider revision。默认与 Compose 保持 v1，保护在途不可变 Plan 重放；实际 provider usage 继续由 ModelAuditStore 在调用后记录。
- **风险：** 不同模型 tokenizer、中文、多字节符号和 JSON 转义会产生估算偏差。接近模型窗口上限时，近似值可能低估输入并触发 provider 拒绝，也可能高估后过早省略证据。
- **建议方向：** 使用现有 evidence/report 契约按 route 归档真实 tokenizer 或 provider usage synthetic 校准集；比较估算/实测误差分布后再缩小 fallback 余量。对缺少可复现 tokenizer 的 provider 保持保守 profile，不根据单次 usage 自动学习或静默改变预算。CLI 的 `eligible` 只代表输入 corpus 零低估且无 fallback，生产启用仍需独立候选评审。
- **处理门槛：** 在 Context 接近任一生产模型窗口的 70%，或引入多模型动态上下文窗口前归档真实 route 校准证据；当前固定 4096 Token 编译预算与启动窗口门禁允许继续 Shadow 观察。

### AD-030：TypeScript Agent 尚缺受认证的远程 Capability 传输

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `agent-runtime`、Core/Message/Conversation 边界、可信身份、只读 Step 执行
- **解决方式：** migration v21 增加与 Task 分离的 `agent_runs` 和 Step claim token/lease；受认证 `dipole-agent` 先通过 admission 固定 Task、Definition version 与 runtime Run，再以 Task/Run 调用 `conversation.list`。Core 服务端从持久 Task 解析 principal、permission 和 resource scope，拒绝 protobuf `RequestContext` 中的模型可控 principal。TS 使用 canonical proto 静态生成 grpc-js client，通过 Capability Registry 执行并持久化 Step result/error；Run completion 支持幂等网络重试。Agent mTLS 身份仅获 Admit/Complete/List 与 health 方法。
- **验证：** Go/TS Task/Run 黄金向量一致；伪造 principal、Runtime binding 和 Agent 调用其他 Core RPC 均被拒绝。真实 MySQL 8.4 覆盖 Run create/replay/CAS、Step 并发 claim、失败重领、旧 token 拒绝和完成 no-op；migration v21 完成 `up→down→up`。真实 Go Core 与 Node grpc-js 通过 loopback 共享密钥完成 admission/list/complete/replay，replay 返回同一 completed Run。
- **长期约束：** 当前远程能力保持只读 shadow，公开 HTTP 旁路继续禁止。新增 Capability 必须先完成 descriptor、服务端持久策略解析、最小 RPC allowlist、Step 轨迹和真实权限测试；write/destructive 能力等待 Approval 与 Temporal 状态机。

### AD-029：Agent 模型预算与调用轨迹尚未跨重试持久化

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `agent-runtime`、模型成本、Kafka retry、Run/Step 审计与故障恢复
- **解决方式：** migration v19 与 MySQL ModelAuditStore 以 Task 唯一 Run 固定预算快照，provider 调用前事务预留 slot，成功/失败写入 route、usage、finish reason、latency 与错误，Run 终止时将遗留 reservation 收敛为 `abandoned`。ModelRouter 已在每条 provider 路径调用 Store，持久写失败禁止 fallback；AI SDK 模式强制 MySQL Store并在 readiness 前探测 v19。无 slot 重试按 Task 条件收敛仍在 running 的 Run。
- **验证：** 真实 MySQL 8.4 连续三轮 16 路并发均只授予 3 个 slot；策略漂移、旧终态更新和越权 Core 表访问被拒绝。两个独立 Router 模拟同一 Kafka Task 重投时 provider 总调用固定为 2，第二次重投获得 0 slot，Run 保留 `calls_reserved=2` 并进入 failed。43 项常规 TS 和 5 项真实 Store 测试通过。
- **长期约束：** AI SDK 内部 retry 保持为 0；所有新增模型调用入口必须先预留持久 slot。Temporal 接入后复用同一 Task/Run，不另建可绕过预算的 Workflow retry 计数器；Tool/Approval/Artifact 使用独立 Step 轨迹扩展。

### AD-028：Agent Kafka 失败转移尚未接入 retry/DLQ

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `agent-runtime`、Kafka poison event、失败重试、offset 提交与故障恢复
- **解决方式：** Agent Runtime 使用 `<prefix>.<topic>`、`.retry`、`.dead` 三个显式 topic；无效 envelope 与 tombstone 直接进入 dead，处理错误按 `retry_attempt` 有界转移，达到上限后以 `handler_failed` 终止。转移保留原始 key/value/header，并增加 `original_topic`、`last_error`、`dead_reason` 和时间诊断。只有失败消息发布成功后 KafkaJS handler 才返回；publisher 异常向上抛出，保留源消息的未完成语义。启动时仅创建缺失 topic，并在 readiness 前验证分区数和副本数。
- **验证：** 31 项 TypeScript 测试覆盖永久失败、tombstone、重试上限、原始 metadata 和 publisher reject。真实 Kafka 3.9 验证 poison event 直达 dead，ledger 绑定冲突经过两次 retry 后以 `retry_attempt=2` 进入 dead；两副本加入/退出触发 rebalance 后 partition 4 均继续消费到 LAG 0。Compose 使用 6 分区和可配置副本数。
- **长期约束：** retry/dead topic 必须与主 topic 使用相同分区数和副本数；新增事件类型需先分类永久/瞬时错误。Temporal 接入后复用持久 Task ID 作为 Workflow ID，不另建重复幂等键。

### AD-026：Readiness 尚未持续感知运行期依赖退化

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** Core、Gateway、Message、Sync、Search、Search Indexer、Cassandra Projector 的流量摘除与故障诊断
- **解决方式：** 统一 metrics listener 增加异步缓存探针、逐探针超时和失败/恢复双阈值；服务生命周期状态与关键依赖状态共同决定 HTTP readiness 和 gRPC health。Core、Gateway、Message、Sync、Search、Search Indexer、Cassandra Projector 均按运行责任接入关键依赖，影子读、可回退存储、Kafka backlog 和可选能力继续使用专项指标。默认配置保持关闭，微服务 Compose 以 5 秒周期、1 秒超时、3 次失败、2 次恢复启用。
- **验证：** race 测试覆盖超时缓存、滞回、防排空反转和状态回调；gRPC health 回归验证 readiness 同步。Elasticsearch 隔离演练确认 Search 与 Search Indexer 退出并恢复 ready，Core、Message、Sync、Gateway 全程 ready，六个应用容器 ID 均未变化。Prometheus 规则测试覆盖具体 `service/dependency` 告警。
- **长期约束：** 新依赖只有在其故障会阻止服务正确处理当前职责时才加入 readiness；可选能力和有验证回退的存储继续通过有界指标告警。探针不得在 `/readyz` 请求路径执行网络 IO，新增探针需提供失败/恢复防抖与隔离故障演练。

### AD-019：MySQL 消息正文退役缺少完整替代读契约

- **本轮验证：** 真实隔离 Cassandra 读路由与 Sync hydration smoke 均通过主读、缺失/损坏回退和 Metadata 回填；证据仍属于隔离环境，未满足共享环境长期观测、责任人批准和兼容窗口退出条件。

- **本轮进展：** 真实隔离 MySQL/Cassandra smoke 已通过 hydration shadow、重复消息恢复、Legacy ID 恢复和 Metadata 回填；测试版本基线已修正至迁移 v47。共享环境主读灰度和旧 Offline 兼容窗口仍待完成。

- **本轮进展：** Gateway/WS 已接受 `message.timeline_notify_mode=primary` 并与 Web `VITE_TIMELINE_NOTIFY_MODE=primary` 对齐；通知仍只携带 locator，客户端验证完整序列后补拉。Cassandra 主读比例、共享环境 Prometheus 窗口和旧 Offline 兼容期仍未晋级，故该债务保持进行中。

- **本轮进展：** Web 已增加默认关闭的 `VITE_TIMELINE_NOTIFY_MODE=primary` 客户端路径，通知驱动的 Timeline 补拉会在序列和 UUID 完整校验后才合并消息；服务端 Cassandra 主读灰度、共享环境观测和旧 Offline 兼容窗口仍按既有门禁执行。

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Cassandra 主读、Sync Timeline、消息幂等、文件授权、搜索重建、迁移回放
- **现状：** `user_sync_inbox` 已持久化并对外暴露 `conversation_key + message_uuid + message_seq` locator。Sync Service 已建立 storage-neutral hydrator，可在返回 MySQL 正文的同时异步比较 Cassandra Timeline；Cassandra 尚未承担 Sync 主读。Direct 与 Group Timeline 均已具备 `after_seq` HTTP/Message v1 gRPC 增量契约，Local/Remote/Shadow adapters 一致，并复用 Cassandra cohort、连续页校验与 MySQL fallback。Gateway 已增加默认关闭的 `sync.item.notify.v1` body-free shadow 通知，Web verifier 会按会话补拉、去重并验证 locator；现有完整消息投递和热群聚合 notify + pull 保持不变。Web 已增加默认关闭的 IndexedDB Sync Engine、shadow 门禁和热群持久 ACK。migration v12 增加无正文 `message_metadata`，与 Message/Inbox/Outbox 原子提交并回填历史 locator；文件授权已改查 Metadata，删除完整 Message 行后仍可验证访问和过期时间。重复发送先通过 Metadata 校验身份，并可在默认关闭的开关下按会话 Seq 从 Cassandra 恢复原响应，缺失/冲突继续回退 MySQL。Cassandra Backfill/Reconciler 已支持经 SHA-256 校验的不可变完整消息归档，Job 绑定 source identity；真实演练删除 MySQL 正文后仍可恢复和全量对账。Message 最小账号暂时保留 `groups/group_members` 只读权限用于旧 Offline 与群文件授权。
- **风险：** 提前停止正文写入仍会让多端同步和重复发送响应缺失正文，并丢失 Cassandra 修复与回滚基准。文件授权的正文依赖已解除，但群文件授权仍需 Core 成员关系。
- **本轮进展：** Sync 新增默认关闭的 Cassandra-first hydration adapter；primary 与 shadow 配置互斥，primary 失败按同一 locator 回退 MySQL，取消或双失败均不返回部分正文。该路径已覆盖命中、回退、双失败和启动配置测试，尚未接入灰度比例、Prometheus 停止门禁或真实主读流量。
- **本轮进展：** 增加 Sync Cassandra hydration evidence v1 与 Go CLI，按窗口和部署 revision 绑定 shadow/primary 聚合指标，重算命中、fallback、缺失、冲突、错误与 p95 门禁。真实客户端流量、Prometheus 原始快照、责任人批准和主读回切证据仍缺失。
- **建议方向：** A5 Search 与 A4 Cassandra 均已具备不可变归档恢复源；重复发送 hydration 与 Timeline notification shadow 均已具备严格 24 小时晋级规则。Web Sync 观察现可用候选 commit/bundle 哈希绑定的 Session/Evidence 归档，仍需在完整服务 Prometheus 和真实客户端流量上运行并固定对象版本。随后继续通知 shadow 证据归档、Sync Cassandra hydration 主读/fallback 和重复发送 hydration 灰度，再引入 `full / metadata_only` 写模式。
- **处理门槛：** 完成固定快照备份与校验、事件回放演练、Sync/Offline 比较、幂等和文件授权契约、至少一个兼容窗口的 Cassandra 稳定主读，并记录可执行回滚期限与责任人；旧 Offline 退役后撤销 Message 对 `groups/group_members` 的临时读取。

### AD-021：Search 重建依赖 Outbox 事件保留契约

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **影响范围：** Elasticsearch 全量重建、事件归档、Outbox 清理、MySQL 消息正文退役
- **现状：** `dipole-search-archive` 可按固定 Outbox mutation 高水位流式导出最终状态 NDJSON 与 SHA-256 manifest，并发布到独立 MinIO object-lock bucket。`dipole-search-outbox-cleanup` 只接受可按精确对象版本恢复的 receipt、已完成且一致的 Reconcile 报告和匹配的 Backfill Job；默认 dry-run，执行时强制维护窗口确认与 operator。sqlc 查询仅删除水位内、已发布的八类 Search mutation，遇到未发布事件时拒绝清理。
- **解决记录：** 2026-08-27 完成专用 `search.mysql.*` 配置和最小授权模板；单测验证批次中断后可重入。真实 MySQL/MinIO/Elasticsearch 演练按 2/2/1 删除 5 条 eligible mutation，保留无关 Outbox，维护账号访问 Core 表被拒绝；随后仅凭保留对象版本从空索引恢复并完成 3/3 hash 对账、Alias 正向切换与回滚。
- **长期约束：** 禁止手工批量删除 Outbox。每次执行必须保存 operator、snapshot/object version、Reconcile 时间、高水位和删除统计；对象保留期、清理窗口或 mutation 类型变化时重新评审本条契约。

### AD-017：Redis Pub/Sub 切主窗口保持 at-most-once 语义

- **本轮验证：** Redis Sentinel 真实三节点故障演练已验证 master 切换和 replica 重加入期间的客户端恢复、Presence、Hot Group 与限流语义；Pub/Sub 在切主瞬间的已发布消息仍无法补读，持久可靠性继续由 Kafka/Sync Timeline 承担。

- **优先级：** P2
- **状态：** 接受风险
- **发现日期：** 2026-08-26
- **影响范围：** Gateway 跨节点在线投递、Redis Sentinel 故障转移、后续 C++ Realtime Delivery
- **现状：** go-redis 会在 Sentinel 选出新 master 后重连命令与 Pub/Sub 连接；连接中断期间已经发布的 Pub/Sub 消息无法补读。Gateway 的 Kafka handler 当前将跨节点 Pub/Sub 视为实时通知通道。
- **风险：** master 切换窗口内，在线用户可能暂时缺少一条跨节点通知；Redis Sentinel 无法提供持久队列或消费位点。
- **接受依据：** 消息事实、用户 Inbox、设备 Cursor 和热群 checkpoint 均保存在 MySQL/Kafka 链路，客户端重连或增量同步能够恢复已确认消息；Redis 只承担实时状态。
- **阶段记录：** 2026-08-28 已建立 `dipole.delivery.v1` envelope、节点批次、逐项 ACK/error 与背压契约，并固定 Kafka source coordinates 和 Go legacy adapter；C++ shadow 已接入独立 Kafka group、hiredis direct/Sentinel reader、单连接 TTL 投影、低敏 evidence v3、mTLS `ObserveNodeBatch` 和 assignment readiness。真实 Kafka+Redis+Gateway 演练覆盖故障保留 offset、同进程恢复重试、稳定 batch 去重、真实 queue saturation/backpressure、同 workload Go/C++ 40/40 对照与最终 lag 归零。`AD-039` 已关闭。默认关闭的 primary seam 提供 connection 定向入队、逐项 ACK、部分成功 connection 重试、有界 Gateway replay state 与 additive WebSocket delivery ID；Web 通过账户隔离的 IndexedDB v4 原子 claim 跨页面重载去重。C++ one-shot probe 经 mTLS 实际验证 `ENQUEUED(1)`、稳定重放去重与 stale Presence `OFFLINE`。显式 primary CLI 现使用独立 `dipole-realtime-primary-*` authority，要求 enable/Presence/transport 三重配置，并将 terminal ACK、低敏 primary evidence 与 Kafka commit 串联；partial/rejected/failed、身份漂移和故障保留同一 pending record。默认关闭的 `realtime-cpp` Compose profile 现提供独立 primary 部署描述，但 Go authority、Gateway primary RPC 和共享环境切换仍保持关闭，必须经过 C3 证据与维护窗口。
- **后续方向：** `benchmarks/c2-primary-runtime-2026-08-28/` 已验证真实 queue saturation、terminal ACK 后 commit、故障 retain、`SIGKILL` 后同坐标重放和 lag 归零；窄 terminal-evidence-to-commit 崩溃窗口仍未作确定性声明。C3 由 `AD-041` 继续跟踪互斥 authority 与自动回切。IndexedDB 不可用时 Web 保持 fail-open，持久记录按 4096 项容量淘汰；保留 Sync Timeline 作为存储故障、去重窗口外重放和进程崩溃窗口的最终补偿路径。
- **重新评估门槛：** 产品要求在线 push 本身具备不丢 SLA，或 Kafka consumer 在 Pub/Sub 发布失败后仍提交 offset 造成可观测缺口时。

### AD-015：Message Service 数据库账号尚未收敛表级权限

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `cmd/message-service`、File metadata、数据表所有权、最小权限
- **解决方式：** 增加继承全局配置的 `message.mysql.*` 专用凭据，独立 Runtime 不再读取 Core MySQL 凭据。`dipole_message` atomic 与 `dipole_message_projector` 两套 GRANT 仅开放 sqlc 实际使用的操作；启动探针逐项验证必要 SELECT/INSERT/UPDATE、拒绝多余 DELETE/UPDATE、Core 表访问和 projector Inbox 访问。微服务 Compose 在 migration 后创建账号，并默认启用 Message/Sync 权限门禁。
- **验证：** 真实 MySQL 8.4 smoke 验证 atomic 提交 Message/Metadata/Outbox/Inbox、projector 提交 Message/Metadata/Outbox 且 Inbox 为零，并拒绝 Core 和多余写权限；完整微服务镜像/Compose smoke 验证权限初始化、Message/Sync 健康启动、mTLS、Gateway/Core 路由。
- **长期约束：** `/messages/offline` 兼容期内保留 `groups/group_members` SELECT；旧接口退役后按 AD-019 撤销。新增 Message sqlc 写操作必须同步更新 GRANT、操作级探针和真实权限 smoke。

### AD-005：群消息成员级写扩散仍然叠加

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-26
- **影响范围：** 普通群 Inbox、Conversation State、热群吞吐
- **现状：** 普通群同时按成员更新 Conversation State 和 Inbox；热群仅跳过 Inbox，Conversation State 仍逐成员更新。
- **风险：** 两类投影职责独立，但成员级写入量会叠加，热群链路仍保留 `O(group_size)` 的会话状态写扩散。
- **基线证据：** 候选提交 `2202f1f` 的 baseline v2 证明 Conversation 写放大在 20/100 人普通与热群中均为 20x/100x，见 `benchmarks/ad005-2026-08-27/`。提交 `4343684` 的 baseline v3 进一步记录逐节点 Repository timing：group-message 单次平均为 12.43-23.07 ms，P95 桶上界为 25-50 ms，四组零错误；普通与热群的调用分布接近，而 100 人端到端 P95 分别为 8189 ms 与 1346 ms，支持 Inbox/投递路径是模式差异的重要来源。完整原始快照见 `benchmarks/ad005-projection-timing-2026-08-27/`。
- **建议方向：** 保留现有 Counter/Histogram 作为回归门禁；在 1000 人固定 workload 或候选实现中比较逐成员串行写、批量 upsert、异步分层投影与热群摘要读扩散，单独记录数据库累计时间、锁等待和投影恢复语义。
- **处理门槛：** 候选优化需在固定 workload 下减少 Conversation 累计写成本或端到端 P95，保持 Seq/read state、投影重放和回滚正确性，并通过普通/热群完整投递对照后才能关闭；当前 v3 证据已完成归因基线，尚未完成行为优化。

### AD-007：架构 Markdown 当前未纳入版本控制

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `docs/*.md`、架构决策可追溯性
- **解决方式：** 移除 `docs/*.md` 通配忽略，以 `docs/architecture-docs.manifest` 固定 canonical 架构文档集合；历史问答、面试资料和本地参考改用文件级忽略规则，`docs/architecture-reference.md` 继续保持本地未跟踪。
- **验证：** `scripts/check-architecture-docs.sh` 校验清单文件存在且已被 Git 跟踪，拒绝通配忽略回归，并验证本地架构参考仍被显式忽略。
- **长期约束：** 新增长期架构约束时同步更新 manifest、实现文档和更新日志；本地草稿晋级前先完成代码与配置对齐。

### AD-008：Agent Tool 允许模型提供用户身份参数

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `internal/modules/ai/tools.go`、会话读取、用户资料、系统消息发送
- **解决方式：** Embedded Go/Eino Service 从已校验的触发 Message 与关联上下文生成 `ExecutionContext`，注入 principal、Agent、触发消息、会话和 request/trace/event ID。五个 Tool Schema 均移除 `user_uuid`，读取和系统消息目标只使用上下文 principal；上下文缺失或发送 Agent 不匹配时 fail closed。
- **验证：** `dipole.agent.eval.v1` 保留两条恶意 `U999` 覆盖用例，结果改为 `identity.execution_context` 与 `principal_enforced`；单元测试覆盖全部 Tool 缺少上下文拒绝、schema 身份字段扫描、发送 Agent 不匹配和 Service 派生链。
- **后续边界：** tenant、委托身份和细粒度权限继续由 G1 Capability API 承担，不能重新加入模型可控身份参数。

### AD-009：Agent 持久任务生命周期尚未完成生产接管

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-26
- **影响范围：** `agent-runtime`、Temporal、长任务、审批、失败恢复和评测
- **现状：** migration v16-v29 已落地 Definition、Task、独立 Runtime Run、可重放模型输出/预算、不可变 Plan/Context manifest、带 lease 的 Step 终态、附加 Workflow projection、版本化 Artifact、Subscription 与 scoped Memory。Temporal Workflow 已持久化 Task/Run admission、三类 Run 终态、Approval/Input Signal 和 deadline Timer；默认关闭的 `read_shadow` 由 Kafka 启动稳定 Workflow，并在 Activity 内执行 ContextCompiler、ModelRouter、只读 Capability Step 和内容寻址 Artifact 创建。Message v1 Envelope 已通过可选 lineage传播根 Task，TS Runtime 在高成本处理前阻断同源 Agent 因果链。Gateway 已提供默认关闭的 JWT Task query/cancel/approval/input API；repair 审计 RPC 只接受 Gateway principal。离线对账与 Shadow 晋级保持只生成证据和 eligible/blocked 决策。Compose 继续关闭 Temporal、Task 控制桥并固定 `foundation`。
- **本轮进展：** MySQL migration integration baseline 已更新至 v44，并覆盖 execution ledger、lineage backfill、pre-model lineage 的连续回滚与表数量校验；本轮验证消除了迁移测试落后于实际 schema 的盲区。
- **本轮进展：** Repository 合同测试在隔离 MySQL 8.4.8 中验证 v44 prepared execution 的创建、精确重放和目标哈希冲突拒绝；没有增加状态推进、Projection 写入或公开执行入口。
- **本轮进展：** 增加受控 `prepared` 准备服务：复核已批准且未过期的 repair proposal、审批计数、Task 存在性和 proposal/task/executor 绑定，再通过 execution store 幂等创建并读取执行意图；该服务不推进状态、不修改 projection。由于 operator grant 当前没有版本字段，`executor_grant_version` 暂只保存计划绑定，运行时 grant 复核仍关闭。
- **本轮进展：** Agent Runtime 增加 `repair:plan` dry-run 计划编译器，按 execution-plan v1 生成确定性的 plan ID、当前/目标/回滚投影 SHA-256 和 15 分钟 CAS 窗口；批准状态、双审批人、独立执行人及 grant version 均在计划生成前校验。CLI 不连接 MySQL/Temporal，不改变 projection，也没有 apply/execute/rollback 入口。单测、类型检查和构建已通过。
- **本轮进展：** 追加 Workflow/Run 身份绑定校验，当前投影与目标投影必须属于同一运行实例，跨运行证据在 plan 编译阶段拒绝；新增回归测试并保持 v1 dry-run 与无写执行器边界。
- **本轮进展：** 增加 `repair:preflight` 二次采证器，按 plan/proposal/grant/current CAS 生成低敏 `ready|blocked` 收据；它不读取数据库、不调用 Temporal、不修改 projection，真实 executor 与生产 authority 继续关闭。
- **本轮进展：** migration v44 与 sqlc 新增 prepared execution ledger，持久化唯一 plan 的执行意图、提案/任务/执行人绑定和 CAS 摘要；应用接口仅支持创建/读取 prepared 记录，未增加状态推进或写入 RPC，便于后续 executor 在独立版本中实现可恢复提交与回滚。
- **风险：** v24 projection 保持 shadow 观察属性，尚未接管原 `agent_tasks.status`；当前 `read_shadow` 只允许 `conversation.list`，也没有 Memory 或真实任务终态 outcome Eval。v25 的 `approved` 只保存审计结论；execution plan v1 仅允许带 CAS/回滚证据的 dry-run，prepared 服务尚未执行状态推进、projection 修改或 rollback。操作员授权当前需要受控 SQL 配置，且 grant 没有可核验版本字段，无法证明执行时授权仍与计划一致。Temporal Worker 停止时 Query 会归类为 unavailable。eligible 决策不能自动切换 active。
- **基线证据：** 真实 Temporal Server 已验证 admission/Approval 历史恢复、单调 revision 投影、取消投影、完成态 Query/Describe 对账和 Activity 丢失完成 ACK 后的模型/Step 重放；真实 MySQL 8.4 已验证 v25 全链升降级、16 路同审批人重放仅一票、两位独立审批后批准，以及原 projection 并发与 shadow cohort keyset 契约。TypeScript/Go canonical evidence SHA-256 使用黄金向量对齐；gRPC 测试验证 Gateway principal 绑定和 Agent 最小权限拒绝。Kafka Shadow 与 Go/Eino 权威业务路径保持不变。
- **建议方向：** canonical Pencil 已维护 Repair evidence review、六态审计矩阵和 desktop/mobile 双人审批边界；下一步按该契约实现 Vue 恢复界面，并设计显式、可回滚、再次授权的 repair executor。完成真实 outcome/trajectory/permission Eval 证据后才评审权威 Task 与回复流量迁移。
- **处理门槛：** 上线 Durable Task 或 Event-driven Agent 前完成。

## 已关闭

### AD-011：前端缺少可版本化的完整设计基线

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-28
- **影响范围：** `frontend`、响应式布局、Agent UI、视觉一致性
- **解决方案：** 建立单一 canonical `design/dipole-ui.pen`、统一设计 token、可复用组件、Login/Chat desktop/mobile、Search 四态、Sync 恢复状态、Agent Workflow Repair 审计状态、关键异常状态、设计日志和评审导出图。
- **验证：** pen.dev CLI 识别 35 个顶层 frame 和 16 个可复用组件；相较关闭时的 23/10 基线，已增量加入 Repair 与 Elicitation 各三个组件和三张画板。结构检查保持零 placeholder、无新增未命名节点、无裁剪或布局告警；Login、Chat、Search、Sync、Repair、Elicitation 代表画板均完成渲染复核。
- **后续范围：** Vue token 映射和自动视觉回归继续由 F4 跟踪，不再阻塞 F1 设计基线关闭。
- **本轮进展：** F4 已新增 `frontend/src/styles/design-tokens.css`，并让 App 壳层与 Search 工作区引用 `--dp-*` token；Vitest 直接读取 `design/dipole-ui.pen` 的 variables 校验颜色、字体、间距和圆角，后续 token 漂移会在测试阶段暴露。页面流程、其余组件迁移和截图视觉回归仍待完成。

### AD-041：Go 与 C++ Realtime Delivery 缺少互斥切流 authority

- **优先级：** P0
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **完成日期：** 2026-08-28
- **解决方式：** 建立默认 Go 的 `go|shadow|cpp` 本地 authority、跨语言 Redis epoch lease 与 fail-closed reader、短 TTL 节点 observation、双 Kafka group 零 lag checkpoint、不可变 attempt workspace、哈希链 journal、幂等 action artifact 与 production executor。`dipole-realtime-cutover run` 在单一同步循环中统一 advance、条件续租、冻结超时回切和阻塞重试，并以 attempt-scoped Redis owner token 排除并发 controller；回切必须先确认 source nodes，且 `rollback_requested` 续租保留回切意图。
- **验证：** 隔离证据覆盖 Go/C++ 各一条客户端 frame、跨客户端 checkpoint、controller artifact 崩溃恢复、Redis outage、Kafka member loss、500 ms expired-freeze 回切、真实 C++ Primary lease/observation/assignment/readiness，以及 Controller A 无 release 进程退出后 B 在 5 秒 TTL 前被拒、到期后从同一 journal 完成。证据归档于 `benchmarks/c3-delivery-authority-2026-08-28/`、`benchmarks/c3-cutover-checkpoint-2026-08-28/`、`benchmarks/c3-cutover-faults-2026-08-28/`、`benchmarks/c3-cutover-cpp-primary-2026-08-28/` 与 `benchmarks/c3-cutover-controller-2026-08-28/`。
- **追加验证：** 2026-08-29 使用当前分支重新运行 C3 真实故障演练，C++ build/CTest 14/14、对比测试 5/5、Controller/C++ 进程替换、Redis outage、Kafka rebalance、过期 freeze 自动回切和 primary 停止恢复全部通过；生产部署仍保持默认 Go authority。
- **性能门禁：** 2026-08-29 对同一 direct created v1 事件执行 100,000 次 Go/C++ JSON 解码与投影，结果计数一致但 C++/Go ops ratio 约为 `0.10`，低于 `1.0` 晋级门槛；当前保留 Go projection，C++ 仅保留故障隔离、authority 和后续连接/批处理数据面评估边界。
- **兼容说明：** tracked deployment 继续默认 Go；关闭该债务只表示 C3 切流协议与回切证据门槛完成，启用 C++ authority 仍需要独立的灰度发布决策和显式配置。

### AD-039：Gateway Kafka assignment 未纳入 readiness

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **解决日期：** 2026-08-28
- **影响范围：** Go Gateway 实时投递、空 Kafka 冷启动、基准门禁、后续 C++ Realtime Delivery 切流
- **解决方式：** Kafka Consumer 通过 coordinator `DescribeGroups` 聚合整个消费组的 assignment；Gateway 新增要求首次成功的 `kafka-assignment` 探针，只有 group 为 `Stable` 且每个已注册 base/retry topic 至少拥有一个分区时才通过。通用 readiness 状态机新增 opt-in 初始失败语义，其他探针继续沿用兼容默认值。微服务 smoke 同时要求 `/readyz` 和 assignment 指标通过，并使用可覆盖的临时证书目录保持演练隔离。
- **验证：** clean revision `958d40c7910ca8a85c0dad6bf57698ae32f9d42f` 镜像来源为 dirty=false；独立栈停止 Gateway 后消费组为 `Empty 0`，重启首样本为 `/readyz=not-ready`、`service_ready=0`、assignment=0，32 个样本约 10.2 秒后达到 `Stable 20`、`/readyz=ready`、assignment=1。完整依赖 readiness smoke、聚焦测试和 canonical Go gate 通过，证据归档于 `benchmarks/c2-gateway-assignment-readiness-2026-08-28/`。
- **保留边界：** 当前探针验证 group 级稳定状态和注册 topic 覆盖；运行期短暂 rebalance 继续由既有失败阈值吸收，长期 reader 无进展仍需结合 fetch/commit/lag 指标独立告警。

### AD-022：前端开发工具链仍停留在 Vite 5

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** 前端开发服务器、Vite、Vitest、Rolldown、依赖审计
- **解决方式：** 前端升级到 Vite 8.2.2、Vitest 4.1.11 和 plugin-vue 6.0.8，固定 Node 22.12+ LTS；Vite 配置使用 `import.meta.dirname` 兼容 native config loader，并允许测试环境覆盖代理目标。旧 Vite/esbuild 开发链由 Vite 8/Rolldown 取代。
- **验证：** Node 22.12.0 干净容器完成 `npm ci`、3 项工具链契约、53 项单测、生产构建和完整/生产依赖零漏洞审计；真实 HTTP/WS 代理及 `/app/` 资源路径通过。Chromium、Firefox、WebKit 的全部适用 Playwright 场景通过，平台专属场景按既有条件跳过。
- **长期约束：** Node 最低版本、Vite/plugin-vue/Vitest peer 范围和 `.nvmrc` 同步维护；工具链主版本升级必须重新运行代理、base path、三浏览器和 audit 门禁。

### AD-006：消息仓储保留未使用的兼容包装

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `MessageRepository` API、消息事务测试替身
- **解决方式：** 全仓调用审计确认生产 Repository 和 `application.MessageStore` 已只保留 `CreateWithSync`、`StoreWithOutboxAndSync` 两个显式写入口；删除测试 stub 中残留的 `Create`、`StoreWithOutbox`，并增加方法集回归测试阻止兼容入口回流。
- **验证：** Repository/Message Service 测试和全仓方法定义扫描通过；现有消息发送、Outbox、Inbox 与 projector/atomic 语义未改变。
- **长期约束：** 新增 Message 写入口必须显式描述 Message、Metadata、Conversation Seq、Outbox 和可选 Inbox 的原子边界，不能通过无 Sync 语义的兼容包装进入生产端口。

### AD-012：用户状态常量与 schema 默认值偏移

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `model.User`、`users.status`、跨语言状态契约
- **解决方式：** `UserStatusNormal=1`、`UserStatusDisabled=2` 改为显式常量，并新增语言中立 `dipole.user.status.v1`；migration v27 将历史 `status=0` 归一为 `1`、修改默认值并通过 CHECK 约束拒绝领域外状态。
- **验证：** 契约测试固定 Schema ID、版本、默认值、枚举与 Go 常量一致；真实 MySQL 8.4 升降级测试覆盖历史回填、默认写入、非法值拒绝，以及 Down 保留已归一数据。
- **回滚边界：** Down 恢复旧默认值并移除约束，保留已归一的 `1`；应用回滚仍可读取既有 `1/2` 语义。

### AD-033：Artifact 仍复用通用文件存储凭据与 bucket

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** Agent Artifact、MinIO 权限隔离、跨域对象覆盖与生产切流
- **解决方式：** Artifact blob client 从全局文件 `MinIOUploader` 拆分为显式启用的独立配置、`dipole-agent-artifacts` bucket 和 `dipoleartifact` Core 身份；policy 仅允许 bucket 定位/列举与固定前缀 Get/Put，不含 Delete。通用存储同时降权为只覆盖文件及两个归档 bucket 的 `dipoleplatform` 身份，TS Agent Runtime 保持无对象存储凭据。
- **验证：** 真实 tmpfs MinIO 正向验证两个身份各自允许路径，拒绝 Artifact 删除、Artifact 前缀逃逸、Artifact 到文件 bucket 及平台身份到 Artifact bucket；三份 Compose 渲染、连续两次初始化、配置/策略测试、Go test/vet/race、TS test/typecheck/build 和生成物漂移门禁通过。
- **保留边界：** Runtime 与 Core 继续没有 Artifact 删除权限；孤儿对象审计和清扫由 AD-032 跟踪，未来使用单独 maintenance 身份与 dry-run receipt。

### AD-027：Agent 权限授予与审批状态尚未持久化

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `ExecutionContext`、Agent Definition、Capability Policy、Human-in-the-loop、远程 TS Runtime
- **解决方式：** migration v16 与 sqlc Store 持久化版本化 Definition、固定版本 Task 和一次性 Approval；Embedded trigger 默认以 `ai.policy_mode=persistent` 创建确定性 Task，重新读取精确 Definition version 后恢复 permission/resource scope，并以 CAS 完成生命周期。`static` 保留显式回滚。migration v17 将身份列 expand-only 扩至 24 字符，兼容默认 21 字符 Assistant UUID。
- **验证：** Tool、Capability、Command 均拒绝 permission 足够但 resource scope 越界的访问；撤销、过期、新版覆盖、重复 Task 和成功/失败生命周期测试通过；真实 MySQL 8.4 使用默认长度身份完成 Definition 初始化、Task 快照与 `running→completed`。G3 的审批 UI 与 Temporal Signal 恢复继续作为计划能力推进。

### AD-016：HTTP Handler 测试并行修改 Gin 全局模式

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `internal/handler/http/*_test.go`、整包 race 门禁
- **解决方式：** 在包级 `TestMain` 进入并行测试前只调用一次 `gin.SetMode(gin.TestMode)`，删除各测试函数中的重复全局写入，同时保留原有 `t.Parallel()` 覆盖。
- **验证：** 修复前 `go test -race ./internal/handler/http` 稳定报告 `gin.SetMode` 写写及与 `gin.New/CreateTestContext` 的读写竞争；修复后整包 race、普通测试和完整 Go 测试通过。
- **长期约束：** Handler 测试不得在测试函数或并行子测试中修改 Gin 包级模式；新增全局测试配置应在 `TestMain` 中串行完成。

### AD-025：Web 本地消息库清理与容量策略需真实浏览器验收

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** IndexedDB Sync Engine、共享设备隐私、浏览器配额、401/强制下线
- **解决方式：** IndexedDB 按用户隔离 Message、manifest 与 Cursor，并在同一事务执行整页提交和高低水位淘汰；显式退出、HTTP 401、WS kick 和账号切换统一进入 Session Terminator，先撤销凭据与运行时状态，再清理当前账号并跳转。增加真实浏览器重开/中断、独立 Chromium 主进程 crash、无特权 128 MiB tmpfs 容量拒绝，以及共享 profile 双账号 HTTP/WS 被动失效验收。
- **验证：** Chromium、Firefox、WebKit 均验证 U1 被 401 或 `session.kicked` 后凭据清空、U1 IndexedDB 归零且 U2 Seq/Message 保留；Chromium 在 `commitPage` pending 时主进程 crash 后保持整页原子性；专用 quota 脚本触发真实容量错误，释放 reserve 后失败页未推进安全 Cursor。`storage_full/sync_error` 有界指标和告警继续作为运行门禁。
- **长期约束：** 公共设备应使用显式退出；若浏览器在清理完成前被强制终止，操作人员或用户需从浏览器设置中清除 Dipole 站点数据。新增本地 store、账号 key 或会话终止入口时必须扩展三浏览器共享 profile 验收。

### AD-023：Sync Service 数据库账号与 Message 写权限尚未分离

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `cmd/sync-service`、`user_sync_inbox`、设备/群 checkpoint、MySQL 最小权限
- **解决方式：** 增加继承全局配置的 `sync.mysql.*` 专用凭据、操作级 Sync 启动探针和 `dipole_sync` 授权；增加 `message.inbox_write_mode=atomic|projector`，独立 owner 在 projector 模式停止 Inbox 写入，同时保留 Message/Seq/群高水位/Outbox 事务。`dipole_message_projector` 无 Inbox 权限，atomic 配置和原授权模板保留即时回滚。
- **验证：** 真实 MySQL 8.4 smoke 验证 Sync/Message 两类最小账号、越权拒绝、Message+Outbox 无 Inbox 写入、Sync 投影收敛和 atomic 回退；单元与 repository contract 覆盖模式校验、重复修复 no-op 和权限边界。

### AD-024：Sync Replay 的历史覆盖受 created Outbox 边界限制

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `cmd/sync-baseline`、历史群消息、Outbox 保留、Message Inbox 写权限退役
- **解决方式：** migration v11 增加不可变 baseline Job/Entry；Capture 在 Repeatable Read 固定 Inbox 高水位，并归档所有缺少 created Outbox 的原始 `sync_seq + recipient + locator`，以规范化 SHA-256 校验完整性。Reconcile 同时扫描快照后新增 legacy 行；Restore 仅修复 missing，保留原 Cursor，并拒绝 extra/conflicting 状态。
- **验证：** 纯领域测试覆盖稳定摘要和差异分类；真实 MySQL 8.4 integration/smoke 覆盖重复 Capture、删行检测、原 `sync_seq` 恢复、越界冲突拒绝、v11 down/up 与并发 migration owner。

### AD-020：Search 删除接口缺少 mutation revision

- **本轮验证：** Elasticsearch Search Service 与三节点 Search Indexer 真实隔离 smoke 已通过授权范围、tombstone 和乱序事件收敛；长期生产流量切换仍遵循 Search/A5 的 Alias、归档和回滚门禁。

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **完成日期：** 2026-08-27
- **解决方式：** `SearchIndex` 收敛为版本化 `Apply(MessageSearchMutation)`；created/edited 生成 searchable 文档，recalled/deleted 生成只含身份、revision、`searchable=false` 与 payload hash 的持久 tombstone。MySQL 与 Elasticsearch 统一接受更高 revision、忽略旧 revision、接受相同重放并拒绝同 revision 不同 payload。
- **验证：** 共享模型单元测试、两种 adapter contract、MySQL 8.4 `000001..000007` Up/Down 与 Elasticsearch 9.5.2 真实 tombstone 演练通过；tombstone 后旧正文事件无法恢复搜索结果。

### AD-018：Cassandra Seq 响应不携带 MySQL 内部 ID

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **完成日期：** 2026-08-27
- **解决方式：** Direct/Group HTTP 与 Message v1 RPC 增加互斥的 `before_seq`；Web 历史首屏固定从 `before_seq=0` 开始，向前分页使用最旧正 Seq，热群补拉改用 `after_seq`，同 UUID 合并优先保留带持久 Seq 的版本。Cassandra 路由只覆盖显式 Seq cursor，legacy ID cursor 始终留在 MySQL。
- **验证：** Local/gRPC 应用契约、HTTP 游标互斥、Service 权限与 Seq 透传测试通过；真实 MySQL 8.4/Cassandra 5.0.9 演练确认 before/after 完整页由 Cassandra 返回，人工缺行后整页回退 MySQL。
- **保留兼容：** Cassandra 响应的 `id` 继续为零；全局身份使用 `message_id`，会话排序和分页使用 `message_seq`。A6 已增加默认关闭的 IndexedDB 持久同步，旧 Offline 双跑对照完成前不改变默认客户端路径。

### AD-004：热群消息缺少持久化同步补偿

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** Message 事务以 O(1) 写入群 Timeline 高水位，Sync 保存用户/设备/群拉取位点；客户端重连后提交已知群列表，经 Core 成员权限校验取得最新 Seq，并使用 `after_seq` 分页追平。IndexedDB v3 将热群消息与本地群 `message_seq` 原子提交，提交后才 ACK 设备群 checkpoint；在线 notify 聚合继续保留，Redis 或 Gateway 重启不会丢失离线发现依据。
- **验证：** 通过历史 migration 回填、消息/Outbox/高水位原子回滚、设备 ACK 单调性、越权拒绝、Message/Sync gRPC、HTTP 零 Seq cursor、真实 MySQL contract 和定向 race 测试；Web 契约覆盖持久化失败禁止 ACK、重开本地恢复、ACK 补交、位点单调性与逐账号清理。
- **兼容说明：** `off` 模式继续执行不 ACK 的内存补拉；`/messages/offline` 覆盖升级前历史和旧客户端。在线 Sync Item 驱动 Cassandra 主读仍按 A6 独立灰度。

### AD-014：M3 grpc 模式存在重复 Local MessageService 实例

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** Runtime 成为 Messaging Services 的唯一 Composition Root，并把同一实例注入 Server 与 Kafka handler 注册；Server 只在兼容构造入口缺少注入时创建服务集合。Conversation notifier 在 Server 建立 WS Hub 后注入现有实例。
- **验证：** Local/Remote transport、Server、Kafka 和 Bootstrap 全部复用 Runtime 的 Messaging Services，并通过相关包 race 测试。

### AD-013：内部 RPC 调用身份尚未绑定服务认证

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 内部 RPC 同时启用共享服务凭据、caller allowlist、常量时间密钥比较和 TLS 1.3 mTLS；认证服务身份写入调用 context，并与 protobuf `caller_service` 强制一致。明文 listener 与 target 仅允许 loopback。
- **验证：** 通过合法/缺失/错误凭据、未授权 caller、payload caller 冲突、真实临时 CA mTLS，以及非 loopback 明文拒绝测试。

### AD-010：GORM 模型与运行时 AutoMigrate 绑定数据结构

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 使用版本化 SQL migration 管理 schema，所有 Repository 经共享真实 MySQL 契约渐进迁移到 `database/sql + sqlc`；最终移除 legacy adapters、model tags、AutoMigrate、SQLite 方言测试、兼容配置和 `gorm.io/*` 依赖。
- **验证：** 全仓 GORM 标识与模块依赖扫描为空；通过 sqlc 生成漂移、全量 Go、真实 MySQL Repository/migration/并发事务、race、vet 和模块完整性测试。

### AD-001：并发事务可能造成 Sync Cursor 永久跳过消息

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 新增 `user_sync_states` 用户锁表；Inbox 事务按用户 UUID 固定顺序获取 `FOR UPDATE` 行锁，再分配全局自增 `sync_seq`。重复投影修复也进入同一事务。
- **验证：** 使用 MySQL 8.4 双连接测试暂停第一条未提交事务，证明第二条同用户事务被阻塞，释放后 Inbox 游标顺序与提交顺序一致；同时通过迁移、回滚、MySQL SQL 契约和定向 race 测试。

### AD-002：旧群消息事件与热群 Sync Fanout 标志存在歧义

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 将 `sync_fanout` 改为三态字段；旧事件缺失时默认执行普通 fanout，显式 `false` 继续表示热群跳过 Inbox。
- **验证：** 覆盖旧事件缺失、显式启用和显式关闭的 Kafka JSON 契约测试，并通过完整测试与定向 race 测试。

### AD-003：幂等冲突可能使用新事件收件人修复旧消息 Inbox

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 修复 Outbox/Inbox 前校验发送者、目标类型、目标 UUID 和会话键；冲突时返回明确错误，并基于已有消息重新计算可信收件人。
- **验证：** 覆盖同一 `client_message_id` 改投其他目标的隔离测试，并通过完整测试与定向 race 测试。
