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
- **状态：** 暂缓
- **发现日期：** 2026-08-27
- **影响范围：** Cassandra 主读、Sync Timeline、消息幂等、文件授权、搜索重建、迁移回放
- **现状：** 会话 Seq 历史已支持 Cassandra 主读，但 `user_sync_inbox` 仍只保存 Message UUID，Sync Service 通过 MySQL `messages` 批量补全正文；旧 Offline API、按 UUID 查询、幂等冲突回放、文件消息访问授权、Cassandra Backfill/Reconciler 也继续读取 MySQL 完整消息。
- **风险：** 提前停止正文写入会让多端同步返回缺失消息，削弱重复发送的确定性响应和文件权限判断，并丢失 Cassandra 修复与回滚基准。仅观察会话历史主读稳定无法覆盖这些链路。
- **建议方向：** A5 提供可从 Cassandra/Kafka 重建的搜索投影；A6 让 Sync Item 携带 `conversation_key + message_seq + message_uuid` 并由 Message Store 补全，另行建立幂等结果快照或 UUID locator、独立文件授权元数据。所有替代契约双跑通过后再引入 `full / metadata_only` 写模式。
- **处理门槛：** 完成固定快照备份与校验、事件回放演练、Sync/Offline 比较、幂等和文件授权契约、至少一个兼容窗口的 Cassandra 稳定主读，并记录可执行回滚期限与责任人。

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
- **状态：** 处理中
- **发现日期：** 2026-08-26
- **影响范围：** `cmd/message-service`、File metadata、数据表所有权、最小权限
- **现状：** 用户、好友、群和文件所有权校验均通过 Core Capability gRPC 完成；独立 Runtime 只组合 Message 与 Outbox adapters。部署仍复用 Core 的 MySQL schema 与数据库账号。
- **风险：** 代码依赖已经收敛，数据库凭据仍具备访问 Core 表的能力，误用或注入风险下的 blast radius 大于 Message Service 实际职责。
- **建议方向：** 增加独立 `dipole_message` 数据库账号，仅授权 `messages`、`conversation_sequences`、`group_sync_states`、`user_sync_inbox`、`user_sync_states`、`outbox_events` 及 migration ledger 的必要读写权限，并加入启动时权限验收。
- **处理门槛：** Message Service 使用独立数据库凭据或 M4 进入正式流量前完成。

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
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** `frontend`、响应式布局、Agent UI、视觉一致性
- **现状：** 当前只有 Login 与 Chat 路由，仓库内没有 `.pen`、design token、组件状态规范和视觉回归资产。
- **风险：** 新增 Sync、Search、Agent Task、Approval 和 Artifact 页面时容易出现交互与视觉漂移，desktop/mobile 状态覆盖无法持续审查。
- **建议方向：** 使用 Pencil 维护 canonical `.pen`，覆盖 foundations、组件、页面与异常状态；通过设计日志、Vue token 和 Playwright 视觉回归保持同步。
- **处理门槛：** 大规模拆分或重写现有前端页面前完成 F1。

## 已关闭

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
- **保留兼容：** Cassandra 响应的 `id` 继续为零；全局身份使用 `message_id`，会话排序和分页使用 `message_seq`。IndexedDB 持久同步仍由 A6 独立推进。

### AD-004：热群消息缺少持久化同步补偿

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** Message 事务以 O(1) 写入群 Timeline 高水位，Sync 保存用户/设备/群拉取位点；客户端重连后提交已知群列表，经 Core 成员权限校验取得最新 Seq，并使用 `after_seq` 分页追平。在线 notify 聚合继续保留，Redis 或 Gateway 重启不会丢失离线发现依据。
- **验证：** 通过历史 migration 回填、消息/Outbox/高水位原子回滚、设备 ACK 单调性、越权拒绝、Message/Sync gRPC、HTTP 零 Seq cursor、Web 类型检查、真实 MySQL contract 和定向 race 测试。
- **兼容说明：** Web 本地消息库上线前不自动 ACK；`/messages/offline` 继续覆盖升级前历史和旧客户端，移除工作另行安排。

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
