# 小助手对话闭环恢复 + 群 @ 触发 —— 实施计划

> 状态：Route B 的 B1/B2 已在体验环境作为入站主链运行并完成复验。Route A 保留为默认关闭的 legacy 回退基线；两条路线保持互斥，可各自单独阅读。

## 0. 背景：仓库里并存两套 Agent

| | 旧「对话机器人」(legacy) | 新「受治理运行时」(current) |
|---|---|---|
| 代码 | `internal/services/agent/legacy/`（package `ai`，基于 eino） | `services/agent-runtime/`（TS）+ `internal/services/agent/`（Go） |
| 触发 | 私信小助手即自动回（`HandleDirectMessage` 消费 `message.direct.created`） | B1 私聊与 B2 群 @ 生成 `agent.interactive.requested`；也支持显式任务 |
| 多轮 | 有（最近 12 条上下文，`context_builder.go`） | 有：每条入站消息创建一个可恢复 Task，`reply()` 读取同一会话最近 12 条作为短期上下文；默认不写入持久 Memory |
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
- 2026-09-08 热更后复验：以一个临时 owner Definition（无 promotion grant）触发新的私聊，Admission 先拒绝该 owner Definition，再仅对 interactive trigger 回退到 `lowrisk-assistant:v1`。任务收敛为 `completed:completed`、只有一条回复；Subscription trigger 保持拒绝，避免静默越权。
- 复验命令（Remote GPU，2026-09-08）：`bash scripts/e2e-b1-inbound-interactive.sh`、`bash scripts/e2e-b2-group-mention.sh` 与 `bash scripts/e2e-b1-owner-definition-fallback.sh` 均通过。三项均使用新注册用户与新消息 UUID；B1、B2 与 owner Definition fallback 的任务均为 `completed:completed`，各自只产生一条助手消息和一条 consumed approval。fallback 实跑任务为 `task:484f398a…`，owner Definition 未绑定任何 active promotion grant。
- 2026-09-08：失败 workflow 的 reclaim/retry 已完成 Remote GPU 受控演练：同一消息仍映射同一 Task，失败后 EventLedger release 可触发新的 Run attempt，且失败 Run 历史保持可审计。业务失败的 Workflow 会在结算 claim 后正常关闭，因此重投允许复用已关闭的 Workflow ID；运行中的 Workflow 继续拒绝并发启动。Runtime 以 Core-bound Run UUID 复核执行上下文，模型预算也按该 Run 分域，最终重投仅产生一条助手回复。见 AD-065。
- 2026-09-08：两轮入站 Context E2E 已通过：新用户在第一条私聊中提供唯一代号，第二条私聊要求复述；两条独立 Task 均完成且第二条回复包含第一轮代号。Route A 仍关闭，公共体验栈保持 11 容器。见 [receipt](../../benchmarks/agent-inbound-context-e2e-2026-09-08/)。
- 2026-09-08：隔离 `subscription_active` read-only smoke 已验证 Definition、owner-scoped Subscription、短期 fixture grant 和一条 Kafka 事件可收敛为一个 completed durable Task；模型调用存在且 Agent 消息为零，退出后公共体验仍为 11 个健康容器。fixture grant 只用于开发期验收。
- 2026-09-08：MySQL 契约已将 proposal/review 控制面与 active Subscription admission 串联：review 签发的 grant 可准入匹配 owner Subscription 并固定其 Definition/Subscription binding；revoke 后新 trigger 被拒绝。Remote GPU 的一次性 MySQL 8.4 容器实跑通过，公共体验保持 11 个健康容器。
- 2026-09-08：`subscription_active` 的 `control` smoke 已用默认关闭的 Gateway operator route 走通真实 proposal/review。fixture 仅预置 immutable completed shadow evidence 与 proposer/reviewer role grant；第二位 reviewer 批准后，Core 为 owner Definition 生成 grant，再投递一条 Kafka 事件并收敛为一个 completed durable read Task、模型调用存在、零 Agent 消息。Gateway 使用服务端 `proposedAt`，测试将 grant 置于短暂未来并等待生效以消除时钟竞争；Remote GPU 清理后候选资源为零、公共体验维持 11 个容器。
- 2026-09-08：`subscription_active` 的 `publication` smoke 已从 Runtime `PromotionEvidencePublisher` 经 mTLS Artifact RPC 发布 content-addressed evidence receipt，并将 receipt 交给同一 Gateway/Core 双人审核链路。Remote GPU 的合成 eligible sample 完成 grant 和一条 owner Subscription Task，模型调用存在、Agent 消息为零；候选容器和卷自动清理，公共体验维持 11 个容器。MySQL JSON metadata 重排导致的 Artifact replay 误冲突已修正为 repository canonicalization。该回执仅证明合成证据的发布与控制面集成，不证明真实模型效果。
- 后续产品切片：Definition 抽屉需明确“私聊和群 @ 无需先创建 Definition”；Subscription 列表现提供 activation readiness，用户可区分有效 grant、待 promotion 与已撤销状态。真实评审 evidence corpus、审核 UI 与共享环境发布仍待完成。Gateway 已提供受既有 operator grant 约束的 promotion evidence 只读端点，供审核页面读取已绑定 `promotion_evaluation` JSON；默认 Gateway promotion route 继续关闭。`control` 模式保留 SQL fixture 作为快速回归，`publication` 为 Runtime 发布路径验收。

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
- 认证用户的显式交互任务使用相同的 `agent.interactive.requested` admission 语义：无 Subscription 且没有可提升 owner Definition 时回退到 `lowrisk-assistant:v1`，不要求用户先创建 Definition。
- 会话上下文：交互任务读取直属会话最近 12 条作为 prompt，并按消息序列从旧到新投影为不可信文本；每条消息仍独立生成可恢复 Task，默认不写入持久 Memory。
验收：私信小助手多轮对话，每轮经 admission/approval/审计。

