# Dipole 使用手册

> 完整的功能说明、术语定义与操作指南

---

## 目录

- [1. 术语表](#1-术语表)
- [2. 快速开始](#2-快速开始)
- [3. 即时通讯功能](#3-即时通讯功能)
  - [3.1 登录与认证](#31-登录与认证)
  - [3.2 联系人管理](#32-联系人管理)
  - [3.3 1v1 对话](#33-1v1-对话)
  - [3.4 群组对话](#34-群组对话)
  - [3.5 文件管理](#35-文件管理)
  - [3.6 消息搜索](#36-消息搜索)
  - [3.7 设备安全](#37-设备安全)
  - [3.8 离线同步](#38-离线同步)
- [4. AI Agent 功能](#4-ai-agent-功能)
  - [4.1 与 AI 私聊](#41-与-ai-私聊)
  - [4.2 群内 @AI](#42-群内-ai)
  - [4.3 Agent 抽屉](#43-agent-抽屉)
  - [4.4 任务管理](#44-任务管理)
  - [4.5 审批流](#45-审批流)
  - [4.6 Agent 定义](#46-agent-定义)
  - [4.7 事件订阅](#47-事件订阅)
  - [4.8 长期记忆](#48-长期记忆)
  - [4.9 任务产物](#49-任务产物)
  - [4.10 Agent 运行时状态](#410-agent-运行时状态)
- [5. API 参考](#5-api-参考)
- [6. 配置参考](#6-配置参考)
  - [6.1 后端配置](#61-后端配置)
  - [6.2 前端 Feature Flags](#62-前端-feature-flags)
- [7. 部署运维](#7-部署运维)

---

## 1. 术语表

| 术语 | 英文 | 定义 |
|------|------|------|
| **Agent** | Agent | AI 助手实体，默认名为 "Dipole AI"，拥有独立的用户身份（类型 `assistant`），通过 Agent Runtime 执行受治理的操作 |
| **会话** | Conversation | 两个或多个参与者之间的通信通道，由 **会话键** 唯一标识 |
| **会话键** | Conversation Key | 会话的唯一标识符。1v1 对话：`direct:{排序后的两个用户UUID}`；群组：`group:{群组UUID}` |
| **消息** | Message | 会话中的一条通信单元，包含发送者、接收者、内容、类型、序号和时间戳 |
| **消息类型** | Message Type | 消息的分类：`Text`（文本）、`File`（文件）、`AIText`（AI 回复）、`System`（系统通知） |
| **任务** | Task | Agent 执行的一个工作单元，由触发器创建，经过治理链路完成目标 |
| **运行** | Run | 任务的一次执行尝试，可以是 `shadow`（只观测）或 `active`（实际执行） |
| **定义** | Definition | Agent 的权限和行为边界声明，版本化管理，由 owner 创建 |
| **能力** | Capability | Agent 可执行的原子操作，如 `conversation.read`（读取会话）、`message.assistant_reply.send`（发送助手回复） |
| **审批** | Approval | Agent 执行写操作前需获得的授权凭证，支持自动审批和人工审批 |
| **订阅** | Subscription | 事件过滤器，将 Agent 定义绑定到特定的 Kafka 事件类型和资源作用域 |
| **记忆** | Memory | Agent 的持久化上下文，分为五种类型（见 [4.8 节](#48-长期记忆)） |
| **产物** | Artifact | Agent 任务执行过程中生成的不可变输出文件 |
| **时间线** | Timeline | 任务执行的追加式事件日志，用于审计和 UI 展示 |
| **触发器** | Trigger | 创建任务的原因来源：入站消息（DM/群 @提及）、显式创建、事件订阅匹配 |
| **MCP** | Model Context Protocol | 模型上下文协议，标准化的工具发现和调用协议 |
| **工具调用** | Tool Invocation | Agent 通过 MCP 协议执行的一次能力操作，经过完整的审计链 |
| **资源作用域** | Resource Scope | 限定 Agent 操作范围的声明，包含资源类型、资源 ID 和操作类型 |
| **晋升** | Promotion | Agent Runtime 版本从 shadow 到 active 的升级过程 |
| **Feature Flag** | Feature Flag | 前端功能开关，控制 UI 中哪些 Agent 功能对用户可见 |

---

## 2. 快速开始

### 2.1 访问系统

Dipole 是一个 Web 应用，通过浏览器访问：

```
https://<部署地址>/app/
```

前端 SPA 嵌入在 Core 服务的 Go 二进制中，由 Gateway 反向代理提供。

### 2.2 注册与登录

1. 打开系统首页，进入登录页
2. 如果是首次使用，点击 **注册** 创建账号（需提供手机号、昵称、密码）
3. 输入手机号和密码登录
4. 登录成功后自动跳转到主聊天界面

> **登录凭证：** 系统使用 JWT token 认证。登录后 token 保存在浏览器本地，下次访问自动登录。

### 2.3 界面概览

登录后的主界面分为以下区域：

```
┌─────────────────────────────────────────────────────────┐
│  导航栏                                                  │
│  ┌──────────┬──────────────────────────────────────────┐ │
│  │ 会话列表   │ 聊天窗口                                  │ │
│  │          │                                          │ │
│  │ 搜索框    │  消息气泡                                  │ │
│  │          │  ...                                     │ │
│  │ 会话 1    │                                          │ │
│  │ 会话 2    │                                          │ │
│  │ 会话 3    │  ┌──────────────────────────────────────┐│ │
│  │ ...      │  │ 输入框                                ││ │
│  │          │  └──────────────────────────────────────┘│ │
│  └──────────┴──────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

导航栏提供以下页面：
- **聊天**（默认）—— 即时通讯主界面
- **联系人** —— 管理好友和好友申请
- **群组** —— 管理群组
- **文件** —— 浏览已上传的文件
- **设备** —— 管理登录设备和会话安全

---

## 3. 即时通讯功能

### 3.1 登录与认证

| 操作 | 说明 |
|------|------|
| 注册 | 手机号 + 昵称 + 密码 |
| 登录 | 手机号 + 密码 |
| 登出 | 点击设置或头像区域的"退出登录" |
| 修改密码 | 通过设置页面修改 |

**设备绑定：** 每个浏览器会话被视为一个设备，分配唯一的 `device_id`。多端登录时，每端独立同步。

### 3.2 联系人管理

#### 添加联系人

1. 进入 **联系人** 页面
2. 点击 **添加好友**
3. 搜索目标用户（按昵称或手机号）
4. 发送好友申请

#### 处理好友申请

- 收到的好友申请会在联系人页面显示
- 可以选择 **接受** 或 **拒绝**
- 接受后双方自动成为联系人

#### 联系人列表

- 显示所有已添加的好友
- 点击联系人可直接发起 1v1 对话

### 3.3 1v1 对话

#### 发起对话

- 方式一：在联系人列表中点击好友
- 方式二：在搜索框中搜索用户名，选择已是好友的用户

#### 发送消息

- 在底部输入框输入文字，按 **Enter** 发送
- 支持发送文本消息和文件消息

#### 消息状态

| 状态 | 说明 |
|------|------|
| 已发送 | 消息已提交到服务端 |
| 已送达 | 对方在线时，通过 WebSocket 实时推送 |
| 已读 | 对方打开会话时自动标记已读 |

#### 会话管理

- **置顶**：长按/右键会话可置顶
- **归档**：归档不常用的会话
- **未读标记**：会话列表显示未读消息数
- **已读同步**：打开会话后，未读数自动清零

### 3.4 群组对话

#### 创建群组

1. 进入 **群组** 页面
2. 点击 **创建群组**
3. 输入群名称
4. 选择初始成员
5. 确认创建

#### 群组操作

| 操作 | 角色要求 | 说明 |
|------|---------|------|
| 发送消息 | 所有成员 | 群内发送文本或文件 |
| @成员 | 所有成员 | 在消息中 @ 特定成员 |
| 邀请成员 | 群主/管理员 | 邀请新成员加入 |
| 移除成员 | 群主 | 移除群成员 |
| 修改群名 | 群主 | 修改群组名称 |
| 退出群组 | 所有成员 | 退出群（群主转让后退出） |

#### @提及

在输入框中输入 `@` 后会弹出成员列表，选择要 @ 的成员。被 @ 的成员会收到特殊提醒。

> **@AI：** 在群里输入 `@Dipole AI` 或 `@AI` 可以触发 AI 助手回复，详见 [4.2 群内 @AI](#42-群内-ai)。

### 3.5 文件管理

#### 上传文件

- 在聊天窗口点击 **附件** 图标
- 选择本地文件上传
- 支持分片上传（大文件自动分片）
- 上传完成后作为文件消息发送

#### 文件页面

- **文件** 页面显示所有已上传的文件
- 可按时间、类型筛选
- 支持预览和下载

**存储：** 文件存储在 MinIO（S3 兼容）对象存储中，元数据保存在 MySQL。

### 3.6 消息搜索

#### 会话筛选搜索

- 在左侧会话列表顶部的 **搜索框** 中输入关键字
- 实时过滤匹配的会话（按会话名/联系人名筛选）
- 按 **Enter** 确认，按 **Esc** 或点击清除按钮清空

#### 全文消息搜索

- 如果启用了全文搜索（需部署 Search + Search Indexer + Elasticsearch），搜索框右侧会显示 **全文搜索** 按钮
- 点击后进入全文搜索模式，可以跨会话搜索消息内容
- 搜索结果按相关性排序，点击结果可跳转到对应消息

### 3.7 设备安全

**设备** 页面提供：

- 查看当前登录的所有设备/会话
- 查看设备的最后活跃时间
- 远程注销其他设备的登录状态

### 3.8 离线同步

Dipole 支持离线消息同步：

1. **WebSocket 实时推送** —— 在线时，消息通过 WebSocket 实时推送到客户端
2. **增量同步** —— 重新上线后，客户端通过 Sync API 拉取离线期间的消息增量
3. **IndexedDB 缓存** —— 客户端将消息缓存在 IndexedDB 中，提升加载速度
4. **设备游标** —— 每个设备独立维护同步游标，确保不同设备各自获取完整增量

---

## 4. AI Agent 功能

Dipole 内置 AI 助手 **Dipole AI**，支持通过受治理的方式参与对话。

### 4.1 与 AI 私聊

#### 如何使用

1. 在联系人中找到 **Dipole AI**（它是一个特殊的 assistant 类型用户）
2. 点击进入 1v1 对话
3. 直接发送消息即可

#### 工作原理

当你发送消息给 AI：

1. 消息通过正常的消息链路到达 Kafka
2. Agent Runtime 消费到消息事件
3. 自动创建一个 **interactive task**（交互式任务）
4. AI 读取会话历史，生成回复
5. 回复通过受治理的链路发送（经过审批 → 消费 → 审计）
6. 你通过 WebSocket 实时收到 AI 的回复

#### 回复特征

- 回复速度通常为 3-5 秒
- AI 回复标记为 `AIText` 消息类型，在 UI 中有特殊标识
- 每条消息会创建一个独立的 Agent Task，可在 Agent 抽屉中查看

### 4.2 群内 @AI

#### 如何使用

1. 在任意群聊中输入 `@Dipole AI` 或 `@AI`
2. 在 @提及之后输入你的问题或指令
3. 发送消息

#### 工作原理

与 1v1 私聊类似，但有以下区别：
- 任务的资源作用域是 **群会话**（`group:<groupUUID>`）
- AI 读取的是 **群聊历史**（而非私聊历史）
- 回复发送到 **群内**（所有群成员可见）
- 审批的资源作用域明确绑定到该群会话

#### 注意事项

- 群里其他成员发的消息不会触发 AI 回复（除非也 @AI）
- AI 的回复在群内对所有成员可见
- 每次 @ 触发一个独立的 Agent Task

### 4.3 Agent 抽屉

Agent 抽屉是 Agent 功能的统一管理界面，位于主聊天界面的右侧。

#### 打开方式

- 点击顶部状态栏的 **DIPOLE** 标识（如有待处理任务会显示数字徽章）
- 或通过 URL 参数 `?agent=1` 直接打开

#### 抽屉结构

Agent 抽屉包含以下标签页（根据 Feature Flag 决定可见性）：

| 标签页 | Feature Flag | 功能 |
|--------|-------------|------|
| **任务** | `taskCreate` 或 `timeline` | 任务列表、创建、详情 |
| **产物** | `artifacts` | Agent 生成的文件和产出物 |
| **定义** | `definitions` | Agent 权限定义管理 |
| **订阅** | `subscriptions` | 事件订阅管理 |
| **记忆** | `memories` | 长期记忆浏览和管理 |

#### 导航

每个标签页的功能详见下方各节。

### 4.4 任务管理

#### 任务列表

在 Agent 抽屉的 **任务** 标签页中：

- 查看所有任务，按创建时间倒序排列
- 每个任务显示：ID、状态、目标摘要、创建时间
- 点击任务行查看详情

#### 任务状态

| 状态 | 含义 | 颜色 |
|------|------|------|
| `created` | 已创建，等待执行 | 灰色 |
| `running` | 正在执行中 | 蓝色 |
| `waiting_input` | 等待用户输入 | 橙色 |
| `waiting_approval` | 等待审批 | 橙色 |
| `completed` | 已完成 | 绿色 |
| `failed` | 执行失败 | 红色 |
| `cancelled` | 已取消 | 灰色 |

#### 任务详情面板

点击任务后，右侧展开详情面板，显示：

- **基本信息：** 任务 ID、状态、Agent、触发类型、创建时间
- **时间线：** 任务执行的每个步骤的追加式事件日志
- **审批项：** 如果任务等待审批，显示审批按钮
- **输入项：** 如果任务等待输入，显示输入表单

**详情面板操作：**
- 点击左侧 **‹** 按钮收起详情面板（列表仍可操作）
- 收起后右侧显示 **›** 按钮可重新展开
- 点击 **×** 按钮关闭详情并取消选中

#### 创建任务

如果启用了 `taskCreate` Flag：

1. 在任务标签页点击 **创建任务**
2. 填写任务目标描述
3. 选择关联的 Agent 定义
4. 提交创建

> **注：** 大多数任务是由入站消息自动创建的（如与 AI 私聊），手动创建适用于需要显式触发的复杂任务。

#### 取消任务

对于处于 `running`、`waiting_input`、`waiting_approval` 状态的任务：
- 在详情面板中点击 **取消** 按钮
- 确认后任务进入 `cancelled` 状态

### 4.5 审批流

当 Agent 需要执行写操作时，需要获得审批。

#### 自动审批 vs 人工审批

| 场景 | 审批方式 | 说明 |
|------|---------|------|
| 入站 DM 回复 | 自动审批 | 低风险操作，由 runtime 自动 approve + consume |
| 群 @回复 | 自动审批 | 低风险操作，scope 限定在目标群 |
| 自定义任务写操作 | 人工审批 | 高风险操作，需要用户在 Agent 抽屉中手动批准 |

#### 人工审批操作

1. 当任务状态变为 `waiting_approval` 时，状态栏徽章会更新
2. 打开 Agent 抽屉，找到等待审批的任务
3. 查看审批详情：
   - 请求的能力名称
   - 目标资源作用域
   - 操作说明
4. 选择 **批准** 或 **拒绝**

#### 审批安全机制

- 每个审批有唯一的 `nonce_sha256`，防止重放
- 审批绑定 `arguments_sha256`，确保审批内容不被篡改
- 审批一旦消费即不可再次使用
- 审批有过期时间

### 4.6 Agent 定义

Agent 定义 (Definition) 是 Agent 的权限和行为边界声明。

#### 查看定义

在 Agent 抽屉的 **定义** 标签页中：
- 查看所有 Agent 定义的列表
- 每个定义显示：版本、状态、有效期、owner

#### 定义内容

一个定义包含：

| 字段 | 说明 |
|------|------|
| Agent UUID | 关联的 Agent 身份 |
| Owner UUID | 定义的拥有者 |
| Permissions | 权限列表（JSON），声明 Agent 可使用的能力 |
| Scopes | 资源作用域（JSON），限定操作的目标范围 |
| Status | `active`（激活）或 `revoked`（已撤销） |
| Valid From/Until | 有效期范围 |

#### 自动创建

对于入站消息场景（1v1 DM / 群 @），如果用户没有预先创建定义，系统会自动创建一个低风险定义（包含 conversation read/write 权限）。

### 4.7 事件订阅

事件订阅 (Subscription) 让 Agent 能够自动响应特定类型的事件。

#### 查看订阅

在 Agent 抽屉的 **订阅** 标签页中：
- 查看所有订阅的列表
- 每个订阅显示：事件类型、资源作用域、状态

#### 订阅模型

```
订阅 = Agent 定义 + 事件类型 + 资源作用域
```

当匹配的事件发生时，Agent Runtime 自动创建一个任务来处理。

#### 事件类型

| 事件类型 | 说明 |
|----------|------|
| `message.group.created` | 群组中创建了新消息 |
| `message.direct.created` | 1v1 中创建了新消息 |

#### 资源作用域

订阅可以限定在特定资源上：
- **不限定**：响应所有匹配事件
- **限定群组**：仅响应特定群组的事件

### 4.8 长期记忆

Agent 记忆 (Memory) 是持久化的上下文信息，让 AI 在跨会话中保持一致性。

#### 查看记忆

在 Agent 抽屉的 **记忆** 标签页中：
- 查看所有记忆条目
- 按类型、时间筛选
- 查看记忆的详细内容和元数据

#### 记忆类型

| 类型 | 英文 | 说明 | 示例 |
|------|------|------|------|
| 工作记忆 | Working | 当前会话的短期上下文 | "用户正在讨论数据库迁移方案" |
| 情景记忆 | Episodic | 具体事件的记忆 | "2026-09-01 用户报告了登录问题" |
| 语义记忆 | Semantic | 事实性知识 | "用户偏好使用 TypeScript" |
| 过程记忆 | Procedural | 操作步骤的记忆 | "部署流程：先构建镜像，再 docker compose up" |
| 观察记忆 | Observational | AI 的观察和推断 | "用户对响应速度比较敏感" |

#### 记忆治理

记忆不是自动保存的，而是经过治理管线：

```
候选 (Candidate) → 审核 (Review) → 晋升 (Promotion) → 持久化 (Persisted)
```

- **候选**：AI 在任务执行过程中提出的记忆候选
- **审核**：需要人工或自动审核其准确性和适当性
- **晋升**：审核通过后正式保存
- **纠正/擦除**：已保存的记忆可以被纠正或擦除

#### 记忆纠正

如果启用了 `memoryCorrection` Flag，记忆列表中会显示纠正按钮，可以修改已保存记忆的内容。

### 4.9 任务产物

Agent 产物 (Artifact) 是任务执行过程中生成的不可变文件。

#### 查看产物

在 Agent 抽屉的 **产物** 标签页中：
- 查看所有产物列表
- 每个产物显示：文件名、类型、大小、关联任务
- 支持下载和预览

#### 产物类型

产物可以是任何类型的文件：
- 文本报告
- 代码片段
- 分析结果
- 生成的图表

产物存储在 MinIO 对象存储中，元数据保存在 MySQL 的 `agent_artifacts` 表。

### 4.10 Agent 运行时状态

Agent 抽屉顶部显示的 **DIPOLE** 标识包含运行时状态信息：

| 显示 | 含义 |
|------|------|
| `DIPOLE` | 正常，无待处理事项 |
| `DIPOLE · N pending` | 有 N 个待处理任务（需要输入或审批） |

**AgentLiveView** 组件提供更详细的运行时状态：
- 活跃任务计数
- 最近的产物列表
- 运行时版本和健康状态

---

## 5. API 参考

### 5.1 认证

所有 API 请求需要在 Header 中携带：

```
Authorization: Bearer <jwt-token>
X-Device-ID: <device-uuid>
```

### 5.2 响应格式

所有 API 响应采用统一信封格式：

```json
{
  "code": 0,
  "data": { ... },
  "message": "success"
}
```

`code = 0` 表示成功，非零表示错误。

### 5.3 主要 API 端点

#### 用户与认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 注册新用户 |
| POST | `/api/v1/auth/login` | 登录获取 JWT |
| GET | `/api/v1/users/me` | 获取当前用户信息 |
| GET | `/api/v1/users/search` | 搜索用户 |

#### 联系人

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/contacts` | 获取联系人列表 |
| POST | `/api/v1/contacts/apply` | 发送好友申请 |
| PUT | `/api/v1/contacts/applications/:id` | 处理好友申请 |

#### 消息

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/messages/direct` | 发送 1v1 消息 |
| POST | `/api/v1/messages/group` | 发送群消息 |
| GET | `/api/v1/messages/direct/:uuid` | 获取 1v1 历史消息 |
| GET | `/api/v1/messages/group/:uuid` | 获取群历史消息 |

#### 会话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/conversations` | 获取会话列表 |
| PATCH | `/api/v1/conversations/:key/read` | 标记已读 |

#### 群组

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/groups` | 获取群组列表 |
| POST | `/api/v1/groups` | 创建群组 |
| POST | `/api/v1/groups/:uuid/members` | 添加群成员 |

#### 文件

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/files/upload` | 上传文件 |
| GET | `/api/v1/files/:uuid/download` | 下载文件 |

#### 同步

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/sync/inbox` | 获取同步增量 |
| GET | `/api/v1/sync/groups/checkpoints` | 获取群同步游标 |

#### Agent

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/agent/tasks` | 获取任务列表 |
| POST | `/api/v1/agent/tasks` | 创建任务 |
| GET | `/api/v1/agent/tasks/:uuid` | 获取任务详情 |
| POST | `/api/v1/agent/tasks/:uuid/approve` | 审批任务 |
| POST | `/api/v1/agent/tasks/:uuid/input` | 提供任务输入 |
| POST | `/api/v1/agent/tasks/:uuid/cancel` | 取消任务 |
| GET | `/api/v1/agent/tasks/:uuid/timeline` | 获取任务时间线 |
| GET | `/api/v1/agent/definitions` | 获取定义列表 |
| POST | `/api/v1/agent/definitions` | 创建定义 |
| GET | `/api/v1/agent/subscriptions` | 获取订阅列表 |
| POST | `/api/v1/agent/subscriptions` | 创建订阅 |
| GET | `/api/v1/agent/memories` | 获取记忆列表 |
| GET | `/api/v1/agent/artifacts` | 获取产物列表 |
| GET | `/api/v1/agent/artifacts/:uuid` | 下载产物 |

#### WebSocket

```
ws(s)://host/api/v1/ws?token=<jwt>&device=web&device_id=<uuid>
```

WebSocket 接收以下类型的推送：
- 新消息（1v1 / 群组）
- Agent 任务状态变更 (`agent.task.waiting`)

---

## 6. 配置参考

### 6.1 后端配置

#### 数据库

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DIPOLE_DB_HOST` | `localhost` | MySQL 地址 |
| `DIPOLE_DB_PORT` | `3306` | MySQL 端口 |
| `DIPOLE_DB_USER` | `root` | MySQL 用户名 |
| `DIPOLE_DB_PASSWORD` | — | MySQL 密码 |
| `DIPOLE_DB_NAME` | `dipole` | 数据库名 |

#### Redis

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DIPOLE_REDIS_HOST` | `localhost` | Redis 地址 |
| `DIPOLE_REDIS_PORT` | `6379` | Redis 端口 |

#### Kafka

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DIPOLE_KAFKA_BROKERS` | `localhost:9092` | Kafka broker 列表 |

#### Gateway

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DIPOLE_GATEWAY_MODE` | `embedded` | `embedded`（单体）或 `remote`（微服务） |
| `DIPOLE_GATEWAY_AGENT_CONTROL_TARGET` | — | Agent Runtime HTTP 地址 |

#### AI / Agent

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DIPOLE_AI_ENABLED` | `false` | 启用 Legacy AI (Route A) |
| `DIPOLE_AI_DIRECT_REPLY_ENABLED` | `false` | Route A 1v1 DM 回复 |
| `DIPOLE_AI_GROUP_REPLY_ENABLED` | `true` | Route A 群 @回复 |
| `DIPOLE_AGENT_INBOUND_DM_ENABLED` | `false` | Route B1 入站 DM |
| `DIPOLE_AGENT_INBOUND_GROUP_MENTION_ENABLED` | `false` | Route B2 入站群 @提及 |
| `DIPOLE_AGENT_MODEL_PROVIDER` | `openai` | LLM 提供商 |
| `DIPOLE_AGENT_MODEL_NAME` | — | 模型名称 |
| `DIPOLE_AGENT_MODEL_API_KEY` | — | API 密钥 |
| `DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS` | `2048` | 最大输出 token |
| `DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS` | `120000` | 总超时（毫秒） |

#### 内部 RPC

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DIPOLE_INTERNAL_RPC_ENABLED` | `false` | 启用 gRPC 服务间通信 |
| `DIPOLE_INTERNAL_RPC_CORE_TARGET` | `core:9091` | Core gRPC 地址 |
| `DIPOLE_INTERNAL_RPC_MESSAGE_TARGET` | `message:9092` | Message gRPC 地址 |
| `DIPOLE_INTERNAL_RPC_SEARCH_TARGET` | `search:9093` | Search gRPC 地址 |
| `DIPOLE_INTERNAL_RPC_SYNC_TARGET` | `sync:9094` | Sync gRPC 地址 |

### 6.2 前端 Feature Flags

前端功能通过两层 Feature Flag 控制：

1. **编译时 (Vite)：** `VITE_AGENT_*_ENABLED` 环境变量
2. **运行时 (Core 注入)：** Core 向前端 HTML 注入 `window.__DIPOLE_FLAGS__` 对象

运行时 Flag 优先于编译时 Flag。

| Flag | Vite 变量 | 运行时 Key | 控制的功能 |
|------|-----------|-----------|-----------|
| 任务审批 | `VITE_AGENT_APPROVAL_ENABLED` | `approval` | 审批 UI 按钮 |
| 用户输入征求 | `VITE_AGENT_ELICITATION_ENABLED` | `elicitation` | 输入表单 UI |
| 事件订阅 | `VITE_AGENT_SUBSCRIPTIONS_ENABLED` | `subscriptions` | 订阅标签页 |
| Agent 定义 | `VITE_AGENT_DEFINITIONS_ENABLED` | `definitions` | 定义标签页 |
| 长期记忆 | `VITE_AGENT_MEMORIES_ENABLED` | `memories` | 记忆标签页 |
| 记忆纠正 | `VITE_AGENT_MEMORY_CORRECTION_ENABLED` | `memoryCorrection` | 记忆纠正按钮 |
| 任务创建 | `VITE_AGENT_TASK_CREATE_ENABLED` | `taskCreate` | 创建任务按钮 |
| 任务时间线 | `VITE_AGENT_TIMELINE_ENABLED` | `timeline` | 任务时间线视图 |
| 任务产物 | `VITE_AGENT_ARTIFACTS_ENABLED` | `artifacts` | 产物标签页 |

---

## 7. 部署运维

### 7.1 环境要求

| 组件 | 最低版本 | 说明 |
|------|---------|------|
| Docker | 20.10+ | 容器运行时 |
| Docker Compose | 2.x | 服务编排 |
| MySQL | 8.4 | 主数据库 |
| Redis | 7.x | 缓存/Pub-Sub |
| Kafka | 3.x | 事件总线 |
| Temporal | 1.x | 工作流引擎 |
| PostgreSQL | 15+ | Temporal 后端 |
| Node.js | 20.x | Agent Runtime |
| Go | 1.22+ | Go 服务编译 |
| Elasticsearch | 8.x | 全文搜索（可选） |
| MinIO | — | 对象存储 |

### 7.2 启动步骤

#### 基础微服务栈

```bash
cd deploy/compose

# 1. 创建 .env 文件（参考 configs/config.dist.yaml）
cp ../../.env.example ../../.env
# 编辑 .env 填入数据库密码、Kafka 地址等

# 2. 运行数据库迁移
docker compose -f docker-compose.microservices.yml run --rm migrate

# 3. 启动基础服务
docker compose -f docker-compose.microservices.yml up -d
```

#### 添加 Agent Experience (Route B)

```bash
docker compose \
  -f docker-compose.microservices.yml \
  -f ../microservices/agent-experience.yml \
  up -d
```

#### 添加搜索功能

```bash
docker compose \
  -f docker-compose.microservices.yml \
  -f ../microservices/agent-experience.yml \
  --profile search \
  up -d
```

### 7.3 健康检查

每个服务暴露健康检查端点：

| 服务 | 端点 | 预期响应 |
|------|------|---------|
| Core | `GET /healthz` | 200 OK |
| Gateway | `GET /healthz` | 200 OK |
| Message | gRPC health | Serving |
| Sync | gRPC health | Serving |
| Agent Runtime | `GET /livez` | 200 OK |
| Agent Runtime | `GET /readyz` | 200 OK |

### 7.4 监控

- **Prometheus 指标：** Agent Runtime 在 `/metrics` 端点暴露 Prometheus 格式的指标
- **日志：** 所有服务输出结构化 JSON 日志
- **Temporal UI：** Temporal Server 提供内建 Web UI，可查看工作流执行详情

### 7.5 数据库迁移

```bash
# 执行所有待运行的迁移
docker compose -f docker-compose.microservices.yml run --rm migrate up

# 回退最后一个迁移
docker compose -f docker-compose.microservices.yml run --rm migrate down 1

# 查看当前迁移版本
docker compose -f docker-compose.microservices.yml run --rm migrate version
```

迁移文件位于 `db/migrations/`，编号从 `000001` 到 `000062`，包含 `*.up.sql` 和 `*.down.sql` 成对出现。

### 7.6 嵌入式模式（开发/回退）

如需以单体模式运行（所有 Go 服务在一个进程中）：

```bash
# 设置 Gateway 模式为嵌入式
export DIPOLE_GATEWAY_MODE=embedded

# 运行 Core（即包含所有功能）
./dipole-core
```

> **注意：** 嵌入式模式下不包含 Agent Runtime（TypeScript 进程需要独立运行）。

### 7.7 Route 切换

在 `agent-experience.yml` 中配置 Route B 取代 Route A：

```yaml
# Core 服务
environment:
  # 关闭 Route A
  DIPOLE_AI_DIRECT_REPLY_ENABLED: "false"
  DIPOLE_AI_GROUP_REPLY_ENABLED: "false"

# Agent Runtime
environment:
  # 开启 Route B
  DIPOLE_AGENT_INBOUND_DM_ENABLED: "true"
  DIPOLE_AGENT_INBOUND_GROUP_MENTION_ENABLED: "true"
```

⚠️ **不要同时开启同一触发类型的 Route A 和 Route B**，否则用户会收到两份回复。
