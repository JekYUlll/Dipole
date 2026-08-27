# 架构债务台账

本文档记录已确认但暂缓处理的架构风险、兼容性缺口和可清理冗余，便于后续按优先级滚动治理。

## 维护约定

- 状态使用：`暂缓`、`处理中`、`已解决`、`接受风险`。
- 优先级使用：`P0` 阻断发布、`P1` 应在正式启用相关能力前解决、`P2` 进入后续迭代、`P3` 按需清理。
- 新问题使用连续编号 `AD-NNN`，保留历史编号，不复用已关闭条目。
- 开始处理时补充负责人或关联 Issue/PR；解决后记录提交、验证方式和完成日期。
- 本台账描述风险和演进方向，不代表当前迭代立即修改对应实现。

## 待处理

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
- **后续方向：** C++ Realtime Delivery 阶段评估节点级有界队列、投递 ACK 和 Kafka offset 提交边界；保留 Sync Timeline 作为最终补偿路径。
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
- **基线证据：** 候选提交 `2202f1f` 增加三节点成功 upsert Counter 与 baseline v2。20 人普通/热群分别保持 20 次 Conversation message 写/消息，100 人普通/热群分别保持 100 次；热群 Inbox 写放大均降为 0，普通群则为 20/100。四组均 100% 投递且 Kafka 最终 lag 为 0。100 人普通群 P95 为 10065 ms，热群为 1853 ms；两者 Conversation 放大相同，说明 Inbox/投递策略贡献显著，当前证据仍不能将剩余延迟单独归因于 Conversation SQL。完整证据见 `benchmarks/ad005-2026-08-27/`。
- **建议方向：** 下一步按 projection 路径记录有界数据库耗时和批次分布，区分 Conversation、Inbox 与实时投递成本；出现可重复的 Conversation 占比后，再评估热群会话摘要读扩散、异步批处理或分层投影。
- **处理门槛：** 在 100/1000 人固定 workload 下证明 Conversation projection 对数据库写延迟或端到端 P95 的独立贡献，并定义回滚语义后启动行为优化；指标和 v2 基线继续作为回归门禁。

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
- **现状：** migration v16-v26 已落地 Definition、Task、独立 Runtime Run、可重放模型输出/预算、不可变 Plan/Context manifest、带 lease 的 Step 终态、附加 Workflow projection、版本化 Artifact，以及默认空授权的 repair proposal/双人审批审计。Temporal Workflow 已持久化 Task/Run admission、三类 Run 终态与 Approval Signal；默认关闭的 `read_shadow` 由 Kafka 启动稳定 Workflow，并在 Activity 内执行 ContextCompiler、ModelRouter、只读 Capability Step 和内容寻址 Artifact 创建。Gateway 已提供默认关闭的 JWT Task query/cancel/approval API；repair 审计 RPC 只接受 Gateway principal，Agent 服务身份无权调用。离线对账输出六类版本化证据；Shadow 晋级策略要求连续 24 小时、24 个观察点、累计 100 个 Task、零异常和完整 Eval，并且只生成 eligible/blocked 决策。Compose 继续关闭 Temporal、Task 控制桥并固定 `foundation`。
- **风险：** v24 projection 保持 shadow 观察属性，尚未接管原 `agent_tasks.status`；当前 `read_shadow` 只允许 `conversation.list`，也没有 Memory、真实任务终态 outcome Eval 或 repair 审批前端。v25 的 `approved` 只保存审计结论；execution plan v1 仅允许带 CAS/回滚证据的 dry-run，执行器和 projection 修改命令保持缺席。操作员授权当前需要受控 SQL 配置。Temporal Worker 停止时 Query 会归类为 unavailable。eligible 决策不能自动切换 active。
- **基线证据：** 真实 Temporal Server 已验证 admission/Approval 历史恢复、单调 revision 投影、取消投影、完成态 Query/Describe 对账和 Activity 丢失完成 ACK 后的模型/Step 重放；真实 MySQL 8.4 已验证 v25 全链升降级、16 路同审批人重放仅一票、两位独立审批后批准，以及原 projection 并发与 shadow cohort keyset 契约。TypeScript/Go canonical evidence SHA-256 使用黄金向量对齐；gRPC 测试验证 Gateway principal 绑定和 Agent 最小权限拒绝。Kafka Shadow 与 Go/Eino 权威业务路径保持不变。
- **建议方向：** 下一步使用 Pencil 维护的 Agent Task/Approval 设计稿实现恢复界面，并设计显式、可回滚、再次授权的 repair executor；完成真实 outcome/trajectory/permission Eval 证据后才评审权威 Task 与回复流量迁移。
- **处理门槛：** 上线 Durable Task 或 Event-driven Agent 前完成。

### AD-011：前端缺少可版本化的完整设计基线

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-26
- **影响范围：** `frontend`、响应式布局、Agent UI、视觉一致性
- **现状：** 已建立 canonical `design/dipole-ui.pen`、设计日志、Search desktop/mobile 四态预览和 Vue 工作区；Sync 状态矩阵、desktop/mobile 恢复稿与 Vue 标题栏状态也已纳入同一基线。Login/完整 Chat、通用 token 到 Vue 的映射及自动视觉回归仍未完成。
- **风险：** 新增 Sync、Search、Agent Task、Approval 和 Artifact 页面时容易出现交互与视觉漂移，desktop/mobile 状态覆盖无法持续审查。
- **建议方向：** 使用 Pencil 维护 canonical `.pen`，覆盖 foundations、组件、页面与异常状态；通过设计日志、Vue token 和 Playwright 视觉回归保持同步。
- **处理门槛：** 大规模拆分或重写现有前端页面前完成 F1。

## 已关闭

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
