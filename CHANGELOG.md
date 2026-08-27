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

- TypeScript Agent Runtime 增加 `message.direct.created` v1 decoder、KafkaJS 独立 shadow consumer、稳定 Task ID 和进程内 EventLedger；冷启动 metadata 未收敛时会断开旧客户端并有界重连，微服务 Compose 可独立启动只读 Agent，真实 Kafka 3.9 重放同一事件只产生一条 metadata plan。持久多副本幂等由 `AD-028` 跟踪。
- 增加独立 `agent-runtime/` TypeScript foundation：Node 22+、Fastify 5、Zod 4、AI SDK 7、KafkaJS 2，提供 trusted ExecutionContext、Go 兼容 Task ID、Capability Registry、resource-scope Policy Engine、shadow 写隔离和 `/livez`/`/readyz`；模型路由与持久审计留待 G2 后续切片。
- Embedded Agent 增加持久执行策略：`ai.policy_mode=persistent` 默认从版本化 Definition 创建确定性 AgentTask、固定并重读精确 policy version，Invocation 携带 permission/resource scope；`static` 保留显式回滚，Task 以 compare-and-set 进入 completed/failed。
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

- Message atomic/projector 账号仅保留 sqlc 实际使用的表操作；显式拒绝 Message UPDATE/DELETE、Outbox DELETE、migration 写入、Core 表访问，以及 projector 模式的 Inbox 权限，关闭 AD-015。
- Sync 与 projector-mode Message 账号按表和操作拆分；启动时拒绝 Sync 修改 Message/Outbox/群高水位或读取 Core 数据，也拒绝 Message projector 访问 Inbox 状态。
- 更新前端生产依赖锁定版本，修复 Axios、form-data、nanoid 和 PostCSS 的高危公告；生产依赖审计恢复为零漏洞。

### 移除

- 移除 legacy GORM repositories、model persistence tags、运行时 `AutoMigrate`、SQLite 方言测试以及 `gorm.io/*` 依赖。
- 移除 `data.mysql_adapter`、`mysql.auto_migrate` 和无依赖 Repository/Server/Kafka 便捷构造入口。

### 迁移说明

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

- 前端完整开发依赖审计仍受 Vite 5/esbuild 链影响，主版本升级和兼容验证记录为 `AD-022`；生产依赖审计已通过。
- Sync Inbox、旧 Offline 与默认关闭的幂等 hydration 尚未完成替代链路观察；Cassandra 恢复工具已可独立使用不可变完整消息归档，正文退役其余条件继续由 AD-019 跟踪。
- `/messages/offline` 真实对照观察窗口仍待执行；Web 本地 Sync Engine 默认关闭，旧客户端继续使用数据库 ID cursor。
- Web IndexedDB 已统一会话清理和容量淘汰实现；真实浏览器配额、共享设备和进程强退验收仍是默认启用门禁，记录为 `AD-025`。
- Web 协议对照首批仅覆盖收到的私聊消息；群聊存在普通群 fanout 与热群 notify/pull 两套语义，需按群类型建立独立比较契约后再纳入。
- `users.status` 的 schema 默认值 `0` 与当前 Go 领域常量 `Normal=1`、`Disabled=2` 存在偏移，已记录为 AD-012。
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
