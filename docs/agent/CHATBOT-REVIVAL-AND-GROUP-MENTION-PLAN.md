# 小助手对话闭环恢复 + 群 @ 触发 —— 实施计划

> 状态：Route B 的 B1/B2 已在体验环境作为入站主链运行并完成复验。Route A 保留为默认关闭的 legacy 回退基线；两条路线保持互斥，可各自单独阅读。

## 0. 背景：仓库里并存两套 Agent

| | 旧「对话机器人」(legacy) | 新「受治理运行时」(current) |
|---|---|---|
| 代码 | `internal/services/agent/legacy/`（package `ai`，基于 eino） | `services/agent-runtime/`（TS）+ `internal/services/agent/`（Go） |
| 触发 | 私信小助手即自动回（`HandleDirectMessage` 消费 `message.direct.created`） | B1 私聊与 B2 群 @ 生成 `agent.interactive.requested`；也支持显式任务 |
| 多轮 | 有（最近 12 条上下文，`context_builder.go`） | 无（一 task 一回复） |
| 工具 | 5 个：查用户资料 / 搜历史 / 列会话 / 读会话 / 发系统消息（`tools.go`） | user.profile.read、conversation.list/read/search 与受控消息发送 |
| 模型 | eino：openai / ollama（`model_factory.go`） | DeepSeek via AI SDK |
| 装配 | 仅 embedded 单体（`embedded/kafka.go:103`） | microservices（体验环境） |
| 开关 | `ai.enabled`，默认 false | `agent_*_enabled` 分档 |

关键事实：
- 默认小助手 `UAI000000000000000001`（"Dipole AI"）仍是一等公民用户（`UpsertAssistant`），不加好友也能私信。
- legacy chatbot 代码活着、带全套测试；体验环境保持其回复开关关闭，避免与 Route B 双回复。
- `message.group.created` 主题已发布；群消息**无结构化 mention 字段**，@ 需文本解析。
- eino 依赖已在 `go.mod`（eino v0.9.17 + ollama/openai ext），core 镜像可直接编译 legacy。

## 0.1 体验环境 B1/B2 验收记录（2026-09-08）

- Remote GPU 的 `dipole-experience` 已将 Core 热更到 `0b1c3f52a`；镜像标签记录同一 revision，容器健康检查通过。
- Core 显式保持 `DIPOLE_AI_DIRECT_REPLY_ENABLED=false` 与 `DIPOLE_AI_GROUP_REPLY_ENABLED=false`；TS Runtime 的 B1/B2 入站触发保持启用，避免两条路线同时回复。
- 新注册、无 Definition 与 grant 的用户私聊已通过端到端验收：任务完成、Definition 固定为 `lowrisk-assistant:v1`、只发送一条助手回复，且 `message.assistant_reply.send` 审批已消费。
- 新注册用户创建群并 `@Dipole AI` 的端到端验收也已通过：任务完成、只发送一条群助手回复，且 `message.group_reply.send` 审批已消费。
- 复验命令（Remote GPU，2026-09-08）：`bash scripts/e2e-b1-inbound-interactive.sh` 与 `bash scripts/e2e-b2-group-mention.sh` 均通过。两次均使用新注册用户和新消息 UUID；B1 任务 `task:17c04102…`、B2 任务 `task:2ed73bfd…` 均收敛为 `completed:completed`，各自只有一条助手消息和一条已消费审批。
- 下一个正确性切片：失败 workflow 的 event ledger 可 reclaim/retry；订阅路径创建 Definition 后提供经审核的 owner grant 绑定；Definition 抽屉说明明确“私聊和群 @ 无需先创建 Definition”。

## 1. 目标与验收
- G1：私信小助手 → 自动 AI 回复，能调用工具（1v1 多轮对话恢复）。
- G2：在含小助手的群里 @小助手 → 群内 AI 回复（命中 @ 才触发）。
- G3：全程走 policy/capability 治理层，有审计与幂等，可一键回滚。

---

# Route A —— 快速恢复（基于 legacy eino chatbot）

> 定位：保留 legacy chatbot 作为可独立回滚的恢复基线。本节自包含，不依赖 Route B，体验环境默认不启用其回复路径。

## A0. 边界与并存策略
- A 路线让 **Go core** 跑 legacy chatbot 消费 `message.direct.created` / `message.group.created`。
- 体验环境的 TS agent-runtime 在 interactive 档负责 B1/B2 入站触发；Core 的 legacy 回复开关保持关闭。因此私信和群 @ 仅由受治理运行时处理。
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

> 目标：把 1v1 与群 @ 收敛到新运行时，统一审批、promotion 与审计，并在可靠性门禁满足后退役 legacy 接线。本节自包含，不依赖 Route A。

## B1. 入站直发触发交互任务
- 状态：已完成并在体验环境验证。
- 把发给 `UAI0001` 的 `message.direct.created` 在 `direct_target` 档接成"起 interactive task"（复用 `InteractiveTaskStartService` 的等价链路，或新增 inbound→task dispatcher）。
- 复用已上线的 assistant_reply 闭环（`AuthorizeInteractiveReply` + `createInteractiveReplyExecutor`），实现真·多轮 1v1。
- 会话上下文：交互任务读取直属会话最近 N 条作为 prompt。
验收：私信小助手多轮对话，每轮经 admission/approval/审计。

## B2. 群 @ 触发（治理版）
- 状态：已完成并在体验环境验证；群 conversation 从 consumed approval 的 resource scope 恢复，不依赖模型参数保存 scope。
- `direct_target` 档额外消费 `message.group.created`；群消息先过 `detectAssistantMention`，命中才 admit。
- Subscription 自动触发保持独立，后续仅在 owner Definition 存在 reviewed promotion grant 后启用。
- 新增自授权原语 `AuthorizeGroupReply`（类比 `AuthorizeInteractiveReply`），scope 精确指向该群会话；新增 group assistant_reply executor（走 `SendSystemGroupMessage`/群 AI 文本）。
- proto RPC + gateway/core 两处 allowlist + 单测，与现有 interactive 一致。
验收：群 @小助手 → 群内回复，全程 grant/approval/consumed 审计。

## B3. 工具移植与 legacy 退役
- 把 legacy 的查资料 / 搜历史 / 列读会话移植为新运行时 capability（proto + registry + policy + 单测）。
- `user.profile.read` 已迁移：Core 从 Task/Run 恢复 owner，Runtime 工具无 subject 参数且要求精确 owner `user/read` scope；只返回低敏简要资料。会话 list/read/search 已有受治理 capability。
- 第一方 MCP 现显式投影 profile、会话 list/read；检索启用时才额外投影 conversation search。每次调用继续经 Task/Run authority 与 Tool invocation 审计。
- 迁移完成后停用 A 路线的 core legacy handler，legacy 目录标记 deprecated / 移除接线。

---

# 执行顺序与里程碑

1. **B1/B2** 已完成：体验环境仅启用 Route B，私聊和群 @ 均使用低风险 Definition、一次性审批和 Temporal 任务。
2. **P0 可靠性**：修复失败 workflow 的 event ledger reclaim/retry，并让成功群 @ 任务稳定收敛为 `completed`。
3. **P1 订阅与工具**：将 Definition → Subscription → reviewed promotion grant 串成可见审核流程；继续收口 B3 的 legacy tool capability。
4. **退役评审**：在幂等、失败恢复、订阅审核和 Eval 门禁均有证据后，移除 Route A 的生产接线；代码目录再单独标记 deprecated 或删除。

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
