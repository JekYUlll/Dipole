# Dipole IM 面试问答

## 30 秒介绍

Dipole IM 是一个 Go 实现的实时通信后端。Gateway 负责 HTTP/WebSocket 接入和
连接路由，Message Service 负责消息事实与 Outbox，Sync Service 维护用户同步流和
设备游标，Search Service 提供权限感知的全文检索。Kafka 解耦消息事实与各类投影，
Redis 维护在线状态和热点群投递策略，MySQL/sqlc 保存事务性领域数据。

## 架构图

```text
Client -> Gateway -> Core / Message -> MySQL + Outbox -> Kafka
                    |                                |       |
                    v                                v       +-> Search Indexer -> Elasticsearch
             Redis Presence                    Sync Service
                                                    |
                                                    v
                                          Inbox Timeline + Device Cursor
```

## 高频问题

### 一条消息如何可靠发送？

客户端携带 Client Message ID。服务端先完成身份、关系或群成员校验，再在同一 MySQL
事务中写入消息和 Outbox 事件；Consumer 以事件 ID 与投影唯一键去重。Kafka 负责
可靠传递领域事件，消息、投影和外部副作用仍由应用层幂等键约束。

### 为什么需要 Outbox？

数据库提交和 Kafka 发布是两个独立系统。直接先后执行会出现“消息已落库但事件未发出”
或“事件已发出但消息回滚”的不一致窗口。Outbox 将待发布事件与消息事实一起提交，
Relay 可安全重试发布，Consumer 则用幂等处理重复事件。

### Conversation Timeline 与 Sync Timeline 分别解决什么问题？

Conversation Timeline 以会话 `seq` 表达历史消息顺序，服务于分页、漫游和定位。
Sync Timeline 以用户 Inbox 与设备 Cursor 表达“该设备还缺哪些消息”，服务于离线补拉
和多端同步。Read Seq 记录用户已读进度，未读状态可以由位置关系稳定恢复。

### Kafka offset 能替代用户同步游标吗？

不能。Kafka offset 属于消费组的基础设施状态，用户同步游标属于具体用户和设备的业务
状态。用户长时间离线、重装客户端或跨设备登录时，都需要由 Sync Timeline 提供独立、
可持久化的补拉位置。

### WebSocket、Redis 与 Kafka 如何分工？

WebSocket 管理客户端长连接和低延迟交付；Redis 保存用户在线节点、连接路由和热点群
状态；Kafka 承接服务间的持久化事件与异步投影。三者分别处理连接、实时状态和事件流，
使任一层重启时都能从业务事实恢复。

### 热点群为什么使用 notify + pull？

普通群可直接向成员投递完整消息。热点群若对每条消息执行全量扇出，会放大数据库写入、
跨节点路由和连接队列压力。Dipole 对热点群广播新序号通知，客户端按会话 Seq 增量拉取；
服务端用 `singleflight` 合并重复补拉，兼顾顺序和读放大控制。

### 如何保证搜索不会越权？

Search Indexer 负责构建索引，Search Service 不把索引命中直接视为授权结果。查询会带入
经认证的主体，Core 复核会话范围与成员关系后才返回结果。Agent 使用同一条 Capability
路径，因此模型无法通过 Tool 参数伪造其他用户身份。

### 为什么选择 sqlc？

消息、同步游标和事务写入依赖明确的 SQL 语义。sqlc 让查询、输入输出类型和数据库迁移
一同进入代码审查，减少手写 ORM 映射的隐式行为；跨服务时也更容易保持数据模型与查询
边界清晰。

### 大文件如何上传？

文件元数据由 IM 域管理，二进制内容写入 MinIO。Multipart Upload 将大文件拆分为多个
Part，可在网络中断后续传，完成时再合并对象并关联文件消息。下载仍通过 IM 的授权边界
确认访问权限。

## 追问：故障时发生什么？

| 场景 | 处理方式 |
| --- | --- |
| 客户端重试发送 | Client Message ID 返回既有消息或拒绝冲突写入 |
| Relay 重复发布 | Consumer 以事件 ID 和投影键去重 |
| Gateway 重启 | 客户端重连；Presence 过期后由新节点重新登记 |
| 设备离线 | 设备携带 Cursor 从 Sync Timeline 增量补拉 |
| 搜索索引延迟 | 消息事实和历史查询保持可用，索引异步追赶 |
| 热点群瞬时高峰 | 通知携带 Seq，客户端按需拉取并合并重复请求 |

## 讲解重点

面试时先画出“消息事实 + Outbox + Kafka + 投影”的主线，再解释双 Timeline 如何将
历史与设备同步拆开，最后以热点群和权限感知搜索展示性能与安全边界。不要承诺 Kafka
天然提供端到端 exactly-once；Dipole 的重复控制来自稳定幂等键、事务和投影约束。
