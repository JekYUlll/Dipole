# Dipole 面试问答整理

> 这是一份持续维护的讲解材料。涉及旧模块名、旧目录或旧技术栈的答案，应以当前架构文档和代码为准。

投递用描述、现场介绍、状态边界和学习路线见 [Dipole IM 项目材料](DIPOLE-IM-LEARNING-AND-INTERVIEW.md)。Agent 相关材料见 [Dipole Agent 项目材料](DIPOLE-AGENT-LEARNING-AND-INTERVIEW.md)。本文只保留 IM 深入追问与展开答案。

## 1. 项目自我介绍

`Dipole` 是我用 Go 独立设计和持续迭代的一个即时通讯后端项目。我希望它覆盖一套链路完整、能体现工程能力的 IM 系统：用户登录鉴权、好友关系、单聊群聊、会话和未读数、文件消息、分片上传，以及三节点部署下的在线连接管理。

技术栈上我主要用了 `Go + Gin + sqlc + MySQL + Redis + Kafka + MinIO + WebSocket`。当前仓库采用面向服务边界的 Monorepo：Core、Gateway、Message、Sync、Search 具备独立入口，消息链路通过 Kafka 和事务型 outbox 解耦，服务仍保留 embedded 兼容路径以支持渐进迁移和回滚。

这个项目里我觉得比较有代表性的点有三个。第一，我把单聊、群聊、会话、文件、已读、离线补拉这些主链路都真正跑通了。第二，我做了三节点部署，结合 Redis presence 和 Pub/Sub，把跨节点 WebSocket 投递串起来了。第三，我针对大群场景做了热点群优化，把完整 push 改成 `notify + pull`，又加了 `singleflight` 和短 TTL 缓存，500 人群压测能稳定在秒级。

如果面试官继续往下问，我通常会重点展开两类内容：一类是消息可靠性，比如 Kafka、outbox、入口幂等和多节点重试；另一类是性能优化，比如热群链路、复合索引、WebSocket 重连和压测分析。因为这些部分最能体现我在这个项目里真正做过工程取舍和问题定位。

---

## 2. 项目定位与架构

### Q1：这个项目的定位是什么？

**答：**

它是一套采用渐进式微服务边界的 IM 平台。Core、Gateway、Message、Sync 和 Search 已有独立的入口或服务目录；embedded 启动路径仍作为兼容与回滚方式保留。相比一次性拆分所有组件，当前方案优先稳定跨服务契约、数据所有权和可回滚门禁。

### Q2：为什么选择模块化单体，而不是一开始做微服务？

**答：**

因为 IM 的复杂度并不只在“服务有几个”，而在于消息链路、状态一致性、连接管理和用户视角的数据模型。相比一开始拆很多服务，模块化单体更适合快速验证核心链路，也更容易测试和重构。等边界稳定后，再把某些模块服务化会更自然。

### Q3：项目的核心模块有哪些？

**答：**

可以分成几层：

- 接入层：Gin HTTP + WebSocket
- 业务层：Core（Auth、User、Contact、Conversation、Group、File）、Message、Sync、Search
- 异步层：Kafka producer / consumer + outbox relay
- 状态层：Redis cache / presence / rate limit / hot-group
- 存储层：sqlc/MySQL、Kafka、Redis、Cassandra、Elasticsearch、MinIO；其中 Cassandra 和 Elasticsearch 按独立投影及回滚门禁逐步接管
- 分布式投递层：Redis presence + PubSubRouter + Hub

### Q4：Nginx 在项目里起什么作用？

**答：**

当前主要有五个作用：

- 统一 HTTPS 入口
- 反向代理 `/api`、`/ws`、`/app`
- 三节点负载均衡
- WebSocket upgrade 代理
- 入口层请求大小控制，比如头像和文件上传体积限制

---

## 3. 用户、联系人与权限

### Q5：登录鉴权是怎么做的？

**答：**

现在走的是 `JWT + Redis 黑名单`。登录后服务端签发 JWT，里面带 `sub/jti/exp` 等字段。鉴权中间件先验签，再查 Redis 黑名单，支持立即注销。这个方案比纯 Redis session 更适合无状态 API，同时还能保留登出失效能力。

### Q6：好友关系在消息链路里起什么作用？

**答：**

单聊权限是和关系链绑定的。普通用户之间只有在好友关系成立时才能发单聊消息，删除好友后历史消息保留，但不能继续发新消息。AI 助手是例外，它作为特殊用户接入现有消息体系，不受好友关系限制。

### Q7：删除好友之后系统怎么处理？

**答：**

后端会删除双向联系人关系；前端会刷新联系人和会话列表；如果用户当前正处在这个单聊页，会切成只读历史状态。为了保证在线用户状态同步，我还加了 `contact.friend_deleted` 事件，通过 Kafka 或本地 notifier 推给双方，让前端不用手动刷新。

---

## 4. Message 与 Conversation

### Q8：`Message` 和 `Conversation` 分别是什么？

**答：**

`Message` 是消息事实本体，存的是谁发的、发给谁、内容是什么、类型是什么、发送时间、文件信息等。

`Conversation` 是用户视角下的会话摘要，存的是：

- 会话对象
- 最近一条消息预览
- 未读数
- 最后活跃时间
- 备注等展示信息

所以它们的关系可以概括成：

- `Message` 负责事实日志
- `Conversation` 负责首页会话索引

### Q9：Conversation 是什么时候创建的？

**答：**

- 单聊：第一条消息成功进入 `message.direct.created` 链路后，会给双方各 upsert 一条会话
- 群聊：建群时先初始化群会话，后续每条群消息继续更新成员会话

这样用户上线后可以先拉会话列表，再按需拉消息正文。

### Q10：为什么要单独维护 Conversation，而不是每次从 Message 聚合？

**答：**

因为消息表天然适合存事实，不适合高频做“按会话分组、取最后一条、统计未读、按活跃时间排序”。如果首页每次都从消息表做聚合，查询成本会高很多。把 Conversation 单独做成用户侧索引后，会话列表查询会稳定很多。

---

## 5. 单聊消息链路

### Q11：单聊消息的完整链路是什么？

**答：**

可以按这条顺序讲：

1. 客户端通过 WebSocket 发 `chat.send`
2. `dispatcher` 做鉴权和限流，调用 `MessageService.SendDirectMessage`
3. 业务层校验目标用户、用户状态、好友关系
4. 生成 `Message`，发布 `message.direct.send_requested`
5. Kafka consumer 消费后执行 `PersistRequestedMessage`
6. 这里会在一个 MySQL 事务里完成“消息落库 + outbox 入队”
7. outbox relay 再发布 `message.direct.created`
8. `created` 事件被消费后更新双方 Conversation，并给在线接收方推 `chat.message`
9. 发送方在入口层会收到 `chat.sent` ACK

### Q12：你们的 ACK 是什么语义？

**答：**

我们现在的 ACK 是 WebSocket 的 `chat.sent`，语义是“服务端已经接收并接受了这次发送请求，消息已进入后续链路”。它适合前端把本地消息状态从“发送中”改成“已发送”。

如果继续细分，还可以有 `persisted`、`delivered`、`read`，当前已经落地的是：

- `chat.sent`
- `chat.read`

### Q13：已读回执是怎么做的？

**答：**

现在先做了单聊已读：

1. 接收方打开单聊页，调用 `PATCH /conversations/direct/:target_uuid/read`
2. 服务端清当前用户这边的未读数
3. 生成 `conversation.direct.read` 事件
4. Kafka 消费后给对端推 `chat.read`

前端就能据此把自己发出的消息标成“已读”。

---

## 6. 群聊消息链路

### Q14：群聊消息的链路和单聊有什么区别？

**答：**

前半段很像，主要多了两个点：

- 发送前要校验群是否存在、群状态是否正常、发送者是不是群成员
- 发送后要更新整个群成员集合对应的 Conversation

链路是：

1. WebSocket `chat.send`
2. `MessageService.SendGroupMessage`
3. 校验群状态和成员权限
4. 发布 `message.group.send_requested`
5. consumer 落库 + outbox
6. relay 发布 `message.group.created`
7. consumer 更新群成员 Conversation
8. 冷群走完整 `chat.message` push，热群走 `group.message.notify + pull`

### Q15：群消息为什么会慢？

**答：**

群消息的瓶颈主要在两个地方：

- Conversation 写放大：一条群消息会触发多名成员的会话 upsert
- 在线投递放大：大群在线成员多时，完整 push 的 fan-out 压力很大

500 人群压测时，冷群完整 push 的平均延迟接近 30 秒，p95 超过 60 秒，这也是后来我做热点群改造的直接原因。

### Q16：热点群是怎么设计的？

**答：**

我做了一个 Redis 热点检测器，综合群成员数和时间窗内消息数判断某个群是否进入热点状态。进入热点后，投递策略从“完整 push 消息正文”切成“轻量通知 + 客户端增量 pull”。

具体来说：

- 冷群：直接推 `chat.message`
- 热群：推 `group.message.notify`
- 客户端收到通知后，调用 `GET /messages/group/:group_uuid?after_seq=...` 增量补拉

### Q17：热点群改造后效果怎么样？

**答：**

500 人群压测下：

- 冷群完整 push：`avg 29.72s`，`p95 60.64s`
- 热群 notify + pull：`avg 2.06s`，`p95 3.57s`

