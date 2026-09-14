# Dipole 面试项目审核与收尾清单

审核日期：2026-09-14。代码基线：`e280f2bd7770ac8ce18ca143af4fa33892d0d9d1`。

## 范围与结论

本报告依据当前源码、Compose、面试文档、测试实现和已有 benchmark，重点审核可演示能力与简历描述是否一致。本轮未运行测试、未部署、未连接共享环境；测试代码的存在不代表本轮验收通过。已有两份架构文档的未提交修改保持原样。

项目已有可靠消息、双 Timeline、权限边界与 Temporal 的技术基础。当前最明显的缺口是检索结果没有进入后续模型推理、体验启动步骤不完整，以及真实业务副作用的恢复证据需要补齐。收尾围绕现有产品流程进行，不扩展技术栈或治理平台。

## 改进顺序

| 顺序 | 编号 | 优先级 | 事项 | 状态 |
| --- | --- | --- | --- | --- |
| 1 | IR-05 | P1 | benchmark 凭据脱敏与导出修复 | 进行中：工作区修复已验证，历史暴露待评估 |
| 2 | IR-01 | P0 | 检索结果进入同一 Task 的二次推理 | 进行中：代码及 MySQL 契约已验证，完整体验待验收 |
| 3 | IR-02 | P0 | 完整体验启动流程 | 进行中：配置和启动说明已修复，完整体验待验收 |
| 4 | IR-03 | P1 | 模型提出写操作并复用现有审批 | 实现与本地 Temporal 测试通过，待真实模型体验 |
| 5 | IR-04 | P1 | 真实业务链路的恢复与幂等验证 | 真实 Worker 恢复通过，Core 数据库闭环待验收 |
| 6 | IR-06 | P1 | 可重复的性能实验与数字口径 | 待处理 |
| 7 | IR-07 | P2 | Memory、MCP 和高可用描述边界 | 待处理 |

状态采用“待处理 / 进行中 / 已验证”。只有满足该项验收条件并补充实际证据，才改为已验证。优先级表示产品影响，执行顺序先处理凭据暴露。

## IR-01：检索结果未回到模型

**现状与依据：** [executeShadowPlan](../../services/agent-runtime/src/events/shadow-processor.ts) 先生成计划，再执行工具，最后返回原计划；工具结果写入 trajectory。[Temporal Activity](../../services/agent-runtime/src/temporal/agent-task-read-activities.ts) 使用原 `plan.summary` 回复。该调用路径没有检索后的模型调用。

**面试追问：** 如何证明“历史讨论总结”使用了搜索命中，而非模型在搜索前生成的文本？

**改进动作：** 在同一 Task 中取回有界工具结果，经 Context Compiler 作为不可信 evidence 输入后续模型调用，再输出答案。复用现有 Capability、权限校验和 Temporal。检查 [ModelRouter](../../services/agent-runtime/src/models/model-router.ts) 按 Task 恢复输出的逻辑，区分首轮计划与后续回答，避免二次推理复用首轮缓存。

**验收条件：**

- [x] 确定性测试中，后续模型输入包含仅存在于工具结果中的事实，返回后续回答而非首轮计划摘要。
- [x] 检索与回答沿用原 Task；已完成步骤返回持久化结果，不重复执行工具。完整 Worker/IM 恢复验收见 IR-04。
- [ ] 无权限结果被服务端拒绝；无结果时如实回答；结果数量和上下文大小有界。
- [x] 测试能够捕获“执行了 search，却仍返回首轮 summary”的回归。

2026-09-14 实现说明：Active 路径有工具步骤时追加一次 answer 模型调用；无工具步骤仍直接回复。
计划与回答使用原 Task 下的独立模型审计阶段，各阶段沿用现有调用次数、超时和输出预算，
因此有检索的任务最多使用两份阶段预算。回答不再产生新工具循环。
部署前必须应用新增 `000052_agent_model_stage` 迁移，旧记录默认归入 plan；
同一 Task 已产生多个阶段时，down migration 会因唯一键冲突拒绝回退，避免丢弃审计历史。
尚未使用真实 Provider 与 Search 服务联调，当前测试不代表模型语义准确率或线上体验已验收。

## IR-02：体验启动步骤不完整

**现状与依据：** [README](../../README.md) 的 Agent 启动命令没有启用 `search` profile。[基础 Compose](../../deploy/compose/docker-compose.microservices.yml) 的 Search、Indexer、Elasticsearch 位于该 profile 下，Gateway 搜索开关默认关闭。[experience 配置](../../deploy/microservices/agent-experience.yml) 未补齐这些启动条件。基础配置还依赖内部 RPC secret 与证书；README 的说明主要强调模型 Key。

