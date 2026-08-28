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

### 安全

- Agent Workflow repair 增加 migration v44 prepared execution ledger 与 sqlc 访问边界：以唯一 plan 记录执行意图、提案/任务/执行人绑定、CAS 摘要和回滚摘要；当前仅允许 `prepared` 创建/读取，未提供状态推进、apply、execute 或 rollback RPC，现有运行流量不受影响。
- Agent Workflow repair 增加默认无副作用的 `repair:preflight`：在未来执行器前重新核对 plan 摘要、批准提案证据、executor grant 版本和当前投影 CAS，仅输出低敏 `ready|blocked` 收据；过期、漂移和绑定不一致统一阻断，继续不提供 projection 写入。
- 收紧 Agent Workflow repair dry-run 计划绑定：当前投影与目标投影必须属于同一 Workflow/Run，跨任务证据在生成 plan 前 fail closed；不改变 v1 仅 dry-run、无 projection 写入的边界。
- Agent Memory lineage rollout 增加 deployment evidence v1 与只读 `agent-memory-lineage-deployment-evidence` CLI：外部共享环境记录必须绑定 rollout review receipt、运行版本、配置摘要、migration 43、健康检查和回滚演练 ID；输出仅保存 deployment ID 摘要与通过标志，`executionAuthority`、`contentRead`、`deletionAuthority`、`runtimeAuthority` 固定为 false。该工具不连接共享环境、不执行回填，缺少真实部署与回滚记录时继续 fail closed。
- Agent Memory 增加有界历史 lineage backfill v1 的语言中立 manifest/receipt 与 Go runner：游标固定为 Shadow Plan ID high-water mark，引用仅允许 exact `memory:<id>` 与 `full|compact`，runner 在目标成功后推进 checkpoint，重复写返回 duplicate 并保持可收敛。Receipt 仅保存 hash、游标和计数，固定不读取正文、不授予删除或 Runtime 权威；MySQL checkpoint/source/target 尚未接线。
- Agent Memory 增加纯离线派生数据 retention policy 决策：严格覆盖 Model Call、Shadow Plan/Step、Artifact、Tool Invocation、Message Action 与 Temporal potential Task，输入绑定已验证的低敏 lineage report，输出绑定 policy/report/decision 三个 SHA-256。parser 会从 lineage 完整性和受影响的人工复核域重新推导阻断原因；CLI 不连接数据库或网络，固定不读取正文、不执行删除且不授予删除或 Runtime 权威。
- Agent Memory migration v42 将 direct lineage 外键从 Shadow Plan 提前到权威 Agent Task；受管 Model planner 在 Context 编译后、模型调用前写入 `context_pre_model`，失败时模型零调用。Plan-time repair 保持幂等且不能降级来源；只有同时缺少 Plan 与任何 Memory lineage 的 owner 模型结果才进入未归因缺口。
- Agent Memory migration v41 增加保守的派生影响边界：Shadow Plan 事务同步保存 `Memory -> Task` 直接引用，精确重放补齐索引，representation 漂移 fail closed；只读审计按 root 统计模型调用、Plan/Step、Artifact、Tool、Message Action 与 Temporal 潜在 Task。报告只含 root SHA-256、有界计数和完整性标志，固定不读取内容、不授予删除或 Runtime 权威；历史未索引 Context 或已完成模型调用缺少 Plan 的未归因 Task 都会返回 `lineageComplete=false`。
- Agent Memory migration v40 增加 root-wide 内容擦除基础：内部 sqlc/Core 事务锁定完整纠正链，撤销 active 版本，并清除正文、compact、URI、resource binding、root 原始来源与自由文本审计原因；只保留 owner、root/version/predecessor、擦除时间和枚举原因码。语言中立 policy/receipt 明确无自动执行或公开 API 权威，当前没有 Proto、Gateway、Vue 或 retention Worker 入口。
- Agent Memory correction 增加纯离线五类 Eval 门禁：严格 manifest/observation 绑定 predecessor/successor、完整 lineage、精确重放、漂移冲突、owner/foreign 权限与 successor-only retrieval，并要求模型、Tool、Token 和模型成本全部为零；输入各限 64 KiB，错误与标准报告不回显 Memory、principal 或正文，也不连接生产数据库或写入 Memory。
- Agent Memory owner correction 继续沿用认证派生权限：公开请求只提交 Memory ID、期望版本、纠正内容与原因，tenant/principal/corrector 由 Gateway/Core 认证链绑定；响应省略内部 provenance URI，并同时返回权威 predecessor 与 successor，客户端不推断并发结果。
- Agent Memory 增加 owner-scoped 治理边界：公开 HTTP 不接收 tenant/principal，Gateway 从已认证会话构造 Core RequestContext；Core 只允许认证 `dipole-gateway` 调用并再次按 tenant、principal 与 Memory ID 约束查询和撤销。公开 DTO 省略内部 provenance URI，撤销必须提交有界原因并保存 revoker、原因和时间；不同原因的终态重放返回冲突。
- Gin HTTP 访问日志统一脱敏敏感查询参数：WebSocket 兼容的 `token`/`access_token` 及 refresh/id token、Authorization、API key、client secret、密码和签名类键按大小写无关识别，重复值全部替换为 `REDACTED`；非法 query 编码整段关闭，普通参数继续规范化记录。真实 WebSocket 握手日志捕获测试同时证明查询凭据与 Authorization Header 不进入结构化字段。
- Agent Runtime 增加默认跳过的外部 MCP encrypted credential 生命周期演练：临时 owner-only 文件以独立 key/ref/version 完成 v3 到 v4 轮换，Catalog 通过原子 rename 发布并在旧版本与当前版本吊销后于 Transport 构造前拒绝；Runtime 重建继续解析 v4，三次成功 Transport 全部关闭。语言中立 v1 证据与 `mcp:credential-drill:check` 固定三开三关、双吊销、canonical SHA-256、24 小时有效期及 `inflight_revocation_authority=false`、`production_authority=false`，不记录凭据、身份、路径或 endpoint。该离线演练不提供在途 socket 主动撤销或共享 provider authority。
- Agent Runtime 新增默认关闭且独占的 `external_mcp_shadow` Temporal activity mode，并将已验证的外部 MCP Kafka/Temporal process owner 接入 `index.ts`。startup policy 要求 Profile 开关、Temporal、Kafka subscription trigger 与 Capability RPC 同时启用，任一部分配置或 mode 漂移均在 runtime 构造前 fail closed；该 mode 跳过旧 Kafka runtime 与旧 Temporal Worker，防止同 task queue 出现不同 Activity catalog。关闭时统一 process 先停 Kafka admission，再 drain Client/Worker/Core。Compose 继续固定 external MCP disabled + `foundation`。本地隔离演练使用 in-memory Temporal 与现代 MCP Client/Server，通过 17 项恢复/取消/只读 discovery 测试且 `tools/call=0`；真实凭据、DNS/TLS 与公网 evidence 仍未启用。
- 外部 MCP deployment route manifest 增加可选的可信 subscription trigger binding：每条 route 可绑定 exact Agent Definition ID/version 与静态业务参数，加载时同时通过代码 Capability schema、route egress 字段和规范 JSON 字节上限，并拒绝重复 Definition version。trigger binding 纳入 deployment SHA-256，参数或 Definition 漂移会改变 Temporal route history authority；Worker composition 在创建 RPC 前校验全部 mapping 属于 exact route catalog，并冻结快照供受管 Client selector 使用。旧 manifest 仍可省略 mapping，但无法进入后续 production subscription process。`index.ts`、Compose 与外部网络保持关闭。
- Agent Runtime 增加默认关闭的 Kafka + 外部 MCP Temporal Shadow process owner：只接受 subscription trigger，先启动完整 Temporal Worker/Client owner，再启动 Kafka consumer；停止、回滚和取消固定先关闭 Kafka admission，再 drain Client 并停止 Worker/Core resource，任一阶段失败仍继续且重复 stop 复用同一 Promise。Worker RPC resource 将同一个认证 client 作为借用 subscription matcher 沿 startup/lifecycle/process 透传，Kafka dispatcher 不再创建第二条 RPC 连接或分裂身份视图。disabled Kafka/Temporal、matcher 缺失和 Kafka 构造/启动失败均 fail closed。该 owner 尚未接入 `index.ts`、Compose、生产 route registration 或外部网络。
- Agent Runtime 增加默认关闭的外部 MCP Shadow Temporal process owner：按 `Worker bootstrap -> managed Client` 顺序启动并传递同一 AbortSignal、Temporal config 与 exact Worker catalog；disabled Worker 不创建 Client。Client 启动失败或交接后取消会回收 Worker，清理失败固定分类；正常停止固定先 drain/关闭 Client，再停止 Worker/connection/Core resource，任一阶段失败仍继续且重复 stop 复用同一 Promise。该 owner 只公开 exact deployment/worker/config、可信 dispatch 与 stop，尚未接入 Kafka、`index.ts`、Compose、生产 route registration 或外部网络。
- Agent Runtime 增加默认关闭的外部 MCP Temporal Client lifecycle：Worker lifecycle 现冻结并公开实际 Temporal config，Client owner 直接复用同一 address/namespace/task queue 与 exact Workflow route catalog，调用方无法另传漂移配置。disabled Worker 零 selector/连接调用；enabled 时惰性构造代码 route selector 和独立 Client resource，取消/启动失败回滚关闭，错误固定脱敏。stop 先拒绝新 dispatch、等待已接受的 Workflow start 收敛，再幂等关闭连接；Client 不接管 Worker/Core resource。该 owner 尚未接入 `index.ts`、Compose、Kafka runtime 或生产 route registration。
- Agent Runtime 增加受信 subscription-to-MCP route binding：subscription mode 现在把 Core 已校验的 winning subscription ID、definition ID/version、tenant 与 Agent 作为可选 `AgentEvent.subscriptionBinding` 一并投影，并保持旧 direct-target/仅 subscription ID 事件兼容。`TemporalMcpSubscriptionRouteSelector` 仅按代码注册的 exact definition version 选择 route，先复核事件 subscription ID 与运行 tenant/Agent，再调用参数 resolver；空集、重复 definition binding、未知版本、附加 route authority 和非对象参数均 fail closed。生产尚未注册任何 definition route/resolver，也未接入 `index.ts`、Workflow Client、Compose 或网络。
- Agent Runtime 增加默认关闭的可信外部 MCP Shadow dispatcher：先复算事件、tenant 与 Agent 绑定的确定性 Task ID，再从已验证 `AgentEvent`/`AgentIdentity` 固化 admission 与固定 goal；宿主 route selector 只能返回严格的 route ID 和业务参数，无法提交 tenant、principal、Agent、admission 或 goal。selector 输入在调用前冻结，输出拒绝附加字段；专用 Temporal client 继续由 host catalog 注入 route version/manifest digest。该原语尚未接入 `index.ts`、Kafka subscription route mapping、Workflow Client 或 Compose，外部 Worker 与网络仍关闭。
- Agent Runtime 将外部 MCP Worker 收敛为 single-RPC Activity snapshot：Agent Capability RPC resource 现从同一个认证 client 派生 persistent admission/finish/projection/approval Activities，同时继续把该 client 用作 MCP Core 与 Artifact writer；host 提供的 `executeAgentTaskStep` 保持不变。managed startup 在创建资源前用 host base Activities 静态校验，资源后再用冻结的实际 snapshot 执行 composition 与 collision/binding 校验；通用 resource 仍可省略 snapshot。Shadow bootstrap 显式传递 base Activities，未来生产接线无需另建 `temporalRPC`。`index.ts`/Compose 与 Workflow client 仍未修改。
- Agent Runtime 增加默认关闭的外部 MCP Shadow Worker bootstrap：用代码 seal 的只读 definitions 加载 exact deployment snapshot，在 enabled 路径内惰性创建 Agent Capability RPC resource，再交给 managed Worker startup 与 Temporal lifecycle。disabled deployment 只构造 definitions 并读取 Profile 开关，不创建 RPC factory/transport 或 Worker；交接前取消由 bootstrap 关闭 startup，交给 lifecycle 后不再重复回收，构造与清理错误固定脱敏。成功结果保留 exact deployment、Worker composition、Workflow route catalog 与有序 stop。该 root 尚未接入 `index.ts`/Compose，也不创建 Workflow client；若未来启用，外部 egress 仍逐请求要求 fresh readiness evidence。
- Agent Runtime 增加默认关闭的外部 MCP Agent Capability RPC resource factory：仅在 managed startup 请求资源时创建认证 RPC transport，并把同一个 `AgentCapabilityRPCClient` 同时投影为 MCP Core 与 Artifact writer，避免身份或连接视图分裂。factory 在建连前要求 RPC 已启用、deployment 至少一个 Profile 且全部 tenant 与 Shadow config 精确一致；构造后取消会回滚关闭，显式关闭成功或失败均幂等，构造/清理错误固定脱敏。该层不修改 Proto、`index.ts` 或 Compose，外部 Worker 与网络仍未启用。
- Agent Runtime 增加首个代码拥有的外部 MCP 只读 Capability definition `repository.issue.read`：参数严格限制为规范化的 `owner/repo/issue_number`，资源 authority 固定为单个 `repository_issue`，权限与 Capability ID 对齐，egress ceiling 仅允许三个参数且最多 1 KiB，deployment route 只能进一步收窄。Definition Registry 新增显式 seal，并冻结 descriptor/ceiling/resolver snapshot，代码 factory 注册后立即封口，部署或启动层无法追加 write/未知定义。该 factory 不读取环境或 manifest，不创建 RPC/网络资源，仍未接入生产启动链。
- Agent Runtime 增加默认关闭的外部 MCP Temporal Worker lifecycle owner：disabled startup snapshot 不创建 Worker；enabled 时仅用同一 snapshot 的 Activities 启动现有 Temporal Runtime。disabled Temporal config、Runtime factory/启动失败都会回收 startup resource；正常停止严格执行 Worker/Temporal connection shutdown 后再关闭 Core/Artifact resource，任一阶段失败仍继续清理，重复停止复用同一 Promise，并将启动、清理和停止错误固定脱敏。该 owner 尚未接入 `index.ts`/Compose，也不创建真实 RPC 或开启外部网络。
- Agent Runtime 增加受管且默认关闭的外部 MCP Temporal Worker startup plan：以同一 AbortSignal 串行执行 deployment load、无副作用静态 composition validation、受管 Core/Artifact resource factory 与 Worker composition；disabled 或静态无效的 deployment 均不创建 resource。资源创建后的取消、composition 异常或空结果会先执行 rollback close，构造和清理错误固定脱敏；成功计划返回 exact deployment/worker snapshot 与幂等 close，底层关闭成功或失败都只调用一次。该计划不创建 Temporal Worker/Workflow Client，不执行 preflight、DNS 或外部连接，且尚未接入 `index.ts`/Compose。
- Agent Runtime 增加默认关闭的外部 MCP Temporal Worker composition：undefined deployment plan 直接返回且不解析 Core/Artifact 端口；enabled 时先复算 exact Runtime binding、验证全部 route bindings、Capability egress policy 与 Activity 名称唯一性，再惰性构造 multi-route runtime。输出将通用 lifecycle Activities、唯一 `executeMcpDispatch`、匹配的 host Workflow catalog、route bindings 和低敏 Runtime digest 收敛为一个不可变 bundle；Runtime 工厂返回任何 binding 漂移都会拒绝。Temporal Worker 注册类型已允许该 additive Activity，但 `index.ts`、生产 Worker、RPC 创建、Temporal Client、preflight、DNS 与网络仍未接线。
- Agent Runtime 增加版本化 `external_mcp_v1` Temporal Workflow history envelope 与专用启动 client：调用方只选择宿主 catalog 中的 route ID 并提交 16 KiB 内 JSON 业务参数，route version/manifest digest 由部署计划返回的 binding 注入，Profile/Server/Tool 不进入启动 API。Workflow 有 envelope 时调用唯一 `executeMcpDispatch`，首次从持久 admission 派生 Task/Run/principal/correlation，恢复时只传 durable checkpoint 与已验证用户输入；普通 goal/event 路径继续使用 `executeAgentTaskStep`。真实 Temporal 测试覆盖 Worker 替换与 Elicitation resume。生产 Worker、`index.ts`、Compose 和网络仍未注册该 Activity。
- Agent Runtime 增加默认关闭的外部 MCP multi-route Temporal Activity dispatcher：部署计划中的每条 route 只构造一次 route-scoped runtime，统一的 `executeMcpDispatch` 入口仅从 begin 输入或 durable checkpoint 提取 route ID，再把完整 payload 交回对应 Activity 校验 route version、manifest/deployment digest、Context 与 checkpoint。空路由、重复或未知 route 以及残缺 routing payload 均在 Core/Transport 前 fail closed；调用取消和 route-local 错误保持原样传播。该 dispatcher 已由 `external_mcp_v1` Workflow 分支引用，但未注册到生产 Temporal Worker、`index.ts` 或 Compose，也不启动网络。
- Agent Runtime 增加默认关闭的外部 MCP deployment composition plan：先固定 owner/取消边界并加载一次 Profile、production I/O manifest 与 deployment route manifest，全部交叉校验成功后才构造无文件依赖读取、无 DNS/socket 的 production runtime。计划将同一 config、I/O、raw Registry 和 readiness binding options 同时提供给 readiness collector 与 gated Worker，并公开可复核的低敏 Runtime binding；输出不含 RPC、Temporal Worker、start/stop、自动 preflight/drill 或网络启动。disabled 模式不读取 Profile/manifest 残留配置，任一 manifest 漂移或取消均不返回部分计划；`index.ts` 与 Compose 仍未接线。
- Agent Runtime 增加 credential-free 外部 MCP deployment route manifest v1：安全 loader 仅在 enabled Profile 下读取 owner-only、canonical parent、`O_NOFOLLOW`、regular/single-link 的有界 UTF-8 JSON；manifest 固定 route/version、Capability、Workflow step/ordinal、Profile/Server/Tool 与 egress policy，并与代码定义的 schema/resource resolver/egress ceiling 及 Profile allowlist 精确 join。重复 route、Capability 或 Workflow 坐标、Tool/Server 漂移及 policy 扩权统一 fail closed。完整部署绑定摘要进入 Temporal route history/checkpoint，漏升版本的 Profile/Tool/egress 漂移也会在 Core 前拒绝。该 loader 未接入 `index.ts`、Temporal Worker 或网络启动。
- Agent Runtime 增加外部 MCP fresh-readiness egress gate：MCP Worker 组合必须提供 host-owned Profile、production I/O 与 raw Registry，Runtime 在每次 `connect` 前自行派生 exact Profile/Runtime binding，并通过 Core 服务端时钟解析 fresh evidence；空结果、回执漂移、underlying Profile 漂移、取消或解析异常均在 Catalog/Transport/网络访问前 fail closed。回执不缓存，调用方无法提交 binding 或查询时间；readiness 采集继续使用独立 raw Registry，避免以现有 evidence 授权自身采集。该组合仍未注册到 `index.ts`、Compose 或自动调度，也不授予 Run admission、Runtime promotion、Profile activation 或外部网络启动权限。
- Agent Capability 增加 additive `ResolveFreshMcpReadinessEvidence` 只读 RPC：只允许 transport 认证且无 principal 的 `dipole-agent`，请求仅含 tenant 与 exact Profile/Runtime binding，查询时间由 Core 服务端时钟确定。应用 Resolver 通过 sqlc freshness 查询后重新验证 canonical evidence、确定性身份、双 binding、非未来采集时间与严格 expiry；响应只返回 `found`、摘要和时间，不包含 evidence JSON、operator 或 request/trace。TS client 对空响应和低敏收据逐字段复核；该 RPC 不读取或改变 admission、activation、promotion 状态。
- Agent Runtime 增加显式单次 `mcp:readiness:publish` 运维入口：命令必须提供 tenant、exact Profile、60 至 3600 秒有效期及 request/trace，先通过安全 manifest 构造 production I/O runtime，串行执行 local preflight 与只读 Shadow Tool discovery，再调用一次 Core Publisher。参数、采集、清理或响应校验失败均 fail closed 且不会发布；收据只含绑定摘要、时间与 created 标志。该入口未接入常驻 `index.ts`、Compose、自动调度、重试或 admission，执行命令是唯一触发外部 DNS/TLS/MCP discovery 的方式。
- Agent Capability 增加 additive `PublishMcpReadinessEvidence` RPC：仅接受 transport 认证且无用户 principal 的 `dipole-agent`，operator 由服务端绑定为认证 service identity，request/trace 从已验证 RequestContext 派生；请求只能提交 tenant、Profile binding、v2 evidence JSON 与 expiry，无法注入状态或 activation。Core 严格解析 16 KiB 内 evidence 后经 migration v37 Publisher 追加；TS client 复算 canonical content hash 与确定性 Evidence ID，并拒绝响应绑定漂移。RPC 已装配但没有自动采集、调度或 admission 调用。
- Agent Core 增加外部 MCP readiness evidence 追加式归档：保留 evidence v1 契约，新增 v2 exact Profile binding；migration v37 建立无 Task/Run 假 lineage、无更新列和无 activation 副作用的独立控制面表。Go Publisher 严格拒绝额外/敏感 JSON 字段，复算 canonical content SHA-256，并用 tenant、Profile/Runtime binding、operator、request/trace 和最长一小时有效期派生确定性 Evidence ID。exact replay 幂等，收据或 binding 漂移保留新历史；fresh 查询同时约束 tenant、双 binding 与 expiry。自动采集调度、admission、签名 attestation 和真实公网 Shadow 归档仍未启用。
- Agent Runtime 增加外部 MCP readiness evidence v1：production runtime 将同一次 local preflight 与 exact Profile 的只读 Shadow drill 收敛到最长 5 分钟窗口，并以 canonical SHA-256 绑定全部排序后 Profile policy、credential/key/secret/CA opaque ref、文件路径拓扑和生效上限。归档 bundle 只含 binding hash、四个时间、Profile/Credential/CA/Tool 计数，语言中立 schema 禁止附加字段；配置、路径或上限漂移会改变摘要，收据时间/计数漂移、依赖失败和清理失败统一拒绝。注入自定义 Transport builder 的测试组合不能生成 evidence。摘要不包含 secret/key/certificate bytes，也没有签名 attestation；`index.ts`、自动 admission 与真实公网 Shadow 归档仍未启用。
- Agent Runtime 增加默认关闭的外部 MCP Shadow connectivity drill：production runtime 只新增受约束演练函数，不公开 DNS/TLS/Secret adapters。调用方必须显式给出 exact Profile/tenant；演练经正式 Registry 完成 Catalog、Secret、public-only DNS、pinned TLS 和 modern MCP discover/list，要求全部 allowlisted Tool 可见后立即关闭 Client，且没有 `tools/call` 表面。成功收据只含 schema、时间和 Tool 数量，Profile、tenant、Server、Tool 与网络细节不落收据；失败与清理异常固定脱敏，取消传播。该函数未注册到 `index.ts` 或自动启动，真实公网证据仍需在隔离只读 Shadow tenant 中归档。
- Agent Runtime 增加外部 MCP production I/O readiness preflight：`createExternalMcpProductionIoRuntime` 只公开 tenant-bound Registry 与预检函数，使用同一固定逻辑时间解析全部 enabled Profile，并对 exact Credential binding 与 CA ref 去重后验证 Catalog 状态、AES-GCM/AAD/key、正式 Bearer 语法及 PEM/X509 文件。成功收据只含 schema、开关、时间和聚合计数；吊销、身份漂移、key/envelope/CA 损坏统一低敏失败。预检不创建 Transport、DNS client 或 socket，统一的 `maximum_secret_bytes` 同时约束 Provider、AuthProvider 与预检。`index.ts` 和外部网络仍未启用，真实连通性由后续隔离 Shadow 演练验证。
- Agent Runtime 增加外部 MCP production I/O manifest v1 与 default-off 安全 loader：语言中立 schema 只允许 Catalog、key/secret/CA opaque ref 的规范绝对路径及有界参数，运行时进一步要求引用和全部路径唯一、secret key 关联存在。主开关关闭时不读取残留 manifest 环境变量；开启后仅接受 owner-only、canonical parent、`O_NOFOLLOW`、regular/single-link 的 UTF-8 JSON 文件，每次加载重新读取且失败不回退旧快照，错误固定脱敏。loader 输出可直接构造 production Registry，但尚未注册到 `index.ts`。
- Agent Runtime 增加默认关闭的外部 MCP production I/O composition：`createExternalMcpProductionIoRegistry` 统一拥有文件 Credential Catalog、AES-GCM Secret Provider、request-local DNS、文件 CA、pinned TLS Dispatcher 与 Streamable HTTP Factory，只公开 tenant-bound Transport Registry。disabled 路径不触达残留 I/O 配置；enabled 构造仅验证映射，不读取文件、查询 DNS 或建连。Catalog 每次 connect 重载，Secret 每次 token 重载；revoke 在 Transport 前阻断，provider identity 漂移在 DNS/TLS 前以固定错误失败。`index.ts` 尚未注册该组合，外部网络开关继续关闭。
- Agent Runtime 增加默认关闭的外部 MCP AES-256-GCM 文件 Secret Provider：Catalog 的 exact credential binding 与版本化 `key_ref` 一起进入 AAD，密文篡改、错 key、tenant/ref/version/provider 漂移和旧映射移除均 fail closed。Provider 每次请求重新打开 32 字节私有 key 与有界二进制 envelope，要求 canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode，并在成功外的路径擦除 key/plaintext buffer；错误固定脱敏。该本地 envelope 适配器尚未装配到 `index.ts`，不等同于独立 KMS 托管，外部网络开关继续关闭。
- Agent Runtime 增加默认关闭的外部 MCP pinned TLS Dispatcher 与文件 CA provider：每次请求重新读取并验证 certificate-only CA bundle，通过自定义 lookup 只连接 Network Guard 批准的 DNS answer，禁用代理和连接复用，并同时校验 TLS chain、ServerName 与实际 remote peer。请求/响应 body 保持流式传输，TLS connect timeout 与取消会销毁独立 socket，3xx 原样返回给上层拒绝；CA 文件要求 canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode 与大小上限。适配器尚未注册到 `index.ts` 或 Worker startup，加密 Secret backend 与外部网络开关继续关闭。
- Agent Runtime 增加外部 MCP production DNS Resolver：每次 guarded fetch 创建独立 `node:dns/promises Resolver`，并行获取完整 A/AAAA 集合且不缓存；单族 `ENODATA/ENOTFOUND` 可由另一族补足，任何瞬时失败或畸形 family/TTL 证据均整体 fail closed。AbortSignal 只取消当前 Resolver，预取消不会创建 DNS client，上游错误统一为低敏消息。Resolver 尚未接入启动链，pinned TLS Dispatcher、加密 Secret backend 和外部网络开关继续关闭。
- Agent Runtime 增加单一 default-off Temporal MCP Runtime factory：由同一 Capability Route Registry 同时构造 stable Invocation producer 与 route-scoped Worker egress policy，再组合 fresh Core Context resolver、receipt-derived terminal Worker 和 untrusted Artifact projector。调用方不再重复提交 Profile/Tool egress map，factory 只返回版本化 route binding 与专用 Activity；端到端测试覆盖 completion-loss receipt/Artifact 重放、durable input 双轮恢复和预取消。生产 Worker、`index.ts`、Activity mode 与外部网络启动仍未接线。
- Temporal MCP dispatch route 增加完整 manifest 摘要：`temporalMcpDispatchRouteBinding` 对 route ID/version、Capability、Workflow step 与 ordinal 生成 canonical SHA-256，受信调度输入及 durable checkpoint 均持久化该摘要。替换 Worker 即使遗漏 route version 提升，只要 Capability 或 Step 坐标发生变化，也会在 Core context/producer 前 fail closed，避免 Activity completion 丢失后派生第二个 Invocation。摘要仍须来自 host-owned 静态 route manifest，模型、goal、事件和客户端无权提供。
- Agent Runtime 增加默认关闭的外部 MCP Artifact projector：完成路径只接受 ExecutionContext、Invocation/Round ID 与 terminal Worker 结果，随后从 Core 重新解析 completed Tool command，逐项核对 tenant/principal/Agent/Task/Run 及 Profile/Server/Tool/Capability。结果经 MCP 标准 schema、canonical JSON 和 128 KiB 上限验证后写入 Invocation-derived type 的 content-addressed Artifact，metadata 标记 `trust=untrusted` 并保存低敏 lineage；重放及 Artifact 已提交后的取消均收敛到同一收据。当前仅允许 running shadow Run，active policy、生产路由和启动接线仍关闭。
- Agent Runtime 增加独立且默认关闭的 Temporal MCP dispatch Activity：Workflow 历史固定 host-owned route ID/version 与业务参数，Activity 每次 begin/retry/resume 都从 Core 重新解析 ExecutionContext、精确重放稳定 Invocation producer，并仅把 Task/Run/Invocation 三个 ID 交给 terminal Worker。等待点以完整性 checkpoint 绑定路由、Step 坐标和 Worker 状态；完成后先把不可信外部结果投影为幂等 Artifact，Workflow 只接收 Invocation/Round/Artifact 收据。取消会在 Core、producer、Worker 和投影边界之间 fail closed；生产 Worker、启动入口、Activity mode 与外部网络仍未接线。
- Agent Core 增加 receipt-derived MCP Invocation 终态所有者：additive RPC 只接收 Task/Run/Invocation/Round 四个持久 ID，Core 重新加载并核对 read-risk Invocation 与 terminal Round，服务端派生结果摘要、字节数、错误码和首次延迟；完成后的重试直接核对已存终态，不由 Runtime 重传易漂移证据。旧 Finish RPC 拒绝带 Profile 的外部 Invocation，避免出现第二终态权威。默认关闭的 terminal Worker composition 在成功或稳定失败 Round 后调用新 RPC，`input_required`、`executing`、绑定漂移和 write Capability 均 fail closed；现有启动入口、Temporal 调度与外部网络仍未接线。
- Agent Runtime 增加默认关闭的受信外部 MCP Invocation producer：strict 输入仅接受 Workflow step/ordinal、注册 Capability ID、业务参数和可选 Approval；tenant/principal/Agent/Task/Run 来自 ExecutionContext，Profile/Server/Tool 来自注入式 Capability 路由。稳定 64-hex Invocation ID 只绑定 host-owned Workflow 坐标，参数或路由漂移复用同一 ID并由 Core exact begin 拒绝，避免配置变化生成第二个执行意图。producer 复用 PolicyEngine、schema 与 MCP egress guard，在 Core begin 前阻断越权资源、未声明参数和敏感字段。
- Agent Core 为外部 MCP Tool Invocation 增加精确 begin/finish 重放与终态 receipt 恢复：重复 begin 仅在全部权威身份、Capability、Profile/Server、canonical 参数、请求关联和审批绑定一致时返回原记录；重复 finish 逐项核对状态、结果摘要/大小/延迟、错误码与 action reference。`ResolveMcpToolCommand` additive 返回 Invocation 状态，terminal Invocation 的 round claim 只能读取并重放已存在 receipt，缺失或漂移时 fail closed，禁止创建新 round 或重新发网。
- Agent Runtime 增加默认关闭的 MCP Worker Runtime 组合器：将认证 Core command resolver/round receipt、tenant Profile Transport Registry、allowlisted modern Client、Activity-safe continuation 与三 ID dispatcher 组装为单一依赖注入边界。已取消请求会在 Core resolve/receipt claim 前停止；本地完成后替换 Runtime 只回放 canonical receipt，`ambiguous` 不创建 Client/Transport。Temporal 自动调度仍等待受信 Agent Step 创建持久 Invocation，不从 goal、模型输出或事件正文选取命令。
- Agent Runtime 增加外部 MCP Streamable HTTP Transport Factory：精确复核 Profile 与 Catalog 的 tenant/ref/version 绑定，为每次连接创建独立 AuthProvider、Network Guard 和官方 SDK Transport；每个请求重新读取 Bearer、重新解析全部公共 DNS 地址并核对 pinned peer，同时关闭 401 自动刷新、403 扩权和 SSE 自动重连。生产 Secret Provider、DNS Resolver、TLS pinned Dispatcher 与专用 Worker 仍未装配，外部连接开关继续 fail closed。
- Agent Runtime 增加默认关闭的 MCP Worker command dispatcher：初始输入严格只接受 Task/Run/Invocation ID，Profile、Server、Tool、Capability、参数和开始时间每次从 Core 持久 Tool Invocation 解析；稳定 request ID 与输入截止时间由 Invocation 派生，恢复前重新核对完整命令和 Activity checkpoint。连接 Session Factory 仅接收 tenant/profile/server/tool 四字段，不再可见 Task、Run、Invocation 或参数。生产 Worker 与外部网络开关继续关闭。
- migration v36、Core/TS RPC 与 MCP Activity 增加 durable Tool round receipt：确定性 Round ID 绑定 Invocation、轮次和请求摘要，MySQL 原子认领后仅原 owner 可一次写入规范结果或稳定失败；Temporal 丢失 Activity completion 时重放已存结果，遗留 `executing` 明确返回 `ambiguous` 并禁止自动 reclaim/retry。外部 Provider、网络开关和 Worker composition 继续关闭；远端已执行但本地收据尚未提交的窗口采用 at-most-once 失败策略。
- Agent Runtime 增加默认关闭的 MCP `2026-07-28` 手工 `input_required` 续接基础：仅接受一个普通 Form，持久 checkpoint 精确绑定原 Tool 参数、请求键、不透明 `requestState` 与 Server/Tool/Invocation lineage，恢复时由新请求回传同一状态；凭据字段、状态漂移、多请求和超限载荷均 fail closed。生产 Activity 编排、外部连接、多轮与敏感 URL 授权仍未启用。
- Agent Runtime 增加默认关闭的 MCP Activity-safe round runner：每个首次/恢复轮次均从 tenant-owned Profile 打开全新现代 Client/Transport，并在成功、取消或握手失败后关闭资源；外层 checkpoint 绑定 tenant/profile/server 与内层 continuation，credential 仅在 Registry `connect` 时解析。现有 Worker 模式与外部连接开关保持关闭。
- migration v35 与 Core/TS RPC 增加持久外部 MCP Tool command：`BeginMcpToolInvocation` 可按 all-or-none 保存 Profile、Server 和 16 KiB canonical 参数，Core 重算 SHA-256 并递归拒绝凭据字段；`ResolveMcpToolCommand` 只向认证 Agent Runtime 返回同一 running Task/Run/Invocation 的权威命令。Worker 切流继续等待外部 Tool round receipt 对模糊执行窗口的治理。
- Agent Elicitation Task Query 新增受限来源绑定：本地请求标记为 Agent，MCP 请求固定携带不可信的 Server/Tool/Invocation 元数据；Runtime 与 Web 双重拒绝凭据类字段，查询失败时 Web 清空缓存 Form 并隐藏上游错误，避免向失效 request 提交数据。
- 修复内部开发证书生成脚本的权限覆盖顺序：公开证书保持 `0644`，CA 与服务私钥最终固定为 `0600`；新增临时目录回归测试，防止后续演练或本地部署生成可被其他用户读取的私钥。
- 修复 C3 首次冻结直接回切的节点证据缺口：`rollback_requested` 现在必须先对当前 frozen lease 使用 source-node manifest 生成独立 `rollback_frozen_confirmed` artifact，再允许 source activation；覆盖 freeze 后尚未完成 target-node confirmation 即超时的路径，避免复用缺失或面向候选节点的旧 proof。

