# 架构问答

## 收件箱/发件箱设计

**Q: 当前设计没有用户的收件箱和发件箱，是否有必要？**

不需要引入。当前设计已覆盖收件箱/发件箱要解决的核心问题：

- 我发出去的消息：`ListByConversationKey` 按 `conversation_key` 拉取，发送方自然包含在内
- 我收到的消息：`ListOfflineByUserUUID` 按 `target_uuid = me` 或群成员关系过滤
- 未读数：`Conversation` 表的 `unread_count`，per-user 独立维护
- 离线补拉：增量游标 `after_id`，客户端维护 `last_synced_message_id`

收件箱/发件箱（写扩散）适用于需要对每条消息独立标记状态（如邮件语义）的场景。Dipole 是 IM 语义，消息是会话的一部分，`Conversation` per-user 记录已承担"用户视图"的职责。

如果后续需要"对我删除"或消息独立状态，可以加一张 `user_message_states(user_uuid, message_id, deleted, ...)` 表，比引入完整收件箱代价小得多。

---

## 群聊消息扩散模式

**Q: 当前的群聊消息是读扩散还是写扩散？**

混合模式：消息存储读扩散 + 未读计数写扩散。

- **消息存储（读扩散）**：消息只写一条记录，`target_type = Group`，`target_uuid = group_uuid`。读取时通过 JOIN 查询动态过滤出当前用户所属群的消息。
- **实时推送（读扩散）**：`syncDispatch=true` 时，dispatcher 遍历群成员列表逐个调用 `hub.SendEventToUser()`，消息本身只有一份。
- **未读数（写扩散）**：发群消息时，`ConversationService` 给每个群成员各自的 `Conversation` 记录做 `unread_count += 1`。

和微信的做法类似。

---

## 布隆过滤器

**Q: 当前设计中加入了布隆过滤器，是否冗余？**

不冗余，但收益有限，适用场景比较窄。

实际保护的场景只有一个：请求了一个从未存在过的 UUID（恶意枚举、客户端 bug、非法请求）。这种情况下布隆过滤器在内存里直接拦截，不走 Redis 也不走 MySQL。

对正常流量没有帮助：有效 UUID 的请求直接放行，真正的性能收益来自 Redis 缓存。

有一个设计缺陷：用户/群组被删除后，布隆过滤器不会更新（bloom filter 本身不支持删除）。删除后的 UUID 仍然会通过布隆过滤器，走到 Redis 和 MySQL 才发现不存在。它只能防"从未创建"，防不了"已删除"。

当前容量（1 万用户 / 2 千群组）也比较小，规模上去之后需要重新评估。

---

## 限流组件

**Q: 当前引入了限流组件，分析其原因，看看是否有效？**

引入合理，实现基本有效，但有几个值得关注的问题。

**算法**：固定窗口计数器（Redis INCR + TTL）。相比令牌桶，固定窗口在窗口边界有突刺问题，但对 IM 场景够用，Redis 原子操作保证了分布式正确性。

**保护范围**：
- 登录：防暴力破解，10次/5分钟
- 消息发送：120条/分钟，防刷消息
- 文件上传：10次/5分钟，防存储滥用

**明显漏洞**：
1. 注册接口没有限流，最容易被滥用的接口反而没保护
2. Redis 故障时静默放行（fail-open），攻击场景下等于限流失效
3. 登录 IP fallback 可被 `X-Forwarded-For` 伪造绕过

最值得修的是注册接口和 fail-open 行为。

---

## 心跳机制

**Q: 当前是否有心跳机制？如果 WebSocket 断开，服务端是否还会向客户端推送消息？**

有心跳机制，但离线消息不会丢失。

**心跳**：WebSocket 协议层 ping/pong 已实现，服务端每 54 秒发一次 ping，客户端需在 60 秒内回 pong，否则读超时断连。没有应用层心跳。

**断连后推送**：`SendEventToUser` 取当前在线的 client 列表，如果用户已断连，client 列表为空，消息直接丢弃。但这不是真正的问题，因为消息在发送时已经持久化到 MySQL，客户端重连后调用 `/messages/offline` 补拉即可。

**值得关注的点**：
- 没有应用层心跳，某些移动端网络或代理会静默丢包但不断连，60 秒后才超时感知。建议加应用层 ping/pong，间隔 15-30 秒。
- presence 状态有最多 60 秒的延迟，这段时间内显示"在线"但消息实际送不到。