这说明广播扇出的确是主要瓶颈，把大群切成轻通知后延迟明显下降。

### Q18：为什么又加了 singleflight？

**答：**

因为热群模式里，很多客户端会在收到同一条 `notify` 后同时发起相同的增量补拉请求。单机内这会形成大量重复查询。我就在群消息增量拉取路径上加了 `singleflight`，按 `group_uuid + after_seq + limit` 合并并发请求，减少热群场景下的读放大。

---

## 7. Kafka、Outbox 与可靠性

### Q19：Kafka 在项目里主要承载哪些事件？

**答：**

当前主要有：

- `message.direct.send_requested`
- `message.group.send_requested`
- `message.direct.created`
- `message.group.created`
- `conversation.direct.read`
- `group.created / updated / members.added / members.removed / dismissed`
- `session.force_logout`
- `contact.friend.deleted`

它承担的是业务事件总线角色，用来解耦消息持久化、会话更新、群事件广播、已读回执和会话控制。

### Q20：为什么消息链路里要区分 `send_requested` 和 `created`？

**答：**

因为两者含义不同：

- `send_requested`：接入层已经接受这次发送请求，后面要去持久化
- `created`：消息已经成功进入系统事实层，后续可以安全地更新会话、做在线投递、触发 AI

把它们拆开之后，链路会更清晰，也更适合异步重试。

### Q21：为什么要引入 outbox？

**答：**

因为我排查消息链路时发现一个真实问题：如果消息已经落库，但 `message.*.created` 发布失败，那么后续会话更新、在线投递、AI 触发都会缺失。这是一个一致性缺口。

所以后来我做了事务型 outbox：

- `messages` 和 `outbox_events` 在同一个 MySQL 事务里提交
- 后台 relay 扫描 pending outbox，再异步发 Kafka

这样只要消息落库成功，`created` 事件就一定有一份持久记录，不会因为进程退出或 Kafka 短暂异常而丢掉。

### Q22：为什么没有直接上 Kafka 事务？

**答：**

Kafka 事务更适合解决“多条 Kafka 消息之间的原子性”或者“生产和 offset 提交的一致性”。我这里最核心的问题是 `MySQL + Kafka` 的跨系统一致性，所以 outbox 更合适。

---

## 8. Redis、缓存与在线状态

### Q23：Redis 在项目里主要做了什么？

**答：**

当前主要有六类用途：

- 用户资料、群资料、群成员缓存
- 联系人关系缓存
- JWT 注销黑名单
- 限流计数
- 在线状态与多端设备会话
- 热点群检测
- 多节点 WebSocket 事件转发

### Q23.1：Redis 在这个项目里分别用了哪些数据结构？

**答：**

我现在主要用了这几类 Redis 数据结构：

1. `String`
- 用在：
  - JWT 黑名单
  - 热群激活标记
  - 热群消息页短 TTL 缓存
- 这类 key 的特点是：
  - 结构简单
  - 适合带过期时间

2. `Hash`
- 用在用户在线连接目录
- 例如：
  - `presence:user:<uuid>:connections`
- field 是 `connection_id`
- value 是序列化后的 `ConnectionState`

3. `ZSet`
- 用在在线用户和在线连接的过期管理
- 例如：
  - `presence:online_users`
  - `presence:online_connections`
- score 存的是过期时间戳
- 这样清理过期连接会比较自然

4. 计数型 key
- 用在限流和热群检测
- 本质上依赖：
  - `INCR`
  - `EXPIRE`
- 例如：
  - 注册/登录/发消息限流窗口
  - 热群消息窗口计数

所以如果让我一句话总结 Redis 结构选择，我会说：

- 状态目录用 `Hash + ZSet`
- 短生命周期结果缓存和标记位用 `String`
- 窗口计数用 `INCR + EXPIRE`

### Q23.1.1：Redis Pub/Sub 在项目里是怎么用的？

**答：**

Redis Pub/Sub 这层主要服务多节点 WebSocket 转发。

场景是：

- 消息可能在 `node1` 上完成消费和业务处理
- 目标用户的 WebSocket 连接实际握在 `node2` 手里

这时候 `node1` 会先通过 Redis presence 查出：

- 这个用户当前有哪些连接
- 这些连接分别在哪些节点上

如果连接就在本机，就直接交给本机 `Hub` 写 socket；如果连接在其他节点，就往目标节点对应的 Redis Pub/Sub 频道发一条内部事件，目标节点订阅到后再交给自己的 `Hub` 去真正下发。

所以这层职责可以概括成一句话：

- Redis presence 负责查地址
- Redis Pub/Sub 负责跨节点转发
- Hub 负责真正写连接

### Q23.1.2：消息转发这里为什么用 Redis Pub/Sub，不直接用 Kafka？

**答：**

因为这里解决的是“节点之间把一个在线 WebSocket 事件快速转给正确节点”，它更像节点内投递链路的一部分，特点是：

- 目标非常明确，就是某个节点
- 追求的是低延迟即时转发
- 只对在线连接有意义
- 节点收到后立刻交给本机 `Hub`

Kafka 在我们项目里主要承担的是业务事件总线，适合：

- `send_requested`
- `message.created`
- `group.dismissed`
- `conversation.direct.read`

这类需要解耦、可重试、可积压处理的异步事件。

而跨节点 WebSocket 转发这件事更轻、更短，最重要的是“尽快送到那个节点手里”。如果这里也改成 Kafka，链路会变长，运维和消费语义也会更重。

所以我会这样概括这两个组件的分工：

- Kafka 负责业务事件传播
- Redis Pub/Sub 负责在线连接的跨节点即时转发

### Q23.1.3：那 Pub/Sub 和消息队列的区别是什么？

**答：**

面试里我一般会从三点来讲。

1. 持久化能力
- Pub/Sub 更偏瞬时广播，订阅方当时不在线，这条消息通常就过去了
- 消息队列通常会保留消息，消费者稍后还能继续处理

2. 消费语义
- Pub/Sub 更像“谁订阅谁就收到一份”
- 消息队列更强调消费组、位点、重试、积压处理

3. 使用场景
- Pub/Sub 适合在线通知、节点间轻量转发、配置变更广播
- 消息队列适合业务异步解耦、削峰填谷、失败重试、可靠投递

放到我们项目里就很好理解：

- Redis Pub/Sub 负责“把 WS 事件尽快转给正确节点”
- Kafka 负责“把业务事件可靠地传播给后续消费者”

所以这里选择 Redis Pub/Sub，本质上是因为这条链路更看重实时节点转发，而不是消息堆积和可靠消费。

### Q23.1.4：那 Redis Pub/Sub 会不会丢消息？

**答：**

会，Redis Pub/Sub 天然就存在丢失的可能。

典型场景有这些：

- 目标节点当时没订阅
- 节点短暂重启或网络抖动
- Pub/Sub 消息发出时，对端应用还没准备好消费

所以我不会把 Redis Pub/Sub 当成可靠消息队列来用。

在我们这个项目里，这件事之所以能接受，是因为它承担的是：

- 在线用户的跨节点即时转发

这条链路的目标是：

- 在线时尽快送达

如果这一跳偶发失败，系统后面还有别的补偿路径：

- 消息事实已经在 MySQL 里
- 业务事件已经进 Kafka
- 用户重连后还能通过历史消息、离线补拉、热群 `notify + pull` 把状态对齐

所以这里的设计思路是：

- Redis Pub/Sub 追求低延迟
- Kafka + MySQL 负责可靠性

如果面试官继续追问，我通常会再补一句：

- 只要某条链路要求“消息不能丢、可以重试、允许积压”，我就不会选 Redis Pub/Sub，我会放到 Kafka 这类消息队列里。

### Q23.1.5：你们在 Redis 里具体设置了哪些过期时间？

**答：**

我会把它按用途分开讲，因为项目里不是所有 Redis key 都用同一种 TTL。

1. Presence 在线状态
- `presence:user:<uuid>:connections`
- `presence:online_users`
- `presence:online_connections`
- 这组的过期时间来自 `presence.ttl_seconds`
- 当前默认是 `120s`
- WebSocket 建连和心跳时会持续刷新，所以只要连接还活着，过期时间就会不断往后推

2. 业务缓存
- 用户资料缓存：`10min`
- 群资料缓存：`10min`
- 群成员列表缓存：`10min`
- 联系人关系缓存：`10min`
- 这些都定义在 [`internal/platform/cache/`](../../internal/platform/cache/)

3. 热群消息页缓存
- 热群增量消息页：`1s`
- 热群空页缓存：`500ms`
- 这组 TTL 很短，目标是吸收同一波补拉请求，避免热群瞬时把 MySQL 打热

4. 热点群检测
- 热点计数窗口：来自 `hot_group.window_seconds`
- 当前默认 `60s`
- 热点激活标记冷却时间：来自 `hot_group.cooling_seconds`
- 当前默认 `180s`
- 这两个 TTL 用来判断一个群在最近一段时间里是否持续活跃，以及避免热/冷状态频繁抖动

5. JWT 注销黑名单
- `auth:revoked:<token_id>`
- TTL 不是固定值，而是“这个 token 剩余的有效期”
- 这样 token 自然过期后，黑名单 key 也会一起消失

