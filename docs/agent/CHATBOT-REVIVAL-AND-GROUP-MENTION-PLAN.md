# 小助手对话闭环恢复 + 群 @ 触发 —— 实施计划

> 状态：计划已定，按「先 A 后 B」执行。Route A 与 Route B 完全独立、互不交叉，可各自单独阅读。

## 0. 背景：仓库里并存两套 Agent

| | 旧「对话机器人」(legacy) | 新「受治理运行时」(current) |
|---|---|---|
| 代码 | `internal/services/agent/legacy/`（package `ai`，基于 eino） | `services/agent-runtime/`（TS）+ `internal/services/agent/`（Go） |
| 触发 | 私信小助手即自动回（`HandleDirectMessage` 消费 `message.direct.created`） | 仅显式建任务（`agent.interactive.requested`） |
| 多轮 | 有（最近 12 条上下文，`context_builder.go`） | 无（一 task 一回复） |
| 工具 | 5 个：查用户资料 / 搜历史 / 列会话 / 读会话 / 发系统消息（`tools.go`） | 仅 conversation.list/read/search + message send |
| 模型 | eino：openai / ollama（`model_factory.go`） | DeepSeek via AI SDK |
| 装配 | 仅 embedded 单体（`embedded/kafka.go:103`） | microservices（体验环境） |
| 开关 | `ai.enabled`，默认 false | `agent_*_enabled` 分档 |

关键事实：
- 默认小助手 `UAI000000000000000001`（"Dipole AI"）仍是一等公民用户（`UpsertAssistant`），不加好友也能私信。
- legacy chatbot 代码活着、带全套测试，但只在单体模式接线，microservices 部署没接。
- `message.group.created` 主题已发布；群消息**无结构化 mention 字段**，@ 需文本解析。
- eino 依赖已在 `go.mod`（eino v0.9.17 + ollama/openai ext），core 镜像可直接编译 legacy。

## 1. 目标与验收
- G1：私信小助手 → 自动 AI 回复，能调用工具（1v1 多轮对话恢复）。
- G2：在含小助手的群里 @小助手 → 群内 AI 回复（命中 @ 才触发）。
- G3：全程走 policy/capability 治理层，有审计与幂等，可一键回滚。

---

# Route A —— 快速恢复（基于 legacy eino chatbot）

> 目标：用现成的 legacy chatbot 最快恢复 1v1 与群 @。本节自包含，不依赖 Route B。

## A0. 边界与并存策略
- A 路线让 **Go core** 跑 legacy chatbot 消费 `message.direct.created` / `message.group.created`。
- 体验环境的 TS agent-runtime 在 interactive 档对 direct.created 只观察不回复，触发源是 control API；两者不在同一触发链冲突：**私信/群@ 走 legacy，显式任务走 TS 运行时**。
- 模型复用体验环境已有的 DeepSeek key，用 eino 的 openai-兼容模型 + `ai.base_url` 指向 DeepSeek。

## A1. 恢复 1v1 自动对话（先做）
改动点：
- 复用/上移 `internal/services/core/bootstrap/embedded/kafka.go` 的 `newAIService` + `handleAIDirectReply`（抽成可被 microservices 复用，或在 core runtime 装配处等价注册）。
- 在 microservices core 运行时（`internal/services/core/bootstrap/runtime.go` 的 Kafka projection 注册处，约 line 123）注册 `message.direct.created` → `handleAIDirectReply(aiService)`。
- 配置（体验环境 env）：`ai.enabled=true`、`ai.provider=openai`、`ai.base_url=<DeepSeek>`、`ai.api_key=<复用>`、`ai.model=<deepseek 模型>`、`ai.max_context_messages=12`。
测试：
- 复用 `legacy/service_test.go`、`baseline_eval_test.go`。
- 新增 microservices 装配测试：core 启动后 `message.direct.created` handler 已注册且 `ai.enabled` 生效。
部署：重建 core 镜像 → 配 `ai.*` env → 重建 core 容器。
验收：私信小助手"帮我看看我最近的会话" → 自动回复且触发 list/read 工具。
回滚：`ai.enabled=false` 重建 core。