### 新增

- Agent Workflow repair 增加 `repair:plan` dry-run 执行计划编译器：仅接受已批准提案、双人审批、独立 executor grant 和重新采集的当前/目标/回滚投影，生成带三组 CAS SHA-256、15 分钟有效期和确定性 plan ID 的语言中立 v1 计划。计划生成不连接 MySQL/Temporal、不提供 apply/execute/rollback 字段，身份复用、回滚证据漂移和窗口外重放均 fail closed。
- 增加默认关闭的 `realtime-cpp` Compose profile：显式配置 `cpp` authority、Primary RPC、Redis fencing epoch 和维护窗口后，才会启动独立 C++ Realtime Delivery；默认 Compose 继续使用 Go，profile 未启用时不创建 C++ 服务。C++ 进程通过 Kafka primary group、Redis authority 和 Gateway mTLS node transport 工作，回滚恢复 Go 配置并移除 profile。
- 增加语言中立 `dipole.agent.memory-derived-lineage` v1 manifest/report、严格 Zod 解析器和 `audit:memory-derived` CLI。owner 授权 manifest 保持本地敏感输入，标准输出省略 tenant、principal、Memory ID 与全部正文；MySQL 审计账号仅新增 Memory 与 lineage 两张表的只读权限。
- Agent Memory 增加 append-only owner correction：migration v39 为每条记录保存 root/version/predecessor/corrector/reason，唯一 predecessor 与 `(tenant, root, version)` 约束阻止分叉；sqlc transaction 在同一事务中撤销前序版本并追加 successor，稳定 correction ID 支持精确重放，payload 或期望版本漂移返回冲突。additive gRPC、Gateway 与 Vue 已形成完整闭环，`VITE_AGENT_MEMORY_CORRECTION_ENABLED=false` 默认关闭纠正入口，Pencil 文件维护 desktop/mobile 与六类纠正状态。
- Agent G3 增加默认关闭的 Memory owner 管理闭环：migration v38 与 sqlc 提供稳定 cursor 分页、owner 隔离的 authoritative get/revoke 和完整撤销审计；additive Agent gRPC 由 Gateway-only 控制面调用，公开 list/revoke API 与 canonical Pencil desktop/mobile 设计、Vue 页面覆盖 loading、ready、empty、inactive、expired、unavailable、revoking 和 conflict 状态。长期 Memory 始终显示 `UNTRUSTED MEMORY`、owner provenance 与自动写入关闭状态；纠正入口等待 append-only 版本模型后再开放。
- 增加语言中立 `dipole.agent.subscription-shadow-collection.v1` 与只读 Prometheus Collector：从无凭据 origin 执行固定 19 次历史查询，要求单 Agent series、全窗口 Shadow enabled、vector 单值和非负安全整数，自动生成 evidence v1 所需的起止 counter、抓取覆盖与 reset 输入；Collector 不修改共享状态、不输出 Prometheus URL，也明确保留部署 revision 的发布记录核验门槛。
- 增加语言中立 `dipole.agent.subscription-shadow-evidence.v1` 与独立 CLI：Prometheus 起止快照绑定 24 小时以上窗口、Runtime/config SHA-256、query revision、抓取覆盖率、六类 comparison、candidate 和 counter resets；至少 95% 抓取、100 个事件、零 reset、零 matcher error 才生成最多有效 24 小时的 canonical-hashed passing evidence。输入/证据 Schema 均拒绝附加字段，收据固定 `production_authority=false` 与 `runtime_change_authority=false`，Runtime 启动链不读取该文件。
- 增加默认关闭的 Agent Subscription 在线 Shadow 对照：`direct_target` Kafka handler 在 EventLedger 前调用同一 Core matcher，只记录固定 `accepted|ignored × match|miss|error` 矩阵和候选总数；matcher 异常不阻断主路径，且不会创建第二个 Task、Workflow 或模型调用。Agent `/metrics` 暴露低敏零值/开关状态，Prometheus 新增 matcher error 与 admission drift 告警；Compose 固定关闭，启用与回滚见 `docs/agent-subscription-shadow.md`。
- 增加默认关闭的 Agent Event Subscription owner create 闭环：Core additive RPC 将 authenticated readable conversation 与精确 Definition scope 求交集并派生 direct/group event type；Gateway 只接受 Definition、conversation 与确定性 filter，从 JWT/配置派生 principal/tenant，并在创建时重新派生 resource。Vue 从 active Definition 目录和权威候选中选择绑定，关键词超限显式阻止提交，成功后以 Core 结果更新列表；canonical Pencil 增加 desktop/mobile 创建稿、七类状态和两个复用组件。Runtime 与 Compose 继续固定 `direct_target`。
- 增加默认关闭的 Agent Definition catalog：sqlc 按 tenant/owner、服务端有效期、`conversation.read` 与可读 conversation scope 筛选 active 版本，应用层二次复核 authority；additive Core RPC 与 Gateway `/api/v1/agent/definitions` 从认证 principal 派生 owner，并以 opaque 复合 cursor 分页。公开投影不含原始 permissions、owner 或非 conversation scope；前端严格 API parser 已就绪，conversation chooser、create UI 与 Runtime 切换继续关闭。
- 增加默认关闭的 Agent Event Subscription owner list/revoke Web 闭环：Gateway 复用现有 Core Agent gRPC 连接，从认证会话派生 principal 并固定 tenant；Vue 以严格响应解析展示 Definition version、conversation scope、确定性 filter 与撤销审计，查询失败清空旧状态，撤销要求精确原因并以权威响应收敛。desktop/mobile 路由已通过 Chromium、Firefox 与 WebKit 验收。该阶段公开 Definition 目录与 create 尚未交付，现由同一 `Unreleased` 中的 owner create 闭环补齐；Runtime `subscription` 继续关闭。
- canonical Pencil 增加 Agent Event Subscription v1：desktop owner 管理页、`loading|empty|unavailable|definition_stale|revoking|revoked` 六态矩阵、mobile 精确撤销确认层及三类可复用组件；设计固定披露 Definition version、conversation scope、确定性 filter、审计原因与 `direct_target` Shadow 边界，2x 评审图归档于 `design/exports/agent-subscription-v1/`。owner list/revoke 已由默认关闭的 Gateway HTTP/Vue 管理页实现；Definition 目录和 create 设计由后续同节条目补齐，Runtime `subscription` 模式仍未启用。
- Agent G4 全栈 Shadow 演练增加测试专用 Go Core RPC fixture：复用生产 TLS 1.3、双向证书验证、shared-secret metadata、caller allowlist 和证书 CN 绑定；脚本生成临时 `dipole-core`/`dipole-agent` 身份并验证错误 secret、错误 CN 与无客户端证书均失败关闭。
- 外部 MCP Shadow 演练证据升级为 v2，新增 `core_rpc_type=go_internal_grpc_mtls`、认证成功和身份拒绝验证门禁；v1 Schema 保留用于解释历史文件，当前 CLI 只接受带完整 Core RPC 证据的 v2。
- Agent G4 隔离全栈 Shadow 证据增加语言中立 JSON Schema、严格 Zod 解析器和 `mcp:shadow-drill:check` CLI；契约固定成功计数/布尔门禁、canonical `content_sha256` 与最多 24 小时有效期，并拒绝额外字段、未同步 hash 的内容漂移、未来或过期文件。
- Agent G4 增加默认跳过的隔离全栈 Shadow 演练：随机 Compose 项目启动独立 MySQL 8.4 与 Kafka 3.9，测试进程启动内存型 Temporal、owner-only route manifest、可信 Core 夹具和本地只读 MCP Server；演练输出 owner-only、无标识符/正文/凭据的 v1 JSON 证据并自动清理容器、网络和卷。
- 增加默认关闭的 Agent Elicitation Web 闭环：`/agent/tasks/:taskId/input` 根据 authenticated Task Query 渲染 `text|select|multiselect|boolean`，精确绑定 Task/request 提交并在提交、取消或过期后重新查询权威状态；desktop/mobile 响应式页面与 3 项组件行为测试已接入，入口由 `VITE_AGENT_ELICITATION_ENABLED=false|true` 控制。
- Agent Elicitation Web 补齐浏览器验收与可访问性语义：Chromium、Firefox、WebKit 覆盖 authenticated Task/request 精确绑定、外部 MCP 来源披露、首次查询失败后的 stale Form 清理与恢复、校验失败聚焦首个无效字段，以及 390x844 单列布局；Form 同步暴露 busy、alert、`aria-invalid` 和错误描述关系，敏感字段与 URL mode 继续关闭。
- canonical Pencil 增加 Agent Elicitation v1：desktop/mobile 普通 Form、外部来源披露、四类受限字段、七态矩阵及三类可复用组件；设计明确旧 request、敏感字段、URL mode 和依赖不可用时 fail closed，2x 评审图归档于 `design/exports/agent-elicitation-v1/`。
- canonical Pencil 增加 Agent Workflow Repair v1：desktop evidence review、`proposed|approved|rejected|expired|unavailable` 六态矩阵、mobile 双人审批层及三类可复用组件；界面明确批准只形成审计结论，不执行 projection repair，2x 评审图归档于 `design/exports/agent-repair-v1/`。
- 增加 canonical Pencil 前端设计，覆盖 foundations、可复用 IM 组件、Login/Chat desktop/mobile 与关键异常状态，并保存 2x 评审导出图。
- C3 增加持续 cutover controller 并关闭 `AD-041`：`dipole-realtime-cutover -operation run` 在一个同步循环中统一拥有状态推进、冻结超时回切、阻塞重试与临近到期的 authority lease 续期。Redis attempt-scoped ownership 通过 owner token 的 acquire/renew/release Lua 比较阻止并发 controller；control lease 至少覆盖两倍 action timeout。当前 authority deadline 只从 initial input 或最新 journal-bound transition artifact 恢复，拒绝采用未入 journal 的孤儿 transition；`rollback_requested` 续租保留回切意图和二次冻结要求。隔离 Docker Redis + race 演练证明 Controller A 无 release 退出后，B 在 5 秒 TTL 前被阻断、到期后从 sequence 1 继续到 completed sequence 6，证据归档于 `benchmarks/c3-cutover-controller-2026-08-28/`。
- C3 隔离故障演练接入真实 C++ Primary authority：演练改用 canonical message topics 与 `dipole-realtime-primary-*` group，目标激活后停止 Go Primary 夹具并启动当前源树构建的 `dipole-realtime-delivery primary`。目标 checkpoint 的 `realtime-delivery/cpp-a` observation 禁止由测试夹具代写，必须由 C++ 进程在校验 CPP active lease 后写入 Redis，并同时通过真实 librdkafka assignment 与 `/readyz`；报告绑定 C++ 二进制、observation payload、consumer group 和 journal 的 SHA-256。持续 controller 所有权仍由 `AD-041` 跟踪。
- C3 增加 production cutover executor：启动时校验 attempt 对初始 lease、三阶段节点清单和双组 checkpoint 清单的 SHA-256 绑定；每个动作固定执行 `artifact lookup -> Redis receipt recovery -> new side effect`，只有明确缺失 receipt 才允许新的 CAS。source/target/rollback checkpoint 复用节点聚合器和双组 collector，正常切换、冻结期源恢复及目标激活后二次冻结回退均验证 authority、epoch、phase、lease 与 manifest。完成、回退意图和回退完成使用版本化 decision artifact。真实 Redis writer + 模拟 observation/Kafka collector 集成覆盖 forward、两条 rollback 和 transition 成功但 artifact 缺失的恢复。恢复 CLI、租约续期和真实 crash/rebalance/Redis 故障演练仍待完成。
- C3 增加自包含 cutover attempt workspace：创建时 canonicalize 并不可覆盖保存 initial transition、source/frozen/target 节点清单与 checkpoint 清单，由代码生成精确绑定 initial lease 和全部输入摘要的 `attempt.json`；重试创建只接受完全相同的 canonical 输入。恢复加载会严格解码并重算每个绑定，再打开独立 artifacts 目录，避免操作员在续切时重新提供已漂移的外部文件。
- C3 增加 `dipole-realtime-cutover` 恢复命令：`create` 在 initial lease 有效期内生成 workspace，`status` 无需 Redis/Kafka 即可回放状态，`advance` 每次只执行一个外部动作并落盘一个事件，`rollback` 从合法状态记录回退意图。变更操作要求显式确认与 operator，单动作 30 秒超时；模糊失败后重复调用会恢复同一 artifact/receipt，禁止一次进程跨多个未持久化副作用。
- C3 增加证据链内的单步 lease 续期：`renew` 使用最新 transition hash 执行可恢复 Redis CAS，将 receipt/artifact/event 依次持久化且不重置冻结中断预算。续期后，绑定旧 lease 的 source checkpoint、frozen observation、target checkpoint 或 rollback observation 会回退至前一采集状态，后续切换必须重新生成当前 lease 证据；真实 Redis 测试验证续期后的 checkpoint 精确绑定新 receipt。持续调度与真实故障演练仍由 `AD-041` 跟踪。
- C3 增加可复跑的隔离故障演练并归档 `benchmarks/c3-cutover-faults-2026-08-28/`：随机 loopback Kafka/Redis、两个真实 consumer group、生产 Redis observation/checkpoint 组件及 TCP fault proxy 在 race 下验证 controller artifact/journal 崩溃恢复、Redis outage 阻断与恢复、primary group member 丢失/重建，三次故障均未错误推进 journal，forward 最终完成 sequence 7。第二 attempt 以 500 ms 中断预算验证首次 freeze 超时后自动回切，经 source-node frozen proof、Go epoch 2 激活和 source checkpoint 收敛为 `rolled_back` sequence 7。脚本自动校验低敏报告并清理 Compose project/volume；持续 controller 与 C++ primary 切流仍由 `AD-041` 跟踪。
- C3 增加持久化 cutover attempt 状态机、单步恢复 orchestrator 与 action artifact store：不可变 manifest 绑定源/目标 authority、初始 epoch、中断预算、三阶段节点清单和双组 checkpoint 清单摘要；哈希链事件日志严格约束顺序、时间与 artifact 摘要。每一步使用确定性 action ID，执行失败不推进 journal，替换进程会重试同一动作；首次 freeze 超过中断预算后自动转入源 authority 回退。artifact envelope 保存并分别绑定完整 canonical action 与严格 JSON transition/checkpoint payload，可脱离控制器状态独立复核；同一 action 重放返回原文件，任何 binding 漂移 fail closed。Redis writer 可按稳定 action ID 严格恢复既有 receipt，覆盖 transition 成功后进程崩溃的模糊窗口，并统一拒绝 receipt 重复字段。正常续切、冻结期直接回退，以及目标激活后二次冻结回退均有确定性归约。文件采用 `0600`、不可覆盖写入与目录 fsync。恢复 CLI、租约续期和真实自动回切演练仍由 `AD-041` 跟踪。
- C3 归档首个隔离双组 checkpoint 实证到 `benchmarks/c3-cutover-checkpoint-2026-08-28/`：真实 kafka-go compatibility member 与 C++ librdkafka member 均为 `Stable`，direct/group 两分区的 committed/log end 都为 `1/1`、lag 0，active epoch 1 的两节点 proof 与 lease hash 完整进入 mode `0600` bundle。删除预期 C++ observation 和停止候选组分别被 fail closed，且未创建输出文件；共享三节点未被修改。该证据验证收据链与跨客户端 assignment，C++ 进程使用 shadow authority，完整 primary 切换和自动回切仍待后续演练。
- C3 增加预期节点聚合与持久双消费组 checkpoint bundle：版本化 manifest 显式列出 Gateway/C++ 实例及其本地 expected authority，控制面逐键读取未过期 Redis observation。active 要求全部节点指向目标 authority 且 authorized；frozen 允许 Go/shadow/C++ 准备节点并存，但全部必须对同一 transition lease 报告 `denied/frozen`。proof 精确绑定 observed authority、epoch、phase、deadline 与 `next_sha256`。只读 Kafka collector 同时验证 compatibility/primary group 均为 `Stable`、assignment 完整、逐分区 committed=read-committed log end、两组高水位一致，并在采集结束后复验短 TTL 节点 proof。原始 DescribeGroups adapter 严格解析 ConsumerProtocol assignment v0-v3 的有界 topic/partition 前缀，并容纳 kafka-go、librdkafka 等客户端的 opaque 扩展尾部；真实隔离 kafka-go + C++ librdkafka 双组收据通过。`dipole-realtime-cutover-checkpoint` 将完整节点聚合与哈希绑定的分区 receipt 以 `0600`、不可覆盖、文件及目录 fsync 的 JSON bundle 落盘。自动续切/回切和真实共享中断演练仍由 `AD-041` 跟踪。
- C3 增加默认关闭的共享 delivery fence Go/C++ 数据面：版本化 Redis JSON lease 固定 `epoch + authority + active|frozen + expiry`，跨语言 golden vectors 统一合法、冻结、过期、ABA、未知、重复字段、非法 phase 及非正 lease 的授权结果和 reason code。Gateway 在每条 message-created 写入/checkpoint 前读取；被拒 handler 停留在当前 Kafka record，不进入业务 retry/DLQ。C++ shadow/primary 在每个 pending record 投影前读取，拒绝时不写 evidence/commit。新增本地运维 CLI，以 Redis Lua CAS 强制 `bootstrap-go -> freeze(epoch+1) -> activate(same epoch) -> renew`，原子保存有 TTL 的低敏幂等 receipt；真实隔离 Redis 生命周期通过。Go Gateway 以稳定 Presence 节点 ID 在启动、5 秒空闲心跳和 readiness 写 15 秒 observation；C++ 要求显式 instance ID，在 evidence/Kafka 初始化前及空 poll heartbeat 写同一契约。两端均绑定预期/实际 authority、epoch、phase、lease deadline 与原始 lease SHA-256，写失败 fail closed；消息热路径保持只读或节流，避免 observation 写放大。真实 C++ Redis 演练在 Kafka 不可用期间持续刷新 authorized epoch 18 记录。预期节点聚合、持久 checkpoint receipt 和自动回切尚未实现，`AD-041` 继续处理中。
- C3 归档本地互斥 delivery authority 实测到 `benchmarks/c3-delivery-authority-2026-08-28/`：隔离 `go` 模式对目标消息只产生一条无 delivery ID 的客户端 frame；隔离 `cpp` 模式只产生一条带稳定 C++ delivery ID 的 frame。两种模式的 Gateway authority 指标均与配置一致，`cpp` 下 Go checkpoint group 与 C++ primary group 同时追至 log end/lag 0，terminal evidence 为 `ENQUEUED(1)/commit`。自动报告 12/12 通过，隔离资源已清理且共享三节点持续运行。本证据完成本地单帧门槛；共享动态 fencing、双 group receipt 和自动回切仍由 `AD-041` 阻断。
- C3 增加默认 `go` 的 `realtime.delivery=go|shadow|cpp` 本地 authority 契约：Gateway 在启动副作用前校验 observation/primary 能力组合，并以有界 Prometheus 标签暴露当前 ownership；`go` 与 `shadow` 保留 Go 消息客户端写入，`cpp` 对消息 Topic 仅验证 v1 事件并推进原 Go consumer group checkpoint，继续处理踢下线、已读和群变更等非消息事件。C++ `shadow`/`primary` 命令分别要求 `shadow`/`cpp` authority，配置错配会在连接 Kafka 前 fail closed。该切片尚未提供跨副本共享 fencing、双 group 切换 receipt 或自动回切，`AD-041` 保持处理中且 tracked Compose 继续为 Go authority。
- C2 归档显式 primary runtime 的真实证据到 `benchmarks/c2-primary-runtime-2026-08-28/`：干净同 revision Go/C++ 镜像在隔离 topology 中完成 `ENQUEUED(1)` 后 offset commit；600 KiB C++ probe 将真实 WebSocket queue 压至 16/16 并返回 `PARTIAL/BACKPRESSURED/QUEUE_FULL`；错误 gRPC target worker 对同一 Kafka 坐标写 deferred/retain，`SIGKILL` 前 lag 为 1，正常 worker 重放后 commit 且 lag 归零。报告 8/8、校验和与低敏扫描通过。并行运行的 Go consumer 同时产生 legacy frame，暴露双投递并登记 P0 `AD-041`，因此 C2 runtime evidence eligible，C3 cutover blocked。
- C2 增加默认关闭的显式 C++ primary runtime：`primary` CLI 要求命令入口、`DIPOLE_REALTIME_PRIMARY_ENABLED=true`、Presence primary 与 node transport primary 同时成立，并使用独立 `dipole-realtime-primary-*` Kafka group。Runner 固定 `poll -> project -> Presence -> DeliverNodeBatch -> primary-evidence.v1 -> commit`；只有完整 terminal `ENQUEUED|OFFLINE` ACK 集合或全部 Presence offline 才提交，partial/backpressure/rejected/failed、身份漂移、RPC 或 evidence 故障均保留同一 pending record。shadow v3 schema、命令和默认 Compose 保持不变；真实 consume-to-ACK commit、queue saturation 与进程崩溃重放仍待归档。
- C2 冻结 primary runtime 的 Kafka offset 决策边界：C++ 对完整 `NodeDeliveryBatch/DeliveryAck` 集合复核 batch/delivery identity，仅当所有结果均为 terminal `ENQUEUED|OFFLINE` 时给出 `commit`；`PARTIAL/BACKPRESSURED`、`REJECTED`、`FAILED` 与任何身份/数量漂移均给出 `retain` 或 fail closed。该纯分类器同时约束 one-shot probe 与显式 primary runner。
- C2 归档默认关闭的 primary seam 跨进程证据到 `benchmarks/c2-primary-delivery-seam-2026-08-28/`：clean revision 的 C++ one-shot probe 经 mTLS 精确投递真实 WS connection，首次 ACK 为 `ACCEPTED/ENQUEUED(1)` 且客户端只收到一条带稳定 `delivery_id` 的帧；相同批次模拟 ACK 丢失后重放，ACK 保持一致且无第二帧；Redis TTL 尚存但 Hub 已关闭的 connection 返回 `ACCEPTED/OFFLINE`。自动报告 7/7 检查通过，并明确保留真实部分 queue saturation 与 primary runtime Kafka offset/crash replay 两项后续门槛。演练同时发现 WS query JWT 进入访问日志，归档已脱敏并登记 `AD-040`。
- C2 增加默认关闭的 Gateway primary Delivery seam：语言中立 `NodeDeliveryService.DeliverNodeBatch` 返回逐项 `DeliveryAck`；Hub 按 `user_id + connection_id` 精确入队并区分 enqueued/offline/backpressured，稳定 `delivery_id` 进入 additive WebSocket envelope。Go dispatcher 对同一 batch 的 payload/target 漂移 fail closed，按 delivery/connection 保存有界 terminal 状态，部分成功重试只触达未完成 connection；Web 以账户隔离的 IndexedDB v4 原子 claim 和 4096 项有界 FIFO 去重重连及页面重载重放，登出时清理对应账户。C++ transport 已验证认证调用和 ACK，ShadowRunner 继续固定 `ObserveNodeBatch`，配置 `delivery_primary_enabled=false`，尚未进行生产切流。
- C2 关闭 `AD-039`：Gateway readiness 新增消费组级 `kafka-assignment` 探针，首次成功前保持不可就绪，并要求 group 为 `Stable` 且所有已注册 base/retry topic 均有分区；现有依赖探针默认启动语义保持兼容。独立 clean revision 冷启动从 `Empty 0` 开始，首样本 `/readyz=not-ready`、assignment=0，约 10.2 秒后收敛为 `Stable 20`、`/readyz=ready`、assignment=1；证据归档到 `benchmarks/c2-gateway-assignment-readiness-2026-08-28/`。微服务 Compose 的内部证书目录支持覆盖，readiness smoke 使用临时目录并校验 assignment 指标。
- C2 归档同一 clean revision 的 Go/C++ 实时投递对照到 `benchmarks/c2-cpp-comparison-2026-08-28/`：Go concurrent workload attempted/accepted/persisted/received 为 40/40/40/40；C++ 观察 80 个 Kafka 坐标，按 `message_type=0` 选中 40 条并将 40 条初始化系统消息保留为 filtered-out，选中记录 projected 与 node requested/observed 均为 40/40，最终拒绝/背压为 0，comparison 决策为 eligible。真实 TCP Gateway queue saturation race 测试验证 `BACKPRESSURED/QUEUE_FULL`；该轮演练发现并登记了后续关闭的 Gateway assignment readiness 缺口 `AD-039`。
- C2 增加语言中立的 Go/C++ 同 workload 对照门禁：Go baseline 显式声明低敏 `message_type` selector，C++ v3 evidence 保留全部 Kafka 坐标并按 selector 排除好友初始化等系统事件；报告同时记录 observed、filtered-out 与 workload 计数。选中记录按 Kafka 坐标折叠 deferred 重试，要求最终 projected、全部节点请求 observed、最终拒绝/背压为零，并与 Go baseline v4 的接受、持久化、接收和 settled lag 精确匹配。报告绑定双端完整 revision 与输入 SHA-256，结构错误、blocked、eligible 使用独立退出码；canonical C++ gate 持续运行其单测与 schema 检查。
- C2 C++ ShadowRunner 在 transport、Presence、evidence 或 commit 失败后保留至多一条未提交 Kafka record，并沿用 Runtime 的有界错误退避在同进程重试；只有 commit 成功才清除 pending record，deferred 尝试不再重复累计 projected 统计。真实 backpressure 和同 workload Go/C++ 对照仍未启用客户端写入。
- C2 归档首个 C++ node delivery 跨进程 shadow 证据到 `benchmarks/c2-cpp-node-delivery-2026-08-28/`：C++ 经 Kafka 与 Redis Presence 聚合节点批次，使用 `dipole-realtime` mTLS 身份调用默认关闭的 Go Gateway observation receiver；Gateway 故障时 offset 保持未提交，恢复并重启 worker 后重放成功，回拨已提交 offset 命中稳定 batch 去重，最终 lag 为 0，全程客户端写入为 0。证据同时记录当前 deferred record 需要 worker 重启才能重放，以及发布镜像前必须刷新预构建 `dist/` 的边界。
- C2 增加默认关闭的 C++ node gRPC transport shadow：显式 `node=target` 路由、10 ms 至 30 s deadline、`dipole-realtime` 服务认证、correlation metadata 和可选 mTLS；明文 target 仅允许 loopback。ShadowRunner 顺序升级为 Presence 投影、节点观察、`shadow-evidence.v3`、Kafka commit；部分成功后的重试依靠稳定 batch ID 与 Gateway 去重，RPC 拒绝/背压/故障会写低敏 `deferred` 证据、保留 offset 并撤销 readiness。真实本地 gRPC、runner 顺序、`-Werror`、clang-tidy 与 11 项 CTest 通过，生产 Compose 仍未启用。
- C2 增加 Gateway 节点投递观察接收端：默认关闭的独立 gRPC listener 只允许认证的 `dipole-realtime` 调用方，按 Presence node ID 校验 `NodeDeliveryBatch`，通过有界队列、稳定 batch 去重和明确 backpressure 返回观察结果；消费端仅累计低敏批次/条目/连接计数，不调用 WebSocket Hub，也不改变现有 Go Delivery。配置样例固定 loopback、容量和重试提示，race 与完整 Go 门禁通过。
- C2 增加 `NodeDeliveryService.ObserveNodeBatch` 跨语言观察合约：`NodeDeliveryObservation` 独立表达 observed/rejected/backpressured、节点批次聚合计数、QueuePressure 与 duplicate 去重，不复用客户端 `DeliveryAck`；Go/C++ validator 与第四组 golden vector 已固定。
- C2 归档 Presence Shadow 与 Sentinel 恢复证据到 `benchmarks/c2-cpp-presence-2026-08-28/`：同一 hiredis reader 在停止当前 master 后完成 80 次读取中的 75 次成功、5 次有界错误，并自动从 `redis-2` 恢复到 `redis-3`，无需进程重启；隔离项目、网络和卷已清理。
- C2 将 Presence 注入 Kafka ShadowRunner，并升级低敏证据为 `shadow-evidence.v2`：记录节点批次及 malformed/observed/eligible/stale/offline 聚合计数；Redis 读取失败不提交 offset，身份漂移记录 `invalid_presence` 后提交。真实联合回放 206 条，205 projected、1 poison rejected、20 个非空节点批次，最终 lag 为 0。
- C2 增加 hiredis 1.2 Presence 只读 adapter：支持 direct 或 Sentinel master discovery、AUTH/SELECT、批量 `HGETALL` pipeline、命令失败后断连重发现，以及 Go Hash 兼容解析；真实 Redis 隔离 fixture 验证通过，尚未注入 Kafka ShadowRunner。
- C2 增加 C++ Presence 纯投影边界：兼容解析 Go Presence Hash JSON/RFC3339 时间，将连接快照按 TTL 过滤后确定性分组为 `NodeDeliveryBatch`，显式统计 malformed/observed/eligible/stale/offline，并对用户身份漂移和跨节点重复 connection 所有权 fail closed；当前尚未连接 Redis 或写 Gateway。
- C2 增加可独立运行的 C++ Kafka shadow：`shadow` 命令在 canonical golden 校验后启动 consumer worker 与动态健康面，只有实际 partition assignment 且最近一次 evidence-before-commit 链健康时 ready；broker 不可达时 live 保持 200、ready 返回 503，SIGTERM 有界退出。`sync_fanout=false` 可在无 Redis 阶段选择热群通知，进程仍不写 Gateway/客户端且未进入生产 Compose。
- C2 增加 Ubuntu 24.04 多阶段 Realtime Delivery 镜像：builder 显式安装 C++/Protobuf/nlohmann-json/librdkafka 并运行全部 CTest，runtime 只保留二进制、共享库与 Delivery golden contracts，同时写入 OCI revision/created/dirty 标签；镜像尚未加入生产 Compose。
- C2 增加 C++ Kafka shadow 消费边界：librdkafka C API 强制独立 `dipole-realtime-shadow-*` group、earliest、手动 offset 与 round-robin assignment；runner 仅在低敏 NDJSON evidence 刷盘后同步提交 offset，poison event 记录固定类别，evidence/commit/poll 失败撤销 readiness。
- C2 增加 C++ Kafka message-created 纯投影层：严格解码 v1/minor-additive envelope，将 direct、普通群、热群、Timeline shadow 和文件消息映射为 canonical `DeliveryEnvelope`，固定稳定 batch/delivery ID、Kafka source coordinates、用户 ordering key 与 Go WebSocket payload 形状；兼容缺少 mutation/revision/actor/Seq 的 legacy created 完整消息，重复 recipient、channel/target 漂移和未知 major version fail closed。当前 executable 仍为 `contract_only`，未连接 broker、Redis、Gateway 或客户端。
- C2 增加首个独立 C++20 Realtime Delivery foundation：CMake 在 build 目录从 canonical `dipole.delivery.v1` 生成 C++ Protobuf 类型，手写 validator 与 Go 共用三组 golden vectors，并以 `contract_only` CLI/进程验证 envelope、节点批次和背压 ACK。进程启动前完成契约校验，只暴露 `/livez`、`/readyz`、`/health`，host/port/mode 非法时 fail closed；当前未消费 Kafka、查询 Redis、写客户端或进入 Compose。统一门禁使用系统 GCC 13、Protobuf 3.21、`-Werror`、clang-tidy 和 CTest。
- C2 建立语言无关的 `dipole.delivery.v1` 实时投递契约：`DeliveryEnvelope` 固定 Kafka source coordinates 与用户级投递项，`NodeDeliveryBatch` 固定 Presence 解析后的节点/connection 批次，逐项 ACK 覆盖 enqueued、offline、backpressured、rejected 和 failed，并用饱和队列 retry hint 表达背压。三个 Protobuf JSON golden vectors 与 Go fail-closed validator 约束枚举、时间戳、批次上限、ID 唯一性和 ACK 一致性；legacy adapter 可映射现有 Go Hub 返回值但暂不接管流量，默认仍为 Go Delivery。
- C1 增加隔离且可回滚的 Go 候选基准拓扑：`docker-compose.dist.yml` 在保持旧默认值的同时支持 image、容器前缀、宿主端口和网段覆盖；`candidate_topology.sh` 只接受与干净工作树同 revision 的镜像，固定 image SHA、关闭 embedded Agent，并依次等待基础设施、执行 MinIO 初始化和 one-shot migration 后启动独立 project，`down` 保留候选卷。迁移编排不进入基础 Compose 服务依赖，既有共享拓扑继续支持 `compose start`。canonical Compose gate 同时验证默认和候选渲染。
- C1 归档同一候选镜像下 20/50/100 并发连接、每连接 2 条消息的 Go 实时数据面梯度证据：三档接收、持久化和投递率均为 100%，Kafka lag 最终归零；吞吐从 2.51 增至 4.85 msg/s 的同时 P95 从 1.07s 增至 8.08s，提示下一步需用故障恢复和分段剖析定位等待/串行化路径。
- C1 增加版本化单节点 stop/start 恢复门禁：恢复 evidence 绑定前后 container/image/revision/PID、单调健康时间线和故障前稳定 consumer group 成员数，要求确实观察到 unavailable、新 PID，并在恢复到相同成员数且连续稳定 5 秒后才运行负载；恢复后复用 baseline v4 验证消息接收、持久化、投递和 Kafka lag，并以精确 baseline SHA-256 生成 recovery report。EXIT trap 在演练异常时尝试恢复目标节点，稳态资源门禁继续拒绝意外 PID 漂移。
- C1 基准健康检查支持自定义节点 URL，operations/baseline v4 归档无凭据的 API 与 WebSocket 实际端点，避免隔离端口与报告环境描述漂移。
- C1 为 Go 实时数据面基准增加运行镜像来源门禁：Docker 镜像写入 OCI revision/created 与 source dirty 标签，`run_bench.sh` 从运行容器解析不可变 container/image ID，并要求被测服务、采集器和干净源码树绑定同一完整 Git revision；operations/baseline v4 归档逐服务来源证据，缺标签、dirty 构建、提交偏差或重复容器绑定均在负载启动前 fail closed。
- C1 增加 Go 实时数据面资源基准采集：`process_metrics.py` 从固定服务 PID 的 `/proc` 多点采样 CPU、RSS、线程与 context switch，`run_bench.sh` 将结果绑定到 operations/baseline v4；进程重启、服务集合漂移和计数异常 fail closed，v1-v3 历史报告继续可读并显式标记资源证据不可用。
- Agent G4 增加 Event Subscription rollout evidence gate：CLI 同时读取 corpus、双评审 review 和 candidate evidence，重新执行 review/prefilter evaluator 后才产出低敏 `eligible|blocked` 决策，避免信任调用方预聚合报告。决策绑定 corpus、review、final-label、candidate evidence/configuration 哈希与 agreement、precision/recall、p95、成本指标；任一门槛失败返回 2，哈希/结构/逐 case 绑定无效返回 1。该门禁不修改 Trigger/Runtime mode 或 Capability authority。
- Agent G4 增加 Event Subscription corpus review v1：语言中立 review/report schema 将两个独立 reviewer 的完整逐 case 标签绑定到 prefilter corpus SHA-256；有分歧时要求第三个独立 adjudicator 精确裁决全部分歧 case。纯离线 evaluator 对身份复用、case 覆盖、裁决集合和最终 corpus 标签执行 fail-closed 校验，CLI 以 `0/2/1` 区分达标、未达标和无效输入。低敏报告只含 review/final-label 哈希、agreement bps、计数和异常 case ID，不回显消息正文或 reviewer 身份。
- Agent G4 增加 provider-neutral Event Subscription prefilter Eval v1：语言中立 corpus/evidence/report schema 将受控事件标签与 `rule|embedding|small_model` 候选决策分离，绑定 corpus、strategy revision 和 configuration SHA-256。纯 TypeScript evaluator 不访问模型、数据库或网络，输出低敏混淆矩阵、保守整数 precision/recall bps、nearest-rank p95、微美元平均/总成本及误判 case ID；缺失/重复 case、hash 漂移、分数/阈值决策漂移和单类 corpus fail closed。首个 rule adapter 直接复用生产 matcher，CLI 使用 `0/2/1` 区分达标、未达标和无效证据；生产仍固定 `direct_target`。
- Agent G4 增加 Gateway-only Event Subscription 管理控制面：migration v34 为 v28 订阅补充 creator/revoker、撤销原因与更新时间，并从绑定的不可变 Definition owner 回填历史审计。认证 `dipole-gateway` 从 RequestContext 派生 owner，可创建、分页查看和撤销精确 Definition version 订阅；Core 重新校验 tenant、Agent、有效期、`conversation.read` 与 resource scope。`message_contains_any` 以 trim/lowercase/去重/排序生成规范 JSON 和稳定 SHA-256 Subscription ID，等价创建及同原因撤销可安全重放，漂移返回冲突。公开 list/create/revoke 已进入默认关闭的 HTTP/Pencil/Vue 页面；语义预筛和生产触发切换仍未启用。
- Agent G4 增加 Gateway-only 晋升证据 review projection：已获 tenant-scoped promotion operator 权限的 reviewer 可按 Proposal ID 读取其精确绑定的 `promotion_evaluation` Artifact 元数据和正文；应用层先复用控制面 `Get` 授权，再校验 Artifact ID/type/version/media/content SHA-256 并从专用对象存储复算正文证据。未授权调用不会触达 Artifact reader，普通 Task-principal 下载权限不变，专用存储未装配时 RPC unavailable；当前未暴露公共 HTTP、active authority 或 write Tool。
- Agent G4 补齐真实 Shadow 晋升证据发布链：`promotion:publish` 只接受语言中立 v1 publication 输入，重新解析完整 promotion v2 证据并要求决策为 `eligible`，随后以 canonical JSON 创建 completed Shadow Run 绑定的不可变 `promotion_evaluation` Artifact。Core 对该终态例外严格校验固定类型/版本/媒体类型、pinned Definition、candidate、Suite SHA-256、正文 envelope 与 metadata 一致性；普通 Artifact 继续仅允许 running Shadow Run。CLI 只输出 Artifact/content/Suite hash 绑定的低敏收据，不自动创建 Proposal、Grant、active Run 或 write Tool。
- Agent G4 增加默认关闭的 approved Capability projection：Core 只在 durable promotion authorizer 已通过的 active admission/context resolve 后，从 pinned Definition 的 `message.write` 与 conversation/write scope 投影显式 allowlist 中唯一的 `message.system.send`；shadow、未知 ID、重复项及 Registry 后增 Tool 均无法扩权。Go RPC 出口与 TS ExecutionContext 双重校验，TS write Policy 仍要求同一 Capability，后续还需精确 Approval consumption、Tool audit 和 Message Command。生产 Bootstrap 未注入 authorizer，Runtime 入口未注册 write projection/executor。
- Agent G4 增加默认关闭的 Runtime promotion control plane：migration v33 与 sqlc 建立 tenant-scoped operator Grant、不可变 Proposal、唯一 Review 和追加式 Revocation 审计。提案必须绑定 `promotion_evaluation` Artifact 的内容哈希及 metadata 中的 Runtime candidate、pinned Definition、Eval Suite SHA-256，并与权威 Task/Run 逐项一致；不同 reviewer 审批时在同一 MySQL 事务生成 durable Grant，授权 revoker 在同一事务追加审计并撤销。四个 additive gRPC 仅接受认证 `dipole-gateway` 并从可信 RequestContext 派生 operator。生产 Bootstrap 只装配控制面审计，仍未向 admission/resolver 注入 active authorizer，也未注册 write Tool。
- Agent G4 增加 durable Runtime promotion grant：migration v32 与 sqlc 持久绑定 tenant、Runtime candidate、pinned Definition、promotion v2、evidence SHA-256 和完整离线 Eval Suite SHA-256，并要求不同 grantor/reviewer 双人签署、生效窗口与可撤销状态。active Run 保存 candidate version；admission 和每次 persistent MCP context 解析都会重新校验 grant，撤销后已有 active Run 立即失去 context authority。生产 Bootstrap 未注入 authorizer，也没有 grant 签发 API、active admission 或 write Tool 注册。
- Agent G4 建立 fail-closed active ExecutionContext promotion seam：`PersistentAgentRunAdmission` 只有在注入的 promotion authorizer 对 Runtime candidate、Task 和 pinned Definition 精确授权后才允许创建 active Run；缺少 authorizer、candidate version 或授权拒绝均不会创建 Task/Run。Core MCP context 现在返回持久 Run 的权威 `runtime_id/mode`，Go Server 与 TS Client 双重拒绝伪造 Runtime 或非法 mode。生产 Bootstrap 未注入 authorizer，公开 admission 继续固定 shadow，write Registry/executor 仍关闭。
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

