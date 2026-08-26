# 架构债务台账

本文档记录已确认但暂缓处理的架构风险、兼容性缺口和可清理冗余，便于后续按优先级滚动治理。

## 维护约定

- 状态使用：`暂缓`、`处理中`、`已解决`、`接受风险`。
- 优先级使用：`P0` 阻断发布、`P1` 应在正式启用相关能力前解决、`P2` 进入后续迭代、`P3` 按需清理。
- 新问题使用连续编号 `AD-NNN`，保留历史编号，不复用已关闭条目。
- 开始处理时补充负责人或关联 Issue/PR；解决后记录提交、验证方式和完成日期。
- 本台账描述风险和演进方向，不代表当前迭代立即修改对应实现。

## 待处理

### AD-004：热群消息缺少持久化同步补偿

- **优先级：** P2
- **状态：** 暂缓
- **发现日期：** 2026-08-26
- **影响范围：** 热群、离线设备、多端同步
- **现状：** 热群跳过成员 Inbox，在线用户依赖 `group.message.notify + group timeline pull`；现有 Web 客户端仍通过 `/messages/offline` 补拉。
- **风险：** 只调用 `/sync` 的新客户端无法发现离线期间的热群消息，因此 `/messages/offline` 当前仍承担热群与升级前历史的兼容补偿。
- **建议方向：** 持久化群级 checkpoint/notify，或定义并实现 `/sync + group timeline pull` 混合同步协议及客户端游标。
- **处理门槛：** 移除 `/messages/offline` 或让前端全面迁移到 `/sync` 前完成。

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

### AD-010：GORM 模型与运行时 AutoMigrate 绑定数据结构

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-26
- **影响范围：** `internal/model`、`internal/repository`、`internal/store`、服务启动与数据库发布
- **现状：** schema 已由版本化 SQL migration 管理，运行时默认关闭 `AutoMigrate`；AICallLog、Admin、File、User 和 Contact 已具备 sqlc adapter 与 GORM 回切路径，其余仓储仍直接耦合 GORM API 或类型。
- **风险：** schema 变化缺少显式版本、审查、部署顺序和稳定回滚记录；跨语言服务难以共享一致的数据契约。
- **建议方向：** 先引入版本化 SQL migration，再通过 Repository Port 与 contract test 分批迁移到 `database/sql + sqlc`，最后删除 GORM。
- **处理门槛：** Message Service 独立拥有数据库和 Cassandra 投影开始前完成。

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
