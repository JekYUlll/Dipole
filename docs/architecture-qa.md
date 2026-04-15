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