- Eino 从 `v0.9.15` 升级至 `v0.9.17`；保持 OpenAI 扩展 `v0.1.13` 不变。Go 全量测试、sqlc drift、Agent Runtime 测试、typecheck 与 build 均通过，未发现 API 兼容性回归。
- 更新正式技术架构图以匹配当前实现：补充 Core/Message/Gateway/Sync/Search/Agent Runtime 边界、`user_sync_inbox` Sync Timeline、sqlc 数据访问及 Cassandra/Elasticsearch 影子投影；移除 `AutoMigrate`、单体无 Inbox 和旧 Eino 主链路等过时描述。该图仅记录当前已实现或明确默认关闭的能力，不改变运行配置。

- Core RPC Agent 方法 allowlist 补齐 readiness evidence publish/resolve；此前真实 RPC 部署会在 MCP egress freshness 查询时返回 `PermissionDenied`。全栈演练中的 subscription、Run、Workflow projection、MCP Invocation/Round、readiness 和 Artifact 均改由正式 TS `AgentCapabilityRPCClient` 访问隔离 Go mTLS fixture。
- 外部 MCP 全栈演练脚本不再内嵌两字段 JSON 判断，统一调用 Runtime 契约校验 CLI；证据创建和离线复核共享相同 canonical hash、时间与成功不变量。
- 外部 MCP fresh-readiness egress gate 现在除双 binding、hash 和时间结构外，还要求 `expiresAt` 严格晚于 Worker 当前时钟；Worker 复用已有可注入时钟完成每次连接前校验，过期证据在 raw Registry、Catalog 和网络访问前拒绝。
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

