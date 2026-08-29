# Dipole AI 接入前置能力清单

## 1. 文档目的

这份清单用于指导 `Dipole` 在接入 `Eino Agent` 前，先把必要的基础能力准备好。

目标有三点：

- 让 AI 能力自然接入现有 IM 主链路
- 避免把 AI 逻辑直接揉进 `handler/service/repository`
- 为后续总结、问答、辅助回复、内容治理等能力打好基础

当前建议采用的接入思路：

- `IM` 主链路继续保持 `HTTP + WebSocket + Kafka`
- `AI` 作为独立业务模块接入
- `Eino` 主要负责 `Agent / Tool / Workflow` 编排

---

## 2. AI 模块的目标能力

第一阶段建议先落这几类能力：

- AI 助手单聊
- 会话总结
- 智能回复建议
- 文件内容理解后的辅助问答
- 敏感内容或风险内容辅助判定

这些能力都依赖同一组基础能力：

- 能拿到会话上下文
- 能识别调用者身份与权限
- 能异步处理耗时任务
- 能把 AI 结果回写为消息或结构化记录

---

## 3. 现在就应该准备的能力

### 3.1 稳定的消息与会话上下文读取能力

AI 调用的第一步通常都是“取上下文”，因此当前就应该保证以下读取能力稳定可用：

- 按会话读取最近 `N` 条消息
- 按时间范围读取历史消息
- 能区分单聊、群聊、系统消息、文件消息
- 能拿到会话参与者、群成员、发送者基础资料
- 文件消息能拿到 `file_id/file_name/file_url/content_type/file_size`

建议收口成明确的应用层接口，例如：

- `GetConversationContext`
- `ListConversationMessages`
- `GetConversationParticipants`
- `GetMessageAttachmentMeta`

### 3.2 明确的 AI 消息类型与系统消息类型

后面 AI 返回内容时，不能和普通文本消息完全混在一起，否则展示、统计、审计都会变乱。

现在就应该约定：

- 消息类型中允许出现 `system`、`ai_text`、`ai_summary` 一类标识
- AI 输出的元数据单独存放
- 普通用户消息与 AI 生成消息在业务层可区分

建议优先准备：

- `message_type` 扩展点
- `message.meta` 或独立的 `ai_message_meta` 结构

### 3.3 稳定的业务事件流

AI 很适合挂在事件之后异步工作，所以当前事件流需要继续收稳。

建议保证以下事件具备统一 `Envelope`：

- `message.direct.send_requested`
- `message.direct.created`
- `message.group.send_requested`
- `message.group.created`
- `conversation.updated`
- `file.uploaded`
- `group.member_joined`
- `group.member_removed`

事件中至少应带：

- `event_id`
- `event_type`
- `occurred_at`
- `actor_uuid`
- `conversation_id` 或 `target_uuid`
- 可被反序列化的业务 `payload`

### 3.4 清晰的工具边界

`Eino Agent` 最终会通过 tools 调业务能力，所以要提前把业务操作收敛在应用层，而不是让 Agent 直接接触 repository。

建议优先整理成 tools 候选接口：

- `GetConversationContext`
- `GetUserProfile`
- `GetGroupProfile`
- `SearchConversationMessages`
- `CreateSummaryRecord`
- `SendSystemMessage`
- `SendAIMessage`
- `GetFileContentMeta`

### 3.5 AI 专用配置与观测基础

AI 模块一接进来，就需要基础配置和日志，否则很难排障。

现在就可以提前留出：

- `ai.enabled`
- `ai.provider`
- `ai.model`
- `ai.timeout`
- `ai.max_context_messages`
- `ai.summary_trigger_count`

同时建议记录：

- `request_id`
- `conversation_id`
- `user_uuid`
- `model`
- `latency`
- `token_usage`
- `error`

### 3.6 权限模型

AI 功能的权限边界要先定清楚，不然后面容易出现“能读不能发”或“能发不能看历史”的混乱情况。

建议先明确：

- AI 是否作为特殊用户存在
- AI 单聊是否属于普通会话
- 群聊中 AI 是否默认可见全部消息
- AI 能否主动发消息，还是只在被调用时响应
- 管理后台触发的 AI 与用户侧 AI 是否共享权限模型

