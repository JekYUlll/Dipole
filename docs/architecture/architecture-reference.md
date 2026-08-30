# Dipole 架构参考与改造方向

## 1. 文档目的

这份文档用于指导 `Dipole` 从当前的 demo 化骨架，逐步演进成一个**适合 IM 场景、又不过度设计**的 Go 项目。

> 当前演进基线（2026-08-28）：项目已完成 Core/Message/Gateway 的最小微服务边界，并进入存储架构与 Agent 治理阶段。Message Timeline 使用会话内 Seq，Conversation 保存用户级 `read_seq`，Sync Timeline 保存用户 Inbox，设备同步位置通过独立 checkpoint 显式确认。Cassandra 通过独立 Projector 与可恢复 Backfill 形成影子 Timeline；MySQL Message Metadata 独立保存幂等 locator、文件授权绑定和 payload hash。Message atomic、Message projector 与 Sync 使用专用 MySQL 凭据，启动时按 sqlc 实际操作验证最小权限。Search 全量恢复使用固定高水位、snapshot ID 和 SHA-256 的事件归档；归档 receipt 固定 MinIO object version ID 和 Governance 保留期，恢复不依赖对象 latest 或本地副本。Agent Memory 已具备默认关闭的 owner list/revoke、append-only correction、root 内容擦除和 Task 级派生影响审计；长期自动写入、公开擦除及派生内容自动删除仍保持关闭。下文关于模块化单体的内容保留为演进来源与回滚参考。

设备 Cursor 与群 `pulled_message_seq` 表示客户端已持久化位置，只接受客户端 ACK 单调推进。消息事件可以重建 Inbox 和群 Timeline 高水位，不能推导设备确认状态；灾难恢复应恢复已备份的 checkpoint，缺失时由客户端以本地安全位点重新确认。

Web Sync 本地持久层已建立 Chromium、Firefox、WebKit Playwright 验收，覆盖 IndexedDB 淘汰、重开、账号隔离、清理顺序和页面中断事务；`storage_full/sync_error` 使用独立的固定 outcome 错误指标，不携带消息身份或正文，也不污染协议比较窗口。真实磁盘配额和浏览器进程强退仍是默认 primary 前的外部门禁。

Agent Memory owner 治理遵循四条固定边界：tenant/principal/corrector 由 Gateway 认证链派生并由 Core 再次约束；公开 DTO 不暴露内部 provenance URI；撤销保留 revoker、原因和时间；纠正通过 root/version/predecessor 追加 successor 并原子撤销前序版本。客户端只以权威 previous/successor 返回更新状态，任何路径都不提供原地覆盖。

Agent Memory 派生影响采用保守 Task 边界：只要某个 Memory 版本进入 Task Context，该 Task 的模型调用、Plan/Step、Artifact、Tool、Message Action 与 Temporal History 都进入潜在影响集合。受管 Model planner 在 Context 编译后、任何模型调用前把规范化引用绑定到权威 Agent Task，Plan 提交再次幂等修复；历史 manifest 只做 ID 探测，未索引引用会让报告返回 `lineageComplete=false`。旁路或旧 Runtime 的已完成模型调用同时缺少 Plan 与 lineage 时，审计以当前 tenant/owner 范围的未归因 Task 计数阻断完整声明。审计报告不读取内容，也不授予删除或运行时执行权。

派生数据 retention 当前只提供离线策略判定。语言中立 v1 策略完整覆盖 Model Call、Shadow Plan/Step、Artifact、Tool Invocation、Message Action 与 Temporal potential Task，并将动作限制为擦除派生正文、保留最小审计、按天到期或人工复核。决策绑定 lineage report、policy 和自身 SHA-256，parser 会重新推导完整性与人工复核阻断；`policyComplete` 只表示策略覆盖充分，固定不代表已执行删除，也不提供数据库、网络、公开 API 或 Runtime 权威。

历史 Memory lineage 回填采用固定 high-water mark、按 Shadow Plan ID 单调推进的有界批次和低敏感 manifest/receipt。当前已完成语言无关契约、哈希校验、v43 MySQL checkpoint、sqlc source/target、owner-scoped adapter、可注入的 Go runner、默认 dry-run/显式审批 CLI，以及隔离 MySQL 的断点恢复、幂等重放、owner 隔离和 v43 up/down/reapply 验证；它不读取内容、不授予删除权或 Runtime 写权。共享环境维护窗口、部署 provenance 和独立审批记录仍需 rollout review，公共擦除与派生数据自动删除保持关闭。