- 修复 `realtime-cpp` 镜像构建阶段遗漏 authority fence 测试契约目录的问题，确保 CMake/CTest 在独立 builder 中读取完整 golden vectors。
- 修复 Agent TypeScript Proto 生成物滞后：重新生成 `CorrectOwnedMemory` RPC、Memory root/version/correction 字段及后续方法索引，使 TS 客户端与已发布的 additive Go Proto 定义恢复一致；未改变 Proto schema 或 Runtime 开关。
- 修复 C1 Kafka lag 采样将无 committed offset 的分区静默当作零积压：独立解析器对 `current-offset=-` 且存在 log end 的行保守计入 retained backlog，并在找不到目标 consumer group 或字段不可解析时 fail closed。真实 node2 首轮恢复演练因此识别出 HTTP 健康早于 72-member consumer group 稳定、`LastOffset` 跳过 40 条 send-requested 的窗口，失败报告未被接受。
- 修复 canonical Go gate 在干净 checkout 中隐式依赖被忽略的 `configs/config.yaml`：配置加载器支持显式 `DIPOLE_CONFIG_FILE`，`scripts/check-go.sh` 默认使用跟踪的 `configs/config.dist.yaml`，调用方仍可覆盖；未设置环境变量的生产/本地启动继续沿用原有 `config.yaml` 搜索行为。
- 修复分布式与微服务 Compose 在干净 checkout 中强制依赖未跟踪 `.env`：本地 env file 改为 optional，关键内部 RPC secret 的 `${VAR:?}` 校验保持不变；新增 `scripts/check-compose.sh` 对全部 Compose 文件执行统一静态解析。
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