6. 限流窗口
- 注册、登录、发消息、文件上传这些限流 key 都会设置窗口 TTL
- 例如当前默认配置里：
  - 注册窗口 `3600s`
  - 登录窗口 `300s`
  - 发消息窗口 `60s`
  - 文件上传窗口 `300s`
- 这类 key 依赖的是 `INCR + EXPIRE`

7. 分片上传会话
- multipart 上传会话元数据和 part 列表都放在 Redis
- TTL 来自 `storage.multipart_session_ttl_minutes`
- 当前默认 `60min`
- 这样上传中断太久时，会话会自动清理，不会一直留在 Redis 里

如果面试官继续追问我怎么概括，我通常会说：

- 在线状态和限流是“状态型 TTL”
- 资料缓存和热群页是“性能型 TTL”
- JWT 黑名单和分片上传会话是“生命周期型 TTL”

### Q23.2：Redis 缓存主要缓存了哪些内容？

**答：**

当前主要缓存了：

- 用户资料
- 群资料
- 群成员列表
- 联系人关系
- 热群消息页

另外还有一些偏状态类的数据：

- 用户在线状态
- 在线设备连接
- JWT 黑名单
- 热群状态

这套设计里，Redis 更偏：

- 读优化
- 在线状态
- 短 TTL 热点数据

消息事实本身还是在 MySQL。

### Q23.3：为什么在线状态用 `Hash + ZSet`，而不是只用一个 key？

**答：**

因为我们需要同时解决两件事：

1. 查某个用户当前有哪些连接
2. 快速清理超时连接和统计在线数量

所以我拆成了两层：

- `Hash`
  - 适合按用户拿到所有连接详情
- `ZSet`
  - 适合按过期时间清理和做在线计数

这样查询和过期管理都比较顺。

### Q24：布隆过滤器是怎么用的？

**答：**

我加了内存布隆过滤器来优化用户 UUID 和群 UUID 的存在性判断。它放在缓存和数据库之前，用来挡掉一批明显不存在的请求，减少高频接口里的无效 Redis / MySQL 查询。

### Q25：Redis 是怎么支撑分布式在线状态的？

**答：**

每条 WebSocket 连接建立后，都会把 `user_uuid / connection_id / node_id / device / last_seen_at` 等信息写进 Redis presence。这样任意节点都可以知道某个用户当前有哪些在线连接、在哪些节点上。这个数据既支撑后台查看在线设备，也支撑节点间消息转发。

---

## 9. 分布式消息投递

### Q26：多节点部署下，消息怎么找到用户在哪个节点？

**答：**

这是 `Redis presence + PubSubRouter + Hub` 共同完成的。

流程是：

1. 用户 WS 建连后，把连接信息写到 Redis presence
2. 业务层要给某个用户发事件时，调用 `SendEventToUser`
3. `PubSubRouter` 先查这个用户有哪些连接、分别在哪些节点
4. 本节点的连接直接交给本机 `Hub`
5. 其他节点的连接通过 Redis Pub/Sub 发到对应节点
6. 对端节点再交给本机 `Hub` 写 socket

### Q27：Redis 在这里是做路由吗？

**答：**

更准确地说，它在做“在线连接目录”和“节点感知”。真正的入口路由是 Nginx，跨业务事件传播是 Kafka，Redis 这里主要解决的是“用户当前连在哪台节点上”。

### Q27.1：你们的 WebSocket 登录是怎么做的？

**答：**

当前 WebSocket 建连入口是：

- `GET /api/v1/ws`

鉴权支持两种 token 传递方式：

- URL query 里的 `token` / `access_token`
- `Authorization: Bearer <token>`

服务端在 upgrade 前会做三步校验：

1. 解析 access token
2. 通过 `TokenService.ResolveSession(...)` 校验 session
3. 再查用户状态，禁用用户不允许建连

通过之后才会 upgrade 到 WebSocket。

相关代码在：

- [auth.go](../../internal/transport/ws/auth.go)
- [handler.go](../../internal/transport/ws/handler.go)

### Q27.2：建连成功后，服务端会做什么？

**答：**

主要做三件事：

1. 创建 `Client`
- 里面会带：
  - `user_uuid`
  - `token`
  - `connection_id`
  - `device`
  - `device_id`
  - `user_agent`

2. 注册到本机 `Hub`
- 这样本机就能统一管理这条连接

3. 立即回一条 `connected` 事件
- 前端能拿到：
  - 当前用户 UUID
  - 当前用户连接数
  - 在线用户数

### Q27.3：你们的 WebSocket 保活是怎么做的？

**答：**

现在是前后端一起做保活：

前端：

- 每 `30s` 发送一次应用层 `ping`

服务端：

- `writePump` 周期性发 WebSocket `PingMessage`
- `readPump` 设置 `pongWait`
- 收到 `pong` 后续期读超时

所以现在同时有：

- 前端应用层心跳
- 服务端协议层 ping/pong

这样更容易发现断链和半开连接。

相关代码在：

- [useWebSocket.ts](../../frontend/src/composables/useWebSocket.ts)
- [client.go](../../internal/transport/ws/client.go)

### Q27.4：WebSocket 重连是怎么做的？

**答：**

前端现在用了一个指数退避策略：

- 断开后自动重连
- 重连间隔按 `1s, 2s, 4s, 8s...` 增长
- 最大退避到 `30s`

如果是用户主动退出登录，就不会继续自动重连。

这样既能应对网络瞬断，也能避免服务异常时高频打爆后端。

### Q27.5：重连后怎么保证未确认消息不丢？

**答：**

这次我把这条链也补齐了：

- 前端发送消息时，先生成 `client_message_id`
- 在收到 `chat.sent` 之前，这条消息会保存在本地 `pendingOutboundMessages`
- WebSocket 重新连上后，会把这批未确认消息按原来的 `client_message_id` 再发一次

服务端这边则通过：

- `(sender_uuid, client_message_id)` 联合唯一键

来做入口幂等，所以重发不会变成两条消息。

### Q27.6：如果 ACK 丢了，但消息已经进了服务端链路，会怎么样？

**答：**

这是 `client_message_id` 设计里很重要的一个场景。

比如：

- 请求已经被某个节点接受
- Kafka 也可能已经收到了 `send_requested`
- 但返回给客户端的 `chat.sent` 丢了

这时前端会继续把这条消息保留在 pending 集合里，等到下一次重连后按同一个 `client_message_id` 重发。

服务端会把它识别成同一个发送动作，避免变成重复消息。

### Q27.7：为什么现在没有在服务端保存更细的重连同步状态？

**答：**

当前我采用的是一个更轻量的方案：

- 在线阶段靠 WS push
- 重连阶段靠会话和消息补拉
- 未确认上行消息由前端本地 pending 集合负责重发

这个方案更适合现在项目阶段，复杂度可控，也已经能覆盖：

- 断线重连
- ACK 丢失后的重发
- 多节点下入口幂等

如果后面要做更强的多端同步，再往“服务端维护更细同步位点”的方向演进会更自然。

### Q27.8：你们服务端是怎么做优雅退出的？

**答：**

我们现在把优雅退出分成四步，重点是先停接入，再收尾后台链路。

1. 主进程监听退出信号
- 在 [main.go](../../cmd/services/core/main.go) 里由服务入口配合 Runtime 监听 `SIGINT` 和 `SIGTERM`

2. 先停止 HTTP 接入
- 收到信号后调用 [Shutdown](../../internal/services/core/server/server.go)
- 底层走 `http.Server.Shutdown(ctx)`
- 这样新的 HTTP 请求和新的 WebSocket 握手不会再进入

3. 主动关闭现有 WebSocket 连接
- HTTP 层收口后，会调用 [CloseAll](../../internal/transport/ws/hub.go)
- 给现有连接发送 `server_shutdown` 事件，再做注销
- 客户端能感知到服务正在退出，并走自己的重连逻辑

4. 最后停止后台组件
- 再由 [runtime.go](../../internal/bootstrap/embedded/runtime/runtime.go) 里的 `Runtime.Close()` 按顺序关闭：
  - outbox relay
  - Kafka consumer
  - Kafka publisher
  - Redis Pub/Sub router

这个顺序是有意设计的。我们的消息主链路是“先落库，再 through outbox / Kafka 分发”，所以退出时我会先把接入停掉，再把后台组件收干净，尽量给 in-flight 的链路留出完成空间。

如果面试官继续追问，我会补一句：

- 直接 `os.Exit(0)` 虽然简单，但会把 HTTP、WS、Kafka consumer 和 outbox relay 一起硬切断
- 相比这种方式，优雅退出更适合线上部署和滚动重启，用户体验和数据链路都会稳很多

---

## 10. 文件消息与对象存储

### Q28：文件消息是怎么设计的？

**答：**

我把“文件上传”和“文件消息发送”拆成了两步：

1. 先上传文件到 MinIO，同时把元数据落到 `uploaded_files`
2. 再发送一条文件类型消息，消息里引用这个 `file_id`

这样消息系统只关心文件引用，不关心二进制上传过程，职责更清楚。

### Q29：为什么单独需要 `uploaded_files` 表？

**答：**

因为它承接的是文件资源元数据，包括：

- object key
- 上传者
- 原始文件名
- 文件大小
- content type

这样发送文件消息时就可以校验“这个文件是否属于当前发送者”，下载时也能基于这张表做对象定位和鉴权。

### Q30：文件下载为什么要走服务端鉴权？

