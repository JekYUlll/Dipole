# 架构债务台账

本文档记录已确认但暂缓处理的架构风险、兼容性缺口和可清理冗余，便于后续按优先级滚动治理。

## 维护约定

- 状态使用：`暂缓`、`处理中`、`已解决`、`接受风险`。
- 优先级使用：`P0` 阻断发布、`P1` 应在正式启用相关能力前解决、`P2` 进入后续迭代、`P3` 按需清理。
- 新问题使用连续编号 `AD-NNN`，保留历史编号，不复用已关闭条目。
- 开始处理时补充负责人或关联 Issue/PR；解决后记录提交、验证方式和完成日期。
- 本台账描述风险和演进方向，不代表当前迭代立即修改对应实现。

## 待处理

### AD-040：WebSocket 查询令牌进入 HTTP 访问日志

- **优先级：** P1
- **状态：** 暂缓
- **发现日期：** 2026-08-28
- **影响范围：** Gateway HTTP 访问日志、日志聚合与保留、WebSocket session JWT
- **现状：** WebSocket 使用 `?token=` 建立连接，Gin 完成日志记录原始 request path。C2 primary seam 演练在未提交日志中检出完整 JWT，归档前已替换为 `token=REDACTED` 并重新执行低敏扫描和校验和。
- **风险：** 具备日志读取权限的主体可能在令牌有效期内重放 session；集中日志、备份和工单会扩大凭据暴露面。
- **建议方向：** 在结构化访问日志入口统一清除敏感 query 参数，WebSocket 后续评审短期 ticket、Cookie 或 `Sec-WebSocket-Protocol` 认证方案；增加日志 capture 测试，拒绝 JWT、共享密钥和授权 Header 进入日志正文。
- **处理门槛：** 任何 Gateway 日志进入共享日志系统前完成脱敏；认证传输方案变化需保留旧客户端兼容窗口和重放威胁测试。

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
- **现状：** 官方 MCP TS SDK v2 Client/Server foundation 与默认关闭的 Gateway/Runtime 网络入口已完成，当前生产只投影 `conversation.list`。第一方授权交换要求 session principal 对 canonical resource 和只读 scope 显式 consent，签发 15 分钟且绑定 `aud/scope/token_use` 的 MCP JWT；普通 session 与 MCP token 互相拒绝。Gateway 剥离外部凭据并向 Runtime 证明已验证 principal/resource/scope。单次 Tool invocation 有 100 ms 至 60 秒有界 timeout、cooperative cancellation 和 `tool_timeout` 审计；外部 Client foundation 的 connect/list/call 也使用 request/total timeout，Runtime 传播连接断开信号。migration v30、统一低敏 OTel、默认关闭的 Collector/Tempo profile、共享 Redis principal 限流与真实 trace smoke 已完成。外部连接 Profile v1 现以严格契约绑定 tenant、HTTPS endpoint、Server identity、Tool/Host/Port allowlist、TLS ServerName、CA 与版本化 credential opaque ref。Credential Catalog v1 进一步保存 tenant/ref/version、生效窗口、active/revoked 状态及 opaque provider secret ref，每次建连前重新加载并精确授权，轮换/吊销无需进入 Task 或 Workflow 状态；受约束文件 source 使用规范绝对路径、canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode 和 256 KiB 默认上限，并支持原子替换。Provider-neutral MCP `AuthProvider` adapter 每次请求读取 fresh bytes，使用独立 timeout/AbortSignal、大小和 Bearer 字符校验、固定脱敏错误与源 buffer 擦除，同时不暴露自动 401 refresh。可注入 Network Guard 对每个 SDK 请求重新校验 exact HTTPS Host/Port/TLS identity，要求全部 DNS 答案为公网地址，把批准地址交给 pinned Dispatcher，并核对实际 peer；重定向、混合/重复/超量答案和 rebinding 均 fail closed。外部 Tool 成功结果可通过有界 adapter 转换为 `section=evidence`、`trust=untrusted` 且绑定 Profile/Server/Tool/Invocation provenance 的 Context fragment；compact 记录不复制外部正文。Core 现提供 active-only Approval grant resolution 与原子 consumption RPC：sqlc 精确查询最多两条候选，应用层要求 active Run、运行中 Task、principal 审批人和唯一未消费 binding；TS 独立复算 scope/arguments 并连接 write gate。`nonce_sha256` 明确作为持久的一次性绑定摘要。认证 Message Command RPC 要求 running Tool Invocation，Core 复算 canonical 参数摘要、派生 Command ID/身份并返回可验证 Message action reference。默认关闭的第一方 Message projection 已能按 `consume -> begin -> command -> finish(action)` 顺序组合这些边界，并要求 active context、显式 executor 与精确 direct conversation。active Run admission 必须经过注入式 promotion authorizer，MCP context 使用持久 Run 的权威 `runtime_id/mode` 并由 Go/TS 双重校验；migration v33 增加仅认证 Gateway 可调用的 Runtime promotion 提案、复核、查询和撤销控制面，Runtime 数据面不能签发 Grant。migration v35 保存可恢复的权威外部 Tool command；migration v36 进一步按确定性 Round ID 原子认领最多两个 Tool round，仅原 owner 可提交终态，已完成/失败结果可重放，任何遗留 `executing` 都返回 `ambiguous` 且没有 lease reclaim/retry 路径。Activity 在发起远端调用前 Claim，并在返回 Temporal 前持久化规范结果。生产未注入 authorizer，Registry、write executor 和 active context 继续关闭。生产进程缺少真实 Resolver/Dispatcher/Factory 时 fail closed。外部 MCP Server、Secret backend 和 write/destructive Tool 均未启用。
- **本轮进展：** Streamable HTTP Transport Factory 已精确复核 Profile/Catalog 的 tenant-ref-version，为每次连接创建独立 AuthProvider、Network Guard 与官方 SDK Transport，并关闭 401 自动刷新、403 扩权和 SSE 自动重连。该组合只完成策略层，未提供生产 Secret/DNS/TLS I/O backend。
- **本轮进展：** 默认关闭的 MCP Worker Runtime 已组合 Core command resolver/round receipt、Transport Registry、Activity continuation 和三 ID dispatcher；替换实例可回放 completed receipt，ambiguous/cancellation 在网络边界前 fail closed。Temporal 调度入口仍等待受信 Agent Step 创建持久 Invocation。
- **本轮进展：** Invocation begin/finish 已支持全字段精确重放，Repository 读取终态摘要与 action reference；Command RPC additive 返回 Invocation 状态。terminal Invocation 的 Round Claim 只允许读取已有 receipt，缺失或漂移时拒绝且不会创建新执行记录，关闭 Invocation finish 后 Activity completion 丢失的重复远端调用路径。
- **本轮进展：** 默认关闭的 `TrustedMcpInvocationProducer` 使用 host-owned Workflow step/ordinal 生成稳定 Invocation ID，通过 Capability route、PolicyEngine、schema 与 egress policy 派生并验证 Profile/Server/Tool/参数；输入无法携带 authority 字段。
- **本轮进展：** additive terminal RPC 只接受 Task/Run/Invocation/Round ID，Core 从 durable receipt 派生 read-risk Invocation 的结果、字节数、错误码和首次 latency；terminal 重放核对已存证据，`executing`、`input_required`、write Capability 与绑定漂移均拒绝。默认关闭的 terminal Worker composition 已连接成功/稳定失败路径，生产仍缺真实路由、Temporal 调度与 I/O backend。
- **本轮进展：** 独立且默认关闭的 Temporal MCP dispatch Activity 已固定 route ID/version、Workflow step/ordinal、canonical 参数和完整性 checkpoint；每次 begin/retry/resume 都重新解析 Core ExecutionContext、精确重放 producer，并只向 terminal Worker 下发 Task/Run/Invocation 三个 ID。完成结果经注入式幂等 projector 收敛为 Artifact 收据，原始外部结果不进入 Workflow 输出；通用 Workflow、启动入口、Activity mode 与外部网络保持未接线。
- **风险：** 第一方交换尚未提供 RFC 9728 Protected Resource Metadata、Authorization Server Metadata、OAuth 2.1 Authorization Code + PKCE 和第三方客户端注册，因此还不能声明为通用 MCP OAuth Server；外部 Server 的加密 Secret backend、provider owner 授权及真实 DNS/TLS pinned Dispatcher 仍未实现。fresh byte buffer 可覆盖，MCP SDK 所需的 JavaScript token 字符串与 Header copy 仍由 GC 管理，无法证明强零化。Catalog 文件依赖 OS mount/owner 信任，没有签名、rollback revision 或可用性告警；拒绝 symlink 也意味着 Kubernetes 默认 projected volume 不能直接使用。Tempo local backend 只适合单机 Shadow/验收，尚无生产对象存储生命周期、Alertmanager 通知链和长期 trace/audit 联查证据。结构化 egress guard 无法识别被改名、编码或嵌入普通文本的敏感值；`trust=untrusted` 提供上下文隔离语义，模型仍可能受 Prompt Injection 影响，接入时还需 trajectory Eval、Tool Policy 和输出 lineage 共同约束。Round receipt 可恢复本地已接收并持久化的成功结果；远端完成副作用后、Runtime 收到结果前或本地收据提交前仍存在无法由本地数据库证明的极小窗口。当前策略把传输异常持久为 `remote_outcome_unknown`，后续重放失败关闭，牺牲自动恢复来避免重复副作用；具备服务端幂等键或查询收据的 Profile 才能进一步收敛。Approval consumption 提供 at-most-once 副作用门槛；Message Command receipt 与 migration v31 action reference 已连接 Approval、Command 和权威 Message UUID，消费后 operation 失败仍需新审批。Command RPC 在服务端提交后遇到客户端 deadline/cancellation 时仍需 receipt-driven action-lineage 收敛演练。promotion operator Grant 仍依赖受控运维预置，生产 authorizer 仍未注入；approved Capability 已有显式 allowlist 投影与 Go/TS 双重校验，UI 风险摘要和真实故障演练仍缺失。
- **建议方向：** 接入真实 OAuth 2.1 Authorization Server 后发布 discovery/PKCE；为现有 Factory 实现加密 Secret Provider、真实 DNS Resolver 和按批准地址建连的 TLS pinned Dispatcher，绝不把 secret 返回 Runtime。外部 Tool 结果作为 untrusted Context fragment。生产 trace 使用对象存储与通知链；write Tool 必须绑定现有 Approval 与 Agent lineage。手工 MRTR continuation、round receipt 和三 ID Worker dispatcher 已固定恢复与命令权威边界，后续仍需专用默认关闭 Worker 装配、多轮策略与敏感授权隔离。
- **处理门槛：** 任何共享环境 MCP 开关启用、外部 Server 连接或 write/destructive Tool 上线前完成。当前网络入口仅用于受控认证和授权边界验证。

