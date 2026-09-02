# 架构问答

## 收件箱/发件箱设计

**Q: Message Store 与 User Inbox Timeline 为什么要分开？**

当前架构已经引入 User Inbox Timeline，并与 Message Store 分工：

- 历史消息：Message Store 按 `conversation_key + message_seq` 提供漫游和分页
- 待同步消息：Sync Store 按 `user_uuid + sync_seq` 保存 `user_sync_inbox` locator
- 未读状态：Conversation State 使用 `last_message_seq + read_seq` 表达用户读位点
- 离线补拉：客户端按设备 Cursor 请求 `after_sync_seq`，再按 locator 拉取完整消息

Message Store 与 Sync Store 分离后，历史查询和用户同步查询分别按自己的访问模式优化。普通群可使用用户级 Inbox 写扩散，热群继续使用聚合通知加 Timeline 拉取，避免按成员无限放大写入。

`user_sync_inbox` 只保存 locator，不复制消息正文；设备 Cursor、群组 checkpoint 和幂等约束共同保证多端恢复。对我删除等用户视图状态可以在此模型上继续增加独立投影。

---

## 群聊消息扩散模式

**Q: 当前的群聊消息采用哪种扩散模式？**

混合模式：Message Store 保持单条消息，Sync Timeline 和 Conversation State 根据群规模选择写扩散或读扩散。

- **Message Store**：消息按会话保存一份，由 `conversation_id + message_seq` 定位。
- **普通群**：消息事件经 Kafka 投影到收件人 Inbox，并更新用户会话状态。
- **热群**：保留群级 Timeline checkpoint，实时层发送聚合 notify，客户端按 Seq 补拉，减少成员级写扩散。
- **未读状态**：使用 `last_message_seq` 与 `read_seq` 计算，并保留投影校正能力。

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

当前有两种主要回执：

**发送回执（chat.sent）**：消息发出后立即回给发送方，含 `delivered` 字段，表示推送时对方是否有在线 WS 连接。Kafka 模式下始终为 `false`（推送由 consumer 异步完成）。

**已读回执（chat.read）**：客户端打开会话时提交会话级 `read_seq`，服务端推进用户会话读位点，并向其他设备同步状态。

当前局限：
- 目前以会话 Seq 读位点为主，消息级送达状态仍属于后续能力
- 群聊没有已读回执，只清未读数，不通知其他成员
- 没有"送达"回执，`delivered: true` 只说明推送时有连接，不代表客户端真正处理了消息
- 已读是主动触发的，不是自动的

---

## Message 与 Conversation 的关系

Message Store 存完整消息事实，`conversations` 表保存每个用户视角下的会话状态，Sync Store 保存用户待同步 locator，三者通过会话标识和 Seq 关联。

`Conversation` 的唯一索引是 `(user_uuid, conversation_key)`，同一个会话每个参与者各有一行。

**conversation_key 格式：**
- 单聊：`direct:UUID_A:UUID_B`（两个 UUID 排序后拼接，保证双向同一个 key）
- 群聊：`group:GUUID`

**群里发一条消息会如何更新用户状态：**

普通群通过 Kafka projector 为收件人更新 Inbox 和 Conversation State；热群使用群级 checkpoint 与聚合 notify，客户端按 Seq 补拉。Conversation State 以 `last_message_seq`、`read_seq` 和投影状态表达未读位置。

普通群仍存在成员级写扩散成本，热群路径通过读扩散降低该成本。

---

## 写扩散延迟问题

**同步兼容模式**：Conversation State 可在发送链路内更新，成员级写入会随群规模增加。

**Kafka 模式**：Message、Sync 和 Conversation projection 在发送链路外异步执行，发送延迟与投影延迟解耦；事件携带的 recipient snapshot 用于幂等和权限边界校验。

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

Dipole 是采用模块化单体渐进演进的事件驱动 IM 平台，当前已提供 Core、Gateway、Message、Sync、Search、Agent Runtime 等独立服务入口；Go 承担 IM 业务与一致性，TypeScript 承担 Agent Runtime，C++ Realtime Delivery 作为独立候选数据面。

### 分层架构

```
Client
    ↓ HTTP / WebSocket
IM Gateway
    ↓ gRPC / Kafka
Core / Message / Sync / Search / Agent Runtime
    ↓
sqlc + MySQL | Kafka | Redis | Cassandra | Elasticsearch | MinIO
```

### 基础设施

| 组件 | 用途 |
|---|---|
| MySQL | Core 状态、Message 元数据/兼容正文、Sync 状态和 Agent ledger |
| Redis | 用户/群组缓存、Presence、限流计数、Bloom filter |
| Kafka | Outbox 事件传播、投影和跨服务异步解耦 |
| Cassandra | Message Timeline 影子/候选主读 |
| Elasticsearch | 消息搜索索引和重建投影 |
| MinIO | 文件存储，presigned URL 下载 |

### 主要设计权衡

| 决策 | 选择 | 代价 |
|---|---|---|
| 消息存储 | Message Store 按会话 Seq 保存事实 | Cassandra 主读仍需灰度与回切证据 |
| 用户同步 | Sync Store 保存 Inbox locator 和设备 Cursor | 普通群存在写扩散，热群需要 checkpoint |
| 未读状态 | Conversation State 使用 `last_message_seq + read_seq` | 投影延迟期间需要状态对账 |
| Kafka 模式 | Outbox 后异步传播 created 事件 | 需要 retry/DLQ、幂等和 ownership 门禁 |
| UUID 生成 | 纯随机无时间戳 | 无法按 UUID 排序，碰撞无重试 |
| 服务演进 | 模块化单体起步、按边界渐进拆分 | 兼容入口和共享 schema 仍需按 rollout 门禁逐步退出 |