- v42 把 `agent_memory_task_lineage.task_uuid` 外键从 `agent_shadow_plans` 改为 `agent_tasks`，并增加 `context_pre_model` 来源。发布顺序为 migration、Runtime writer、模型流量；旧 Runtime 的 Plan-time `runtime_write` 继续兼容。Down 会删除尚无 Plan 的 pre-model lineage，并把其余来源归一为 `runtime_write` 后恢复 v41 Plan 外键，因此回滚前必须保留低敏影响报告并停止模型 admission。
- v41 新增 `agent_memory_task_lineage`，以 Memory/Task 主键和 Shadow Plan 外键保存 direct Context reference；发布顺序为 migration、sqlc/TS query、Agent Runtime Shadow writer，旧 Runtime 可继续写 Plan 但会被审计标记为历史未索引。回滚前停止新 Runtime 写入；Down 只删除 direct-reference 索引，不修改 Memory、Plan、Step 或其他派生数据。该迁移不执行历史回填，也不启用删除 Worker、公开 API 或生产 Runtime。
- v40 为 `agent_memories` 增加内容擦除审计列与固定 tombstone 约束。Down migration 只删除 v40 列和约束，已被擦除的正文、来源及自由文本不会恢复；回滚仍保留 revoked 状态和 v39 纠正链，因此执行擦除前必须按隐私策略确认不可逆影响。
- v39 在 `agent_memories` 上追加 lineage 与 correction 审计列，并把历史记录回填为 `root=self/version=1`。发布顺序为 migration/sqlc 与 Core transaction、additive gRPC/Gateway、最后显式开启 `VITE_AGENT_MEMORY_CORRECTION_ENABLED`；回滚前先关闭前端入口，Down 会删除纠正 lineage 字段，执行前需确认是否保留已产生的版本链审计。
- migration v38 为 `agent_memories` 增加 `revoked_by_uuid` 与 `revoke_reason`，并约束 active 记录审计字段为空、revoked 记录同时具备时间、revoker 和原因。历史 revoked 记录以原 principal 和固定 `legacy internal revocation` 回填。发布顺序为 Core migration/sqlc 与 additive gRPC、Gateway、最后显式开启 `gateway.agent_memory_enabled` 和前端 `VITE_AGENT_MEMORIES_ENABLED`；回滚先关闭两个入口，Down 会移除新增撤销审计列，需先确认审计保留要求。
- Agent Subscription owner create 使用 additive `ListEligibleSubscriptionConversations` RPC，无数据库迁移。先确认 Core 已应用 migration v34，再滚动 Core/Go Proto、Gateway/Web 和 TS 生成客户端；旧 Runtime 不调用该 RPC。`gateway.agent_subscription_enabled=false` 与 `VITE_AGENT_SUBSCRIPTIONS_ENABLED=false` 仍需同时显式启用。回滚先关闭前端入口，再关闭 Gateway adapter；该流程不修改 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`。
- `PublishMcpReadinessEvidence` 是 additive Agent Capability RPC，依赖 migration v37。先迁移并滚动 Core，再发布 TS Runtime；旧 Runtime 不调用该方法。回滚时先确保没有证据 Publisher 调用，再回退 Runtime/Core；当前没有 startup scheduler 或 admission consumer，部署后不会自动建立外部连接或激活 Profile。
- migration v37 新增 `agent_mcp_readiness_evidence` 追加式控制面表及 tenant/Profile/Runtime binding freshness 索引。先迁移 Core，再发布 additive Publisher RPC client；当前没有自动采集或 admission 启动接线。回滚前停止证据写入并完成审计留存，v37 Down 会删除全部 readiness evidence 历史，不影响 Agent Task、Run、Artifact 或 Runtime promotion 表。
- `ResolveMcpToolCommandResponse.status` 是 additive 字段，无数据库迁移。先滚动 Core，再滚动 Agent Runtime；旧 Runtime 忽略未知字段。回滚 Runtime 后可继续由新 Core 服务旧调用方；回滚 Core 前须确保新 Runtime 不再执行 terminal receipt recovery，否则空状态会 fail closed。生产 Worker 与外部网络开关仍保持关闭。

- migration v34 为 `agent_event_subscriptions` 增加 owner/revocation 审计列和一致性约束。先迁移 Core，再滚动发布 additive Go/TS Proto；迁移会从固定 Definition version 回填 creator，历史 revoked 行使用 `legacy_v28_migration` 标明来源。回滚前停止新的管理 RPC 写入；v34 Down 只删除新增审计列，保留 v28 订阅与 Task 绑定。生产 `subscription` 模式仍需完成 AD-034 的前端管理与语义基线门禁。
- 数据库新增 migration v33：创建 `agent_runtime_promotion_operator_grants`、`agent_runtime_promotion_proposals`、`agent_runtime_promotion_reviews` 和 `agent_runtime_promotion_revocations`。升级不会自动创建 operator Grant；部署者需通过受控运维流程预置 tenant-scoped proposer/reviewer/revoker，且应保持职责分离。回滚 v33 会删除控制面提案、复核和撤销审计表，不影响 v32 durable Grant 表，但应先停用控制 API 并归档审计证据。

- 新增 MySQL migration v32：创建 `agent_runtime_promotion_grants`，并为 `agent_runs` 增加可空 `candidate_version`。现有 embedded/shadow Run 无需回填；历史 active Run 缺少 candidate 或 production authorizer 时会 fail closed，回滚会先移除 Run candidate 字段再删除 grant 表。
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
- 外部 MCP Profile foundation 与 Transport Factory 没有数据迁移，Compose 固定 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=false`。现阶段不要在部署环境开启该变量；Runtime 会在发现启用配置后拒绝启动，直到后续里程碑注入加密 Secret Provider、真实 DNS Resolver 与 TLS pinned Dispatcher。Profile JSON 禁止保存 Token、密码、私钥或 CA 正文。
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