参考对象有两个：

- `acc/KamaChat`：学习型项目，适合帮助我们快速理解 IM 业务主线
- `acc/im-server`：企业级实现，适合帮助我们校正系统边界、模块拆分和演进方向

我们的目标不是照搬任一项目，而是：

- 吸收 `KamaChat` 的业务完整性
- 吸收 `im-server` 的边界意识和模块设计
- 保持当前阶段仍然是**可快速推进的模块化单体**

---

## 2. 两个参考项目分别给我们的价值

### 2.1 KamaChat 提供的价值

`KamaChat` 更像一个“从业务视角组织的 IM 单体项目”，优点是链路很完整，容易上手：

- 用户：注册、登录、验证码登录、封禁、管理员
- 联系人：添加、删除、黑名单、申请/通过/拒绝
- 会话：单聊/群聊会话列表
- 消息：文本、文件、音视频数据
- WebSocket：在线收发消息
- Redis：缓存消息列表、验证码等
- Kafka：作为可选消息通道

值得借鉴的部分：

- 业务模块覆盖面完整
- 从 HTTP 到 service 到存储再到 WebSocket 的链路比较直观
- 数据模型对 IM 业务比较友好，例如 `UserInfo`、`Session`、`Message`

对应参考文件：

- `acc/KamaChat/internal/model/user_info.go`
- `acc/KamaChat/internal/model/session.go`
- `acc/KamaChat/internal/model/message.go`
- `acc/KamaChat/internal/service/gorm/user_info_service.go`
- `acc/KamaChat/internal/service/chat/server.go`
- `acc/KamaChat/internal/https_server/https_server.go`

### 2.2 im-server 提供的价值

`im-server` 更像一个“IM 核心平台”，不是普通业务项目。它强调的是边界、扩展性和部署演进：

- `connectmanager`：只负责连接与协议
- `usermanager`：只负责用户领域
- `message`：只负责消息处理与分发
- `conversation`：只负责会话
- `friendmanager`：只负责好友/关系
- `group`：只负责群
- `historymsg`：只负责历史消息
- `apigateway`：对外 HTTP API
- `navigator`：导航地址下发

值得借鉴的部分：

- **连接管理和业务管理解耦**
- **HTTP Gateway 与核心服务解耦**
- **消息、用户、会话、关系分别成域**
- **启动器统一初始化配置、数据库、服务注册**
- **存储层可切换**，例如 `message/storages/storage.go` 中按配置选择 MySQL/Mongo 实现

对应参考文件：

- `acc/im-server/launcher/main.go`
- `acc/im-server/services/connectmanager/server/imwebsocketserver.go`
- `acc/im-server/services/connectmanager/server/imlistener.go`
- `acc/im-server/services/usermanager/starter.go`
- `acc/im-server/services/usermanager/services/userservice.go`
- `acc/im-server/services/usermanager/storages/dbs/userdao.go`
- `acc/im-server/services/message/starter.go`
- `acc/im-server/services/message/services/msgservice.go`
- `acc/im-server/services/message/storages/storage.go`
- `acc/im-server/services/apigateway/routers/router.go`

---

## 3. 我们不应该直接照搬的部分

### 3.1 KamaChat 里不该直接照搬的部分

- 路由全部集中在一个文件里，扩展后会迅速失控
- controller 直接依赖全局 service 单例，模块边界较弱
- WebSocket 连接管理、消息落库、消息分发耦合较重
- 大量全局变量和 `init()` 初始化，不利于测试和替换
- HTTP 返回码设计较学习化，不适合长期扩展

### 3.2 im-server 里当前阶段不该照搬的部分

- 一开始就拆成大量服务
- actor system / cluster / rpc 总线
- protobuf 全链路协议
- 多种存储引擎并存
- 复杂的网关、导航、管理台体系

原因很明确：`Dipole` 目前还处于“把单体主链路做稳”的阶段，直接引入这些复杂度只会拖慢我们。

---

## 4. 对 Dipole 的核心判断

### 4.1 当前问题

当前 `Dipole` 已经有了：

- 配置加载
- HTTP 服务骨架
- MySQL/Redis 初始化
- demo 级 `user` 链路

但问题也很明显：

- `user` 模型还是偏 demo，不是 IM 用户模型
- 还没有 auth/register/login 主链路
- HTTP、业务、连接管理、消息模型之间还没有清晰边界
- 当前目录结构虽有分层，但**还不够模块化**

### 4.2 目标选择

当前阶段最合理的目标不是“做成 im-server 那样的微服务”，而是：

