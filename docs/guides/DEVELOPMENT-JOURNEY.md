# Dipole 开发历程

> 从即时通讯单体到 Governed Agent 微服务平台的演进叙事

---

## 目录

- [引言](#引言)
- [Phase 0 — 项目创世 (2026-02)](#phase-0--项目创世-2026-02)
- [Phase 1 — IM 核心构建 (2026-02 ~ 2026-04)](#phase-1--im-核心构建-2026-02--2026-04)
- [Phase 2 — 多节点 IM 与嵌入式 AI (2026-04)](#phase-2--多节点-im-与嵌入式-ai-2026-04)
- [Phase 3 — 平台规划与微服务拆分 (2026-08-26)](#phase-3--平台规划与微服务拆分-2026-08-26)
- [Phase 4 — Agent Runtime 与治理框架 (2026-08-27 ~ 2026-09-04)](#phase-4--agent-runtime-与治理框架-2026-08-27--2026-09-04)
- [Phase 5 — Governed Interactive Path (2026-09-04 ~ 2026-09-07)](#phase-5--governed-interactive-path-2026-09-04--2026-09-07)
- [设计决策与考量](#设计决策与考量)
- [时间线总览](#时间线总览)

---

## 引言

Dipole 起源于一个纯 Go 即时通讯 (IM) 单体应用，在约七个月的演进过程中，逐步成长为一个由 Temporal 编排、MCP (Model Context Protocol) 治理、gRPC 连接的微服务平台。它的核心使命从"能聊天"变成了"让 AI 助手以受治理的方式参与任何对话"。

本文记录这段旅程中的关键里程碑、架构决策及其背后的考量。

---

## Phase 0 — 项目创世 (2026-02)

```
2026-02-14  16bd26c2  Initial commit
2026-02-14  82a2aa50  Establish original project baseline
2026-02-25  6b32e1c9  init — 纯 Go 单体
```

初始代码极其精简：

```
cmd/server/main.go         # 唯一入口
docker-compose.yml         # MySQL + Redis + Kafka
go.mod
README.md
```

一个 `main.go`、一个 `docker-compose.yml`——这就是 Dipole 的全部。此时的目标很明确：先把最基本的服务端跑起来，验证 Go + MySQL + Redis + Kafka 的技术选型。

**设计考量：** 选择 Go 是因为其在服务端领域的成熟生态、出色的并发模型和编译型部署便利性。MySQL 作为关系型主存储，Redis 做实时状态，Kafka 做事件总线——这三件套从第一天就确定下来，后续的微服务拆分证明了这个基础设施选型的前瞻性。

---

## Phase 1 — IM 核心构建 (2026-02 ~ 2026-04)

```
2026-04-07  fe395b12  Bootstrap server config + persistence
2026-04-07  890f347c  User login + auth
2026-04-13  c5bd6942  Conversations + contacts
2026-04-13  32df3d72  Groups + Kafka event flow
```

在不到两个月的时间里，完成了 IM 的核心功能：

| 功能 | 技术要点 |
|------|----------|
| 用户认证 | JWT token，设备绑定 |
| 联系人 & 好友申请 | 双向好友关系 |
| 1v1 对话 | 确定性会话键 `direct:{sorted-uuids}` |
| 群组 | 群成员管理，群消息分发 |
| 消息 Kafka 事件 | `message.direct.created`、`message.group.created` |

**设计考量：** 会话键 (conversation key) 的设计是一个关键早期决策。`direct:` 前缀 + 排序后的 UUID 保证了同一对用户无论谁先发起，都映射到同一会话。`group:` 前缀 + groupUUID 使群会话键全局唯一。这个约定一直延续到 Agent 系统，成为权限作用域 (resource scope) 的基础。

---

## Phase 2 — 多节点 IM 与嵌入式 AI (2026-04)

```
2026-04-13  92c83005  First AI system — Eino-based assistant agent
2026-04-16  90984ba6  Redis Pub/Sub cross-node WebSocket routing
2026-04-16  e3a0b239  Vue 3 + Vite frontend
2026-04-18  08e026fb  Hot-group notify-pull flow
2026-04-19  294f79e6  Transactional outbox message delivery
2026-04-20  4f9b1e60  MinIO object storage + multipart upload
2026-04-21  db6b81b3  Hot-group batch notify + throttle
```

这一阶段有三个重要的架构里程碑：

### 1. 嵌入式 AI 助手 (Eino)

第一个 AI 系统以"嵌入式聊天机器人"的形态出现：代码位于 `internal/services/agent/legacy/`，直接在 Go 进程内运行。当用户发 DM 给助手时，Kafka 消费者触发 Eino provider 调用 LLM 并回复。

这个设计**有意为之**地简单——作为可行性验证，不需要独立进程、不需要审批流、不需要能力治理。它证明了"AI 参与对话"这个产品概念可行，但也暴露了嵌入式方案的局限：
- 无法独立扩缩 AI 负载
- 无法治理 AI 的行为（它可以做任何 Core 进程能做的事）
- 无法持久化任务状态或支持人机交互审批

### 2. 多节点 WebSocket

Redis Pub/Sub 实现跨节点消息推送，为后续 Gateway 服务拆分奠定基础。

### 3. Transactional Outbox

消息持久化与 Kafka 发布在同一个数据库事务中，保证"消息一旦存入 DB 就一定会被发布到 Kafka"。这个 exactly-once 语义成为后续所有事件驱动流程的可靠性基石。

**此时的系统形态：** 一个单进程 Go IM 服务器，带 Kafka 事件、Redis 实时推送、MinIO 文件存储，以及一个嵌入式 Eino AI 聊天机器人。前端是 Vue 3 SPA。

---

## Phase 3 — 平台规划与微服务拆分 (2026-08-26)

```
2026-08-26  ca672553  Platform evolution roadmap
2026-08-26  3231d0c9  Agent Runtime design doc
2026-08-26  6a80d3a9  M2: GORM → sqlc 迁移
2026-08-26  eedc35f3  M3: gRPC contracts
2026-08-26  4caac188  M4: Gateway 提取
2026-08-26  cace09d1  M4: Message Service 提取
2026-08-26  c8664b2c  M6: Minimum microservice topology
```

这是 Dipole 历史上最密集的架构变革日。**一天之内**完成了从单体到微服务的完整拆分：

### 拆分策略：Strangler Fig Pattern

没有尝试一次性重写，而是采用"绞杀者模式"逐步提取：

```
阶段    里程碑    提取内容
M1      代码组织     internal/ 按服务边界重组
M2      GORM → sqlc  ORM 替换为编译时安全的 SQL
M3      gRPC 契约    定义 proto 服务边界
M4      Gateway      HTTP/WS 边缘与业务逻辑分离
M4      Message      消息事实 & outbox 独立
M6      完整拓扑     第一个 docker-compose.microservices.yml
```

### 拆分后的服务

| 服务 | 职责 | 通信方式 | 数据所有权 |
|------|------|----------|------------|
| **Core** | 身份、社交图谱、会话状态、文件、认证 | HTTP + gRPC | Users, groups, contacts, files, conversations |
| **Gateway** | 客户端边缘：REST/WS、限流、代理 | HTTP/WS + gRPC clients | 无（Redis 状态） |
| **Message** | 消息命令路径：发送、历史、幂等性、序号 | gRPC + Kafka | messages, outbox_events |
| **Sync** | 用户收件箱时间线、设备游标、群同步 | gRPC + Kafka | sync_inbox, checkpoints |

**设计考量：**

1. **Gateway 无数据库** —— Gateway 只做协议转换和路由，不持有任何持久状态。这意味着它可以随时水平扩容，且不会成为数据一致性的瓶颈。

2. **嵌入式回退** —— Core 保留了 `embedded` 模式，可以在一个进程内运行所有服务。这是一个关键的安全网：微服务拆分出问题时，可以一键回退到单体。

3. **为什么在 Core 中保留 Agent gRPC？** —— Agent 的数据（定义、任务、审批等）存在 Core 的 MySQL 中，Agent gRPC 服务器 (`AgentCapabilityService`) 也由 Core 托管。这是有意的：Agent 数据在逻辑上属于 Agent 领域，但物理上共享 Core 数据库可以避免引入第六个微服务。TS Agent Runtime 通过 gRPC 调用 Core 来执行所有受治理操作。

### 并行：存储架构演进

```
A1  存储契约      Cassandra timeline schema
A2  集群基线      Kafka/MySQL/Redis cluster configs
A3  Cassandra     Shadow projector + backfill
A4  Elasticsearch Search indexer + backfill
A5  Kafka 分区    Topic 分区策略
```

Cassandra 和 Elasticsearch 作为可选存储层引入，为大规模部署的读扩展做准备。Search 和 Search Indexer 两个服务也在此阶段添加。

---

## Phase 4 — Agent Runtime 与治理框架 (2026-08-27 ~ 2026-09-04)

```
2026-08-27  17217a82  Scaffold TypeScript Agent Runtime
2026-08-27  40723c52  Event subscriptions
2026-08-28  09f2c8ca  MCP tool system
2026-08-28  d0d733c6  Agent MCP + Temporal process
2026-08-28  4caa5cae  Governed long-term memory
2026-09-01  f54975a8  Approved interactive task delivery
2026-09-02  74cbbeb0  Owner-scoped Definition control
2026-09-02  3a64c1fd  Subscriptions bound to owner execution
2026-09-04  393619c7  Temporal durable runtime enabled by default
```

这是 Dipole 从"IM + chatbot"到"governed agent platform"的质变阶段。

### 为什么选择 TypeScript？

Agent Runtime 选择 TypeScript 而非 Go，有几个核心理由：

1. **Temporal SDK 生态** —— Temporal 的 TypeScript SDK 最为成熟，workflow 代码可以直接用 async/await 写，比 Go 的 activity-based 模型更自然。
2. **AI SDK 生态** —— OpenAI / Anthropic / Vercel AI SDK 都以 JavaScript/TypeScript 为一等公民。
3. **MCP 协议** —— Model Context Protocol 的参考实现和社区工具都是 TypeScript。
4. **关注点分离** —— Go 适合做基础设施（消息、同步、存储），TypeScript 适合做编排逻辑和 LLM 交互。

### 核心治理概念

这一阶段引入了 Dipole 的核心治理模型，每个概念都有明确的设计意图：

#### Definition（定义）
- Agent 的"权限证书"，由 owner 创建和管理
- 版本化，可撤销，有时间有效期
- 包含 permissions（权限列表）和 scopes（资源作用域）
- **为什么需要？** 将 Agent 的权限从代码级约束提升到数据级声明，使非开发者也能管理 Agent 行为边界

#### Capability（能力）
- 命名的原子操作（如 `conversation.read`、`message.assistant_reply.send`）
- 分为 read/write/destructive 三个风险等级
- **为什么需要？** 细粒度的能力模型让审计和治理成为可能——你可以精确知道 Agent 在执行什么操作

#### Approval（审批）
- 写操作需要的"许可凭证"
- 流程：request → approve → consume
- Nonce 和 SHA-256 binding 确保不可伪造和不可重用
- **为什么需要？** 人在环路 (human-in-the-loop)——AI 不能在没有授权的情况下执行写操作

#### Task & Run（任务 & 运行）
- Task 是工作单元，Run 是一次执行尝试
- Task 状态机：`created → running → waiting_input | waiting_approval → completed | failed | cancelled`
- Run 有三种模式：embedded（嵌入式）、shadow（影子/观测）、active（活跃/写入）
- **为什么区分 shadow 和 active？** Shadow 模式让新版本 Runtime 先观察和规划但不执行写入，验证其行为后再升级到 active

#### MCP Tool System
- 工具调用经过完整的审计链：begin → execute → finish
- 每个 invocation 记录 argumentsSHA256、resultSHA256、latencyMS
- 写工具通过 approval gate 控制
- **为什么走 MCP 而不是直接 RPC？** MCP 提供了标准化的工具发现和调用协议，未来可以接入外部 MCP 服务器

#### Memory（记忆）
- 五种类型：working（工作）、episodic（情景）、semantic（语义）、procedural（过程）、observational（观察）
- 候选 → 审核 → 晋升 → 持久化
- **为什么需要记忆治理？** 防止 Agent 无限制地积累可能有误的上下文，记忆的创建和保留都需要人类参与

### Temporal 的角色

Temporal 被选为工作流引擎，因为它提供：

1. **持久化执行** —— workflow 在进程重启后自动恢复
2. **信号和查询** —— `provideTaskInput`、`resolveTaskApproval`、`cancelTask` 信号让人机交互成为 workflow 的一等公民
3. **活动重试** —— LLM 调用的瞬时失败自动重试
4. **可见性** —— 每个任务的执行轨迹可查询、可重放

---

## Phase 5 — Governed Interactive Path (2026-09-04 ~ 2026-09-07)

```
2026-09-06  d6458d03  Plan: chatbot revival + group @-mention (Route A then B)
2026-09-06  ea2548cc  A1: Legacy 1v1 auto-reply in microservices
2026-09-06  1f1949a9  A2: Group @-mention via legacy chatbot
2026-09-06  9dd0335d  B1: Inbound DM auto-enrollment
2026-09-06  587e0b49  B2: Group @-mention governed interactive reply
2026-09-07  44dbd805  Dedicated single-call reply for inbound DM/group
2026-09-07  2004e84c  Complete governed group @-reply completion audit
```

这一阶段解决了"让 AI 主动且安全地参与对话"的最后一公里问题。

### 双路线策略：Route A vs Route B

| 维度 | Route A (Legacy) | Route B (Governed) |
|------|-----------------|-------------------|
| 运行时 | Go Eino 嵌入式 | TS Agent Runtime + Temporal |
| 触发 | Kafka → Core 直接处理 | Kafka → Runtime → Temporal workflow |
| 治理 | 无 | 完整能力/审批/审计链 |
| 速度 | 快（同进程） | 稍慢（跨服务 + workflow） |
| 用途 | 快速验证 & 回退 | 生产目标架构 |

**为什么同时保留两条路线？** Route A 是"先让它能工作"的务实选择，Route B 是"让它正确工作"的架构目标。通过配置开关 (`DIPOLE_AI_DIRECT_REPLY_ENABLED`、`DIPOLE_AI_GROUP_REPLY_ENABLED`) 实现互斥——同一环境中只有一条路线处理某类触发，避免重复回复。

### B1：1v1 DM Auto-Enrollment

当用户首次 DM 助手时：
1. Kafka 事件到达 Agent Runtime
2. Runtime 检测到该用户没有 Definition → 自动创建低风险 Definition（read + write conversation 权限）
3. 自动创建 Promotion Grant
4. 启动 Temporal workflow 作为 `interactive_active` 任务
5. 调用 `planner.reply()` 直接生成回复（跳过昂贵的 discovery plan）
6. 通过审批链发送回复

### B2：群组 @提及

当用户在群里 @AI 时：
1. 解析消息中的 `@Dipole AI` 或 `@AI` 别名
2. 创建以群会话为作用域的 interactive 任务
3. 从 approval scope 的 `group:<uuid>` 恢复群会话身份
4. 回复投递到群内

### 性能优化：Dedicated Reply Path

最初的实现走完整的 plan → synthesize 流程，一次回复需要 3 次 LLM 调用，耗时 60-90 秒。优化后的 dedicated reply path 只做一次 LLM 调用（`planner.reply()`），直接生成自然语言回复，耗时降至 3-5 秒。

### 完成审计修复

群回复的完成审计最初失败，因为整个审计链路（action reference 校验 + DB CHECK 约束）只认 `assistant_reply` / `system_message` 两种 command kind。修复涉及三层：
1. `AgentToolActionReferenceV1.Validate()` 允许 `group_reply`
2. `verifyMessageActionReference` 从 consumed approval 的 resource scope 恢复群会话身份
3. MySQL 迁移 000062 扩展 CHECK 约束

---

## 设计决策与考量

### 1. 治理优先 vs 速度优先

Dipole 选择了**治理优先**：每个 Agent 写操作都经过 capability check → approval resolve → consume → audit 链路。这增加了延迟和复杂度，但确保了：
- 人类始终拥有最终控制权
- 每个操作可审计、可追溯
- 权限可精确管理和撤销

### 2. 确定性 ID

Task ID、Run ID、Invocation ID 都通过确定性哈希生成（非随机 UUID），好处是：
- 幂等性：相同输入产生相同 ID，重试安全
- 可溯源：从 ID 可反推其构成要素
- 冲突检测：意外重入会生成相同 ID 并命中唯一约束

### 3. 嵌入式回退

从 Phase 3 起，Core 始终保留 embedded 模式。这不仅是安全网，更是对架构健康度的持续验证——如果嵌入式模式跑不通，说明服务边界划分有问题。

### 4. 历史精简 (History Curation)

2026-09-02，`master` 分支从 4,286 个提交精简为 20 个里程碑快照。原始历史保存在 `archive/pre-history-rewrite-2026-09-02`。这个决定源于：
- 大量实验性提交增加了 git 操作的认知负担
- 里程碑快照更适合架构级文档叙事
- 原始历史仍可追溯（未删除）

### 5. 多语言运行时

Go + TypeScript 的组合不是妥协，而是刻意选择：
- Go 做基础设施（消息、同步、存储、认证）——性能敏感、变化少
- TypeScript 做编排（Agent workflow、LLM 交互、MCP）——迭代快、生态好
- gRPC 是两者之间的契约层

---

## 时间线总览

```
2026-02-14  ──── 项目创建（Go 单体 IM）
2026-04-13  ──── 群组 + Kafka + Eino AI 聊天机器人（嵌入式）
2026-04-16  ──── 多节点 Redis WS + Vue 3 前端
2026-04-18  ──── 热门群组基础设施
2026-08-26  ──── 平台规划 + 微服务拆分（Gateway / Message / Sync / Search）
2026-08-26  ──── sqlc 替代 GORM；gRPC 契约版本化
2026-08-27  ──── TypeScript Agent Runtime 脚手架
2026-08-28  ──── MCP worker + Temporal + 订阅 + 记忆治理
2026-08-28  ──── Cassandra / ES / Search 里程碑集成
2026-09-01  ──── 带审批的 Interactive 任务投递
2026-09-02  ──── 历史精简（4286 → 20 提交）；Definition / Subscription owner 绑定
2026-09-04  ──── Temporal 默认开启；OAuth 握手；MCP shadow drills
2026-09-06  ──── Active interactive 回复；Route A 遗留聊天机器人复活
2026-09-06  ──── Route B B1 入站 DM + B2 Governed 群 @提及回复
2026-09-07  ──── 群回复完成审计、DB 约束、性能优化
```

Dipole 的演进弧线：**学习型 IM 单体 → 事件驱动多节点 IM → 增量提取微服务 → 带 Temporal / MCP / 审批的 Governed TypeScript Agent Runtime → 入站对话 AI（DM + 群 @）走 Governed 路径**，遗留 Eino 聊天机器人作为 Route A 快速回退保留在配置开关之后。