## A2. 群 @ 触发
前置：mention 约定（无结构化字段 → 文本解析）
- 约定触发 token：群消息内容包含小助手昵称 mention（如 `@Dipole AI`）。前端 @ 时插入该 token。
- 解析器：`detectAssistantMention(content, assistantNickname) bool`，带边界/大小写/去空白，避免误伤。
改动点：
- 新增 `Service.HandleGroupMessage(ctx, message)`：`TargetType=group` 且命中 mention 才触发；复用 legacy 工具（已支持读群会话 `tools.go:427/515`）。
- 回复投递：先用 `SendSystemGroupMessage(groupUUID, content)`；如需 AI 文本类型再加 `SendAssistantGroupMessage`（MessageTypeAIText → group）。
- 授权 scope：`policy.Start` 的 `ResourceScopes` 加"群会话读 + 群发言"。
- 幂等：以群消息 UUID 为触发键（`AICallLog.TriggerMessageUUID` 已有），防重复回。
- 注册 `message.group.created` → `handleAIGroupReply(aiService)`。
测试：mention 解析单测；群回复幂等 & 非 @ 不触发；工具读群会话。
部署：重建 core → 重建 core 容器。
验收：群里 @小助手 → 群内 AI 回复；不 @ 不回；重复投递只回一次。
回滚：注销 group handler 或 `ai.enabled=false`。

## A3. 打磨
- 策略：默认「@ 才回」；可选群白名单。
- 速率限制 / 并发去重 / 失败兜底话术。
- 前端确认群内 AI 消息（MessageTypeAIText）渲染正常。

---

# Route B —— 架构收敛（折叠进受治理运行时）

> 目标：把 1v1 与群 @ 都做成新运行时的触发源，统一审批/promotion/审计，最终让 legacy 退役。本节自包含，不依赖 Route A。

## B1. 入站直发触发交互任务
- 把发给 `UAI0001` 的 `message.direct.created` 在 `direct_target` 档接成"起 interactive task"（复用 `InteractiveTaskStartService` 的等价链路，或新增 inbound→task dispatcher）。
- 复用已上线的 assistant_reply 闭环（`AuthorizeInteractiveReply` + `createInteractiveReplyExecutor`），实现真·多轮 1v1。
- 会话上下文：交互任务读取直属会话最近 N 条作为 prompt。
验收：私信小助手多轮对话，每轮经 admission/approval/审计。

## B2. 群订阅 + mention（治理版）
- 走 subscription 档（已支持 `message.group.created`，`agent_subscription.go:170`）。
- mention 前置过滤：订阅事件先过 `detectAssistantMention`，命中才 admit。
- 新增自授权原语 `AuthorizeGroupReply`（类比 `AuthorizeInteractiveReply`），scope 精确指向该群会话；新增 group assistant_reply executor（走 `SendSystemGroupMessage`/群 AI 文本）。
- proto RPC + gateway/core 两处 allowlist + 单测，与现有 interactive 一致。
验收：群 @小助手 → 群内回复，全程 grant/approval/consumed 审计。

## B3. 工具移植与 legacy 退役
- 把 legacy 的查资料 / 搜历史 / 列读会话移植为新运行时 capability（proto + registry + policy + 单测）。
- 迁移完成后停用 A 路线的 core legacy handler，legacy 目录标记 deprecated / 移除接线。

---

# 执行顺序与里程碑

1. **A1**（1v1 恢复）→ 体验环境验证 → 提交推送。
2. **A2**（群 @）→ 体验环境验证 → 提交推送。
3. **A3**（打磨）。
4. 之后再启 **B1 → B2 → B3**，逐步把触发链从 legacy 收敛到受治理运行时；B 稳定后 A 退役。

边界纪律：
- A 与 B 不共享触发链；同一时刻同一环境只让一条链对某类触发负责，避免"双回复"。
- 每一步：改动有测试、体验环境端到端验证、单一可 review 提交、可一键回滚。
- 合入主干前不改基础 Compose/Temporal 默认；不接入真实 OAuth；开关默认关。

# 附：关键代码坐标
- legacy 服务：`internal/services/agent/legacy/{service,eino_agent,tools,context_builder,model_factory}.go`
- legacy 单体接线：`internal/services/core/bootstrap/embedded/kafka.go:100-107,189-253`
- microservices core Kafka：`internal/services/core/bootstrap/runtime.go:79-135`
- 小助手用户：`internal/services/core/application/assistant.go`、`internal/model/user.go`（`IsAssistant`）
- 群发送：`internal/services/message/domain/message_service.go:287 SendSystemGroupMessage`
- 群事件主题：`message.group.created`（已发布）
- 配置：`internal/config/config.go:552-565`（`ai.*` 默认）