---

## 4. 接入 Eino 前必须补齐的能力

### 4.1 AI 上下文装配器

需要有一个专门组件负责把会话、用户、群、文件等信息装配成 `Agent` 可用上下文。

建议位置：

- `internal/modules/ai/application/context_builder.go`

职责建议包括：

- 读取最近消息
- 裁剪上下文长度
- 处理文件消息摘要信息
- 组装用户画像与会话信息
- 统一输出给 `Eino`

### 4.2 AI 结果落库与回写机制

AI 输出不能只停留在内存里，需要能写回业务系统。

建议至少支持两种结果：

- 回写为消息
- 回写为结构化记录，例如会话总结、内容标签、风险判定结果

建议补齐对应存储：

- `conversation_summaries`
- `ai_call_logs`
- `ai_task_records`

### 4.3 异步任务执行路径

多数 AI 任务都不适合卡在主请求链路里。

建议把以下能力做成异步：

- 会话总结
- 文件内容抽取
- 群聊待办提炼
- 风险内容判定

当前项目已经有 `Kafka`，因此建议沿用：

- 业务事件进入 `Kafka`
- AI worker 消费事件
- 生成结果后回写数据库
- 需要时再投递一条系统消息或 AI 消息

### 4.4 文件消息可读化

如果后面想让 AI 处理文件，当前文件能力还需要多一步“可理解化”。

建议补齐：

- 文件元数据读取
- 文本类文件内容提取
- 图片类文件的基础描述占位
- 失败重试与大小限制

第一阶段可以只支持：

- `txt`
- `md`
- `pdf`

---

## 5. 接入 Eino 后的增强项

这些不必阻塞第一版 Agent 落地，但应该提前留出扩展空间。

- Prompt 模板配置化
- 多模型切换
- Tool 调用审计
- 用户级限流和额度控制
- 会话摘要缓存
- 向量检索或 RAG
- 管理员侧 AI 排障工具

---

## 6. 推荐的模块结构

建议单独建立 AI 模块，保持与现有业务域平行：

- `internal/modules/ai/domain`
- `internal/modules/ai/application`
- `internal/modules/ai/infrastructure`
- `internal/modules/ai/delivery/http`
- `internal/modules/ai/delivery/kafka`

推荐职责划分：

- `domain`：AI 任务、总结记录、调用日志等领域对象
- `application`：上下文装配、任务编排、工具适配
- `infrastructure`：Eino provider、模型配置、日志与存储实现
- `delivery/http`：AI 调试接口、总结查询接口
- `delivery/kafka`：消费业务事件并触发 AI 任务

---

## 7. 推荐的第一版 Eino 落地顺序

建议按下面顺序推进：

1. AI 助手账号与 AI 会话类型
2. 会话上下文装配器
3. 接入一个 `Eino + LLM provider`
4. 打通 AI 单聊问答
5. 打通会话总结异步任务
6. 打通文件消息的内容提取与问答
7. 增加智能回复建议与内容治理

这个顺序比较适合当前项目，因为它优先复用我们已经有的：

- 用户体系
- 会话体系
- 消息体系
- 文件上传能力
- Kafka 事件链路

---

## 8. 当前阶段的明确建议

结合 `Dipole` 当前状态，接下来优先做这些准备最合适：

- 把“按会话取上下文”的服务接口正式收口
- 继续保持事件 `Envelope` 规范稳定
- 给消息模型留出 AI 消息元数据扩展点
- 为 AI 结果增加独立表，而不是全部塞进普通消息字段
- 将文件消息能力继续补到“可读取元数据、可做后续解析”

当前阶段先不急着做：

- 多 Agent 编排
- 向量数据库
- RAG
- 复杂工作流引擎
- 过早的多模型路由

---

## 9. 完成标志

如果以下几点都达成，就说明 `Dipole` 已经具备较好的 `Eino Agent` 接入基础：

- AI 能稳定读取会话上下文
- AI 能区分普通消息、系统消息和文件消息
- AI 能通过明确 tools 调业务接口
- AI 能异步消费事件并回写结果
- AI 有基础日志、限流、错误记录和权限边界

达到这个阶段后，再正式接入 `Eino Agent` 会比较顺，也更不容易出现大面积返工。