### AD-036：Elicitation 缺少 MCP continuation 与敏感授权隔离

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Human-in-the-loop、Web 客户端、MCP 集成、凭据与第三方授权
- **现状：** `dipole.agent.elicitation.v1` 已固定 text/select/multiselect/boolean Form、动态响应校验、大小上限和绝对截止时间；Gateway JWT API 经 Core Task owner 复核后发送精确 request ID 的 Temporal Signal，Worker 替换可恢复同一等待点和 Timer，到期自动以 `input_expired` 取消。默认关闭的 MCP adapter 已将受限 form mode 映射为 `wait_input`，以 checkpoint 绑定 untrusted Server/Tool/Invocation/Form/deadline，并拒绝 URL、敏感字段与有损 schema。MCP `2026-07-28` Client seam 显式声明 Form Elicitation 并关闭进程内自动 fulfilment；手工 MRTR continuation 只接受一个 input request，将原 Tool 参数、请求键、可选 opaque `requestState` 和 lineage 绑定到完整性 checkpoint，并可在新连接中精确生成下一次 `tools/call`。真实 SDK Streamable HTTP 双轮契约已通过。canonical Pencil 与默认关闭的 Vue 页面覆盖 desktop/mobile 普通 Form、来源披露和七态，经 authenticated Task query/input/cancel API 精确提交当前 Task/request；Runtime 与 Web 均拒绝凭据类字段。当前尚未把 continuation 装配进生产 Temporal Activity/外部 Transport Factory，也未交付多轮、敏感授权 URL mode、产品入口编排和对应浏览器 E2E。
- **风险：** 浏览器闭环只能恢复已进入 `waiting_input` 的 Task；MCP Server 仍无法在 durable input 完成后恢复原 Tool 调用。将密码、Token 或 OAuth 信息放入普通 Form 会进入 HTTP、日志或 Workflow history，扩大敏感数据暴露面；未来生产接线仍需处理连接丢失、用户取消和 Server 无恢复能力等差异。
- **建议方向：** 保持普通 Form 的字段白名单和默认关闭灰度，补充 authenticated 路由 E2E、键盘/可访问性与视觉回归。Activity-safe runner 已能跨实例重开现代 Client、校验 tenant-owned Profile 并关闭失败资源；下一步将其接入独立默认关闭的 Worker mode，并固定持久 Tool invocation、progress/cancel 和审计映射。第三方授权继续采用独立 URL mode、短期 challenge 与回调绑定。
- **处理门槛：** Project Guardian 的普通 Form UI 已完成并保持默认关闭；任何凭据、支付、OAuth 或外部 MCP Elicitation 上线前完成独立敏感输入隔离、continuation 和威胁建模。