**答：**

因为如果直接暴露对象存储直链，会绕过业务权限，也不利于做有效期控制。现在的链路是：

- 客户端请求 `/api/v1/files/:file_id/download`
- 服务端校验当前用户是否有权限访问这条文件消息
- 校验是否过期
- 再返回短时签名链接

这样更符合 IM 场景。

### Q31：为什么后来又做了分片上传？

**答：**

因为普通表单上传在稍大一点的文件下会被 Nginx 或应用层体积限制拦住，而且体验也比较一般。后来我接了 MinIO multipart，把大文件分成多个 part 上传，最后再 complete 合并。前端会根据文件大小自动切换直传和分片上传。

---

## 11. 管理后台与可观测性

### Q35：项目有没有后台能力？

**答：**

有基础版，目前有：

- 管理员总览接口 `GET /api/v1/admin/overview`
- 管理员用户列表
- 管理员修改用户状态

它现在更像一个后台能力起点，后面还可以继续补消息检索、用户在线会话查看、群管理、outbox 诊断这类开发者后台能力。

### Q36：日志与文件落盘做了什么？

**答：**

项目支持日志落盘和按日期切割，我还在启动阶段补了 MySQL、Redis、Kafka 等初始化成功日志，方便联调和排障。

---

## 12. 性能与压测

### Q37：你是怎么发现消息延迟问题的？

**答：**

我用 WebSocket 双端联调和 `k6` 压测追了一条消息的完整链路，发现单聊 ACK 和收包延迟稳定在 1 秒和 2 秒量级。继续排查后定位到是 Kafka writer 的默认批等待和 consumer 的 `MaxWait` 导致固定延迟。

### Q38：后来怎么优化的？

**答：**

主要做了两件事：

- 把 Kafka writer 调成低延迟参数，例如很小的 `BatchTimeout`
- 把 consumer 的 `MaxWait` 从 500ms 降低到 10ms 级别

优化后单聊 ACK 从约 `1006ms` 降到 `2ms` 左右，对端收包从约 `2042ms` 降到 `28ms` 左右。

### Q39：你做过哪些压测？

**答：**

做过：

- 单聊延迟测试
- 100 并发 WebSocket 建连
- 500 人群消息压测
- 冷群与热群模式对比压测

也把服务打进 Docker，在限制 CPU 和内存的情况下跑过性能验证。

---

## 13. 设计取舍

### Q40：Message Store 和 Sync Store 如何分工？

**答：**

`messages` 按 `conversation_key + message_seq` 保存会话历史，承担 Message Store；`user_sync_inbox` 按 `user_uuid + sync_seq` 保存用户同步 Timeline，承担 Sync Store。Conversation State 保存最近消息和 `read_seq`，设备侧通过 Sync Cursor 增量消费 Inbox。普通群使用成员级投影，热群保留高水位加 `notify + pull`，以控制写扩散。

### Q41：为什么先做 outbox，而不是马上做 inbox？

**答：**

Outbox 保证 Message Store 的事实写入与 `message.created` 事件发布之间可恢复；Sync Projector 再以幂等方式写入 User Inbox。两者职责分离后，消息事实、同步状态和投递重放可以分别验证和回滚。

### Q42：singleflight 为什么只放在热群 pull 路径？

**答：**

因为它擅长解决“同一进程内、同一个 key 的并发重复请求”。热群场景里大量客户端会同时拉同一个群、同一个 after_id、同一个 limit，这个点收益最明显。消息发送主链路每条消息都是独立 key，就不适合滥用 singleflight。

### Q42.1：你们说的“推拉结合”具体是怎么做的？

**答：**

当前这套推拉结合，主要体现在热点群消息同步上。

我把它拆成两条路径：

1. 推
- 服务端先通过 WebSocket 给在线客户端发一个轻量事件
- 热群里发的是：
  - `group.message.notify`
- 这条通知里只带：
  - `group_uuid`
  - `latest_message_id`
  - 一些轻量元信息

2. 拉
- 客户端收到 `notify` 后
- 再调用：
  - `GET /api/v1/messages/group/:group_uuid?after_id=...`
- 按游标把这一小段新消息补拉回来

所以它的语义是：

- 先推一个“这个群有新消息了”的信号
- 再由客户端按位点去拉正文

### Q42.2：为什么不直接全推，或者全拉？

**答：**

因为两种极端方式各有问题。

如果全推：

- 大群里每条消息都要把完整正文广播给很多在线成员
- WS 扇出和节点压力会很高

如果全拉：

- 客户端要频繁轮询
- 延迟和无效请求都会比较差

所以推拉结合更像是在两者之间取平衡：

- 用推保证实时感
- 用拉控制正文分发成本

### Q42.3：你们现在在哪些场景用了推拉结合？

**答：**

当前最典型的是热群：

- 冷群：完整 `chat.message` push
- 热群：`group.message.notify + pull`

另外从更宽一点的角度看，系统整体也有类似思路：

- 在线时优先 push
- 断线、离线、重连后再 pull 补齐

所以这套项目里，推拉结合既体现在热群链路，也体现在整体的消息补偿思路里。

### Q42.4：推拉结合后，怎么避免很多客户端同时拉爆后端？

**答：**

这块我做了三层控制：

1. 客户端 pull 节流
- 收到很多 `notify` 时，不会每条都立刻拉
- 会做一个短时间窗口合并

2. 服务端 `singleflight`
- 同一台节点上，相同的：
  - `group_uuid + after_id + limit`
- 会合并成一次真实回源

3. Redis 短 TTL 页缓存
- 热群增量页会先走短 TTL 缓存
- 这样同一波补拉里很多请求都能直接命中缓存

所以推拉结合不是只做一个 `notify + pull` 就结束了，后面还要继续把补拉这条链路做稳。

### Q42.5：singleflight 在这里具体起什么作用？现在用在什么地方？

**答：**

它在这里的作用可以概括成一句话：

- 把同一台节点上的重复回源请求合并成一次真实查询

当前它主要用在：

- 热群增量补拉路径
- 对应的方法是：
  - `ListGroupMessagesAfter(...)`

场景是这样的：

- 热群里很多客户端会同时收到同一个 `group.message.notify`
- 然后几乎同时发：
  - `group_uuid + after_id + limit`
 这组参数相同的补拉请求

如果不处理：

- 同一台节点会把同一页消息重复打到 MySQL

我现在的做法是：

1. 先查 Redis 短 TTL 页缓存
2. 如果 miss，再进 `singleflight`
3. 同 key 的并发请求只让一个真正查库
4. 其他请求复用这次结果

我这里用的 key 大致是：

- `group_pull:<group_uuid>:<after_id>:<limit>`

所以它解决的是：

- 热群场景下的单机内读放大

### Q42.6：那 singleflight 能解决多节点重复查询吗？

**答：**

不能，它解决的是单进程内的并发重复请求。

如果现在是三节点部署：

- node1 上的相同请求能合并
- node2 上的相同请求也能各自合并

但 node1 和 node2 之间不会因为 singleflight 自动共享结果。

所以我们后面才又叠了一层：

- Redis 短 TTL 页缓存

这两层一起配合后，才更适合现在的热群补拉场景。

### Q43：为什么文件上传同步做，而消息发送走 Kafka？

**答：**

文件上传本身是强交互操作，客户端需要立刻知道文件到底有没有成功写入对象存储、有没有拿到 `file_id`。消息发送后面天然带着会话更新、在线投递、AI 等衍生动作，很适合用 Kafka 解耦。所以我把“资源创建”和“消息事件”分开了。

---

## 14. 可能继续追问的问题

### Q44：你觉得当前项目还有哪些明显可以继续优化的点？

**答：**

我会重点说这几项：

- 群会话更新的写放大还比较重
- 分布式投递目前还是逐用户查 Redis presence，可以继续按节点聚合
- 开发者后台还缺消息检索、在线会话排障、outbox 诊断
- AI 目前主要是单聊助手，后面可以继续做总结、建议回复、文件问答

### Q45：如果让你再做一轮重构，你会优先动哪里？

**答：**

我会优先做三件事：

1. 继续优化群消息路径，减少 Conversation 的 fan-out 写放大
2. 把分布式投递改成更明显的按节点聚合发送
3. 补开发者后台和运行时观测，方便排障和压测分析

---

## 15. 面试时的回答建议

### 16.1 回答顺序建议

建议尽量按这个顺序回答：

1. 项目目标
2. 技术栈
3. 核心业务能力
4. 一到两个关键技术设计
5. 一个你真实排查和优化过的问题

### 16.2 不要只报菜名

尽量不要只说：

- 用了 Kafka
- 用了 Redis
- 用了 MinIO

更好的说法是：

- Kafka 用来解耦 `send_requested -> created -> conversation update -> delivery`
- Redis 用来做在线状态、缓存、限流、热点群检测
- MinIO 用来承接文件消息的对象存储和分片上传

### 16.3 推荐重点讲的三个亮点

如果时间有限，优先讲：

1. 消息链路 + outbox
2. 热点群 `notify + pull`
3. AI 助手作为特殊用户接入现有消息体系

这三点最能体现你对 IM、异步架构和工程取舍的理解。

---

## 16. Kafka 深挖题

### Q46：你们当前 Kafka 的 topic 是怎么命名的？

**答：**

当前统一走：