**做成一个模块化单体（modular monolith）的 IM 后端。**

也就是：

- 业务上按域拆分
- 部署上先保持单进程
- 存储上先用 MySQL + Redis
- WebSocket 保留为单独的连接层
- 后续若需要横向扩展，再把部分模块服务化

---

## 5. 推荐的目标架构

推荐逐步演进到如下结构：

```text
cmd/
  server/

internal/
  bootstrap/
    app.go
    http.go
    store.go

  platform/
    config/
    logger/
    database/
    cache/

  transport/
    http/
      middleware/
      response/
      routes/
    ws/
      hub/
      codec/
      session/

  modules/
    auth/
      domain/
      application/
      infrastructure/
      delivery/http/
    user/
      domain/
      application/
      infrastructure/
      delivery/http/
    contact/
    conversation/
    message/
    group/

docs/
```

### 5.1 为什么这样设计

这套结构综合了两个参考项目的优点：

- 从 `KamaChat` 学业务主线
- 从 `im-server` 学按领域拆模块
- 但不在当前阶段引入分布式 RPC 和 actor

### 5.2 当前项目与目标结构的映射

当前已有目录可以作为过渡：

- `internal/config` -> 未来并入 `platform/config`
- `internal/store` -> 已完成拆分并退役；数据库与缓存基础设施分别位于 `internal/platform/mysql` 和 `internal/platform/cache`
- `internal/gateway/http` -> 未来按远程服务 contract 继续拆分为 Gateway delivery adapters
- `internal/repository` -> 未来进入各模块 `infrastructure`
- `internal/service` -> 未来进入各模块 `application`

所以我们会沿着现有代码继续整理边界，不做推翻式重写。

---

## 6. 对用户域的直接参考结论

### 6.1 用户模型不应该再停留在 demo 级

我们当前的 `user` 需要从 demo 模型升级为 IM 用户模型。

建议第一阶段保留这些核心字段：

- `ID`
- `UUID`
- `Nickname`
- `Telephone`
- `Email`
- `Avatar`
- `Password`
- `Status`
- `IsAdmin`
- `CreatedAt`
- `UpdatedAt`

先不急着加：

- `Gender`
- `Birthday`
- `Signature`
- `LastOnlineAt`
- `LastOfflineAt`
- `DeletedAt`

这些可以在用户主链路跑稳之后再补。

### 6.2 UUID 和自增 ID 双轨并存

参考 `KamaChat` 和 `im-server`，我们应该明确：

- DB 主键用 `ID`
- 对外业务标识用 `UUID`

这样做的好处：

- 数据库存储和关联简单
- 外部 API 不暴露内部自增主键
- 后续会话、消息、联系人等模块都能统一以 `UUID` 为业务键

### 6.3 注册/登录应该成为下一个业务主线