### AD-035：Memory foundation 缺少受控写入、压缩与删除治理

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Memory、Context Compiler、隐私删除、长期事实质量、Project Guardian 演示
- **现状：** migration v29 与 sqlc Store 已保存五类不可变 scoped Memory、full/compact content、priority、有效期和 provenance；Core 根据运行中的 Task/Run 固定 principal、tenant、Agent 与 conversation read scope，并使用 Task 创建时间阻止后续新增记录进入重放，撤销/过期立即 fail closed。TS 仅在显式启用且实际命中时使用独立预算和 `untrusted` trust level，默认与 Compose 均关闭。当前仅有内部 Store 写入与撤销能力，生产读取在没有受控记录时返回空集。
- **风险：** 自动从消息写入会引入错误事实、Prompt Injection 固化、跨用户泄漏、无限增长和删除不完整；缺少用户纠正与删除入口时，长期启用会造成无法治理的个性化状态。仅按 priority 的精确 scope 检索也无法衡量 recall、precision 和 context 成本。
- **建议方向：** 先建立 Gateway principal 派生的查看/纠正/撤销 API、append-only 版本和删除审计，再增加离线 Observation/Reflection Worker；写入策略要求来源证据、置信度、TTL、幂等键和冲突合并。基于 retrieval Eval 比较 MySQL 精确检索、Elasticsearch hybrid/vector 与 reranker，模型生成记录始终保持不可信数据边界。
- **处理门槛：** 在共享环境自动写入消息 Memory、启用跨 Task 长期召回或向用户展示个性化结论前，完成管理/删除链和 permission/retrieval Eval；当前 foundation 只允许受控 seed 与 Shadow 读取。