- `topic_prefix + "." + event_name`

默认前缀在配置里是：

- `topic_prefix: dipole`

所以实际 topic 形态是：

- `dipole.message.direct.send_requested`
- `dipole.message.direct.created`
- `dipole.message.group.send_requested`
- `dipole.message.group.created`
- `dipole.conversation.direct.read`
- `dipole.group.created`
- `dipole.group.updated`
- `dipole.group.members.added`
- `dipole.group.members.removed`
- `dipole.group.dismissed`
- `dipole.contact.friend.deleted`
- `dipole.session.force_logout`

同时还会有一套配对的 `.retry` topic，用于消费失败后的重试。

### Q47：你们当前 Kafka 的分区和副本是怎么设置的？

**答：**

当前这套开发和演示环境里：

- Kafka 是单 broker
- topic 自动创建
- 核心 topic 当前都是：
  - `PartitionCount = 1`
  - `ReplicationFactor = 1`

我实际核对过的几个核心 topic，包括：

- `dipole.message.direct.send_requested`
- `dipole.message.direct.created`
- `dipole.message.group.send_requested`
- `dipole.message.group.created`
- `dipole.conversation.direct.read`

当前都只有一个分区。

这个配置适合当前项目阶段：

- 链路更容易观察
- 顺序性更直观
- 本地联调和压测成本低

如果要上更高吞吐和更高可用，后面会往：

- 多 broker
- 关键 topic 多 partition
- 副本数大于 1

这条方向演进。

### Q48：为什么现在没有把 message topic 配成多个 partition？

**答：**

当前项目阶段我优先保证：

- 链路清晰
- 顺序好理解
- 排障简单

现在 topic 单分区时，消息链路更容易验证，也更适合把注意力集中在：

- outbox
- 热群 `notify + pull`
- Conversation 更新
- 分布式在线路由

如果直接上多 partition，马上就会引入新的权衡：

- 同一会话如何稳定落到同一分区
- 消费组并发后的顺序边界
- 热点会话与负载均衡之间的平衡

这部分我后面会在吞吐进一步提升时再处理。

### Q49：Kafka message key 是什么？起什么作用？

**答：**

当前 `PublishEvent` 时都会传 `key`。

常见做法是：

- 消息事件：`message.UUID`
- 群事件：`group.UUID`
- 会话已读：目标用户或目标会话相关主键

不过我现在的 writer 使用的是：

- `LeastBytes` balancer

所以当前 `key` 的作用更多是：

- 业务标识
- 调试定位
- 为后续可能的按 key 路由保留接口

当前它还没有承担“强制同 key 落同分区”的职责。

### Q50：如果以后要把 Kafka topic 扩成多 partition，你会怎么做？

**答：**

我会优先按“有顺序要求的业务键”来定路由，而不是盲目平均。

例如：

- 单聊消息：按 `conversation_key`
- 群聊消息：按 `group_uuid`
- 已读回执：按 `target_uuid` 或 `conversation_key`

这样可以把“同一会话相关事件”的顺序尽量收在同一 partition 内。

与此同时我会做两件事：

1. 把生产端 balancer 调整成按 key 生效的方式
2. 明确消费侧对乱序和重复的幂等策略

### Q50.1：Kafka 的 topic 和分区现在是手动建的吗？如果以后要正式化，你会怎么收？

**答：**

当前这套环境里，topic 主要还是自动创建，目的是先把链路跑通、把行为验证清楚。

如果后面进入更正式的环境，我会把这块收成显式治理：

- 核心 topic 由部署脚本或 IaC 显式创建
- 明确每个 topic 的：
  - partition 数
  - replication factor
  - 保留时间
  - 清理策略
- 把业务关键 topic 和 retry / dead-letter topic 一起管理

这样做的好处是：

- 环境之间更一致
- 不会因为自动创建把默认参数带偏
- 吞吐、顺序、保留策略都更容易审计和调优

### Q50.2：你们 Kafka 消费者向 broker 提交 ACK / offset 是自动的还是手动的？

**答：**

当前是手动提交。

我在消费者里用的是：

- `FetchMessage(...)` 拉消息
- handler 全部执行成功后，再调用 `CommitMessages(...)`

也就是说，只有当这条消息对应的业务处理完成后，我们才会把 offset 提交给 broker。

这套语义更接近：

- 先处理
- 再确认消费成功

这样做的好处是：

- 进程中途挂掉时，这条消息还有机会重新被消费
- 和我们现在的 `outbox + 幂等落库` 更容易配合

代价是：

- consumer 需要自己处理重复消费
- 所以消费侧幂等一直是我后面要继续补强的一块

### Q50.3：项目里怎么保证消息时序性？

**答：**

我会把时序性分成三层来讲。

1. 当前 Kafka 主题是单分区
- 这让同一个 topic 内的消费顺序更直观
- 对当前项目阶段非常友好

2. 业务上尽量把“消息先持久化，再做后续分发”
- 先落 `messages`
- 再通过 `message.created` 去更新会话、在线推送、触发 AI
- 这让链路里的先后关系更稳定

3. 真正需要精细顺序时，依赖会话维度的业务键
- 单聊看 `conversation_key`
- 群聊看 `group_uuid`
- 当前历史翻页和增量补拉也主要按消息表的自增 `id` 走

所以现在的顺序保证，更像是：

- Kafka 侧单分区提供一个清晰的消费顺序
- MySQL 侧的消息事实和自增 `id` 提供会话内的读取顺序
- 业务链路约束“先落库，再分发”

### Q50.4：如果以后用 Kafka 来保证更强的时序性，你会怎么做？

**答：**

如果后面 topic 扩成多分区，我会明确把“会话内顺序”和“全局吞吐”拆开看。

更稳的做法是：

- 单聊事件按 `conversation_key` 作为 key
- 群聊事件按 `group_uuid` 作为 key
- 让同一个会话的事件稳定进入同一分区

这样可以保证：

- 会话内顺序尽量收敛在同一 partition

同时还要配合两件事：

1. 生产端 balancer 要改成按 key 生效
- 当前 `LeastBytes` 更偏负载均衡
- 后面如果更重视顺序，就要换成更适合按 key 路由的策略

2. 消费侧要承认“Kafka 只能保证分区内顺序”
- 不能把它理解成全局顺序
- 所以后续仍然要依赖：
  - 会话维度 key
  - 消费幂等
  - 读取侧游标或序列

如果面试官让我一句话总结，我会说：

- 当前阶段，我们用“单分区 + 先落库再分发 + 会话维度读取顺序”来保证时序
- 后面如果扩分区，就用业务 key 把会话内顺序收进同一 partition

### Q50.5：如果 Kafka 扩成多个 partition，怎么继续保证有序？

**答：**

这时候我会把“有序”从全局有序收成会话内有序。

对 IM 来说，真正重要的是：

- 同一个单聊会话内消息顺序稳定
- 同一个群会话内消息顺序稳定

而不是要求所有消息在整个系统里全局严格有序。

具体做法我会这么收：

1. 用业务键做分区路由
- 单聊按 `conversation_key`
- 群聊按 `group_uuid` 或群会话 key

2. 让生产端按 key 路由真正生效
- 当前的 `LeastBytes` 更偏负载均衡
- 如果要保顺序，就要换成更适合按 key hash 的 balancer

3. 承认 Kafka 只保证分区内顺序
- 所以设计上不要让同一个会话散到多个 partition

4. 下游处理继续按会话维度建模
- Conversation 更新
- 在线投递
- 热群通知
- 历史补拉

都围绕“会话内顺序”来设计

如果继续追问，我会再补一句：

- 多 partition 解决的是吞吐扩展
- 会话维度 key 解决的是顺序边界
- 热点群如果特别极端，还要继续靠热群降级和 pull 模式来削峰

### Q50.6：怎么保证“同一个会话的事件稳定进入同一分区”？依靠 Kafka 的什么特性实现？

**答：**

这里核心依靠的是 Kafka 的两件事：

1. message key
2. 按 key 选 partition 的路由策略

Kafka 在生产消息时，可以带一个 `key`。只要生产端使用的是按 key 生效的分区策略，同一个 key 就会稳定映射到同一个 partition。

放到 IM 里，做法就是：

- 单聊事件把 `conversation_key` 作为 key
- 群聊事件把 `group_uuid` 或群会话 key 作为 key

这样同一个会话相关的事件，都会被 hash 到同一个 partition 里。

而 Kafka 本身保证的是：

- 单个 partition 内消息顺序有序

所以组合起来就是：

- 先用业务 key 把同一会话收进同一 partition
- 再利用 Kafka 的“分区内有序”保证会话内顺序

如果继续往实现层说，我们项目当前还差一步：

- 现在 writer 用的是 `LeastBytes`

这个策略更偏负载均衡，`key` 现在更多是业务标识，还没有承担“强制同 key 落同分区”的职责。

如果后面要真正做多 partition 顺序收口，我会把生产端改成：

- 使用按 key hash 的 balancer

这样：

- 同一个 `conversation_key`
- 或同一个 `group_uuid`

才能稳定进入同一个 partition。

### Q50.11：除了 Kafka 分区，你们有没有在业务层保证消息的时序性？

**答：**

有，而且这层很重要。

我现在不会把时序性完全压在 Kafka 分区上，业务层也做了几层配合：

