# Dipole IM 面试分册

## 项目定位

Dipole IM 是 Go 实现的实时通信后端，重点展示消息可靠性、连接管理、异步事件、分层存储、热点群优化和渐进式微服务化。

## 一句话架构

```text
Client -- HTTP / WebSocket --> Gateway
                                  |
                                  v
                         Core / Message Service
                           |             |
                           v             v
                     MySQL + sqlc      Kafka
                           |             |
                           v             +--> Conversation / Sync / Search / Agent consumers
                     Metadata         Redis Presence / MinIO / Cassandra / Elasticsearch
```

服务边界以 [SERVICE-BOUNDARIES.md](../architecture/SERVICE-BOUNDARIES.md) 和 `cmd/services/` 为准。embedded 路径用于兼容和回滚，独立服务路径通过 gRPC 和契约逐步接管。

## 消息发送链路

```text
WS chat.send
  -> Gateway 鉴权、限流、request/trace 注入
  -> Message Service 校验关系、群成员和消息类型
  -> message.*.send_requested
  -> Consumer: MySQL transaction(message + outbox)
  -> outbox relay: message.*.created
  -> Conversation / Sync / Search / Agent / Delivery projections
  -> chat.sent ACK and online delivery
```

`send_requested` 表示接入层接受了发送意图，`created` 表示消息事实已经落库并可以驱动后续投影。这个区分让重试和故障定位更清楚。

## 重点设计

### 消息事实与会话状态

Message 保存事实；Conversation 保存用户侧的最近消息、未读和排序摘要。首页会话列表不需要每次扫描消息事实表重新聚合。

### 双 Timeline 与游标

Conversation Timeline 用会话内 `seq` 组织历史；User Sync Timeline 用用户/设备游标组织离线补拉和多端增量同步。消息 ID 用于唯一性，`conversation_seq`、`read_seq` 和 `device_cursor` 用于顺序与恢复。

### 热点群

冷群可以完整 push；热点群发送轻量 `notify`，客户端携带序号进行增量 pull。服务端用 Redis 保存热点判断和在线状态，单机重复 pull 通过 `singleflight` 合并。该策略把写扩散和连接 fan-out 从同一条热路径拆开。

### 可靠性

- 入口使用 Client Message ID 等幂等键。
- 消息事实和 outbox 在同一数据库事务中提交。
- 消费者用稳定事件 ID、投影唯一键和重试边界避免重复副作用。
- Redis、Kafka、Cassandra、Elasticsearch 都有明确的数据所有权：消息事实和元数据仍由主存储负责。
- 默认路径切换前需要 shadow、故障证据、责任人批准和可执行回滚。

## Go 在 IM 中的使用

| Go 特性 | 在 Dipole 的用途 | 面试重点 |
| --- | --- | --- |
| goroutine | HTTP/WS 服务、Kafka worker、投影和后台生命周期 | 每个 goroutine 都有退出条件，使用 context 取消 |
| channel | 服务错误回传、worker 队列、关闭信号 | 有界队列背压，关闭顺序可观测 |
| `context.Context` | 请求、trace、超时、服务关闭和 Agent trusted context | 不把用户输入放进身份字段 |
| interface | Application Store、Service、RPC client、投递 sink | 先稳定边界再替换本地实现为 RPC |
| `sync.Once` | Snowflake/运行时初始化等一次性资源 | 避免并发初始化和重复副作用 |
| mutex/atomic | 连接队列、去重表、状态和计数器 | 明确锁范围，避免锁内网络调用 |
| `singleflight` | 热点群重复增量 pull 合并 | 降低读放大，保留请求超时 |
| `errgroup`/取消树 | 并行 worker 与优雅退出 | 任一关键 worker 失败可收敛服务状态 |
| `database/sql` + sqlc | 类型安全查询、事务和 Querier | SQL 是显式资产，生成代码与迁移版本化 |
| protobuf/gRPC | 跨服务接口、兼容 adapter 和错误边界 | proto 是契约，业务服务不共享数据库连接 |

## 高频问题

### 为什么用 Go 做 IM 后端？

Go 的 goroutine、channel、context 和标准网络库适合大量 I/O 连接与后台 worker；静态类型和 interface 让服务边界容易测试。真正的可靠性来自幂等、事务、事件契约和证据门禁，语言本身只提供实现基础。

### Kafka 和 WebSocket 各解决什么问题？

Kafka 负责服务间持久化事件流、消费组和重放；WebSocket 负责单个客户端连接和实时交付。Kafka 的 offset 不等于用户同步游标，用户离线同步仍由 Sync Timeline 承担。

### 为什么使用 sqlc？

SQL 查询和 schema 变更显式可审查，生成的 Go 类型减少手写映射错误，也更方便把 SQL 语义对齐到其他语言服务。Repository 通过接口暴露领域语义，sqlc 只位于数据访问实现层。

### 为什么 Cassandra 不直接替换 MySQL？

消息正文顺序存储和元数据事务是不同负载。先用 MySQL 保持可验证的事实源，再对 Cassandra 做回填、影子读、对账和灰度，能把存储迁移与服务拆分分开验证。

### 大群优化的代价是什么？

notify + pull 把即时完整 push 延迟转成客户端补拉和游标管理，客户端、Sync Timeline、权限校验和重试复杂度会上升。它适合热点群，普通群仍使用更简单的完整投递。

### 如何证明消息没有重复副作用？

说明稳定事件 ID、入口幂等键、outbox、投影唯一约束、投递 ID、重启重放测试和最终状态对账。不要只说“Kafka exactly once”，因为客户端 WebSocket 和外部副作用仍需应用层幂等。

## IM 现状边界

当前可直接展示的内容包括 Go 服务入口、Kafka/outbox、Redis presence、消息和会话链路、sqlc 数据访问、Cassandra/Elasticsearch 的独立投影与回滚材料。C++ realtime delivery 仍属于候选数据面，必须用 shadow/primary 状态和对应 benchmark 口径描述。