**面试追问：** 从干净工作区按 README 操作，能否直接体验聊天、检索和 Agent？

**改进动作：** 修正现有启动说明与 experience 配置，列出模型配置、证书、内部凭据、迁移和 Search 所需步骤，复用已有脚本与服务。

2026-09-14：README 已补充 Go 镜像构建、开发证书、内部 secret、模型参数、
`--profile search`、迁移和前端代理到 8080 的步骤。experience 显式开启 Gateway 搜索，
Agent/Gateway 等待 Search 健康；漏启 Search profile 会在 Compose 校验阶段报错。
修复嵌套变量表达式错误：只设置 `DIPOLE_AGENT_MODEL_API_KEY` 时不再强制要求旧变量
`DIPOLE_AI_API_KEY`。全部 Key 缺失时仍由 Runtime 拒绝模型调用。
现有 `scripts/check-compose.sh` 已加入该配置回归并通过；本轮未启动完整体验栈。

**验收条件：**

- [ ] 按文档从隔离环境启动，记录版本、命令和必要前置条件，不记录秘密。
- [ ] 登录、普通聊天、群 @AI、历史检索和 Agent 回复均可操作。
- [ ] 搜索命中来自已写入并完成索引的测试消息，能解释索引追赶延迟。

## IR-03：审批主要由固定命令触发

**现状与依据：** [requestedSystemMessage](../../services/agent-runtime/src/temporal/agent-task-read-activities.ts) 对 `/system ...` 在模型执行前直接进入审批。当前模型工具选择主要覆盖只读能力。已有审批机制可以证明等待与恢复，但不足以单独证明模型提出写操作。

**面试追问：** 写操作是否由模型根据意图提出？用户实际批准的是哪些参数和目标？

**改进动作：** 复用现有 Approval 和 Message Command，实现一个自然语言提出写操作的完整案例；审批绑定目标资源与参数。同步修正 [Agent 面试文档](INTERVIEW-AGENT.md) 对“所有写入必须人工审批”的笼统描述，明确受限自动回复与高风险操作的授权差别。

**验收条件：**

- [x] 自然语言请求产生明确的写操作提案和审批，批准前无写入副作用（确定性模型测试）。
- [x] 拒绝产生零副作用；重复批准只产生一次副作用（Temporal + 幂等命令测试替身）。
- [ ] 修改批准后的参数或资源不能沿用原批准；Core 执行时复核权限。
- [x] 普通 AI 回复仍可自动完成，文档准确说明该授权边界。

实现范围：当前 owner 与 AI 的私聊系统消息。模型只能提出正文，Runtime 从可信上下文确定目标；审批 checkpoint 固定正文与会话，恢复时不重复调用模型。`/system` 快捷路径保留。

## IR-04：恢复测试与真实业务证据之间仍有距离

**现状与依据：** [Temporal integration test](../../services/agent-runtime/src/temporal/agent-task-workflow.integration.test.ts) 包含 Worker replacement 场景，关键业务 Activities 使用测试实现，且由 `DIPOLE_AGENT_TEMPORAL_INTEGRATION` 开关控制。这证明测试设计覆盖了恢复，不等同于当前真实 Core、MySQL 和 IM 写入全链路已验收。

**面试追问：** 消息写入成功但 Activity 响应丢失，重试后如何保持一次业务副作用？

**改进动作：** 扩展现有测试或 smoke，接入实际业务实现，保留稳定 Task、调用标识和数据库幂等约束，不新增故障框架。

**验收条件：**

- [x] 等待审批后重启 Worker，再批准，原 Task 完成（实际 Temporal，本地隔离进程）。
- [ ] 重复批准与命令重试后，数据库中只有一条对应消息。
- [ ] 注入“消息已提交、响应丢失”场景，重试返回相同业务结果。
- [ ] 记录实际执行命令、Task ID、消息数量和测试版本，敏感字段脱敏。

简历使用“在指定故障场景下验证恢复与幂等”，不作无限定的零丢失、零重复承诺。

2026-09-14：现有 `agent-task-workflow.integration.test.ts` 新增自然语言提案场景，使用真实
ModelShadowPlanner、Temporal Activity 和消息执行器；模型、Core 授权与命令接收端为测试替身。
审批等待后替换 Worker，重复批准及提交后响应丢失重试收敛为一个 Task、一次模型调用和一条消息；拒绝为零条。
命令 `DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm --prefix services/agent-runtime test -- --run src/temporal/agent-task-workflow.integration.test.ts -t natural-language`
通过 2 项。首次因 CLI 下载超时失败，使用本地已有 Temporal CLI 缓存后通过，无远程部署。
全量 Agent 测试通过 677 项、跳过 30 项；typecheck、build 通过。真实 Core/MySQL 消息条数验收仍保留未完成。

