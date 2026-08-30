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
| Contact 目录读取 | 已验证（隔离 Remote GPU 前端门禁） | `frontend/src/components/ContactDirectory.vue` |
| Group 目录读取 | 已验证（隔离 Remote GPU 前端门禁） | `frontend/src/components/GroupDirectory.vue` |
| File 目录读取 | 已验证（隔离 Remote GPU 前端门禁） | `frontend/src/components/FileDirectory.vue` |
| Device Security 会话控制 | 已验证（Remote GPU Node 22）；浏览器执行待补 | `frontend/src/components/DeviceSecurity.vue` |
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
- **追问：** “为什么预签名直传仍默认关闭？” 直传默认切流须先通过版本化 evidence receipt：24 小时同版本窗口、直传样本、错误与延迟阈值、clear alert、relay 回切演练和独立 reviewer；当前尚未取得共享环境 receipt。
- **限制：** 默认权威路径仍为 relay；预签名直传未作为生产默认。
- **下一步：** 完成跨网络故障矩阵、Prometheus/Alertmanager 路由验收与共享环境 receipt，再审阅默认策略变更。
- **复核条件：** 修改分片大小、URL TTL、重试策略、对象存储或上传默认策略时。

#### File 目录读取

- **状态：** 已验证（隔离 Remote GPU 前端门禁）
- **简历句：** 为文件元数据建立 owner-scoped 的只读目录：SQLC 使用稳定文件 UUID cursor 查询，Core 通过版本化 gRPC 传递低敏投影，HTTP 与 Vue 端都阻止对象键、存储 URL、校验值和上传会话进入目录。
- **对外表述：** 文件数据面与目录读取分开治理。浏览器获取目录后仍须为每个下载请求重新获得授权 URL，避免把长期存储位置作为列表数据暴露或缓存。
- **演示：** 访问 `/files`，展示文件名、大小、内容类型和创建时间；令目录请求失败后，页面进入不可用状态且清空旧条目；点击下载时才请求单文件授权链接。
- **证据：** `db/queries/file.sql`、`internal/services/core/application/application.go`、`api/proto/dipole/core/v1/core.proto`、`internal/gateway/http/file_handler_test.go`、`frontend/src/api/files.test.ts`、`frontend/src/components/FileDirectory.test.ts` 与 `design/exports/file-directory-review.png`；Remote GPU Node 22 在 `a29d9927` 通过 38 个测试文件、157 项测试、typecheck 和 production build。
- **追问：** “为什么不把 MinIO URL 放进列表响应？” 目录是低敏投影，下载 URL 具有短期授权语义；每次下载重新授权能让所有权和过期策略在服务端集中执行。
- **限制：** 当前目录不提供上传、删除、分享、跨浏览器视觉回归或预签名上传默认切流；上传继续从会话编辑器进入既有 Multipart 路径。
- **下一步：** 独立设计删除、分享或文件关联写路径的授权、审计和回退契约。
- **复核条件：** 修改 `/api/v1/files` projection、Core File RPC、文件 cursor、下载授权或任何文件写入口时。

#### Device Security 会话控制