1. 先落库，再分发
- 当前消息链路里，先消费 `send_requested`
- 再把消息写入 `messages`
- 然后通过 `outbox` 发 `message.created`
- 再去更新会话、在线投递、触发 AI

这让链路里的先后关系比较清晰，能避免“后续副作用先发生，消息事实还没稳定”的问题。

2. 会话内读取顺序优先使用 `message_seq`，兼容请求继续使用消息表自增 `id`
- 历史翻页
- 增量补拉
- 热群 `after_seq`

新链路使用 `conversation_key + message_seq`；旧客户端路径仍依赖 `messages.id`

所以读取侧的顺序基线并不只靠 Kafka，而是靠：

- 消息事实先落库
- 再按会话内 `message_seq` 做历史和增量读取，旧路径按 MySQL 自增 `id` 读取

3. 业务上把顺序语义收成“会话内顺序”
- 单聊关注 `conversation_key`
- 群聊关注 `group_uuid`

这样顺序边界更明确，也更适合后面多分区扩展。

4. 出现重复消费时尽量用幂等把副作用收住
- 发送入口幂等
- 落库幂等
- outbox 可靠发布

这能减少“重复事件把时序感打乱”的副作用

如果让我一句话总结，我会说：

- Kafka 分区提供的是分区内顺序
- 业务层通过“先落库再分发 + 会话内读取顺序 + 幂等”把消息时序进一步收紧

### Q50.7：你们的消费者组是怎么设计的？为什么这么设计？

**答：**

当前这套实现里，Kafka consumer group 比较直接：

- `groupID = clientID + "-consumer"`

也就是说，同一套应用节点会共享同一个消费者组。

比如现在三节点部署时，只要它们使用同一个 Kafka `clientID` 配置，它们就会以同一个 consumer group 身份去消费对应 topic。

这样设计的原因主要有三个：

1. 同一条消息只希望被这一组应用实例处理一次
- 例如 `message.direct.created`
- 我们希望它被某个节点消费后完成会话更新、在线投递等后续逻辑
- 不希望每个节点都各消费一遍

2. 方便横向扩展
- 节点数增加时，可以直接通过 consumer group 分摊 topic 的分区消费压力

3. 和当前项目阶段匹配
- 现在 topic 还是单分区为主
- 用一个统一的 consumer group，语义清晰，排障也简单

### Q50.8：那当前 topic 还是单分区，共享一个 consumer group 有什么实际效果？

**答：**

当前单分区下，效果其实很直观：

- 一个 topic 的这一个 partition
- 在同一个 consumer group 里，同一时刻只会被一个节点上的 consumer 拿到

这意味着：

- 处理顺序更稳定
- 不会出现三个节点对同一条 Kafka 消息同时各处理一遍

这也解释了为什么我们现在虽然是三节点部署，Kafka 消费侧仍然能保持比较清晰的处理语义。

### Q50.9：如果以后扩成多分区，消费者组会怎么工作？

**答：**

扩成多分区后，consumer group 的作用会更明显：

- 同一个 group 内，不同 partition 可以分配给不同节点去并行消费

这样：

- 吞吐可以随着 partition 数和节点数一起扩起来

但这里也要注意一个边界：

- consumer group 帮我们解决的是“谁来消费”
- 会话内有序还要靠“同一个会话稳定落到同一个 partition”

所以后面真正的组合应该是：

- consumer group 负责并行扩展
- message key + partition 路由负责顺序边界

### Q50.10：为什么不把不同业务 handler 拆成不同 consumer group？

**答：**

这个问题要分 topic 看。

当前我们更多是：

- 不同业务事件进不同 topic
- 同一个 topic 上注册该 topic 对应的 handler

比如：

- `message.direct.send_requested`
- `message.direct.created`
- `group.dismissed`

它们天然已经把阶段拆开了。

如果把同一个 topic 再拆成很多 consumer group，确实可以让每组都收到一份消息，但也会带来新的复杂度：

- 重复消费语义会更多
- 运维面更大
- 时序和副作用边界更难收

对我们现在这套项目来说，当前设计更偏：

- 先按事件类型拆 topic
- 再让同一组应用实例共享同一个 consumer group

这样链路更清楚，也更适合当前阶段。

---

## 17. MySQL 深挖题

### Q51：你们当前 MySQL 里最核心的表有哪些？

**答：**

最核心的是这几张：

- `users`
- `messages`
- `conversations`
- `contacts`
- `contact_applications`
- `groups`
- `group_members`
- `uploaded_files`
- `ai_call_logs`
- `outbox_events`

其中最重要的两张是：

- `messages`：消息事实库
- `conversations`：用户视角的会话索引

### Q52：你们给 `messages` 加了哪些关键索引？

**答：**

当前比较关键的有：

- `uuid` 唯一索引
- `(conversation_key, id)` 复合索引
- `(target_type, target_uuid, id)` 复合索引
- `(target_type, sender_uuid, id)` 复合索引
- `(file_id, message_type, sent_at)` 复合索引
- `(sender_uuid, client_message_id)` 联合唯一索引

这些索引分别服务于：

- 历史消息翻页
- 离线消息补拉
- 文件消息鉴权
- 入口幂等

### Q53：为什么要给 `messages` 增加 `(sender_uuid, client_message_id)` 唯一键？

**答：**

这是为了解决多节点场景下的入口重试幂等。

场景是：

- A 发消息到 node1
- node1 已经成功把请求推进后续链路
- 但返回给 A 的 ACK 丢了
- A 重试时打到 node2

如果没有客户端幂等键，这两次请求会被当成两条不同消息。

所以我引入了：

- 客户端生成的 `client_message_id`
- MySQL 联合唯一键 `(sender_uuid, client_message_id)`

这样服务端就能把“同一个发送动作”的重复请求收敛成一条消息。

### Q53.1：你们接口的幂等性做了吗？

**答：**

做了一部分，而且是按接口语义分层处理的。

当前最核心的幂等点有三层：

1. 发送入口幂等
- 通过：
  - `client_message_id`
  - `(sender_uuid, client_message_id)` 联合唯一键
- 解决的是：
  - ACK 丢失
  - 跨节点重试
  - 前端重发

2. 消息落库幂等
- 通过：
  - `messages.uuid` 唯一约束
  - duplicate 后回查已有消息
- 解决的是：
  - `send_requested` 重复消费

3. 事件可靠发布
- 通过：
  - `outbox`
  - `messages + outbox_events` 同事务
- 解决的是：
  - 消息已落库但 `created` 事件丢失

所以现在的幂等重点主要落在：

- 发送入口
- 落库
- 事件发布

如果面试官让我一句话概括，我会说：

- 高价值写接口重点做强幂等
- 状态覆盖型接口尽量做成天然幂等
- 查询接口保持简单

### Q53.2：除了消息发送，还有哪些接口天然带幂等或接近幂等？

**答：**

有一些接口天然比较接近幂等，比如：

- 清未读
- 更新备注
- 更新拉黑状态
- 更新群资料

因为这些操作本质上都是“把某个状态改成目标值”，重复执行结果基本一致。

另外像：

- Conversation upsert
- 群会话初始化

这类写法我也尽量做成了 `upsert` 语义，减少重复事件对数据的一次次放大。

### Q53.3：那消费侧幂等做完了吗？

**答：**

还没有完全做完，这也是后面明确要继续补的一块。

当前已经比较稳的是：

- 消息发送入口幂等
- 消息落库幂等
- outbox 可靠发布
- AI 触发日志的唯一键约束

但消费侧如果某些事件被重复消费，仍然可能带来：

- Conversation 未读数重复变化
- WS 重复推送
- 某些副作用重复触发

所以后面我会考虑继续补一层：

- `event_id + consumer_name` 维度的消费去重

### Q53.4：为什么没有把所有接口都强行做成统一幂等模型？

**答：**

因为不同接口的语义不一样。

像消息发送这种入口：

- 业务上非常怕重复
- 用户也会因为网络抖动重试

这类接口非常值得做强幂等。

但像普通查询接口、资料读取接口，本身就是安全的读操作；像某些状态更新接口，天然已经接近幂等。把所有接口都强行套成一套“请求幂等 token”模型，复杂度会明显升高，收益不一定匹配。

所以我现在的策略是：

- 对高价值、易重复、代价高的写操作重点做幂等
- 对天然幂等的更新接口保持简单

### Q54：`message_id` 和 `client_message_id` 的职责分别是什么？

**答：**

- `client_message_id`
  - 由客户端生成
  - 用来标识“同一个发送动作”
  - 服务于入口幂等

- `message_id`
  - 由服务端生成
  - 用来标识消息事实对象
  - 服务于 MySQL、Kafka、outbox、日志、前端下游引用

我把这两个 ID 分开之后，链路职责会更清楚。

### Q55：为什么服务端消息 ID 要迁到雪花算法？

**答：**

当前我把服务端 `message.UUID` 迁到了雪花算法，主要是想得到：

- 全局唯一
- 趋势递增
- 更适合日志排查和后续演进

同时它还是一个纯服务端生成的消息事实 ID，不依赖客户端。

这和 `client_message_id` 的分工刚好互补：

- 客户端负责发送动作幂等
- 服务端负责消息事实 ID

### Q56：你们有没有用到 MySQL 锁？

**答：**

有两类：

1. 显式锁
- outbox relay 的 claim 使用了 `FOR UPDATE SKIP LOCKED`