下一步重点不应是继续扩 `GET /users/:id` 这种 demo 接口，而是补上：

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/:uuid`
- `PATCH /api/v1/users/:uuid/profile`

这是从 demo 化走向业务化的第一步。

---

## 7. 对消息与连接层的直接参考结论

### 7.1 三个参考项目里的 WebSocket 分别怎么做

`KamaChat` 的参考文件：

- `acc/KamaChat/api/v1/ws_controller.go`
- `acc/KamaChat/internal/service/chat/server.go`
- `acc/KamaChat/internal/service/chat/client.go`

它的特点是：

- HTTP 入口直接升级为 WebSocket 连接
- `ChatServer` 自己维护在线客户端映射和广播 channel
- 连接管理、消息解析、消息落库、在线转发集中在同一个聊天服务里

这套方案非常适合学习 IM 的最小闭环，因为链路短、阅读成本低、容易快速跑起来。

`im-server` 的参考文件：

- `acc/im-server/services/connectmanager/server/imwebsocketserver.go`
- `acc/im-server/services/connectmanager/server/imlistener.go`
- `acc/im-server/services/connectmanager/server/codec/*`

它的特点是：

- `connectmanager` 负责连接生命周期、协议和消息入口
- `listener` 负责把不同消息类型分派给后续业务处理
- 连接层和用户、消息、会话等业务域分层更明确

这套方案更像一个“连接管理中心”，很适合帮助我们建立边界意识。

`open-im-server` 的参考文件：

- `acc/open-im-server/internal/msggateway/ws_server.go`
- `acc/open-im-server/internal/msggateway/client.go`
- `acc/open-im-server/internal/msggateway/client_conn.go`
- `acc/open-im-server/internal/msggateway/user_map.go`
- `acc/open-im-server/internal/msggateway/message_handler.go`

它的特点是：

- 网关职责拆分更细
- 用户连接映射、多端管理、消息处理、压缩等能力都有独立对象
- 更适合多节点、多端、多协议演进

这套设计更成熟，也更重，当前阶段更适合作为远期边界参考。

### 7.2 我们采用的路线

`Dipole` 当前采用：

**`KamaChat` 的最小闭环 + `im-server` 的分层边界**

具体含义是：

- 学 `KamaChat`，先把单聊消息跑通
- 学 `im-server`，从第一版开始就把连接层和消息业务层分开
- 参考 `open-im-server`，提前知道后续多端、多节点、多协议会长成什么样

### 7.3 WebSocket 组件的职责划分

当前建议在单体内先形成下面这组职责：

- `transport/ws`：连接建立、鉴权、读写循环、心跳、断线清理
- `transport/ws/hub`：在线连接注册、注销、按用户路由连接
- `modules/message/application`：消息校验、发送用例、在线投递调用
- `modules/message/infrastructure`：消息持久化

第一版避免把太多能力塞进 WebSocket 层，先保证边界清楚：

- 连接层不直接操作复杂业务规则
- 消息层不直接感知底层连接实现细节
- 存储层不承担在线分发职责

### 7.4 当前阶段仍然不做微服务拆分

虽然 `im-server` 把 `connectmanager` 和 `message` 拆成独立服务是合理的，但 `Dipole` 当前不直接这么做。

当前最佳做法是：

- 先在单体内拆包
- 先把单聊 MVP 跑通
- 等接口和主链路稳定后，再考虑独立进程化

---

## 8. 我们接下来真正要做的改造顺序

### Phase 1：先把用户体系从 demo 改成 IM 体系

1. 重构当前 `user` 模型为 IM 用户模型
2. 新增 `auth` 模块
3. 完成注册、登录、查询资料、修改资料
4. 明确 DTO、entity、repository interface 的边界

### Phase 2：先拉起单聊消息与 WebSocket MVP

1. `transport/ws` 建连与鉴权
2. 在线连接管理与心跳
3. 文本消息发送、持久化、在线投递
4. 离线消息的最小回补能力

### Phase 3：补齐会话层

1. `conversation` 模型
2. 最近会话列表
3. 未读数与最后一条消息摘要
4. 单聊和群聊会话抽象

### Phase 4：补联系人链路

1. 联系人关系
2. 联系申请
3. 联系人列表
4. 黑名单能力

### Phase 5：做群与群消息

1. 群组
2. 群成员与角色
3. 群会话
4. 群消息

### Phase 6：接入 AI 能力

1. AI 助手账号体系
2. AI 会话与消息编排
3. 总结、辅助回复、内容治理等能力

#### Eino 版本与能力采用策略

- 当前基线：`Eino v0.9.17`、OpenAI 组件 `v0.1.13`、Ollama 组件 `v0.1.9`；alpha 评估结果见 `docs/architecture/EINO-V010-ALPHA-SPIKE.md`。
- 近期优先：在配置支持多个模型后接入 `ModelRetryConfig` 与 `ModelFailoverConfig`，提高限流、超时和供应方故障下的回复成功率。
- 中期评估：当客户端支持流式回复、取消生成或新消息抢占旧任务时，再以会话为粒度评估 `TurnLoop`。
- 按需引入：`AgenticMessage` 适合需要供应方原生工具、缓存或 MCP 语义的模型；`DeepAgents` 适合复杂任务规划和子 Agent 委派。当前单聊助手继续使用 `ChatModelAgent + schema.Message`，保持链路简单。
- 暂缓版本：`v0.10` 仍处于 alpha 阶段，等待稳定版和迁移说明后再评估持久会话、后台执行与自动记忆能力。

### Phase 7：接入 Cgo 高性能模块

1. 为热点路径建立 benchmark
2. 选择 1-2 个高收益点做 Cgo 加速
3. 保留 pure Go 回退实现

### Phase 8：补齐工程化能力

1. 监控
2. 限流
3. 后台管理
4. 部署与压测

---

## 9. 当前阶段的明确技术决策

为了避免后续摇摆，先明确以下决策：

- 架构形态：**模块化单体**
- 核心存储：**MySQL + Redis**
- 用户业务键：**UUID**
- 对外接口风格：**REST + JSON**
- 长连接：**WebSocket**
- 当前不引入：`etcd`、actor system、服务注册发现、Mongo 多引擎

具体执行顺序、每阶段交付项与阶段完成后的重构任务，见 [开发路线图](DEVELOPMENT-ROADMAP.md)。

---

## 10. 一句话版本的改造原则

**KamaChat 让我们知道“做什么”，im-server 让我们知道“怎么拆”，Dipole 接下来要做的是：先用模块化单体把 IM 核心主链路做稳，再按需要演进，而不是一开始就把企业级复杂度搬进来。**

---

## 11. 当前 Search 恢复与清理边界（2026-08-27）

- Search 全量重建源已从临时 Outbox 保留演进为 MinIO 不可变对象版本；receipt 固定 snapshot、高水位、SHA-256、对象 key 和 version ID。
- Backfill、Reconcile、Alias 与 Outbox Cleanup 必须使用同一 source descriptor，禁止换源或越过归档水位。
- Outbox Cleanup 使用独立 MySQL 账号，默认 dry-run；执行要求维护窗口、operator、对象版本可恢复、Job 完成和 Reconcile 一致。
- 清理后验收以归档恢复到全新空索引、3/3 hash 对账和 Alias 正反切换为准；手工批量删除继续禁止。
- 该边界解决 AD-021，并为后续 MySQL 正文退役提供独立 Search 恢复源；Sync、幂等响应和 Cassandra 修复仍由 AD-019 单独跟踪。

## 12. Cassandra 幂等响应恢复边界（2026-08-27）

- 重复发送先由 MySQL Metadata 校验 client id、目标与 payload hash，再用 `conversation_key + message_seq + message_uuid` 精确定位 Cassandra Timeline。
- Cassandra body 命中后由 Metadata v14 补回 legacy MySQL ID，保持旧 HTTP/gRPC 响应兼容；定位缺失、冲突、异常或历史 Seq 缺失时回退 MySQL。
- `message.cassandra_duplicate_hydration` 默认关闭，独立于历史/Sync Cassandra 主读比例；以 `hit|fallback|skipped_no_seq` 有界指标积累正文退役证据。

## 13. Cassandra 完整消息归档边界（2026-08-27）

- Cassandra Timeline 灾难恢复使用完整消息快照，保留 client ID、conversation key、Seq、正文、文件字段和时间；它与 Search 最终 mutation 快照具有不同领域语义。
- snapshot manifest 固定 MySQL Message 高水位、entry count 和 SHA-256；发布 receipt 固定 MinIO bucket、object key、Version ID、ETag 与 Governance 保留截止时间。
- migration v15 将 Backfill Job 绑定到 `source_kind + source_snapshot_id + source_sha256`。Backfill 与 Reconcile 必须使用同一归档，换源需要新 Job。
- MySQL 仍保存 Job/checkpoint 运维元数据；归档源可在 `messages` 正文删除后独立提供 Timeline 重建和完整对账。
- `dipole-message-archives` 与 Search 归档使用独立 bucket 和保留配置，共享无领域含义的版本对象存储端口。

## 14. 在线会话 Timeline 增量契约（2026-08-27）

- Direct 与 Group 历史接口均支持 `after_seq`，以会话内 `message_seq` 作为在线通知后的增量补拉游标。
- HTTP、Message v1 gRPC、Local/Remote/Shadow adapters 使用相同契约；protobuf 仅追加 `after_sequence` 字段，保留已有调用方 wire compatibility。
- Direct 与 Group 共用 Cassandra cohort、连续页校验、抽样验证和 MySQL fallback；当前客户端投递行为保持不变。
- 后续 `sync.item.notify` 先以默认关闭的 shadow 模式接入，完成通知丢失、重复、乱序和主读灰度门禁后再替换完整 WS 正文投递。

## 15. Timeline 轻量通知 Shadow 边界（2026-08-27）

- `sync.item.notify.v1` 只携带版本、event ID、Message UUID、会话 key/Seq 和目标 locator，禁止携带正文、文件或发送者资料。
- Gateway `message.timeline_notify_mode=shadow` 在现有完整消息之后附加通知；默认 `off`，运行时暂不接受 primary，便于无数据回滚。
- Web `VITE_TIMELINE_NOTIFY_MODE=shadow` 按会话串行补拉，使用 event ID 有界去重，并从最后已验证 Seq 恢复通知丢失形成的间隔。
- 普通 Direct/Group 进入验证链路；热群继续使用单一聚合 notify + pull，避免重新引入成员级实时通知写扩散。
- 晋级证据固定为完整 24 小时、至少 100 次 match、零 missing/mismatch/error/invalid；shadow 期间完整消息继续驱动用户界面。

## 16. Web IndexedDB 完整进程强退边界（2026-08-27）

- Playwright 使用独立 Chromium persistent profile，避免 crash 影响测试 runner 自身管理的浏览器。
- 测试必须先确认生产 `IndexedDBSyncStore.commitPage` Promise 仍 pending，再通过 CDP `Browser.crash` 终止浏览器主进程。
- 同一 profile 重启后只接受整页回滚或整页提交；Message 数量、安全 `sync_seq` 和 manifest 必须属于同一结果。
- 完整进程强退证据已经完成；真实 origin 配额拒绝和共享设备端到端清理继续由 AD-025 跟踪。

## 17. Web IndexedDB 真实容量拒绝边界（2026-08-27）

- `check-web-sync-real-quota.sh` 在无特权 user/mount namespace 内挂载 128 MiB tmpfs，Chromium profile 与 IndexedDB 数据全部位于受限文件系统。
- 24 MiB reserve file 保证容量拒绝后仍可释放空间并读取恢复状态；测试正文使用浏览器随机字节，避免压缩掩盖真实占用。
- 每次提交一页并记录最后成功 Seq；首次容量错误必须被现有 `isLocalSyncCapacityError` 识别，释放 reserve 后失败 Seq 不得进入 Message、Cursor 或 manifest。
- 普通三浏览器 E2E 保持快速且默认跳过该外部门禁；专用脚本在支持 user namespace/tmpfs mount 的 Linux CI 或验收机执行。

## 18. Web 共享设备会话清理边界（2026-08-27）

- E2E 在生产默认 IndexedDB 同时保存 U1/U2，再加载真实 Vue 应用；HTTP 通过 Axios 401 interceptor，WS 通过 `useWebSocket` 注入 `session.kicked`。
- 两条链路都必须清除 token、user、legacy cursor 与 U1 Message/Cursor，并保留 U2 的 Seq 和 Message。
- Chromium、Firefox、WebKit 使用相同断言，证明清理语义不依赖单一浏览器实现。
- 公共设备应显式退出；浏览器在清理确认前被强制终止时，需要从浏览器设置清除 Dipole 站点数据。

## 19. Core v1 事件契约边界（2026-08-27）

- `contracts/events/message/v1` 是 Go、后续 TypeScript Agent 与 C++ Delivery 的语言中立来源；pre-persistence `send_requested` 和 confirmed Message fact 使用独立 schema。
- Envelope 缺少版本时按 legacy v1 读取；`v1.x` 只允许追加字段，消费者忽略未知 payload 字段，未来 major 默认拒绝直到所有活跃消费者显式支持。
- `service.DecodeMessageEventPayload` 统一 legacy created mutation/revision/actor 默认值，并校验 direct/group channel 与 `target_type`；Gateway、三个在线 Projector、Search Backfill 和 Sync Replay 禁止自行复制 decoder。
- Sync/Cassandra 的正 Seq、群 recipient snapshot 等要求属于投影契约，在公共事件 decoder 之后校验，避免存储策略污染跨语言 schema。
- `contracts/events/domain/v1` 覆盖 Group、Conversation Read、Contact Friend Deleted 与 Session Force Logout；各 Gateway handler 统一调用 service decoder。
- `kafkaManagedTopics()` 必须被 schema event type 唯一覆盖。新增受管 Topic 时，缺少契约或重复声明都会让 bootstrap contract test 失败。

## 20. Agent Subscription 创建权限边界（2026-08-28）

- Gateway 从 JWT 会话派生 principal、从配置固定 tenant；公开 create body 只接受 Definition ID/version、conversation key 与确定性 filter。
- Core 使用现有会话发现能力计算 `principal readable ∩ Definition conversation scope`，并将 conversation key 映射为精确 direct/group event type；候选读取失败、畸形 key、Definition 漂移或 scope 漂移全部 fail closed。
- Gateway 在提交时重新派生 event/resource，Core 在 Store 写入前再次执行同一 authority 校验；模型与浏览器不能提供 principal、tenant、event type 或 resource。
- `all|message_contains_any` 保持确定性规范化；关键词最多 32 项、单项最多 64 个 Unicode 字符，前端不得静默截断用户意图。
- list/create/revoke、Definition 目录和 conversation chooser 共享默认关闭的 Gateway/Frontend 开关。控制记录 active 只表示持久授权有效，Runtime 与 Compose 继续固定 `direct_target`。