## IR-05：benchmark 产物保存完整 JWT

**现状与依据：** 已跟踪的 [k6 summary 示例](../../benchmarks/ad005-projection-timing-2026-08-27/ad005-4343684-timing-regular-100.k6-summary.json) 包含完整 `token` 字段。审核未验证其有效性，本报告不复制令牌内容。

**面试追问：** 项目如何避免测试、日志和公开产物泄露凭据？

**改进动作：** 清理当前被跟踪产物中的凭据，修复产生这些报告的导出逻辑。检查清理是否影响已有校验和或引用，并同步处理。若发现仍有效的凭据，按实际环境处理失效；历史提交中的暴露需要单独评估，未经明确授权不重写 Git 历史。

**验收条件：**

- [x] 当前被跟踪 benchmark 的 JWT 扫描为零；9 份 JSON 中的 JWT 已替换，未改变指标。
- [x] 新生成报告仅导出指标与阈值，2 项测试及隔离 k6 运行确认 setup 数据不进入产物。
- [x] 对应 SHA256SUMS 已更新，脱敏说明见 benchmarks/REDACTION.md。
- [ ] 明确记录是否需要进一步处理历史暴露或凭据失效。

## IR-06：性能数字需要可重复实验支撑

**现状与依据：** [千人群 SQL 对照](../../benchmarks/ad005-conversation-batch-2026-08-29/README.md) 使用 `-benchtime=1x`，每组单次操作；报告明确限定为结构对照，不能推导端到端延迟的同等改善。

**面试追问：** 倍率如何得到？数据量、预热、并发量、采样次数和统计波动是什么？

**改进动作：** 复用已有 benchmark，固定环境和数据，进行重复采样。逐项核对简历数字与原始实验的对应关系，避免混用不同版本或不同测量层次的数据。

**验收条件：**

- [ ] 记录环境、版本、数据量、预热方式、并发量与采样次数。
- [ ] 多次采样报告中位数及适合样本量的统计指标；P99 必须有足够样本支持。
- [ ] 区分 SQL 投影耗时、吞吐和端到端延迟，保留原始输出。
- [ ] 每条对外性能描述能指向对应实验，删除未测量的占位数字。

## IR-07：产品描述与可选能力边界

**现状与依据：** [基础 Compose](../../deploy/compose/docker-compose.microservices.yml) 关闭 Memory；[experience](../../deploy/microservices/agent-experience.yml) 关闭 MCP Server 和 external MCP。基础 Kafka 为单节点、复制因子为 1。以上属于默认配置观察，不否定其他实现或实验环境的能力。

**改进动作：** 文档分别说明默认可体验能力、已有内部组件和可选实验。用连接生命周期、消息事务、用户同步、异步索引及长任务执行解释服务边界；高可用描述绑定真实测试拓扑。Cassandra、C++ 与外部 MCP 不扩展为本轮新任务。

**验收条件：**

- [ ] Memory、MCP 的对外描述明确实际入口与启用状态；无演示证据时不宣称默认可体验。
- [ ] 可独立部署与已验证集群高可用分别描述。
- [ ] Dipole IM 与 Dipole Agent 简历口径分开，每个核心组件均能对应用户需求或可靠性问题。

## 关闭记录

每项完成后在此追加一行，并更新上方状态；只记录实际验证结果。

| 日期 | 编号 | 修改范围 | 验证命令与结果 | 剩余限制 |
| --- | --- | --- | --- | --- |
| 2026-09-14 | 全部 | 建立审核与验收清单 | 文档整理；未运行功能测试 | 各项等待逐一改进 |
| 2026-09-14 | IR-05 | 9 份报告脱敏、1 份校验和更新、两个 k6 入口采用安全 summary | Node 单元测试 2 passed；k6 单迭代离线 fixture 通过；JSON、JWT 扫描与 Bash 语法检查通过 | 历史提交未改写，令牌有效性未在线验证；尚未运行完整负载测试 |
| 2026-09-14 | IR-01 | 工具证据返回、回答阶段、阶段审计迁移、sqlc/TS 查询生成 | Agent 全量 676 passed / 28 skipped；独立 MySQL 两文件 16 passed；typecheck、build、sqlc generate、Go generated 包编译通过 | 真实 Provider/Search 体验与 Worker 业务恢复待验收；未提交、未部署 |
| 2026-09-14 | IR-02 | experience Search 依赖、模型 Key 回退、README 完整前置步骤 | scripts/check-compose.sh 通过；缺少 search profile 时明确拒绝；git diff --check 通过 | 完整注册/登录、真实模型与索引演示尚未执行；未提交、未部署 |