---

## 消息发送链路

**Q: 一条消息的发送和接收链路是怎样的？**

### 单聊（Kafka 启用时）

```
发送方 WS ──► dispatcher.handleChatSend
                ├─ [同步] 限流检查
                ├─ [同步] 校验目标用户存在、是好友
                ├─ [同步] 构造 Message 对象
                ├─ [异步] 发布 message.direct.send_requested → Kafka
                ├─ [同步] UpdateDirectConversations（更新双方会话记录）
                ├─ [同步] hub.SendEventToUser → 推送给接收方（syncDispatch=true）
                └─ [同步] 回 ACK 给发送方（含 delivered 标志）

Kafka Consumer ──► persistDirectMessageHandler
                    ├─ message 写入 MySQL
                    └─ 发布 message.direct.created

Kafka Consumer ──► updateDirectConversationHandler（更新接收方未读数）
Kafka Consumer ──► deliverDirectMessageHandler（syncDispatch=false 时推送 WS）
```

### 群聊（Kafka 启用时）

```
发送方 WS ──► dispatcher.handleGroupChatSend
                ├─ [同步] 限流检查
                ├─ [同步] 校验群存在、发送方是成员
                ├─ [同步] 拉取全部群成员 UUID
                ├─ [同步] 构造 Message 对象
                ├─ [异步] 发布 message.group.send_requested → Kafka（含所有成员 UUID）
                ├─ [同步] UpdateGroupConversations（更新所有成员会话 + 未读数）
                ├─ [同步] 遍历成员逐个 hub.SendEventToUser（syncDispatch=true）
                └─ [同步] 回 ACK 给发送方

Kafka Consumer ──► persistGroupMessageHandler
                    ├─ message 写入 MySQL
                    └─ 发布 message.group.created

Kafka Consumer ──► updateGroupConversationHandler（再次更新所有成员未读数）
Kafka Consumer ──► deliverGroupMessageHandler（syncDispatch=false 时逐个推送 WS）
```

### 同步 vs 异步汇总

| 操作 | Kafka 启用 | Kafka 禁用 |
|------|-----------|-----------|
| 消息落库 | 异步 | 同步 |
| 发送方会话更新 | 同步 | 同步 |
| 接收方会话 + 未读数 | 同步 | 同步 |
| WS 推送给接收方 | 同步（syncDispatch=true） | 同步 |
| ACK 返回发送方 | 同步 | 同步 |

**注意**：Kafka 启用时，ACK 先于落库返回。发送方认为消息已发出，但此时 MySQL 里还没有这条消息。群聊同步推送在请求协程里串行遍历所有成员，群越大延迟越高。

---

## 消息回执

当前有两种回执：

**发送回执（chat.sent）**：消息发出后立即回给发送方，含 `delivered` 字段，表示推送时对方是否有在线 WS 连接。Kafka 模式下始终为 `false`（推送由 consumer 异步完成）。

**已读回执（chat.read）**：客户端打开会话时主动调 `POST /conversations/{target_uuid}/read`，服务端清零未读数，并将 `last_read_message_uuid` 推送给对方。

当前局限：
- 粒度是会话级（最后一条消息），不是消息级
- 群聊没有已读回执，只清未读数，不通知其他成员
- 没有"送达"回执，`delivered: true` 只说明推送时有连接，不代表客户端真正处理了消息
- 已读是主动触发的，不是自动的

---

## Message 与 Conversation 的关系

`messages` 表存消息本身，`conversations` 表是每个用户视角下的会话摘要，两者通过 `conversation_key` 关联。

`Conversation` 的唯一索引是 `(user_uuid, conversation_key)`，同一个会话每个参与者各有一行。

**conversation_key 格式：**
- 单聊：`direct:UUID_A:UUID_B`（两个 UUID 排序后拼接，保证双向同一个 key）
- 群聊：`group:GUUID`

**群里发一条消息会创建多少条 Conversation 记录：**

`UpdateGroupConversations` 遍历所有 N 个成员，每人一次 `UpsertGroupMessage`（ON CONFLICT DO UPDATE）。首次发消息 INSERT N 条，后续 UPDATE N 条。发送方 `unread_count` 不增加，其余 N-1 人各加 1。

这是写扩散，群越大写放大越高。

---

## 写扩散延迟问题

**无 Kafka**：`UpdateGroupConversations` 在发送链路上同步执行，N 次串行 MySQL 写全部完成后才回 ACK。100 人群 = 100 次串行写，延迟直接叠加。