- Agent Workflow repair v44 repository 合同测试在隔离 MySQL 8.4.8 中验证 `prepared` execution 创建、精确重放和同 plan 目标哈希漂移拒绝；当前测试不推进状态，也不修改 Workflow projection。
- MySQL migration integration baseline 更新至 v44，并覆盖 v44 execution ledger、v43 lineage backfill、v42 pre-model lineage 的连续回滚与表数量断言；避免新迁移已发布但集成测试仍停留在 v42 的验证盲区。
- 发布级隔离门禁通过：`scripts/check-compose.sh` 校验全部根目录 Compose 与 Agent Shadow 配置；`scripts/smoke-microservices.sh` 使用独立项目启动并清理 Core、Message、Sync、Gateway、Agent、MySQL、Kafka、Redis 和 MinIO，验证 readiness/metrics、mTLS、Gateway 认证代理与远程 WS ownership。Agent 镜像在隔离构建中通过 `npm ci`、构建和零漏洞审计。
- 发布级回归重新通过：Go 全量包、Agent Runtime `580 passed / 26 skipped`、前端 `85 passed` 与生产构建、sqlc 漂移和架构文档门禁均通过；修正 Agent Runtime README 对 Subscription 管理 API 的过时描述。Vitest 使用项目原生命令运行，未使用不兼容的 Jest `--runInBand` 参数。