2. 隐式行锁
- 消息落库
- Conversation upsert
- 群成员变更
- 未读数清零

其中最值得关注的热点其实不是显式锁本身，而是：

- 群消息导致的 Conversation 高频 upsert

### Q57：如果面试官问“为什么还没分表”，你怎么答？

**答：**

我会说消息分表是明确的后续演进方向，但当前优先级更高的是：

- 群会话写放大
- 热群 pull 路径优化
- 消费侧幂等
- 后台排障能力

当前消息表已经通过复合索引把：

- 历史翻页
- 离线补拉
- 文件鉴权

这些关键查询先收住了。等热点路径和幂等层更稳后，再做消息分表会更合适。

---

## 18. 场景题

### Q58：如果某个用户反馈“我明明发成功了，对方没收到”，你会怎么排查？

**答：**

我会按链路顺序查：

1. 先查客户端是否拿到了 `chat.sent`
2. 看服务端有没有收到 `chat.send`
3. 查 Kafka 有没有 `message.*.send_requested`
4. 查 `messages` 表里是否已经落库
5. 查 `outbox_events` 是否已经发布 `message.*.created`
6. 查 Conversation 是否更新
7. 查接收方是否在线、连在哪个节点
8. 查 Redis Pub/Sub 或本机 Hub 是否把 WS 事件发出

这也是为什么我后面一直在补开发者后台和排障能力。

### Q59：如果 ACK 丢了，客户端重试打到了另一个节点，会不会重复发消息？

**答：**

如果没有额外设计，这种场景确实会造成重复。

我现在的处理是：

- 客户端生成 `client_message_id`
- 重试时复用它
- 服务端用 `(sender_uuid, client_message_id)` 做唯一约束

这样即使重试打到不同节点，也会被收敛成同一个发送动作。

### Q60：如果热群突然从 500 人扩大到 5000 人，你最先担心什么？

**答：**

我最先担心三件事：

1. Conversation 写放大
2. 热群 notify 后的大量补拉洪峰
3. 分布式投递和 Redis presence 查询压力

正文 push 这一层已经通过 `notify + pull` 降了不少，但会话写和补拉读放大还会继续放大。

### Q61：如果 Kafka 宕机了，当前系统会怎么样？

**答：**

项目里确实有一条“不走 Kafka 的直接链路”，但它是在 Kafka 未启用时使用的。

例如消息发送这层，代码里是：

- 如果 `events == nil`
- 就直接本地落库
- 不再走 `message.*.send_requested`

所以从代码结构上看，系统是支持“无 Kafka 模式”运行的。

但当前运行态如果 Kafka 已经启用，而 broker 在运行中宕机，系统不会自动切回本地直发模式。

这时候：

- 发送请求推进 Kafka 会失败
- 新消息链路会受影响

但已经成功落库并进入 outbox 的消息，不会因为短时 Kafka 异常而丢失：

- outbox 记录还在
- relay 恢复后还能继续发

所以这里要区分：

- 入口阶段 Kafka 是否可用
- 落库后事件发布是否可恢复

### Q61.1：那除了 Kafka，现在项目里有没有直接发送的链路？

**答：**

有，当前代码里保留了本地直接处理路径。

最典型的是消息发送：

- Kafka 未启用时
- `MessageService` 会直接走本地持久化
- 不发布 `send_requested`

这条链路更适合：

- 单节点运行
- 本地开发
- 关闭 Kafka 的简化场景

所以现在的代码结构其实是：

- Kafka 开启时：走异步事件链路
- Kafka 关闭时：走本地直接链路

### Q61.2：Kafka 宕机时，系统能不能继续运行？

**答：**

要分“进程是否能启动”和“消息主链路是否还能正常发送”两层来看。

1. Kafka 配置关闭时
- 可以正常运行
- 会走本地直接链路

2. Kafka 配置开启，但 broker 启动时就不可用
- 当前初始化会失败
- 服务进程起不来

3. Kafka 配置开启，服务已经启动后 broker 再宕机
- 服务进程还能活着
- 读接口、部分非 Kafka 依赖功能还能工作
- 但新消息发送、依赖 Kafka 的异步事件会失败

所以当前这套系统更准确的描述是：

- 支持无 Kafka 模式运行
- 但 Kafka 开启后，还没有做“运行中自动降级回本地直发”的切换

如果后面要把可用性再往前推，我会考虑两种方向：

- 启动时支持显式降级模式
- 运行中把消息发送切到本地 fallback，但这一步要把时序、幂等和 outbox 边界一起重新设计清楚

### Q61.3：那你们现在除了 Kafka，还有没有直接发送的链路？

**答：**

有，不过这条链路当前主要服务“Kafka 关闭”的运行模式。

最典型的是消息发送这层：

- `MessageService` 里如果发现 `events == nil`
- 就不会发布 `message.direct.send_requested` / `message.group.send_requested`
- 而是直接走本地持久化

也就是说，当前系统代码里同时保留了两种模式：

1. Kafka 开启
- 走异步事件链路

2. Kafka 关闭
- 走本地直接链路

这让项目在单节点、本地开发、禁用 Kafka 的场景下还能正常运行。

### Q61.4：那既然有本地直接链路，Kafka 宕机后为什么不自动切过去？

**答：**

因为“启动时显式关闭 Kafka”和“运行中 Kafka 突然宕机”是两种复杂度完全不同的场景。

启动时如果明确配置 Kafka 关闭：

- 整个系统会在一致的前提下走本地模式

但如果运行中 Kafka 突然宕机，再临时把消息发送切到本地直发，就要重新处理很多边界：

- 时序边界会变化
- 消费链路会分叉
- outbox 语义会变化
- 某些副作用可能一部分走 Kafka，一部分走本地

所以当前这套实现选择的是：

- 支持无 Kafka 模式
- 但 Kafka 开启后，还没有做“运行中自动切回本地直发”的降级切换

这个取舍更保守，但边界更清楚。

### Q62：如果某个消费 handler 重复消费，会有什么问题？你怎么防？

**答：**

风险主要有：

- Conversation 未读数被重复加
- 在线消息重复推送
- AI 助手重复回复

当前消息落库这一层已经有幂等，但消费侧统一幂等还值得继续补。我下一步会考虑做：

- `event_id + consumer_name` 的消费去重表

先覆盖：

- Conversation 更新
- 在线投递
- AI 触发

### Q63：如果面试官问“为什么现在不做批量落库”，你怎么回答？

**答：**

我会说批量落库有价值，但当前不急着上，原因有三点：

1. 当前更大的瓶颈在群会话写放大和热群补拉
2. 批量落库会引入新的延迟和崩溃丢批权衡
3. 现有链路是“先持久化，再分发”，批量化要同时处理好 outbox 和 offset 语义

所以现阶段我更愿意先把正确性和热点路径做稳。

---

## 19. 设计演进题

### Q64：你们现在有“消息同步库”吗？

**答：**

按现代 IM 那套“两库模型”来说：

- 我们已经有“消息存储库”
  - 对应 `messages`
- 还没有独立的“消息同步库”
  - 当前同步更多是从 `messages` 事实库推导出来

Redis 现在主要承担的是：

- 缓存
- presence
- 热群页缓存
- 限流和状态层

它还没有承担“每个接收端一个 Timeline”的同步库职责。

### Q64.1：Redis 现在开了 RDB 和 AOF 吗？

**答：**

当前运行环境里，两者都开着。

我实际核对过 Redis 配置：

- `appendonly = yes`
- `save = 3600 1 300 100 60 10000`

也就是说：

- AOF 开启，用来增强写操作持久化
- RDB 的定时快照也保留着

对我们当前项目来说，这样的组合比较稳，原因是 Redis 这里承担了：

- 在线状态
- 缓存
- 限流和热点状态
- 节点间 Pub/Sub 辅助

这类数据里有一部分是可再生的，比如缓存和热点页；也有一部分在节点重启后保留会更顺，比如黑名单和状态类 key。所以当前选择是同时保留 RDB 和 AOF。

### Q64.2：既然开了 AOF 和 RDB，为什么还不把 Redis 直接当消息同步库？

**答：**

因为“Redis 能持久化”和“它已经成为消息同步库”是两回事。

要成为文章里说的消息同步库，核心不是只把数据放到 Redis，而是要真正具备：

- 每个接收端一条独立 Timeline
- 明确的同步位点
- 固定生命周期和回收策略
- 多端补拉的一致性模型

我们现在的 Redis 还没有承担这层职责。当前它更偏：

- 缓存
- 在线状态
- 热点页
- 分布式节点间轻量转发

所以即使 Redis 开了持久化，也还没有变成一个“接收端 Timeline 库”。

### Q65：Timeline 模型在你们当前系统里怎么体现？

**答：**

现在主要体现在两层：

1. 会话 Timeline
- 由 `messages + conversation_key` 承担

2. 会话摘要索引
- 由 `conversations` 承担

也就是说，我们当前已经有“会话 Timeline”，但还没有“接收端 Timeline”。

### Q66：如果以后真的要引入消息同步库，你会怎么推进？

**答：**

我不会一上来就把整个消息系统推翻。

更稳的推进方式是：

1. 先在逻辑层引入 `user timeline / sync inbox`
2. 先对部分同步场景接入
3. 保留现有 `messages` 作为事实库
4. 逐步把多端同步从“读事实库派生”迁到“读接收端 Timeline”

