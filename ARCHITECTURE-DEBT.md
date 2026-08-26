# 架构债务台账

本文档记录已确认但暂缓处理的架构风险、兼容性缺口和可清理冗余，便于后续按优先级滚动治理。

## 维护约定

- 状态使用：`暂缓`、`处理中`、`已解决`、`接受风险`。
- 优先级使用：`P0` 阻断发布、`P1` 应在正式启用相关能力前解决、`P2` 进入后续迭代、`P3` 按需清理。
- 新问题使用连续编号 `AD-NNN`，保留历史编号，不复用已关闭条目。
- 开始处理时补充负责人或关联 Issue/PR；解决后记录提交、验证方式和完成日期。
- 本台账描述风险和演进方向，不代表当前迭代立即修改对应实现。

## 待处理

### AD-019：MySQL 消息正文退役缺少完整替代读契约

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Cassandra 主读、Sync Timeline、消息幂等、文件授权、搜索重建、迁移回放
- **现状：** `user_sync_inbox` 已持久化并对外暴露 `conversation_key + message_uuid + message_seq` locator。Sync Service 已建立 storage-neutral hydrator，可在返回 MySQL 正文的同时异步比较 Cassandra Timeline；Cassandra 尚未承担 Sync 主读。Web 已增加默认关闭的 IndexedDB Sync Engine、shadow 门禁和热群持久 ACK。migration v12 增加无正文 `message_metadata`，与 Message/Inbox/Outbox 原子提交并回填历史 locator；文件授权已改查 Metadata，删除完整 Message 行后仍可验证访问和过期时间。重复发送先通过 Metadata 校验身份，并可在默认关闭的开关下按会话 Seq 从 Cassandra 恢复原响应，缺失/冲突继续回退 MySQL。Cassandra Backfill/Reconciler 已支持经 SHA-256 校验的不可变完整消息归档，Job 绑定 source identity；真实演练删除 MySQL 正文后仍可恢复和全量对账。Message 最小账号暂时保留 `groups/group_members` 只读权限用于旧 Offline 与群文件授权。
- **风险：** 提前停止正文写入仍会让多端同步和重复发送响应缺失正文，并丢失 Cassandra 修复与回滚基准。文件授权的正文依赖已解除，但群文件授权仍需 Core 成员关系。
- **建议方向：** A5 Search 与 A4 Cassandra 均已具备不可变归档恢复源；重复发送 hydration 已具备严格 24 小时晋级规则，仍需真实流量证据。A6 继续完成 Web 观察窗口、Sync Cassandra hydration 主读/fallback 和重复发送 hydration 灰度，再引入 `full / metadata_only` 写模式。
- **处理门槛：** 完成固定快照备份与校验、事件回放演练、Sync/Offline 比较、幂等和文件授权契约、至少一个兼容窗口的 Cassandra 稳定主读，并记录可执行回滚期限与责任人；旧 Offline 退役后撤销 Message 对 `groups/group_members` 的临时读取。

### AD-021：Search 重建依赖 Outbox 事件保留契约

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **影响范围：** Elasticsearch 全量重建、事件归档、Outbox 清理、MySQL 消息正文退役
- **现状：** `dipole-search-archive` 可按固定 Outbox mutation 高水位流式导出最终状态 NDJSON 与 SHA-256 manifest，并发布到独立 MinIO object-lock bucket。`dipole-search-outbox-cleanup` 只接受可按精确对象版本恢复的 receipt、已完成且一致的 Reconcile 报告和匹配的 Backfill Job；默认 dry-run，执行时强制维护窗口确认与 operator。sqlc 查询仅删除水位内、已发布的八类 Search mutation，遇到未发布事件时拒绝清理。
- **解决记录：** 2026-08-27 完成专用 `search.mysql.*` 配置和最小授权模板；单测验证批次中断后可重入。真实 MySQL/MinIO/Elasticsearch 演练按 2/2/1 删除 5 条 eligible mutation，保留无关 Outbox，维护账号访问 Core 表被拒绝；随后仅凭保留对象版本从空索引恢复并完成 3/3 hash 对账、Alias 正向切换与回滚。
- **长期约束：** 禁止手工批量删除 Outbox。每次执行必须保存 operator、snapshot/object version、Reconcile 时间、高水位和删除统计；对象保留期、清理窗口或 mutation 类型变化时重新评审本条契约。