- Agent Memory lineage backfill CLI 通过默认 dry-run、审批/manifest hash 绑定、owner 身份匹配和输入大小边界测试；隔离 MySQL 8.4 通过 v43 migration up/down/reapply、失败后恢复、owner 隔离和精确重放验证。
- Backfill execute approval 收紧为独立 operator/approver 双身份，二者必须不同并共同绑定 job、manifest hash 与 source high-water；CLI 不提供自审批路径。
- 增加只读 `agent-memory-lineage-rollout-review` CLI，验证 v43、runtime/config SHA-256、未来维护窗口、回滚/备份检查和双人审批；eligible receipt 固定 `executionAuthority=false`，不会触发 backfill。
- 增加 `scripts/agent-memory-lineage-rollout-input.sh`，只读采集 OCI revision/dirty label、配置 SHA-256、manifest/approval 绑定和人工维护窗口参数；支持现有 40 位或 64 位 revision，缺失或 dirty provenance 直接 fail closed。

- Agent Memory lineage backfill runner/contract 测试通过 `7/7`，覆盖固定 manifest/receipt hash、unknown field、authority/counter drift、resume、duplicate convergence、目标失败不推进 checkpoint、非法引用和配置边界；JSON Schema examples 通过 Draft 2020-12 校验。v43 migration、sqlc 生成物和隔离 MySQL 8.4 已验证固定 high-water、跨 owner 引用阻断、失败后恢复与精确重放幂等。
- Agent Memory 派生 retention policy 聚焦测试通过 `6/6`，与 lineage 回归合计 `11/11`；覆盖完整/缺失 lineage、受影响与零影响人工复核域、policy/report/decision hash 漂移、authority 提升、64 KiB 双输入和低敏失败。完整 Agent Runtime 为 `586 passed / 26 expected skipped`；TypeScript typecheck/build、Draft 2020-12 strict Schema 示例、canonical Go test/vet、sqlc、Go/TS Proto、Compose、架构文档、观测规则和生产依赖零漏洞门禁通过。
- Agent Memory pre-model lineage 通过 Planner 顺序/失败单测与真实 MySQL 20/20 contract：Context 来源优先、Plan repair 不降级、未知 Task 原子拒绝、foreign owner 隔离、无 Plan 模型结果先 fail closed 再由 lineage 恢复 root attribution；v1→v42 与 v42→v41 回滚删除无 Plan 行均通过。完整 Agent Runtime 为 580 passed / 26 expected skipped，Go test/vet、typecheck/build 与生产依赖零漏洞门禁通过。
- Agent Memory 派生血缘测试覆盖 Context 引用排序/去重、非法 ID、representation 冲突、报告 hash/不变量、CLI 脱敏、并发 Plan 重放、历史缺口、下划线相似 ID 与无 Plan 模型结果；真实 MySQL 8.4 通过 12/12 Runtime contract，并验证 migration v1→v41、48 张表及 v41/v40/v39 分步回滚。完整 Agent Runtime 为 579 passed / 24 expected skipped，Go test/vet、Go/TS Proto、sqlc、Compose、架构文档、观测规则、typecheck/build 与生产依赖零漏洞门禁通过。
- Agent Memory privacy retention 通过领域/Core/sqlc 聚焦测试和真实 MySQL 8.4 contract：两版本 root 全量 tombstone、原字段清除、Context 零召回、越权拒绝、精确重放，以及 v40/v39 分步回滚均通过。
- Agent Memory correction 通过应用/事务/传输/Gateway/Vue 聚焦测试、canonical Go 全仓测试、前端 85 项 Vitest、工具链 3 项与生产构建；真实 MySQL 8.4 完整 migration v1→v39、v39→v38→后续逐级回滚及 owner repository 并发精确重放通过。真实测试同时修复显式 migration 目标版本、SQL NULL Tool arguments 与 MySQL JSON compact 的兼容基线。
- Agent Memory owner 治理测试覆盖稳定分页、并发精确撤销重放、不同原因冲突、owner/tenant 隔离、Gateway 服务身份、客户端 principal 注入拒绝、公开 URI 省略、严格前端响应解析和 authoritative row replacement；Vue 21 个文件共 83 项 Vitest、生产构建及 Chromium、Firefox、WebKit 共 6 项 desktop/mobile E2E 通过。真实 MySQL 8.4 验证 migration v1→v38、v38→v37 回滚、生命周期 CHECK、历史审计回填和 owner sqlc contract；canonical Go test/vet、完整 Agent Runtime 566 项通过且 22 项按环境预期跳过，双端构建与官方 npm audit 零高危漏洞。
- Agent Subscription Shadow Collector 与 evidence 聚焦测试通过 `13/13`，完整 Runtime 通过 `566 passed / 22 expected skipped`；覆盖固定查询/时间、单 series、持续启用、URL 凭据、缺失/多值/非整数/error envelope、CLI 低敏失败、三类严格 Schema 及 Collector-to-evidence 兼容。typecheck/build、官方 npm 源 `0 vulnerabilities`、canonical Go、TS/Go Proto、sqlc、Compose、架构文档和 Agent/服务观测门禁通过。
- Agent Subscription Shadow evidence 合同通过聚焦 `7/7` 与完整 Runtime `560 passed / 22 expected skipped`，覆盖 CLI create/verify、Schema 字段对照、部分窗口、低覆盖、reset、matcher error、低样本、counter 回退、过期和 canonical hash 篡改；typecheck/build、官方 npm 源零高危审计、TS/Go Proto、sqlc、Compose、架构文档和 Agent/服务观测门禁通过。
- Agent Subscription Shadow observation 通过聚焦 `15/15`、完整 Runtime `553 passed / 22 expected skipped`、typecheck/build 和官方 npm 源零高危审计；Prometheus 五条 Agent 规则及测试、Compose、服务观测与架构文档门禁通过。测试固定 matcher match/miss/error、direct-target accepted/ignored、EventLedger 单一路径和默认关闭指标面。
- Agent Subscription create 权限链通过 canonical Go test/vet、Agent Runtime `549 passed / 22 expected skipped`、前端 `78/78`、工具链 `3/3`、两端生产构建与官方 npm 源零高危审计；Go/TS Proto、sqlc、Compose、架构文档和全部维护中的观测规则门禁通过。Chromium、Firefox、WebKit 完整矩阵为 `34 passed / 8 expected skipped`，覆盖认证 Definition/options/create、精确瘦请求、权威响应、撤销与不可用/mobile 状态；WebKit 使用临时用户态兼容库，未修改系统或仓库配置。
- Agent Subscription owner list/revoke 通过 Gateway/config/bootstrap 聚焦 Go 测试、5 项 Vue API/组件测试和 Chromium/Firefox/WebKit 共 6 项路由 E2E；WebKit 在本机通过临时用户态兼容库运行，未修改系统包或仓库配置。服务端覆盖认证派生、游标/limit、撤销原因、nil/错误映射与默认关闭，浏览器覆盖 Bearer、精确撤销、不可用状态清理和 390x844 mobile sheet。
- Go Core mTLS fixture 认证测试通过：正确 Agent 身份可调用，错误 shared secret 返回 unauthenticated，证书 CN 与 caller 不一致返回 permission denied，无客户端证书无法建立可用 RPC；联合演练随后通过真实 mTLS 完成全部 12 类 Core RPC，并输出 v2 低敏证据。
- 外部 MCP 证据契约聚焦测试通过 7 项，并由真实隔离演练生成包含 `collected_at`、`expires_at` 与 `content_sha256` 的文件后经独立 CLI 验证；篡改计数/布尔值/hash、附加字段、未来时间和过期时间均失败关闭。
- 外部 MCP 全栈演练通过：2 个 subscription 事件经 Kafka、MySQL EventLedger 与 Temporal 收敛；首个事件完成 1 次 allowlisted read Tool 和 1 个 Artifact，同事件在 Runtime 重启后由持久 ledger 抑制，第二个事件使用过期 readiness 后 Workflow failed 且 Tool 调用仍为 1。证据固定 `production_authority=false`，共享 Docker 服务未触达。
- C2 C++ shadow 在 Kafka 3.9/librdkafka 2.3.0 上完成首次真实 earliest replay：205 条合法 group event、1 条 poison event 均在 evidence 后提交，最终 lag=0；双实例分担 12 个 partition，停止一例后另一例完成接管并保持 ready=200。归档明确 direct topic 当时为空，尚未证明 direct broker 样本、节点路由或性能收益。
- C1 node2 恢复演练保留一组 fail-closed 与一组 passing 证据：首轮 `f657100` 在 HTTP 恢复后立即负载，40 条 accepted 消息均未持久化，暴露 consumer group 尚未稳定及旧 lag 零值误判；修复后 `ce4b600` 在 fresh project 中验证 PID `887973→898410`、72 members 前后稳定、完整 readiness 13.53s，恢复后 40/40 持久化与投递、峰值 lag 4、settled lag 0。原始证据和 SHA-256 清单归档于 `benchmarks/c1-go-recovery-2026-08-28/`。
- Subscription rollout gate 测试覆盖源证据重算、同 corpus 绑定、review/candidate 独立阻断、hash 漂移、CLI 三态退出码和低敏输出。synthetic 三件套得到 `eligible`，绑定 candidate evidence SHA-256 `2809bbcc5318cb41af6b86f09625abf9ccf05b0f178507459c47ef2e2afbbae3`；完整 Agent Runtime 为 291 passed / 19 expected skipped，该结果只验证 Harness。
- Subscription corpus review 测试覆盖双 reviewer 身份/Review ID 分离、完整 case 绑定、第三方精确裁决、最终标签漂移、review 顺序规范化、黄金哈希、CLI 退出码与正文/身份不回显。synthetic 示例达到 10000 bps agreement；完整 Agent Runtime 为 286 passed / 19 expected skipped，真实 Project Guardian review 仍待受控归档。
- Subscription prefilter Eval 测试覆盖 TP/TN/FP/FN、保守 basis-point 舍入、p95/成本门槛、候选 score/threshold 一致性、case 完整/唯一绑定、corpus hash 漂移、生产规则 matcher 复用、CLI 参数/退出码与正文不回显。synthetic 示例规则基线达到 10000 bps precision/recall、零成本，完整 Agent Runtime 为 280 passed / 19 expected skipped。
- Agent Event Subscription 控制测试覆盖大小写无关集合规范化、稳定 ID、等价创建重放、owner/tenant/Definition/scope 越权、分页隔离、撤销审计与冲突重放、Gateway/Agent 服务身份分离；真实 MySQL 8.4 验证 migration v1→v34、v33→v34 历史回填、sqlc owner 查询、创建/revoke CAS 和显式回到 v27 后重建。
- Runtime promotion 测试覆盖 v2 policy、SHA-256、双人签署、candidate/Definition 漂移、有效期、撤销、冲突重放、active admission 与逐次 context 重查；真实 MySQL 8.4 验证 sqlc grant contract，以及完整 migration v1→v32→v1 和全部历史回填集成测试。
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