- **状态：** 已验证（Remote GPU Node 22）；浏览器执行待补
- **简历句：** 为设备会话控制建立低敏 projection 和显式确认动作：公共 HTTP 仅返回登出所需连接标识、粗粒度设备信息与时间，并通过稳定 Device ID 将“登出其他设备”与全设备退出区分。
- **对外表述：** 会话管理页不能把 Redis Presence 的节点、IP 和原始 UA 直接暴露给客户端。动作从认证上下文派生当前稳定设备，批量撤销只处理其他连接，页面确认后重新读取权威列表。
- **演示：** 访问 `/devices`，展示当前设备与其他设备；选择单设备或全部其他设备后先进入确认态，再展示列表刷新。读取失败时旧列表清空。
- **证据：** `internal/dto/httpdto/session.go`、`internal/services/core/domain/session/session_service.go`、`frontend/src/api/devices.ts`、`frontend/src/components/DeviceSecurity.test.ts`、`frontend/e2e/device-security.spec.ts`、`design/exports/device-security-desktop-review.png`；Remote GPU Node 22 在候选切片通过前端 `40` 个测试文件、`162` 项测试、typecheck 和 production build。
- **追问：** “为什么不用 `logout-all`？” 该旧动作会撤销当前 Token 和所有连接，无法满足“保留当前设备”的产品语义；专用动作依据当前 `X-Device-ID` 排除同设备会话。
- **限制：** 当前只覆盖在线 Presence，会话历史、设备命名、跨浏览器视觉与真实跨节点踢出仍待验证；Remote GPU Playwright browser binary 下载未完成，因此三浏览器用例只完成发现，尚未执行。
- **下一步：** 在隔离 Remote GPU 环境执行 Node 22、Chromium/Firefox/WebKit 和 Redis Presence 交互验证，再决定是否开放设备命名或会话历史能力。
- **复核条件：** 修改设备投影字段、Device ID 语义、登出 API、Presence 存储或任一设备动作时。

#### Contact 目录读取

- **状态：** 已验证（隔离 Remote GPU 前端门禁）
- **简历句：** 为 IM 联系人目录建立受认证的只读投影页，对服务端响应执行严格 shape 校验，并在权威读取失败时清空旧数据，避免把陈旧关系状态显示为当前结果。
- **对外表述：** 目录读取与关系修改分开交付；先保证身份边界、响应解析和失败语义，再单独引入备注、拉黑、删除与申请处理的授权、审计和确认流。
- **演示：** 访问 `/contacts`，展示联系人别名与状态；断开目录请求后页面显示不可用状态和重试入口，且不保留上一次的联系人条目。
- **证据：** `frontend/src/api/contacts.ts`、`frontend/src/components/ContactDirectory.vue`、`frontend/src/components/ContactDirectory.test.ts`、`frontend/src/router/index.test.ts`；Remote GPU Node 22 前端门禁通过 `34` 个测试文件、`147` 项测试、typecheck 与 production build。
- **追问：** “为什么先做只读？” 联系人备注、拉黑和删除会改变用户关系，需要将权限、确认、审计和回退作为一个独立写路径切片验证。
- **限制：** 当前没有 Contact 写操作，也没有跨浏览器交互或视觉回归证据。
- **下一步：** 为每一类写操作定义 owner 授权、确认与失败恢复契约，再接入设计稿中的申请和安全状态。
- **复核条件：** 修改 `/api/v1/contacts` 投影、路由认证、联系人状态语义或新增任何写入口时。

#### Group 目录读取

- **状态：** 已验证（隔离 Remote GPU 前端门禁）
- **对外表述：** 群目录复用认证会话投影确定可见范围，再逐项读取群权威详情；页面不承载成员、资料或解散等写操作，热群仅表达 `notify + pull` 同步语义。
- **演示：** 访问 `/groups`，展示普通群、热群和已解散群的只读状态；断开任一权威读取后页面进入不可用状态且不会保留旧群条目。
- **证据：** `frontend/src/api/groups.ts`、`frontend/src/components/GroupDirectory.vue`、对应 Vitest 与 `design/exports/group-v1/`；Remote GPU Node 22 通过 36 个前端测试文件、152 项测试、typecheck 和 production build。
- **追问：** “为什么不直接在目录中做群管理？” 群管理会改变成员资格与历史边界，应将精确授权、确认、审计和失败回退作为独立写路径验证。
- **限制：** 当前按会话列表上限 50 派生群范围，未提供群管理写操作、分页聚合或跨浏览器视觉回归。
- **下一步：** 为可分页群范围定义专用只读 API，再独立设计和实现受授权的管理写路径。
- **复核条件：** 修改会话范围、`/api/v1/groups/:uuid` 投影、路由认证或增加群管理入口时。

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