### AD-022：前端开发工具链仍停留在 Vite 5

- **优先级：** P2
- **状态：** 暂缓
- **发现日期：** 2026-08-27
- **影响范围：** 前端本地开发服务器、Vite、Vitest、esbuild、依赖审计
- **现状：** 生产依赖高危公告已修复且 `npm audit --omit=dev` 为零漏洞；完整开发依赖审计仍报告 Vite 5 间接使用旧 esbuild，自动修复要求跨主版本升级到 Vite 8，并同步升级测试链。
- **风险：** 开发服务器在不受信任网络上运行时可能暴露响应读取风险；直接强制升级会同时改变构建、插件和测试运行时，缺少独立兼容验证。
- **建议方向：** 在独立前端工具链分支升级 Vite/Vitest，验证 Node LTS、生产 bundle、代理 WebSocket、测试和静态资源基路径后再合并。
- **处理门槛：** 前端开发服务器需要暴露到共享网络前完成；当前仅绑定可信本机开发环境。

### AD-025：Web 本地消息库清理与容量策略需真实浏览器验收

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** IndexedDB Sync Engine、共享设备隐私、浏览器配额、401/强制下线
- **现状：** Sync Engine 按用户隔离消息与游标；显式退出、HTTP 401、WS kick 和账号切换进入统一 Session Termination。凭据与运行时消息先同步撤销，在途 Sync 收敛并删除账号 IndexedDB 后才跳转登录页，避免导航中止清理；并发终止复用 singleflight，快速重登等待旧清理完成。IndexedDB v2 增加逐用户 `message_count/compacted` manifest、5000/4000 默认高低水位和按会话保底淘汰；单页消息、淘汰和完整 `next_seq` 在同一事务提交，配额失败映射为 `storage_full`。Playwright 已在 Chromium、Firefox、WebKit 验证真实 IndexedDB 淘汰、重开、账号隔离、延迟清理和页面中断事务原子性；`storage_full/sync_error` 进入独立有界错误指标及告警。
- **风险：** IndexedDB 删除失败或浏览器进程强退时，共享设备仍可能保留本地消息；浏览器总配额还会受其他站点数据和实现差异影响。极端会话数超过低水位时，为满足硬上限会淘汰部分会话的最后一条本地缓存，历史仍可从服务端重新拉取。
- **建议方向：** 继续执行真实磁盘配额拒绝、完整浏览器进程强退与共享设备 401/kick 端到端验收。Chromium 151 的实验性 CDP quota override 虽报告 active，仍未拒绝 IndexedDB 写入，不能作为晋级证据；应使用受限 profile/文件系统或真实设备配额。记录浏览器清站点数据说明。
- **处理门槛：** `VITE_SYNC_ENGINE_MODE` 计划从 `off/shadow` 改为默认 `primary` 前完成真实浏览器容量压测、共享设备和进程强退验收；当前单元测试与 Pencil 交互基线只完成实现门禁。

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

### AD-016：HTTP Handler 测试并行修改 Gin 全局模式

- **优先级：** P3
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `internal/handler/http/*_test.go`、整包 race 门禁
- **现状：** 多个历史测试在并行执行期间调用 `gin.SetMode()`；普通测试稳定通过，整包 `go test -race` 会报告 Gin 全局变量读写竞争。本次 Sync Handler 定向 race 独立通过。
- **风险：** HTTP handler 整包无法作为可靠 race 门禁，真实业务竞争可能被大量模式切换告警淹没。
- **建议方向：** 在包级 `TestMain` 统一设置 Gin test mode，删除各测试内的重复全局写入，并保留 handler 并行执行。
- **处理门槛：** 下一次集中治理 HTTP 测试基础设施时完成。

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
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** 普通群 Inbox、Conversation State、热群吞吐
- **现状：** 普通群同时按成员更新 Conversation State 和 Inbox；热群仅跳过 Inbox，Conversation State 仍逐成员更新。
- **风险：** 两类投影职责独立，但成员级写入量会叠加，热群链路仍保留 `O(group_size)` 的会话状态写扩散。
- **建议方向：** 在压测数据达到瓶颈后，再评估热群会话摘要读扩散、异步批处理或分层投影，避免当前阶段过度设计。
- **处理门槛：** 以群规模和消息频率压测结果作为启动条件。