这样风险更可控，也更适合在现有系统上逐步演进。

### Q66.1：如果以后真要用 Redis 来做消息同步库，你会怎么设计？

**答：**

我会先把它定义成“逻辑上的接收端 Timeline”，再去选具体 Redis 结构。

一个比较自然的第一版会是：

- 每个用户一条同步 Timeline
- 用递增 `seq` 表示同步位点
- Timeline 中只保留最近一段时间的数据，比如 7 天或 30 天

如果用 Redis 落，我会优先考虑两层结构：

1. `ZSet`
- key 类似：
  - `sync:user:<user_uuid>`
- member 放：
  - `message_uuid` 或 `sync_item_id`
- score 放：
  - 递增 `seq`
- 这样很适合按位点范围拉取

2. `Hash` 或独立对象 key
- 存同步项详情
- 例如：
  - `sync:item:<id>`
- 里面放：
  - `message_uuid`
  - `conversation_key`
  - `sender_uuid`
  - `target_uuid`
  - `sent_at`
  - 必要的消息摘要

客户端同步时就走：

- 先拿本地最新 `seq`
- 再从 `sync:user:<user_uuid>` 里按分数拉 `> seq` 的项目
- 再批量取详情

如果以后要支持清理，我会加：

- TTL
- 或按 score 的定期裁剪

这样 Redis 这层就更像“同步收件箱”，而 MySQL `messages` 继续做“消息事实库”。

### Q66.2：如果真这么做，为什么还要保留 MySQL 的 `messages`？

**答：**

因为两者解决的是两类问题：

- Redis 同步库更适合接收端补拉和短期同步
- MySQL `messages` 更适合历史漫游、审计、后台检索和长期存储

所以更合理的分工通常是：

- Redis：同步 Timeline
- MySQL：消息事实 Timeline

这样才能把“多端同步”和“历史消息存储”同时做好。

---

## 20. 更底层的面试题

### Q67：为什么聊天长连接你选择 WebSocket，而不是纯 HTTP？

**答：**

聊天系统对连接模型的要求很明确：

- 服务端要能主动推消息
- 延迟要低
- 连接要尽量长时间复用
- 已读、撤回、在线状态这类事件要能实时下发

相比纯 HTTP 轮询，WebSocket 更适合这些需求，因为：

- 建连后是全双工
- 服务端可以主动推送
- 不需要频繁建立 TCP 连接
- 头部开销更低

如果完全用 HTTP，也能做，比如：

- 短轮询
- 长轮询
- SSE + HTTP API

但对 IM 这种双向实时交互场景，WebSocket 的整体体验和成本结构会更合适。

### Q68：你了解 WebSocket 的底层机制吗？

**答：**

了解，核心可以分成三层：

1. 握手阶段
- 先走 HTTP/1.1 请求
- 带 `Upgrade: websocket`
- 带 `Connection: Upgrade`
- 服务端校验后返回 `101 Switching Protocols`

2. 建连后阶段
- 底层还是同一条 TCP 连接
- 之后通信不再是 HTTP 报文，变成 WebSocket frame

3. 帧机制
- 文本帧、二进制帧、ping/pong、close
- 客户端发往服务端的 frame 需要 mask
- 服务端回客户端的 frame 不需要 mask

我们项目里虽然没有自己手写底层帧解析，但从接入层设计上是围绕这个模型来用的：

- Gin 提供握手入口
- Hub 管理连接
- ping/pong 和连接保活交给 WS 层

### Q69：如果不用 WebSocket，能不能用 UDP 设计一个聊天系统？

**答：**

能设计，但复杂度会明显上升。

UDP 的优点是：

- 轻量
- 时延低
- 不要求连接建立成本

但聊天系统对这些能力也有很强要求：

- 顺序
- 重传
- 去重
- 拥塞控制
- 连接保活
- NAT 穿透
- 加密与安全

如果基于 UDP 做 IM，往往要自己补很多可靠传输层逻辑，最后会比较接近“在 UDP 上再做一套简化版 TCP/QUIC 语义”。

对我们这个项目阶段来说，WebSocket over TCP 的工程收益更高。

如果以后是：

- 游戏实时状态同步
- 音视频信令
- 弹幕

这类低时延容忍丢包的场景，再考虑 UDP/QUIC 会更合理。

### Q70：WebSocket 和 TCP 的关系是什么？

**答：**

WebSocket 是应用层协议，它通常跑在 TCP 之上。

也就是说：

- TCP 负责可靠传输、顺序、重传、拥塞控制
- WebSocket 负责在这条连接上提供全双工消息语义

所以很多 IM 场景里我们说“WebSocket 实时通信”，底层真正保证数据可靠性的还是 TCP。

### Q71：你们系统里用到了分布式锁吗？

**答：**

当前主链路里没有用 Redis 分布式锁。

我们现在主要依赖的是：

- MySQL 唯一约束
- MySQL 事务
- `FOR UPDATE SKIP LOCKED`
- Outbox
- 业务幂等

最典型的显式锁使用场景是 outbox relay 的 claim：

- 多个节点同时扫 `outbox_events`
- 通过 `FOR UPDATE SKIP LOCKED` 抢一批任务
- 一条任务只会被一个 worker 认领

这是数据库层的并发控制，不是 Redis 分布式锁。

### Q72：那为什么现在没有上 Redis 分布式锁？

**答：**

因为当前几个核心问题，用数据库唯一约束和事务更直接、更稳：

- 消息去重
- Outbox claim
- 关系创建幂等

如果用 Redis 锁，通常还要处理：

- 过期时间
- 锁续约
- 持锁节点崩溃
- 锁和数据库最终状态之间的一致性

所以当前阶段，我优先使用：

- DB 唯一键
- DB 事务
- 消费幂等

如果以后遇到：

- 跨系统全局互斥任务
- 某些不能只靠唯一键表达的临界区

再引入 Redis 分布式锁会更合适。

### Q73：如果让你做消息加密，你会怎么设计？

**答：**

我会分三层看：

1. 传输加密
- 先保证 `HTTPS / WSS`
- 这层保护链路不被中间人窃听

2. 服务端存储加密
- 对敏感字段做库内或应用层加密
- 比如文件元数据、部分高敏消息内容

3. 端到端加密
- 如果要做真正高等级隐私保护，会引入 E2EE
- 服务端只转发密文，不掌握明文

如果在我们当前项目里做第一版，我会先从：

- 强制 HTTPS/WSS
- 文件对象私有化
- 对敏感消息内容预留应用层加密接口

这一步开始。

如果进一步做 E2EE，我会考虑：

- 每个会话维护会话密钥
- 客户端本地加解密
- 使用双棘轮一类的密钥演进思路

但那会明显改变：

- 服务端搜索能力
- AI 能力
- 管理后台排障能力

所以要看产品目标来决定。

### Q74：如果做端到端加密，会影响你们现有的哪些能力？

**答：**

影响会很明显，尤其是这几项：

- 服务端无法直接看消息正文
- 基于消息内容的 AI 能力会受限
- 搜索、审计、敏感词处理会变难
- 后台排障难度会上升

所以如果上 E2EE，就要提前对齐：

- 哪些会话需要 E2EE
- 哪些功能可以退化
- 客户端密钥管理怎么做

### Q75：Kafka 消息积压了怎么办？

**答：**

我会先分三层排查：

1. 看积压发生在哪个 topic
- `send_requested`
- `created`
- 群事件
- 已读回执

2. 看是生产太快，还是消费太慢
- 生产速率
- 消费速率
- 消费失败重试
- 某个 handler 是否变慢

3. 看慢点落在哪层
- MySQL 落库
- Conversation 更新
- Redis presence 查询
- 热群补拉
- AI 调用

处理手段一般会是：

- 扩消费者并发
- 把热点 handler 拆轻
- 降级某些非关键副作用
- 增加 partition
- 缩短慢 SQL
- 对热群走更强的降级策略

### Q76：如果 `message.created` 积压最严重，你会先怀疑哪里？

**答：**

我会优先怀疑：

- Conversation 写放大
- 群消息 fan-out
- Redis presence 查询过多
- 热群下重复 notify / pull
- AI 触发拖慢同类消费链路

因为 `created` 后面挂着的副作用最多，它最容易把系统真实瓶颈暴露出来。

### Q77：如果让你设计 Kafka 积压时的降级策略，你会怎么做？

**答：**

我会先保核心、降边缘：

优先保：

- 消息事实落库
- 基本 ACK
- 基本消息送达

可降级项：

- AI 回复异步延后
- 热群更激进地只发轻通知
- 某些后台统计异步延后
- 非关键审计链路延后

这类系统在高峰时最重要的是：

- 核心消息不丢
- 主链路别阻塞

### Q78：如果面试官问“你们当前这套 WebSocket 有没有可能被 HTTP/2 或 gRPC 替代”，你怎么答？

**答：**

可以讨论，但要看目标。

如果是：

- 浏览器端 IM
- 双向实时交互

WebSocket 依然是更普遍、兼容性更好的选择。

gRPC 更适合：

- 服务间 RPC
- 强 schema
- 内部调用链

HTTP/2 server push 在浏览器侧并没有形成主流 IM 方案。对我们当前项目来说，WebSocket 是更自然的选型。