### AD-034：Event Subscription 缺少用户界面与语义预筛

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Trigger Engine、Definition 授权、模型成本、Gateway/前端配置与 Project Guardian 演示
- **现状：** migration v28 与 v34、sqlc Store、Core resolver 和受认证 RPC 已持久化精确 Definition version 订阅，并提供 Gateway principal 派生 owner 的创建、历史分页与可审计撤销。TS Runtime 可在 EventLedger、Temporal 和模型前确定性过滤。语言中立 prefilter Eval 已支持有界标签 corpus、三类 candidate evidence、分类/延迟/成本指标和生产规则基线；corpus review v1 要求双 reviewer 完整标签与第三方分歧裁决。rollout gate 从三份源证据重算 review/prefilter 结果，并以同一 corpus 哈希输出低敏 `eligible|blocked` 决策。Compose 与默认配置继续使用 `direct_target`。
- **风险：** 管理能力目前仅为 Gateway-only 内部 gRPC，尚无公开 HTTP/Pencil 用户界面；确定性关键词无法覆盖语义等价表达。直接启用共享环境订阅模式仍会造成难以运维的策略或相关事件漏触发。
- **建议方向：** Pencil transport 恢复后增加 owner 管理页面和公开 Gateway adapter；使用同一 reviewed corpus 采集 embedding 与小模型 candidate evidence，并与规则基线比较。高成本 Agent 只接收预筛后的事件。
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

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Cassandra 主读、Sync Timeline、消息幂等、文件授权、搜索重建、迁移回放
- **现状：** `user_sync_inbox` 已持久化并对外暴露 `conversation_key + message_uuid + message_seq` locator。Sync Service 已建立 storage-neutral hydrator，可在返回 MySQL 正文的同时异步比较 Cassandra Timeline；Cassandra 尚未承担 Sync 主读。Direct 与 Group Timeline 均已具备 `after_seq` HTTP/Message v1 gRPC 增量契约，Local/Remote/Shadow adapters 一致，并复用 Cassandra cohort、连续页校验与 MySQL fallback。Gateway 已增加默认关闭的 `sync.item.notify.v1` body-free shadow 通知，Web verifier 会按会话补拉、去重并验证 locator；现有完整消息投递和热群聚合 notify + pull 保持不变。Web 已增加默认关闭的 IndexedDB Sync Engine、shadow 门禁和热群持久 ACK。migration v12 增加无正文 `message_metadata`，与 Message/Inbox/Outbox 原子提交并回填历史 locator；文件授权已改查 Metadata，删除完整 Message 行后仍可验证访问和过期时间。重复发送先通过 Metadata 校验身份，并可在默认关闭的开关下按会话 Seq 从 Cassandra 恢复原响应，缺失/冲突继续回退 MySQL。Cassandra Backfill/Reconciler 已支持经 SHA-256 校验的不可变完整消息归档，Job 绑定 source identity；真实演练删除 MySQL 正文后仍可恢复和全量对账。Message 最小账号暂时保留 `groups/group_members` 只读权限用于旧 Offline 与群文件授权。
- **风险：** 提前停止正文写入仍会让多端同步和重复发送响应缺失正文，并丢失 Cassandra 修复与回滚基准。文件授权的正文依赖已解除，但群文件授权仍需 Core 成员关系。
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