## B2. 群 @ 触发（治理版）
- 状态：已完成并在体验环境验证；群 conversation 从 consumed approval 的 resource scope 恢复，不依赖模型参数保存 scope。
- `direct_target` 档额外消费 `message.group.created`；群消息先过 `detectAssistantMention`，命中才 admit。
- Subscription 自动触发保持独立，后续仅在 owner Definition 存在 reviewed promotion grant 后启用。
- 新增自授权原语 `AuthorizeGroupReply`（类比 `AuthorizeInteractiveReply`），scope 精确指向该群会话；新增 group assistant_reply executor（走 `SendSystemGroupMessage`/群 AI 文本）。
- proto RPC + gateway/core 两处 allowlist + 单测，与现有 interactive 一致。
验收：群 @小助手 → 群内回复，全程 grant/approval/consumed 审计。

## B3. 工具移植与 legacy 退役
- 状态：能力迁移完成。legacy 的查资料 / 搜历史 / 列读会话已收敛为新运行时 capability，并由 proto、registry、policy 与测试覆盖。
- `user.profile.read` 已迁移：Core 从 Task/Run 恢复 owner，Runtime 工具无 subject 参数且要求精确 owner `user/read` scope；只返回低敏简要资料。会话 list/read/search 已有受治理 capability。
- 第一方 MCP 现显式投影 profile、会话 list/read；检索启用时才额外投影 conversation search。每次调用继续经 Task/Run authority 与 Tool invocation 审计。
- Route A 的 core legacy handler 在体验环境保持关闭，`internal/services/agent/legacy` 已标记为 deprecated 回退基线。新 Agent capability 只能进入受治理 Runtime；实际删除 legacy 代码仍等待订阅审核、Eval 与退役评审完成。

---

# 执行顺序与里程碑

1. **B1/B2** 已完成：体验环境仅启用 Route B，私聊和群 @ 均使用低风险 Definition、一次性审批和 Temporal 任务。
2. **P0 可靠性**：已完成。事件账本由 Temporal workflow 终态结算：成功才 complete，failed/cancelled release 后可 reclaim；定向与 Temporal 测试覆盖 dispatcher 交接和终态 activity。Remote GPU 已注入 Provider 故障并重投同一原始事件，确认新的 Run/Workflow generation 成功、账本完成且最终消息副作用精确一次；B1/B2 回归均为单回复 `completed`。
3. **P1 订阅与工具**：已完成 Definition → Subscription → reviewed promotion grant 的受控 Compose 闭环；Gateway 已补齐 operator-scoped evidence 只读 API，前端审核页与真实 evidence 归档继续推进。B3 legacy tool capability 已收口为受治理 read capability。
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