- 历史 lineage backfill 已具备 v43 MySQL checkpoint、sqlc source/target、owner-scoped adapter、隔离数据库测试和默认 dry-run 的审批 CLI；当前仍缺少共享环境执行审批记录与 production rollout 证据，不能在共享环境执行回填。sqlc v1.31.1 不支持 inline JSON_TABLE，当前采用 sqlc manifest retrieval、Go 严格展开和 owner-scoped sqlc lookup。
- Memory root 派生影响已具备逐域离线 retention policy 决策，但尚未实现 Shadow plan、Step、Artifact、Agent Message 或 Temporal history 的字段级擦除器；v42 已为受管 Model planner 建立模型前 root attribution，历史/旁路缺口继续由 owner-scoped 未归因计数阻断。公开 owner 擦除 API、自动 retention Worker 与账号级隐私删除继续关闭并由 `AD-035` 跟踪。
- Memory v1 已提供默认关闭的 owner list/revoke HTTP/Pencil/Vue 闭环和追加式撤销审计；自动写入、append-only 纠正/版本冲突、Observation/Reflection Worker、置信度策略及 hybrid/vector retrieval 仍待完成。共享 Shadow 仅在已有受控记录时读取，详见 `AD-035`。
- Event Subscription 已具备默认关闭的公开 Definition 目录、authenticated conversation chooser、owner list/create/revoke HTTP/Pencil/Vue 闭环、撤销审计、provider-neutral 离线预筛 Eval 和双评审 agreement 合同；尚未归档真实 Project Guardian corpus/review report、embedding/小模型 candidate evidence或 subscription Runtime 灰度证据。共享环境继续固定 `direct_target`，详见 `AD-034`。
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