- **优先级：** P2
- **状态：** 接受风险
- **发现日期：** 2026-08-26
- **影响范围：** Gateway 跨节点在线投递、Redis Sentinel 故障转移、后续 C++ Realtime Delivery
- **现状：** go-redis 会在 Sentinel 选出新 master 后重连命令与 Pub/Sub 连接；连接中断期间已经发布的 Pub/Sub 消息无法补读。Gateway 的 Kafka handler 当前将跨节点 Pub/Sub 视为实时通知通道。
- **风险：** master 切换窗口内，在线用户可能暂时缺少一条跨节点通知；Redis Sentinel 无法提供持久队列或消费位点。
- **接受依据：** 消息事实、用户 Inbox、设备 Cursor 和热群 checkpoint 均保存在 MySQL/Kafka 链路，客户端重连或增量同步能够恢复已确认消息；Redis 只承担实时状态。
- **阶段记录：** 2026-08-28 已建立 `dipole.delivery.v1` envelope、节点批次、逐项 ACK/error 与背压契约，并固定 Kafka source coordinates 和 Go legacy adapter；C++ shadow 已接入独立 Kafka group、hiredis direct/Sentinel reader、单连接 TTL 投影、低敏 evidence v3、mTLS `ObserveNodeBatch` 和 assignment readiness。真实 Kafka+Redis+Gateway 演练覆盖故障保留 offset、同进程恢复重试、稳定 batch 去重、真实 queue saturation/backpressure、同 workload Go/C++ 40/40 对照与最终 lag 归零。`AD-039` 已关闭。默认关闭的 primary seam 提供 connection 定向入队、逐项 ACK、部分成功 connection 重试、有界 Gateway replay state 与 additive WebSocket delivery ID；Web 通过账户隔离的 IndexedDB v4 原子 claim 跨页面重载去重。C++ one-shot probe 经 mTLS 实际验证 `ENQUEUED(1)`、稳定重放去重与 stale Presence `OFFLINE`。显式 primary CLI 现使用独立 `dipole-realtime-primary-*` authority，要求 enable/Presence/transport 三重配置，并将 terminal ACK、低敏 primary evidence 与 Kafka commit 串联；partial/rejected/failed、身份漂移和故障保留同一 pending record。当前 Go Redis Pub/Sub 流量语义未切换，primary 未进入 Compose。
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
- **风险：** v24 projection 保持 shadow 观察属性，尚未接管原 `agent_tasks.status`；当前 `read_shadow` 只允许 `conversation.list`，也没有 Memory 或真实任务终态 outcome Eval。v25 的 `approved` 只保存审计结论；execution plan v1 仅允许带 CAS/回滚证据的 dry-run，执行器和 projection 修改命令保持缺席。操作员授权当前需要受控 SQL 配置。Temporal Worker 停止时 Query 会归类为 unavailable。eligible 决策不能自动切换 active。
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

### AD-041：Go 与 C++ Realtime Delivery 缺少互斥切流 authority

- **优先级：** P0
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **完成日期：** 2026-08-28
- **解决方式：** 建立默认 Go 的 `go|shadow|cpp` 本地 authority、跨语言 Redis epoch lease 与 fail-closed reader、短 TTL 节点 observation、双 Kafka group 零 lag checkpoint、不可变 attempt workspace、哈希链 journal、幂等 action artifact 与 production executor。`dipole-realtime-cutover run` 在单一同步循环中统一 advance、条件续租、冻结超时回切和阻塞重试，并以 attempt-scoped Redis owner token 排除并发 controller；回切必须先确认 source nodes，且 `rollback_requested` 续租保留回切意图。
- **验证：** 隔离证据覆盖 Go/C++ 各一条客户端 frame、跨客户端 checkpoint、controller artifact 崩溃恢复、Redis outage、Kafka member loss、500 ms expired-freeze 回切、真实 C++ Primary lease/observation/assignment/readiness，以及 Controller A 无 release 进程退出后 B 在 5 秒 TTL 前被拒、到期后从同一 journal 完成。证据归档于 `benchmarks/c3-delivery-authority-2026-08-28/`、`benchmarks/c3-cutover-checkpoint-2026-08-28/`、`benchmarks/c3-cutover-faults-2026-08-28/`、`benchmarks/c3-cutover-cpp-primary-2026-08-28/` 与 `benchmarks/c3-cutover-controller-2026-08-28/`。
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
