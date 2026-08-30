# Dipole IM 项目学习、简历与面试

本文件只描述 Dipole 的即时通信、存储、同步、微服务与文件数据面。Agent Runtime 请使用 [Dipole Agent 项目材料](DIPOLE-AGENT-LEARNING-AND-INTERVIEW.md)。

## 1. 使用规则

只依据代码、测试、基准报告和归档运行记录描述能力。状态使用“已验证”“默认关闭”“规划中”；隔离环境结果必须注明环境，不能外推为生产结论。

### 滚动维护契约

消息、Sync、存储、服务边界、文件上传或性能结论变化时，同步更新本文件的简历句、演示、证据、限制和下一步。事实细节以 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 与对应运行手册为准。

### 能力卡片模板与索引

| 能力 | 状态 | 证据 |
| --- | --- | --- |
| Message / Conversation / Sync Timeline | 已验证 | [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md) |
| SQLC 与渐进式微服务 | 已验证 | [服务边界](../architecture/SERVICE-BOUNDARIES.md) |
| MinIO Multipart 与恢复上传 | 已验证（隔离 Remote GPU） | [平台演进计划](../architecture/PLATFORM-EVOLUTION-PLAN.md) |
| Cassandra、Elasticsearch、C++ 数据面切流 | 默认关闭 / 规划中 | [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) |

#### Sync Timeline 与可靠消息

- **状态：** 已验证
- **简历句：** 设计基于 Conversation Timeline、User Sync Timeline 与 Device Cursor 的多端增量同步模型，并用事务型 Outbox 和 Kafka 投影解耦消息持久化与下游消费。
- **对外表述：** 历史消息、用户会话状态和设备同步进度是三个不同维度；分别建模后，分页、未读、重连与投影重试都有稳定语义。
- **演示：** 展示同一会话的 conversation sequence、用户 inbox sync sequence 和设备 cursor 的推进，再模拟重复事件并核对幂等结果。
- **证据：** [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md)、[Kafka 事件契约](../data/KAFKA-EVENT-CONTRACT.md)、[Sync Service](../architecture/SYNC-SERVICE.md)。
- **追问：** “Kafka 能否直接充当离线同步队列？” 用户同步需要长期域状态、权限与设备 cursor，Kafka consumer offset 只服务内部消费。
- **限制：** Cassandra 主读与旧兼容路径退役仍受 shadow、对账、回滚和真实观察窗口门禁约束。
- **下一步：** 完成真实 Web Sync 观察窗口后，再评估 Cassandra hydration 主读与旧 Offline 兼容窗口收敛。
- **复核条件：** 修改事件 schema、seq、cursor、投影所有权或默认读路径时。

#### S3 Multipart 数据面

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 基于 MinIO S3 Multipart Upload 实现分片、校验、暂停恢复、Redis/对象存储对账与生命周期清理，并将预签名直传保持为可回切候选路径。
- **对外表述：** Core 管理会话、授权和完成事务；对象存储承载大文件数据面。浏览器只对网络异常及 `408`、`429`、`5xx` 有界重试，确定性 `4xx` 立即失败。
- **演示：** 运行 `scripts/smoke-minio-multipart.sh` 与 `scripts/smoke-minio-multipart-restart.sh`，展示乱序分片、替换、重启续传、完成校验和重复 Abort。
- **证据：** `frontend/src/upload/multipartUpload.ts`、`scripts/smoke-minio-multipart.sh`、[架构债务 AD-055](../architecture/ARCHITECTURE-DEBT.md)。
- **追问：** “为什么预签名直传仍默认关闭？” 还需浏览器级断网、代理故障、告警路由和切流回退的同版本证据。
- **限制：** 默认权威路径仍为 relay；预签名直传未作为生产默认。
- **下一步：** 完成跨网络故障矩阵与回切演练。
- **复核条件：** 修改分片大小、URL TTL、重试策略、对象存储或上传默认策略时。

## 2. 一句话定位

Dipole IM 是一个以 Go 为核心的现代即时通信后端：通过 SQLC、MySQL、Kafka、Redis、MinIO 和 WebSocket 实现可靠消息、Timeline 多端同步与渐进式微服务拆分。

## 3. 简历描述

```text
Dipole IM | Go, sqlc, MySQL, Kafka, Redis, MinIO, WebSocket
- 设计消息幂等、Transactional Outbox 与 Kafka 事件投影，将消息事实、会话状态和用户 Sync Timeline 分层建模，支持多端 cursor 增量同步。
- 以 Core、Gateway、Message、Sync、Search 服务边界推进渐进微服务化，保留 embedded 兼容路径，并通过版本化 gRPC 契约、Shadow 与回滚门禁控制迁移风险。
- 基于 MinIO S3 Multipart Upload 实现大文件分片、恢复、对账和生命周期清理；预签名直传保持默认关闭并具备回切路径。
```

不要将 Cassandra 主读、Elasticsearch 默认搜索或 C++ 实时数据面写为已上线能力。

## 4. 现场介绍

### 60 秒版本

Dipole IM 的核心是把消息事实、用户会话摘要和多端同步流分开。消息经过鉴权、幂等和持久化后与 Outbox 在同一事务提交，Kafka 再驱动会话、Sync、搜索和实时投递。服务拆分遵循渐进式策略：先稳定接口、数据所有权和回滚路径，再抽出独立部署单元。

### 3 分钟版本

先讲数据模型：Conversation Timeline 用于历史，Conversation 保存用户侧首页状态，User Inbox Timeline 与 Device Cursor 提供增量同步。消息投递以 Outbox 收敛落库和发布间隙，投影通过幂等处理重复事件。群聊区分普通 fanout 和热点群 notify + pull。存储与部署演进使用 SQLC、版本化 migration、gRPC 契约、shadow 和可执行回退，避免一次性迁移数据库与服务拓扑。

## 5. 可展开的工程故事

| 主题 | 取舍 |
| --- | --- |
| 三类序列 | Message ID 管唯一性，Conversation Seq 管会话顺序，Sync Seq 管用户增量消费。 |
| Outbox | 事务内记录待发布事件，consumer 用幂等与重试处理至少一次投递。 |
| 热点群 | notify + pull 控制成员级写扩散，接受客户端补拉复杂度。 |
| SQLC | 关键查询、索引和锁语义显式可审查，领域规则保留在 application 层。 |
| Multipart | 对象存储负责字节，Core 负责会话与授权，先 relay 后直传。 |

## 6. 高频追问

### 为什么不直接拆成很多微服务？

先稳定消息语义、数据所有权和回滚边界，再抽取有独立负载或部署需求的模块。服务数量本身不能解决可靠性问题。

### 为什么不用 `messages.id` 同时完成历史、未读和同步？

三个问题的分区和推进主体不同。混用会把会话分页、用户未读和多设备重连耦合在一起。

### 深入问答

见 [Dipole IM 深入问答](INTERVIEW-QA.md)。

## 7. 学习路线

1. 手画 Message、Conversation、Inbox 与 Device Cursor 的数据流。
2. 用一次重复事件解释 Outbox、幂等和投影收敛。
3. 从性能报告中选择一组数据，说明环境、指标与局限。
4. 说明默认关闭的 Cassandra、Elasticsearch 与 C++ 路径需要哪些切流证据。

## 8. 面试前检查

1. 复核 [README](../../README.md) 的服务和启动入口。
2. 选择一条 Timeline/Outbox 故事和一条 Multipart 故事。
3. 从 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 说明一个未完成风险与回退方案。