### AD-006：消息仓储保留未使用的兼容包装

- **优先级：** P3
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `MessageRepository` API
- **现状：** `Create` 和 `StoreWithOutbox` 仅转发到带 Sync 的新方法，当前生产代码没有调用。
- **风险：** 多套相近写入入口会增加维护者选择成本，并可能在未来绕过 Inbox 写入约束。
- **建议方向：** 确认无外部调用后删除包装，或收敛为一个明确的事务写入参数对象。
- **处理门槛：** 后续仓储接口整理时一并处理。

### AD-007：架构 Markdown 当前未纳入版本控制

- **优先级：** P3
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `docs/*.md`、架构决策可追溯性
- **现状：** `.gitignore` 忽略全部 `docs/*.md`，Swagger 文件例外是因为其已被 Git 跟踪；本轮详细架构说明只存在本地工作区。
- **风险：** 架构设计、约束和后续决策无法随代码提交进行审查与追溯。
- **建议方向：** 将 `docs/*.md` 改为按需忽略，或显式允许需要版本化的架构文档。
- **处理门槛：** 项目开始通过 PR 协作或需要公开架构文档时处理。

### AD-008：Agent Tool 允许模型提供用户身份参数

- **优先级：** P1
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `internal/modules/ai/tools.go`、会话读取、用户资料、系统消息发送
- **现状：** 多个 Tool Schema 暴露 `user_uuid`，执行时直接使用模型生成的参数查询资源或确定消息目标。
- **风险：** Tool 缺少由认证链注入的 principal 与统一 Capability Policy，模型参数可能造成越权读取、错误目标写入或审计身份不清。
- **建议方向：** 引入不可由模型覆盖的 `ExecutionContext`，将 principal、委托身份、权限和 trace 注入 Tool；模型只提交资源参数，Capability API 执行服务端授权。
- **处理门槛：** TypeScript Agent Runtime 获得任何生产读写流量前完成。

### AD-009：Agent 仅有调用级日志，缺少持久任务生命周期

- **优先级：** P2
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `ai_call_logs`、长任务、审批、失败恢复和评测
- **现状：** 当前记录 trigger、response、Token 和 latency，执行仍以单次 Kafka handler 和模型调用为中心。
- **风险：** 服务重启、等待用户输入或审批、Tool 重试和多步骤 Artifact 无法形成可恢复、可审计的统一状态。
- **建议方向：** 引入 AgentTask、Run、Step、ToolInvocation、Approval 和 Artifact 模型，由 Temporal Workflow 管理状态与恢复。
- **处理门槛：** 上线 Durable Task 或 Event-driven Agent 前完成。

### AD-012：用户状态常量与 schema 默认值偏移

- **优先级：** P2
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `model.User`、`users.status`、手写 SQL、跨语言状态契约
- **现状：** `DefaultAvatarURL` 与用户状态常量位于同一 `const` 块，受 `iota` 行号影响，当前 `UserStatusNormal=1`、`UserStatusDisabled=2`；baseline schema 的默认值仍为 `0`。
- **风险：** 依赖 schema 默认值或手写字面量的写入/查询可能产生领域未定义状态；多语言服务若只读取 SQL schema，会与 Go 领域值产生分歧。
- **建议方向：** 新增显式常量值与数据库约束，先审计并迁移现有 `status=0` 数据，再通过共享枚举契约和 migration 收敛。
- **处理门槛：** User Service 独立部署或其他语言直接消费用户状态前完成。

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