**有 Kafka**：conversation 更新由 consumer 异步执行，不在发送链路上，发送延迟不受影响。但 consumer 侧存在一次冗余的 `ListMembers` 查询——`send_requested` payload 里已经带了 `RecipientUUIDs`，consumer 没有复用，而是重新查了一次成员列表。

---

## Kafka 异步落库的可靠性保障

Kafka 模式下消息发出即回 ACK，落库是异步的，当前的保障机制：

**已有保障：**
- consumer 处理失败 → 重试最多 `maxAttempts` 次（默认 3），指数退避
- 重试耗尽 → 路由到 `{topic}.dead` topic，提交 offset，不阻塞后续消息
- 只有处理成功（或成功路由 dead letter）才 CommitMessages，不会因 consumer 崩溃丢消息

**存在的风险：**

| 风险 | 当前状态 |
|---|---|
| consumer 处理失败 | 已覆盖，retry + dead letter |
| broker 宕机丢消息 | 部分覆盖，`RequireOne` 有窗口，改 `RequireAll` 可消除 |
| dead letter 消息无人处理 | 未覆盖，静默丢失，无监控无重放入口 |
| 首次启动跳过历史消息 | 已知行为，`StartOffset: LastOffset` 是有意为之 |

最值得修的是 `RequiredAcks: RequireOne` → `RequireAll`，一行改动，消除最主要的丢消息窗口。

---

## UUID 生成机制

所有实体 UUID 格式统一：**类型前缀 + `crypto/rand` 随机字节的大写 hex 编码**，没有使用标准 UUID v4。

| 类型 | 前缀 | 随机字节 | 总长度 |
|---|---|---|---|
| 用户 | `U` | 10 bytes | 21 chars |
| 群组 | `G` | 10 bytes | 21 chars |
| 消息 | `M` | 10 bytes | 21 chars |
| 文件 | `F` | 10 bytes | 21 chars |
| Kafka 事件 | `E` | 10 bytes | 21 chars |
| Token ID | 无 | 16 bytes | 32 chars |

AI 助手 UUID 是硬编码配置值（`UAI000000000000000001`），不走生成函数。

**分布式友好**：完全基于 `crypto/rand`，每个节点独立生成，无需协调，无时钟依赖，不存在 Snowflake 的时钟回拨问题。

**碰撞处理**：80 bits 随机性下碰撞概率约 `10^-24`，极低但不为零。用户和群组创建路径没有重试逻辑，碰撞直接报错。消息有重复处理逻辑，但那是为了应对 Kafka at-least-once 重投，不是 UUID 碰撞重试。

**无时间戳分量**：不像 ULID/UUID v7，UUID 本身无法推断创建时间，排序依赖 `created_at` 或自增 `id`。

---

## 系统整体设计总结

### 定位

Go 实现的模块化单体 IM 后端，支持单聊、群聊、文件传输和 AI 助手。以单机同步模式为基础，通过 Kafka 开关切换到异步分布式模式，两种模式共用同一套业务逻辑。

### 分层架构

```
HTTP/WebSocket
    ↓
Handler（Gin）          — 参数校验、鉴权、限流
    ↓
Service                 — 业务逻辑、权限检查
    ↓
Repository              — 数据访问接口（GORM）
    ↓
Store                   — MySQL + Redis
    ↑
Platform                — Kafka / MinIO / Bloom / Presence / RateLimit
```

### 基础设施

| 组件 | 用途 |
|---|---|
| MySQL | 消息、用户、群组、会话持久化 |
| Redis | 用户/群组缓存、Presence、限流计数、Bloom filter |
| Kafka | 可选，消息异步落库和事件分发 |
| MinIO | 文件存储，presigned URL 下载 |

### 主要设计权衡

| 决策 | 选择 | 代价 |
|---|---|---|
| 消息存储 | 读扩散（一条消息一行） | 群消息查询需 JOIN |
| 未读计数 | 写扩散（每人一行） | 大群发消息 N 次写 |
| Kafka 模式 | ACK 先于落库 | 发送方看到"已发"但消息可能还没持久化 |
| UUID 生成 | 纯随机无时间戳 | 无法按 UUID 排序，碰撞无重试 |
| 单体架构 | 模块化单体 | 水平扩展需要 Kafka + 共享 Redis，不能独立扩展各模块 |
