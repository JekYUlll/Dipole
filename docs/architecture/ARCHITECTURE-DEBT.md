# 架构债务台账

- 2026-09-02：开发期 `agent-subscription-shadow.yml` 已将 owner Definition/Subscription 管理与 matcher 对照封装为显式 Compose profile；它固定 `shadow + direct_target`，并关闭 Task Control、Memory、MCP 与 External MCP。基础 Compose、Subscription trigger 和 Runtime 灰度均未改变，真实观察窗口与 `AD-034` 的评审证据继续待完成。

- 2026-09-02：Remote GPU 的隔离、loopback-only Interactive Active Compose smoke 已验证用户 Definition 的 HTTP/gRPC/SQLC 持久化闭环：认证 owner 重放创建、列出目录，并复核单条 Definition 的 owner、Assistant、只读权限和 wildcard scope。Definition-only project、容器与 volumes 已自动清理；Subscription trigger、reviewed Shadow 与 Runtime 灰度继续由 `AD-034` 跟踪。

- 2026-09-02：固定 `conversation.read` 的 owner Definition API 已从 Subscription 控制门禁中独立为 `gateway.agent_definition_enabled`。默认配置仍关闭，Interactive Shadow/Active 的隔离 Compose profile 可显式启用；其不授予订阅写入、subscription trigger 或消息写入。Subscription 的 reviewed Shadow、Runtime 灰度与回滚验证继续由 `AD-034` 跟踪。

- 2026-09-02：standalone Core 已用与 embedded 路径一致的 SQLC Agent 组合装配 Definition catalog、Subscription resolver 和 owner-scoped Subscription control；Gateway 调用不再因独立服务缺少控制面而返回未装配。`gateway.agent_subscription_enabled=false`、受控 Shadow 观察、Runtime `subscription` 灰度和可执行回滚仍是启用前置条件，`AD-034` 保持处理中。

- 2026-09-02：Gateway/Core 的 owner-scoped user Definition 创建已具备，固定为 Assistant 的 `conversation.read` wildcard 权限并使用稳定 ID 重放；客户端不能指定 tenant、owner、Agent 或写权限。嵌入式 Core 已装配该控制面。standalone Core 尚未装配 Subscription/Definition control，因此 Gateway 开关继续默认关闭；须完成同一服务组合、受控 Shadow 观察和回滚验证后再启用。

- 2026-09-02：Agent Definition 现按 `(tenant, owner, agent)` 查询，migration v58 允许不同 owner 为同一 Agent 使用相同 version。Subscription 的可信 principal 已与该 owner query 对齐；嵌入式 direct-target 保持 Agent owner。用户自助创建 Definition、Subscription 默认启用、真实 Shadow 观察窗口和 Runtime 灰度仍未完成，`AD-034` 保持处理中。

- 2026-09-02：Event Subscription enforced Runtime 现已按 Subscription fan-out，可信执行主体改为记录的 `created_by_id`，Core 复核 owner 与 Definition/Subscription binding，Task/EventLedger 幂等键分别加入 subscription identity。定向 TypeScript 回归覆盖两个 owner 命中一条事件的独立派发、Temporal binding 与重复键分离；Go 回归待使用非 Xlings 工具链或 Remote GPU 复核。本机默认仍为 `direct_target`，subscription trigger 和共享环境切换保持关闭，须先完成 AD-034 的 reviewed corpus、Shadow 观察窗口、rollout gate 与受控部署证据。

- 2026-09-02：同版本 `81d8da66` 的 Remote GPU 隔离 `interactive_active` Compose 已再次完整覆盖 active 写入与 Worker 替换：临时 `user_gray` manifest、mTLS、独立 Kafka/Temporal 和短期 grant 仅用于候选项目；Worker 在 `waiting_approval` 后重启并恢复，deny 维持零 Tool/Message，重复 approve 收敛为一次 Tool、一次 Message 和两条 Sync Inbox。退出后 grant、项目、卷和回环端口均已清理。该确定性 `/send` fixture 不调用模型，不能替代 browser HITL、shared tenant、Core/Message replacement、partial-effect rollback、容量或成功率证据，`AD-009` 保持处理中。

- 2026-09-02：同版本 `dc0129a7` 的隔离 Remote GPU `interactive_active` Compose smoke 已覆盖真实 Agent Worker 替换。Task 到达持久 `waiting_approval` 后，测试只重启 Agent Worker 并等待 `/readyz`；随后同一批准重放继续收敛为一次完成 Tool、一次 Message 与两条 Sync Inbox，拒绝路径仍为零 Tool/Message，项目、卷和 `127.0.0.1:18131` 均在退出后清理。该回执关闭 active Compose 单 Worker 替换后的审批恢复缺口；Core/Message 联合替换、EventLedger lease、部分副作用 rollback、浏览器 HITL、shared tenant 与容量仍由 `AD-009` 跟踪，不能据此填写成功率或性能结论。

- 2026-09-02：完成远端分支收敛。810 条已合并 ref 已直接回收，40 条未合并 tip 先进入 `archive/remote-branch-consolidation-2026-09-02` 后按活跃 worktree 与 Epic 身份筛选，最终远端保留 6 条分支。当前保留 Agent/Frontend Epic 与两条活跃功能分支；其余分支可由 archive tag 恢复，但恢复前仍须在新分支完成语义复核与主干对齐。

- 2026-09-02：分支与 worktree 曾按测试、retry、receipt 和小修复持续拆分，形成 65 条本地分支与 31 个 worktree，降低主干集成速度。当前主干经正常 merge 对齐远端并通过全量门禁，创建 `archive/branch-consolidation-2026-09-02` 保全 57 个回收 tip，移除 27 个 clean worktree，保留 8 条本地长期分支和 4 个受保护 worktree。远端收敛已在同日上一条记录完成；后续新增远端分支继续受 archive、owner 和 merge 条件约束。

- 2026-09-02：Interactive read-shadow 的多会话 owner scope 已补 Remote GPU 同版本回执：认证 owner 可见两条会话时，Task 进入 `waiting_input`；伪造 request ID 返回 `409` 且不改变等待态，确认已展示候选后，持久轨迹精确收敛为一次 `conversation.list`、一次确认会话的 `conversation.read`、零次未确认会话读取与一份 digest Artifact。同一 fixture 的 owner cancel 从 `waiting_input` 收敛到 `cancelled/user_cancelled`，未完成 read 计划行的授权和完成数均为零。随后仅将 Agent 镜像更新到 `d60ace70`，以 2 秒确认 TTL 验证 Gateway start `202`、owner query/Timeline `200`、Task/Run `cancelled/input_expired` 和零授权读取；`input_expired` 状态转换本身要求持久 `waiting_input`。该回执关闭 Gateway 到 Temporal 的恢复、取消、到期及读取 scope 精确绑定的受控功能缺口，详见 [read-scope receipt](../../benchmarks/agent-read-scope-confirmation-2026-09-02/)。Worker/Core/lease 联合故障、共享开发环境和独立人工评审多路径窗口仍未完成；不得将两会话 fixture 外推为成功率、模型质量、性能或 active 写入结论。

- 2026-09-02：Remote GPU 的 Trace 绑定 read-shadow 样本暴露 Core Run/Task 终态投影缺口：`agent_runs` 已为 `completed`，父 `agent_tasks` 仍为 `running`。`PersistentAgentRunAdmissionV1.Finish` 现已在每条 Run 终态路径以 CAS 收敛同名 Task 状态，并允许相同终态重放补齐此前部分提交；Task 处于 `waiting_approval` 等中间态或出现冲突终态仍 fail closed。Agent application、Agent gRPC 回归、同版本 [N=1 terminal convergence receipt](../../benchmarks/agent-terminal-convergence-2026-09-02/) 与真实读取 [N=2 Shadow Eval](../../benchmarks/agent-shadow-eval-window-2026-09-02-read-n2/) 均通过，故代码、单样本部署和固定单会话读取分支均已解决。仍待独立人工评审的多路径任务集覆盖多会话选择、失败/重试与 shared-development 情形；在此之前禁止将现有小样本回执外推为成功率或 promotion 结论。

- 2026-09-02：新增只读 `shadow-eval-review-pack-cli`，为终态观测导出 Task/Run/Trace/资源的域分隔哈希、能力轨迹、证据指纹和计量完整性。子记录缺少授权、延迟或单次 attempt 审计时，包仍可供人工失败分类，并以 `evaluatorEligibility=blocked` 固定原因；最终 evaluator 继续拒绝该样本。该包刻意不能生成可执行 manifest 或推导标签；审核者仍须在受控工作区独立填写 outcome、trajectory、permission、retrieval 和 cost，并用绑定 manifest 执行评测。当前 clean candidate 的多样本窗口、晋级和简历成功率结论继续保持关闭。

- 2026-09-02：clean `f72e47cf` 已在 Remote GPU 独立 Compose 中完成修复后 read-shadow 回归。低敏 receipt 与数据库聚合确认完成 EventLedger、Shadow Run、模型调用和 `conversation_digest`，消息表为零，Gateway 仅绑定 `127.0.0.1:18117`。此证据覆盖单条受控 Kafka 事件，尚未形成固定人工评审多样本集，禁止据此填写任务成功率或放开 promotion。

- 2026-09-02：Remote GPU 同版本 Shadow Smoke 发现 Planner 将事件裸 `target_uuid` 传入要求 canonical conversation key 的 Runtime evidence reader，Temporal Run 因本地输入校验失败，未产生 Model Call、Shadow Step 或 Artifact。代码已改为传递事件 `conversation_key`，direct/group 定向回归与 TypeScript 构建通过。修复前的 failed Run 不计入成功率；必须以 clean revision 重建候选，重跑真实 Kafka、Capability RPC、模型调用、Artifact 和人工评审多样本窗口后，才可更新任务成功率或 promotion 结论。

- 2026-09-02：Remote GPU 用 legacy Docker builder 构建多服务候选时发现，`DIPOLE_BINARY` 即使未被依赖安装命令引用，只要在该层前声明也会切分 Docker cache。镜像已将所有服务特有 build args 与 provenance 标签下移到 `ca-certificates`/`tzdata` 层之后，并用层序测试固定。该修复降低后续候选的网络和 Docker I/O，不影响现有镜像的 provenance 复核、运行验收或回滚要求。

- 2026-09-02：Shadow Eval 窗口已将最低评审样本数变为显式、回执可见的采集门禁。调用者可通过 `DIPOLE_AGENT_SHADOW_EVAL_MIN_MANIFESTS` 固定本窗口阈值；低于阈值会在读取或拷贝任何 manifest 前失败，v2 `manifest-set.json` 同时保存所需与实际数量，既有 v1 回执保持原 schema 可验证。默认 `1` 仅服务单样本调试；成功率 claim 仍须使用人工复核的固定多样本窗口，且不能由该门禁单独证明。

- 2026-09-02：Agent Task 控制面新增受认证的低敏 Runtime status 查询。Gateway 从会话派生 principal 并通过内部共享凭据转发；Runtime 只返回 mode、Temporal activity mode、Task control 与交互写开关，拒绝匿名调用且不返回 Provider、端点、凭据、Task 或消息数据。该状态用于诊断任务创建是否已装配，`AD-009` 的真实任务、HITL、Worker 故障、共享环境和性能证据继续开放。

- 2026-09-02：同版本 `9884b848` 的隔离 Remote GPU `interactive_active` Compose smoke 已通过 Gateway JWT 调用 Runtime status，并复核 deny 零副作用、approve 单次 Tool/Message 与两条 Sync Inbox。候选容器、卷和回环端口在退出后已确认清理。该确定性 fixture 不覆盖真实模型效果、浏览器 HITL、共享 tenant、Worker/Core/Message 故障、部分副作用回滚或容量，`AD-009` 保持处理中。

- 2026-09-02：`reviewed_shadow` Eval 窗口现具备固定任务集输入边界。评审者先对 manifest 目录计算内容摘要，采集器在连接运行中 Agent 前复核摘要和单一 candidate version，并生成不含 Task/Prompt/用户/消息/标签正文的 manifest-set receipt。Remote GPU shell fixture 通过正常、失败和输入漂移拒绝路径。该能力使多样本成功率窗口可复跑；当前尚无同一固定任务集的真实多样本运行，成功率 `[XX]%` 与 shared-development 结论继续保持空缺。

- 2026-09-02：Interactive Agent 的 completed Tool terminal 现对 `UNAVAILABLE` / `DEADLINE_EXCEEDED` 执行一次同载荷重放。Core 已有精确 terminal 幂等校验，因此 Runtime 只重发首次生成的 invocation、结果摘要、action reference 和延迟字段；若两次都处于不确定状态，Runtime 不会把可能已提交的 completed 覆盖为 failed。Remote GPU Node 22 定向 Vitest `10/10` 与 typecheck 通过。`AD-009` 仍跟踪 Worker 替换后的审批恢复、真实 Core/Message 跨进程响应丢失、部分副作用 rollback、浏览器 HITL、shared tenant 和容量证据。

- 2026-09-02：Interactive active Compose 回归揭示 Message command 从 Kafka 入队到 MySQL receipt committed 存在短暂间隔，Core 若立即固化 Tool action reference 会将已提交消息误判为冲突。Tool 审计现只对 `absent` receipt 在 `2s` 内有界确认，committed 后再完成审计；任何错误、nil receipt 或超时后的 absent 仍 fail closed。Remote GPU loopback-only 同版本候选已验证并发 deny 的零副作用及并发 approve 的单次 Tool/Message/Sync 收敛，project、volumes 和临时 grant 均已清理。`AD-009` 继续跟踪 Core/Message 响应丢失、Worker 替换、部分副作用 rollback、浏览器 HITL、shared tenant 和容量证据。

- 2026-09-02：Agent Message command 已增加隔离 MySQL 8.4 的回包丢失恢复 smoke，详见 [Interactive Message MySQL Receipt](../agent/AGENT-INTERACTIVE-ACTIVE-MESSAGE-MYSQL-RECEIPT.md)。真实 SQLC repository 在受认证 Core-to-Message gRPC 调用中持久化一条 Message 与 metadata；代理随后返回 `UNAVAILABLE`，Core 以稳定 `client_message_id` 读取 receipt 恢复相同消息。数据库断言 sender/target 两条 Sync Timeline 项各自唯一，测试容器自动清理。该证据没有启动 Compose、Kafka、Temporal、shared tenant 或部分副作用 rollback，`AD-009` 继续开放。

- 2026-09-02：Agent Message command 的 Core-to-Message gRPC 回包丢失恢复已增加受认证 transport 回归，详见 [Interactive Message Transport Receipt](../agent/AGENT-INTERACTIVE-ACTIVE-MESSAGE-TRANSPORT-RECEIPT.md)。服务端先提交受控 system message，代理在 response 前返回 `UNAVAILABLE`；Core 使用稳定 `client_message_id` 查询 receipt 并恢复同一条 Message，精确提交计数为一。Remote GPU Go 1.27 已通过 Agent application 与 Message gRPC 包回归。该测试使用 bufconn 与内存模型，不能代表 MySQL、Compose、跨进程网络、Worker 替换、部分副作用回滚或共享环境恢复；这些继续由 `AD-009` 跟踪。

- 2026-09-02：Interactive active 消息写入已增加“Message 提交后 RPC 响应丢失”的定向 Temporal 故障门禁，详见 [Interactive Active Retry Receipt](../agent/AGENT-INTERACTIVE-ACTIVE-RETRY-RECEIPT.md)。对 Core `UNAVAILABLE` / `DEADLINE_EXCEEDED`，同一 Task/Run/会话/内容派生稳定 Message command ID，运行中的 Tool Invocation 保留至 Activity 重试；Remote GPU Node 22 的内存 Temporal 用例证明两次调用使用同一命令标识、模型 side-effect store 仅记录一次提交，最终只写入一次 completed Tool Invocation。该证据不连接真实 Core、Message、MySQL、Kafka 或 Compose，不能代表部分副作用回滚、Worker 替换或共享环境故障恢复；这些继续由 `AD-009` 跟踪。

- 2026-09-02：`interactive_active` 的干净同版本 Remote GPU 隔离 Compose 已补齐真实跨服务回执，详见 [Interactive Active Remote Receipt](../agent/AGENT-INTERACTIVE-ACTIVE-REMOTE-RECEIPT.md)。同一 `430c9e38` 且 `dirty=false` 的镜像在 loopback-only 项目中验证 owner 并发 deny 只产生一次 `approval_denied` 且 Tool/Message 为零；并发 approve 只消费一次 approval，并产生一次完成 Tool、一次 action reference 与恰好一条 Message。开发 promotion grant 已在验证后撤销。该结果关闭此前“干净同版本、owner deny、重复 approve/deny、副作用精确计数”缺口；Shared 环境、浏览器 HITL、Worker/Core/Message 故障重试、部分副作用回滚、MCP 和指标结论继续由 `AD-009` 跟踪。

- 2026-09-02：修复 Shadow Runtime 的 candidate-version admission 漂移。candidate version 只属于 active promotion binding；Shadow admission 现在固定传空值，避免 Core 在 Task 创建后拒绝无效的 Shadow Run。Remote GPU Node 22 的定向回归 `15/15`、typecheck 与 production build 已通过；干净 `70bd4c74` 镜像的隔离 Compose 回归进一步证明认证 Task 创建 `202` 后，Temporal Workflow 与持久 Run 均完成，且 Run 为 `shadow / candidate_version=NULL / completed`。候选无可读会话，仅生成只读摘要 Artifact，消息写入数为零。该证据不扩大 shared 环境、active authority、写 Capability、MCP 或 Memory；这些继续由 `AD-009` 跟踪。

> 2026-08-31 Claim-first 更新：简历中“零丢失、零重复副作用”、Cassandra/Sync/Search/端到端 P99 和 Agent 任务成功率均须以 [简历 Claim 验收矩阵](../guides/RESUME-CLAIM-READINESS.md)定义的可重跑报告为准。当前优先补齐消息与 Durable Task 故障 receipt、Sync 观察、数据面基准和 Agent Eval；未完成项保持为占位符或限定范围表述。

- 2026-09-01：Interactive Shadow Compose 的有效配置门禁已恢复。此前脚本检查 Gateway 的 control secret 固定值，却漏传对应的 `DIPOLE_GATEWAY_AGENT_CONTROL_SECRET`，即使 Overlay 保持默认关闭的只读边界也会在静态渲染失败。门禁现在使用隔离测试值并有变量级回归测试；该修复不启动服务、不扩大 Shadow 权限，真实 shared Compose receipt 继续由 `AD-009` 跟踪。

- 2026-09-01：Interactive Agent 的隔离 Temporal 集成覆盖已增加 owner `denied` 与重复 Signal 边界：同一 pending approval 的并发重放只调用一次持久 resolution，Task 确定性收敛为 `cancelled/approval_denied`，写步骤计数保持零。Remote GPU 的干净 `f9634d3c` 候选以 Node 22 通过两份 Temporal 集成文件 `16/16` 与 TypeScript typecheck；该门禁不连接真实 Core、Message、MySQL、Kafka 或 Compose。共享环境的 owner deny、重复 approve、Activity 重试、消息副作用计数与回滚 receipt 继续由 `AD-009` 跟踪。

- 2026-09-01：Remote GPU loopback-only 的 `interactive_active` 隔离 Compose 已验证一次真实受认证写闭环：Gateway Task create `202`，owner approve `202`，Temporal Task/Run、Tool Invocation 均为 `completed`，Approval 为一次 `consumed`，且通过持久 action reference 关联到恰好一条 Message。过程暴露并修复两处跨服务边界缺口：Core Agent RPC allowlist 漏掉消息命令方法，以及 standalone Core 的 Tool 审计在 remote Message transport 下仍访问空本地 repository 而 panic。候选使用 dirty development images 与短期直接续期的 developer promotion grant，结束时已 revoke 并清理临时令牌。**仍未收口：** clean same-revision image/provenance、owner deny、并发或重复 approve、Core/Message/Worker 故障重试、回滚、浏览器 HITL、外部 MCP 和可统计成功率；这些仍由 `AD-009` 跟踪，不能据此填写生产或简历指标。

- 2026-09-01：`interactive_active` 已将交互写入与 `read_active` 隔离成独立 Runtime/Temporal/Compose 契约。它允许 Control API 和已批准的直属会话 `/send`，要求 mTLS、专用队列与显式开关，其他扩展仍被 profile 拒绝。当前只有隔离单元、类型和 Temporal 编排证据；共享 Compose 的 owner approve/deny、重复消费、Activity retry、消息副作用计数与回滚 receipt 继续由 `AD-009` 跟踪。

- 2026-09-01：Interactive Agent 已具备默认关闭的显式直属会话消息写入编排。活动只识别 `/send <内容>`，由可信 ExecutionContext 推导 `direct:<owner>:<agent>`，并将 canonical 参数、scope、approval ID、Task/Run 写入 durable checkpoint；批准恢复后经既有 MCP approval gate、一次性消费、Tool Invocation 与 Core 消息命令执行。Remote GPU 隔离验证覆盖 activity、组合器和 Temporal `waiting_approval -> completed`，未连接共享 Core、Temporal、Kafka、MySQL 或 Compose。真实批准/拒绝/重复消费/故障回滚 receipt、active Compose overlay 和浏览器体验继续由 `AD-009` 跟踪。

- 2026-09-01：前端 V3 统一已收口**应用侧**色板。语义 token（rail/accent/ink/line 系）重映射到品牌板并与 `design-tokens.test.ts` 契约、`.pen` 变量锁步；Agent 组件身份色从旧青绿别名归位为"海军蓝身份文本 + 金色进度标记"；ChatView/Settings/Memory 的硬编码青绿、微信品牌绿、emoji 图标与硬编码渐变全部清零，新增功能性 `--dp-success` 承接在线/同步/成功信号；`--dp-v3-*` 重复色阶已退役、LoginView 迁移到语义 token；`brand-v3-ui-brief.md` 的估读十六进制已改回品牌板实测值。`rg` 确认 `frontend/src` 已无旧青绿/微信绿硬编码色。

- 2026-09-02：设计源 `design/dipole-ui.pen` 的**帧内联色**债已收口。确定性脚本 `scripts/pen-v3-recolor.mjs` 把两代并存的变量单代化（363 处遗留变量引用重指向 V3 后删除 6 个遗留变量，新增 `success`/`success-soft` 对齐 `--dp-success`），并把 240 处内联 fill/stroke 旧盘 hex（64 种）按角色映射到 V3 变量；脚本幂等、带残留旧盘 hex 与悬空引用断言，`.pen` 变量 38→34，`check-pencil-design` 通过，pen 无头渲染抽查已确认视觉为 V3。**残留的唯一设计债缩小为纯渲染项**：`design/exports/` 的已批准评审 PNG 仍是改色前快照。无头 pen CLI 的 `get_screenshot` 只出 400px 缩略图、无缩放参数，无法复现已批准的全分辨率基线（如 login/chat 2880×1800、foundations 3360×1000），因此**不以缩略图降质覆盖已批准导出**；faithful 全分辨率重出须在 pen.dev 桌面应用（`pen interactive --app desktop`）逐帧按 node ID 重渲染，属无配色决策的机械跟进。设计源与运行时/服务 authority 不受影响。

- 2026-09-01：品牌资产已收口为脚本生成的单一来源（`scripts/generate-brand-assets.mjs` + `scripts/generate-brand-wordmarks.mjs`），色值按 V3 品牌板实测校正，`npm run test:brand` 拦截手改 SVG 造成的漂移；favicon 与 Login 标识镜像进前端自身根目录，构建不再跨出 `frontend/` 引用 `docs/`。遗留的 4 个青绿 SVG 与 `#07c160` 占位 favicon 已退役。**仍未收口**：`frontend/src/styles/design-tokens.css` 与 `design/dipole-ui.pen` 中并存三代色板（微信绿 `#07C160`、青绿 `accent #00A86B`、V3），且 `brand-v3-ui-brief.md` 记录的是早期估读十六进制值；ChatView 等页面级 UI 仍使用旧青绿语言。页面级 V3 迁移与 token 统一在前端设计轨道中单独验收，不改变运行时或服务 authority。

- 2026-09-01：多会话读取范围已改为 Task owner 确认。发现步骤产出两个及以上会话时，Run 在 claim 读取 Step 前暂停并返回 `wait_input`，因此暂停点不占用 Step lease，恢复后仍能按同一 Step 编号 claim；select Form 最多提供 8 个候选并显式披露发现总数。恢复期不重新规划：已验证的 `conversation.list` 到 `conversation.read` 结构由代码重建，避免二次模型规划改变 Step 编号与 trajectory 重放语义，代价是 plan 摘要经 Workflow history 携带，从 MySQL 不可变 plan 读回仍待独立切片。多于一对 discovery 的 plan 在需要确认时 fail closed。用户选择按不可信输入处理，必须命中 checkpoint 候选集合与确定性 request ID，Core 保持最终读取授权。真实 approve/deny/expire receipt、共享环境窗口与该路径端到端评测仍由 AD-009 跟踪。

- 2026-09-01：修复 active Approval 的跨服务模式漂移。此前 `RequestApproval` 在 Core adapter 中固定为 shadow，而 MCP grant/consume 与消息命令只接受 active，导致审批通过后仍无法进入真实受控写链路。RPC 已显式传递 Runtime mode，空值兼容映射为 shadow；Core allowlist 仅增加 `ResolveApprovalGrant`、`ConsumeApproval` 和 `ExecuteMcpMessageCommand` 三项既有 Agent RPC。Go/TypeScript 定向回归与生成契约检查通过。真实浏览器审批、active Compose、消息副作用 receipt 与回滚演练仍由 `AD-009` 跟踪。

- 2026-09-01：Remote GPU 仅安装 pinned protobuf 工具链时，TypeScript 生成器会因标准库 timestamp 文件未处于声明的 proto path 而失败。生成脚本现复用已解析的 include 目录作为第二个 `--proto_path`；该修复只改善跨语言契约生成，不改变任何 Runtime authority。

- 2026-09-01：Agent Task 控制面已增加 Runtime HTTP 到 Temporal 的审批集成门禁。Remote GPU `9217d826` 在内存 Temporal Test Server 通过 owner-bound pending read、foreign denial、approved Signal 与 completed terminal Activity 的 `8/8` 验证。该测试未接入浏览器、Gateway、真实 Core approval persistence 或共享环境，因此真实 HITL UI receipt 继续由 `AD-009` 跟踪。

- 2026-09-01：开发阶段的 Remote GPU 资源策略已收敛为直接复用本轨道 Dipole project，登录会话和 GPU 任务仅作为资源快照；运行依赖允许受控 `sudo` 安装。该调整不放宽宿主网络、Docker daemon、其他项目资源或生产切流的操作边界。

- 2026-09-01：复用长驻 Agent Interactive Shadow 候选的 `.env` 单独重建 Core 时，遗漏宿主 `DIPOLE_INTERNAL_CERT_DIR` 会将缺失的 `core.pem` bind source 创建为目录，Core 因 mTLS 证书加载失败重启。已先恢复原镜像并确认健康，再以显式证书目录更新 `6d274a54` Core；Gateway、Timeline 与所有项目服务恢复健康。操作手册已固定该变量，后续将由候选部署入口统一注入，避免依赖手工环境继承。
- 2026-09-01：Agent Task Timeline 仅在受认证 Task 的 `waiting_approval` 事件同时包含 approval ID 时，才显示进入既有 owner-scoped 审批页的入口；终态审批和无 approval ID 的事件不显示操作。组件测试覆盖该条件和现有 Artifact 路由。真实 Shadow deny/HITL UI receipt、外部 MCP 与 Worker/Core/lease 联合故障仍由 AD-009 跟踪。

- 2026-09-01：Remote GPU 已在 `6beab05d` 归档隔离 Temporal Worker replacement 的 approval/input recovery receipt。两条路径均固定 `running -> waiting_* -> running -> completed` 修订序列；approval 路径包含一次注入终态重试并只有一次持久写入，input 路径拒绝两次无效/过期 Signal 后只恢复一次。CLI 可独立复核归档 receipt。运行使用内存 Temporal Test Server，未启动 Compose 或连接 Core、Kafka、MySQL、共享 tenant、active authority；联合 Worker/Core/lease 故障与 HITL UI 继续开放。

- 2026-09-01：默认关闭的 Gateway Artifact 控制面已增加受限的 `conversation_digest` Markdown 正文读取。Gateway 仍通过 mTLS Core RPC 从认证 principal 重新解析 owner，并复核正文长度与 SHA-256；公开响应只包含 Artifact ID、媒体类型和正文。Remote GPU 隔离验收验证 owner metadata/content 为 `200`、foreign content 为 `404`。对象键、Metadata JSON、其他 Artifact 类型和通用下载继续关闭，前端正文展示留待独立前端里程碑。

- 2026-09-01：Artifact 前端里程碑已接入 owner-scoped `conversation_digest` 阅读区。客户端仅在 metadata 精确匹配 `conversation_digest` 与 `text/markdown` 时读取正文，并复核响应的 Artifact ID/媒体类型；正文不可用时 metadata 继续保留且可重试。页面以文本阅读区呈现 Markdown 源文，未增加下载、对象键、Metadata JSON、公开 URL 或写入口。Remote GPU Node 22 已通过定向 Vitest `7/7`、typecheck、生产构建和 Chromium 功能/视觉回归；Pencil v2 brief 已归档，实际画布增量继续由 AD-044 跟踪。

- 2026-09-02：Remote GPU 的全新 loopback-only Compose 候选以同一 clean revision `d7fee99a` 构建 Core、Gateway、Message、Sync、Search、迁移、修复与 Agent 镜像，并完成 JWT Interactive Task `202 -> completed -> Timeline` 两页续页验收。Runtime 继续固定 `shadow/read_shadow`，MCP、Memory promotion、active authority 与写 Capability 未开启；此开发期单任务结果不构成成功率、生产发布、公开体验或共享观察窗口证据。
- 2026-09-02：Go 微服务候选镜像已调整为先建立可跨 revision 复用的 Alpine 依赖层，再写入 provenance 标签和服务二进制。该改动降低同机候选构建的重复网络与 Docker I/O，且由静态层顺序测试锁定；它不替代同版本 OCI 标签复核、Compose 运行验收或回滚证据。
- 2026-09-01：全新 Remote GPU 候选已从空 MySQL 卷完成 migration v57，当前 Core 与 read-shadow Agent 启动。验收发现 Artifact Timeline 的拼接 event ID 超过 MySQL `VARCHAR(64)`，写入被投影层吸收；修复改用 Artifact 的 64 位内容寻址 ID，并在领域层校验上限。重建同版本 Core 后，新受认证只读任务已完成，Artifact metadata 与 Timeline 中唯一对应 Artifact 事件均返回 `200`；该证据未扩大 active authority 或写 Capability。

- 2026-09-01：Remote GPU 真实 Provider 验收表明，自由 `record` 输入 schema 会让模型偶发构造裸 `conversation.read` target，即使 prompt 已声明 discovery 规则。计划 schema 已收紧为 `conversation.list` 和固定 `$discovered.previous` 的 `conversation.read`，并保留执行层验证。候选数据库必须先应用 `000057_agent_model_run_stages`，两阶段 plan/synthesis 环境需配置 `DIPOLE_AGENT_MODEL_MAX_CALLS>=2`；同版本迁移镜像和可重跑体验 receipt 仍待完成。

- 2026-09-01：空会话列表属于新用户的正常状态。依赖发现的读取步骤现持久化为 `skipped/no_discovered_conversation`，不再触发 Activity 重试或远端读调用；摘要仍基于已完成的 List/skip 输出生成。多会话选择和用户确认读取范围仍待后续编排切片。

- 2026-09-01：隔离交互 Task 验证显示单轮 Planner 会在缺少任何发现结果时生成 `conversation.read`，随后因无法从伪造或空的 conversation key 推导可信 target 而失败并触发 Temporal 重试。当前已将单轮模型动作面限制为 `conversation.list`，维持事件驱动的预取读取与 MCP 的受控读取；多轮 orchestrator、已验证 discovery result 到 read target 的数据流绑定和该路径的端到端评测仍是 Agent P0。

- 2026-09-01：Remote GPU 隔离交互 Shadow 已在 DeepSeek V4 Flash 下验证一次新用户只读任务：HTTP 查询为 `completed`，Task `workflow_status` 与唯一 Run 为 `completed`，模型调用、Step、Artifact 均精确为 `1`。`agent_tasks.status` 仍是 Shadow 策略投影并保持 `running`，因此 API、评测与前端必须以持久 Workflow/Run 终态表达用户可见完成；共享环境、多轮 conversation read、写能力、MCP 与 active authority 仍无此证据。

本文档记录已确认但暂缓处理的架构风险、兼容性缺口和可清理冗余，便于后续按优先级滚动治理。

## 维护约定

- 状态使用：`暂缓`、`处理中`、`已解决`、`接受风险`。
- 优先级使用：`P0` 阻断发布、`P1` 应在正式启用相关能力前解决、`P2` 进入后续迭代、`P3` 按需清理。
- 新问题使用连续编号 `AD-NNN`，保留历史编号，不复用已关闭条目。

### 本轮进展

- 2026-09-01：Remote GPU 已在 `f0dcf98a` 运行并归档 [Approval v2 receipt](../../benchmarks/agent-mcp-approval-shadow-2026-09-01-v2/)。denied grant、consumed grant replay 与 failed-operation replay 都被拒绝，三类路径均未产生新增 effect；同次 MCP drill 继续验证本地 Tool/Artifact、EventLedger 去重、过期 readiness 与 mTLS identity denial。审批 UI、共享服务、真实外部 MCP、凭据生命周期与 active authority 继续开放。

- 2026-09-01：Approval gate drill receipt 升级为 v2，将已拒绝 grant、已消费 grant 重放和失败操作后的重放作为独立布尔断言，并继续绑定相应的零副作用计数。v1 保留为历史 evidence；v2 Remote GPU receipt、审批 UI 与共享环境验证仍待完成。

- 2026-09-01：Remote GPU 已在候选 `3c1f3eba` 复跑 disposable External MCP/approval Shadow drill，并归档 [低敏 receipt](../../benchmarks/agent-mcp-approval-shadow-2026-09-01/)。本地 MCP Tool/Artifact、EventLedger 重启去重、过期 readiness 拒绝、Core mTLS identity denial 和一次 approved fixture operation 的副作用基数均通过。MySQL AIO 兼容参数仅作用于该 disposable drill；共享服务、真实外部 DNS/TLS、凭据生命周期、approval UI deny 和 active authority 继续开放。

- 2026-09-01：External MCP/approval 的 disposable Shadow drill 曾因 Remote GPU Linux AIO 配额触发 MySQL `io_setup() EAGAIN`。drill Compose 现仅对自身 MySQL 增加 `--innodb-use-native-aio=0`，并由配置门禁锁定；该变更不影响基础微服务或其他候选项目。

- 2026-09-01：Remote GPU 的新候选 checkout 曾因上一次中断留下的 `node_modules` 在 `npm ci` 中报 `ENOTEMPTY`。`node-test` 现只匹配该确定错误后原子隔离候选 app 的 ignored 目录并重试一次；其他安装失败仍直接退出，隔离目录保留供诊断。该修复不改变 lockfile、已运行容器或共享工作树。

- 2026-09-01：Remote GPU 的隔离交互 Shadow 候选完成两次公开 JWT Task admission 到 Timeline cursor 续页的只读验收；两条 Task 均收敛为 `completed`，每条 Timeline 的前两页各返回两条事件。Gateway 使用 `4ab924b87` 的专用候选镜像，Core/Agent 为兼容的既有候选，因此该证据只说明混合候选的开发兼容性。详细边界见 [Agent Interactive Shadow Remote Receipt](../agent/AGENT-INTERACTIVE-SHADOW-REMOTE-RECEIPT.md)。同版本镜像、可重跑低敏 receipt、受控观察窗口、active authority 与写 Capability 继续作为独立门禁。

- 2026-09-01：Remote GPU 同时运行多个隔离 MySQL 时，宿主 Linux AIO 使用量达到 `55,300 / 65,536`，新候选初始化触发 `io_setup() EAGAIN`。新增只作用于候选 project 的 `remote-gpu-mysql-aio-compat.yml`，保留基础参数并增加 `--innodb-use-native-aio=0`；此兼容模式不改变已有服务，验证结束后随候选 project 回滚。

- 2026-09-01：Remote GPU Node 验证从两阶段 `npm ci` 加 `npm install` 调整为单阶段 `npm ci --include=optional`。此前第二次安装可能与 npm 目录替换发生 `ENOTEMPTY`；新路径保留 lockfile、可复现依赖和 optional package，同时减少一次网络与磁盘操作。

- 2026-09-01：Remote GPU bundle 回退已修正为用 `HEAD` 生成完整、可检出的 Git bundle；目标 SHA 继续通过独立参数与远端 `rev-parse` 固定校验。裸 SHA 会被 Git 解释为空归档，修复后才进入受控远端验证。

- 2026-09-01：为 Remote GPU 的 origin clone/fetch 固定可配置的 20 秒 timeout。远端 GitHub 不可达时，开发验证会转入 commit-pinned bundle 回退，不会长期占用会话或阻塞后续 Agent 同版本门禁；该 timeout 不改变任何运行中 Compose 服务。

- 2026-09-01：Remote GPU 候选源码同步曾因远端 `ssh.github.com:443` 超时而无法进入测试。`remote-dev.sh` 现为每个 clean candidate 创建 commit-pinned Git bundle 并通过既有 SSH 上传；远端 origin clone/fetch 失败时才回退至 bundle，随后验证 exact commit 并在退出清理。该改动不启用隧道、代理或共享服务，后续同版本 Compose evidence 仍需独立执行。

- 2026-09-01：根 README 已从旧 PNG 品牌板切换到受版本控制的 `dipole-v3-brand-lockup.svg`，与 Web Login 的 V3 SVG 使用同一海军蓝/信号红/轨道金语言。旧 PNG 仅保留为历史评审资产；该文档改动不改变服务 authority 或前端设计验收范围。

- 2026-09-01：修复 Go 微服务镜像构建 context 在循环中累积二进制的问题。每个服务现在创建独立临时 context，并由子 shell 的 `EXIT` trap 在成功或失败后清理；静态回归测试同时锁定单二进制 context 和清理边界。下一次 Remote GPU 同版本 smoke 仍需记录每个 context 的实际大小与总构建耗时。

- 2026-09-01：V3 前端设计的 Pencil CLI 调用按临时文件/原子替换策略两次超时，未修改 canonical `.pen` 或批准导出。V3 SVG、additive token 和 Login 已可独立构建，完整 Chat/Agent 画板、全局 token 迁移和视觉回归仍依赖成功的 Pencil 增量生成；禁止将当前兼容切片表述为全站改版完成。

- 2026-09-01：Remote GPU 隔离 Interactive Shadow 候选已验证认证用户从 Gateway 创建任务、异步 admission、Temporal 执行到终态读取的完整只读链路，receipt 归档于 [`agent-interactive-control-2026-09-01`](../../benchmarks/agent-interactive-control-2026-09-01/)。Provider 使用 `deepseek/deepseek-v4-flash` 且显式禁用 thinking，避免 JSON-text 输出为空；该配置目前只在候选环境采用。Gateway/Core 未重建到 Agent `c9f3f424`，同版本发布、体验 URL 验收、active authority、写 Capability、MCP 和统计成功率仍需独立门禁。

- 2026-09-01：Remote GPU interactive Shadow 复验发现：任务已被 Core 所有权授权且投影为终态时，Temporal 对关闭 Workflow 的 Query 返回不可用，Gateway 随之错误映射为 `404`。Task control 现仅在该特定终态条件下返回 Core 持久投影；运行中、缺投影或任意其他依赖错误保持失败关闭。后续仍需以同版本镜像重跑完整交互 receipt，当前不构成 active authority 或用户体验验收。

- 2026-09-01：Remote GPU 已以 clean `676a6d93` 完成同版本 Core/Gateway/Message/Sync/Agent 候选 smoke。私聊写入后重启 Core，Message、Outbox 与目标 Inbox 精确均为 `1`，低敏 receipt 归档于 [`microservices-same-revision-smoke-2026-09-01`](../../benchmarks/microservices-same-revision-smoke-2026-09-01/)。该结果覆盖 atomic Inbox 的一条服务恢复路径；Agent interactive control、Cassandra 主读、Kafka/broker/in-flight 故障矩阵与 A6 Web Sync 观察继续独立验收。

- 2026-09-01：Remote GPU 同版本 smoke 观察到每个 Go 微服务镜像重复传输完整 `dist` 上下文。构建脚本现只为目标二进制创建临时 Docker context，并在缺失或无执行权限时失败关闭；服务镜像模板继续只复制一个 `/app/service`。该优化不改变镜像 provenance、服务隔离或默认 authority，后续需在下一次同版本 smoke 记录实际 context 大小与总耗时。

- 2026-09-01：C++ Realtime Delivery 的 `primary` 投影已与 Go body-free Timeline 主路径对齐：direct/普通 group 仅生成 `sync.item.notify.v1`，`shadow` 仍生成完整事件加 locator，冲突策略 fail closed；热群 notify + pull 保持原有处理。Ubuntu 24.04 容器门禁通过 `14/14` CTest。该修复仅收敛候选实现语义，C++ 性能晋级门槛未通过，默认 Go authority 和 C++ 灰度关闭状态不变。

- 2026-09-01：审计发现 `message.timeline_notify_mode=primary` 在 Gateway Kafka 与 embedded Dispatcher 中仍同时发送完整消息和 locator，与 body-free 主路径契约冲突。primary 现只向接收方投递 `sync.item.notify.v1`，发送者保留 `chat.sent` 回执；`shadow` 继续双投递以支持观察，`off` 保留完整消息，热群聚合不变。Go direct/group/embedded 回归通过；真实 Cassandra 主读观察、自动停止和回切证据继续由 A6/AD-019 门禁约束。

- 2026-09-01：Agent Runtime 本地全量门禁曾因默认关闭的 Approval mTLS drill 在 suite 注册期读取远程变量、离线 security Eval 缺少默认 token availability 以及 Temporal 只读夹具缺少授权审计 sink 而不可复现。三处测试契约已对齐，当前离线运行通过 `158` 个文件、`796` 项测试，`10` 个显式外部依赖测试跳过，并通过 TypeScript typecheck 与 production build。Remote GPU 同版本 Compose、Kafka/Temporal/Capability RPC 演练仍需独立执行，不能由本地测试替代。

- 2026-09-01：Agent Shadow Runtime 将 Provider thinking 设为显式、Provider 专有的默认关闭选项，并将单次 Planner 的模型可见 Capability 收紧至 `conversation.list`。这样受限 JSON-text 预算不会被默认 reasoning 消耗，模型也不能在未获得会话发现结果前构造任意读取目标。多轮绑定、写 Capability、active authority 与 MCP 仍由 AD-009 的独立门禁约束。

- 2026-09-01：A6 Web Sync observation 的候选绑定已从单一 bundle 文件扩展为完整静态发布目录摘要。候选目录以稳定相对路径和内容 SHA-256 生成摘要，空目录、符号链接及任意资源集漂移均 fail closed；单文件兼容入口保留。真实浏览器 24 小时窗口、100 个 match、Prometheus 原始响应归档与责任人批准仍未完成，旧 Offline 兼容窗口和默认客户端模式保持不变。

- 2026-09-01：Remote GPU 的长驻隔离交互候选以新临时用户复验了 Agent Task 创建到终态查询：Gateway 接受只读请求并返回 `202`，Task 在有界轮询中收敛为 `completed`。候选 Gateway/Core 为 `406c3154`，Agent 镜像为 `thinking-4e9740a0`，存在受控版本偏差；它支持跨版本 Shadow 兼容性，不能替代同版本候选、可重跑 receipt、active authority、写 Capability 或任务成功率的验收。后续体验候选应将 Gateway/Core/Agent provenance 固定为同一 revision，再纳入 Claim 证据。

- 2026-09-01：个人资料入口补齐了当前密码验证、bcrypt 重哈希和当前 session 撤销的密码更新闭环。当前仅撤销发起修改的会话；“修改密码后撤销全部设备令牌”需要先让 Session/Token Store 提供用户范围的可审计失效能力，作为后续安全增强，不将当前实现表述为全设备登出。

- 2026-09-01：密码更新的隔离端到端验证发现资料缓存按设计脱敏 `PasswordHash`，认证中间件的 cached user 不能直接参与 bcrypt 对比。流程现经已认证主体的手机号加载权威记录并核验 UUID；用户资料缓存继续不保存密码哈希。后续认证能力需保持“敏感凭据从权威 store 获取、资料 cache 只承载脱敏投影”的边界。

- 2026-09-01：体验环境实测发现 Agent Task 创建页的 Function prop 默认值返回了函数而非 UUID 字符串，前端在网络请求前失败，Gateway/Temporal 控制链不会收到请求。现已修正并以组件回归测试锁定；Remote GPU 的同 revision `71bf6507` 候选已重建 Gateway 静态资源，验证认证、Task 创建、Temporal 终态和 5 条 Timeline 事件。该回执只覆盖 read-shadow，任务成功率与写能力证据仍待独立收集。

- 2026-09-01：增加 reviewed Shadow Eval window collector。它只执行已评审 manifest，保存每份低敏 report、输入与去重 Trace/Suite 汇总，并保留有效失败窗口以统计失败分类；没有自动任务创建、标签生成或环境切流。收集器从运行中 `agent` 容器读取 clean OCI revision，拒绝缺失或 dirty provenance，避免脚本 checkout 与实际评测镜像漂移。Remote GPU 已归档 [受控完成子集 N=2](../../benchmarks/agent-shadow-eval-window-2026-09-01-n2/)；五类报告全部通过，但 `100%` 仅描述该子集，当前尚无固定多样本任务集和共享环境窗口，任务成功率继续保留占位符。

- 2026-09-02：Shadow Eval 对外汇总升级为 `shadow-summary-report.v2`，移除原始 Trace ID。运行时仍在受限输入中以 Trace 去重并关联审计，归档只保留 suite 哈希和聚合统计；历史 N=2 样例已按同一限制语义转换。后续共享观察窗口也必须遵循该边界。

- 2026-09-02：Remote GPU 已归档 [N=4 安全跳过窗口](../../benchmarks/agent-shadow-eval-window-2026-09-02-n4/)。它只覆盖 `conversation.list` 成功、随后可信空发现使 `conversation.read` 成为 `not_required/no_discovered_conversation` 的受控路径；四例均通过五类结构性 Eval。固定单路径 cohort 不能填写总体任务成功率，也不能替代恢复、多轮检索、写能力或共享环境证据。

- 2026-09-01：同一受控栈的一条 Provider 空 JSON-text 失败事件在持久 Run 中保留 `model run budget exhausted`，但模型调用缺失 token 计量，现有五类 Eval 会因不完整 Cost observation fail closed。后续需让失败调用输出明确的计量可用性/不可用性并纳入失败分类，禁止以通过样本替代整体成功率。

- 2026-09-01：Remote GPU 受控窗口暴露汇总 schema 只接受 64 位摘要、与 OCI 的 40 位 Git revision 不兼容。契约已放宽为两种有效 revision 长度并覆盖回归；窗口重跑前不产生汇总结论。

- 2026-09-01：Remote GPU 的 Node `22.12.0` 重跑 Temporal 集成套件时发现“模型结果后置确认丢失”夹具未随 Step 授权审计契约更新，导致本应重放的已完成 Step fail closed。夹具现提供审计 sink 并断言授权审计精确为一次；修复后两个 Temporal 集成文件 `10/10` 通过。该结果只覆盖内存 Temporal Test Server 的隔离重放，Core restart、EventLedger lease expiry、共享环境与 active authority 仍须独立 receipt。

- 2026-09-01：Remote GPU 的 disposable read-shadow Compose 已生成新的 [五类 Shadow Eval 报告](../../benchmarks/agent-shadow-eval-2026-09-01-rerun/)。候选 `agent-runtime@064568d9` 的 Outcome、Trajectory、Permission、Retrieval 和 Cost 全部通过；Retrieval precision/recall 均为 `1`，并绑定持久 Step lease 的精确授权 scope。该数据只有隔离 `N=1`，不能推导任务成功率、共享环境质量、active authority 或写 Capability 安全性；应收集版本一致、去重 Trace 的受审窗口后再填写简历指标。

- 2026-09-01：真实 read-shadow Eval 发现控制面基线被计入 Retrieval precision。adapter 已将其排除于 Retrieval 指标，并在 `agent-runtime@064568d9` 的隔离复跑中取得 `1.0/1.0`；控制面基线继续保留在 Context/Trajectory 审计中，当前没有成功率结论。

- 2026-09-01：Compose 新增专用 `dipole_agent_eval` 只读账号。Remote GPU 隔离 read-shadow 已以该账号生成五类报告，并确认零行 `UPDATE` 被 MySQL 拒绝；自动评测、共享环境窗口和 active authority 仍未启用。

- 2026-09-01：Shadow Eval 的 Permission evidence 已改为读取 Step lease 内持久化的 `resourceType/resourceId/action/decision`，并对 manifest 逐项核对；旧、部分、空值和非 `allowed` 记录全部拒绝。隔离 MySQL/Runtime 拓扑的新报告已通过该校验，旧撤回样本仍不得恢复为成功率结论。

- 2026-09-02：Remote GPU 的只读 review pack 发现一个完成 Run 含有无授权的 `conversation.read` Step。复核后该行来自可信 `conversation.list` 的空结果，执行器按设计未准备或调用读取能力，却以 completed skip 写入 trajectory。Eval observation 现读取受限 `output_json`，仅接受 `conversation.read + completed + null authorization + {status:"skipped",reason:"no_discovered_conversation"}` 的精确组合；该组合在审阅包中显示为 `not_required`，不计 Tool 调用。其余任意空授权继续 `blocked`，因此旧 review pack 与任何结构漂移样本仍不可进入评测或成功率。

- 2026-09-02：该分类已在 Remote GPU 的现有 `agent-shadow-eval-f72e47cf` 隔离 Compose 上用新的干净 Agent 镜像复验。一次性容器只经专用只读 Eval 账号读取 Task/Run 观测，输出 `eligible`、列表 Step 的 allowed scope 与读取 Step 的 `not_required/no_discovered_conversation`，并经标识符泄露检查；运行中 Compose 服务、MySQL、Kafka 与消息数据未写入。候选源通过 patch 落地在隔离目录，镜像 provenance 对应其干净内容提交；主线 Git revision 仍由该切片的受审提交固定。样本数仍为一，任何成功率或 promotion 结论继续禁止。

- 2026-08-31：EventLedger lease expiry 现有 `dipole.agent.event-lease-reclaim.v1` receipt，要求过期 claim 被第二次 claim 回收、旧 token 完成失败且最终 completed 行唯一。Remote GPU 已在 loopback-only MySQL 8.4 临时容器通过真实集成 `3/3`、receipt 单测 `4/4` 和 Runtime typecheck；该结论只覆盖消费 ownership，Temporal Workflow、共享 Kafka/Temporal 与 active authority 仍需独立门禁。

- 2026-08-31：隔离 Core restart 演练暴露 `BUILD_IMAGE=1` 只构建 Go 镜像、Compose Agent 可能沿用旧 `latest` 的版本漂移。`docker-build-microservice-images.sh` 现按同一 revision 构建 `dipole-agent`，并由静态测试锁定；Remote GPU 已在候选 `a7bc03ef` 的新 Agent 镜像下重跑，Core 重启后 Gateway 与单次 Ledger/Task/Run/模型调用/Artifact 均收敛。默认 authority 不变。

- 2026-08-31：read-shadow Core restart 演练现可输出 `dipole.agent.core-restart-read-shadow.v1` 低敏 receipt。脚本仅在一次隔离 Compose Core 重启后同时确认 Core readiness、Gateway 代理和同一事件的 Ledger/Task/Run/模型调用/`conversation_digest` Artifact 精确收敛时写入 SHA-256 绑定结果，且固定 `production_authority=false`。Remote GPU 已在候选 `a7bc03ef` 完成运行并由镜像内 CLI 复核；Worker replacement 与 Core restart 联合场景、lease expiry、共享 tenant 和写 authority 继续作为 Agent P0 门禁。

- 2026-08-31：Approval gate 联合演练已升级为 `dipole.agent.approval-gate-drill.v1` 低敏 receipt。Remote GPU 生成的 receipt 经独立 CLI 复核 exact effect 基数、mTLS 类型、canonical SHA-256 与 24 小时窗口；artifact 为 gitignored 临时文件。该证据覆盖 fixture operation 的授权与重放边界，未覆盖 IM 持久化、service-side commit 不确定性、审批 UI 或共享 Shadow。

- 2026-08-31：Remote GPU 在 disposable MySQL/Kafka/Temporal/Go Core mTLS/local MCP 拓扑完成 Approval gate drill。Runtime 使用真实 `AgentCapabilityRPCClient` 验证 approved grant 单次执行、denied/consumed 零执行与 operation failure 后不自动重放；同轮 External MCP receipt 继续通过。脚本现在将 `DIPOLE_NODE_BIN` 的目录置于 `PATH` 首位并以路径标记驱动 lockfile 重装，避免 Node 18 启动 Node 22 依赖。该 fixture operation 不写 IM，active write、审批 UI、共享 Shadow 与提交后不确定性 receipt 保持开放。

- 2026-08-31：补齐 Core Agent mTLS caller allowlist 对 `ResolveApprovalGrant` 与 `ConsumeApproval` 的显式授权。隔离认证测试现验证 request、approve、exact grant 与单次 consume 的闭环，并验证错误 shared secret 和非 Agent 证书拒绝。真实写投递、审批 UI、共享 Shadow 观察与 service-side commit 后不确定性 receipt 继续作为 Agent P0 门禁。

- 2026-08-31：External MCP Shadow drill 已补齐干净候选 worktree 的 Node 依赖前置：仅在缺少本地 Vitest 时使用显式 Node 相邻 npm 安装 lockfile 依赖。此前 Go/mTLS fixture 成功后会因缺依赖中止，无法生成有效 Shadow evidence；修复后仍只运行 disposable 资源与只读路径。

- 2026-08-31：修复后在 Remote GPU 的隔离 MySQL/Kafka/Temporal/Go Core mTLS/本地 MCP 拓扑完成 Shadow drill。EventLedger `2/2`、Tool `1`、Artifact `1`，重启重复被抑制、过期 readiness 被拒绝且 identity denial 通过；receipt 为 24 小时有效的低敏开发期证据，`production_authority=false`，共享 Shadow tenant 与 active write authority 继续开放。

- 2026-08-31：Agent Temporal 增加 `worker_replacement_approval_resume` fault receipt v1。隔离 Temporal Worker replacement 演练将状态修订、终态写入重试和副作用基数通过 SHA-256 绑定；状态或单次副作用漂移固定为 `ineligible`。共享环境的 Core restart、lease expiry、input resume 与归档 receipt 仍待完成，当前结论不扩张至 active authority。

- 2026-08-31：同一 Temporal fault receipt 已覆盖 Worker 替换后的 Elicitation input resume：无效值和过期 request ID 不恢复任务，精确输入只恢复一次。该证据仍为隔离 Temporal，Core restart、lease expiry 和共享环境归档保持开放。

- 2026-08-31：README 已切换到用户提供的 V3 双产品品牌板，并使用可核验的技术/许可证 badge；`docs/images/dipole-brand-v3.png` 作为后续 IM 与 Agent 产品视觉的一致性参考。页面级 UI 迁移仍在前端设计轨道中单独验收，不改变运行时或服务 authority。

- 2026-09-01：Context Ablation Eval v1 已建立 baseline/retrieval/memory 的统一低敏对照汇总。真实 Task/Context/Memory 审计查询 adapter、人工评审任务集与共享 Shadow 样本仍待完成，当前结果不能外推模型效果。

- 2026-09-01：Context Ablation 实验绑定现由 migration `000056` 与 SQLC 查询持久化，固定 case SHA-256 与三种条件到独占 Task/Run。只读观测 adapter、评审任务集和运行证据尚未完成，因此该表不代表效果结论或默认 Runtime 行为。

- 2026-09-01：Context Ablation observation adapter 已将只读绑定观测与人工评审的低敏 manifest 汇编为 Eval 输入，并拒绝非终态记录、缺失授权/延迟/Token 计量、未知路由价格、Case 重复或候选版本漂移。评审任务集、共享 Shadow 窗口与可复现报告 CLI 仍待完成，当前不可外推为模型效果。

- 2026-09-01：Context Ablation 报告 CLI 已复用 Agent Eval 的只读 MySQL 账户，并固定一个 manifest 对应一个 experiment 的三条件聚合。受控 Shadow 任务集、人工复核记录和窗口级效果证据仍待完成，CLI 成功不能作为模型效果或发布提升依据。

- 2026-09-01：Context Ablation 已补齐评审 manifest 的 JSON Schema/低敏示例，并将 binding 表加入 Eval 账号的最小只读授权。隔离环境尚未迁移到 `000056` 或生成三条件受控样本；共享 read-shadow 仍停留在旧 migration，不能作为该能力的证据。

- 2026-09-01：Context Ablation 的 baseline/retrieval/memory 已拆为默认不加载的独立 Compose overlay，固定互斥 Context 开关和独立 Temporal queue。隔离 fixture 预置、三次真实 Task/Run、binding 写入、只读 CLI 报告和窗口归档仍待完成；共享 read-shadow 配置没有改变。

- 2026-09-01：AI SDK Shadow Planner 的会话 hydration 已改用事件的 `target_uuid` 调用 Core `ReadConversation`，避免将 `conversation_key` 误作目标用户或群标识。隔离 fixture、三条件受控 Task/Run 与窗口级报告仍待完成，因此尚无新的模型效果结论。

- 2026-09-01：隔离微服务 smoke 现可按需导出低敏 Agent Task/Run receipt，使临时栈销毁前的三条件绑定成为可能。receipt 不含消息或模型正文；同源 fixture、三次真实执行、binding 写入与聚合报告仍待完成。

- 2026-09-01：Context Ablation 增加隔离 MySQL 预检，验证 migration `000056`、binding 表和 Eval 账号的只读权限。它不覆盖真实三条件运行、fixture 审阅、Task/Run binding 或窗口级效果报告；这些步骤仍须在独立 Compose 项目完成。

- 2026-09-01：隔离 MySQL 8.4 预检发现 `000056` 未显式继承 Agent Task/Run 的 `utf8mb4_unicode_ci`，导致外键创建拒绝；migration 已修正并由预检静态断言覆盖。真实三条件 fixture 与运行证据仍待完成。

- 2026-09-01：Shadow Eval 汇总 Runtime 已接受 40 位 Git revision，但发布 JSON Schema 曾仅允许 64 位摘要，导致外部 Schema 校验与 OCI provenance 不一致。Schema 已对齐并由 Runtime 测试锁定；窗口仍仅代表受控 Shadow 样本。

- 2026-08-31：Remote GPU 以 `53a4edf7` 在独立 Compose 项目完成 Message Service 的持久化后重启与同一幂等键重放。最终 Message、Outbox、目标 Inbox 均为 `1`，退出后候选容器、卷和网络均清理；receipt 归档于 [`microservices-message-recovery-2026-08-31`](../../benchmarks/microservices-message-recovery-2026-08-31/)。该证据只覆盖一个 post-persistence service restart，Kafka/broker/in-flight 故障矩阵继续开放。

- 2026-08-31：候选消息恢复演练的 readiness 和首次持久化检查现在在失败时输出受限 `compose ps/logs` 与 `wscli` 尾部，避免基础服务未就绪被无上下文地归因为消息幂等缺陷。失败路径继续自动清理，只有归档的成功 receipt 才能作为恢复副作用证据。

- 2026-08-31：Remote GPU 候选消息恢复演练发现停止态 `docker compose exec` 客户端会无限阻塞 readiness/计数探针。`smoke-microservice-isolated-images.sh` 已将这些调用收敛为默认 20 秒的 `SMOKE_EXEC_TIMEOUT_SECONDS`，并在 5 秒 grace 后强制结束；发生该类问题时演练失败并走隔离项目清理，干净重跑 receipt 仍待归档。

- 2026-08-31：微服务 smoke 已支持只读 `read_shadow` 的 Compose overlay、受控 Compose 环境文件、事件发布后 Core 重启和模型/Artifact 绑定断言。Remote GPU 已在独立 `dipole-read-shadow-restart` 项目和专用 loopback 端口完成演练，确认 EventLedger、Task/Run、完成的模型调用与 `conversation_digest` Artifact 收敛，退出后容器数为零；默认基础 Shadow、写 Capability、MCP 与 active authority 保持关闭。

- 2026-08-31：基础 Compose 的 metadata Shadow Agent 已显式屏蔽宿主 `.env` 遗留的 v2 route context profile，维持固定 v1 Context Compiler 的可启动性；AI SDK/active overlay 仍负责显式启用 v2。该隔离消除 Remote GPU 并行环境中的配置漂移，未改变默认 Shadow authority 或 active 开关。

- 2026-08-31：微服务 smoke 支持可选 `RESTART_CORE=1` 隔离 Core 重启，在重启后复核 Core readiness、Gateway 代理与既有 Agent EventLedger/Task/Run 幂等。Remote GPU 已在独立 `dipole-core-restart-final` Compose 项目、专用 loopback 端口和独立卷中成功完成，脚本退出后确认容器数为零；默认路径保持不重启。该证据未覆盖需要 Capability RPC 的 read-shadow model 调用，后续恢复演练继续独立跟踪。

- 2026-08-31：Agent Capability RPC 的重连包装器改为在每次方法调用时解析当前 gRPC channel。Core `UNAVAILABLE` 后，即使上层缓存了方法引用，下一次事件级调用也会进入 replacement channel；transport 继续不重放失败调用，Kafka/EventLedger 仍负责幂等重试。隔离 Core 重启/Temporal 收敛演练继续待补。

- 2026-08-31：默认关闭的 Agent OAuth callback handoff executor 在私钥解封前和 Provider processor 前均复核 durable handoff 的 lease/expiry。过期检查失败会在产生外部副作用前释放 lease；processor 或 completion 结果不确定时保留 lease，避免把不确定副作用重新排队。callback HTTP、Provider exchange、token 生命周期和默认运行时装配仍由 OAuth release gate 限制。

- 2026-08-31：Cassandra read-rollout 已可把不可覆盖的 Message Service Prometheus 起止快照转换为 evidence v1。转换器拒绝 route/verification counter 回退、未知标签、histogram bucket 漂移和延迟覆盖缺口，`mysql_fallback` 同时归入 MySQL 最终路径与 fallback 比例。真实共享环境观察、责任人审核和读比例扩大仍由 AD-043 的运行证据门槛约束。

- 2026-08-31：补齐主链路外部阻塞期间的并行治理规则。前端设计、只读体验、视觉回归、文档入口和图表可在独立分支推进，但不改变服务 authority、默认 feature flag 或真实环境证据门槛。该规则减少等待窗口造成的工程停滞；共享环境切流、负载测试和 active 能力仍按各自验收条件执行。

- 2026-08-31：Remote GPU 的 DeepSeek V4 Flash shadow 审计显示已完成真实 Kafka/Capability RPC/模型/Shadow Plan 链路的至少一条成功调用，但当前观察样本仍存在 Provider `response_format` 不可用、空输出与 JSON-text 包装造成的失败。Runtime 已将单一、短包装的 JSON 对象恢复为本地 Zod 校验输入，并继续拒绝多对象或不合法结构；需在 Temporal `read_shadow` 实机启动后收集新的成功率、Run 终态与 Artifact 证据，AD-009 不关闭。
- 2026-09-01：实机 Interactive Task 复现 DeepSeek V4 Flash 的默认 `thinking` 可在 `1024` 输出 token 内只返回 `reasoning_content`，AI SDK 可见正文为空并使 JSON-text 调用失败。Runtime 新增 Provider-scoped `DIPOLE_AGENT_MODEL_THINKING_MODE=disabled`，仅在显式选择时透传 `thinking.type=disabled`；同一大上下文探针由 `length`/空正文恢复为 `stop`/非空正文。现已通过 `agent-deepseek-v4-flash-shadow.yml` 固定 JSON-text/disabled 兼容组合，并以同 revision Remote GPU 候选重跑 Task、Timeline 与 Artifact 验收。active/write/MCP 默认关闭边界不变；任务成功率和受控观察窗口仍待补。

- 2026-08-31：真实 Shadow read 联调发现 standalone Core 将无本地 repository 的 Message application 传给 Agent Capability，`conversation.read` 会触发 nil repository panic。现已在 gRPC Message transport 下切换为惰性、可关闭的 Core-to-Message history reader；reader 保持 `dipole-core` RPC 身份，运行时缺失 Message 只返回调用错误，不再使 Core 进程退出。Core/Message 联合恢复演练仍待完成。

- 2026-08-31：Agent Capability RPC 在 `UNAVAILABLE` 后替换底层 gRPC transport，失败请求不在客户端层重放，由 Kafka/EventLedger 按原有幂等语义重新领取和尝试，避免 Core 容器重建后长期持有过期 DNS 地址。真实 Core 重建、retry 重新领取与 dead-letter 恢复的完整演练仍待完成。

- 2026-08-31：Agent Compose 启动顺序现显式等待 Core health，避免首次启动时能力 RPC 在 Core listener 就绪前失败。运行中故障恢复继续由 transport reconnect 和 EventLedger 重试共同覆盖。

- 2026-08-31：修复微服务 Compose 中 Core assistant identity seed 与 TS Agent UUID 的漂移。Core 以 `ai.enabled=true`、`ai.runtime_mode=remote` 维护唯一系统用户，TS Runtime 仍只消费 Shadow 事件；该组合不恢复 embedded Eino consumer。真实私聊到 Shadow plan 的远程证据继续待补。

- 2026-08-31：修复 standalone Core 将完整 Agent Capability adapter 误绑定到 Search 开关的装配缺口。内部 RPC 与 mTLS 完整时，基础只读/任务能力始终注册；Search client 仍仅在 `internal_rpc.agent_conversation_search_enabled=true` 时建立，关闭时只拒绝 `conversation.search`。Remote GPU 的 DeepSeek shadow 验证和公网体验入口仍需完成。

- 2026-08-31：OAuth callback Runtime 配置契约默认关闭，启用时要求独立 secret、固定 lease owner 与非空 key mapping；`index.ts` 未读取它，因此没有新增网络 surface。后续完整装配必须复用此唯一契约并增加私钥/processor/Compose gate。

- 2026-08-31：control handler 到 executor 的内存集成测试已锁定 Gateway 认证、最小 handoff body、进程内去重和固定 Runtime lease owner；它不提供 provider、Store 或默认 bootstrap 证据，外部 callback 继续关闭。

- 2026-08-31：Runtime 增加未装配 control service adapter，将 Gateway 已认证的 handoff ID/correlation 映射到固定 Runtime lease owner 的 executor 请求；optional field 不显式传递 `undefined`，维持 exact optional type 约束。adapter 不接收用户主体或授权码，`index.ts` 和默认 listener 均未装配。

- 2026-08-31：Runtime 增加默认未装配的 OAuth callback handoff executor 组合 seam，覆盖 claim、key open、AAD binding、processor 和 complete/release。只有解封前失败或显式 retryable 结果会释放 lease；未知 processor 结果保持 lease，防止重复外部副作用。它不含 provider 实现，`index.ts`、配置和 HTTP route 均未接线。

- 2026-08-31：Core claim 响应新增仅 Runtime mTLS 可见的 `owner_user_id`，补足 envelope AAD 的 owner binding；Runtime client 在解封前强制校验该字段。该兼容字段不进入 Gateway control HTTP、浏览器、Kafka、Temporal、日志或审计。executor、key open、code exchange、token lifecycle、browser binding 与 callback route 仍未装配。

- 2026-08-31：Runtime 增加未装配的 OAuth callback handoff terminal client。它经 `dipole-agent` mTLS 通道只提交 handoff ID、lease owner 与 correlation，并严格校验无敏感字段的 ID 回包；错误、冲突回包与无效输入统一失败关闭。`index.ts`、control handler 与默认配置均未注入该 client，key open、code exchange、token lifecycle、browser binding 与 callback route 保持关闭。另为四个既有生成 gRPC 回调补充协议元素类型，恢复锁定 TypeScript 的 no-implicit-any gate。

- 2026-08-31：Core 增加默认未装配的 OAuth callback handoff `Complete`/`Release` RPC。两者仅信任 `dipole-agent` mTLS caller，并将 handoff ID 与 lease owner 交给 SQLC Store 的条件终态转换；任何 principal、授权码、密文、key 或 token 均不进入请求或响应，缺 Store 固定 `Unavailable`。TypeScript proto 生成器优先复用已安装的锁定 `protoc`，隔离 Remote GPU 无需再次下载编译器。Runtime 终态 client、key open、code exchange、token 生命周期、browser binding 与 callback route 继续关闭。

- 2026-08-31：Runtime 增加未装配的 OAuth callback handoff control handler。它独立于用户任务控制认证，只接受 Gateway service secret、严格单字段 `{handoff_id}` 与可选 correlation，principal header 直接拒绝；成功固定 `202`。有界进程内去重在一个 Runtime 生命周期内避免重复下游派发，派发失败释放 claim；重启后的 recovery 继续依赖 Core conditional lease。`index.ts`、环境变量和默认 listener 未装配该 handler，Store writer、browser binding、complete/release、code exchange 与 token 生命周期保持关闭。

- 2026-08-31：Gateway 增加未装配的 OAuth callback handoff notifier。控制 HTTP 仅使用 `POST /internal/v1/agent/oauth/callback-handoffs` 与 `{handoff_id}`；它固定 Gateway service identity、保留 correlation、拒绝携带 principal，远程 target 强制 HTTPS，非 `202` 固定失败关闭。Runtime control handler、Gateway bootstrap、callback Store writer、browser binding、complete/release、code exchange 与 token 生命周期继续未接线。

- 2026-08-31：Runtime 增加未装配的 OAuth callback handoff claim client。它沿用 Runtime-to-Core mTLS caller metadata，输入只含 handoff ID 和 Runtime lease owner；所有响应 binding、密文 envelope、key ID 与两个 expiry 都在解封前复核。该 client 未进入 Runtime config 或 process composition，Gateway notifier、complete/release、code exchange、token lifecycle 与故障演练仍关闭。

- 2026-08-31：OAuth callback handoff 已增加默认未装配的 Core claim RPC。它仅接受 `dipole-agent` 调用方，Core 固定 30 秒租约并从 SQLC handoff Store 恢复 owner/binding；响应只在 mTLS 链中返回 Runtime-only 密文及其校验 metadata。Gateway、浏览器和 RequestContext principal 无法参与 claim，缺 Store 固定 `Unavailable`。Runtime client、Gateway handoff-ID notifier、complete/release RPC、code exchange、token 生命周期与重启/过期租约演练仍未接线。

- 2026-08-31：OAuth callback handoff 的 transport release gate 已拆分 Gateway control HTTP 与 Runtime-to-Core mTLS 两条信任链。前者仅承载 handoff ID 与 correlation，后者由 Core 恢复 owner/binding 并执行条件 lease transition；敏感授权材料禁止进入两条链以外的事件、日志和持久元数据。代码接线、provider retry owner review 与故障演练仍未完成。

- 2026-08-31：Runtime private-key source 已以未装配 Node 文件适配器落地：必须明确映射 key ID、绝对路径、owner 和私有权限，私钥仅在单个 callback 内以 Buffer 传递并在 finally 清零，RSA PKCS#8 modulus 小于 2048 位直接拒绝。它尚未读入 Runtime config、Compose、Gateway 或任何 OAuth HTTP/RPC 路径，默认 surface 不变。

- 2026-08-31：OAuth callback handoff envelope v1 已提供 Gateway public-key seal 与 Runtime private-key open 原语。每个 handoff 使用独立 AES-256-GCM data key，Runtime public key 只用于 RSA-OAEP-SHA256 包装；AAD 同时绑定 handoff、transaction、owner、issuer、redirect、code digest、key ID 和 RFC3339 毫秒 expiry。代码仍未读取/装配 Runtime key、未写 handoff Store、未注册 RPC/HTTP 或换码，默认 OAuth surface 不变。

- 2026-08-31：Agent OAuth callback handoff 已增加 Agent-owned SQLC durable record：`000053` 只保存 Runtime key ID、code SHA-256、Runtime-only ciphertext 和受信 binding；状态仅允许 `callback_recorded`、`exchange_claimed`、`exchanged`。条件领取允许恢复已过期租约，领取租约受 handoff expiry 限制；完成和释放都要求当前 lease owner 与未过期租约。该层未装配 Gateway callback、envelope encryption、Runtime claim/exchange 或 token 生命周期，默认没有生产写入，发布门禁继续生效。

- 2026-08-31：OAuth callback handoff 已增加发布前设计门禁。当前单次 consume 无法在 Runtime 不可达后可靠重放，也尚未定义从 callback opaque state 恢复 transaction/owner 的受信 correlation；因此禁止注册 callback HTTP route。后续必须先交付 browser binding、issuer/redirect 校验、Runtime-key 加密的 durable handoff、lease/terminal state 和故障演练，详见 `docs/agent/agent-oauth-callback-handoff.md`。

- 2026-08-31：Gateway 已有一个未装配的 OAuth transaction consume client，可经 Core mTLS 以认证 owner、transaction ID 和 state digest 原子消费，并仅在内部返回密封 verifier。该 client 没有 Gateway Dependency、bootstrap 装配或 HTTP 路由，且其结果不得进入日志、审计或浏览器响应。后续仍需定义 Gateway 到 Runtime 的短时受认证 handoff，才可实现外部 callback。

- 2026-08-31：Core standalone bootstrap 已增加 OAuth transaction consume Store 的显式 mTLS gate。默认配置不注入 SQLC Store，受限 adapter 可与 Memory receipt commit 共同存在；所以 RPC 合同默认仍返回 `Unavailable`。HTTP callback、Runtime 密钥注入/轮换、解封、code exchange、token 生命周期与共享环境演练仍未接线。

- 2026-08-31：Core Agent Capability 已增加默认关闭的 OAuth transaction consume RPC。它限制 `dipole-gateway` 身份、由认证 context 恢复 owner、只接收 state digest，并在获取密封 verifier 前完成 SQLC 条件消费；缺 Store 返回 `Unavailable`。当前没有 HTTP callback、Core bootstrap 注入、Runtime 解封/换码、密钥轮换或 token 生命周期，默认部署继续零写入。

- 2026-08-31：OAuth transaction 的 Agent-owned SQLC storage foundation 已落地：`000052` 只保存密封 verifier 和 state digest；conditional consume 同时匹配 transaction、owner、state、expiry 与 `consumed_at IS NULL`，因此重复 callback 无法获得第二次消费。Core owner-recovery RPC、callback、密钥注入/轮换与 code exchange 尚未接线，默认部署没有该表写入路径。

- 2026-08-31：OAuth 授权事务已定义持久记录安全契约：仅 state SHA-256 与 AES-256-GCM 密封 verifier 可落库，AAD 固定 transaction/owner/issuer/callback/state digest/expiry。SQLC Store 必须以 owner、state digest、expiry 和未消费状态进行原子消费；当前没有内存 fallback、callback 或 token exchange。受保护 Store、密钥轮换、RFC 9728、客户端注册、refresh/revoke 与共享环境演练继续待办。

- 2026-08-31：OAuth authorization-server metadata discovery 已具备注入式、默认关闭的 RFC 8414 HTTP client：精确 issuer-derived URL、`redirect: manual`、HTTPS、JSON、10 秒和 64 KiB 响应限制，错误统一 fail closed。该函数未接入 Runtime composition；RFC 9728 resource metadata、owner-scoped state/verifier、callback、code exchange、客户端注册、refresh/revoke 与共享环境演练仍未完成。

- 2026-08-31：交互式 Agent Task 创建已从认证 IM 主界面获得默认关闭的导航入口。入口要求创建与 Timeline 双 feature flag，且只导航到既有 guarded route；principal、tenant、Agent、Capability、Memory 与 Runtime 仍由后端恢复或固定。共享环境切流与 active authority 继续由 AD-009 管理。

- 2026-08-31：Agent Runtime 增加显式 `test:temporal:integration` 门禁，Remote GPU 已在内存 Temporal Test Server 上通过 Agent Task 与 reviewed Memory promotion 的 `9` 项 durable workflow 用例。它覆盖任务恢复、审批、输入、超时、取消、步数预算、后效重放和 receipt 重试；该证据不连接 Compose、Kafka、Core、MySQL 或 production authority，AD-009、AD-061 的共享环境要求不变。

- 2026-08-31：Agent Task Timeline 的组件测试统一提供 `RouterLink` 桩，消除读取、失败与 Artifact metadata 链接状态中的未解析组件警告。该测试卫生改动不扩张 Artifact 内容访问、路由或 Agent authority；跨浏览器和共享环境证据继续由 AD-044、AD-009 跟踪。

- 2026-08-31：交互式 Agent Task 创建现有默认关闭的认证前端入口。页面仅提交本地幂等键与目标文本，并在严格验证 `accepted` 回包后跳转既有只读 Timeline；principal、tenant、Agent、Tool、Memory 和 Runtime 控制不进入浏览器输入。Remote GPU Node 22 已通过定向 `15` 项 Vitest、typecheck 与 production build。Pencil canonical 创建 desktop/mobile/五态画板、三项复用组件和 2x 导出已完成，Chromium 认证 fixture 已固定初始表单的截图回归；Firefox/WebKit、交互状态和共享环境切流继续由 AD-044 跟踪，active authority 仍由 AD-009 管理。

- 2026-09-01：新增显式 `agent-interactive-shadow` Compose overlay，Gateway 与 Runtime 同时开启 Task control，同时固定 `shadow + read_shadow`、Memory/检索/MCP/外部 MCP 关闭。该组合仅用于隔离体验候选，未替换基础 Compose 或共享环境默认路径；Compose gate 会拒绝其向 active 或写 authority 漂移。

- 2026-08-31：Agent Task 控制面已增加默认关闭的交互式创建 seam。公开 Gateway 路由从 JWT 派生 principal，Runtime 私有路由仅接受 `dipole-gateway` 服务身份；tenant/Agent 身份由 Runtime 配置固定，`client_request_id` 生成确定性 Task/Event ID 并交给已有 Temporal dispatcher。Gateway Go 回归与 Remote GPU Node 22 的定向 Vitest `10` 项、typecheck/build 通过。该切片未启用 Compose、Kafka、Temporal 或 active authority，完整用户入口、共享环境联调与回滚证据仍由 AD-009 管理。

- 2026-08-31：Remote GPU 在 `8e99bde7` 以 Node `22.12.0` 复核完整 Agent Runtime 开发期门禁：`134 passed / 9 skipped` 测试文件、`703 passed / 30 skipped` 测试、typecheck 与 production build 均通过，验证后候选工作树干净。该命令未启动 Compose、Kafka、Temporal 或 production Agent authority，不能替代 AD-009、AD-061 的共享环境证据。

- 2026-08-31：Remote GPU 在 `6f15f887` 以 Node 22.12.0 复核完整 Agent Runtime 与前端门禁。Agent Runtime 为 `134 passed / 9 skipped` 文件、`702 passed / 30 skipped` 测试，前端为 `41 passed` 文件、`165 passed` 测试；两侧 typecheck/build 均通过，生成 Web 产物自动恢复，远端候选工作树干净。该验证不启动 Compose、Kafka、Temporal 或生产 Agent authority，不能替代 AD-009、AD-061 的共享环境证据。

- 2026-08-31：Remote GPU 候选同步新增目标 blob SHA-256 守卫。远端有已跟踪修改时直接拒绝；只清理与待 checkout 提交逐字节一致的未跟踪路径，内容不同的文件与其他 checkout 冲突保持 fail-closed。该保护只处理可再生测试产物，不替代候选目录隔离、活动会话保护或发布审批。

- 2026-08-31：Core 已具备默认关闭的 Agent Search assembly：内部 RPC 与 mTLS 完整时，基础持久 Agent adapter 已注册；仅在 `internal_rpc.agent_conversation_search_enabled=true` 时，Core 才以 `dipole-core` 身份建立 Search client 并提供 Search Capability。关闭时不拨号 Search，`conversation.search` 返回 `Unavailable`。Search RPC allowlist 扩展为 Gateway/Core，默认 Compose 与静态门禁固定该开关为 `false`。共享 Shadow、召回质量与生产切流仍未完成。

- 2026-08-31：基础 Compose、active read 与 External MCP Shadow overlay 现显式固定 `DIPOLE_AGENT_RETRIEVAL_ENABLED=false` 和 `DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED=false`，静态 Compose 门禁同时复核渲染值。宿主环境无法借由未声明变量扩张默认 Capability surface；受控 Search assembly、共享 Shadow 观察和生产切流仍未完成。

- 2026-08-31：`conversation.search` 已完成 Core/Proto/TS 契约，严格从 Task/Run 恢复 principal，独立检查 permission 与 wildcard read scope，并将 query、结果和正文限制为有界 `untrusted` evidence。Runtime 现以默认关闭的 `DIPOLE_AGENT_RETRIEVAL_ENABLED` 将该 Capability 装配到 AI SDK Shadow/Temporal read 路径；关闭时 Registry、模型 allowlist 与执行 Context 均不包含 search。生产 Elasticsearch、跨会话灰度与完整 Context Compiler retrieval orchestration 继续关闭。

- 2026-08-31：Planner 新增默认关闭的 retrieval-to-Context 编排。独立开关只接受当前消息正文的有界查询，经 Core Capability 返回最多 8 条命中并以 query hash、message ID、会话与 sequence provenance 作为 `untrusted` evidence；解析/检索异常会在模型调用前 fail closed。真实 Search assembly、共享 shadow 观察、跨会话/向量检索和生产切流仍未完成。

- 2026-08-31：Remote GPU 全量 Go 门禁发现 legacy Eino 测试桩未随 `AgentCapabilityV1.SearchConversations` 扩展，现已补齐受控返回/调用记录并增加编译期接口断言。该修复只恢复测试契约，Runtime 默认检索开关与生产搜索路径不变。

- 2026-08-30：固定 Agent 检索 Context 的 fail-closed 边界。`dipole-agent` 不获得 Search 直连身份；后续由 Core Agent Capability 从 Task/Run 恢复 principal、permission 和 scope 后调用检索，结果只能进入有界 `untrusted` evidence。Core/Proto/TS 契约尚待实现，生产 Elasticsearch、默认启用和跨会话检索继续关闭。

- 2026-08-30：平台计划已按当前实现证据重写汇总状态：Context Compiler 已具备 v1/v2 预算、可信度、会话 evidence、Memory、Capability descriptor 和 route tokenizer，完整检索编排与生产灰度继续开放；F2 只剩 Settings，F3 只剩 MCP 多轮/敏感授权/产品编排及未覆盖视觉回归。该项不改变默认关闭的 Runtime 或共享环境证据边界。

- 2026-08-30：Remote GPU 的默认 `dipole-dev/<user>` 仅作为提交绑定、按用户隔离的候选引用。`remote-dev.sh` 以精确远端 lease 更新它，使 squash 合并后的 revision 可复用同一引用，并在并发更新时拒绝覆盖；显式共享分支继续只接受快进。该修复不启动 Compose、GPU 或测试服务。

- 2026-08-31：Remote GPU 候选同步的远端 tracking ref 现仅对 `dipole-dev/<user>` 使用显式强制 refspec，消除 squash 后正常移动造成的非快进警告；共享引用维持普通 fetch，fetch 失败不再被忽略。提交 SHA detached checkout、活动用户保护和 Compose 启动边界保持不变。

- 2026-08-30：F2 Device Security 已完成 Pencil desktop/mobile/七态矩阵、三项复用组件、认证 `/devices` 路由和低敏会话 parser。公共 `DeviceSessionResponse` 现只保留登出所需 connection ID、粗粒度 device/device ID 与时间，IP、节点和原始 User-Agent 不再跨 HTTP 边界；新增 `logout-others` 通过当前稳定 Device ID 排除自身，避免 UI 将全设备退出误写为“其他设备”。Go 定向测试、Pencil/文档门禁以及 Remote GPU Node 22 前端 `40/162`、typecheck/build 已通过；远端 Playwright browser binary 下载未完成，跨浏览器执行、Chromium 像素回归和真实 Redis Presence/跨节点踢出继续待验证。
- 2026-08-31：F2 Settings 已新增认证 `/settings` 路由与 Chat 入口，签名更新只调用当前 owner 的 profile API，同步状态只读，设备详情跳转既有低敏 `/devices` 页面，退出复用现有会话终止器。canonical Pencil desktop/mobile/四态画板、批准导出与 Chromium 视觉基线已完成；Remote GPU Firefox 已通过认证路由和披露断言。WebKit 二进制已安装但共享宿主缺少 `libgstreamer-plugins-bad1.0-0` 与 `libavif16`，系统依赖安装和 WebKit 验证继续按 AD-044 跟踪。

- 2026-08-30：校正 External MCP Shadow 文档口径：`external_mcp_shadow` 已作为独占、默认关闭的 Runtime mode 接入 `index.ts`，完整配置才会启动受控 Kafka/Temporal/Capability RPC process；基础 Compose 仍不选择该 mode。隔离全栈证据不覆盖共享 Shadow tenant、真实外部 DNS/TLS、凭据、Provider owner 或生产 authority，相关债务继续开放。

- 2026-08-30：修复 Remote GPU Node 验证的 Vite 清理流程：`remote-dev.sh` 仅恢复受控 `internal/services/core/server/webapp` 固定 `HEAD`，再删除未跟踪构建产物，避免反向 patch 误删已有 hashed assets。`3f1f3936` 复跑后远端候选工作树干净；该修复不启动 Compose 或 GPU 任务。

- 2026-08-30：F2 File Directory 已建立从 SQLC owner-scoped 查询、Core gRPC 到认证 `/files` 的低敏读取边界；前端严格拒绝额外存储字段，读取失败清空旧数据，下载逐项重新授权。Remote GPU Node 22 在 `a29d9927` 通过 38 个测试文件、157 项测试、typecheck 与 production build。上传仍在会话编辑器，文件删除、设备与设置页面、跨浏览器视觉回归和预签名直传默认切流继续按独立切片推进。

- 2026-08-31：F2 File Directory 已新增 Chromium canonical screenshot，受控 fixture 固定 owner-scoped metadata、一次性下载入口和对象键/存储 URL/校验值/上传会话不披露的边界。该视觉基线不访问对象存储，不覆盖上传、删除、下载真实跳转或 Firefox/WebKit，相关能力继续保持独立验证要求。

- 2026-08-30：Remote GPU 已为 `253cf3d29ec79a0f58bcc06c58f5fbad20974b45` 生成 Web Sync Shadow candidate bundle，SHA-256 为 `f4a3a90c5ed5d7d04575a9b939ca738b7e6bd92f53fe3ef818a7249941725f9d`；远端本次未启动 Compose、Prometheus 或客户端流量。当前存在其他用户的活动 tmux 会话，按共享环境保护策略不启动 A6 观察窗口；bundle 仅可作为后续受批准 Observation Session 的输入，不能推导 `promotion_ready`。

- 2026-08-30：Go/Eino legacy 兼容链已由服务布局门禁收口：production code 只允许 embedded Kafka composition 引入 legacy，Eino module import 只允许保留在 `internal/services/agent/legacy`。该约束防止独立 Gateway、Core 或 TypeScript Runtime 绕过 Capability/Temporal/promotion 边界回接旧 Agent；真实 active Runtime 授权与共享环境证据仍按 `AD-009`、`AD-037` 跟踪。

- 2026-08-30：F2 Contact 已完成受认证只读目录 `/contacts`，`/api/v1/contacts` 投影经严格 shape 校验，权威读取失败时清空旧条目并提供重试；Remote GPU Node 22 前端 `34` 个测试文件、`147` 项测试、typecheck 和 production build 通过。备注、拉黑、删除、申请处理与跨浏览器视觉回归仍须作为独立权限和交互切片验证。

- 2026-08-30：F2 Contact 已建立可定位的 Pencil desktop/mobile、loading/empty/request-pending/safety-blocked 状态矩阵与批准导出，避免后续 Vue 页面脱离既有认证 Contact API 语义；写操作与跨浏览器视觉回归仍待独立实现与验证。

- 2026-08-30：IM 深度面试问答已对齐当前两条 Timeline：`messages` 为会话 Message Store，`user_sync_inbox` 与设备 Cursor 为用户 Sync Store。文档同时移除“Redis 将作为权威同步库”的遗留建议，保留 Cassandra 主读、A6 真实观察与旧 Offline 兼容窗口的未完成边界，避免对外材料夸大或低估当前能力。

- 2026-08-30：Remote GPU 在 `37d02383` 上使用 `amtool check-config` 验证 Alertmanager 配置，确认 route 与唯一 discard receiver 可解析；验证只运行一次性容器，未启动常驻 Compose 服务、未发送外部通知。生产 receiver 投递与故障演练继续开放。

- 2026-08-30：A7 将 Prometheus 与 loopback-only Alertmanager 接入开发 observability profile；仓库 receiver 固定为 discard，只验证规则投递与配置语义。生产通知目标、凭据、升级策略和真实告警演练仍待受控环境配置，预签名上传默认路径保持 relay。

- 2026-08-30：A6 增加 Remote GPU 隔离 observability smoke：独立 Compose project 使用 loopback Gateway/Prometheus 端口，检查 Core、Message、Sync、Gateway 的 metrics target。该动作默认清理环境，且不注入 incoming-direct 流量、不创建观察 Session，真实 24 小时窗口与晋级仍保持开放。

- 2026-08-30：Remote GPU 已完成 Web Sync Shadow 候选 `b0e4f252` 的不可变 bundle 构建与 SHA-256 固定，候选可作为观察会话输入；Prometheus 抓取、Sync Projector lag/告警清零与 incoming-direct 真实流量尚未部署，因此 24 小时窗口未开始，不能推导 `promotion_ready`。

- 2026-08-30：Web Sync bundle 打包默认来源已对齐 Vite 的真实生产输出目录，消除 Shadow 候选构建完成后仍因读取已废弃 `frontend/dist` 而无法归档的发布链路漂移；真实 Prometheus 观察窗口仍待开始。

- 2026-08-30：前端 Agent 路由安全契约已补齐新增 Artifact 路由的认证与 feature flag 断言，避免后续路由扩展使静态安全覆盖与实际页面集合漂移；Artifact 继续仅提供默认关闭、owner-scoped metadata 读取。

- 2026-08-30：Remote GPU 隔离 External MCP Shadow drill 已通过 MySQL EventLedger、Kafka consumer、Temporal、Go Core mTLS fixture 与本地 MCP 的完整链路，并验证重启重复事件不产生第二次 Tool 调用、过期 readiness 拒绝且 `production_authority=false`。共享 Core/Kafka/Temporal、真实外部 DNS/TLS、凭据轮换或吊销、Provider owner 与 Shadow tenant 观测仍未覆盖，外部 MCP 和 active authority 继续默认关闭。

- 2026-08-30：README 品牌资产已恢复为深青/橙色 Signal 视觉，并为 IM、Agent 材料限定各自的图形用途；品牌 SVG 为静态文档资产，不将 Agent 标记解释为 active authority 已启用，避免对外材料误导运行状态。

- 2026-08-30：Agent active overlay 的 Kafka group 启动边界现由 Runtime config、Kafka consumer 与 Compose 静态门禁共同限制：active 仅接受 `dipole-agent-active-*`，shadow 仅接受 `dipole-agent-shadow-*`。单元测试已覆盖 active 消息至 Temporal dispatcher 的确定性 Task binding；Remote GPU 隔离 Node 22 worktree 的独立全量 Runtime 测试、typecheck 与 build 均通过。真实 Kafka broker、Core mTLS、Temporal Worker、grant 和 Memory 提交仍未纳入同一演练，`AD-009` 继续开放。

- 2026-08-30：Remote GPU 隔离 Temporal/Core/MySQL mTLS fixture 已补齐 owner-scoped Memory rollback：首个 receipt durable retry 后得到同一条 Memory，grant 撤销后预 admission receipt 被拒绝且候选零写入，最后 owner application control 持久撤销该 Memory。Kafka trigger、Gateway owner revoke 网络传输、共享 authority 与 overlay rollback 未纳入此演练，`AD-009` 继续开放。

- 2026-08-30：学习与面试材料改为双项目叙事：`Dipole IM` 单独维护消息、同步、存储、微服务和文件数据面；`Dipole Agent` 单独维护可信上下文、Capability、Temporal、Memory、MCP 与 active 边界。总入口、README、文档目录、架构清单和自动门禁同时校验两份材料，降低将默认关闭 Agent 能力或 IM 存储实现写入错误项目口径的风险。

- 2026-08-30：Remote GPU 隔离联合演练已将 admission 后 grant 撤销纳入同一 Temporal/Core/MySQL mTLS fixture：同一 active grant 先 admission 两个 Task/Run，首个 receipt 在故障重试后收敛，撤销 grant 后第二个预 admission Run 的 receipt 被 Core 拒绝，MySQL 断言候选未生成 Memory。该证据不包含 Kafka、共享环境 authority、overlay rollback 或业务级 Memory rollback，`AD-009` 继续开放。

- 2026-08-30：Remote GPU 在隔离 worktree 中通过 Temporal/Core/MySQL mTLS 联合演练：临时 MySQL 8.4、实际 `MemoryPromotionReceiptServer`、loopback TCP+mTLS 和内存 Temporal test server 串联；首个 Core 持久提交后，受控 Worker 故障触发 Activity 重试，第二次请求返回同一条 Memory。演练同时修复了重复候选晋级将首次 `ValidFrom` 与重试墙钟时间比较而错误冲突的问题。Kafka、admission 后 grant 撤销和 overlay rollback 尚未纳入同一运行，`AD-009` 继续开放。

- 2026-08-30：Remote GPU 在 `ac7b8790` 一次性 worktree 上通过 receipt MySQL mTLS contract。临时 CA 下的 loopback Core listener 使用项目 `platformrpc.Dial`、Core Agent 方法白名单和 `dipole-agent` 客户端证书；真实 adapter 继续验证首个提交、同 receipt 重放和 admission 后 grant 撤销拒绝。Temporal Worker 与该 TCP/MySQL 链仍未同组运行，`AD-009` 继续开放。

- 2026-08-30：Remote GPU 在 `c88798c3` 一次性 worktree 上通过 MySQL 8.4 receipt contract。真实 `MemoryPromotionReceiptServer` 经 `dipole-agent` 身份拦截器调用持久 Commit Service，完整 migration 覆盖首个提交、同 receipt 重放和 admission 后 grant 撤销拒绝；该测试未建立 mTLS 网络连接或运行 Temporal/Kafka，因此 `AD-009` 的联合证据仍未关闭。

- 2026-08-30：Remote GPU 在 `76eb89c3` 一次性 worktree 上使用内存 Temporal test server 通过 Agent Memory promotion workflow integration：prepared receipt 持久返回，第一次受控 commit 失败后重试并保持同一 receipt binding。该环境未启动 Core、MySQL、Kafka 或 active Compose，因此 `AD-009` 的跨进程 grant 撤销与回滚联合证据继续开放。

- 2026-08-30：Web Multipart 重试已按故障类别收敛：浏览器网络异常及预签名 `408`、`429`、`5xx` 保留指数退避，确定的预签名 `4xx` 不再重复 PUT。专项 28 项 Vitest、typecheck 与生产构建通过；该结果限于客户端调度，真实代理断网和跨网络故障矩阵继续由 `AD-055` 跟踪。

- 2026-08-30：Remote GPU 使用显式 `DIPOLE_REMOTE_GO_ROOT` 复核 MinIO Multipart lifecycle 与 restart smoke，确认分片状态可跨服务重启恢复并完成内容校验；客户端断网、签名服务故障、跨标签页互斥与预签名默认切流仍待完成。

- 2026-08-30：Remote GPU 全量 Go 门禁发现 active receipt authorizer 接线与旧 default-off 结构断言冲突，已改为检查显式开关与 mTLS 条件；该修复不提升默认 active 权限，联合环境证据仍待完成。

- 2026-08-30：embedded runtime 的 active Run admission 已复用持久 promotion authorizer，消除 Core receipt resolver 与 Task/Run admission 授权边界不一致的问题；联合环境证据仍由 `AD-009` 跟踪。

- 2026-08-30：修复 Core/embedded receipt composition 未注入 active promotion authorizer 的接线缺口；真实 active receipt 因此可复用已验证的 grant 复核，缺 grant 仍拒绝。跨进程 Core/Temporal/MySQL 联合演练继续由 `AD-009` 跟踪。

- 2026-08-30：隔离 Remote GPU MySQL 8.4 contract 已验证 receipt commit 的持久 Task/Run、grant、candidate/review 和幂等晋级事务，并发现后修复 promotion Memory lineage 未 canonicalize 导致的数据库约束回滚。该证据不覆盖 Core mTLS、Temporal、Kafka 或共享环境 rollback，`AD-009` 继续开放。
- 2026-08-30：学习、简历与面试主文档补充合并前复核清单，门禁同时检查现场介绍、工程故事与独立问答入口；该约束降低叙事遗漏风险，但无法替代每项技术结论的实现、测试、基准和运行记录复核。
- 2026-08-30：新增 Agent Memory promotion 跨语言 mTLS RPC drill：Go loopback fixture 认证 `dipole-agent`、拒绝错误 secret/证书，TypeScript generated client 提交 prepared receipt 并复核低敏 response binding；脚本支持显式用户态 `DIPOLE_GO_BIN`，避免远端系统 Go 自动下载。该 fixture 不连接 Core 持久事务、Temporal、Kafka 或 grant，真实提交/撤销/回滚仍由 `AD-009` 跟踪。
- 2026-08-30：Remote GPU 在隔离 Node 22 worktree 上通过 Agent Memory promotion profile、隔离 Temporal receipt retry 与 TypeScript typecheck；运行前后未启动 Dipole Compose 或 GPU workload。该开发期证据不覆盖跨进程 Core、Kafka、真实 grant、撤销或 rollback，`AD-009` 继续开放。
- 2026-08-30：Agent Memory promotion Compose overlay 已加入正向渲染和缺失 operator authority 的负向门禁，明确 Core receipt commit 与 Agent `promotion_active` 需同时配置；该门禁只覆盖静态配置，真实 Core/Temporal/grant/replay/rollback 联合证据仍是 `AD-009` 的开放条件。
- 2026-08-30：学习、简历与面试主文档已加入架构文档门禁，入口、核心章节与能力卡片模板字段可自动检查；技术事实仍需在每个合并切片以实现、测试、基准或运行证据人工复核，避免将模板通过误作能力验收。
- 2026-08-30：架构文档门禁新增索引相对链接校验，覆盖项目、Agent 和契约入口；该检查只验证本地导航存在性，不替代 Schema、运行时和共享环境证据。
- 2026-08-30：新增 `contracts/README.md` 顶层契约索引并从 README/Agent 文档入口链接，降低跨服务协议发现和版本漂移风险；契约索引只改善治理与导航，不改变各 Agent 能力默认关闭及共享环境证据门槛。
- 2026-08-30：Agent 文档已增加独立入口 `docs/agent/README.md`，统一 Runtime 边界、默认开关、验收顺序和真实环境证据边界；该整理降低文档入口漂移风险，但不改变共享环境证据、active authority 或外部 MCP 的未完成状态。
- 2026-08-30：直接 Multipart smoke 脚本现已尊重显式 `DIPOLE_REMOTE_GO_ROOT`，无效路径 fail-closed 并禁止隐式 Go toolchain 下载；Remote GPU 使用用户态 Go `1.27.0` 复跑基础与 restart smoke 均通过。完整浏览器断网、过期会话、网关限流、代理超时及跨存储故障矩阵仍待完成。
- 2026-08-30：在 `master` `a26b3fb3` 上显式使用 `LAB113-OPS` 验证远程同步和 Multipart smoke；SSH 别名与自动 Go 发现均正常，未复现此前的小写别名解析错误，因此保留为观察项，不修改主机配置。
- 2026-08-30：Remote GPU 使用 Go `1.27.0` 完成真实 MinIO cleanup 生命周期 smoke，验证未完成 upload 的 listing、cutoff 选择、Abort 和清理后无残留；测试通过有界等待处理 listing 收敛，完整浏览器断网、过期会话、网关限流、代理超时和跨存储矩阵仍待完成。
- 2026-08-30：cleanup 生命周期测试确认当前 MinIO 对目录前缀 listing 存在实现差异，改用完整对象键做隔离服务端列举并保留业务前缀清理 contract；该适配仅用于测试证据，不放宽生产清理工具的 `message-files/` 范围。
- 2026-08-30：File Service 新增过期 Multipart session fail-closed 回归测试，覆盖 status、presign、register、upload、complete 和 abort 六个入口；storage 调用保持为零，过期 Redis session 不会继续访问 MinIO。

- 2026-08-30：确认 `docker-compose.cluster.yml` 与 `docker-compose.redis-cluster.yml` 继续提供 Kafka/Redis 组件级演练；`docker-compose.microservices.yml` 仍绑定单 broker、单 Redis、单 MySQL，业务 override 已增加 MySQL Router/InnoDB Cluster、Kafka 三节点和 Redis Sentinel 的可渲染组合。真实业务故障切换和自动回切证据继续保持待办。
- 2026-08-30：新增 `docker-compose.business-cluster.yml`，以 override 方式将三 broker 与 Redis Sentinel 接入 Core、Message、Sync、Gateway、Agent 和 Search Indexer；该文件只证明配置可组合，未替代业务层故障注入、消息收敛和回滚 receipt。
- 2026-08-30：新增业务集群隔离生命周期脚本，统一 `config/up/status/down` 和 project 边界；活动用户默认阻断、GPU 任务允许并行，卷保留以支持故障复盘。Kafka broker/Redis master 故障注入及业务收敛证据仍待执行。
- 2026-08-30：微服务 Gateway 宿主端口改为可配置，业务集群入口默认使用 `18080`，降低隔离演练与其他开发栈的端口冲突风险；容器内端口和默认 `8080` 兼容性保持不变。

- 2026-08-30：将 C1 stop/start recovery drill 接入 Remote GPU 统一入口，补齐 `/tmp` 报告挂载和 k6 fallback；真实报告仍要求候选 revision、恢复后 post-load、Kafka lag 和自动清理全部通过，未改变生产默认路径。

- 2026-08-30：C1 recovery 复盘发现 Dockerized k6 只挂载仓库目录时无法写入 `/tmp` 报告目录；已通过显式只读边界之外的 `/tmp` 挂载修复容器输出路径，后续远程演练可复用统一 wrapper。

- 2026-08-30：Remote GPU 在存在 `2` 个外部 GPU 任务期间完成 C1 候选 `dipole-node2` stop/start 恢复证据；恢复时间线、PID 变化、镜像 revision、40/40 post-load 和 Kafka lag 已通过 `recovery-report.v1` 校验，候选拓扑清理后无残留。该证据关闭本轮节点恢复观察项，业务拓扑 broker/Redis 故障、背压和 C++ 性能门禁仍按阶段计划开放。

- 2026-08-30：远程并行启动回归发现批准标志只存在于本地 shell，未传入远端 heredoc，导致即使用户已批准仍被活动会话拒绝；已改为显式 SSH positional argument，并由契约测试覆盖。GPU 任务仍只记录快照，活跃用户仍需明确批准。

- 2026-08-30：同步远程部署操作手册与启动脚本的并行策略：GPU 任务仅触发资源快照，不再阻断 CPU/容器型开发动作；活动登录会话仍保持默认保护。历史记录中的旧门禁结果保留为审计证据，不代表当前有效规则。
- 2026-08-30：修正远程开发脚本与并行资源政策不一致的问题：GPU 进程只进入资源快照和提示，不再阻断 CPU/容器型开发动作；活跃登录用户仍保持默认阻断。该放宽仅适用于开发阶段，确需 GPU 的 Agent/模型任务仍需单独声明设备、显存预算和冲突检查。

- 2026-08-30：Remote GPU C1 benchmark 入口支持宿主 k6 缺失时使用固定 Dockerized k6，并自动注入 `dipole-c1` 候选端口；修复 SSH 空参数造成的镜像参数偏移和 k6 镜像 entrypoint 重复调用。工作流契约已通过，最终候选拓扑的完整 k6 基线仍需在该 revision 上采集。
- 2026-08-30：Eino `v0.10.0-alpha.26` 已完成只读 API 评估；其 Session Timeline、Checkpoint/Resume 和 backgroundtask lease/CAS 可作为 adapter 参考，但与现有 Temporal durable execution 重叠，alpha 依赖继续禁止进入默认构建和 active authority。
- 2026-08-30：跨标签页 Multipart 并发由浏览器 Web Locks 按文件 session 串行化；不支持该 API 的旧浏览器仍可上传，但缺少跨标签页互斥，需要后续兼容策略或最低浏览器版本门禁。
- 2026-08-30：预签名刷新失败路径已由前端回归测试锁定：只完成一次失败 PUT，刷新服务错误向上返回，不改变可恢复 session；真实签名服务故障矩阵仍待共享环境证据。
- 2026-08-30：新增 `multipart-restart-smoke`，用独立 MinIO 数据卷验证服务重启后继续上传、Complete 和对象内容一致性；Remote GPU 已在提交 `3fe5d00f` 上通过真实验证，客户端断网、签名服务不可用和跨标签页并发仍需矩阵覆盖。
- 2026-08-30：预签名过期恢复已收敛到可测试上传原语，`403` 会按分片刷新 URL 后重试，回调失败仍由上层有限退避控制；`12/12` 上传测试与 Frontend typecheck 通过。真实代理断网、签名服务不可用和跨标签页并发仍需环境故障证据。
- 2026-08-30：Multipart 客户端对预签名分片 `401/403` 触发按分片重新签名，并保留原有退避重试；`11/11` 上传测试和 Frontend typecheck/build 通过。真实代理断网、签名服务不可用、服务重启恢复和跨标签页并发仍需故障矩阵证据。
- 2026-08-30：Multipart 前端失败路径已保留 Redis/MinIO session 和本地文件身份，恢复重试可依据服务端已上传 part 跳过重复传输；Frontend `29/114`、typecheck/build 通过。服务端重启恢复、预签名过期后重新签名和完整断网故障矩阵仍需真实环境证据。
- 2026-08-30：远程开发入口新增 `multipart-smoke` 并统一固定远端本地 Go toolchain，避免 Multipart 验证因隐式下载超时；入口契约 `7/7` 通过。该流程仍未覆盖客户端断网重试、服务重启恢复、预签名默认切流和完整故障矩阵。
- 2026-08-30：Remote GPU 在 `master` revision `67235080` 复跑 MinIO Multipart 真实生命周期 smoke，乱序上传、同编号替换、Complete、内容校验和重复 Abort 通过；网络型 Go toolchain 阻断已通过固定远端 Go 1.27 与 `GOTOOLCHAIN=local` 排除。该证据仍未覆盖客户端断网重试、服务重启恢复、预签名默认切流和完整故障矩阵。
- 2026-08-30：A6 在真实 Chromium 中验证 Sync Timeline 的浏览器重开恢复：第一轮提交并 ACK 到 `2`，重开后先幂等 ACK 已提交 cursor，再从 `after_seq=2` 拉取并提交 `3..4`，最终安全 cursor 为 `4` 且消息无重复；该证据仍为隔离客户端验收，真实部署观察窗口、共享环境切流和旧 Offline 正文退役条件继续开放。
- 2026-08-30：C3 性能基准增加显式容器 builder 路径，当前 revision、编译器来源和 runner 会进入报告；宿主机缺少 `clang-tidy` 时可独立取得基准，远端缺少 Docker builder 依赖或当前 C++/Go 性能比未达门槛时继续 fail closed。
- 2026-08-30：记录远端 Go 1.22 PATH 与已安装 Go 1.27 toolchain 的差异，并为 benchmark 增加显式 `DIPOLE_GO_BIN`；网络不可用时仍必须使用已验证本地 toolchain，禁止以自动下载成功推断性能证据有效。
- 2026-08-30：归档 Remote GPU 同版本容器 benchmark：C++/Go ratio `0.119227`，低于 `1.0` 门槛，C3 性能收益门禁继续阻断 C++ primary/gray rollout；GPU 任务前后均为 2 个且未被操作。
- 2026-08-30：完成更接近同契约的 C++/Go `DeliveryEnvelope` workload，ratio `0.247269` 仍低于门槛；C3 继续保持 Go authority，后续应针对 JSON parse、Protobuf construction 和 allocation profile 做 profiling，禁止仅凭该微基准切流。
- 2026-08-30：5 次 Remote GPU 稳定性采样确认 C++/Go ratio 约 `0.25` 且运行抖动有限；下一步 profiling 需要定位稳定的 JSON、时间校验、Protobuf 和 allocation 成本，C++ primary/gray 继续关闭。
- 2026-08-31：Remote GPU 在 `bed7a5d06f5f69bbccc0de4586235881e5b6d5ae` 以 Ubuntu 24.04 builder 重跑 100,000 次同契约 projection workload，C++ CTest `14/14` 通过，C++/Go ratio 为 `0.239956`，低于 `1.0` 门槛并判定 `blocked`；证据归档于 [C3 projection benchmark](../../benchmarks/c3-cpp-projection-benchmark-2026-08-31/)。因此继续保留 Go authority，当前 C++ 不进入 primary 或灰度路径。经当前开发优先级决策，C++ 轨道暂缓，资源转向 Agent Runtime 安全闭环；恢复条件为新的可复现收益证据和 Agent Runtime 里程碑完成。

- 2026-08-31：OAuth callback handoff 已从分散的 claim、key source、executor、terminal 和 control adapter 收口为注入式 Runtime composition factory，并以 fake mTLS transport 验证 handoff-ID-only control 到完成路径。跨实例测试进一步固定：只有前一 Runtime 显式 release 后，替换实例才能从 Core 条件租约重新领取；进程内去重从不承担恢复权威。`index.ts`、Compose、浏览器 callback、Provider code exchange 和 token lifecycle 均未装配；后续需要先完成独立 processor 的受控实现、真实重启/过期租约演练，再评估单独的默认关闭部署 profile。

- 2026-09-01：read-shadow 的单轮 Planner 允许受限两步发现读取：`conversation.read` 必须紧邻 `conversation.list` 并携带唯一 `$discovered.previous` 标记。执行层仅从已完成 List 输出提取首个合法会话键，再进入 Capability scope/permission 检查；模型构造 ID、越过前置步骤或空输出会在 Tool 调用前失败。当前未提供任意索引、多会话 fan-out 或写 Capability，后续选择策略必须以同等的服务端数据绑定与审计实现。

- 2026-09-01：受信发现约束同时在 Planner 与执行层实施。执行层对任何非 `$discovered.previous` 的 `conversation.read` 固定拒绝，并在拒绝路径不调用远程 Capability、不记录 allowed authorization；因此后续新增 Planner、MCP adapter 或测试夹具不能靠直接 ID 意外绕开绑定。多候选选择、用户显式选择和写能力仍需独立的可审计契约。

- 2026-09-01：`000057` 将模型审计运行的唯一约束升级为 `(task_uuid, stage)`。默认 `plan` 继续使用 v1 deterministic run ID，非默认 stage 使用携带 stage 的 v2 ID，防止历史计划重放漂移；Router 对 stage 名称、预算和恢复结果均失败关闭。该层仅提供 `synthesis` 的 durable 运行隔离，读取结果编译、第二次模型调用、Artifact 替换及真实 Shadow receipt 仍待后续切片完成。

- 2026-09-01：read-shadow 在已审计计划和已完成 Tool Step 后调用独立 `synthesis` stage，最终 Artifact 使用该摘要。Tool 输出在 synthesis prompt 中始终标记为 untrusted data，且空输出不会触发第二次调用；Metadata Planner 保持单阶段。当前输出截断为 12 KiB，尚未接入 ContextCompiler 的多来源结果压缩、跨会话选择或写入/approval loop，真实 Remote GPU receipt 仍待安全窗口。

- 2026-08-31：Runtime bootstrap 现显式解析 OAuth callback 配置并拒绝启用状态，避免缺 Provider processor 的环境变量被静默忽略。拒绝发生在任何网络资源初始化前；后续独立 profile 需要将 processor、Core credential、key mount、运行证据和回滚开关作为同一部署契约交付。
- 2026-08-30：长时 C++ profiling 受远端内核缺少匹配 linux-tools 阻断，未安装系统包也未将 `perf` 失败误判为热点结论；下一步可在具备匹配工具链的隔离 runner 中采集，当前仍禁止据 benchmark 切换 C++ authority。

- 2026-08-30：补充开发与远程资源工作流，明确 GPU 任务存在时仍可运行不申请 GPU 的 Dipole 构建、Smoke 和压力测试；要求使用隔离 Compose project、记录前后资源快照，并禁止触碰其他任务。共享环境切流和确需 GPU 的 Agent 任务仍保留独立审批与资源门禁。

- 2026-08-30：新增 `scripts/smoke-minio-multipart.sh` 与真实 MinIO 集成测试，覆盖两分片乱序上传、同编号分片替换、按序完成、对象内容校验和重复 Abort；测试资源使用临时容器并自动清理。该证据仍未覆盖客户端断网重试、服务重启恢复、预签名默认切流和完整故障矩阵。

- 2026-08-30：在 `epic/03-agent-runtime` 的 `services/agent-runtime` 运行标准 `npm test`、`npm run typecheck` 和 `npm run build`；Vitest `125` 个测试文件/`665` 个测试通过，`7` 个文件与 `27` 个测试按条件跳过，类型检查和生产构建通过。该结果只证明独立代码质量基线，真实 Kafka、Temporal、Capability RPC、外部 MCP 和 active authority 联调仍按 AD-009/AD-030 保持开放。
- 2026-08-30：本机开发工具链已切换至 `planning-with-files` 上游稳定版 `v3.11.2`，并移除重复 `.agents` skill 来源；Codex 适配器对并行 PreToolUse/PostToolUse 采用同 session/项目根短窗口去重，三次真实并行 Bash 仅产生一份计划上下文。该改动只约束开发期提示频率，不改变项目运行时或 Agent authority。
- 2026-08-30：复核官方 Eino 上游：稳定版本仍为 `v0.9.17`，最新为 `v0.10.0-alpha.26` 预发布版；v0.10 方向包含 runner-managed Session、可重放中间件状态、后台任务和 Automemory，但官方明确 alpha 期间可能发生破坏性 API/行为变化。暂不升级生产 Go/Eino 回滚链路，后续仅在隔离 spike 分支评估与 TS Runtime/Temporal/Memory 的边界映射。
- 2026-08-30：修正平台演进计划中滞后的 Agent/Frontend 质量数字和 F4 描述，明确已验证的 token、流程与跨浏览器功能范围，同时保留截图级视觉基线、真实 Pencil CLI 增量编辑和共享环境门禁为未完成项。
- 2026-08-30：Agent Task Timeline 的 Chromium visual E2E 使用受控低敏 fixture 固定只读 revision、Capability、等待审批和分页入口，并确认 raw event kind 不直接进入页面。该基线只覆盖 Chromium 当前布局；其余页面、浏览器与完整视觉验收继续由 AD-044 跟踪。
- 2026-08-30：Agent Definition Catalog 已使用真实 Pencil 增量设计、authenticated Gateway 查询、fail-closed 目录清理和 Chromium visual E2E 固定只读 Definition/version/scope 边界。该切片不开放 Runtime、Definition 写控制、active authority 或跨浏览器视觉验收，剩余产品入口和平台覆盖继续由 AD-036、AD-044 跟踪。
- 2026-08-30：Definition Catalog 的认证读取与只读边界已在 Chromium、Firefox、WebKit 三浏览器复核；截图基线仍仅覆盖 Chromium，不能替代其余浏览器的像素级评审、active Runtime 或共享环境证据。
- 2026-08-30：Gateway 已新增默认关闭的 Artifact metadata seam，复用受认证 Core gRPC 重新绑定 principal，并要求精确 64 位 SHA-256 Artifact ID、复核正文大小和 SHA-256 后丢弃正文、对象键与 metadata JSON。Task Timeline 现以同一内容寻址 ID 提供受限可发现性，Pencil/Vue metadata 页面和 Chromium、Firefox、WebKit 认证读取已覆盖读取、失败清理与披露边界；视觉快照仅固定 Chromium，正文、下载和远程运行时门禁仍缺失，Artifact 继续不能描述为完整 Web 文件能力。
- 2026-08-30：主线综合复核通过架构文档、服务布局、SQLC 与 Go 全量 test/vet 门禁；文档目录继续保持根目录入口、`docs/` 分类和历史证据分离，后续共享环境切换仍需独立运行证据。
- 2026-08-30：在 `master` revision `a3a433be` 上通过 Remote GPU Node 验证：Agent Runtime `125/665`、Frontend `29/114`、typecheck 与构建均通过；`node-test` 已增加 `webapp` 脏状态前置拒绝和退出清理，远端 detached worktree 验证后保持干净。该证据覆盖开发期 Node 质量门禁，不替代共享环境 Compose、负载和生产切换证据。
- 2026-08-30：在 `master` revision `b96403b0` 上通过 Remote GPU Go canonical 门禁：白名单 Go test、服务布局和架构文档检查全部通过，未启动 Compose，远端源码工作树保持干净。该证据覆盖开发期代码质量基线，不替代共享环境服务发布、负载和故障演练证据。
- 2026-08-30：新增 Remote development entrypoint contract test，覆盖 clean committed sync、Node `--package-lock=false`、`webapp` dirty guard/exit cleanup、Node 22 门禁和活动主机保护，`4/4` 通过。该测试强化开发期工作流回归，不替代共享环境部署与负载证据。
- 2026-08-30：清理开发路线图中对已退役 `internal/service` 的过时依赖描述，改为引用共享兼容适配器和服务边界目录；结构门禁与历史债务记录继续保留对旧路径的负向检查。
- 2026-08-30：在 `master` revision `801e69ce` 上复跑完整微服务 Compose smoke，逐服务 readiness/metrics、Core proxy、mTLS、远程 WS ownership 和 Agent EventLedger/Task/Run 幂等均通过；该证据限于隔离拓扑，生产 Kafka ownership、候选发布和可执行回滚 receipt 仍待完成。
- 2026-08-30：在 `master` revision `69055e87` 上复跑 Cassandra primary Compose smoke，schema init、Sync primary 配置、依赖 readiness 和 `readyz` 通过；该证据只覆盖隔离启动门禁，真实 Inbox hydration、共享环境观测、责任人批准和可执行回切仍待完成。
- 2026-08-30：复核 Context Compiler fixture 校准命令，五类样本无低估并生成 report SHA-256 `d5bce2090f8d4b4c6af786d75dee656fd9dd33554ecaf8026e5880abe4863562`；该结果只证明合成 tokenizer profile 的确定性和证据完整性，真实候选模型 route 校准仍未完成。
- 2026-08-30：补齐前端类型检查脚本并加入工具链契约测试，`npm run typecheck` 与 `vue-tsc --noEmit` 现在具有统一、可发现的验证入口；不改变前端默认构建和发布边界。
- 2026-08-30：修复 Chat 初始化认证恢复的未处理 Promise rejection；共享设备 HTTP 401 E2E 已覆盖会话清理、登录页跳转和三浏览器无 `pageerror`，避免认证失败污染前端错误信号。
- 2026-08-30：在 `master` revision `d2507377` 上重建候选镜像并复跑 Agent Timeline repair Compose smoke；v50、UTC、专用最小权限、readiness、pending intent 恢复和 event UUID 幂等均通过，证据仍限于隔离环境，共享 operator 灰度、告警抓取、回切演练和默认开关继续待完成。
- 2026-08-30：复核 Go/Eino 回滚基线，项目当前锁定 `github.com/cloudwego/eino v0.9.17`，`go list -m -u` 未发现更高稳定升级；预发布版本不进入生产依赖，Eino 继续仅承担 embedded legacy 回滚职责。
- 2026-08-30：Agent Active Compose 增加缺失 release manifest/candidate 的负向门禁，部署插值不完整时 fail closed；默认 Shadow、显式 `user_gray` manifest 和回滚路径保持不变。
- 2026-08-30：修正 Vite 构建输出从退役的横向 `internal/server/webapp/` 回流到 Core-owned `internal/services/core/server/webapp/`，并同步工具链断言；前端构建不会再产生未跟踪的旧目录产物。
- 2026-08-30：修正 Agent Memory Chromium E2E 的标题查询歧义；Agent Memory 目标场景 `3/3` 和完整 Chromium E2E `28` 项通过，F4 当前仍保留跨浏览器和真实 Pencil 增量编辑待办。
- 2026-08-30：完成 Chromium、Firefox、WebKit 全浏览器 Playwright 回归，`64` 项通过、`26` 项按平台/条件跳过；F4 的真实 Pencil 增量编辑和未覆盖平台场景仍保持待办。
- 2026-08-30：C++ Realtime Delivery 标准容器门禁通过，CMake 配置、构建和 14/14 CTest 在 Ubuntu 24.04 容器内成功；本机 host gate 因缺少 `grpc++` 仅记录为环境缺口，不影响默认 Go authority，也不授权 C++ primary 灰度。
- 2026-08-30：C++ Realtime Delivery CMake 已支持本地源码布局与容器扁平布局的 canonical 根目录探测，并在缺少 delivery proto/fence testdata 时 fail fast；该修复只恢复一致构建路径，C++ primary 仍保持默认关闭。
- 2026-08-30：Agent Runtime 配置解析新增 `shadow|remote` 严格枚举和非法值 fail-closed 回归测试；该修复防止配置拼写错误被静默降级，不改变默认 Shadow 或 active promotion 门禁。
- 2026-08-30：前端新增 Agent 路由安全契约测试，覆盖认证保护、独立 flag fail-closed 回退和 5 条页面路由清单；该测试只锁定现有安全边界，不代表 Agent active authority 或相关生产能力已开启。
- 2026-08-30：前端设计计划已按当前 Router 清单校正：Login/Chat 与 5 条受 flag 保护的 Agent 页面路由已记录，Search/Sync 标注为 Chat 工作区能力；未完成的 Contact、Group、File、Device、Settings 和真实视觉回归继续保留为待办。
- 2026-08-30：Web Sync Observation CLI 新增 Session 起点校验，早于 `started_at` 的 status 采样统一拒绝并由回归测试锁定；该修复只强化证据时间完整性，不改变 24 小时晋级门槛或生产开关。
- 2026-08-30：主线复核前端设计和 Web Sync 观测契约均通过：Pencil 结构门禁 54 个 Frame/2036 个节点/36 个变量/23 个可复用组件，前端 Vitest 102 个测试，观测契约 9 个测试；这些结果不替代真实客户端 24 小时共享环境窗口，A6 仍保持未晋级。
- 2026-08-30：微服务 Compose 已显式声明 `DIPOLE_AGENT_RUNTIME_MODE=shadow`，并增加配置门禁防止默认部署漂移到 `active`；该切片只强化配置可见性和 fail-closed 约束，不改变 Agent authority。
- 2026-08-30：收敛 Agent Runtime 的部署文档语义：Compose 默认启动容器并运行 Kafka Shadow，`active` authority、模型调用和写能力仍默认关闭；补充 active 启用前的 release manifest、Temporal 和 promotion binding 前置条件。该变更只澄清现状，不改变运行时开关。
- 2026-08-30：诊断 Pencil Gemini 增量路径，CLI 在执行前因缺少 selected model API key 退出；safe-edit wrapper 已清理临时输出，canonical `.pen` 未改变。Claude 执行路径仍受 MCP 调用超时影响，AD-044 等待可用凭据和稳定执行窗口。
- 2026-08-30：使用显式 `--prompt`/`--prompt-file` 重试 Agent Task Timeline Pencil 增量编辑；CLI 成功进入 Agent 会话但在 90 秒内未完成，safe-edit wrapper 已清理临时输出且 canonical `.pen` 未改变，AD-044 仍等待稳定增量执行。
- 2026-08-30：补充根级 `/agent-runtime/` 遗留构建产物的显式忽略规则和仓库结构说明；TypeScript Agent 源码继续唯一归属 `services/agent-runtime/`，Go 继续通过显式包白名单避免递归扫描本地构建输出。
- 2026-08-30：兼容服务测试根退休后的规范 Go 门禁复核通过：`scripts/check-go.sh` 的全量 `go test` 与 `go vet` 均成功；直接 `go test ./...` 会把本地忽略的 `agent-runtime` 构建依赖目录纳入扫描，项目继续以脚本包白名单作为可复现门禁。
- 2026-08-30：在最新 `master` 再次执行 `scripts/check-go.sh`，白名单 Go 包的 test/vet 均通过；本次未改变直接 `go test ./...` 对本地旧构建目录的已知扫描边界。
- 2026-08-30：`internal/compat/service` 已无生产依赖，仅剩跨版本 domain-event 契约测试；测试已迁入 `internal/platform/events/contract` 外部测试包，兼容 service 根目录删除，并由服务布局门禁阻止回流。
- 2026-08-30：调用审计确认 `internal/app` 已无生产代码和测试调用者；11 个 Agent application 边界测试已迁入 `internal/services/agent/application` 外部测试包，删除聚合测试壳并由服务布局门禁阻止回流。
- 2026-08-30：Gateway 旧 `NewServer` 兼容包装已完成调用审计并删除；测试和独立 bootstrap 均使用显式 `NewServerWithDependencies`，结构门禁同时阻止隐式构造和 Gateway Server 回流 Core Auth 实现。
- 2026-08-30：Gateway 显式 Composition Root 构造已强制要求 `application.TokenResolver`，缺失 verifier 会在服务启动前 fail fast；旧构造仅保留兼容包装，回归测试和结构门禁通过。
- 2026-08-30：Gateway Server 已增加显式 `application.TokenResolver` 注入入口，独立 Gateway bootstrap 负责组合 Core JWT verifier；旧构造仅保留兼容包装，服务层不再自行决定认证实现。
- 2026-08-30：WebSocket Authenticator 已改用 `internal/application.TokenSessionResolver`，移除 transport 对 Core Auth 具体 TokenSession 类型的编译依赖；Core 继续拥有 JWT verifier，Gateway 后续可通过 Composition Root 注入替代实现。
- 2026-08-30：共享 authentication middleware 已改用 `internal/application` 的 token resolver/session contract，移除对 Core Auth 具体 TokenService 的编译依赖；Core 继续拥有 JWT issuer/verifier，后续可独立替换 Gateway verifier。
- 2026-08-30：Agent MCP resource 默认值和配置解析已归属 `internal/application` contract；Gateway bootstrap、proxy 和 middleware 不再调用 Core 的资源解析实现，Core 仅保留 token issuer/verifier 与兼容包装，结构门禁和定向测试通过。
- 2026-08-30：Gateway Agent MCP 代理的 resource、scope 和安全 URL 校验已下沉到 `internal/application`；Core 继续拥有 token issuer/verifier，Gateway 只依赖认证 contract，结构门禁和安全校验测试已通过。
- 2026-08-30：Gateway Kafka consumer 所需的群组、会话、联系人和已读事件 decoder 已下沉到 `internal/application` 跨服务 contract；Gateway 生产代码不再依赖 Core domain，服务布局门禁已固定该边界，Core 仍保留事件生产与自身投影实现。
- 2026-08-30：调用审计确认 embedded Message repository wrapper 无独立调用者，已删除 `NewMessageProcessRepositories` 及其 inbox 开关转发，聚合入口直接使用 Message-owned constructor；整体 `NewRepositories` 仍作为 embedded 回滚组合保留。
- 2026-08-30：调用审计确认 embedded Search 字段没有生产或测试使用者，已删除聚合层 Search SQLC 构造；Search Service 继续由 Elasticsearch-owned runtime 独立装配，聚合层仅保留 embedded 回滚所需的四类 process composition。
- 2026-08-30：embedded repository composition 已将 Core 的 User、Group、Contact、File、Conversation、Admin 仓储访问收敛到 `CoreProcessRepositories`，移除聚合根的 Core 扁平字段，并通过 embedded/Core contract 测试；聚合层当前仅保留 process composition 指针。
- 2026-08-30：embedded repository composition 已将 Message、Outbox、Sync 仓储访问收敛到各自 process 分组，移除聚合根的 Message/Sync 扁平字段，并通过 embedded、Message、Sync contract 测试；Core/Agent 聚合字段继续按后续边界切片收敛。
- 2026-08-30：embedded repository composition 已将 Agent 全部仓储访问收敛到 `AgentProcessRepositories`，移除聚合根的 Agent 扁平字段，并通过 Agent 初始化与 repository contract 测试；Core/Message/Sync 的聚合字段仍按后续切片收敛。
- 2026-08-30：Core server 与 standalone bootstrap 已切换到 Core-owned repository/messaging 端口和本地组合函数，移除对 `internal/bootstrap/embedded` 聚合类型的直接依赖；embedded 仅在回滚组合边界适配，新增服务布局门禁防止 Core 依赖回流。
- 2026-08-30：复跑 `scripts/smoke-sync-cassandra-primary-compose.sh`，隔离微服务 Compose 真实验证 Cassandra schema init、MySQL migration、Core/Message/Sync 依赖 readiness 和 `primary=true` 配置；临时拓扑自动清理，共享环境长期观测、责任人批准和可执行回切仍未完成。
- 2026-08-30：复跑 `scripts/smoke-cassandra-read-routing.sh`，隔离环境真实验证 migration v50、Cassandra Timeline 页面主读，以及 payload 损坏和缺行按同一 locator 回退 MySQL；资源自动清理，生产主读比例、共享环境窗口、责任人批准和可执行回切仍未启用。
- 2026-08-30：Gateway HTTP/WS server、Agent 控制代理和 Search 边缘适配已从横向 `internal/gateway/` 迁入 `internal/services/gateway/server/`；Gateway bootstrap 直接引用服务自有实现，HTTP contract 与远程回滚行为保持兼容。
- 2026-08-30：Core HTTP/WS server、静态资源和通知适配器已从横向 `internal/server/` 迁入 `internal/services/core/server/`，Core 独立与 embedded 入口共用服务自有边界；旧目录扫描契约同步更新，HTTP、WS 和回滚行为保持兼容。
- 2026-08-30：服务布局门禁新增 shared `internal/bootstrap` 根目录生产 Go 文件回流检查，当前 embedded runtime 物理路径和各独立服务 bootstrap 边界由自动化契约持续保护。
- 2026-08-30：embedded 聚合 runtime 已从共享 `internal/bootstrap` 迁入 `internal/bootstrap/embedded/runtime/` 子包，Core embedded 兼容入口直接引用 embedded-owned runtime；共享 bootstrap 根目录不再持有生产级聚合实现，生命周期、Kafka、transport 和回滚语义保持兼容。
- 2026-08-30：embedded 聚合的 Message persistence ownership 策略及测试已迁入 `internal/bootstrap/embedded/`，共享 bootstrap 仅保留 runtime 生命周期编排；local/gRPC/remote 判断和回滚语义保持兼容。
- 2026-08-30：embedded Kafka managed topic 清单及契约测试已迁入 `internal/bootstrap/embedded/`，runtime 直接使用 embedded-owned API；topic 列表和版本化契约回归通过，独立服务 consumer ownership 保持不变。
- 2026-08-30：清理服务布局门禁中针对已删除 `internal/bootstrap/internal_rpc.go` 的六段过时检查，并修正仓库结构文档中的已删除 helper 描述；门禁现可无 IO 警告地验证当前 Core RPC/Kafka 物理边界。
- 2026-08-30：embedded Kafka 组合及其消息事件契约测试已迁入 `internal/bootstrap/embedded/`，legacy `internal/bootstrap` 不再持有 Kafka 注册实现；聚合 runtime、旧 Eino 回滚路径和实时投递注册顺序回归通过，后续继续评估 embedded composition 的剩余共享生命周期边界。
- 2026-08-30：Core RPC contract 测试已全部改用 `internal/services/core/rpc`，共享 bootstrap 的 Core RPC 函数 wrapper、类型别名和生产服务名常量均已删除；测试常量收纳至 `_test.go`，Core/embedded RPC 行为回归通过。
- 2026-08-30：Message service/错误兼容入口已删除，Message HTTP/WS 和事件消费者直接依赖 Message-owned contract；同步修正服务边界文档中关于 Message/Sync 兼容入口仍在保留的历史描述，剩余 `internal/compat/service` 仅承担跨版本 domain-event decoder 辅助。
- 2026-08-30：Group、Conversation、Contact、Session 的 domain-event decoder 已分别下沉到 Core-owned domain，生产代码对 `internal/compat/service` 的依赖归零；服务布局门禁新增生产 import 回流检查，兼容目录仅保留回归测试。
- 2026-08-30：Core RPC 组合逻辑已迁入 `internal/services/core/rpc/`，embedded runtime 改用 Core-owned composition；共享 `internal/bootstrap/internal_rpc.go` 已完全移除，新增边界测试并通过 Core/embedded RPC contract。
- 2026-08-30：审计确认 `internal/data/mysql` 及旧 repository facade 已不存在，业务 SQLC 仓储均位于对应服务 infrastructure；修正 `REPOSITORY-STRUCTURE.md` 与 `SERVICE-BOUNDARIES.md` 中残留的旧目录描述，并通过结构门禁，避免文档继续指导已退役布局。
- 2026-08-30：在最新 `master` 提交 `3adc755` 上复测 AD-005 的真实 MySQL 8.4、1000 成员 Conversation SQLC 批量投影；serial/batch 与并发对照均通过，batch 相比 serial 约降低 46.2/286.9 倍，投影行数均为 1000，锁等待增量为零。该结果属于单轮 SQL 层复测，端到端 P95、多轮统计和共享拓扑容量验证仍待完成。
- 2026-08-30：Kafka rebalance 隔离 smoke 通过，验证双 consumer、成员退出后的六分区 ownership 接管和 lag 归零；生产 Kafka ownership 切换、候选发布和可执行回滚 receipt 仍待共享环境门禁。
- 2026-08-30：Agent Runtime 独立回归通过，Vitest 125 个测试文件/662 个测试、TypeScript typecheck 和生产 build 均成功；AD-009/AD-030 的真实 Kafka、Temporal、Capability RPC、外部 MCP 和 active authority 联调仍待共享环境证据。
- 2026-08-30：Elasticsearch Search Service 隔离 smoke 通过，验证 Elasticsearch 9.5.2 查询、Core 派生 scope 和内部 RPC contract；Search 生产 Alias 切换、共享环境观测、回滚责任人和可执行回切仍待完成。
- 2026-08-30：Cassandra read-routing 隔离 smoke 通过，真实验证 Timeline 页面 Cassandra 主读及 payload 损坏/缺行按同一 cursor 回退 MySQL；AD-019 的消息正文替代读契约、共享环境主读观测和可执行回切仍待完成。
- 2026-08-30：Cassandra primary 隔离 Compose smoke 通过，验证 schema init、MySQL migration、Sync primary 配置和 readiness；AD-043 的共享环境真实 hydration、主读灰度、责任人批准与可执行回切仍待完成。
- 2026-08-30：shared `newInternalRPCServer` 与 `dialInternalRPC` 已完成调用审计并退休；Core embedded 组合和 RPC fixture 直接使用 `internal/platform/rpc`，认证、TLS、caller allowlist 和回滚行为保持兼容。
- 2026-08-30：Core Agent RPC caller-to-method 权限策略已迁入 `internal/services/core/rpcpolicy`，shared bootstrap 仅调用 Core-owned policy；Core/Agent mTLS、服务 caller allowlist 和 Agent/Search/Sync 权限 contract 通过回归测试，embedded 回滚入口保持可用。
- 2026-08-30：在最新 `master` 上重跑微服务隔离 smoke，真实验证 readiness、metrics、Core proxy、mTLS 启动、远程 WS ownership 以及 Agent EventLedger/Task/Run 幂等；隔离拓扑自动清理，AD-049 的共享环境 ownership、候选发布与可执行回滚 receipt 仍待完成。
- 2026-08-30：追加执行同一微服务隔离 smoke，所有候选服务和基础依赖继续通过健康检查及业务断言；结果仍限于隔离拓扑，不改变生产 Kafka ownership、候选发布或回滚 receipt 状态。
- 2026-08-30：调用审计确认导出 `RestrictCoreServiceMethods` 无仓内调用者，已删除 shared policy 包装；Core RPC server 保留私有权限策略，已有 Agent/Search/Sync 权限 contract 不变。
- 2026-08-30：调用审计确认 shared `NewCoreRPCServerWithAgent` 仅被测试使用，已删除该 facade；Agent RPC contract 测试直接构造 adapter，仍被 runtime 使用的 control/projection/artifact 构造路径保持兼容。
- 2026-08-30：shared `RegisterCoreProjectionKafkaHandlers` 已完成调用审计并退休；Core standalone runtime 直接使用 Core-owned projection 注册器，Conversation projection、Kafka ownership 和回滚语义保持兼容。
- 2026-08-30：shared `internal/bootstrap.RunServer` 已完成调用审计并退休；Core/Gateway 的 TLS 与服务启动入口由各自 bootstrap 持有，embedded 聚合不再暴露通用 server runner。
- 2026-08-30：shared Kafka 注册 facade `RegisterKafkaHandlersWithRepositories` 与 `RegisterCoreKafkaHandlersWithRepositories` 已完成调用审计并退休；embedded runtime 继续使用私有组合，Core projection 保持由 Core-owned 入口注册，Kafka ownership 和回滚语义保持兼容。
- 2026-08-30：Gateway Agent 与 Message Core capability client 已迁入各自服务 bootstrap，shared `DialGatewayAgentCapability`、`DialCoreCapability` 和通用 caller dialer 已删除；Gateway/Message 服务身份、Core 权限范围和回滚行为保持兼容。
- 2026-08-30：Gateway Core capability client facade 已完成调用者迁移并从 shared `internal/bootstrap` 删除；Gateway 自有 bootstrap 直接持有 client 身份和平台 transport，Core 权限校验与回滚行为保持兼容。
- 2026-08-30：Search/Sync Core capability client facade 已完成调用者迁移并从 shared `internal/bootstrap` 删除；Search/Sync 自有 bootstrap 直接持有服务身份和平台 RPC transport，权限校验与回滚行为保持兼容。
- 2026-08-30：Search/Sync RPC client facade 已完成调用者迁移并从 shared `internal/bootstrap` 删除；Gateway、Search、Sync 与 embedded 各自持有所需 client 装配，协议、身份和回滚行为保持兼容。
- 2026-08-30：Search/Sync RPC server facade 已完成调用者迁移并从 shared `internal/bootstrap` 删除；contract 测试直接使用各服务 bootstrap，服务协议、认证和回滚行为保持兼容。
- 2026-08-30：调用审计确认 shared Message RPC server/client facade 无仓内调用者，已删除 `NewMessageRPCServer`、`DialMessageApplication` 和 `DialCoreMessageApplication`；Message、Gateway 与 embedded 各自使用服务边界内的 RPC 装配，协议和认证行为保持兼容。
- 2026-08-30：Sync transport/shadow 已从共享 `internal/bootstrap` 迁入 `internal/bootstrap/embedded/`，embedded runtime 改用 embedded-owned transport；local/grpc/shadow 回退和 checkpoint 语义保持兼容，shared bootstrap 的 Message/Sync transport 实现均已完成物理收敛。
- 2026-08-30：Message transport/shadow 已从共享 `internal/bootstrap` 迁入 `internal/bootstrap/embedded/`，embedded runtime 改用 embedded-owned transport；local/grpc/shadow 回退语义保持兼容，Sync transport 仍待独立切片收敛。
- 2026-08-30：调用审计确认 `internal/bootstrap.NewCoreRPCServerWithAgentControl` 无生产或测试调用者，已删除该 shared RPC 包装；`NewCoreRPCServerWithAgent` 与 `WithAgentControlAndProjection` 因仍有 contract/embedded 调用继续保留。
- 2026-08-30：Cassandra Projector runtime 已从共享 `internal/bootstrap` 迁入 Message bootstrap，`cmd/tools/cassandra-projector` 直接使用服务自有入口；共享 bootstrap 不再持有 Cassandra Projector 生命周期，projection 与回滚语义保持稳定。
- 2026-08-30：全仓调用审计确认 Core、Agent、Search repository alias 无仓内调用者，已删除三组 alias、目录说明及 `internal/data/mysql` 历史兼容目录；服务 SQLC repository 由各自 infrastructure 唯一持有，门禁已阻止旧目录回流。
- 2026-08-30：全仓调用审计确认 `internal/data/mysql/store_compat.go` 无仓内调用者，已删除该 Store facade；MySQL 事务边界继续由 `internal/platform/mysql` 唯一持有，Core/Agent/Search repository alias 仍按调用者保留。
- 2026-08-30：全仓调用审计确认 `internal/store` MySQL/Redis 入口无仓内调用者，已删除两个兼容实现和目录说明，生产与运维代码继续统一使用 `internal/platform/mysql`、`internal/platform/cache`；服务布局门禁已阻止旧 store 回流。
- 2026-08-30：全仓调用审计确认 Message/Sync repository facade 无生产或测试调用者，已删除 `internal/data/mysql/repository/message_compat.go` 与 `sync_compat.go`，并收紧服务布局门禁；Core、Agent、Search 兼容入口及 embedded 回滚边界保持不变。
- 2026-08-29：调用审计确认 `internal/bootstrap.RegisterGatewayKafkaHandlers` 已无调用者，embedded 装配已直接使用 Gateway infrastructure 注册器并删除 facade；Gateway Kafka 兼容入口完成退休。
- 2026-08-29：Gateway runtime 已直接调用 Gateway Kafka infrastructure 注册器，移除生产路径对 `internal/bootstrap` Kafka 兼容入口的依赖；架构测试锁定 runtime 不得回流共享 bootstrap。
- 2026-08-29：Gateway Kafka 注册器与 authority handler factory 已迁入 `internal/services/gateway/infrastructure/kafka/`，`internal/bootstrap` 仅保留兼容转发及 Core/Message projection；Gateway Kafka 装配边界已完成收敛。
- 2026-08-29：Gateway group message delivery handler 已迁入 `internal/services/gateway/infrastructure/kafka/`，覆盖普通群 fan-out、hot-group notify、文件消息和 Timeline notify；Gateway Kafka handler 的共享实现迁移已完成，后续转入兼容入口审计与删除。
- 2026-08-29：Gateway direct message delivery handler 已迁入 `internal/services/gateway/infrastructure/kafka/`，Timeline notify 和文件消息映射由服务自有实现持有；group message delivery 仍待处理 hot-group 依赖后迁移。
- 2026-08-29：Gateway 群事件 Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka/`，覆盖创建、更新、成员变更和解散通知；Core 的 `group.created` 会话初始化仍保留公共解码依赖，剩余消息 delivery handler 继续待迁移。
- 2026-08-29：Gateway `session.force_logout` Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka/`，连接控制接口归属 Gateway；剩余消息与群事件 delivery handler 仍待继续迁移。
- 2026-08-29：Gateway `contact.friend.deleted` Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka/`，新增服务自有契约测试；剩余消息与会话事件 delivery handler 仍待继续迁移。
- 2026-08-29：Gateway `conversation.direct.read` Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka/`，新增服务自有契约测试；其余消息与会话事件 delivery handler 仍待继续迁移。
- 2026-08-29：Gateway realtime delivery authority fence 已迁入 `internal/services/gateway/infrastructure/kafka/`，embedded 装配改用服务自有实现；完整消息 delivery handler 仍待继续迁移。
- 2026-08-29：Gateway 热群通知聚合器及测试已迁入 `internal/services/gateway/infrastructure/kafka/`，共享 handler 改用服务自有 `Notifier`；完整消息投递 handler 仍待按依赖闭包继续迁移。
- 2026-08-29：Message `send_requested` 持久化 Kafka handler 已迁入 `internal/services/message/infrastructure/kafka/`，独立和 embedded runtime 均直接使用服务自有 handler，原共享注册包装已退休。
- 2026-08-29：Message Outbox relay 已迁入 `internal/services/message/infrastructure/kafka/`，独立和 embedded runtime 均直接使用服务自有 relay，原共享 alias/构造包装已退休。
- 2026-08-29：Message shadow 的 Query-only adapter 及测试已迁入 `internal/services/message/bootstrap/`，独立 runtime 直接使用服务自有实现并移除对应共享 facade；Message 专属旧兼容入口已按调用者完成清理。
- 2026-08-29：惰性 Core Capability adapter 及其重试测试已迁入 `internal/services/message/bootstrap/`，Message runtime 直接使用服务自有实现并移除对应共享 facade；AD-049 的共享环境冷启动、ownership 和回切证据仍待完成。
- 2026-08-29：五条主要 Epic 分支已合并当前 `master` 并推送，均恢复为以最新主线为祖先的阶段开发基线；后续短分支继续按单一里程碑隔离并回合并。
- 2026-08-29：Agent application 兼容 facade 的剩余测试调用已迁移至服务边界，删除空的 `internal/app/agent_application_compat.go`，并同步更新服务布局门禁与仓库边界文档；`internal/app` 继续仅保留实际仍被使用的兼容测试/聚合入口。
- 2026-08-29：Agent Execution Policy 测试已改用 Agent application 的持久策略构造器，删除 `internal/app` 中无调用的策略 alias 与构造转发；剩余兼容入口继续按真实调用者收敛。
- 2026-08-29：Memory Resolver 测试已迁入 `internal/services/agent/application`，其 memory、invocation 和 task reader stub 随测试归属迁移，并删除 `internal/app` 对应 facade。
- 2026-08-29：Runtime Promotion Evidence Review 测试已迁入 `internal/services/agent/application`，其 operator control 与 artifact reader stub 随测试归属迁移，并删除 `internal/app` 对应 facade。
- 2026-08-29：Memory Owner Control 测试已迁入 `internal/services/agent/application`，其 owner store stub 与 fixture 随测试归属迁移，并删除 `internal/app` 对应 facade。
- 2026-08-29：Artifact Service 测试已迁入 `internal/services/agent/application`，其 policy、metadata、blob stub 均随测试归属迁移，并删除 `internal/app` 对应 facade。
- 2026-08-29：Memory Candidate Promotion 测试已迁入 `internal/services/agent/application`，直接覆盖 Agent-owned 实现并删除 `internal/app` 对应 facade。
- 2026-08-29：MCP Tool Round 与 Terminal 测试已迁入 `internal/services/agent/application`，共享 stub 随测试归属迁移，并删除 `internal/app` 对应构造转发。
- 2026-08-29：MCP Tool Round 测试已补齐 Agent application 自有的最小 invocation reader stub，避免服务测试依赖 `internal/app` 测试包。
- 2026-08-29：MCP Readiness Evidence 测试已迁入 `internal/services/agent/application`，直接覆盖 Agent-owned publisher/resolver 并删除 `internal/app` 对应 facade。
- 2026-08-29：Definition Catalog 测试已迁入 `internal/services/agent/application`，直接覆盖 Agent-owned 实现并删除 `internal/app` 对应 facade，继续保留共享 policy stub 依赖的兼容测试。
- 2026-08-29：Approval Grant Resolver 测试已迁入 `internal/services/agent/application`，直接覆盖 Agent-owned 实现并删除 `internal/app` 对应 facade；审批主服务仍依赖共享 policy stub，暂保留其兼容测试路径。
- 2026-08-29：Agent facade 调用审计确认 `validSubscriptionDefinitionV1` 和 `agentCommandCapabilityIDV1` 两个未导出转发无测试或生产调用者，已删除；剩余 Agent application 兼容入口仍由兼容测试或 embedded 回滚路径使用。
- 2026-08-29：调用审计确认 Core、Sync、Agent repository facade 已无生产或测试调用者，已删除 `internal/app/*_repository_compat.go` 三组兼容入口并收紧服务布局门禁；同时移除门禁中过时的 Agent repository 存在性断言，`internal/app` 当前仅保留仍被 Agent 测试使用的 application facade。
- 2026-08-29：`internal/app/composition_compat.go` 已无生产调用者，composition 测试已迁入 `internal/bootstrap/embedded` 并改为直接覆盖 embedded composition；聚合 app composition facade 已删除，剩余兼容入口继续按调用者逐步退休。
- 2026-08-29：调用审计确认 `internal/app/composition_compat.go` 中的 Inbox 写入开关转发和旧 Message application 构造均无外部调用者，已删除两处兼容入口；其余 composition facade 仍服务于 embedded 回滚或兼容测试，继续按调用者迁移。
- 2026-08-29：服务布局门禁已同步删除 Core capability facade 的历史必需登记，当前兼容根目录只保留仍有调用者的 adapter、说明文件与兼容测试。
- 2026-08-29：全仓调用审计确认 `internal/app/core_capability.go` 无生产或测试调用者，已删除该孤立兼容构造；Core application 和 embedded repository composition 继续作为唯一装配路径。
- 2026-08-29：Core 独立 runtime 的模式校验已完成测试归属迁移，删除旧 bootstrap 中无生产调用者的重复 facade；embedded 组合入口仍保留，Core standalone 与 remote/embedded 模式约束由服务自身测试锁定。
- 2026-08-29：Core bootstrap 已将 embedded 初始化别名与独立入口物理拆分；`entrypoint.go` 现在完全脱离旧 bootstrap，兼容路径集中在 `embedded_compat.go`，独立服务与 embedded 回滚行为保持不变。
- 2026-08-29：Core 服务入口已收回自身 HTTP/TLS 启动逻辑，旧 bootstrap 仅继续承担 embedded 初始化兼容；独立 Core 的 TLS 文件校验、日志和优雅运行入口由服务 bootstrap 自有，新增架构测试防止旧 RunServer 转发回流。
- 2026-08-29：隔离微服务 smoke 已完成真实 `message.direct.created` 事件验证；同一 Kafka 事件连续发布两次后，MySQL EventLedger、Shadow Plan、Shadow Run 各保持单条并完成，Task 保持 `running` 以遵循当前 Task/Run 分层生命周期。生产事件流仍保持 shadow，Temporal/active authority 未开启。
- 2026-08-29：微服务隔离 smoke 已实际验证 Agent Runtime health endpoint、Kafka shadow consumer group 加入和主/retry 分区分配；真实 message publish 到 Agent Task 的 shadow 语义已补充验证，生产 active authority 仍关闭。
- 2026-08-29：独立 Agent Runtime 已完成默认安全配置的进程 smoke，`/livez`、`/readyz` 和 SIGINT 退出均通过；真实 Kafka/Temporal/Capability RPC 联调仍需按环境准备外部依赖和可回滚 receipt。
- 2026-08-29：Agent Runtime TypeScript generated protobuf 已与当前 Go contract 对齐，补齐 system-message RPC；Runtime 完整测试、typecheck、build 和 proto drift 均通过，下一步继续验证独立 Runtime 的真实服务启动与事件触发。
- 2026-08-29：assistant seed 已迁移到 `internal/services/core/application`，独立 Core 与 embedded 路径共享 Core-owned 初始化；Core bootstrap 的旧业务初始化依赖已清除，剩余兼容依赖集中在 embedded composition 和少量平台生命周期 facade。
- 2026-08-29：Core Conversation Kafka projection 已迁移到 `internal/services/core/infrastructure/kafka`，独立 runtime 直接使用服务自有 projector；旧 bootstrap 仅保留兼容转发，assistant seed 仍待进一步迁移。
- 2026-08-29：复核生产 Go 代码、`go.mod`/`go.sum` 和 sqlc 生成漂移，确认当前无 GORM 运行时引用或模块依赖；服务布局门禁新增 GORM 回流检查，继续保障 `database/sql + sqlc` 统一数据访问边界。
- 2026-08-29：独立 Core 的 runtime、system-message sender 和 RPC adapter 已迁移到 `internal/services/core/bootstrap`，生产入口不再通过旧 bootstrap 初始化 Core；Core Kafka projection 与 assistant seed 仍是显式兼容依赖，后续继续收敛。
- 2026-08-29：Gateway 生产 RPC server/client 已迁入 Gateway bootstrap 并直接使用平台 transport，覆盖 Message、Sync、Core、Search 和 realtime delivery observation；Kafka handler 仍保留共享兼容边界，后续继续收敛。
- 2026-08-29：Sync 生产 RPC adapter 已迁入 Sync bootstrap 并直接使用平台 transport，保留原有 Core capability 调用方身份和 query server 白名单；剩余 legacy 依赖继续按服务切片收敛。
- 2026-08-29：Message 生产 RPC adapter 已迁入 Message bootstrap 并直接使用平台 transport，runtime 的 RPC server 字段也已切换为 `internal/platform/rpc.Server`，不再依赖共享 bootstrap RPC 类型；Lazy Core、权限校验和其他服务基础设施兼容边界仍待后续切片收敛。
- 2026-08-29：Embedded 兼容入口 `internal/bootstrap.NewMessageRPCServer` 曾转发 Message bootstrap 的服务自有实现；后续调用审计已确认无仓内调用者并于 2026-08-30 退休。
- 2026-08-29：Message bootstrap 的惰性 Core 重试测试已改用本地最小 gRPC adapter，测试包不再反向依赖共享 bootstrap，进一步固定 Message 服务的物理边界。
- 2026-08-29：Embedded Kafka 装配已直接注册 Message-owned persistence handlers，删除无外部调用者的共享 `RegisterMessageKafkaHandlers` 包装，Message Kafka 兼容表面进一步缩小。
- 2026-08-29：Embedded runtime 已直接持有并创建 `messagekafka.Relay`，删除仅供旧 bootstrap 内部使用的 Outbox alias/构造包装；Outbox 启动条件和 embedded 回滚语义保持兼容。
- 2026-08-29：删除已无调用者的 `internal/bootstrap.VerifyMessageDatabaseBoundary` 兼容转发，Message 数据库权限探针由 `internal/services/message/infrastructure/mysql` 唯一持有，权限语义保持不变。
- 2026-08-29：TLS 证书与私钥路径校验已下沉至 `internal/platform/runtime.ValidateTLSFiles`，Core、Gateway 和 embedded runtime 统一使用平台实现，删除共享 bootstrap 重复 helper。
- 2026-08-29：时间线通知模式校验已下沉到 `internal/platform/runtime.ValidateTimelineNotifyMode`，Gateway、Core 和 embedded runtime 统一使用平台启动校验，删除共享 bootstrap 重复实现与无调用者兼容入口。
- 2026-08-29：共享 `internal/bootstrap/dependency_readiness.go` 已确认无生产调用者并删除，readiness 实现和 assignment/fence 测试统一归属 `internal/platform/runtime`。
- 2026-08-29：Search 生产 RPC bootstrap 已脱离 `internal/bootstrap`，直接使用平台 RPC transport；Core capability server 仅作为测试 fixture 使用 legacy helper，避免重复实现 Core 方法权限策略，后续继续迁移 Message、Sync 和 Gateway 协议 adapter。
- 2026-08-29：Internal RPC 通用 transport 已迁入 `internal/platform/rpc/`，并由旧 `internal/bootstrap` helper 转发；平台层覆盖认证、TLS 1.3 mTLS、health check、拨号超时和优雅关闭，服务协议 adapter 与方法权限仍按服务边界继续收敛。
- 2026-08-29：修复 Agent MCP RPC drill fixture 对旧 `internal/transport/grpc/gen` 生成路径的引用，统一切换到 `api/gen/go`；`master` 全量 Go 测试、服务布局、架构文档和 Compose 门禁均已恢复通过。
- 2026-08-29：修复 Gateway runtime 迁移后服务入口 `RunServer` 自递归导致的启动回归；架构测试现在锁定入口必须委托 `RunGatewayServer`，并通过 Gateway 与全量 Go 测试验证。
- 2026-08-29：Gateway runtime 已从共享 `internal/bootstrap` 迁入 `internal/services/gateway/bootstrap/`，直接组合 Gateway HTTP/WS、Redis Presence/限流、Kafka 和 realtime authority；共享 RPC、TLS 仍按平台兼容边界管理，Gateway Kafka handler 与注册兼容入口已完成迁移和退休。
- 2026-08-29：Message runtime 与配置校验测试已从共享 `internal/bootstrap` 迁入 `internal/services/message/bootstrap/`，直接组合 Message SQLC repository、Kafka/Cassandra、Outbox 和平台 runtime；Lazy Core、少量共享基础设施和 Internal RPC 仍按回滚边界治理。
- 2026-08-29：Sync runtime、数据库权限边界校验及相关测试已从共享 `internal/bootstrap` 迁入 `internal/services/sync/bootstrap/`，直接组合 Sync infrastructure、Kafka/Cassandra 与平台 runtime；共享 Internal RPC 暂保留窄 compatibility adapter，后续继续抽取平台 RPC transport。
- 2026-08-29：Search runtime、单测和 Elasticsearch 集成测试已从共享 `internal/bootstrap` 迁入 `internal/services/search/bootstrap/`，Search application 与平台 runtime 直接由服务边界组合；共享 Internal RPC 暂保留窄 compatibility adapter，后续继续抽取平台 RPC transport。
- 2026-08-29：Search Indexer runtime 已从共享 `internal/bootstrap` 迁入 `internal/services/search-indexer/bootstrap/`，直接组合服务自有 projector 与 Kafka、Elasticsearch、metrics/readiness 平台能力；旧实现路径由结构门禁阻止回流，后续继续处理 Search、Sync、Message 和 Gateway 的实际启动实现迁移。
- 2026-08-29：将依赖 readiness 编排、Kafka consumer 初始分配检查、Cassandra schema 检查和 RPC serving 绑定下沉到 `internal/platform/runtime`，各服务 runtime 已切换公开平台 API，并保留旧 bootstrap helper 作为回滚兼容出口；服务特有启动校验和共享环境 readiness 证据仍待继续收敛。
- 2026-08-29：Kafka 三节点 quorum、consumer rebalance 和 Prometheus observability smoke 均通过，验证 RF=3/min ISR=2 下的 broker 故障拒绝与恢复、6 分区 ownership 接管、lag 归零及 retry/DLQ/ISR 指标；同时修复 cluster profile 漏挂 duplicate hydration 和 Agent Timeline repair rule files，并加入 Compose 挂载门禁。共享候选环境的 Kafka ownership 切换与可执行回滚 receipt 仍待完成。
- 2026-08-29：新增 `deploy/microservices/inbox-projector.yml` 可回滚 override，将 Message projector ownership、最小 MySQL 账号和 Sync projector 开关绑定到同一配置切片；`scripts/check-compose.sh` 已加入一致性门禁，实际候选环境切换仍待维护窗口 receipt。
- 2026-08-29：重新通过 `scripts/smoke-sync-write-ownership.sh`，真实 MySQL 8.4 验证 atomic/projector 最小权限、Inbox 写责任切换和 rollback contract；该证据支持候选部署，仍不替代共享环境的 Kafka ownership 与生产回切 receipt。
- 2026-08-29：隔离微服务消息 smoke 已增加可选 `SMOKE_INBOX_PROJECTOR=1` overlay 路径，并对异步 Inbox 物化使用有界等待；运行时 projector 候选验证与共享环境 ownership receipt 仍待执行。
- 2026-08-29：候选 projector 端到端 smoke 已通过，覆盖 Gateway WebSocket、Message/Outbox、Sync 异步 Inbox 和 Seq 查询；首次运行的宿主端口冲突通过更换隔离端口恢复，结果不改变共享环境 Kafka ownership 与生产回切 receipt 的待办状态。
- 2026-08-29：隔离微服务 smoke 已生成 `dipole.microservices.smoke-receipt.v1` 成功 receipt，绑定 revision、Compose project、运行模式和回滚动作；该 receipt 提升候选证据可复核性，仍不替代共享环境 ownership 切换审批。
- 2026-08-29：receipt contract 实际校验通过，确认候选 projector 拓扑的 schema、模式标志、无数据迁移回滚声明和文件权限均符合约束；共享环境 Kafka ownership 切换与回切 receipt 仍待完成。
- 2026-08-29：MySQL 全局连接初始化已从 `internal/store` 收敛到 `internal/platform/mysql`，Core、Message、embedded runtime、Bloom 和维护工具已切换新入口；`internal/store/mysql_compat.go` 仅保留回滚兼容，Redis 全局状态仍待后续单独收敛。
- 2026-08-29：Redis 客户端初始化和全局状态已从 `internal/store` 收敛到 `internal/platform/cache`，Core、Gateway、Message、Presence、Hot Group、限流和 realtime 运维工具已切换新入口；`internal/store/redis_compat.go` 仅保留回滚兼容。
- 2026-08-29：Hot Group Detector 已支持显式 Redis 客户端并由生产 Composition Root 注入，结构门禁阻止生产装配回到无参数全局构造；兼容构造仍保留，Presence 和限流的显式客户端注入继续作为后续切片。
- 2026-08-29：Presence 已支持显式 Redis 客户端并由 Gateway、embedded Server 和 WebSocket 路由注入，新增隔离测试与结构门禁；兼容构造仍保留，限流的显式客户端注入继续作为后续切片。
- 2026-08-29：Rate Limiter 已支持显式 Redis 客户端并由 Gateway、embedded Server 和 Agent MCP 入口注入，新增隔离测试与结构门禁；兼容构造仍保留，Redis 业务适配器的全局状态清理进入后续阶段。
- 2026-08-29：Rate Limiter 执行路径已移除对全局 Redis 的回退，普通业务 fail-open 与 Agent MCP fail-closed 均基于实例客户端判定；全局状态仅由兼容构造保留，Redis 业务适配器清理进入收尾阶段。
- 2026-08-29：Sync Kafka Projector 已迁入 `internal/services/sync/infrastructure/kafka/`，复用 Message domain event contract；旧 `internal/projector/sync/` 已由结构门禁阻止回流，后续仍需继续收敛跨服务运维工具和共享 SQLC 基础设施。
- 2026-08-29：Search Indexer Kafka Projector 已迁入 `internal/services/search/infrastructure/kafka/`，复用 Message domain event contract；旧 `internal/projector/search/` 已由结构门禁阻止回流，Cassandra Projector 仍保留为独立实验性运行时，后续继续评估其入口归属。
- 2026-08-29：Cassandra Message Projector 已迁入 `internal/services/message/infrastructure/cassandra/`，复用 Message domain event contract；旧 `internal/projector/cassandra/` 已由结构门禁阻止回流，独立 `cmd/tools/cassandra-projector` 入口暂保留用于可选存储实验和回滚。
- 2026-08-29：SQLC MySQL 事务 Store 已迁入 `internal/platform/mysql/`，旧 `internal/data/mysql/store_compat.go` 已在后续调用审计后退役；SQLC generated、mapper 和回填工具仍待按服务/平台职责继续拆分。
- 2026-08-29：SQLC generated 输出和 mapper 已迁入 `internal/platform/mysql/`，`sqlc.yaml` 与漂移检查已同步；回填/清理工具仍保留在 `internal/data/mysql`，后续需按服务职责继续拆分。
- 2026-08-29：Elasticsearch client、版本化 schema、Alias 和 projection adapter 已迁入 `internal/platform/elasticsearch/`；Search/Indexer 业务边界保持独立，后续需继续评估 Elasticsearch 连接 owner 与 Search Service 独立 module 的最终收敛。
- 2026-08-29：Search 回填、归档、对账、Alias 切换和 Outbox 清理的装配代码已从 `internal/bootstrap/` 收纳到 `internal/operations/search/`；长期服务启动包与一次性运维操作边界已通过结构门禁固定，Sync/Cassandra 运维运行时仍待按同一模式收敛。
- 2026-08-29：Sync baseline/replay/reconcile 与 Cassandra backfill/archive/reconcile 装配已分别迁入 `internal/operations/sync/`、`internal/operations/cassandra/`；长期服务运行时保留在 bootstrap，三类运维目录均由结构门禁保护。
- 2026-08-29：Agent Memory lineage backfill 装配已迁入 `internal/operations/agent/`，Agent 长期运行时与高风险一次性维护入口完成目录隔离；后续仍需继续收敛共享 Composition Root。
- 2026-08-29：embedded 聚合 `Repositories`、`MessagingServices` 及其构造实现已迁入 `internal/bootstrap/embedded/`；`internal/app` 收缩为兼容 facade，生产 bootstrap 已切换新边界，Agent 兼容构造仍单独保留待后续拆分。
- 2026-08-29：Agent bootstrap 已改用 Agent-owned application constructors，清除 runtime/kafka 对 `internal/app` 聚合 facade 的最后两处生产引用；服务布局门禁现禁止所有外部生产代码依赖该入口，后续待 embedded/兼容测试退休后删除 facade。
- 2026-08-29：protobuf Go 生成物已从 `internal/transport/grpc/gen/` 收纳到 `api/gen/go/`，同步更新所有 transport、Gateway、Bootstrap 和 Realtime 引用；协议源、生成物与服务适配层边界已由 `check-proto` 和服务布局门禁固定。
- 2026-08-29：Cassandra routing、shadow message store 和 Sync hydration fallback 已迁入 `internal/platform/storage/`；装饰器仍只通过 application port 运行，后续需继续评估迁移完成后的删除时机和 routing/shadow 配置 owner。
- 2026-08-29：跨 Message/Sync 复用的 Cassandra Timeline、连接和 hydration 适配器已迁入 `internal/platform/cassandra/`；服务业务 projection 保持在各自边界，后续仍需评估 routing/shadow 装饰器和 Cassandra 数据 owner 的最终归属。
- 2026-08-29：兼容别名已从 `internal/service` 收纳到 `internal/compat/service`；旧 `internal/service` 实现已清空，后续继续缩减其他兼容入口。
- 2026-08-29：确认 `internal/service/event_publisher.go` 已无调用者并删除，`internal/service` 不再承载 Go 实现；服务布局门禁阻止该目录重新出现业务实现，跨服务事件契约继续由 application port 和版本化事件包承载。
- 2026-08-29：滚动更新日志已同步当前 SQLC-only 数据访问和 Eino `v0.9.17` 依赖，清除 GORM 共存与旧 Eino 版本的过期表述。
- 开始处理时补充负责人或关联 Issue/PR；解决后记录提交、验证方式和完成日期。
- 本台账描述风险和演进方向，不代表当前迭代立即修改对应实现。
- 2026-08-30：确认 Remote GPU 开发阶段策略：GPU 任务存在时仍允许启动 Dipole CPU/容器/压测任务，但必须完成资源、端口、活动会话检查并使用隔离资源；GPU 进程及其容器、绑定和数据目录保持只读保护。
- 2026-08-30：远程开发入口已增加用户态 Go 自动发现，未指定路径时选择 `/home/admin1/.local/go-*` 最高版本并保持 `GOTOOLCHAIN=local`；显式路径优先，系统 Go 不修改。
- 2026-08-30：A6 Web Sync observation finalize 新增对象存储归档收据校验，Evidence 固定 URI、object version、ETag 和未来 retention；缺少收据只产生 `blocked`，真实客户端观察窗口与共享环境归档仍待执行。
- 2026-08-30：新增 `package-web-sync-bundle.sh` 与 3 项回归测试，以干净 Git revision、显式 `shadow|primary|off` 模式和稳定 tar 元数据生成不可覆盖 `web-sync-bundle.v1`；输出固定 `0600`，源目录内输出 fail-closed。该工件为真实观察窗口提供可复核输入，客户端流量和 Prometheus 共享窗口仍待完成。
- 2026-08-30：`remote-dev.sh web-sync-bundle` 纳入远程 CPU/容器工作流，提交同步后固定生成 shadow bundle 到 `/tmp`，Remote GPU 可在已有 GPU 任务期间执行；入口 16 项契约测试通过，不改变默认 Sync 模式。
- 2026-08-31：Remote GPU 已为 `45a80b3d475f4ba0317addab9d11ee0cb93397f2` 生成 `web-sync-shadow-45a80b3d` 候选归档，记录于 [A6 bundle 证据](../../benchmarks/a6-web-sync-bundle-45a80b3d/README.md)。该工件只绑定 revision、模式和 SHA-256；因主机存在活动登录会话，隔离 Prometheus smoke 与真实客户端 24 小时窗口尚未启动。
- 2026-08-31：Remote GPU 在隔离 worktree 为当前 `2ca6b1992d409ad4b4dab4fc86842cd28cc1e543` 生成 `web-sync-shadow-2ca6b199` Shadow 包，SHA-256 为 `d72207d7c70a88d4dc0f11c348c2e545589c3d5dbfd056aab323f1aee78b3b18`，权限 `0600`，记录于 [A6 bundle 证据](../../benchmarks/a6-web-sync-bundle-2ca6b199/README.md)。Vue production build 与 observation contract 的 14 项测试通过；由于主机仍有 25 个活动登录会话，未启动 Compose、Prometheus、Alertmanager 或真实流量窗口，默认客户端模式不变。

### AD-042：正式技术架构图与已发布分层拓扑发生漂移

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **完成日期：** 2026-08-28
- **影响范围：** `docs/technical-architecture.svg`、微服务边界、Timeline/Projection 说明、Agent Runtime 迁移叙事
- **解决方式：** 将架构图更新为当前 Core/Message/Gateway/Sync/Search/Agent Runtime 分层，补充 sqlc、`user_sync_inbox`、Cassandra/Elasticsearch 影子投影和回滚门禁，并移除 `AutoMigrate`、无 Inbox 单体及旧 Eino 主链路的过时表述。图中仍明确标注本地合并启动、影子能力和默认关闭边界。
- **验证：** `scripts/check-architecture-docs.sh`、SVG XML 解析和 `git diff --check` 通过；本次只修改文档，不改变运行配置或服务权限。
- **长期约束：** 服务拓扑、数据 ownership、默认开关或语言职责变化时，必须同步更新架构图、对应正式文档、更新日志和台账；架构图不得把 shadow、fallback 或离线契约描述成生产主路径。
- **本轮进展：** `ARCHITECTURE-QA.md` 已同步当前 Message Store、User Inbox Timeline、Conversation Seq/read_seq、sqlc 和微服务拓扑，移除早期无 Inbox、GORM 与纯模块化单体的现状描述。
- **本轮进展：** 面试问答、消息存储模型和同步策略已同步当前服务目录与 Timeline 实现；旧 `after_id`、`/messages/offline` 和 `unread_count` 已明确标注为兼容语义，避免当前设计说明继续引用过时主路径。新增项目学习与面试主文档，以状态标签、证据链接和限制项约束简历及现场表述，避免将默认关闭或规划能力描述为已上线成果。
- **本轮进展：** 学习与面试主文档增加合并切片维护记录，固定对外表述、演示、证据、追问、限制和复核条件；根 README 新增直接入口。该流程持续要求以代码、契约、测试和运行记录校正叙事，文档本身不构成运行时验收。
- **本轮进展：** 长期开发路线图已同步三大阶段和独立 C++ Realtime Delivery 轨道，移除旧 Cgo 必做叙述；C++ 仍保持候选服务和 Go authority，未改变默认运行路径。

## 待处理

### AD-054：服务入口已拆分但共享实现区仍缺少服务级物理边界

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** `internal/app`、`internal/service`、`internal/store`、服务级数据库所有权和后续多语言迁移
- **现状：** `cmd/services/` 已按部署单元提供 Core、Gateway、Message、Sync、Search 和 Search Indexer 入口，`docs/architecture/SERVICE-BOUNDARIES.md` 已固定职责与共享层规则；多个服务仍通过 `internal/` 共享业务实现，部分 Core 兼容链路仍保留跨域组合。
- **风险：** 仅凭独立二进制和镜像无法证明服务实现自治；后续 sqlc 多语言统一、Cassandra/Elasticsearch 存储替换或 C++ 数据面切换时，跨服务隐式依赖可能造成重复写入和回滚范围不清。
- **下一步：** 以 application port 和 contract test 为边界，按 Core、Message、Sync、Search、Agent 顺序拆分 Composition Root、业务实现和数据访问包；每次迁移保持旧入口可回切，并同步更新服务边界清单。
- **验证门槛：** 新增服务必须有独立入口、构建制品、数据 ownership、依赖清单、contract test 和回滚说明；结构门禁、Go 全量测试、镜像隔离检查和对应服务 smoke 必须通过。
- **本轮进展：** embedded runtime 已直接调用 `internal/platform/runtime` 的 metrics API，删除无生产调用者的 `internal/bootstrap` metrics facade，并将行为 contract test 归档到平台 runtime；指标启停和服务 readiness 语义保持兼容。
- **本轮进展：** embedded 聚合实现、测试与生命周期已从共享 `internal/bootstrap/embedded/` 收敛到 `internal/services/core/bootstrap/embedded/`，由 Core 的唯一 `embedded_compat.go` 作为本地回滚桥接；服务布局门禁拒绝旧共享目录回流，独立服务仍不得依赖该聚合路径。
- **本轮进展：** Delivery Observation RPC 的实现与测试调用已统一归属 `internal/services/gateway/bootstrap`，删除共享 `internal/bootstrap` facade；Realtime caller identity、mTLS transport 和队列 backpressure contract 已由 Gateway-owned 测试覆盖。
- **本轮进展：** Core RPC 的测试 helper 已切换到 `internal/services/core/bootstrap`，共享 `internal/bootstrap.NewCoreRPCServer` facade 已删除；Core capability 的认证、mTLS 和协议 contract 继续由 Core-owned 实现覆盖。
- **本轮进展：** 已新增服务入口索引、服务边界清单和结构门禁检查；本条债务保留，代表代码物理边界尚未全部收敛。
- **验证记录：** 当前分支全量 `CGO_ENABLED=0 go test ./...` 通过，根级目录白名单、服务布局和架构文档门禁通过；仍有调用者的兼容 facade 保留为 embedded 测试与回滚边界，已完成审计的 Message/Sync facade 不再保留。
- **本轮进展：** Gateway Kafka consumer 所需的群组、会话、联系人和已读事件 payload/decoder 已下沉到 `internal/application` 跨服务 contract；Gateway 生产代码不再依赖 Core domain，结构门禁已固定该回流路径，Core 仍负责事件生产与自身 projection。
- **本轮进展：** Agent infrastructure contract tests 已切换到 Agent-owned application constructors，Agent 服务结构门禁现在阻止对聚合 `internal/app` 的直接依赖；Core 兼容层和其他共享基础设施仍按后续切片继续收敛。
- **本轮进展：** 为 `internal/application` 增加 contract ownership README 与架构测试，禁止其生产契约文件依赖服务实现、旧数据层和运维目录；该边界为后续 SQLC 多语言协议迁移提供回流防护，不改变现有 Go contract。
- **本轮进展：** 将 embedded 聚合依赖收紧为 Core `embedded_compat.go` 唯一兼容/回滚桥接，新增服务布局门禁和 Core bootstrap 回归测试；独立服务与其他 service-owned 代码不得直接引用 `internal/bootstrap/embedded`，embedded 运行时仍保留以支持回滚。
- **本轮进展：** Core Auth TokenService 已通过 `internal/platform/cache` 访问 Redis 撤销状态，移除 Core domain 对 `internal/store` 的直接依赖；Redis 缺失时仍保持 fail-closed，其他 Core Redis 使用点继续按后续切片收敛。
- **本轮进展：** Core 文件分片会话已通过 `internal/platform/cache` 执行 Redis raw read、transaction、hash 和 delete，domain 实现移除对 `internal/store` 的直接依赖；上传会话事务与失败回滚语义保持不变，其他 Core Redis 使用点继续按后续切片收敛。
- **本轮进展：** Search application 及其测试已从 `internal/app` 迁入 `internal/services/search/application/`，Search runtime 改用服务专属包；结构门禁已防止旧路径回流，其他服务仍待按同一方式迁移。
- **本轮进展：** Gateway 全部 HTTP handler 及测试已从通用 `internal/handler/http` 迁入 `internal/gateway/http/`，保留认证、错误映射和各 application contract；结构门禁已增加旧目录回流检查。
- **验证备注：** Gateway HTTP 普通测试、完整 Go 门禁、架构文档门禁、Compose 门禁和差异检查通过；本机 `go test -race ./internal/gateway/http` 因 Homebrew Go 运行环境缺少 `libresolv.so.2` 无法启动，未发现代码级 race 结果。
- **本轮进展：** Sync application 装配已从 `internal/app` 迁入 `internal/services/sync/application/`，`MessagingServices` 只持有共享 `SyncApplication` port，独立 Sync runtime 与 embedded 兼容路径共用服务专属 factory；结构门禁已增加 Sync application 路径检查。
- **本轮进展：** Search 入口装配已收敛到 `internal/services/search/bootstrap/`，`cmd/services/search` 不再直接依赖共享 `internal/bootstrap`；当前底层 Search runtime 仍通过兼容 facade 调用共享 gRPC、metrics 和 readiness 设施，后续继续完成实现迁移。
- **本轮进展：** Message 入口装配已收敛到 `internal/services/message/bootstrap/`，`cmd/services/message` 不再直接依赖共享 `internal/bootstrap`；数据库权限探针已迁入 `internal/services/message/infrastructure/mysql/` 并由独立 runtime 直接调用，embedded 仅保留兼容转发，其他共享基础设施继续按回滚切片收敛。
- **本轮进展：** Sync 入口装配已收敛到 `internal/services/sync/bootstrap/`，`cmd/services/sync` 不再直接依赖共享 `internal/bootstrap`；当前底层 Sync runtime 仍通过兼容 facade 调用共享 Kafka projector、Cassandra hydration、数据库、gRPC、metrics 和 readiness 设施，后续继续完成实现迁移。
- **本轮进展：** Gateway 入口装配已收敛到 `internal/services/gateway/bootstrap/`，`cmd/services/gateway` 不再直接依赖共享 `internal/bootstrap`；Gateway Kafka handler、注册器和 authority factory 已归属服务 infrastructure，runtime 直接使用服务实现，剩余共享兼容边界集中在平台生命周期能力。
- **本轮进展：** Core 入口装配已收敛到 `internal/services/core/bootstrap/`，`cmd/services/core` 不再直接依赖共享 `internal/bootstrap`；入口显式区分独立 Core 与 embedded 回滚路径，底层 Core runtime 仍通过兼容 facade 调用共享 RPC、Kafka、storage、metrics 和 readiness 设施，后续继续完成实现迁移。
- **本轮进展：** Search Indexer 入口装配已收敛到 `internal/services/search-indexer/bootstrap/`，`cmd/services/search-indexer` 不再直接依赖共享 `internal/bootstrap`；底层 Search Indexer runtime 仍通过兼容 facade 调用共享 Kafka、Elasticsearch、metrics 和 readiness 设施，后续继续完成实现迁移。
- **本轮进展：** 跨服务 metrics 生命周期已下沉到 `internal/platform/runtime/`，所有长期 runtime 已切换新平台 API，`internal/bootstrap/metrics.go` 仅保留兼容 helper；依赖 readiness 探针和内部 RPC server 仍待按服务边界继续拆分。
- **本轮进展：** Message application 装配已从 `internal/app` 迁入 `internal/services/message/application/`，保留包含 Agent command、Outbox 和持久化扩展方法的 local adapter；`internal/app` 仅负责 Composition Root 参数转换，结构门禁已增加 Message application 路径检查。
- **本轮进展：** Core capability 实现已从 `internal/app` 迁入 `internal/services/core/application/`，factory 只接收实际使用的最小 store 接口；`internal/app` 保留兼容构造入口，结构门禁已阻止旧具体实现回流。
- **本轮进展：** Core Conversation application 的装配已迁入 `internal/services/core/application/`，`MessagingServices` 改持有服务专属 local adapter；底层 `internal/service` 实现暂保留，后续继续按 application port 拆分。
- **本轮进展：** Core User application 装配已迁入 `internal/services/core/application/`，Server 通过服务专属 factory 注入 User/File store 与对象存储；底层 `internal/service` 实现暂保留，HTTP contract 和回滚入口未改变。
- **本轮进展：** Core Contact application 装配已迁入 `internal/services/core/application/`，Server 通过服务专属 factory 注入 Contact/User store、事件、通知和系统消息；底层 `internal/service` 实现暂保留，联系人 HTTP contract 和回滚入口未改变。
- **本轮进展：** Core Group application 装配已迁入 `internal/services/core/application/`，Server 通过服务专属 factory 注入 Group/User store、事件、热群、文件、对象存储和系统消息；底层 `internal/service` 实现暂保留，群组 HTTP contract 和回滚入口未改变。
- **本轮进展：** Core File application 装配已迁入 `internal/services/core/application/`，Messaging composition root 通过服务专属 factory 注入 File metadata、Message store 和对象存储；底层 `internal/service` 实现暂保留，文件 HTTP contract 和回滚入口未改变。
- **本轮进展：** Core Auth/Admin/Session application 装配已迁入 `internal/services/core/application/`，Server 继续使用原 HTTP contract，同时将认证、后台统计和设备会话的 legacy Service 构造收敛到 Core adapter；底层实现暂保留，回滚入口未改变。
- **本轮进展：** Core Group domain 实现及测试已迁入 `internal/services/core/domain/group/`；Group HTTP/DTO、Gateway Kafka 和 embedded 解码调用已直接依赖 Core-owned contract，删除无调用者的 `internal/compat/service/group_compat.go`，HTTP/Kafka contract 保持兼容。
- **本轮进展：** Core File domain、Redis 分片会话实现及测试已迁入 `internal/services/core/domain/file/`；文件 HTTP/DTO 调用已直接依赖 Core-owned contract，删除无调用者的 `internal/compat/service/file_compat.go`，文件 HTTP contract 保持兼容。
- **本轮进展：** Message event payload、mutation、Search/Sync projection、HTTP/WS 错误和 Gateway/Core/embedded Kafka 调用已直接依赖 Message-owned contract，删除无调用者的 Message service facade；兼容层仅保留跨版本 domain-event decoder 辅助。
- **本轮进展：** Core Auth domain 及测试已迁入 `internal/services/core/domain/auth/`；Auth HTTP/DTO、Middleware 和测试调用已迁移到 Core-owned Auth contract，删除无调用者的 `internal/compat/service/auth_compat.go`，认证与 MCP grant HTTP contract 保持兼容。
- **本轮进展：** Core Admin domain 及测试已迁入 `internal/services/core/domain/admin/`；HTTP/DTO 与测试调用已迁移到 Core-owned Admin contract，删除无调用者的 `internal/compat/service/admin_compat.go`，User 权限错误继续共享同一错误值。
- **本轮进展：** Core Session domain 及测试已迁入 `internal/services/core/domain/session/`；设备会话 DTO、HTTP、Core Session Kick 和 Gateway Kafka 测试已迁移到 Core-owned contract，删除无调用者的 `internal/compat/service/session_compat.go`，设备会话 HTTP 与事件 contract 保持兼容。
- **本轮进展：** Core User domain 及测试已迁入 `internal/services/core/domain/user/`；User HTTP/DTO 及测试调用已迁移到 Core-owned User contract，删除无调用者的 `internal/compat/service/user_compat.go`，头像对象存储与用户管理 HTTP contract 保持兼容。
- **本轮进展：** Core Contact domain 及测试已迁入 `internal/services/core/domain/contact/`；联系人 DTO、HTTP、Gateway Kafka 测试和错误契约已迁移到 Core-owned Contact contract，删除无调用者的 `internal/compat/service/contact_compat.go`，联系人 HTTP 与事件 contract 保持兼容。
- **本轮进展：** Core Conversation domain 及测试已迁入 `internal/services/core/domain/conversation/`；Conversation HTTP/DTO、Core 通知器、embedded 装配和 Gateway 事件测试已迁移到 Core-owned contract，删除无调用者的 `internal/compat/service/conversation_compat.go`，Conversation、已读回执和投影观察 contract 保持兼容。
- **本轮进展：** Sync domain 及测试已迁入 `internal/services/sync/domain/`；全仓调用审计确认 `internal/compat/service/sync_compat.go` 无生产或测试调用者，已删除该兼容入口并收紧服务布局门禁，设备 Cursor、群组 checkpoint 和增量同步 contract 保持兼容。
- **本轮进展：** Message event contract 与 Sync projection 实现及测试已迁入 `internal/services/message/domain/`；`internal/compat/service/message_event_compat.go` 仅保留类型、错误和函数兼容入口，事件、Search mutation 和 Inbox locator contract 保持兼容，旧实现路径由结构门禁阻止回流。
- **本轮进展：** Message 核心 domain 实现及测试已迁入 `internal/services/message/domain/`；`internal/compat/service/message_event_compat.go` 继续提供兼容类型、错误和构造入口，消息发送、历史查询、幂等、Outbox、Seq、文件授权和热群策略 contract 保持兼容，旧核心实现路径由结构门禁阻止回流。
- **本轮进展：** Message 专属 sqlc MySQL repository 及 contract tests 已迁入 `internal/services/message/infrastructure/mysql/`；`internal/data/mysql/repository/message_compat.go` 仅保留兼容别名和构造入口，生成代码、事务 Store 和消息表 ownership 保持稳定，旧共享 repository 路径由结构门禁阻止回流。
- **本轮进展：** Message 独立 runtime 已直接装配 `internal/services/message/infrastructure/mysql` 与 Message application，移除对 `internal/app` 聚合 Composition Root 的依赖；服务布局门禁已固定该启动边界，embedded 聚合入口继续保留作为回滚路径。
- **本轮进展：** Sync repository、hydrator、projection 和 process composition 已迁入 `internal/services/sync/infrastructure/mysql/`，独立 Sync runtime 已移除对 `internal/app` 聚合装配的依赖；旧 repository 兼容入口和 embedded 回滚路径保留。
- **本轮进展：** Inbox ownership 校验已要求 Message projector 模式同时启用 Sync projector 与 Kafka，并增加缺失 Sync projector 的 fail-closed 测试；atomic 模式和原授权回滚路径保留。
- **本轮进展：** Core repository composition 已抽出 `ProcessRepositories` 并迁入 `internal/services/core/infrastructure/mysql/`；独立 Core runtime 直接加载该集合，聚合入口仅作为 embedded 回滚路径。
- **本轮进展：** 聚合 `Repositories` 已显式持有 Core、Message、Sync、Agent 四类 process composition，embedded 入口开始复用服务所有权分组；独立启动链仍待切换到这些分组，当前聚合入口保留为回滚路径。
- **本轮进展：** Core remote 入口已切换到 `InitializeCoreService`，只装配 Core-owned `ProcessRepositories`、Core projection Kafka consumer 和 Core Capability RPC；embedded 模式保留原聚合入口作为本地兼容路径。Core/Message/Agent 的数据库账号和全量运行时切换仍按后续门禁推进。
- **本轮进展：** Agent repository composition 已抽出 `AgentProcessRepositories` 并由聚合 `NewRepositories` 复用，明确 Agent-owned SQL repository 集合；Core 兼容 RPC 仍共享同一进程装配，TS Runtime 完全接管前需继续拆分启动链。
- **本轮进展：** Agent 专属 sqlc MySQL repository 及契约测试已迁入 `internal/services/agent/infrastructure/mysql/`；共享 `internal/data/mysql/repository/agent_compat.go` 仅保留兼容别名和构造入口，服务布局门禁已阻止实现文件回流。
- **本轮进展：** Core 专属 sqlc MySQL repository 及契约测试已迁入 `internal/services/core/infrastructure/mysql/`；共享 `internal/data/mysql/repository/core_compat.go` 仅保留兼容别名和构造入口，服务布局门禁已阻止实现文件回流。
- **本轮进展：** Search Index SQLC repository 及契约测试已迁入 `internal/services/search/infrastructure/mysql/`；共享 `internal/data/mysql/repository/search_index_compat.go` 仅保留兼容别名和构造入口，服务布局门禁已阻止实现文件回流。
- **本轮进展：** 清理共享 repository 中已无调用者的事务别名和 UUID 辅助文件，并将共享目录约束收紧为兼容入口集合；Core、Agent、Search、Message 和 Sync 的仓储实现均由各自服务 infrastructure 持有。
- **本轮进展：** Compose 编排已从根目录收纳至 `deploy/compose/`，仅保留默认 `docker-compose.yml` 作为本地入口；所有编排引用和 Compose 静态门禁已同步，TS Agent Runtime 保留用于 Go 工具链扫描隔离的独立 module 边界。`internal/app` 已退出外部生产依赖，后续重点转为 embedded/兼容测试退休，以及 `internal/application`、`internal/bootstrap` 和其他兼容层的最终物理边界收敛。
- **本轮进展：** 2026-08-29 修正 ownership smoke 的旧 repository 测试路径，并增加 selector 命中 fail-closed；真实 MySQL atomic/projector/rollback smoke 与三节点 Kafka Sync projector dual-run smoke 均通过。生产级候选镜像切换、Kafka ownership 深度核对和可执行回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-29 使用隔离候选微服务 Compose 完成 Gateway 端到端消息验证，覆盖服务健康、注册/登录、好友关系、WebSocket、Message/Outbox/Inbox 幂等和 Seq Timeline 读取；生产 Kafka ownership 切换与可执行回滚 receipt 仍待完成。
- **本轮进展：** Core repository composition 与 User/Group/Contact cache adapter 已迁入 `internal/services/core/infrastructure/mysql/`；独立 Core Runtime 直接依赖 Core-owned composition，`internal/app` 仅保留 embedded 兼容别名，结构门禁阻止实现回流。
- **本轮进展：** Agent repository composition 已迁入 `internal/services/agent/infrastructure/mysql/`；`internal/app` 仅保留 embedded 兼容别名，聚合入口改用 Agent-owned composition，结构门禁阻止 Agent composition 回流。
- **本轮进展：** Sync repository composition 已迁入 `internal/services/sync/infrastructure/mysql/`；`internal/app` 仅保留 embedded 兼容别名，独立与聚合启动均使用 Sync-owned composition，结构门禁阻止 Sync composition 回流。
- **本轮进展：** 2026-08-29 收紧服务布局门禁：`internal/app`、`internal/store` 和 `internal/data/mysql` 仅允许登记的兼容 adapter、SQLC 别名、README 与兼容测试；后续调用审计已完成 `internal/store` 与 `internal/data/mysql` 目录退役，门禁继续阻止旧目录回流。
- **验证记录：** 2026-08-29 负向测试使用未跟踪的未登记文件验证门禁拒绝路径，随后删除夹具并重新通过正向门禁；检查范围覆盖已跟踪和未忽略未跟踪文件。

### AD-048：Go 微服务默认部署仍使用共享镜像

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** Go 服务镜像、Compose 发布、回滚和供应链 provenance
- **现状：** 服务入口已拆分，微服务 Compose 默认引用各自只包含 `/app/service` 的 `DIPOLE_*_IMAGE`；legacy Compose 继续保留共享镜像。构建脚本覆盖 migrate、六个长期服务和可选 Timeline repair worker，并统一写入 revision/dirty provenance。
- **风险：** 候选镜像尚未完成生产级回滚切换演练；若逐服务标签、Kafka consumer ownership 或配置发布不一致，可能造成服务无法启动或重复消费。
- **下一步：** 在维护窗口执行候选镜像切换，记录 Kafka consumer ownership、历史读取、故障停止和恢复后的可执行回切 receipt；证据完整后再评估默认生产发布。
- **验证门槛：** `scripts/check-compose.sh`、`scripts/check-service-layout.sh`、Go backend 构建、逐服务镜像内容隔离检查和 `scripts/smoke-microservice-isolated-images.sh` 的独立核心栈 health/readiness 演练必须通过；Search profile 的独立运行时 smoke 也必须通过；legacy Compose 共享镜像和 authority 行为保持可回滚。
- **本轮进展：** 2026-08-29 通过 `SMOKE_SEARCH_PROFILE=1` 完成独立 Search 运行时 smoke，Elasticsearch、Search Indexer、Search 及核心依赖链均通过 health/readiness；消息写入、Kafka ownership 和生产回滚切换仍未完成。
- **本轮进展：** 2026-08-29 `smoke-sync-write-ownership.sh` 与 `smoke-sync-projector.sh` 已通过，补齐真实 MySQL atomic/projector ownership、三节点 Kafka backlog/实时事件、retry/DLQ 和 projector 收敛证据；候选镜像经 Gateway 的端到端消息发送及生产回滚仍待完成。
- **本轮进展：** 2026-08-30 `smoke-sync-write-ownership.sh` 新增 `SMOKE_REPORT_FILE` receipt，绑定 revision/dirty、projector 写入与 atomic 回滚模式、退出状态及隔离容器清理结果；Remote GPU 在 GPU 任务启动期间完成真实 MySQL 最小权限验证，GPU 任务未被触碰，receipt 为 `0600`。该证据仍限于开发候选，生产 Kafka ownership 切换与共享环境回滚 receipt 继续待完成。
- **本轮进展：** 2026-08-29 使用 `SMOKE_MESSAGE_FLOW=1` 完成候选镜像端到端消息 smoke：注册/登录、好友关系、WebSocket 发送，以及 Message/Outbox/目标用户 Inbox 持久化均通过；重复请求、Kafka authority 和生产回滚仍待完成。
- **本轮进展：** 2026-08-29 扩展候选消息 smoke，按 `before_seq=0` 和 `after_seq=0` 通过 Gateway 读取同一消息，并校验返回持久化 `message_seq`；历史读取证据已覆盖，Kafka authority 和生产回滚仍待完成。
- **本轮进展：** 2026-08-29 在已提交 revision `fe84b7b` 上重建七个候选镜像，逐项核对同一 revision、`io.dipole.source.dirty=false` 和服务二进制标签；独立消息流程再次通过，候选供应链与 Timeline 读取证据已闭合，Kafka authority 和生产回滚仍待完成。
- **本轮进展：** 2026-08-29 在 `SMOKE_MESSAGE_FLOW=1` 中复用同一 `client_message_id` 重发消息，数据库核对确认 Message、Outbox 和 Inbox 各保持单条，候选 Message Service 幂等路径通过；Kafka authority 深度核对和生产回滚仍待完成。
- **本轮进展：** 2026-08-31 新增 `SMOKE_MESSAGE_RESTART_SERVICE` 持久化后恢复 receipt：首次 WebSocket 消息已写入后才重启 Core、Gateway、Message 或 Sync，随后重放相同 client ID 并重新核对 Message、Outbox、Inbox 三类基数。该演练尚待 Remote GPU 实跑归档；Kafka consumer 中断、broker 故障与 in-flight commit 仍属于独立 P0 故障矩阵。
- **本轮进展：** 2026-08-30 在逐服务候选镜像拓扑中完成真实消息流程 smoke：注册/登录、好友关系、WebSocket 发送、Message/Outbox/Inbox 幂等，以及 `before_seq` 历史和 `after_seq` 增量读取均通过；receipt 归档于 `benchmarks/ad048-message-flow-2026-08-30/receipt.json`。共享环境 Kafka ownership、生产切换和可执行回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-30 修复 Inbox projector 隔离 smoke 的失败拓扑证书保留逻辑，并在 Message overlay 显式注入 `DIPOLE_SYNC_PROJECTOR_ENABLED=true` 以满足 fail-closed 启动校验；projector ownership 模式下的 readiness、注册/登录、好友关系、WebSocket 发送、Message/Outbox/Inbox 幂等及 `before_seq`/`after_seq` 读取均通过，receipt 归档于 `benchmarks/ad048-projector-message-flow-2026-08-30/receipt.json`。共享环境 Kafka ownership、生产切换和可执行回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-30 基于干净 revision `81730409` 重建逐服务 Go 镜像，在 `SMOKE_SEARCH_PROFILE=1` 隔离拓扑中验证 Core、Message、Sync、Gateway、Search 和 Search Indexer 均 healthy/ready；receipt 归档于 `benchmarks/ad048-independent-images-2026-08-30/receipt.json`。共享环境 Kafka ownership、生产切换和可执行回滚 receipt 仍待完成。
- **脚本维护：** 2026-08-30 发现 `smoke-sync-write-ownership.sh` 的三个数据库边界 selector 随服务目录迁移失效，已改为 `internal/services/sync/bootstrap` 与 `internal/services/message/infrastructure/mysql`；修复后的 Remote GPU 真实演练待重跑确认。
- **本轮进展：** 收紧 Compose 静态门禁，默认拓扑现在同时锁定 Agent 独立镜像及 `services/agent-runtime` 构建上下文，Timeline repair profile 锁定独立镜像和 `/app/service` 入口；共享环境候选切换与回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-29 以 `ISOLATED_IMAGES=1` 运行依赖 readiness smoke，Kafka assignment 建立、Search/Indexer 候选服务、Elasticsearch 停止降级与恢复、核心容器身份稳定性均通过；生产切换与回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-29 基础微服务 Compose 切换为逐服务镜像与统一 `/app/service` 入口，补充 repair worker 镜像构建；基础核心 smoke、Search profile 消息 smoke 和 repair profile v50 恢复/幂等 smoke 均通过。共享环境 Kafka ownership、发布切换与可执行回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-29 使用 `COMPOSE_PROJECT_NAME` 隔离项目运行 `scripts/smoke-microservices.sh`，Core、Message、Sync、Gateway、Agent 及 MySQL、Redis、Kafka、MinIO 均完成冷启动并达到 healthy；readiness、metrics、TLS 1.3 mTLS、Core HTTP 代理和 remote WS ownership 均通过，脚本自动清理拓扑。共享环境 Kafka ownership、生产切换和可执行回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-29 将 Agent 审批、审批授权和任务控制 application 实现迁入 `internal/services/agent/application/`；embedded `internal/app` 保留兼容别名与构造转发，Bootstrap 和 Agent SQLC 契约测试已直接依赖服务专属包；新增结构门禁阻止这三类实现回流。其余 Agent application、聚合 Composition Root、独立数据库账号和服务自治仍待继续收敛。
- **本轮进展：** 2026-08-29 继续将 Agent Definition Catalog、Memory Candidate Promotion 和 Task Workflow Projection application 实现迁入同一服务边界；embedded 兼容转发保持，结构门禁已扩展覆盖六类已迁移实现。其余 Agent application、聚合 Composition Root、独立数据库账号和服务自治仍待继续收敛。
- **本轮进展：** 2026-08-29 继续将 MCP readiness、MCP tool round、tool invocation audit、Runtime promotion evidence 和 Workflow repair audit application 实现迁入 Agent 服务边界；Bootstrap 与 SQLC 契约测试已直接使用服务包，结构门禁覆盖十一类已迁移实现。Agent capability/command、execution policy、Memory owner、Subscription、Artifact、Workflow repair executor 及聚合 Composition Root仍待继续收敛。
- **本轮进展：** 2026-08-29 将 Agent Artifact 和 Memory Owner application 实现迁入 Agent 服务边界；Bootstrap 已直接使用服务包，Artifact policy 依赖改为显式接口，结构门禁覆盖十三类已迁移实现。Agent capability/command、execution policy、Subscription、Workflow repair executor 及聚合 Composition Root仍待继续收敛。
- **本轮进展：** 2026-08-29 将 Agent Event Subscription application 实现迁入 Agent 服务边界；Bootstrap 已直接使用服务包，结构门禁覆盖十四类已迁移实现。Agent capability/command、execution policy、Workflow repair executor 及聚合 Composition Root仍待继续收敛。
- **本轮进展：** 2026-08-29 将 Agent Capability 与 Command application 实现迁入 Agent 服务边界；Bootstrap 已直接使用服务包，消息与会话依赖显式化，结构门禁覆盖十六类已迁移实现。Execution Policy、Workflow repair executor 及聚合 Composition Root仍待继续收敛。
- **本轮进展：** 2026-08-29 将 Workflow Repair Prepare 和 Executor application 实现迁入 Agent 服务边界；兼容入口保留，结构门禁覆盖十八类已迁移实现。Execution Policy、Agent Runtime 独立 Composition Root 及聚合 Composition Root仍待继续收敛。
- **本轮进展：** 2026-08-29 将 Agent Execution Policy、Invocation Resolver 和 Run Admission 实现迁入 Agent 服务边界；兼容入口保留并增加 deterministic clock 构造，结构门禁覆盖十九类已迁移实现。Agent Runtime 独立 Composition Root、剩余轻量兼容实现和 TS Runtime 正式接管仍待继续收敛。
- **本轮进展：** 2026-08-29 将 Agent MCP tool terminal、Memory、Message command execution、Runtime promotion control 和 Runtime promotion application 实现迁入 Agent 服务边界；Bootstrap 已直接使用服务包，Memory task reader 与时间依赖改为显式服务契约，结构门禁覆盖二十四类已迁移实现。Agent Runtime 独立 Composition Root、聚合兼容装配收敛和 TS Runtime 正式接管仍待继续推进。
- **本轮进展：** 2026-08-29 Agent Capability RPC 的 Admission、Complete、Finish 增加显式 `runtime_id + mode` 绑定和 active candidate version，TS client 默认 shadow，Go Core active admission 继续要求 promotion authorizer；旧调用按 shadow 兼容。active Activity、写能力接线、独立 Composition Root 和生产切换证据仍待完成。

### AD-049：Core 与 Message 远程初始化存在双向依赖

- **优先级：** P1
- **状态：** 处理中
- **2026-08-30 复跑：** 在含超时探针的当前基线再次完成 readiness smoke；完整微服务拓扑的冷启动、Gateway assignment、Elasticsearch 故障降级/恢复和核心服务容器不重启均通过，临时资源已清理。共享环境发布和可执行回滚 receipt 仍待完成。
- **2026-08-30 验证：** readiness smoke 通过 Core/Message/Sync/Gateway 冷启动、Gateway assignment、Elasticsearch 停止后的 Search/Indexer readiness 降级与恢复，并确认核心服务容器未因依赖退化重启；同时为探针增加 10 秒 `docker compose exec` 超时，避免异常时无限等待。共享环境发布、Kafka ownership 深度核对和可执行回滚 receipt 仍待完成。
- **发现日期：** 2026-08-29
- **影响范围：** Core/Message 微服务冷启动、Compose 健康依赖、消息表写入 ownership
- **现状：** Core 的 system-message 写入已通过受限 Message RPC 访问，Message 的 Core Capability 改为惰性 RPC adapter；两侧启动阶段不再强制互相拨号，失败连接不缓存，后续请求和就绪探针会触发有界重试。微服务 Compose 默认使用远程 transport，embedded/local 仍保留回滚路径。
- **风险：** 当前已消除初始化阶段的双向硬依赖，但共享环境仍需验证 Core/Message/Gateway 的完整冷启动顺序、RPC mTLS、Kafka consumer 唯一性和服务级数据库权限；消息写路径的生产切换证据尚未闭合。
- **下一步：** 在隔离 Compose 与共享环境记录冷启动、依赖 readiness、端到端消息和 Local 回切 evidence，再继续收紧 Core/Message 数据库账号与服务启动权限。
- **验证门槛：** 默认微服务 Compose 冷启动中 Core、Message、Sync、Gateway 均 healthy；Core 专用 transport 配置单测、远程 Message mTLS contract、端到端消息 smoke 和 Local 回切 smoke 均通过。
- **本轮进展：** 远程模式下 Core 的本地启动兼容层不再注册 Message persistence consumer，也不初始化消息 topic；消息写入与 topic ownership 继续收敛到 Message Service，新增 ownership 单测并由 Compose 配置门禁固定全局 transport 为 gRPC。
- **本轮进展：** Gateway 已直接注册消息历史与 Sync HTTP 路由并通过受认证的 Message/Sync RPC 访问；Core 仅在 embedded 模式注册对应 HTTP/WS 数据路由，remote 模式的公共消息与同步入口已收口到 Gateway。Core 内部系统消息已通过受限 Message RPC 接入，连接建立采用惰性 adapter。
- **本轮进展：** Message Core Capability 改为惰性连接：构造时不拨号，首次调用或依赖就绪探针按当前 RPC 认证配置建立连接；连接失败不进入缓存，Core 恢复后可重试，新增冷启动/重试/关闭回归测试。完整隔离 Compose 和共享环境证据仍待补齐。
- **本轮进展：** Compose 门禁已固定默认微服务拓扑中 Core 与 Message 不得互相 `depends_on`，且默认 Core Message transport 必须为 gRPC；`cassandra-primary` 的 embedded/local 回滚覆盖层仍单独保留并验证。
- **本轮验证：** 2026-08-30 在最新 `master` 重新执行 `scripts/smoke-microservices.sh`，Core、Message、Sync、Gateway、Agent 与 MySQL、Redis、Kafka、MinIO 均完成隔离冷启动；readiness、metrics、Core proxy、mTLS、远程 WS ownership 和 Agent EventLedger/Task/Run 幂等通过。共享环境发布、Kafka ownership 深度核对和可执行回滚 receipt 仍待完成。
- **本轮进展：** 2026-08-29 隔离微服务 Compose 已验证 Core/Message/Sync/Gateway 冷启动、依赖 readiness、RPC mTLS、Core 代理和远程 WS ownership；当前证据覆盖开发候选拓扑，Local 回切与共享环境发布窗口演练仍待完成。
- **本轮进展：** 运维代码、服务集成测试和平台测试已停止引用 `internal/data/mysql/repository` 历史兼容别名，统一使用各服务自有 SQLC repository；后续调用审计已完成该历史目录退役，结构门禁阻止新的运行时代码回流。
- **本轮进展：** 为 `internal/app`、`internal/data/mysql`、`internal/data/mysql/repository` 和 `internal/store` 增加目录级 ownership/迁移说明，并由服务布局门禁检查；后续调用审计已完成 `internal/store` 与 `internal/data/mysql` 目录退役。
- **本轮进展：** 删除已无调用者的共享 repository contract helper；各服务的 MySQL contract database helper 已在自身 infrastructure 测试边界内维护，历史 repository 包进一步收敛为别名与构造转发。
- **本轮进展：** 校正平台演进计划中的 Message transport 叙述，明确 `local` 是 M3 历史兼容默认值，当前微服务 Compose 默认使用受认证 `grpc`，embedded/local 仅承担回切职责。

### AD-047：受限实验主机的 Elasticsearch 磁盘水位需要隔离约束

- **优先级：** P2
- **状态：** 接受风险
- **发现日期：** 2026-08-29
- **影响范围：** `deploy/compose/docker-compose.storage-lab.yml`、Elasticsearch storage-lab 健康检查
- **现状：** 受限实验主机磁盘使用率可能超过 Elasticsearch 默认 high watermark，导致单节点集群保持 red 并拒绝索引写入。storage-lab 使用显式 lab-only 磁盘水位参数（low/high/flood-stage 为 `90%/99%/99.5%`），健康检查要求 yellow/green；该参数未进入生产 Compose 或应用配置。
- **风险：** 若实验主机继续逼近 flood-stage，隔离 smoke 仍会失败；放宽实验水位不能替代生产磁盘容量、监控和清理策略。
- **下一步：** 保持实验栈与生产配置分离，定期清理 Docker volume 并在共享环境补充磁盘告警；生产部署遵循 Elasticsearch 官方水位和容量门禁。
- **验证：** 2026-08-29 storage-lab smoke 通过 Cassandra 5.0.9、Elasticsearch 9.5.2 和 MinIO CRUD，且未产生生产流量。
- **本轮进展：** Cassandra hydration/read-routing smoke 改用动态宿主机端口并反查映射，消除并行实验之间的固定端口竞争；默认仍只运行隔离实验，不改变生产端口或主读开关。
- **本轮进展：** 2026-08-29 现场验证确认 storage-lab 失败由宿主机 `95.9%` 磁盘使用率触发 Elasticsearch `high=95%` 分配保护；仅在 lab Compose 将 low/high/flood-stage 调整为 `90%/99%/99.5%`，并为 API 探针增加有界重试。修复后 Cassandra 5.0.9、Elasticsearch 9.5.2 和 MinIO 隔离 CRUD smoke 通过，生产配置保持不变。

### AD-046：Timeline repair worker 尚未纳入默认服务拓扑

- **优先级：** P1
- **状态：** 处理中
- **2026-08-30 验证：** `check-agent-timeline-repair-alerts.sh` 通过 2 条 Prometheus 规则及 promtool 测试；Compose 隔离 smoke 通过 migration v50、最小权限、worker readiness、pending intent 恢复和 event UUID 幂等重放，临时资源已清理。该证据仍不覆盖共享环境 operator 灰度、告警抓取、回切演练和默认生产开关。
- **发现日期：** 2026-08-29
- **影响范围：** Timeline repair、MySQL 权限、Compose 发布与运行时告警
- **现状：** 已提供独立 `dipole-agent-task-timeline-repair` 镜像二进制、专用最小权限账号和默认关闭的 `agent-timeline-repair` Compose profile；隔离 MySQL 进程级 smoke 已验证 claim/replay、completed 收敛和事件幂等，并新增短窗口失败/持续 retry 的 Prometheus 告警规则与 promtool 测试，worker 仍需 operator 显式启用。
- **风险：** 未完成共享环境 operator 灰度、指标抓取和告警演练前，Timeline repair intent 仍可能停留在 pending/retry，不能宣称生产自动修复闭环。
- **下一步：** 在隔离环境启用 profile，验证 readiness、repair counter、重启恢复与回滚；证据完整后再评估默认拓扑或告警策略。
- **运维约束：** 启用、暂停和回切步骤已收敛到 `docs/agent/AGENT-TIMELINE-REPAIR-OPERATIONS.md`；当前仍要求显式 profile、完整窗口和原始指标快照，未满足时保持默认关闭。
- **本轮进展：** repair binary 增加 `-once` 有界执行模式，已由隔离 smoke 真实验证单批次完成；共享环境仍需 operator 灰度和告警演练。
- **本轮进展：** Compose repair 权限初始化已收敛为同一密码变量，覆盖值会在授权 SQL 后显式更新，危险 SQL 字符 fail closed；仍需共享环境轮换和回滚演练。
- **本轮进展：** 新增 Compose profile 级隔离 smoke，先断言 v49 migration/Timeline 表，再验证专用权限、worker `readyz`、持续 replay 和 event UUID 幂等；演练发现 MySQL `Asia/Shanghai` 与 Go UTC 的 DATETIME 比较偏移，已将 Compose MySQL 固定为 UTC，并改用同步 `compose run --rm` 执行一次性 migration。共享环境 operator 灰度、指标抓取和轮换/回滚演练仍待完成。
- **本轮进展：** Compose smoke 进一步在 worker 启动前写入 pending intent，确认 repair profile 启用后能恢复积压并保持单事件收敛；同时将全局和会话时区 `+00:00/+00:00` 纳入部署前置断言，防止 lease/retry 时间基准回归。共享环境 operator 灰度、指标抓取和轮换/回滚演练仍待完成。
- **本轮进展：** 新增 `agent-timeline-repair-rollout` v1 evidence/policy/report 契约与只读 CLI，按窗口、样本、错误比例、readiness、operator、告警和回滚演练输出低敏 `eligible|blocked`；CLI 不改变 worker 状态，真实共享环境采集与 operator 决策仍待完成。
- **本轮进展：** 2026-08-29 将 repair profile 的部署前置基线统一到 v50；旧本地共享镜像按 v27 运行时被 preflight 正确拒绝，使用当前源码构建候选镜像后通过 v50、UTC、最小权限、worker readiness、pending intent 恢复和事件幂等 smoke。共享环境 operator 灰度、指标抓取和轮换/回滚演练仍待完成。
- **本轮进展：** 2026-08-29 复跑默认镜像隔离 Compose smoke，确认 v50 migration、UTC、专用权限、worker readiness、pending intent 恢复和 event UUID 幂等均通过；临时栈已自动清理，证据不改变默认关闭状态，共享环境 operator 灰度、指标抓取和轮换/回滚仍待完成。
- **本轮进展：** 2026-08-31 Compose smoke 动态推导 migration 版本和文件数，并将正常退出的 `mysql-permissions` 作为一次性初始化容器轮询。Remote GPU 使用 MySQL migration v53 验证 UTC、专用权限、worker readiness、pending intent 恢复和事件 UUID 幂等重放；随机 Compose 项目、卷和临时工作树均自动清理。该隔离证据不改变默认关闭状态，共享环境 operator 灰度、指标抓取和轮换/回滚演练仍待完成。

### AD-045：Agent Task Timeline 缺少完整运行时闭环

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** Agent Task UI、Core/Gateway 只读 API、Run/Step/Tool/Artifact 审计
- **现状：** Task、Run、Shadow Step、Model Call、Tool Invocation、Approval 和 Artifact 已分别持久化；Gateway 当前只提供权威 Task 当前状态、输入和审批控制。已建立 `contracts/agent-task-timeline/v1/`，规定 Core principal 复核、稳定 `event_seq`、增量游标和低敏事件 DTO。
- **风险：** 若由 Gateway 直接拼接多张 Agent 表或读取 Temporal 历史，会绕过服务 ownership、产生跨 Run 顺序歧义并泄露 prompt、参数或外部结果；当前前端不能声称展示完整执行历史。
- **本轮进展：** migration v48 新增 append-only `agent_task_timeline_events`，以数据库生成的 `event_seq` 保存低敏 Task/Run/Capability/Approval 元数据，并通过 sqlc 提供 append/list 查询与领域校验。生产事务装配现在让 Task/Run 创建和状态迁移与 Timeline append 一起提交；Core 已提供 owner-scoped list 与仅 `dipole-agent` 可用的 append RPC，并接入生产仓储；Runtime/Gateway 已贯通认证只读代理；前端已在 `VITE_AGENT_TIMELINE_ENABLED` flag 下支持低敏展示和 cursor 分页，失败清空并回退；Tool Invocation begin/finish、Approval request/resolve、Model call begin/finish、Artifact create 已追加确定性、可幂等重放的低敏 Timeline 事件。migration v49 新增 repair ledger，投影失败会以 event UUID 幂等落账，并提供租约 claim、完成和重试状态接口；新增显式 repairer 状态机、独立 `agent-task-timeline-repair` 运维进程和可选低基数 Prometheus 观测。自动生成事件 ID 已收敛为固定 64 位摘要，兼容最大长度 Task/Run UUID；真实 MySQL 故障注入已验证 retry 到 completed 的恢复，Agent 及其余 repository contract 已完成分组验证。进程和指标默认关闭，operator 灰度、完整串行 repository 套件稳定运行、默认生产开关和视觉评审仍未开放。
- **本轮进展：** 独立 Core 启动链现明确装配 Timeline Store，Core mTLS allowlist 允许 `dipole-agent` 调用 owner-scoped `ListAgentTaskTimeline`；Runtime HTTP 将 `bigint` revision/timestamp/sequence 映射为 Web DTO 的安全数字或字符串，并保留每个 event 的 Task ID。Remote GPU 隔离候选完成 `202 -> completed -> Timeline 200`，返回 4 个低敏 Task/Run 事件且序列、时间和 owner binding 通过检查。空会话 read-shadow 不产生 Artifact，Artifact、写 Capability、active authority 和成功率结论继续不扩大。
- **本轮验证：** 2026-08-31 Remote GPU 以 Go `1.27.0` 在随机隔离 Docker network/MySQL 容器执行 `smoke-agent-timeline-repair.sh`，worker 成功重放一个 repair intent 并幂等收敛。脚本现允许通过 `DIPOLE_GO_BIN` 固定经验证工具链；该证据不构成 operator 灰度或默认生产开关验收。
- **本轮进展：** Core 新增受认证 `ReadConversation` RPC，沿用 Task/Run principal 解析、Core 精确会话授权和低敏消息映射；TS Runtime 增加 `conversation.read` Capability 并接入模型可用能力集合，为 Context Compiler 补上会话证据读取边界。完整 Timeline UI 生产开关、repair operator 灰度和视觉评审仍未开放。
- **本轮进展：** `conversation.read` 输入已统一为 canonical conversation key，Runtime 对 direct/group key 做确定性 target 解析并先执行 exact scope 检查，减少多语言 Capability 适配差异；完整上下文检索编排和生产开关仍待完成。
- **本轮进展：** ModelShadowPlanner 已在模型调用前通过该 Capability 读取最多 20 条会话消息，并以 `untrusted` provenance、sequence、full/compact 和统一 evidence 预算编译；读取失败不降级为无证据模型调用。全文检索、排序、生产上下文灰度仍待完成。
- **本轮进展：** 会话 evidence 的 protobuf Timestamp 已采用显式 `seconds` 字符串和 `nanos` 表示，消除 TypeScript bigint JSON 序列化风险；跨语言消息字段完整性仍需继续扩展测试。
- **本轮进展：** Planner 对远程会话 evidence 增加 20 条消息与单条 8 KiB 正文上限，并用 `contentTruncated` 保留低敏截断事实；Core 仍负责最终读取授权，后续继续完善分页/检索语义。
- **本轮进展：** TypeScript `AgentCapabilityRPCClient` 增加 direct/group `conversation.read` 跨语言契约测试，覆盖 canonical target 解析、完整 `ExecutionContext` 类型约束、非法 scope 和响应冲突拒绝；后续仍需完善分页/检索语义与生产灰度。
- **本轮进展：** RPC 客户端在 transport 边界拒绝超过请求上限的消息响应，并对未找到结果同样执行 target 一致性校验；Planner 的 20 条/8 KiB context 防线继续作为第二层预算保护。
- **本轮进展：** Context Compiler capability section 已从运行时 Registry 注入排序稳定的 descriptor 元数据，并只投影允许集合；模型仍无法获得输入 schema、凭据或 authority 字段，后续按 route-specific schema 证据继续扩展。
- **本轮进展：** 两个只读 Capability 已提供低敏输入 Schema 摘要并进入 Context Compiler；Schema 仍由代码拥有且执行侧保留 Zod 最终校验，其他 Capability 和 route-specific tokenizer 继续按门禁扩展。
- **本轮进展：** Registry 已在注册边界校验 Schema 摘要关键字、`properties` 映射和 4 KiB 上限，阻止未知描述字段或异常膨胀进入 Context；后续新增 Capability 仍需补齐 descriptor 与契约测试。
- **本轮进展：** Registry 现在深度冻结注册 descriptor 及嵌套 Schema，形成稳定的 capability authority snapshot；新增 Capability 仍需通过 descriptor、Schema 和权限契约测试。
- **本轮进展：** Context Compiler v2 现在接收 route-aware 的最大输入窗口，按最小候选模型窗口扣除最大输出预算，超出请求在编译入口 fail closed；新增回归测试，v1/旧构造路径保持兼容。
- **本轮进展：** Memory candidate 的公开解析边界新增正文与 compact 摘要凭据模式校验；即使绕过 ObservationWorker 直接提交候选，也会在 Reflection/Ledger 前 fail closed，并由回归测试覆盖两条内容路径。
- **本轮进展：** TypeScript Timeline RPC client 现在在消费前校验每个事件的 Task 绑定、非空 `event_id` 和严格递增 `event_seq`，并通过跨任务、重复和倒序事件测试验证 fail-closed；服务端协议和默认 Timeline 开关保持不变。
- **本轮进展：** Context manifest 已为实际选中的 full/compact fragment 保存 SHA-256，审计可在不落正文的前提下核验重放与上下文漂移；完整生产 evidence 仍待共享环境窗口。
- **建议方向：** 以已验证的 repair contract 为基础补齐 operator 灰度、运行时告警和全套件稳定运行证据，再以共享环境证据开启前端 flag；继续只返回低敏元数据，随后按证据逐步加入 Artifact 引用与 Pencil/视觉回归。
- **处理门槛：** Core/Gateway 契约测试覆盖 foreign Task、游标重复/漂移、跨 Run 事件、事件缺失和字段脱敏；前端默认关闭，未收到 v1 response 时保持当前 Task Query 页面。

### AD-043：Sync Cassandra hydration 缺少共享环境运行时证据闭环

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** Sync Service、Cassandra primary/fallback、Prometheus、灰度与回切门禁
- **现状：** Sync Service 已提供默认关闭的 Cassandra primary 路径和 MySQL 即时回退；离线 evidence evaluator 可消费 hit、fallback、missing、conflict、error 与 p95 聚合。运行时现在按低基数 outcome 暴露 `dipole_sync_hydration_route_total` 与 `dipole_sync_hydration_route_duration_seconds`，并保留旧日志观测。
- **风险：** 当前仍缺少真实客户端窗口、共享 Cassandra/Sync 环境采集、missing/conflict 细分的端到端归因、责任人批准、自动停止门禁与可执行回切演练；collector 只能证明进程内路由结果，不能单独证明生产 eligible。
- **本轮进展：** Prometheus snapshot adapter 现拒绝重复 outcome/family、错误类型、额外标签、未知 outcome 和非单调 histogram，并要求起止快照差分；它仍只提供受校验的低敏输入，不替代共享环境身份、客户端窗口和人工批准。
- **本轮进展：** 修复 `cassandra-primary` Compose override 对仓库根目录 schema/config 的相对挂载错误；隔离 primary smoke 已验证 Cassandra schema init、显式 primary 配置和 Sync readiness。该证据仍不替代共享环境长期窗口、客户端流量、责任人批准和可执行回切。
- **验证记录：** 2026-08-29 `scripts/smoke-cassandra-read-routing.sh` 通过真实隔离 Cassandra、MySQL 和 migration v50，验证 Seq 页面 Cassandra 主读，以及 payload 损坏和缺行按同一 cursor 回退 MySQL；默认生产主读比例和开关保持不变。
- **验证记录：** 2026-08-30 重新执行 `scripts/smoke-cassandra-read-routing.sh`，真实验证 migration v50、Cassandra Seq 页面主读，以及 payload 损坏和缺行按同一 cursor 回退 MySQL；临时 Compose 资源自动清理，生产主读比例、共享环境窗口和责任人批准保持未启用。
- **追加验证：** 2026-08-30 再次执行同一 read-routing smoke，结果保持一致；本次仍仅证明隔离候选路径和即时回退，不提升生产 Cassandra 主读比例。
- **验证记录：** 2026-08-29 `scripts/smoke-sync-cassandra-primary-compose.sh` 通过隔离微服务 Compose：Cassandra schema init、Core/Message/Sync 依赖 readiness、primary hydration 配置和 Sync `/readyz` 均通过，临时拓扑自动清理；共享环境长期观测、责任人批准和生产回切演练仍待完成。
- **验证记录：** 2026-08-30 重新执行 `scripts/smoke-sync-cassandra-primary-compose.sh`，真实验证 Cassandra schema init、Core/Message/Sync 依赖 readiness、primary hydration 配置和 Sync `/readyz`；临时拓扑自动清理，生产 Cassandra 主读、共享环境长期观测、责任人批准和生产回切演练仍待完成。
- **本轮进展：** Cassandra read-rollout evaluator 已按运行时语义将 `mysql_fallback` 校验为 MySQL 最终路由子集；真实 Cassandra 错误回退不再被误拒绝为无效 evidence。默认比例、MySQL 即时回退和共享环境门槛保持不变。
- **本轮进展：** 新增只读 Cassandra read-rollout 起止 Prometheus 快照采集脚本，固定 deployment revision 与配置比例，拒绝覆盖既有窗口。evidence JSON 转换、共享环境采集和回切演练继续待完成。
- **建议方向：** 将 Prometheus snapshot 与脱敏服务 revision、Cassandra schema revision、配置比例、窗口和回切演练 ID 合成为 hydration evidence，再交给既有 evaluator；同时独立建立 Cassandra 历史读取 cohort 的 read-rollout evidence。两条服务端 Cassandra 轨道可与 Agent 和 Web Sync 客户端观察并行执行；Web Sync 窗口只约束旧 Offline 协议退役与客户端 locator 主路径。
- **处理门槛：** 任一 Cassandra 服务端比例提升前，必须归档对应共享环境 evidence、复核人批准和自动回切记录。evidence 间断、fallback、payload mismatch、冲突或延迟越界时立即将该轨道比例归零并回退 MySQL；MySQL 完整消息持续保留，直到各自替代契约完成验收。

### AD-044：Pencil 增量设计任务缺少稳定的 CLI 执行闭环

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-29
- **影响范围：** `design/dipole-ui.pen`、设计导出、F3 Agent Task Timeline、前端视觉回归
- **现状：** Pencil CLI `0.3.5` 认证和版本检查正常，canonical `.pen` 与既有导出资产可读取；本轮 Agent Task Timeline 增量任务在重复画布调用阶段长时间无输出，未生成新 `.pen` 或导出图，原设计文件保持不变。
- **风险：** 没有稳定的增量执行与导出结果时，无法把 Agent Task Timeline 设计资产纳入评审，也不能宣称 F3/F4 视觉基线已完成。
- **建议方向：** 将 Pencil CLI 调用拆成小批次、设置任务超时并在每次调用后校验输出文件、节点命名和导出图；失败时保留原文件并记录 CLI/skill 版本，必要时使用已批准 frame 作为回滚点。
- **处理门槛：** 新设计必须同时提交 canonical `.pen`、导出预览、`DESIGN-CHANGELOG.md` 条目和结构/视觉检查结果；未满足前不修改现有设计基线。
- **本轮进展：** 已保留 `design/agent-task-timeline-v1-brief.md` 作为下一次小批次输入；使用 Pencil `0.3.5` 和受限模型重复尝试仍在超时窗口内未完成，未生成 Timeline frame 或导出图，safe-edit wrapper 验证 canonical 未被覆盖。
- **本轮进展：** 新增 `design/export-manifest.json` 与设计门禁测试，持续校验现有批准导出资产存在且包含非空 PNG；该切片不宣称 Pencil CLI 增量执行已恢复，Agent Task Timeline F3/F4 仍等待稳定 CLI 结果。
- **本轮进展：** Pencil `0.3.5` 已通过安全包装器完成 Agent Task Timeline 小批次：canonical `.pen` 原子替换，desktop/mobile/state matrix、四个可复用组件和 2x 导出均已归档，并通过结构门禁。该证据解决 CLI 增量执行与导出闭环；完整页面截图基线、未覆盖平台和其余 F2/F3 页面仍待推进。

### AD-040：WebSocket 查询令牌进入 HTTP 访问日志

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **解决日期：** 2026-08-28
- **影响范围：** Gateway HTTP 访问日志、日志聚合与保留、WebSocket session JWT
- **解决方式：** Gin 统一访问日志在 handler 执行前解析 query，对 token、Authorization、API key、client secret、密码和签名类键进行大小写无关匹配，并把每个重复值替换为固定 `REDACTED`；非法 query 不回退原文，整段记录为固定脱敏值。普通参数规范化后继续提供路由诊断，现有 WebSocket query/Bearer 认证协议保持兼容。
- **验证：** 单元测试覆盖普通参数、百分号编码键、大小写变体、重复凭据和非法分隔符；Zap observer 经真实 Gorilla WebSocket upgrade 捕获访问日志，确认 query token、编码 access token 与 Authorization Header 均未进入结构化字段。logger/server/WS 相关包 race 测试通过。
- **长期约束：** 新增任何 query credential、短期 WS ticket 或签名参数时，必须同步更新敏感键集合和真实日志 capture 测试。反向代理与外部日志采集器仍需独立确认不记录脱敏前的原始 URI；认证传输方案变化需保留客户端兼容窗口和重放威胁测试。

### AD-038：Agent 离线评测缺少真实 Task adapter 与生产语料

- **优先级：** P1
- **状态：** 处理中
- **2026-08-30 进展：** `project-guardian-synthetic-corpus` 已把四类 Project Guardian 关注事件与四类干扰事件收口为版本化、双 reviewer agreement 的低敏基线。它使用固定 fixture 标识；规则 evidence 直接复用生产 `matchEventSubscriptions`，回归测试同时验证 corpus/review hash 绑定及 production matcher 的 precision/recall/cost 门槛。Remote GPU Node 22 已通过 Agent Runtime `133` 个测试文件、`695` 项、typecheck 与 build。真实 Task/Run、人工受控语料、retrieval relevance、模型成本分位与共享观察窗口仍未具备，状态保持处理中。
- **2026-08-30 验证：** Context calibration fixture 已通过 5 类合成样本并生成 hash-bound report；该结果不构成真实 Task、模型和人工语料证据，不能开启 Agent active authority 或生产上下文灰度。
- **发现日期：** 2026-08-27
- **影响范围：** Agent Eval、Shadow 晋级、Memory/Retrieval、模型与 Prompt 发布
- **现状：** TypeScript Runtime 已提供严格的 outcome、trajectory、permission、retrieval、cost deterministic Harness、语言中立 Suite/Report schema、canonical SHA-256 和三态 CLI；promotion v2 强制绑定同一候选版本的完整五类报告并逐类别阻断。security suite 串联真实结构边界。真实 Shadow adapter 现通过 sqlc/TS 共享只读查询提取 Task/Run/Context/Step/Artifact/ModelCall/ToolCall，将数据库 observation 与版本化评审 manifest 合成五类 Suite；Task/Run 摘要绑定 case ID，独立 MySQL 账号仅具八张审计表 SELECT。通过门槛的 v2 证据可发布为不可变 `promotion_evaluation` Artifact，并通过 Gateway-only projection 审阅。Subscription corpus review v1 另以 corpus SHA-256 绑定双 reviewer 完整标签和第三方分歧裁决，输出不含正文/身份的 agreement 报告。migration v32/v33 已建立 durable grant 与双人控制面，active context 会逐次重查有效期和撤销状态。
- **补充：** 失败模型调用的 Token 字段为 `NULL` 时，Shadow Eval 现输出完整五类报告并将 Cost 的 `availability.tokenMetrics` 记为 `unavailable`，稳定失败原因为 `token_metrics_unavailable`。已知调用数和延迟可以审计，Token/成本仅作为已报告值的下界，不能通过成本门槛或外推成功率。
- **风险：** 当前证据可证明 Harness、结构性门禁、评审一致性合同和真实持久执行转换语义。缺少实际归档的 Project Guardian outcome/evidence 与 review 报告、模型语义攻击 corpus、检索相关性集合和按模型/场景校准的成本分位阈值时，`eligible` 仍无法证明产品效果或生产成本满足目标。Step 表仅保存最后一次 attempt 的时间，真实 adapter 会拒绝 `attempt_count != 1`，逐 attempt 成本审计仍待补充。
- **本轮进展：** 新增 `reviewed_shadow` 窗口汇总契约与 CLI。它只接收同一候选版本、唯一 Suite SHA-256 且五类均为 Task/Run 摘要绑定的终态 Shadow 报告，输出脱敏的样本量、任务成功率、类别通过率与失败原因计数；混入合成 Suite、混版本或重复证据均 fail closed。该汇总仍只声明人工评审 Shadow 样本范围，真实固定任务集、trace 链接、共享观察窗口和用户灰度继续待完成。
- **本轮进展：** `agent_runs` 现持久化 Core admission 传入的受信任 `trace_id`，SQLC 读取、Shadow adapter 和 `shadow-report.v1` 将其绑定到每个五类报告；缺失/非法 Trace、Trace 复用、重放 Trace 漂移均 fail closed。`shadow-summary-report.v2` 仅在受限输入中使用 Trace 去重，对外汇总不回显 Task、Run、Trace 或模型正文。旧 Run 未回填 Trace，真实固定任务集和共享观察窗口仍待归档。
- **建议方向：** 建立版本化 Project Guardian corpus 和双评审 agreement，使用真实 adapter 按场景统计 precision/recall、trajectory 差异和成本分位数；报告仅引用受控 evidence ID。候选模型、Prompt、Tool Schema 和 Memory Policy 必须先离线，再 shadow，最后灰度。
- **处理门槛：** 任何 Agent active authority、自动 Memory 写入、语义检索切流或面向用户的主动消息发送前，至少归档一份真实候选五类报告及对应 Suite hash；当前 promotion v2 只可作为 Harness/Shadow 工程门禁。
- **本轮进展：** 新增 `dipole.agent.release-manifest.v1`，把 candidate、模型、Prompt、Capability Schema、Memory Policy 和 offline Eval Suite SHA-256 绑定，并要求 promotion 仅使用 `shadow` 阶段清单；真实 Project Guardian 语料、共享观察窗口和用户灰度仍未完成。
- **本轮进展：** release manifest 已接入 promotion publication 的显式新入口和 CLI；manifest 哈希随 Artifact/receipt 持久化，携带 manifest 的请求无法绕过 shadow 阶段或 Eval Suite 绑定，旧证据回放保持兼容。
- **本轮进展：** release manifest 增加单步阶段转移与回滚校验，禁止跨越 `offline`、`shadow`、`user_gray` 的相邻门禁；该函数只生成新 manifest，仍需 operator 证据才能改变实际 Runtime 开关。
- **本轮进展：** active Runtime 启动已强制读取 release manifest，并校验 `user_gray` 阶段与 candidate 一致；缺失、读取失败或版本/阶段漂移均 fail closed，默认 shadow 和 Go/Eino 回滚路径保持不变。真实五类评测、共享环境观察窗口和用户灰度仍待完成。
- **本轮进展：** 增加独立 `deploy/microservices/agent-active.yml` override，要求显式 candidate、manifest、独立 Kafka group、OpenAI-compatible Provider、v2 Context profile 与 Temporal endpoint/namespace/queue，并验证只读挂载；基础 Compose 仍固定 shadow，移除 override 即可回滚。生产 active 仍待真实五类评测、共享环境观察窗口和用户灰度。
- **本轮进展：** Active 部署运行手册已记录 input、静态渲染、共享环境证据、低敏记录和回滚步骤；Compose 成功只作为配置检查，不能替代 user-gray authority 验收。
- **本轮进展：** active overlay 强制关闭 subscription shadow、Memory、Control、MCP Server 与 External MCP，即使 host 环境设置基础开关也不会扩张 `read_active` 的 Capability 边界。
- **本轮进展：** TypeScript Runtime 启动链增加同一 active read profile 纯门禁，直接环境变量部署也会在创建 MCP/Control/external 运行资源前拒绝越界开关。
- **本轮进展：** 增加 `agent-external-mcp-shadow.yml` 受控 Compose overlay，显式绑定 Profile、I/O/route manifests、只读 secrets、Kafka broker/独立 consumer group 和 Temporal 运行参数；默认 Compose 不变，缺 Profile 渲染失败，关闭开关时 Runtime 不读取残留 Profile getter。该证据仍不覆盖共享 Core/Kafka、真实公网 DNS/TLS、凭据 owner 或外部 Server。
- **本轮进展：** 学习与面试主文档增加 Active Agent、Temporal Approval、SQLC、远程验证和 C++ 数据面证据速查；口径继续要求将共享环境 authority、默认关闭能力和规划项明确标注，不能以本地门禁替代运行时验收。
- **本轮进展：** 微服务 Compose 已显式固定 Agent 默认 `shadow`、candidate 和 manifest 路径；默认不挂载 manifest，active override 必须以只读方式提供 `user_gray` 清单，防止部署层绕过启动绑定。生产 active 仍待真实五类评测、共享环境观察窗口和用户灰度。

### AD-037：MCP 网络入口尚缺 OAuth、外部连接与写能力门禁

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Runtime、MCP Client/Server、Gateway/OAuth、Capability Policy、外部数据流
- **现状：** 官方 MCP TS SDK v2 Client/Server foundation 与默认关闭的 Gateway/Runtime 网络入口已完成，当前生产只投影 `conversation.list`。第一方授权交换要求 session principal 对 canonical resource 和只读 scope 显式 consent，签发 15 分钟且绑定 `aud/scope/token_use` 的 MCP JWT；普通 session 与 MCP token 互相拒绝。Gateway 剥离外部凭据并向 Runtime 证明已验证 principal/resource/scope。单次 Tool invocation 有 100 ms 至 60 秒有界 timeout、cooperative cancellation 和 `tool_timeout` 审计；外部 Client foundation 的 connect/list/call 也使用 request/total timeout，Runtime 传播连接断开信号。migration v30、统一低敏 OTel、默认关闭的 Collector/Tempo profile、共享 Redis principal 限流与真实 trace smoke 已完成。外部连接 Profile v1 现以严格契约绑定 tenant、HTTPS endpoint、Server identity、Tool/Host/Port allowlist、TLS ServerName、CA 与版本化 credential opaque ref。Credential Catalog v1 进一步保存 tenant/ref/version、生效窗口、active/revoked 状态及 opaque provider secret ref，每次建连前重新加载并精确授权，轮换/吊销无需进入 Task 或 Workflow 状态；受约束文件 source 使用规范绝对路径、canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode 和 256 KiB 默认上限，并支持原子替换。Provider-neutral MCP `AuthProvider` adapter 每次请求读取 fresh bytes，使用独立 timeout/AbortSignal、大小和 Bearer 字符校验、固定脱敏错误与源 buffer 擦除，同时不暴露自动 401 refresh。可注入 Network Guard 对每个 SDK 请求重新校验 exact HTTPS Host/Port/TLS identity，要求全部 DNS 答案为公网地址，把批准地址交给 pinned Dispatcher，并核对实际 peer；重定向、混合/重复/超量答案和 rebinding 均 fail closed。外部 Tool 成功结果可通过有界 adapter 转换为 `section=evidence`、`trust=untrusted` 且绑定 Profile/Server/Tool/Invocation provenance 的 Context fragment；compact 记录不复制外部正文。Core 现提供 active-only Approval grant resolution 与原子 consumption RPC：sqlc 精确查询最多两条候选，应用层要求 active Run、运行中 Task、principal 审批人和唯一未消费 binding；TS 独立复算 scope/arguments 并连接 write gate。`nonce_sha256` 明确作为持久的一次性绑定摘要。认证 Message Command RPC 要求 running Tool Invocation，Core 复算 canonical 参数摘要、派生 Command ID/身份并返回可验证 Message action reference。默认关闭的第一方 Message projection 已能按 `consume -> begin -> command -> finish(action)` 顺序组合这些边界，并要求 active context、显式 executor 与精确 direct conversation。active Run admission 必须经过注入式 promotion authorizer，MCP context 使用持久 Run 的权威 `runtime_id/mode` 并由 Go/TS 双重校验；migration v33 增加仅认证 Gateway 可调用的 Runtime promotion 提案、复核、查询和撤销控制面，Runtime 数据面不能签发 Grant。migration v35 保存可恢复的权威外部 Tool command；migration v36 进一步按确定性 Round ID 原子认领最多两个 Tool round，仅原 owner 可提交终态，已完成/失败结果可重放，任何遗留 `executing` 都返回 `ambiguous` 且没有 lease reclaim/retry 路径。Activity 在发起远端调用前 Claim，并在返回 Temporal 前持久化规范结果。生产未注入 authorizer，Registry、write executor 和 active context 继续关闭。真实 DNS Resolver、pinned TLS Dispatcher 与文件 CA provider 已独立实现但尚未装配到生产启动链，进程继续 fail closed。外部 MCP Server、加密 Secret backend 和 write/destructive Tool 均未启用。
- **本轮进展：** Streamable HTTP Transport Factory 已精确复核 Profile/Catalog 的 tenant-ref-version，为每次连接创建独立 AuthProvider、Network Guard 与官方 SDK Transport，并关闭 401 自动刷新、403 扩权和 SSE 自动重连。该组合只完成策略层，未提供生产 Secret/DNS/TLS I/O backend。
- **本轮进展：** 新增 production DNS Resolver、pinned TLS Dispatcher 与受约束文件 CA provider。Dispatcher 每次重载 CA，只允许自定义 lookup 返回当前批准地址，禁用代理/连接复用，并验证 chain、ServerName、remote peer、connect timeout、取消和流式 body；启动链与外部网络开关未接线。
- **本轮进展：** 新增本地 AES-256-GCM encrypted-file Secret Provider。二进制 envelope 的 AAD 绑定 tenant、credential ref/version、provider、secret ref 与 key ref；key 和密文文件独立映射、每次读取、权限约束且错误脱敏，版本轮换可移除旧映射而不 fallback。该适配器尚未接入启动链，也不替代独立 KMS、lease 和吊销告警。
- **本轮进展：** 新增 default-off production I/O composition，把文件 Catalog、encrypted Secret、Node DNS、文件 CA、pinned TLS 与 Transport Factory 收敛到只公开 Registry 的单一构造边界。disabled 不读取残留配置，enabled 构造也无文件/DNS/socket side effect；`index.ts`、环境路径加载和外部 Worker startup 仍未接线。
- **本轮进展：** 新增语言中立 production I/O manifest v1 与安全 loader。manifest 只保存 opaque ref、规范路径和有界参数，运行时复核全局唯一路径、key 关联及 owner-only/canonical/`O_NOFOLLOW` 文件证据；disabled 完全不读，重载失败不复用旧配置。`index.ts` 注册、下游文件 preflight、tenant 灰度仍待完成。
- **本轮进展：** production I/O runtime 现只公开 Registry 与 readiness preflight。预检在单一逻辑时间解析全部 Profile，去重后验证 Catalog active binding、encrypted Secret/Bearer 与 CA 文件，固定低敏收据和错误，并复用 Provider/Transport 的同一 secret 大小上限；真实文件测试覆盖 revoke、错 key、envelope/CA 损坏和恢复。该路径不创建 Transport、DNS 或 socket；`index.ts` 注册、tenant allowlist、隔离 Shadow 连通与回滚演练仍待完成。
- **本轮进展：** production runtime 增加显式 Profile/tenant 的只读 Shadow connectivity drill。它复用正式 Registry 和 modern allowlisted Client，仅执行连接、Server identity 与 Tool discovery，要求完整 allowlist 后关闭资源；协议级测试证明 `tools/call` 为零，收据及失败固定低敏。`index.ts`、自动调度与生产网络保持关闭，仍需在隔离 Shadow tenant 归档真实公网 DNS/TLS/协议、超时和回滚证据。
- **本轮进展：** readiness evidence v1 保持兼容，新增 v2 把 local preflight 与 exact Profile online drill 约束在有界时间窗，并以 canonical SHA-256 分别绑定 exact Profile 与完整 production I/O topology。migration v37 和 Go Publisher 以确定性 ID 追加保存 canonical 低敏 bundle、tenant、双 binding、operator、request/trace 与最长一小时有效期；exact replay 幂等，漂移留新历史，fresh reader 强制 tenant、双 binding 和 expiry。证据仍未签名，真实 Shadow 归档、自动 admission、独立审计导出和回滚演练待完成。
- **本轮进展：** additive readiness Publisher RPC 只允许认证且无 principal 的 `dipole-agent`，服务端派生 operator/request/trace 并严格解析 v2；TS adapter 复算 content hash 与确定性 Evidence ID，响应漂移 fail closed。RPC 已进入 Core composition，但自动采集/调度、admission consumer 和外部网络 startup 继续关闭。
- **本轮进展：** 显式单次 readiness 发布器把 production collector 与认证 RPC 串联，要求受控 tenant/Profile、60 至 3600 秒有效期和非空 request/trace；证据完成后只调用一次 Publisher，失败或取消不会重试。独立 CLI 才会读取 manifest 并执行只读公网 discovery，常驻 `index.ts`、Compose、自动调度与 admission consumer 继续关闭。
- **本轮进展：** fresh readiness Resolver 以 Core 服务端时钟执行 tenant/双 binding 精确查询，并在 Store 返回后复核 canonical 内容、确定性身份和 freshness；只读 RPC 仅向认证 `dipole-agent` 返回低敏收据或 `found=false`。自动 admission、activation、签名和独立审计导出仍待完成。
- **本轮进展：** MCP Worker construction root 已强制接收 host-owned Profile、production I/O 与 raw Registry，并在每次外部连接前自行派生 exact Profile/Runtime binding、调用 fresh Resolver、复核低敏回执和 underlying Profile。证据不缓存，缺失、漂移、取消与解析失败均在 raw Registry/Catalog/网络前关闭；readiness 采集继续使用独立 raw Registry以避免循环门禁。该控制只约束单次 exact Profile egress，不改变 Run admission、promotion 或 activation；Worker startup 与真实外部网络仍关闭。
- **本轮进展：** 新增 credential-free deployment route manifest v1 与安全 loader，把部署拥有的 route/version、Workflow 坐标、Profile/Server/Tool/egress policy 同代码拥有的 Capability schema、resource resolver 和 egress ceiling 精确 join；重复坐标、Profile allowlist 漂移和扩权均拒绝。完整部署摘要纳入 Temporal history/checkpoint，覆盖漏升版本时的 Profile/Tool/egress 漂移。loader 尚未进入 `index.ts` 或 Worker registration，外部网络继续关闭。
- **本轮进展：** 新增 default-off deployment plan，将 Profile、I/O manifest、route manifest 与 production runtime 收敛为一次加载和一个失败边界，并复用 exact config/I/O/options/raw Registry 给 readiness collection 与 gated Worker。构造不读取运行期凭据文件、不建 RPC/Worker/网络状态；`index.ts`、Compose、自动 preflight/drill 和路由调度仍关闭。
- **本轮进展：** 默认关闭的 MCP Worker Runtime 已组合 Core command resolver/round receipt、Transport Registry、Activity continuation 和三 ID dispatcher；替换实例可回放 completed receipt，ambiguous/cancellation 在网络边界前 fail closed。Temporal 调度入口仍等待受信 Agent Step 创建持久 Invocation。
- **本轮进展：** Invocation begin/finish 已支持全字段精确重放，Repository 读取终态摘要与 action reference；Command RPC additive 返回 Invocation 状态。terminal Invocation 的 Round Claim 只允许读取已有 receipt，缺失或漂移时拒绝且不会创建新执行记录，关闭 Invocation finish 后 Activity completion 丢失的重复远端调用路径。
- **本轮进展：** 默认关闭的 `TrustedMcpInvocationProducer` 使用 host-owned Workflow step/ordinal 生成稳定 Invocation ID，通过 Capability route、PolicyEngine、schema 与 egress policy 派生并验证 Profile/Server/Tool/参数；输入无法携带 authority 字段。
- **本轮进展：** additive terminal RPC 只接受 Task/Run/Invocation/Round ID，Core 从 durable receipt 派生 read-risk Invocation 的结果、字节数、错误码和首次 latency；terminal 重放核对已存证据，`executing`、`input_required`、write Capability 与绑定漂移均拒绝。默认关闭的 terminal Worker composition 已连接成功/稳定失败路径，生产仍缺真实路由、Temporal 调度与 I/O backend。
- **本轮进展：** 独立且默认关闭的 Temporal MCP dispatch Activity 已固定 route ID/version、Workflow step/ordinal、canonical 参数和完整性 checkpoint；每次 begin/retry/resume 都重新解析 Core ExecutionContext、精确重放 producer，并只向 terminal Worker 下发 Task/Run/Invocation 三个 ID。完成结果经注入式幂等 projector 收敛为 Artifact 收据，原始外部结果不进入 Workflow 输出；生产 Worker、启动入口、Activity mode 与外部网络保持未接线。
- **本轮进展：** 默认关闭的 `ExternalMcpArtifactProjector` 会重新解析 completed Invocation 并核对完整身份与 Profile/Server/Tool/Capability，从 terminal 结果生成 128 KiB 内的 canonical JSON Artifact；Invocation-derived type 隔离同一 Task 的多次调用，metadata 固定 untrusted lineage，精确重放和 Artifact 提交后取消均返回同一 content-addressed 收据。现有 Artifact policy 只允许 running shadow Run，active 结果仍 fail closed。
- **本轮进展：** Temporal dispatch route manifest 现对 route ID/version、Capability、Workflow step/ordinal 生成 canonical SHA-256，并同时绑定 begin history 与 wait checkpoint；同 route version 下的配置漂移也会在 Core 访问前拒绝。生产调度仍需确保该摘要来自版本化 host manifest，不能接受模型、事件或客户端注入。
- **本轮进展：** `createTemporalMcpDispatchRuntime` 已将 route-bound Activity、fresh Core Context、stable producer、terminal Worker 和 Artifact projector 收敛为单一 default-off construction boundary。Worker egress policy 直接从指定 Capability route 派生，删除装配调用方的重复 Profile/Tool policy 输入；公开结果只含 route binding 与专用 Activity。完成丢失、durable resume 和取消已通过端到端组合测试，生产启动仍关闭。
- **本轮进展：** `createTemporalMcpMultiRouteRuntime` 已把 deployment plan 中全部 route-scoped runtime 收敛到唯一 Activity 表面，构造时拒绝空集/重复 route，begin 与 resume 分别按输入或 durable checkpoint 的 route ID 选取 runtime，随后继续执行 route-local 全量绑定校验。该层无 I/O 或启动副作用；Workflow 已引用该 Activity，生产 Worker registration、`index.ts`、真实路由调度与外部网络仍关闭。
- **本轮进展：** `external_mcp_v1` Workflow history envelope 与专用 Temporal client 已隔离普通 goal/event 路径。route version/manifest digest 由 host catalog 注入，Workflow 首次派生 Task/Run/principal，恢复仅传 durable checkpoint 与验证后的 Signal；真实 Temporal Server 通过 Worker replacement/Elicitation resume 及现有 Workflow 回归。生产 Worker 尚未注册 MCP Activity，`index.ts`、真实 Capability route 与外部网络继续关闭。
- **本轮进展：** active Agent Runtime 增加独占 `read_active` Temporal Activity profile；Task/Run/Event 绑定后由 Core RPC 返回权威 ExecutionContext，终态沿用显式 `runtime_id + mode`。`ai_sdk` 模式现使用显式 OpenAI-compatible Provider adapter，Provider name 绑定 route，base URL 与密钥在启动前校验；`DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS` 默认关闭，只有已验证 JSON Schema response format 的 Provider 才能显式开启，避免泛用兼容端点收到不支持的 schema 字段。默认 `metadata` 保持零 Provider 构造。该切片只覆盖 `conversation.list/read`，Artifact、Message write 和其他 active Capability 仍 fail closed。
- **本轮进展：** 开发阶段的 AI SDK Shadow Compose 输入已收敛到版本控制的 `agent-ai-sdk-shadow.yml`。overlay 只选择 Provider 类别和严格变量名，具体 base URL、API key、route、预算、Context profile 与 structured-output 能力仍从受忽略 `.env` 注入；移除 overlay 即恢复 metadata Shadow，避免共享环境依赖未跟踪的运行时 YAML。
- **本轮进展：** Provider response-format 兼容性增加显式 `json_schema/json_text` 输出模式。`json_text` 不扩张调用次数：单次 provider 响应必须是纯 JSON，并由原始 Zod schema 本地拒绝无效结构；真实 Provider 兼容性仍需逐 route 受控验证，不能将开发探测外推为 active authority 证据。
- **本轮进展：** `json_text` 兼容解析仅额外接受完整 JSON Markdown 围栏，随后仍用原始 Zod schema 严格验证；任意围栏外解释、无效 JSON 或 schema 漂移继续失败关闭，避免把模型自然语言回答误作 Capability plan。
- **本轮进展：** 针对兼容 Provider 偶发内联 reasoning，`json_text` 只会剥离起始且完整闭合的 `<think>...</think>` 区块，再走既有 JSON/Zod 验证；该正文不持久化，任意其他前后自然语言仍 fail closed。
- **本轮进展：** `json_text` 可从前置标签后提取唯一且终止于响应末尾的平衡 JSON object，拒绝任何对象后的文本。提取不绕过 Zod schema、只读 Capability allowlist、ModelRouter 单次预算或持久审计；真实 Provider 行为仍需逐路由验证。
- **本轮进展：** Remote GPU 开发 Compose 已以 DeepSeek V4 Flash 完成真实私聊事件到 Kafka、Core Capability RPC、单次模型审计与 Shadow Plan 的只读闭环；最新 Run/Call 均为 `completed`，并新增一条 Shadow Plan。该验证仍未覆盖 Temporal active、写 Capability、用户灰度或生产 Provider authority。
- **本轮进展：** 增加显式 `agent-temporal-read-shadow` Compose overlay：Temporal 与 PostgreSQL 仅在加载 overlay 时启动且不映射公网端口，Agent 固定 `read_shadow`、独立队列和 v2 Context；Memory、检索、Control、MCP、OAuth callback 与写 Capability 均被覆写关闭。Temporal 必须显式绑定 `0.0.0.0`，否则镜像自动选择容器 IP 会使 loopback 健康检查阻塞 Agent 启动；Compose 合同门禁已固定该前提。独立 Core 同步装配 approval、Task control、Workflow projection/repair 与受存储开关保护的 Artifact RPC，避免独立服务路径退化为基础 Capability adapter。Agent 服务显式读取受忽略 `.env`，避免容器重建丢失开发期 Provider route/credential；缺少该配置时 AI SDK overlay 继续拒绝启动。2026-08-31 Remote GPU 使用受控 synthetic read-only 事件验证 Task/Run 投影、Temporal、Core mTLS、单次 EventLedger 和 `conversation_digest` Artifact 均收敛；Flash 输出预算仅在开发 `.env` 调为 `1024`。该证据不开放消息写入、active authority、外部 MCP 或默认 Compose，移除 overlay 可恢复基础 Shadow Planner。
- **本轮进展：** OAuth callback handoff 的组合测试固定 Runtime 重启后不依赖内存去重，以及 terminal completion 失败时 lease 保留。Core conditional lease 仍是跨进程重复的权威门禁；callback route、key source 装配、provider code exchange、token lifecycle 和默认配置继续关闭。
- **本轮进展：** default-off Temporal Worker composition 已将 deployment plan、multi-route Activity、普通 lifecycle Activities 和 matching Workflow catalog 组成同一不可变 authority snapshot。disabled 不解析端口；enabled 在端口 provider 前复算 Runtime binding 并验证 route/egress/collision，随后拒绝工厂 binding 漂移。Worker 类型已支持 additive Activity，但生产 `index.ts` 尚未加载 plan、创建 RPC/Client 或实际注册，外部网络继续关闭。
- **本轮进展：** managed Worker startup plan 已固定 `load -> validate -> resource -> compose` 生命周期。disabled 与静态 composition 冲突都不创建 resource；资源后的取消/组合失败先 rollback close，成功句柄幂等关闭并对构造/清理错误固定脱敏。该层仍不创建 Temporal Worker/Client 或执行 readiness/network；生产接线、真实 read-only Capability definition、Shadow route 与停止顺序集成证据仍缺失。
- **本轮进展：** default-off Worker lifecycle owner 已组合现有 Temporal Runtime 与 managed startup snapshot。disabled 零 Worker 调用；构造/启动失败回收 resource；正常关闭严格执行 Worker/connection stop 后再关 Core/Artifact resource，并在任一失败下继续清理且幂等脱敏。该层尚未接入 `index.ts`/Compose 或真实 RPC；生产 Shadow 配置、readiness publication 与进程级信号集成证据仍缺失。
- **本轮进展：** 首个代码拥有的外部只读 definition `repository.issue.read` 已固定严格参数、规范化单 Issue resource scope、read 权限与 1 KiB egress ceiling。Definition Registry 可 seal 且冻结 authority snapshot，deployment 无法追加定义或扩权。生产 startup 尚未使用该 factory；受控 Profile/route manifest、RPC resource factory 与 Shadow 运行证据仍缺失。
- **本轮进展：** 默认关闭的 Agent Capability RPC resource factory 已将一个认证 client 精确复用为 MCP Core/Artifact ports，并在 transport 前拒绝 disabled RPC、空 Profile 和跨 Shadow tenant deployment。取消回滚与显式 close 均幂等脱敏。生产 startup 尚未组合 definition/resource/lifecycle，`index.ts`、Compose、readiness publication 和外部网络仍关闭。
- **本轮进展：** default-off Shadow Worker bootstrap 已组合 seal definitions、deployment loader、lazy RPC resource、managed startup 与 Temporal lifecycle，固定 disabled 零 RPC/Worker、副作用交接和单 owner 回滚。它已是可调用的完整 Worker root，但生产 `index.ts`/Compose 未接线，Workflow client、受控 manifests/readiness 发布、Shadow 演练和进程信号集成证据仍缺失。
- **本轮进展：** RPC resource 现用同一认证 client 同时派生 persistent Workflow lifecycle Activities、MCP Core 与 Artifact writer，并保留 host Step Activity；startup 在 RPC 前校验 base snapshot、RPC 后重新校验实际冻结 snapshot，消除未来 `index.ts` 另建 lifecycle RPC transport 的需求。生产 activity-mode 约束、Workflow client authority 与真实进程接线仍缺失。
- **本轮进展：** 新增可信外部 MCP Shadow dispatcher 原语。它复算确定性 Task ID，从已验证事件/身份固化 admission 与 goal，并把冻结快照交给宿主 selector；selector 的严格输出只能含 route ID 和业务参数，版本/manifest authority 继续由 Workflow catalog 注入。当前 subscription match 仍只投影 `subscriptionId`，尚未保存或装配 definition-to-route 映射；生产 Workflow Client、`index.ts`、Compose 与真实 Shadow 运行继续关闭。
- **本轮进展：** subscription match 现将 Core 已授权的 winning definition ID/version、tenant 与 Agent 连同 subscription ID 固化到可选事件 binding；代码拥有的 route selector 只按 exact definition version 路由，并在参数 resolver 前复核 subscription/tenant/Agent。旧事件保持兼容，生产尚无 route/resolver 注册、受管 Workflow Client 或进程接线，外部 Worker 与网络继续关闭。
- **本轮进展：** 新增受管外部 MCP Temporal Client lifecycle。Worker owner 冻结其实际 Temporal config，Client 复用同一地址、namespace、task queue 与 exact route catalog；disabled 零副作用，取消/构造失败回滚，停止时 drain 已接受 start 后幂等关连接且不越权关闭 Worker。生产 route registration、Kafka/Client/Worker 进程级装配、`index.ts`、Compose 与真实 Shadow 证据仍缺失。
- **本轮进展：** 新增外部 MCP Shadow Temporal process owner，将 Worker bootstrap 与 managed Client 收敛到一次所有权交接和同一取消边界。Client 失败回收 Worker；停止严格执行 Client drain/close 后再停 Worker/Core，任一失败仍继续且幂等。Kafka consumer 与该 owner 的进程级装配、生产 route/resolver、`index.ts`、Compose、readiness 发布和真实 Shadow 证据仍未接线。
- **本轮进展：** 新增 Kafka + 外部 MCP Temporal Shadow process owner，只允许 subscription trigger，按 Temporal 后 Kafka 启动、Kafka 后 Temporal 停止，并覆盖取消和部分启动回滚。subscription matcher 从 Worker 持有的同一认证 RPC resource 逐层借用，Kafka 不再创建重复 RPC transport；matcher 缺失会在 Kafka 构造前回收 Temporal。生产 route/resolver、`index.ts`、Compose、readiness 发布与真实 Shadow 证据仍未接线。
- **本轮进展：** deployment route manifest 现可选绑定 exact Agent Definition ID/version 与经 Capability schema/egress 校验的静态参数，并把完整 trigger mapping 纳入 route digest。Worker composition 在 RPC 前拒绝重复或 catalog 外 mapping，冻结快照后由 production selector factory 使用。旧 manifest 保持可读但没有 trigger authority；独占 Temporal activity mode、`index.ts` 生命周期、Compose 与真实只读 Shadow 演练仍待接线。
- **本轮进展：** `external_mcp_shadow` 已作为独占 Temporal activity mode 接入 `index.ts`：Profile/Temporal/Kafka subscription/RPC 必须完整对齐，旧 Kafka 与旧 Worker 在该 mode 下不构造，统一 process 负责启动回滚和 Kafka-first 停止。Compose 默认仍关闭。隔离 in-memory Temporal 与现代 MCP Client/Server 演练通过 17 项，覆盖 Worker replacement/resume/cancel 和 `tools/call=0` discovery；真实 production manifests、Core/MySQL、Kafka、凭据、DNS/TLS 与公网 readiness evidence 联合演练仍缺失。
- **本轮进展：** 增加可重复的隔离全栈 Shadow 演练：独立 MySQL/Kafka、临时 Temporal、owner-only route manifest、可信 Core 夹具和本地只读 MCP 串联同一 subscription event；持久 ledger 重启重放保持一次 Tool/Artifact，过期 readiness 在 raw Registry 前阻断第二次 Tool。低敏证据明确 `production_authority=false`，本地 transport boundary 不替代真实公网 DNS/TLS、凭据和 Core mTLS 演练。
- **本轮进展：** 隔离演练证据已固定为语言中立 v1 Schema，并由 Runtime 创建器与独立 CLI 共同校验 strict 字段、成功不变量、canonical content SHA-256 和最多 24 小时有效期；未同步更新 hash 的内容漂移及自然过期文件无法通过脚本。content hash 不提供签名来源、防伪造能力或生产授权。
- **本轮进展：** 全栈演练 Core 边界已从进程内夹具替换为测试专用 Go mTLS RPC fixture，复用生产 TLS 1.3、客户端证书验证、secret metadata、caller allowlist/CN 绑定和正式 TS RPC Client。错误 secret、错误 CN、缺失客户端证书及完整 12 类 RPC 链均有证据；v2 仍固定 `production_authority=false`，共享 Core 与真实外部身份尚未覆盖。
- **本轮进展：** 增加离线 encrypted-file Credential 生命周期演练与严格低敏 v1 证据。真实临时 key/envelope、Catalog 原子替换和 production I/O Registry 证明 v3 到 v4 的版本切换、重建恢复、旧/当前版本吊销均在新 Transport 构造前 fail closed，三次成功连接全部关闭；证据明确没有在途请求主动吊销或生产 authority。共享 provider owner、真实 Server 端失效和公网 TLS 联合演练仍缺失。
- **本轮进展：** `NodeExternalMcpDnsResolver` 已提供每请求独立、无缓存的 production DNS 实现，并行解析 A/AAAA、接受单族无记录、对部分瞬时失败和畸形证据 fail closed，AbortSignal 仅取消请求本地 Resolver。Network Guard 仍负责公网地址、去重和 32 条上限复核；真实 pinned TLS Dispatcher 与加密 Secret backend 尚未实现。
- **风险：** 第一方交换尚未提供 RFC 9728 Protected Resource Metadata、Authorization Server Metadata、OAuth 2.1 Authorization Code + PKCE 和第三方客户端注册，因此还不能声明为通用 MCP OAuth Server。encrypted-file Secret、Node DNS、pinned TLS Dispatcher、文件 CA adapter、隔离 Core mTLS composition 与离线凭据生命周期演练已实现；共享环境仍缺 provider owner 授权、KMS/Secret Manager、真实公网 DNS/证书链/peer pinning、下游 Server 吊销及真实 Core 身份联合证据。Catalog 只能阻断新连接，无法主动中断已发出的远端请求。fresh byte buffer 可覆盖，MCP SDK 所需的 JavaScript token 字符串与 Header copy 仍由 GC 管理，无法证明强零化。Catalog 文件依赖 OS mount/owner 信任，没有签名、rollback revision 或可用性告警；拒绝 symlink 也意味着 Kubernetes 默认 projected volume 不能直接使用。Tempo local backend 只适合单机 Shadow/验收，尚无生产对象存储生命周期、Alertmanager 通知链和长期 trace/audit 联查证据。结构化 egress guard 无法识别被改名、编码或嵌入普通文本的敏感值；`trust=untrusted` 提供上下文隔离语义，模型仍可能受 Prompt Injection 影响，接入时还需 trajectory Eval、Tool Policy 和输出 lineage 共同约束。Round receipt 可恢复本地已接收并持久化的成功结果；远端完成副作用后、Runtime 收到结果前或本地收据提交前仍存在无法由本地数据库证明的极小窗口。当前策略把传输异常持久为 `remote_outcome_unknown`，后续重放失败关闭，牺牲自动恢复来避免重复副作用；具备服务端幂等键或查询收据的 Profile 才能进一步收敛。Approval consumption 提供 at-most-once 副作用门槛；Message Command receipt 与 migration v31 action reference 已连接 Approval、Command 和权威 Message UUID，消费后 operation 失败仍需新审批。Command RPC 在服务端提交后遇到客户端 deadline/cancellation 时仍需 receipt-driven action-lineage 收敛演练。promotion operator Grant 仍依赖受控运维预置，生产 authorizer 仍未注入；approved Capability 已有显式 allowlist 投影与 Go/TS 双重校验，UI 风险摘要和真实故障演练仍缺失。
- **本轮进展：** Runtime 增加不带 I/O 的 OAuth discovery/PKCE foundation：RFC 8414 metadata URL 按 issuer path 派生，metadata 强制 exact issuer、HTTPS endpoint 与 `S256`，Authorization request 只输出短时 verifier/challenge/state 材料且不把 verifier 放入 URL。HTTP discovery、state/verifier owner 存储、code exchange、refresh token、动态客户端注册与生产授权仍未接线。
- **本轮进展：** 平台级 Go/sqlc/Compose/架构文档/OTel 门禁全部通过，独立 OTel smoke 已验证 trace 经 Collector 写入并可由 Tempo 查询；该证据仅覆盖本地隔离环境，生产对象存储、Alertmanager 通知和长期 trace/audit 联查仍待完成。
- **建议方向：** 接入真实 OAuth 2.1 Authorization Server 后发布 discovery/PKCE；为现有 Factory 实现加密 Secret Provider、真实 DNS Resolver 和按批准地址建连的 TLS pinned Dispatcher，绝不把 secret 返回 Runtime。外部 Tool 结果作为 untrusted Context fragment。生产 trace 使用对象存储与通知链；write Tool 必须绑定现有 Approval 与 Agent lineage。手工 MRTR continuation、round receipt 和三 ID Worker dispatcher 已固定恢复与命令权威边界，后续仍需专用默认关闭 Worker 装配、多轮策略与敏感授权隔离。
- **处理门槛：** 任何共享环境 MCP 开关启用、外部 Server 连接或 write/destructive Tool 上线前完成。当前网络入口仅用于受控认证和授权边界验证。

### AD-036：Elicitation 缺少 MCP continuation 与敏感授权隔离

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Human-in-the-loop、Web 客户端、MCP 集成、凭据与第三方授权
- **现状：** `dipole.agent.elicitation.v1` 已固定 text/select/multiselect/boolean Form、动态响应校验、大小上限和绝对截止时间；Gateway JWT API 经 Core Task owner 复核后发送精确 request ID 的 Temporal Signal，Worker 替换可恢复同一等待点和 Timer，到期自动以 `input_expired` 取消。默认关闭的 MCP adapter 已将受限 form mode 映射为 `wait_input`，以 checkpoint 绑定 untrusted Server/Tool/Invocation/Form/deadline，并拒绝 URL、敏感字段与有损 schema。MCP `2026-07-28` Client seam 显式声明 Form Elicitation 并关闭进程内自动 fulfilment；手工 MRTR continuation 只接受一个 input request，将原 Tool 参数、请求键、可选 opaque `requestState` 和 lineage 绑定到完整性 checkpoint，并可在新连接中精确生成下一次 `tools/call`。真实 SDK Streamable HTTP 双轮契约已通过。canonical Pencil 与默认关闭的 Vue 页面覆盖 desktop/mobile 普通 Form、来源披露和七态，经 authenticated Task query/input/cancel API 精确提交当前 Task/request；Runtime 与 Web 均拒绝凭据类字段。Chromium、Firefox、WebKit 已验证精确请求绑定、首次失败后的 stale Form 清理与恢复、键盘错误聚焦、ARIA 关系和移动端单列布局。当前尚未把 continuation 装配进生产 Temporal Activity/外部 Transport Factory，也未交付多轮、敏感授权 URL mode、产品入口编排和视觉回归基线。
- **风险：** 浏览器闭环只能恢复已进入 `waiting_input` 的 Task；MCP Server 仍无法在 durable input 完成后恢复原 Tool 调用。将密码、Token 或 OAuth 信息放入普通 Form 会进入 HTTP、日志或 Workflow history，扩大敏感数据暴露面；未来生产接线仍需处理连接丢失、用户取消和 Server 无恢复能力等差异。
- **建议方向：** 保持普通 Form 的字段白名单和默认关闭灰度，后续补充 Pencil 视觉回归与产品入口编排。Activity-safe runner 已能跨实例重开现代 Client、校验 tenant-owned Profile 并关闭失败资源；下一步将其接入独立默认关闭的 Worker mode，并固定持久 Tool invocation、progress/cancel 和审计映射。第三方授权继续采用独立 URL mode、短期 challenge 与回调绑定。
- **处理门槛：** Project Guardian 的普通 Form UI 已完成并保持默认关闭；任何凭据、支付、OAuth 或外部 MCP Elicitation 上线前完成独立敏感输入隔离、continuation 和威胁建模。
- **本轮进展：** 第一方 Form Elicitation 已首次由生产 read Activity 产出：多会话发现在 claim 读取 Step 前返回 `wait_input`，Form 只含单个 select 字段与不可信会话候选，恢复时按确定性 request ID 与 checkpoint 候选集合双重校验。该路径不涉及 MCP continuation、URL mode 或凭据字段，敏感授权隔离与外部 Server 恢复仍未开放。

### AD-035：Memory foundation 缺少受控写入、版本纠正与压缩治理

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Memory、Context Compiler、隐私删除、长期事实质量、Project Guardian 演示
- **现状：** migration v29 与 sqlc Store 已保存五类不可变 scoped Memory、full/compact content、priority、有效期和 provenance；Core 根据运行中的 Task/Run 固定 principal、tenant、Agent 与 conversation read scope，并使用 Task 创建时间阻止后续新增记录进入重放，撤销/过期立即 fail closed。v38 已交付默认关闭的 owner list/revoke，v39 已交付 append-only correction 与五类离线结构 Eval，v40 提供 root-wide 内容擦除收据。v41 建立 `Memory -> Task` 直接引用与低敏影响审计。v42 将 lineage 生命周期绑定到权威 `agent_tasks`，`ModelShadowPlanner` 在 Context 编译后、任何模型调用前原子保存所选 Memory；写入失败时模型零调用，Plan-time 写入继续作为幂等修复且不会降级 `context_pre_model` 来源。历史 Context 只探测引用，真正缺少 Plan 和 lineage 的 owner 模型结果继续计入 `unattributedModelTasks` 并阻断完整声明。语言中立 derived-retention v1 现已完整覆盖七个持久域，离线决策绑定 policy/report/decision SHA-256，并从 lineage 完整性与人工复核域重新推导阻断原因；固定不读取正文、不执行删除且不授予删除或 Runtime 权威。历史回填已具备固定 high-water mark manifest/receipt、v43 MySQL checkpoint、sqlc source/target、owner-scoped adapter、可重放 Go runner、默认 dry-run/独立 operator/approver 审批 CLI、只读 rollout review CLI、OCI/config provenance collector、deployment evidence contract/CLI 和隔离 MySQL resume/owner/up-down 测试，仍未接入共享环境生产启动链或自动执行。Core 擦除、TS 审计、owner 页面和 correction 各自保持独立开关，自动写入继续关闭。
- **风险：** v42 已消除受管 Model planner 的 Context 到模型调用归因窗口，逐域策略也已可离线判定，但尚未证明字段级副本或实现 Shadow plan summary、Step input/output、Artifact body/metadata、Agent Message 与 Temporal history 的擦除/到期执行器。历史回填虽已具备可验证的有界索引链路，仍未获得共享环境 rollout 证据，任何旁路/旧 Runtime 产生的无 lineage 模型结果仍会阻断完整声明。真实多次纠正、语义冲突和 retrieval ranking 标注语料仍缺失，仅按 priority 的精确 scope 检索无法衡量生产 recall、precision 和 context 成本。
- **建议方向：** 下一步定义 owner 可见的派生治理收据和有界历史 lineage 回填，再按逐域策略分别设计字段级执行器与故障恢复；在执行能力启用前归档真实 correction/retrieval corpus并增加离线 Observation/Reflection Worker。写入策略要求来源证据、置信度、TTL、幂等键和冲突合并；基于 retrieval Eval 比较 MySQL 精确检索、Elasticsearch hybrid/vector 与 reranker。
- **处理门槛：** 在共享环境自动写入消息 Memory、启用跨 Task 长期召回、开放 owner 擦除 API或根据 Memory 自动执行动作前，完成派生域删除语义、历史索引完整性、Temporal/对象存储治理、真实 owner correction/retrieval 验收及安全评审；当前仅允许受控 seed、Shadow 读取、只读影响审计及默认关闭的 owner 查看、撤销和追加纠正，内部擦除方法没有外部调用路径。
- **本轮进展：** Observation/Reflection worker 的幂等键已从裸事件/窗口 ID 收敛为 tenant、principal、Agent、资源与 ID 的完整 scope；跨租户和跨资源复用标识的回归测试通过，避免候选被错误去重。其 shadow-only、人工评审和真实语料门禁保持不变。
- **本轮进展：** 新增默认 shadow-only 的 Observation/Reflection worker 与 `memory-candidate.v1` 严格契约。Observation 以事件 ID 幂等提取决定、任务和风险片段，Reflection 仅聚合同租户/主体/Agent/资源范围内的唯一 evidence window；两者都不访问模型、数据库、Kafka 或 Memory sink。超限、凭据模式、跨范围和重复窗口均 fail closed，后续仍需 candidate ledger、人工评审、Temporal durable 编排和真实 reviewed corpus。
- **本轮进展：** migration v45 与 TS MySQL ledger 持久化候选摘要、来源/证据 ID、策略版本、规范哈希和 `pending|accepted|rejected` 状态；重复候选必须通过 exact hash，冲突 fail closed，完整 candidate content 不进入 SQL 参数。ledger 仍不授予 `agent_memories` 写权限，人工评审、accepted 投影、Temporal receipt 和真实 corpus 继续待完成。
- **本轮进展：** migration v46 增加 append-only review ledger；`accepted|rejected` 记录绑定 candidate hash、reviewer、有限理由、时间和 review hash，并与候选状态在同一事务内更新。精确重复审查可重放，candidate/hash/status 漂移回滚；v46 回滚保留 v45 候选且不删除 Memory。accepted 到 `agent_memories` 的 Core 投影、Temporal receipt、双人/owner 策略和真实 reviewed corpus 仍未接线。
- **本轮进展：** migration v47 增加 promotion receipt，Core-owned service 与 sqlc Repository 在同一事务中锁定 accepted candidate/review、写入摘要型 observational Memory 并记录 `promoted_memory_uuid`；稳定重试返回既有 Memory，候选/审核/状态漂移回滚。公开 RPC、Temporal receipt、双人审批策略、真实 corpus 与自动写入开关仍未接线。
- **本轮进展：** 增加 `PromoteMemoryCandidate` additive gRPC 与 Gateway HTTP 控制入口。Gateway 只提交候选 ID、候选哈希和 review ID，Core 从认证 principal 派生 owner 并调用 v47 promotion service；Gateway/TS client 对返回 Memory 的来源、review 绑定和 active 状态进行复核。Temporal 自动触发、Runtime 旁路、双人审批和真实 reviewed corpus 继续关闭。
- **本轮进展：** 增加 `agent-memory-promotion-receipt.v1` 语言中立契约和 Temporal preparation Activity。receipt 仅绑定 Task/Run、owner、候选/审核摘要与短时效窗口，支持确定性重放和 fail-closed 过期检查；当前只形成 durable intent，未开放 Runtime 直接写 Memory、自动晋级或生产灰度。后续 active executor 已固定为独立 `dipole-agent` 内部提交 RPC：Core 必须从 Task/Run 恢复身份，校验 active admission、有效 Runtime promotion grant、receipt 与 candidate/review 重读，不能复用 Gateway owner RPC 或将 receipt hash 视作授权凭据；详见 [active executor 设计](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)。Core application 已新增 commit service，使用 JS `toISOString()` 兼容的毫秒 canonical 向量复核 receipt，再委托既有 owner-reviewed 事务。`CommitMemoryPromotionReceipt` protobuf/gRPC seam 现仅对认证 `dipole-agent` 放行，缺失注入明确 `Unavailable`；其返回模型已移除正文、资源和 owner 字段，TS client 在 active 模式严格复核低敏回包。Core bootstrap 仅在默认关闭的 `internal_rpc.agent_memory_promotion_receipt_commit_enabled` 与 mTLS 同时满足时注入该 service。Temporal 已具备可注入的 commit Activity：Workflow 必须显式请求提交，基础 Worker 固定拒绝，Activity 只传递 receipt/correlation 并记录低敏结果。独立 `promotion_active` Worker profile 现要求 active Runtime、Temporal、Capability RPC mTLS、`operator_approved` authority 与 Runtime/Core 双开关共同成立，且继续拒绝 Control、MCP、自动 Memory、subscription 与消息写入。共享环境的重放、失效 grant、观察和回滚证据仍未完成，默认路径继续关闭。
- **本轮进展：** `promotion:memory-worker-drill` 与 v2 worker drill evidence 契约现将共享环境演练的 revision、manifest/configuration/promotion evidence 摘要、grant、首个提交、同 receipt 重试、撤销 grant 拒绝和 overlay 回滚收敛为低敏 `eligible|blocked` decision。输入不包含 Memory 正文、候选摘要、凭据或消息内容；CLI 只检查记录一致性，不连接 Runtime、Temporal、Core 或数据库，不能把手工 JSON 转化为写入授权。真实共享环境原始日志、指标快照、维护窗口与回滚工单仍需独立归档。
- **本轮进展：** 隔离 Temporal test server 已覆盖 `commit=true` 的 durable workflow 路径：首次 commit Activity 失败后按 Workflow retry policy 重试，相同 prepared receipt hash 在最终结果中与低敏 Memory binding 一致。该测试不启动 Core/Kafka、不验证真实 grant，也不构成共享环境写入证据；它只缩小 Worker replacement 前的 Activity 重试风险。
- **本轮进展：** `promotion_active` 的永久 receipt commit 失败现收敛到 Agent Task `failed`，并继续调用持久 `finishAgentTask`。Temporal 集成测试覆盖 Activity 用尽重试后的终态与受限错误回写，防止 Workflows 以未完成的 Run 直接失败退出；真实 Core grant、Kafka 触发、共享环境回放和 overlay 回滚证据仍未覆盖。
- **本轮进展：** `promotion_active` 现同时验证独立 `dipole-agent-memory-promotion-` Temporal queue 前缀。Runtime profile 与 Compose render gate 都拒绝复用通用/read-active queue，减少 reviewed Memory commit Worker 误消费跨 profile 任务的风险；共享环境队列隔离、Kafka trigger 和 overlay 回滚仍需运行证据。
- **本轮进展：** active read/promotion profile 现将 retrieval 与 retrieval-to-Context 纳入 Runtime surface gate。Compose 以外的直接环境变量启动也无法开放 `conversation.search`，从而保持 active capability 为 `conversation.list/read` 加 reviewed receipt commit 的受限集合；Shadow retrieval 的独立评测和共享环境灰度仍未改变。
- **本轮进展：** 增加 Memory reviewed corpus v1、双 reviewer/独立 adjudicator 门禁和离线 CLI。输入不含消息正文，只绑定候选类型、资源、证据数量与内容哈希；报告不输出 case/reviewer 标识，分歧、覆盖不完整、gold drift 或 corpus hash 漂移均 fail closed。当前仓库仅有脱敏测试夹具，真实 owner-approved corpus、retrieval ranking 标注和灰度证据仍待完成。
- **本轮进展：** 增加 source manifest v1 与安全加载器，要求 owner UID、绝对规范路径、无符号链接、严格文件权限/大小、批准时间窗口以及 corpus/review 双哈希一致；该边界允许后续接入真实脱敏文件，同时阻断任意本地文件冒充已批准语料。当前仍缺真实 owner-approved corpus、发布签名和共享环境评测证据。
- **本轮进展：** 增加 Memory prefilter evidence v1 与 `eval:memory-prefilter` 离线 CLI，embedding/small_model 候选必须完整绑定 reviewed corpus、配置哈希和 score/threshold；报告仅含聚合分类、延迟、成本指标与门禁原因，缺失 case、哈希漂移或阈值漂移均 fail closed。当前仍缺真实 owner-approved corpus、embedding/小模型采集、retrieval ranking 标注和在线灰度证据。
- **本轮进展：** 增加 Memory prefilter rollout decision v1 与 `eval:memory-prefilter-rollout`，在发布判定前重新计算双 reviewer/gold 与 candidate evidence，绑定 corpus/review/final-label/evidence 哈希并以 `eligible|blocked` 输出。该门禁仍为离线证据，真实语料、候选采集、审批授权和在线灰度继续关闭。
- **本轮进展：** 增加 `runtime-binding.v1` 与 provider-neutral 三态 gate：`shadow` 只观察、`enforced` 仅接受精确绑定的 `eligible` 决策，所有模式固定无 Memory 写权。该 gate 尚未接入真实模型、Kafka subscription 或自动晋级，仍需真实语料和在线回切证据。
- **本轮进展：** 增加 Cassandra read rollout evidence v1 与 Go CLI，校验窗口/部署标识、路由计数不变量，并按样本量、观察比例、fallback、verification 和 Cassandra p95 门槛重算 `eligible|blocked`。当前仍缺真实共享环境 Prometheus 快照、责任人批准、快照/回放证据和生产回切窗口。

### AD-034：Event Subscription 缺少用户界面与语义预筛

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Trigger Engine、Definition 授权、模型成本、Gateway/前端配置与 Project Guardian 演示
- **现状：** migration v28 与 v34、sqlc Store、Core resolver 和受认证 RPC 已持久化精确 Definition version 订阅，并提供 Gateway principal 派生 owner 的创建、历史分页与可审计撤销。TS Runtime 可在 EventLedger、Temporal 和模型前确定性过滤。canonical Pencil 和默认关闭的 Gateway/Vue 页面已交付 owner list/create/revoke：创建候选由 Core 对 authenticated readable conversation 与 Definition scope 求交集，Gateway 从会话派生 principal/tenant 并从 conversation key 派生 event/resource，Core 在写入前再次复核；前端严格解析候选和权威结果，拒绝静默截断关键词。默认关闭的在线 Shadow 对照可在 direct-target 主路径进入 EventLedger 前调用同一 matcher，仅记录六种低基数结果和候选总数；matcher error 不阻断主路径，Prometheus 已提供 error/drift 告警。语言中立 prefilter Eval 已支持有界标签 corpus、三类 candidate evidence、分类/延迟/成本指标和生产规则基线；corpus review v1 要求双 reviewer 完整标签与第三方分歧裁决。Compose 与默认配置继续使用 `direct_target` 且 Shadow 关闭。
- **本轮进展：** Runtime matcher 在解析前限制最多 256 条 Core 候选，超限集合 fail-closed；既有本地过滤、Shadow 观测和生产开关语义保持不变。
- **本轮进展：** Event Subscription 控制面测试已迁入 Agent application 边界并直接验证创建、幂等回放、scope/owner 授权、可读会话交集、分页和撤销审计；聚合 `internal/app` 仅供该测试使用的兼容转发已删除，服务实现成为测试与生产装配的共同入口。
- **本轮进展：** Agent Command 测试已迁入 Agent application 边界并直接验证可信身份、关联 ID、幂等收据、异常恢复、绑定漂移和 fail-closed 行为；聚合 `internal/app` 仅供该测试使用的兼容入口已删除。
- **本轮进展：** Agent Capability 测试已迁入 Agent application 边界并直接验证主体限制、会话读取、权限与资源范围校验、关联上下文传递和依赖 fail-closed；聚合 `internal/app` 仅供该测试使用的兼容入口已删除。
- **本轮进展：** Active Run Promotion Authorizer 测试已迁入 Agent application 边界并直接验证租户、Runtime、候选版本、Definition 版本和时间窗口绑定，以及缺失授权和存储故障语义；聚合 `internal/app` 中无调用的兼容入口已删除。
- **本轮进展：** Task Control 测试已直接切换 Agent application 构造器，验证仍复用 `internal/app` 的共享 policy fixture，同时移除该测试专属兼容入口，减少聚合层依赖。
- **本轮进展：** Task Workflow Projection 测试已直接切换 Agent application 构造器，继续验证 Task/Run/Revision 绑定、终态漂移拒绝和 shadow cohort 分页；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** Agent Approval Service 测试已直接切换 Agent application 构造器，继续验证 Task/Run/Principal 绑定、审批幂等、伪造 Actor 拒绝和一次性精确消费；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** Runtime Promotion Control 测试已直接切换 Agent application 的时钟注入构造器，继续验证 evidence 绑定、跨租户拒绝、双角色审查和撤销审计；聚合 `internal/app` 中无调用的兼容入口已删除。
- **本轮进展：** Workflow Repair Audit 测试已直接切换 Agent application 构造器，继续验证修复提案 evidence 绑定、双人审批、冲突重放和拒绝优先；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** Workflow Repair Prepare 测试已直接切换 Agent application 构造器，继续验证已批准 quorum、执行计划幂等和未批准/绑定不匹配拒绝；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** Workflow Repair Executor 测试已直接切换 Agent application 构造器，继续验证新鲜授权、提交/回滚事务、前置条件失败和执行状态落账；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** Message Command Execution 测试已直接切换 Agent application 构造器，继续验证已绑定 Tool、Command 派生、identity/argument drift 拒绝和依赖 fail-closed；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** Agent Execution Policy 测试已直接切换 Agent application 的 Invocation Resolver 与 Run Admission 构造器，继续验证定义/Task/Run 绑定、授权窗口、active-run promotion 和 fail-closed；聚合 `internal/app` 中对应兼容入口已删除。
- **本轮进展：** 复核并删除 `internal/app` 中无调用的 Static Execution Policy 与 Memory Task Reader 兼容符号；其生产实现和接口继续由 Agent application 单独拥有。
- **本轮进展：** Shadow 指标已修正为记录原始候选集合大小，避免以匹配数替代候选数造成成本证据偏差；后续灰度仍需共享环境抓取和完整窗口。
- **本轮进展：** Shadow metrics observer 已在运行时拒绝闭集之外的 outcome，保持 Prometheus label vocabulary 与 evidence schema 一致；共享环境窗口仍待完成。
- **本轮进展：** 只读 Prometheus Collector 已对响应体实施 256 KiB 流式上限，并在超限、读取失败或 JSON 异常时统一 fail-closed；共享环境窗口与发布 artifact 交叉核对仍待完成。
- **本轮进展：** 增加 `SubscriptionRuntimeGate` 三态 rollout seam，并接入 Kafka Shadow Runtime 可选依赖：`off` 保持规则路径，`shadow` 允许任务并记录观察，`enforced` 仅接受精确哈希绑定且 `eligible` 的候选证据；默认未注入 Kafka、模型和生产 Task 创建。
- **风险：** 控制面已可安全创建确定性订阅，但 Runtime 仍未消费共享 subscription 流量。在线 Shadow 已有 24 小时窗口、抓取覆盖、counter reset 和零 error 的低敏证据合同，并用只读 Collector 固定查询/单 series/持续启用检查，但尚未归档真实共享环境 evidence；当前指标也无法独立证明部署 artifact revision，仍需发布记录交叉核对。确定性关键词无法覆盖语义等价表达；公开候选接口当前返回有界 Definition scope 的完整交集，尚无超大 scope 分页协议。直接启用共享环境订阅模式仍会造成难以运维的策略或相关事件漏触发。
- **建议方向：** 使用真实 Project Guardian reviewed corpus 采集 embedding 与小模型 candidate evidence，并与规则基线比较；随后设计 subscription Runtime 的分批灰度、漏触发/成本告警和回切证据。若单 Definition scope 扩展到当前上限之外，再增加稳定 cursor 的候选分页。高成本 Agent 只接收预筛后的事件。
- **处理门槛：** Project Guardian 或共享环境启用 `subscription` 前完成用户管理界面，归档真实事件 corpus、reviewer agreement 和至少一个候选 evidence/report；synthetic 规则示例只证明 Harness。语义预筛需先离线达标，不能直接对每条消息调用大模型。

### AD-032：Artifact 对象写入后缺少孤儿清扫证据

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Agent Artifact、MinIO 容量、MySQL 元数据、故障恢复与审计
- **现状：** migration v26 保存不可变 Artifact 元数据，正文使用 Task/Run/版本/内容哈希导出的确定性对象键。已增加固定前缀、24 小时门槛、sqlc 二次存在性查询和 SHA-256 报告的只读 dry-run；maintenance authorization/receipt 再绑定双审批、职责分离、15 分钟有效期、对象 Stat 与执行前元数据复核。三个离线/运行时身份均无删除权限。
- **风险：** MinIO 写入成功后若 MySQL 持续失败且任务不再重试，会留下无法从用户 API 引用的内容寻址对象。该对象不会覆盖其他版本，也不会获得读取授权，但会长期占用容量。
- **建议方向：** 在真实 Shadow 观察窗口持续归档 dry-run 报告和 receipt；是否增加 DeleteObject-capable 执行器需单独评审，并要求新的不可回退契约版本、独立删除身份、对象版本/保留策略、审批持久化和删除后 receipt，现有 Runtime/Core/audit/inspect 账号继续没有删除权限。
- **处理门槛：** Artifact 进入 active 模式或配置自动保留期限前完成；Shadow 阶段以容量指标和人工审计接受该风险。
- **本轮进展：** Agent Task Timeline 的 v1 protobuf、SQLC Store 和 repair queue 现保存可选 `artifact_id`；新 Artifact 事件要求小写内容寻址 ID，非 Artifact 事件携带该字段会 fail closed。该关联只支持 owner-scoped metadata 发现，不开放正文、对象键、下载或公开 URL；历史事件允许缺失关联以保持读取兼容。

### AD-031：Context Token 预算使用确定性近似估算

- **优先级：** P2
- **状态：** 处理中
- **2026-08-30 验证：** `services/agent-runtime` 的 `context:calibrate` fixture 通过，`eligible=true`，生成 `route-calibrated-v1:sha256:597b970fa91e846b0a0d8ae6407dba279cc77e051068954ec7d58553a7ec7afe` 与稳定报告哈希；该证据只覆盖 fixture tokenizer/合成语料，真实候选模型 route 校准仍待完成。
- **发现日期：** 2026-08-27
- **影响范围：** `agent-runtime`、Context Compiler、多模型路由、长上下文与成本门禁
- **现状：** 显式启用的 Context Compiler v2 已支持 route 声明 context window、UTF-8 bytes/token 校准值与安全余量，并对所有候选 route 取最大估算和最小窗口；未声明 route 使用固定保守 fallback，配置 SHA-256 estimator ID 随 Plan manifest 持久化。语言中立 evidence/report 与离线 CLI 已要求每个 route 覆盖中英文、代码、Emoji、Tool schema，逐项记录 reference/estimate/error、正文哈希及 provider revision。默认与 Compose 保持 v1，保护在途不可变 Plan 重放；实际 provider usage 继续由 ModelAuditStore 在调用后记录。
- **风险：** 不同模型 tokenizer、中文、多字节符号和 JSON 转义会产生估算偏差。接近模型窗口上限时，近似值可能低估输入并触发 provider 拒绝，也可能高估后过早省略证据。
- **建议方向：** 使用现有 evidence/report 契约按 route 归档真实 tokenizer 或 provider usage synthetic 校准集；比较估算/实测误差分布后再缩小 fallback 余量。对缺少可复现 tokenizer 的 provider 保持保守 profile，不根据单次 usage 自动学习或静默改变预算。CLI 的 `eligible` 只代表输入 corpus 零低估且无 fallback，生产启用仍需独立候选评审。
- **处理门槛：** 在 Context 接近任一生产模型窗口的 70%，或引入多模型动态上下文窗口前归档真实 route 校准证据；当前固定 4096 Token 编译预算与启动窗口门禁允许继续 Shadow 观察。
- **本轮进展：** Context Compiler 增加 provider-neutral `RouteTokenizerAdapter` 注入边界；未配置真实 tokenizer 时继续使用校准 UTF-8 fallback，跨 route 取保守最大估算，候选模型校准证据仍是生产接入前置条件。
- **本轮进展：** Model planner 对相互独立的已授权会话、Memory 和检索 evidence 读取采用并行 hydration，并以低敏数量 span 记录结果；任一读取异常仍在模型路由前失败，未扩张 retrieval、Memory 或跨会话访问边界。

### AD-030：TypeScript Agent 尚缺受认证的远程 Capability 传输

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `agent-runtime`、Core/Message/Conversation 边界、可信身份、只读 Step 执行
- **解决方式：** migration v21 增加与 Task 分离的 `agent_runs` 和 Step claim token/lease；受认证 `dipole-agent` 先通过 admission 固定 Task、Definition version 与 runtime Run，再以 Task/Run 调用 `conversation.list`。Core 服务端从持久 Task 解析 principal、permission 和 resource scope，拒绝 protobuf `RequestContext` 中的模型可控 principal。TS 使用 canonical proto 静态生成 grpc-js client，通过 Capability Registry 执行并持久化 Step result/error；Run completion 支持幂等网络重试。Agent mTLS 身份仅获 Admit/Complete/List 与 health 方法。
- **验证：** Go/TS Task/Run 黄金向量一致；伪造 principal、Runtime binding 和 Agent 调用其他 Core RPC 均被拒绝。真实 MySQL 8.4 覆盖 Run create/replay/CAS、Step 并发 claim、失败重领、旧 token 拒绝和完成 no-op；migration v21 完成 `up→down→up`。真实 Go Core 与 Node grpc-js 通过 loopback 共享密钥完成 admission/list/complete/replay，replay 返回同一 completed Run。
- **长期约束：** 当前远程能力保持只读 shadow，公开 HTTP 旁路继续禁止。新增 Capability 必须先完成 descriptor、服务端持久策略解析、最小 RPC allowlist、Step 轨迹和真实权限测试；write/destructive 能力等待 Approval 与 Temporal 状态机。

### AD-029：Agent 模型预算与调用轨迹尚未跨重试持久化

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `agent-runtime`、模型成本、Kafka retry、Run/Step 审计与故障恢复
- **解决方式：** migration v19 与 MySQL ModelAuditStore 以 Task 唯一 Run 固定预算快照，provider 调用前事务预留 slot，成功/失败写入 route、usage、finish reason、latency 与错误，Run 终止时将遗留 reservation 收敛为 `abandoned`。ModelRouter 已在每条 provider 路径调用 Store，持久写失败禁止 fallback；AI SDK 模式强制 MySQL Store并在 readiness 前探测 v19。无 slot 重试按 Task 条件收敛仍在 running 的 Run。
- **验证：** 真实 MySQL 8.4 连续三轮 16 路并发均只授予 3 个 slot；策略漂移、旧终态更新和越权 Core 表访问被拒绝。两个独立 Router 模拟同一 Kafka Task 重投时 provider 总调用固定为 2，第二次重投获得 0 slot，Run 保留 `calls_reserved=2` 并进入 failed。43 项常规 TS 和 5 项真实 Store 测试通过。
- **长期约束：** AI SDK 内部 retry 保持为 0；所有新增模型调用入口必须先预留持久 slot。Temporal 接入后复用同一 Task/Run，不另建可绕过预算的 Workflow retry 计数器；Tool/Approval/Artifact 使用独立 Step 轨迹扩展。

### AD-028：Agent Kafka 失败转移尚未接入 retry/DLQ

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `agent-runtime`、Kafka poison event、失败重试、offset 提交与故障恢复
- **解决方式：** Agent Runtime 使用 `<prefix>.<topic>`、`.retry`、`.dead` 三个显式 topic；无效 envelope 与 tombstone 直接进入 dead，处理错误按 `retry_attempt` 有界转移，达到上限后以 `handler_failed` 终止。转移保留原始 key/value/header，并增加 `original_topic`、`last_error`、`dead_reason` 和时间诊断。只有失败消息发布成功后 KafkaJS handler 才返回；publisher 异常向上抛出，保留源消息的未完成语义。启动时仅创建缺失 topic，并在 readiness 前验证分区数和副本数。
- **验证：** 31 项 TypeScript 测试覆盖永久失败、tombstone、重试上限、原始 metadata 和 publisher reject。真实 Kafka 3.9 验证 poison event 直达 dead，ledger 绑定冲突经过两次 retry 后以 `retry_attempt=2` 进入 dead；两副本加入/退出触发 rebalance 后 partition 4 均继续消费到 LAG 0。Compose 使用 6 分区和可配置副本数。
- **验证记录：** 2026-08-30 重新执行 `scripts/smoke-kafka-rebalance.sh`，真实验证双 consumer 成员、成员退出后的六分区接管和 lag 归零；临时集群自动清理，生产 offset、retry/DLQ 和 consumer group 配置保持不变。
- **验证记录：** 2026-08-30 重新执行 `scripts/smoke-kafka-observability.sh`，真实验证三节点 Kafka、Prometheus 规则、consumer lag、retry/DLQ、ISR 缺口和 broker 恢复；临时集群自动清理，生产 Kafka ownership、topic 和 consumer group 配置保持不变。
- **长期约束：** retry/dead topic 必须与主 topic 使用相同分区数和副本数；新增事件类型需先分类永久/瞬时错误。Temporal 接入后复用持久 Task ID 作为 Workflow ID，不另建重复幂等键。

### AD-026：Readiness 尚未持续感知运行期依赖退化

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** Core、Gateway、Message、Sync、Search、Search Indexer、Cassandra Projector 的流量摘除与故障诊断
- **解决方式：** 统一 metrics listener 增加异步缓存探针、逐探针超时和失败/恢复双阈值；服务生命周期状态与关键依赖状态共同决定 HTTP readiness 和 gRPC health。Core、Gateway、Message、Sync、Search、Search Indexer、Cassandra Projector 均按运行责任接入关键依赖，影子读、可回退存储、Kafka backlog 和可选能力继续使用专项指标。默认配置保持关闭，微服务 Compose 以 5 秒周期、1 秒超时、3 次失败、2 次恢复启用。
- **验证：** race 测试覆盖超时缓存、滞回、防排空反转和状态回调；gRPC health 回归验证 readiness 同步。Elasticsearch 隔离演练确认 Search 与 Search Indexer 退出并恢复 ready，Core、Message、Sync、Gateway 全程 ready，六个应用容器 ID 均未变化。Prometheus 规则测试覆盖具体 `service/dependency` 告警。
- **长期约束：** 新依赖只有在其故障会阻止服务正确处理当前职责时才加入 readiness；可选能力和有验证回退的存储继续通过有界指标告警。探针不得在 `/readyz` 请求路径执行网络 IO，新增探针需提供失败/恢复防抖与隔离故障演练。

### AD-019：MySQL 消息正文退役缺少完整替代读契约

- **本轮验证：** 真实隔离 Cassandra 读路由与 Sync hydration smoke 均通过主读、缺失/损坏回退和 Metadata 回填；证据仍属于隔离环境，未满足共享环境长期观测、责任人批准和兼容窗口退出条件。

- **本轮进展：** 真实隔离 MySQL/Cassandra smoke 已通过 hydration shadow、重复消息恢复、Legacy ID 恢复和 Metadata 回填；测试版本基线已修正至迁移 v47。共享环境主读灰度和旧 Offline 兼容窗口仍待完成。

- **本轮进展：** Gateway/WS 已接受 `message.timeline_notify_mode=primary` 并与 Web `VITE_TIMELINE_NOTIFY_MODE=primary` 对齐；通知仍只携带 locator，客户端验证完整序列后补拉。Cassandra 主读比例、共享环境 Prometheus 窗口和旧 Offline 兼容期仍未晋级，故该债务保持进行中。

- **本轮进展：** Web 已增加默认关闭的 `VITE_TIMELINE_NOTIFY_MODE=primary` 客户端路径，通知驱动的 Timeline 补拉会在序列和 UUID 完整校验后才合并消息；服务端 Cassandra 主读灰度、共享环境观测和旧 Offline 兼容窗口仍按既有门禁执行。

- **优先级：** P1
- **状态：** 处理中
- **发现日期：** 2026-08-27
- **影响范围：** Cassandra 主读、Sync Timeline、消息幂等、文件授权、搜索重建、迁移回放
- **现状：** `user_sync_inbox` 已持久化并对外暴露 `conversation_key + message_uuid + message_seq` locator。Sync Service 已建立 storage-neutral hydrator，可在返回 MySQL 正文的同时异步比较 Cassandra Timeline；Cassandra 尚未承担 Sync 主读。Direct 与 Group Timeline 均已具备 `after_seq` HTTP/Message v1 gRPC 增量契约，Local/Remote/Shadow adapters 一致，并复用 Cassandra cohort、连续页校验与 MySQL fallback。Gateway 已增加默认关闭的 `sync.item.notify.v1` body-free shadow 通知，Web verifier 会按会话补拉、去重并验证 locator；现有完整消息投递和热群聚合 notify + pull 保持不变。Web 已增加默认关闭的 IndexedDB Sync Engine、shadow 门禁和热群持久 ACK。migration v12 增加无正文 `message_metadata`，与 Message/Inbox/Outbox 原子提交并回填历史 locator；文件授权已改查 Metadata，删除完整 Message 行后仍可验证访问和过期时间。重复发送先通过 Metadata 校验身份，并可在默认关闭的开关下按会话 Seq 从 Cassandra 恢复原响应，缺失/冲突继续回退 MySQL。Cassandra Backfill/Reconciler 已支持经 SHA-256 校验的不可变完整消息归档，Job 绑定 source identity；真实演练删除 MySQL 正文后仍可恢复和全量对账。Message 最小账号暂时保留 `groups/group_members` 只读权限用于旧 Offline 与群文件授权。
- **风险：** 提前停止正文写入仍会让多端同步和重复发送响应缺失正文，并丢失 Cassandra 修复与回滚基准。文件授权的正文依赖已解除，但群文件授权仍需 Core 成员关系。
- **本轮进展：** Sync 新增默认关闭的 Cassandra-first hydration adapter；primary 与 shadow 配置互斥，primary 失败按同一 locator 回退 MySQL，取消或双失败均不返回部分正文。该路径已覆盖命中、回退、双失败和启动配置测试，尚未接入灰度比例、Prometheus 停止门禁或真实主读流量。
- **本轮进展：** 增加 Sync Cassandra hydration evidence v1 与 Go CLI，按窗口和部署 revision 绑定 shadow/primary 聚合指标，重算命中、fallback、缺失、冲突、错误与 p95 门禁。真实客户端流量、Prometheus 原始快照、责任人批准和主读回切证据仍缺失。
- **本轮进展：** 修正 migration integration test 从 v47 漂移到实际 v49 的基线与逐步回滚断言；重新执行隔离 Cassandra/MySQL hydration smoke，Metadata backfill、重复响应恢复、Legacy ID 恢复和 shadow comparison 均通过。该证据仍不替代共享环境主读窗口、责任人批准和可执行回切。
- **本轮进展：** 在同一 v49 隔离迁移环境重新执行 Cassandra read-routing smoke，Cassandra 页面读取、payload 损坏和缺失行回退 MySQL 均通过；生产主读比例、共享环境窗口和责任人批准保持未启用。
- **本轮进展：** storage-lab Compose 改用动态 Cassandra 宿主机端口，hydration 与 read-routing smoke 已并行通过，分别验证 hydration/Metadata 回填和 Cassandra 主读及损坏/缺失回退；临时资源自动清理，生产主读和共享环境证据门槛保持不变。
- **本轮进展：** 修复 `smoke-sync-cassandra-hydration.sh` 对已退役 `internal/service` 的路径漂移，改用 Message domain 服务目录；修复后真实隔离 hydration smoke 的 shadow comparison、重复响应恢复、Legacy ID 恢复和 Metadata backfill 均通过，生产主读和共享环境长期观测门禁保持不变。
- **本轮进展：** 2026-08-29 为 Sync 微服务 Compose 补齐 primary hydration、Cassandra enabled/hosts 的显式环境契约，并以 Compose gate 固定默认关闭与显式启用值；实际 Cassandra 主读、共享环境观测、责任人批准和可执行回切仍待完成。
- **本轮进展：** 增加显式 `cassandra-primary` Compose profile、Cassandra schema init 和 Sync `service_completed_successfully` 依赖，结构门禁验证 profile 只在显式启用时接线；真实消息 hydration、共享环境观测、责任人批准和可执行回切仍待完成。
- **本轮进展：** 增加可重复 `smoke-sync-cassandra-primary-compose.sh`，在临时容器网络中验证 Cassandra schema init、Sync primary 配置与 readiness，完成后自动清理 volume；真实 Inbox 消息 hydration、共享环境观测、责任人批准和可执行回切仍待完成。
- **本轮进展：** 2026-08-29 修正 migration integration v50 基线与 Metadata 测试回退步数，重新通过隔离 hydration smoke；v12 legacy-message backfill、重复响应恢复和 Legacy ID 恢复证据已闭合，生产主读与共享环境窗口仍未启用。
- **本轮进展：** Web Sync Observation 工具对 `start/status/finalize` 增加 5 分钟未来时间偏差门禁，并以可注入时钟测试开始、状态和结束三个入口；防止 Prometheus 未来时间查询伪造完整观察窗口，真实客户端观察、责任人批准和主读切换条件保持不变。
- **建议方向：** A5 Search 与 A4 Cassandra 均已具备不可变归档恢复源；重复发送 hydration 与 Timeline notification shadow 均已具备严格 24 小时晋级规则。Web Sync 观察现可用候选 commit/bundle 哈希绑定的 Session/Evidence 归档，仍需在完整服务 Prometheus 和真实客户端流量上运行并固定对象版本。随后继续通知 shadow 证据归档、Sync Cassandra hydration 主读/fallback 和重复发送 hydration 灰度，再引入 `full / metadata_only` 写模式。
- **处理门槛：** 完成固定快照备份与校验、事件回放演练、Sync/Offline 比较、幂等和文件授权契约、至少一个兼容窗口的 Cassandra 稳定主读，并记录可执行回滚期限与责任人；旧 Offline 退役后撤销 Message 对 `groups/group_members` 的临时读取。

### AD-021：Search 重建依赖 Outbox 事件保留契约

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **影响范围：** Elasticsearch 全量重建、事件归档、Outbox 清理、MySQL 消息正文退役
- **现状：** `dipole-search-archive` 可按固定 Outbox mutation 高水位流式导出最终状态 NDJSON 与 SHA-256 manifest，并发布到独立 MinIO object-lock bucket。`dipole-search-outbox-cleanup` 只接受可按精确对象版本恢复的 receipt、已完成且一致的 Reconcile 报告和匹配的 Backfill Job；默认 dry-run，执行时强制维护窗口确认与 operator。sqlc 查询仅删除水位内、已发布的八类 Search mutation，遇到未发布事件时拒绝清理。
- **解决记录：** 2026-08-27 完成专用 `search.mysql.*` 配置和最小授权模板；单测验证批次中断后可重入。真实 MySQL/MinIO/Elasticsearch 演练按 2/2/1 删除 5 条 eligible mutation，保留无关 Outbox，维护账号访问 Core 表被拒绝；随后仅凭保留对象版本从空索引恢复并完成 3/3 hash 对账、Alias 正向切换与回滚。
- **长期约束：** 禁止手工批量删除 Outbox。每次执行必须保存 operator、snapshot/object version、Reconcile 时间、高水位和删除统计；对象保留期、清理窗口或 mutation 类型变化时重新评审本条契约。

### AD-017：Redis Pub/Sub 切主窗口保持 at-most-once 语义

- **本轮验证：** Redis Sentinel 真实三节点故障演练已验证 master 切换和 replica 重加入期间的客户端恢复、Presence、Hot Group 与限流语义；Pub/Sub 在切主瞬间的已发布消息仍无法补读，持久可靠性继续由 Kafka/Sync Timeline 承担。
- **验证记录：** 2026-08-30 重新执行 `scripts/smoke-redis-failover.sh`，真实验证三节点 Sentinel 切主、客户端重连、Pub/Sub、Presence、Hot Group、限流语义恢复，以及原主节点重启后重新加入为副本；该证据来自隔离栈，生产 Redis 配置和切换策略保持不变。
- **追加验证：** 2026-08-29 修正 smoke 构建入口至 `internal/platform/cache` 后重新完成三 Redis + 三 Sentinel 演练；当前 master 停止、新 master 发现、Pub/Sub 重连、Presence/Hot Group/限流恢复及旧 master 以 replica 重加入均通过。Pub/Sub at-most-once 边界保持不变。

- **优先级：** P2
- **状态：** 接受风险
- **发现日期：** 2026-08-26
- **影响范围：** Gateway 跨节点在线投递、Redis Sentinel 故障转移、后续 C++ Realtime Delivery
- **现状：** go-redis 会在 Sentinel 选出新 master 后重连命令与 Pub/Sub 连接；连接中断期间已经发布的 Pub/Sub 消息无法补读。Gateway 的 Kafka handler 当前将跨节点 Pub/Sub 视为实时通知通道。
- **风险：** master 切换窗口内，在线用户可能暂时缺少一条跨节点通知；Redis Sentinel 无法提供持久队列或消费位点。
- **接受依据：** 消息事实、用户 Inbox、设备 Cursor 和热群 checkpoint 均保存在 MySQL/Kafka 链路，客户端重连或增量同步能够恢复已确认消息；Redis 只承担实时状态。
- **阶段记录：** 2026-08-28 已建立 `dipole.delivery.v1` envelope、节点批次、逐项 ACK/error 与背压契约，并固定 Kafka source coordinates 和 Go legacy adapter；C++ shadow 已接入独立 Kafka group、hiredis direct/Sentinel reader、单连接 TTL 投影、低敏 evidence v3、mTLS `ObserveNodeBatch` 和 assignment readiness。真实 Kafka+Redis+Gateway 演练覆盖故障保留 offset、同进程恢复重试、稳定 batch 去重、真实 queue saturation/backpressure、同 workload Go/C++ 40/40 对照与最终 lag 归零。`AD-039` 已关闭。默认关闭的 primary seam 提供 connection 定向入队、逐项 ACK、部分成功 connection 重试、有界 Gateway replay state 与 additive WebSocket delivery ID；Web 通过账户隔离的 IndexedDB v4 原子 claim 跨页面重载去重。C++ one-shot probe 经 mTLS 实际验证 `ENQUEUED(1)`、稳定重放去重与 stale Presence `OFFLINE`。显式 primary CLI 现使用独立 `dipole-realtime-primary-*` authority，要求 enable/Presence/transport 三重配置，并将 terminal ACK、低敏 primary evidence 与 Kafka commit 串联；partial/rejected/failed、身份漂移和故障保留同一 pending record。默认关闭的 `realtime-cpp` Compose profile 现提供独立 primary 部署描述，但 Go authority、Gateway primary RPC 和共享环境切换仍保持关闭，必须经过 C3 证据与维护窗口。
- **验证记录：** 2026-08-29 修正 Redis Sentinel smoke 的静态测试构建路径，从已迁移的 `internal/store` 兼容目录切换到 `internal/platform/cache` ownership 包；后续故障切换演练需以该入口验证。
- **后续方向：** `benchmarks/c2-primary-runtime-2026-08-28/` 已验证真实 queue saturation、terminal ACK 后 commit、故障 retain、`SIGKILL` 后同坐标重放和 lag 归零；窄 terminal-evidence-to-commit 崩溃窗口仍未作确定性声明。C3 由 `AD-041` 继续跟踪互斥 authority 与自动回切。IndexedDB 不可用时 Web 保持 fail-open，持久记录按 4096 项容量淘汰；保留 Sync Timeline 作为存储故障、去重窗口外重放和进程崩溃窗口的最终补偿路径。
- **重新评估门槛：** 产品要求在线 push 本身具备不丢 SLA，或 Kafka consumer 在 Pub/Sub 发布失败后仍提交 offset 造成可观测缺口时。

### AD-015：Message Service 数据库账号尚未收敛表级权限

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `cmd/services/message`、File metadata、数据表所有权、最小权限
- **解决方式：** 增加继承全局配置的 `message.mysql.*` 专用凭据，独立 Runtime 不再读取 Core MySQL 凭据。`dipole_message` atomic 与 `dipole_message_projector` 两套 GRANT 仅开放 sqlc 实际使用的操作；启动探针逐项验证必要 SELECT/INSERT/UPDATE、拒绝多余 DELETE/UPDATE、Core 表访问和 projector Inbox 访问。微服务 Compose 在 migration 后创建账号，并默认启用 Message/Sync 权限门禁。
- **验证：** 真实 MySQL 8.4 smoke 验证 atomic 提交 Message/Metadata/Outbox/Inbox、projector 提交 Message/Metadata/Outbox 且 Inbox 为零，并拒绝 Core 和多余写权限；完整微服务镜像/Compose smoke 验证权限初始化、Message/Sync 健康启动、mTLS、Gateway/Core 路由。
- **长期约束：** `/messages/offline` 兼容期内保留 `groups/group_members` SELECT；旧接口退役后按 AD-019 撤销。新增 Message sqlc 写操作必须同步更新 GRANT、操作级探针和真实权限 smoke。

### AD-005：群消息成员级写扩散仍然叠加

- **优先级：** P2
- **状态：** 处理中
- **发现日期：** 2026-08-26
- **影响范围：** 普通群 Inbox、Conversation State、热群吞吐
- **现状：** 普通群同时按成员更新 Conversation State 和 Inbox；热群仅跳过 Inbox，Conversation State 仍逐成员更新。
- **风险：** 两类投影职责独立，但成员级写入量会叠加，热群链路仍保留 `O(group_size)` 的会话状态写扩散。
- **基线证据：** 候选提交 `2202f1f` 的 baseline v2 证明 Conversation 写放大在 20/100 人普通与热群中均为 20x/100x，见 `benchmarks/ad005-2026-08-27/`。提交 `4343684` 的 baseline v3 进一步记录逐节点 Repository timing：group-message 单次平均为 12.43-23.07 ms，P95 桶上界为 25-50 ms，四组零错误；普通与热群的调用分布接近，而 100 人端到端 P95 分别为 8189 ms 与 1346 ms，支持 Inbox/投递路径是模式差异的重要来源。完整原始快照见 `benchmarks/ad005-projection-timing-2026-08-27/`。
- **建议方向：** 保留现有 Counter/Histogram 作为回归门禁；在 1000 人固定 workload 或候选实现中比较逐成员串行写、批量 upsert、异步分层投影与热群摘要读扩散，单独记录数据库累计时间、锁等待和投影恢复语义。
- **处理门槛：** 候选优化需在固定 workload 下减少 Conversation 累计写成本或端到端 P95，保持 Seq/read state、投影重放和回滚正确性，并通过普通/热群完整投递对照后才能关闭；当前 v3 证据已完成归因基线，尚未完成行为优化。
- **本轮进展：** 新增 sqlc `INSERT ... SELECT` 批量 upsert seam，服务层仅在 Repository 声明支持时启用，旧实现继续逐成员写入；真实 MySQL 8.4 contract 已验证 sender/recipient Seq、未读计算和重复写入幂等，锁等待与 1000 人 workload 对照仍待完成。
- **本轮进展：** 在提交 `4ac1540` 上完成真实 MySQL 8.4.8、1000 成员的 serial/batch 及并发对照；batch 数据库层耗时约降低 37.3-353.8 倍，四组投影行数校验通过，`Innodb_row_lock_waits`/`Innodb_row_lock_time` 增量均为零，证据归档于 `benchmarks/ad005-conversation-batch-2026-08-29/`。端到端 P95、多轮统计和共享拓扑容量验证仍待完成。

### AD-007：架构 Markdown 当前未纳入版本控制

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `docs/*.md`、架构决策可追溯性
- **解决方式：** 移除 `docs/*.md` 通配忽略，以 `docs/architecture-docs.manifest` 固定 canonical 文档集合；架构、Agent、数据、运行、性能和指南文档按职责归档，参考材料纳入 `docs/architecture/architecture-reference.md`。
- **验证：** `scripts/check-architecture-docs.sh` 校验清单文件存在且已被 Git 跟踪，拒绝通配忽略回归，并验证根目录 Markdown 仅保留项目入口、更新日志和协作约定。
- **长期约束：** 新增长期架构约束时同步更新 manifest、实现文档和更新日志；本地草稿晋级前先完成代码与配置对齐。

### AD-008：Agent Tool 允许模型提供用户身份参数

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `internal/services/agent/legacy/tools.go`、会话读取、用户资料、系统消息发送
- **解决方式：** Embedded Go/Eino Service 从已校验的触发 Message 与关联上下文生成 `ExecutionContext`，注入 principal、Agent、触发消息、会话和 request/trace/event ID。五个 Tool Schema 均移除 `user_uuid`，读取和系统消息目标只使用上下文 principal；上下文缺失或发送 Agent 不匹配时 fail closed。
- **验证：** `dipole.agent.eval.v1` 保留两条恶意 `U999` 覆盖用例，结果改为 `identity.execution_context` 与 `principal_enforced`；单元测试覆盖全部 Tool 缺少上下文拒绝、schema 身份字段扫描、发送 Agent 不匹配和 Service 派生链。
- **后续边界：** tenant、委托身份和细粒度权限继续由 G1 Capability API 承担，不能重新加入模型可控身份参数。

### AD-009：Agent 持久任务生命周期尚未完成生产接管

- **优先级：** P2
- **状态：** 处理中
- **2026-08-30 验证：** 在 `master` 当前基线复跑 Go 全仓、服务布局、Compose 契约和 Agent Runtime 独立门禁；Vitest 为 125 个文件/665 个测试通过，`typecheck`、生产构建和 active overlay Compose 契约通过。该证据确认当前代码边界稳定，仍不覆盖共享 Temporal、真实 Kafka、active authority 和生产故障回切。
- **发现日期：** 2026-08-26
- **影响范围：** `agent-runtime`、Temporal、长任务、审批、失败恢复和评测
- **现状：** migration v16-v29 已落地 Definition、Task、独立 Runtime Run、可重放模型输出/预算、不可变 Plan/Context manifest、带 lease 的 Step 终态、附加 Workflow projection、版本化 Artifact、Subscription 与 scoped Memory。Temporal Workflow 已持久化 Task/Run admission、三类 Run 终态、Approval/Input Signal 和 deadline Timer；默认关闭的 `read_shadow` 由 Kafka 启动稳定 Workflow，并在 Activity 内执行 ContextCompiler、ModelRouter、只读 Capability Step 和内容寻址 Artifact 创建。Message v1 Envelope 已通过可选 lineage传播根 Task，TS Runtime 在高成本处理前阻断同源 Agent 因果链。Gateway 已提供默认关闭的 JWT Task query/cancel/approval/input API；repair 审计 RPC 只接受 Gateway principal。离线对账与 Shadow 晋级保持只生成证据和 eligible/blocked 决策。Compose 继续关闭 Temporal、Task 控制桥并固定 `foundation`。
- **本轮进展：** MySQL migration integration baseline 已更新至 v44，并覆盖 execution ledger、lineage backfill、pre-model lineage 的连续回滚与表数量校验；本轮验证消除了迁移测试落后于实际 schema 的盲区。
- **本轮进展：** Repository 合同测试在隔离 MySQL 8.4.8 中验证 v44 prepared execution 的创建、精确重放和目标哈希冲突拒绝；没有增加状态推进、Projection 写入或公开执行入口。
- **本轮进展：** Shadow Eval observation 已区分策略 Task 与 Durable Workflow：常规 Task 仍要求 `agent_tasks.status` 终态，read-shadow 仅在 CAS `workflow_status` 和对应 Run 均终态时可生成五类报告。该收口避免将 Shadow 的运行中策略记录误判为不完整或以非终态 Workflow 伪造成功率；真实人工标注窗口与共享环境证据继续待补。
- **本轮进展：** 真实 read-shadow 观察发现默认 policy 的 `conversation/*` scope 无法由旧 Eval manifest 表达。契约现将 `permission.resourceId` 限定为稳定标识符或唯一 `*`，保留实际授权范围供评测绑定；它不产生新授权，真实人工标注窗口与共享环境样本仍待归档。
- **本轮修正：** Remote GPU disposable read-shadow 的 `N=1` 五类 Eval 报告已撤回。复核确认 Step 只持久化 capability/status，Permission 的 resource scope 尚未由数据库 observation 证明，不能将 manifest 标签视为 Runtime 授权证据。当前只保留 Kafka/Temporal/MySQL/Go Core 拓扑 smoke 结论；后续必须在 Step lease 内持久化 resolved resource 与 policy decision、由 Eval 精确比对并重跑，才可重新归档五类报告。
- **本轮进展：** 增加受控 `prepared` 准备服务：复核已批准且未过期的 repair proposal、审批计数、Task 存在性和 proposal/task/executor 绑定，再通过 execution store 幂等创建并读取执行意图；该服务不推进状态、不修改 projection。由于 operator grant 当前没有版本字段，`executor_grant_version` 暂只保存计划绑定，运行时 grant 复核仍关闭。
- **本轮进展：** Agent Runtime 增加 `repair:plan` dry-run 计划编译器，按 execution-plan v1 生成确定性的 plan ID、当前/目标/回滚投影 SHA-256 和 15 分钟 CAS 窗口；批准状态、双审批人、独立执行人及 grant version 均在计划生成前校验。CLI 不连接 MySQL/Temporal，不改变 projection，也没有 apply/execute/rollback 入口。单测、类型检查和构建已通过。
- **本轮进展：** 追加 Workflow/Run 身份绑定校验，当前投影与目标投影必须属于同一运行实例，跨运行证据在 plan 编译阶段拒绝；新增回归测试并保持 v1 dry-run 与无写执行器边界。
- **验证记录：** TS Agent Runtime 独立执行 `npm test -- --run` 通过（125 个测试文件、661 个测试），`npm run typecheck` 与 `npm run build` 通过；当前 Compose 仍固定 shadow/metadata/foundation，尚未宣称生产接管。
- **制品验证：** `services/agent-runtime/Dockerfile` 真实构建成功，生产镜像以 `node` 用户启动；foundation 配置下容器 `/readyz` 返回 200，Kafka/RPC 关闭时无外部副作用。该证据覆盖独立交付，不替代 active Runtime 的 Temporal、Capability、真实 Kafka 和共享环境切换门槛。
- **门禁固化：** 新增 `scripts/check-agent-runtime-container.sh`，构建时绑定 OCI revision/created/dirty provenance，并自动验证镜像 provenance、非 root 用户和 foundation readiness；默认不改变 shadow/metadata/foundation 回滚配置。
- **本轮进展：** 增加 `repair:preflight` 二次采证器，按 plan/proposal/grant/current CAS 生成低敏 `ready|blocked` 收据；它不读取数据库、不调用 Temporal、不修改 projection，真实 executor 与生产 authority 继续关闭。
- **本轮进展：** migration v44 与 sqlc 新增 prepared execution ledger，持久化唯一 plan 的执行意图、提案/任务/执行人绑定和 CAS 摘要；应用接口仅支持创建/读取 prepared 记录，未增加状态推进或写入 RPC，便于后续 executor 在独立版本中实现可恢复提交与回滚。
- **本轮进展：** migration v50 为 Workflow Repair operator grant 增加 `grant_version` 与独立 `can_execute` 能力；旧授权默认保持提案/审批权限，执行器必须绑定非零版本并单独授予执行权，避免仅凭旧 `executor_grant_version` 进入写路径。
- **本轮进展：** 增加跨 Go/TypeScript 对齐的 projection hash precondition guard，在任何未来 mutation 前校验 active executor grant、版本、Task 绑定以及当前/目标 projection 摘要；该 guard 只读且无副作用，继续保留 production executor 关闭。
- **本轮进展：** 增加 transactional projection commit primitive，使用同一 MySQL transaction 绑定 projection CAS 与 execution `committed`；CAS 或 execution 条件不满足时事务回滚，rollback projection 与生产装配仍待完成。
- **本轮进展：** 增加 transactional rollback primitive，按当前 target projection CAS 恢复旧 projection 或清空 projection，并原子标记 `rolled_back`；真实 executor 装配、操作员再次授权和共享环境演练仍待完成。
- **本轮进展：** 增加默认未接线的应用层 `PersistentAgentWorkflowRepairExecutorV1`，执行前 fresh-read execution、Task projection 和带版本的 active `can_execute` grant，执行 claim 后通过跨 Go/TypeScript canonical hash precondition，再调用事务 commit；失败固定落为 `failed`。rollback 仅接受原执行人、fresh grant、已提交 execution 和匹配的 target/rollback hash，仍未接入公开 RPC、生产启动链或共享环境。
- **本轮进展：** Executor 在 claim 前进一步要求 Execute 请求携带与 execution ledger 一致的 rollback projection；缺失、非法、Task 不匹配或 SHA-256 漂移均 fail closed，避免准备记录与实际可回滚载荷分离。
- **本轮进展：** 增加 Gateway-only 的 Execute/Rollback gRPC 契约与可选执行器注入点；未配置执行器时明确返回 `Unavailable`，公开控制面保持默认关闭，先完成协议和认证回归再推进生产启动接线。
- **本轮进展：** 已核实 operator grant 版本化与 CAS executor 实现：migration v50 增加 `grant_version`/`can_execute`，执行器和事务 commit/rollback 均复核 fresh grant、执行绑定与 projection hash；Go 执行器、Repository 和 Agent Runtime 相关测试通过。公开控制面、生产启动链和共享环境故障演练仍为 AD-009 未完成门禁。
- **本轮进展：** read Activity 已从单步扩展为「发现 + owner 确认读取」两步，首次让持久 `waiting_input` 出现在真实读取路径上：暂停发生在 claim 读取 Step 之前，恢复由确定性 request ID、checkpoint 候选集合与 Core 授权三重约束。到期沿用既有 `input_expired` 定时器，无新增状态。Remote GPU 候选 `aec1b867` 已归档由生产 read Activity 驱动的 approve/deny/expire 三份 receipt：确认路径在伪造 request 被拒绝后只读取被确认的会话，拒绝与到期路径的会话读取计数为零。共享环境窗口、该路径的 outcome/trajectory/permission 评测和多于一对 discovery 的编排仍未完成。
- **本轮进展：** `interactive_active` 已在干净同版本、loopback-only 的 Remote GPU Compose 内完成真实 Core/Message/Temporal 回执：owner deny 重放无 Tool/Message，owner approve 重放精确收敛为一次 approval consume、一次完成 Tool/action reference 与一条 Message。测试开发 grant 已撤销，完整范围与排除项见 [Interactive Active Remote Receipt](../agent/AGENT-INTERACTIVE-ACTIVE-REMOTE-RECEIPT.md)。这不替代 Shared 环境、浏览器 HITL、故障恢复、部分副作用回滚、MCP、性能和成功率门禁。
- **风险：** v24 projection 保持 shadow 观察属性，尚未接管原 `agent_tasks.status`；当前 `read_shadow` 只允许 `conversation.list`，也没有 Memory 或真实任务终态 outcome Eval。v25 的 `approved` 只保存审计结论；execution plan v1 仍只允许带 CAS/回滚证据的 dry-run，应用 Executor 已具备 commit/rollback 语义但尚未接入公开控制面、生产启动链和共享环境。操作员授权仍需要受控 SQL 配置，Temporal Worker 停止时 Query 会归类为 unavailable。eligible 决策不能自动切换 active。
- **基线证据：** 真实 Temporal Server 已验证 admission/Approval 历史恢复、单调 revision 投影、取消投影、完成态 Query/Describe 对账和 Activity 丢失完成 ACK 后的模型/Step 重放；真实 MySQL 8.4 已验证 v25 全链升降级、16 路同审批人重放仅一票、两位独立审批后批准，以及原 projection 并发与 shadow cohort keyset 契约。TypeScript/Go canonical evidence SHA-256 使用黄金向量对齐；gRPC 测试验证 Gateway principal 绑定和 Agent 最小权限拒绝。Kafka Shadow 与 Go/Eino 权威业务路径保持不变。
- **建议方向：** canonical Pencil 已维护 Repair evidence review、六态审计矩阵和 desktop/mobile 双人审批边界；下一步为 Executor 增加公开控制面前的 operator 再授权、共享环境故障注入和审计 receipt，再按该契约实现 Vue 恢复界面。完成真实 outcome/trajectory/permission Eval 证据后才评审权威 Task 与回复流量迁移。
- **处理门槛：** 上线 Durable Task 或 Event-driven Agent 前完成。

## 已关闭

### AD-011：前端缺少可版本化的完整设计基线

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-28
- **影响范围：** `frontend`、响应式布局、Agent UI、视觉一致性
- **解决方案：** 建立单一 canonical `design/dipole-ui.pen`、统一设计 token、可复用组件、Login/Chat desktop/mobile、Search 四态、Sync 恢复状态、Agent Workflow Repair 审计状态、关键异常状态、设计日志和评审导出图。
- **验证：** pen.dev CLI 识别 35 个顶层 frame 和 16 个可复用组件；相较关闭时的 23/10 基线，已增量加入 Repair 与 Elicitation 各三个组件和三张画板。结构检查保持零 placeholder、无新增未命名节点、无裁剪或布局告警；Login、Chat、Search、Sync、Repair、Elicitation 代表画板均完成渲染复核。
- **后续范围：** Vue token 映射和自动视觉回归继续由 F4 跟踪，不再阻塞 F1 设计基线关闭。
- **本轮进展：** F4 已新增 `frontend/src/styles/design-tokens.css`，并让 App 壳层与 Search 工作区引用 `--dp-*` token；Vitest 直接读取 `design/dipole-ui.pen` 的 variables 校验颜色、字体、间距和圆角，后续 token 漂移会在测试阶段暴露。页面流程、其余组件迁移和截图视觉回归仍待完成。
- **本轮进展：** Agent Task Timeline 已迁移至共享 `--dp-*` token，并用组件契约测试锁定核心颜色、字体和表面样式；其余 Agent/IM 页面迁移及截图视觉回归仍待完成。
- **本轮进展：** Login 页面已迁移至共享 `--dp-*` token，并增加源码契约测试；其余 Agent/IM 页面迁移及截图视觉回归仍待完成。
- **本轮进展：** Agent Task Timeline 路由页面外壳已与组件共同使用共享 `--dp-*` token；其余 Agent/IM 页面迁移及截图视觉回归仍待完成。
- **本轮进展：** Agent Event Subscription 管理页已接入共享 `--dp-*` 主题 token，并以组件测试锁定 token 映射；其余 Agent/IM 页面迁移及截图视觉回归仍待完成。
- **本轮进展：** Agent Memory 管理页已通过后置主题覆盖接入共享 `--dp-*` token，兼容保留现有状态结构；其余 Agent/IM 页面迁移及截图视觉回归仍待完成。
- **本轮进展：** 2026-08-29 Agent Runtime 与 Frontend 测试、类型检查和生产构建通过，嵌入式 `internal/services/core/server/webapp` 已同步到同一构建产物；完整页面视觉回归仍由 F4 跟踪。
- **本轮进展：** Agent Task Timeline 已增加 Playwright authenticated mock 流程，固定路由 flag、Bearer 请求、cursor 参数和低敏展示边界；截图级跨浏览器视觉回归仍待完成。
- **本轮进展：** Agent Approval 与 Elicitation 表单已迁移到共享 `--dp-*` token，并以源码契约测试锁定画布、表面、字体和状态色边界；截图级视觉回归仍待完成。
- **本轮进展：** Agent Approval 页面已增加 Playwright authenticated mock 流程，固定审批请求体、Bearer、失败隐藏和移动端布局边界；截图级视觉回归仍待完成。
- **本轮进展：** Agent Approval 与 Elicitation 已增加 Chromium canonical 截图回归，固定主要桌面布局并保留三浏览器功能验收；真实 Pencil 增量编辑和其他页面视觉基线仍待完成。
- **本轮进展：** Agent Subscription 与 Memory 管理页已增加 Chromium canonical 截图回归，覆盖治理控制面共享 token；其余页面和真实 Pencil 增量编辑仍待完成。
- **本轮进展：** Search Workspace 已清理残留主题硬编码并统一使用共享 `--dp-*` token，设计契约测试覆盖搜索、错误和骨架状态；截图级 Search 视觉回归仍待完成。
- **本轮进展：** Search Workspace 已建立仅供 E2E 的五态 visual harness 和 Chromium canonical 截图基线，覆盖 Idle、Loading、Results、Empty、Error；真实 Pencil 增量编辑和跨平台截图差异仍待完成。
- **本轮进展：** F2 Group Directory 已将 Pencil desktop/mobile/state matrix 和三个复用组件映射为受认证 `/groups` 只读页面。客户端严格验证会话范围和群详情的公开投影，任一读取异常即清空旧目录；Remote GPU Node 22 的 36 个前端测试文件、152 项测试、typecheck 和 production build 已通过。热群仅显示 `notify + pull` 状态，成员和群管理写操作继续保留在既有聊天入口，跨浏览器视觉回归仍待独立切片。

### AD-041：Go 与 C++ Realtime Delivery 缺少互斥切流 authority

- **优先级：** P0
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **完成日期：** 2026-08-28
- **解决方式：** 建立默认 Go 的 `go|shadow|cpp` 本地 authority、跨语言 Redis epoch lease 与 fail-closed reader、短 TTL 节点 observation、双 Kafka group 零 lag checkpoint、不可变 attempt workspace、哈希链 journal、幂等 action artifact 与 production executor。`dipole-realtime-cutover run` 在单一同步循环中统一 advance、条件续租、冻结超时回切和阻塞重试，并以 attempt-scoped Redis owner token 排除并发 controller；回切必须先确认 source nodes，且 `rollback_requested` 续租保留回切意图。
- **验证：** 隔离证据覆盖 Go/C++ 各一条客户端 frame、跨客户端 checkpoint、controller artifact 崩溃恢复、Redis outage、Kafka member loss、500 ms expired-freeze 回切、真实 C++ Primary lease/observation/assignment/readiness，以及 Controller A 无 release 进程退出后 B 在 5 秒 TTL 前被拒、到期后从同一 journal 完成。证据归档于 `benchmarks/c3-delivery-authority-2026-08-28/`、`benchmarks/c3-cutover-checkpoint-2026-08-28/`、`benchmarks/c3-cutover-faults-2026-08-28/`、`benchmarks/c3-cutover-cpp-primary-2026-08-28/` 与 `benchmarks/c3-cutover-controller-2026-08-28/`。
- **追加验证：** 2026-08-29 使用当前分支重新运行 C3 真实故障演练，C++ build/CTest 14/14、对比测试 5/5、Controller/C++ 进程替换、Redis outage、Kafka rebalance、过期 freeze 自动回切和 primary 停止恢复全部通过；生产部署仍保持默认 Go authority。
- **性能门禁：** 2026-08-29 对同一 direct created v1 事件执行 100,000 次 Go/C++ JSON 解码与投影，结果计数一致但 C++/Go ops ratio 约为 `0.10`，低于 `1.0` 晋级门槛；当前保留 Go projection，C++ 仅保留故障隔离、authority 和后续连接/批处理数据面评估边界。
- **追加验证：** 当前候选 revision `c063594` 重新执行 100,000 次固定 workload，C++/Go ops ratio 为 `0.0976283897`，结果计数一致且仍为 `blocked`；证据归档于 `benchmarks/c2-cpp-projection-benchmark-2026-08-29-rerun/`，继续保留 Go projection。
- **追加验证：** 当前 `master` 基线 `92d0c58` 在 Ubuntu 24.04 容器中完成依赖安装、CMake Release 构建和 14/14 CTest，镜像 provenance 为 `dirty=false`；Go/C++ projection 性能对照仍低于晋级阈值，C++ 继续保持候选数据面，不改变默认 Go authority。
- **构建验证：** 2026-08-29 使用 `services/realtime-delivery/Dockerfile` 在 Ubuntu 24.04 隔离环境完成 C++ 依赖安装、镜像构建和 14/14 CTest；宿主机缺少 `grpc++ >= 1.51` 仅影响本地直接运行，不改变源码门禁结论。
- **验证入口：** 已增加 `scripts/check-cpp-realtime-container.sh`，统一复用上述 Dockerfile 并绑定 revision、created、dirty provenance；容器门禁通过不会改变默认 Go authority 或放宽 C3 灰度条件。
- **兼容说明：** tracked deployment 继续默认 Go；关闭该债务只表示 C3 切流协议与回切证据门槛完成，启用 C++ authority 仍需要独立的灰度发布决策和显式配置。
- **状态边界复核：** 2026-08-30 计划审计确认 C3 故障注入、自动回切和 authority 证据已完成；性能收益门禁与按节点/用户灰度仍保持未完成，主计划应标记为进行中。
- **灰度前置：** 2026-08-30 增加纯策略 `RolloutPolicy`，以 `node/user` 作用域、稳定盐值和百分比生成确定性 authority 选择；默认回到 Go，异常输入 fail closed。策略尚未接入 Gateway，等待性能收益与完整灰度观察证据。
- **边界修复：** 2026-08-30 修复灰度百分比 `101..255` 未被拒绝的问题，并以回归测试锁定 `0..100` 合法范围；策略仍未接入 Gateway。

### AD-039：Gateway Kafka assignment 未纳入 readiness

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-28
- **解决日期：** 2026-08-28
- **影响范围：** Go Gateway 实时投递、空 Kafka 冷启动、基准门禁、后续 C++ Realtime Delivery 切流
- **解决方式：** Kafka Consumer 通过 coordinator `DescribeGroups` 聚合整个消费组的 assignment；Gateway 新增要求首次成功的 `kafka-assignment` 探针，只有 group 为 `Stable` 且每个已注册 base/retry topic 至少拥有一个分区时才通过。通用 readiness 状态机新增 opt-in 初始失败语义，其他探针继续沿用兼容默认值。微服务 smoke 同时要求 `/readyz` 和 assignment 指标通过，并使用可覆盖的临时证书目录保持演练隔离。
- **验证：** clean revision `958d40c7910ca8a85c0dad6bf57698ae32f9d42f` 镜像来源为 dirty=false；独立栈停止 Gateway 后消费组为 `Empty 0`，重启首样本为 `/readyz=not-ready`、`service_ready=0`、assignment=0，32 个样本约 10.2 秒后达到 `Stable 20`、`/readyz=ready`、assignment=1。完整依赖 readiness smoke、聚焦测试和 canonical Go gate 通过，证据归档于 `benchmarks/c2-gateway-assignment-readiness-2026-08-28/`。
- **保留边界：** 当前探针验证 group 级稳定状态和注册 topic 覆盖；运行期短暂 rebalance 继续由既有失败阈值吸收，长期 reader 无进展仍需结合 fetch/commit/lag 指标独立告警。

### AD-022：前端开发工具链仍停留在 Vite 5

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** 前端开发服务器、Vite、Vitest、Rolldown、依赖审计
- **解决方式：** 前端升级到 Vite 8.2.2、Vitest 4.1.11 和 plugin-vue 6.0.8，固定 Node 22.12+ LTS；Vite 配置使用 `import.meta.dirname` 兼容 native config loader，并允许测试环境覆盖代理目标。旧 Vite/esbuild 开发链由 Vite 8/Rolldown 取代。
- **验证：** Node 22.12.0 干净容器完成 `npm ci`、3 项工具链契约、53 项单测、生产构建和完整/生产依赖零漏洞审计；真实 HTTP/WS 代理及 `/app/` 资源路径通过。Chromium、Firefox、WebKit 的全部适用 Playwright 场景通过，平台专属场景按既有条件跳过。
- **长期约束：** Node 最低版本、Vite/plugin-vue/Vitest peer 范围和 `.nvmrc` 同步维护；工具链主版本升级必须重新运行代理、base path、三浏览器和 audit 门禁。

### AD-006：消息仓储保留未使用的兼容包装

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `MessageRepository` API、消息事务测试替身
- **解决方式：** 全仓调用审计确认生产 Repository 和 `application.MessageStore` 已只保留 `CreateWithSync`、`StoreWithOutboxAndSync` 两个显式写入口；删除测试 stub 中残留的 `Create`、`StoreWithOutbox`，并增加方法集回归测试阻止兼容入口回流。
- **验证：** Repository/Message Service 测试和全仓方法定义扫描通过；现有消息发送、Outbox、Inbox 与 projector/atomic 语义未改变。
- **长期约束：** 新增 Message 写入口必须显式描述 Message、Metadata、Conversation Seq、Outbox 和可选 Inbox 的原子边界，不能通过无 Sync 语义的兼容包装进入生产端口。

### AD-012：用户状态常量与 schema 默认值偏移

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `model.User`、`users.status`、跨语言状态契约
- **解决方式：** `UserStatusNormal=1`、`UserStatusDisabled=2` 改为显式常量，并新增语言中立 `dipole.user.status.v1`；migration v27 将历史 `status=0` 归一为 `1`、修改默认值并通过 CHECK 约束拒绝领域外状态。
- **验证：** 契约测试固定 Schema ID、版本、默认值、枚举与 Go 常量一致；真实 MySQL 8.4 升降级测试覆盖历史回填、默认写入、非法值拒绝，以及 Down 保留已归一数据。
- **回滚边界：** Down 恢复旧默认值并移除约束，保留已归一的 `1`；应用回滚仍可读取既有 `1/2` 语义。

### AD-033：Artifact 仍复用通用文件存储凭据与 bucket

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** Agent Artifact、MinIO 权限隔离、跨域对象覆盖与生产切流
- **解决方式：** Artifact blob client 从全局文件 `MinIOUploader` 拆分为显式启用的独立配置、`dipole-agent-artifacts` bucket 和 `dipoleartifact` Core 身份；policy 仅允许 bucket 定位/列举与固定前缀 Get/Put，不含 Delete。通用存储同时降权为只覆盖文件及两个归档 bucket 的 `dipoleplatform` 身份，TS Agent Runtime 保持无对象存储凭据。
- **验证：** 真实 tmpfs MinIO 正向验证两个身份各自允许路径，拒绝 Artifact 删除、Artifact 前缀逃逸、Artifact 到文件 bucket 及平台身份到 Artifact bucket；三份 Compose 渲染、连续两次初始化、配置/策略测试、Go test/vet/race、TS test/typecheck/build 和生成物漂移门禁通过。
- **保留边界：** Runtime 与 Core 继续没有 Artifact 删除权限；孤儿对象审计和清扫由 AD-032 跟踪，未来使用单独 maintenance 身份与 dry-run receipt。

### AD-027：Agent 权限授予与审批状态尚未持久化

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `ExecutionContext`、Agent Definition、Capability Policy、Human-in-the-loop、远程 TS Runtime
- **解决方式：** migration v16 与 sqlc Store 持久化版本化 Definition、固定版本 Task 和一次性 Approval；Embedded trigger 默认以 `ai.policy_mode=persistent` 创建确定性 Task，重新读取精确 Definition version 后恢复 permission/resource scope，并以 CAS 完成生命周期。`static` 保留显式回滚。migration v17 将身份列 expand-only 扩至 24 字符，兼容默认 21 字符 Assistant UUID。
- **验证：** Tool、Capability、Command 均拒绝 permission 足够但 resource scope 越界的访问；撤销、过期、新版覆盖、重复 Task 和成功/失败生命周期测试通过；真实 MySQL 8.4 使用默认长度身份完成 Definition 初始化、Task 快照与 `running→completed`。G3 的审批 UI 与 Temporal Signal 恢复继续作为计划能力推进。

### AD-016：HTTP Handler 测试并行修改 Gin 全局模式

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **解决日期：** 2026-08-27
- **影响范围：** `internal/gateway/http/*_test.go`、整包 race 门禁
- **解决方式：** 在包级 `TestMain` 进入并行测试前只调用一次 `gin.SetMode(gin.TestMode)`，删除各测试函数中的重复全局写入，同时保留原有 `t.Parallel()` 覆盖。
- **验证：** 修复前旧 Handler 包的 `go test -race` 稳定报告 `gin.SetMode` 写写及与 `gin.New/CreateTestContext` 的读写竞争；修复后 Gateway HTTP 包的 race、普通测试和完整 Go 测试通过。
- **长期约束：** Handler 测试不得在测试函数或并行子测试中修改 Gin 包级模式；新增全局测试配置应在 `TestMain` 中串行完成。

### AD-025：Web 本地消息库清理与容量策略需真实浏览器验收

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** IndexedDB Sync Engine、共享设备隐私、浏览器配额、401/强制下线
- **解决方式：** IndexedDB 按用户隔离 Message、manifest 与 Cursor，并在同一事务执行整页提交和高低水位淘汰；显式退出、HTTP 401、WS kick 和账号切换统一进入 Session Terminator，先撤销凭据与运行时状态，再清理当前账号并跳转。增加真实浏览器重开/中断、独立 Chromium 主进程 crash、无特权 128 MiB tmpfs 容量拒绝，以及共享 profile 双账号 HTTP/WS 被动失效验收。
- **验证：** Chromium、Firefox、WebKit 均验证 U1 被 401 或 `session.kicked` 后凭据清空、U1 IndexedDB 归零且 U2 Seq/Message 保留；Chromium 在 `commitPage` pending 时主进程 crash 后保持整页原子性；专用 quota 脚本触发真实容量错误，释放 reserve 后失败页未推进安全 Cursor。`storage_full/sync_error` 有界指标和告警继续作为运行门禁。
- **长期约束：** 公共设备应使用显式退出；若浏览器在清理完成前被强制终止，操作人员或用户需从浏览器设置中清除 Dipole 站点数据。新增本地 store、账号 key 或会话终止入口时必须扩展三浏览器共享 profile 验收。

### AD-023：Sync Service 数据库账号与 Message 写权限尚未分离

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `cmd/services/sync`、`user_sync_inbox`、设备/群 checkpoint、MySQL 最小权限
- **解决方式：** 增加继承全局配置的 `sync.mysql.*` 专用凭据、操作级 Sync 启动探针和 `dipole_sync` 授权；增加 `message.inbox_write_mode=atomic|projector`，独立 owner 在 projector 模式停止 Inbox 写入，同时保留 Message/Seq/群高水位/Outbox 事务。`dipole_message_projector` 无 Inbox 权限，atomic 配置和原授权模板保留即时回滚。
- **验证：** 真实 MySQL 8.4 smoke 验证 Sync/Message 两类最小账号、越权拒绝、Message+Outbox 无 Inbox 写入、Sync 投影收敛和 atomic 回退；单元与 repository contract 覆盖模式校验、重复修复 no-op 和权限边界。

### AD-024：Sync Replay 的历史覆盖受 created Outbox 边界限制

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **解决日期：** 2026-08-27
- **影响范围：** `cmd/tools/sync-baseline`、历史群消息、Outbox 保留、Message Inbox 写权限退役
- **解决方式：** migration v11 增加不可变 baseline Job/Entry；Capture 在 Repeatable Read 固定 Inbox 高水位，并归档所有缺少 created Outbox 的原始 `sync_seq + recipient + locator`，以规范化 SHA-256 校验完整性。Reconcile 同时扫描快照后新增 legacy 行；Restore 仅修复 missing，保留原 Cursor，并拒绝 extra/conflicting 状态。
- **验证：** 纯领域测试覆盖稳定摘要和差异分类；真实 MySQL 8.4 integration/smoke 覆盖重复 Capture、删行检测、原 `sync_seq` 恢复、越界冲突拒绝、v11 down/up 与并发 migration owner。

### AD-020：Search 删除接口缺少 mutation revision

- **本轮验证：** Elasticsearch Search Service 与三节点 Search Indexer 真实隔离 smoke 已通过授权范围、tombstone 和乱序事件收敛；长期生产流量切换仍遵循 Search/A5 的 Alias、归档和回滚门禁。
- **本轮验证：** 2026-08-30 重新执行 `scripts/smoke-search-service.sh`，真实验证 Elasticsearch 9.5.2 查询路径、Core 派生 scope 和 Internal RPC 契约；临时存储栈自动清理，生产 Search Alias、索引切换和长期流量窗口保持未改变。

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **完成日期：** 2026-08-27
- **解决方式：** `SearchIndex` 收敛为版本化 `Apply(MessageSearchMutation)`；created/edited 生成 searchable 文档，recalled/deleted 生成只含身份、revision、`searchable=false` 与 payload hash 的持久 tombstone。MySQL 与 Elasticsearch 统一接受更高 revision、忽略旧 revision、接受相同重放并拒绝同 revision 不同 payload。
- **验证：** 共享模型单元测试、两种 adapter contract、MySQL 8.4 `000001..000007` Up/Down 与 Elasticsearch 9.5.2 真实 tombstone 演练通过；tombstone 后旧正文事件无法恢复搜索结果。

### AD-018：Cassandra Seq 响应不携带 MySQL 内部 ID

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-27
- **完成日期：** 2026-08-27
- **解决方式：** Direct/Group HTTP 与 Message v1 RPC 增加互斥的 `before_seq`；Web 历史首屏固定从 `before_seq=0` 开始，向前分页使用最旧正 Seq，热群补拉改用 `after_seq`，同 UUID 合并优先保留带持久 Seq 的版本。Cassandra 路由只覆盖显式 Seq cursor，legacy ID cursor 始终留在 MySQL。
- **验证：** Local/gRPC 应用契约、HTTP 游标互斥、Service 权限与 Seq 透传测试通过；真实 MySQL 8.4/Cassandra 5.0.9 演练确认 before/after 完整页由 Cassandra 返回，人工缺行后整页回退 MySQL。
- **保留兼容：** Cassandra 响应的 `id` 继续为零；全局身份使用 `message_id`，会话排序和分页使用 `message_seq`。A6 已增加默认关闭的 IndexedDB 持久同步，旧 Offline 双跑对照完成前不改变默认客户端路径。

### AD-004：热群消息缺少持久化同步补偿

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** Message 事务以 O(1) 写入群 Timeline 高水位，Sync 保存用户/设备/群拉取位点；客户端重连后提交已知群列表，经 Core 成员权限校验取得最新 Seq，并使用 `after_seq` 分页追平。IndexedDB v3 将热群消息与本地群 `message_seq` 原子提交，提交后才 ACK 设备群 checkpoint；在线 notify 聚合继续保留，Redis 或 Gateway 重启不会丢失离线发现依据。
- **验证：** 通过历史 migration 回填、消息/Outbox/高水位原子回滚、设备 ACK 单调性、越权拒绝、Message/Sync gRPC、HTTP 零 Seq cursor、真实 MySQL contract 和定向 race 测试；Web 契约覆盖持久化失败禁止 ACK、重开本地恢复、ACK 补交、位点单调性与逐账号清理。
- **兼容说明：** `off` 模式继续执行不 ACK 的内存补拉；`/messages/offline` 覆盖升级前历史和旧客户端。在线 Sync Item 驱动 Cassandra 主读仍按 A6 独立灰度。

### AD-014：M3 grpc 模式存在重复 Local MessageService 实例

- **优先级：** P2
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** Runtime 成为 Messaging Services 的唯一 Composition Root，并把同一实例注入 Server 与 Kafka handler 注册；Server 只在兼容构造入口缺少注入时创建服务集合。Conversation notifier 在 Server 建立 WS Hub 后注入现有实例。
- **验证：** Local/Remote transport、Server、Kafka 和 Bootstrap 全部复用 Runtime 的 Messaging Services，并通过相关包 race 测试。

### AD-013：内部 RPC 调用身份尚未绑定服务认证

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 内部 RPC 同时启用共享服务凭据、caller allowlist、常量时间密钥比较和 TLS 1.3 mTLS；认证服务身份写入调用 context，并与 protobuf `caller_service` 强制一致。明文 listener 与 target 仅允许 loopback。
- **验证：** 通过合法/缺失/错误凭据、未授权 caller、payload caller 冲突、真实临时 CA mTLS，以及非 loopback 明文拒绝测试。

### AD-010：GORM 模型与运行时 AutoMigrate 绑定数据结构

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 使用版本化 SQL migration 管理 schema，所有 Repository 经共享真实 MySQL 契约渐进迁移到 `database/sql + sqlc`；最终移除 legacy adapters、model tags、AutoMigrate、SQLite 方言测试、兼容配置和 `gorm.io/*` 依赖。
- **验证：** 全仓 GORM 标识与模块依赖扫描为空；通过 sqlc 生成漂移、全量 Go、真实 MySQL Repository/migration/并发事务、race、vet 和模块完整性测试。
- **2026-08-30 复核：** `scripts/check-sqlc.sh`、服务布局门禁和 Go 全仓回归再次通过；生产 Go 源码及 `go.mod`/`go.sum` 未发现 GORM 或 `AutoMigrate` 回流，继续保持 SQLC-only 运行边界。
- **2026-08-30 门禁收紧：** `scripts/check-sqlc.sh` 现 fail closed 扫描 GORM module、任意 Go import/selector 和 `AutoMigrate`，并有临时 Git fixture 覆盖干净基线、GORM import、`AutoMigrate` 与旧 module 回流。多语言服务继续以版本化 SQL、SQLC 生成契约和 RPC/Event 边界协作。

### AD-001：并发事务可能造成 Sync Cursor 永久跳过消息

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 新增 `user_sync_states` 用户锁表；Inbox 事务按用户 UUID 固定顺序获取 `FOR UPDATE` 行锁，再分配全局自增 `sync_seq`。重复投影修复也进入同一事务。
- **验证：** 使用 MySQL 8.4 双连接测试暂停第一条未提交事务，证明第二条同用户事务被阻塞，释放后 Inbox 游标顺序与提交顺序一致；同时通过迁移、回滚、MySQL SQL 契约和定向 race 测试。

### AD-002：旧群消息事件与热群 Sync Fanout 标志存在歧义

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 将 `sync_fanout` 改为三态字段；旧事件缺失时默认执行普通 fanout，显式 `false` 继续表示热群跳过 Inbox。
- **验证：** 覆盖旧事件缺失、显式启用和显式关闭的 Kafka JSON 契约测试，并通过完整测试与定向 race 测试。

### AD-003：幂等冲突可能使用新事件收件人修复旧消息 Inbox

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-26
- **完成日期：** 2026-08-26
- **解决方式：** 修复 Outbox/Inbox 前校验发送者、目标类型、目标 UUID 和会话键；冲突时返回明确错误，并基于已有消息重新计算可信收件人。
- **验证：** 覆盖同一 `client_message_id` 改投其他目标的隔离测试，并通过完整测试与定向 race 测试。
### AD-050：一次性运维代码仍混杂在横向目录

- **优先级：** P2
- **状态：** 已完成
- **发现日期：** 2026-08-29
- **影响范围：** 仓库导航、服务边界识别、运维工具维护
- **现状：** 回填、基线、清理、切换、证据和对账代码已按 Agent、Cassandra、Search、Sync 服务域迁入 `internal/operations/<service>/`，长期运行时仍位于 `internal/bootstrap/`，命令入口仍位于 `cmd/tools/`。
- **解决方式：** 删除 `internal/backfill`、`internal/baseline`、`internal/cleanup`、`internal/cutover`、`internal/reconcile` 和 `internal/evidence` 横向目录；通过 `check-service-layout.sh` 阻止旧目录回流，并在操作目录 README 中固定分层约定。
- **验证：** 旧目录和旧包引用扫描为空，结构门禁与 Go 全量测试通过后合并。

### AD-051：MySQL 运维 adapter 与平台基础设施边界混杂

- **优先级：** P2
- **状态：** 已完成
- **发现日期：** 2026-08-29
- **影响范围：** `internal/data/mysql`、migration runner、DSN 配置、运维 contract test
- **现状：** MySQL 事务 Store 已位于 `internal/platform/mysql`；迁移 runner、DSN 组装和运维 adapter 曾继续分散在旧数据目录。
- **解决方式：** migration runner 和 DSN 配置迁入 MySQL 平台目录，Agent/Cassandra/Search/Sync adapter 按操作域迁入 `internal/operations/<service>/<operation>/mysql/`；`internal/data/mysql` 历史兼容目录已退役。
- **验证：** 操作域、MySQL 平台和工具包定向测试通过；结构门禁阻止旧 adapter、migration 和 DSN 目录回流。
- **本轮验证：** 2026-08-30 通过 `scripts/smoke-mysql-cluster.sh` 完成隔离 MySQL 8.4.8 三节点 InnoDB Cluster 验收：migration v50、Router writer failover、已提交数据连续可见和停止成员 AdminAPI rejoin 均通过；脚本使用一次性 YAML 配置固定 Router 地址并显式 `CGO_ENABLED=0`，临时 Compose 资源自动清理。

### AD-052：Message domain 直接依赖 Core 文件 domain

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-29
- **完成日期：** 2026-08-29
- **影响范围：** Message/Core 服务物理拆分、共享错误契约和兼容 HTTP 错误映射
- **现状：** Message domain 曾直接导入 `internal/services/core/domain/file`，仅用于复用文件存储、缺失和权限错误值，形成跨服务 domain 依赖。
- **解决方式：** 将三个跨服务文件错误提升到 `internal/application`，Core File domain 与 Message domain 均引用 application contract；增加服务布局门禁，阻止 Message 重新依赖 Core domain 实现。
- **验证：** Message/Core File/兼容服务定向测试、`scripts/check-service-layout.sh` 和 `CGO_ENABLED=0 go test ./...` 通过；错误值身份保持兼容。

### AD-053：Core standalone 系统消息曾回落到本地 Message facade

- **优先级：** P1
- **状态：** 已解决
- **发现日期：** 2026-08-29
- **完成日期：** 2026-08-29
- **影响范围：** Core/Message 启动依赖、联系人与群组系统消息、Message 写入 ownership
- **现状：** Core standalone 使用空 Message repository 构造 embedded facade，联系人和群组触发的系统消息无法明确进入 Message Service 的独立写路径。
- **解决方式：** Message RPC 增加 Core-only system direct/group command；Core standalone 使用懒连接 adapter，首次调用才执行带健康检查的 RPC 连接，避免 Core 与 Message 同时启动时形成阻塞环；embedded 继续使用本地 adapter。
- **验证：** 协议生成检查、Compose 默认远程 transport 检查、Message RPC handler、Core/Message/Server 定向测试和全量 Go 测试通过；未认证 system command 被拒绝。

### AD-055：大文件上传仍经 Core 业务服务串行中转

- **优先级：** P1
- **状态：** 进行中
- **发现日期：** 2026-08-30
- **影响范围：** 大文件上传吞吐、Core 连接占用、断点恢复、对象存储清理和上传完整性
- **现状：** 项目已经使用 MinIO 原生 S3 Multipart Upload；超过 Web 端 `4 MiB` 阈值后由 Core 逐片接收并调用 `PutObjectPart`，默认单文件上限为 `50 MiB`、分片大小为 `5 MiB`，会话与 part ETag/Size 保存在 Redis。当前前端使用 3 路有界并发、最多 2 次指数退避重试、可选 `X-Part-SHA256` part 校验和基于文件指纹的 session 恢复，Core 已提供受所有权保护的会话状态查询；预签名直传与 Gateway 同源代理已具备默认关闭的实现，整文件 SHA-256 绑定与可选强制校验已完成，生产默认仍使用业务服务中转。
- **风险：** 文件越大，Core 的请求连接、带宽和超时压力越高；暂停/继续控制尚未提供完整的用户操作面，Redis/MinIO 的过期 upload 清理、跨存储 reconciliation、告警和 Complete/Abort 幂等仍需要运维闭环，预签名直传默认切流也需要完整故障矩阵证据。
- **计划：** 在 A7 中继续完成暂停/恢复、Complete/Abort 幂等、未完成 Multipart 生命周期清理、Redis/MinIO upload reconciliation、生命周期指标与告警，以及真实 MinIO 故障矩阵验收；旧单请求与当前服务端 Multipart 路径继续保留为 feature flag 回滚路径。
- **幂等进展：** Complete 成功后写入短期完成收据，重复 Complete 在收据有效期内返回同一文件记录；Abort 对已取消会话幂等成功，对已完成会话拒绝，避免重复触发对象存储操作。收据持久化与业务文件记录之间的异常窗口仍纳入后续 reconciliation 验收。
- **Reconciliation 进展：** `dipole-multipart-cleanup --reconcile` 已增加只读 MinIO/Redis 对照，报告 MinIO upload 缺少 Redis session、Redis session 缺少 MinIO upload 两类漂移；默认不删除，扫描不完整或读取异常会 fail closed，后续继续补齐告警与经确认的修复动作。
- **告警进展：** `--reconcile-fail-on-drift` 已提供显式告警门禁，发现漂移时返回退出码 `3`，可由 CronJob 或发布检查采集；自动修复、Prometheus 指标接入和通知路由仍待完成。
- **指标进展：** Core Multipart 指标已为整文件 checksum mismatch 暴露专用 `outcome`，并新增 Prometheus 规则与 promtool 契约测试；通知路由、漂移报告指标化和生产 Alertmanager 联调仍待完成。
- **本轮进展：** `--reconcile --metrics-output` 已支持输出固定名称、低基数的 Prometheus textfile gauges，包含扫描数量、两类漂移、完成状态和最后运行时间；文件通过同目录临时文件原子替换，默认不输出且不改变 JSON/退出码语义。textfile 新鲜度告警、Alertmanager 联调和真实部署仍待完成。
- **本轮进展：** cleanup textfile 输出已补充低基数生命周期状态：active、expired、aborted、failed、complete 和 duration；`--metrics-output` 现在支持 cleanup-only，同时保留 reconciliation 输出兼容。retry 与 checksum mismatch 仍由 Core operation 指标观察，真实 Prometheus/Alertmanager 接入和默认直传切流继续待完成。
- **本轮进展：** Core 已将同一 Multipart session 重复 `partNumber` 识别为 retry，单独增加 `upload_part` retry counter 且不重复写入 duration；presence 检测为可选能力，读取异常会 fail open 继续上传。真实告警路由、retry 规则和预签名默认切流仍待完成。
- **本轮进展：** retry counter 已加入 `DipoleMultipartPartRetries` Prometheus 规则，按 operation 聚合并以 5 分钟持续窗口触发；promtool firing 时序已锁定。真实 Alertmanager 路由和预签名默认切流仍待完成。
- **告警进展：** Multipart 规则已消费 reconciliation gauges，新增漂移、扫描不完整和超过 15 分钟未刷新的告警，并以 `promtool` 触发时序测试锁定；规则仍需在真实 Prometheus/Alertmanager 环境完成抓取、路由和通知验收。
- **本轮进展：** 新增默认 dry-run 的 `dipole-multipart-cleanup` 工具，按 MinIO `message-files/` 未完成 upload 的发起时间筛选，执行必须显式 `--execute --confirm`，输出逐 upload 状态并保留单项失败；`--redis-orphans` 增加有界 Redis meta/parts 扫描，识别无 TTL meta 和 meta 已过期的孤儿 parts，只有 `--execute --confirm` 才删除孤儿 parts，并以回归测试保证重复执行幂等、扫描截断显式标记不完整；新增按归属会话批量签发、绑定 `uploadId + partNumber` 的短期预签名 part URL contract，以及直传后由 MinIO 实际元数据核验并登记 ETag/尺寸的接口；Web 已接入默认关闭的直传试运行 flag。真实验收确认当前开源 MinIO 不支持 Bucket CORS API，三套 Compose 已移除会失败的 `mc cors set` 初始化命令；Gateway 已增加默认关闭的同源 S3 PUT 代理，校验签名字段、固定 bucket、PUT 方法和分片体积，XML 策略仅保留给支持 Bucket CORS 的对象存储部署参考。可选真实 MinIO 集成测试已验证代理转发、S3 Host 签名、ETag、Complete 和对象内容，并自动清理测试对象；Core 已接入低基数 Multipart operation counter/histogram；Multipart 初始化可绑定整文件 SHA-256，`storage.multipart_require_checksum` 开启后 Complete 会校验已完成对象并清理不匹配对象；现有中转路径仍为默认回滚路径，完整 MinIO upload reconciliation、Redis 告警与默认切流仍待补齐。
- **验证：** 当前 MinIO Multipart 单元/服务契约和 HTTP handler 测试已覆盖初始化、分片、完成、缺片、越权及 Abort 基本语义；服务端完成阶段已校验 part 实际大小、请求实际读取长度、part SHA-256 和可选整文件 SHA-256，状态查询已覆盖所有权边界，前端 Multipart helper 的并发、分片边界、重试、checksum 探测、跳过已确认 part 和永久失败停止调度测试通过；Redis 孤儿扫描、代理转发和可选真实 MinIO Multipart 流程已有隔离验证；隔离 MinIO 实测 Multipart CORS 配置返回 `501 NotImplemented`，生产默认继续使用可回滚的 Core 中转路径。
- **本轮进展：** Web 调度器现区分可恢复故障：无状态的浏览器断连，以及预签名 PUT 的 `408`、`429`、`5xx` 采用原有指数退避；确定的 `4xx` 立即失败。28 项专项 Vitest 覆盖该分类、暂停/恢复、取消、续传和跨标签锁，typecheck 与生产构建通过；真实代理、浏览器离线切换和跨网络恢复仍待独立环境证据。
- **本轮进展：** Web Multipart 增加可见的暂停/继续控制；暂停仅阻止新 part 调度，不 Abort 或清理 Redis/MinIO 会话，继续时复用原 `upload_id` 并继续跳过已确认 part。上传 helper 已覆盖暂停等待和恢复调度测试，前端专项测试、typecheck 与生产构建通过；预签名默认切流、完整生命周期告警和真实故障矩阵仍待完成。
- **本轮进展：** 新增 `contracts/multipart-upload/v1` 版本化策略和 SHA-256 release manifest，统一记录直传阈值、文件上限、分片大小、并发、重试、退避和预签名 URL TTL；校验脚本强制默认 `relay` 与旧路径回切，当前只完成配置契约门禁，尚未切换生产流量。
- **本轮进展：** Core 增加认证的 Multipart policy 查询，前端按服务端策略执行阈值、并发、重试和预签名模式，并对版本/字段异常 fail closed 后回退 `v1/relay`；源码注释和三份静态 Swagger 文档均已同步，生成器在当前 Go 1.27 标准库解析下仍需后续工具链升级，预签名默认切流仍待共享环境证据。

### AD-058：业务拓扑 Compose 门禁曾依赖 healthcheck 数组顺序

- **优先级：** P3
- **状态：** 已解决
- **发现日期：** 2026-08-30
- **解决日期：** 2026-08-30
- **影响范围：** `scripts/check-compose.sh`、业务集群静态验证
- **现状：** Router healthcheck 的命令数组中端口参数位置发生变化时，旧断言可能将地址字段误判为端口，导致合法业务拓扑无法通过门禁。
- **解决方式：** 改为在 healthcheck 参数集合中按值匹配 `3306`，并在业务拓扑契约中锁定该语义；默认单节点路径和运行时拓扑不受影响。
- **验证：** 业务拓扑契约 `6/6`、完整 `scripts/check-compose.sh`、Shell 语法和 diff 检查通过。

### AD-057：业务集群 MySQL Router 已接入，真实故障收敛证据仍待补齐

- **优先级：** P2
- **状态：** 进行中
- **发现日期：** 2026-08-30
- **影响范围：** 业务集群 Compose、migration/readiness、MySQL writer failover、消息链路恢复
- **现状：** 业务 override 已将应用侧 `mysql` 稳定服务名映射到 MySQL Router，并加入 `mysql-1/2/3` 与 `mysql-cluster-init`；默认微服务路径仍使用单节点 MySQL。当前只完成可渲染拓扑和 fail-closed 静态门禁，未在 Remote GPU 业务组合中执行 broker/Redis/MySQL 联合故障矩阵。
- **解决方式：** 以独立 Compose project、固定 revision 和保留卷的回滚入口执行最小启动、MySQL Router writer failover、migration/权限重启、消息/InBox/搜索/投递收敛验证；活动登录会话继续需要明确批准，GPU 任务可按已授权规则并行。
- **验证：** `scripts/check-compose.sh`、业务拓扑契约和 Compose config 已覆盖 Router 镜像、三节点成员、初始化顺序与 `mysql:3306` 入口；真实业务故障 receipt 仍待维护窗口。

### AD-056：开发压测环境与共享主机资源边界尚未完成部署验收

- **优先级：** P2
- **状态：** 进行中
- **发现日期：** 2026-08-30
- **影响范围：** 远程开发部署、微服务 smoke、负载测试证据、主机资源和并行实验隔离
- **现状：** Remote GPU 已具备完整验证所需资源，但当前存在多个登录会话和 GPU 任务；TencentCloud_01 仅有 2 vCPU、2 GiB 内存和 50 GiB 磁盘，只适合轻量 smoke；本机根分区剩余约 19 GiB，完整集群压测风险较高。
- **解决方式：** 使用 `scripts/check-dev-host.sh` 做启动前 fail-closed 检查，采用独立工作目录、Compose project、端口、网络、非生产凭据和提交绑定镜像；先在维护窗口完成 Remote GPU smoke/基线，再做 TencentCloud 低资源回归。
- **验证：** 主机资源和 Docker/Compose 配置门禁已有测试，轻量 smoke 已通过 Gateway 依赖闭包契约测试；实际远程部署、运行证据、故障演练和清理回滚仍待明确窗口后完成。
- **只读主机证据：** 2026-08-30 通过 SSH 内存脚本复核 Remote GPU（224 vCPU、可用内存约 163510 MiB、可用磁盘约 1084340 MiB）和 TencentCloud_01（2 vCPU、可用内存约 1172 MiB、可用磁盘约 34347 MiB）的 `check-dev-host.sh` profile；两者均通过资源门禁，未启动容器，仍不构成部署或负载测试证据。
- **代码同步证据：** 2026-08-30 通过 `scripts/remote-dev.sh sync` 在 Remote GPU 创建 `/home/zhangzhuyu/workspaces/Dipole`，并更新至提交 `c3739971`；同步不启动容器，资源 preflight 通过，但主机缺少 Docker Compose v2 插件，Compose 部署、readiness、压测和清理回滚证据仍待维护窗口。
- **管理员连接校正：** Remote GPU 的可用管理员 SSH alias 为 `LAB113-OPS`，实际用户 `admin1`；后续远端工作流默认目录切换为 `/home/admin1/workspaces/Dipole`。现有 `zhangzhuyu` 工作目录保留，不清理、不覆盖。
- **管理员 preflight 证据：** 2026-08-30 使用 `LAB113-OPS` 完成代码同步并复核资源；首次核验时 `admin1` 不在 `docker` 组，且主机未安装 Docker Compose CLI 插件，因此当时仅能完成代码同步，构建/Compose/压测继续 fail-closed。修复只需在维护窗口由管理员补充 Compose v2 插件并按最小范围授予 Docker 访问，再重新执行 preflight。
- **前置修复证据：** 2026-08-30 按授权将 `admin1` 加入 `docker` 组，并安装 Ubuntu 24.04 `docker-compose-v2` 2.40.3；新登录会话 Docker daemon 可访问，`scripts/remote-dev.sh preflight` 已通过。活动用户/GPU 保护仍阻止构建与启动。
- **远端测试阻塞证据：** 2026-08-30 `scripts/remote-dev.sh test` 已完成提交同步，但远端系统 Go 为 1.22.2，项目要求 Go 1.26.0；首次执行尝试联网下载工具链并因网络超时退出，复核使用 `GOTOOLCHAIN=local` 后确认版本不足。脚本现改为禁止隐式下载并快速报告版本缺口；升级 Go 1.26+ 前，远端 Go canonical 测试仍待执行。
- **用户态工具链证据：** 2026-08-30 已将本机 Go 1.27.0 工具链同步至 Remote GPU `/home/admin1/.local/go-1.27.0`，验证 `go version` 为 `go1.27.0`；系统 Go 1.22.2 未修改。待用 `DIPOLE_REMOTE_GO_ROOT` 重新执行完整远端 canonical 测试。
- **最新远端验证：** 2026-08-30 在 Remote GPU 的 `/home/admin1/workspaces/Dipole` 对提交 `9c0f2702` 使用用户态 Go 1.27.0、`GOTOOLCHAIN=local` 完成 `scripts/remote-dev.sh test`；Go 白名单测试、服务布局和架构文档门禁全部通过，未启动 Compose、未创建容器，远端源码保持提交绑定的干净状态。构建、Smoke、Benchmark 和真实故障矩阵仍需单独维护窗口与活动用户保护证据。
- **远端 canonical 测试证据：** 2026-08-30 在提交 `a92b9a8c` 使用用户态 Go 1.27.0 和临时只读 module proxy 执行 `scripts/remote-dev.sh test`，Go test、Compose、服务布局和架构文档门禁全部通过，退出码 `0`；测试未启动容器。本机 Dipole Compose 拓扑已停止，远端完整构建、smoke、基线压测仍待维护窗口。
- **离线复核与本机降载证据：** 2026-08-30 关闭临时 module proxy 后，Remote GPU 使用已同步的 Go module cache 以 `GOPROXY=off` 完成 `scripts/remote-dev.sh test`，提交 `9b83ab31` 的 Go test、Compose、服务布局和架构文档门禁全部通过，退出码 `0`。本机 Dipole 主拓扑、隔离 smoke 和观测拓扑已停止，卷/镜像保留；远端完整构建、smoke、基线压测仍待维护窗口。
- **降载工作流固化证据：** 2026-08-30 新增 `scripts/drain-local-dipole.sh`，默认 dry-run，只有显式 `--apply` 才停止名称以 `dipole` 开头的运行中容器；脚本不删除容器、卷或镜像，并通过 Node 契约测试锁定目标范围。后续远端同步成功后可复用该入口，避免本机负载残留。
- **最新远端 canonical 证据：** 2026-08-30 将 `master` 提交 `3dfaf53d` 同步至 `/home/admin1/workspaces/Dipole`，使用用户态 Go 1.27.0 和 `GOPROXY=off` 完成 `scripts/remote-dev.sh test`；Go test、服务布局和架构文档门禁通过，退出码 `0`，未启动容器。完整构建、smoke、基线压测仍受 Remote GPU 活动用户/GPU 保护。
- **远端 Node 验证证据：** 2026-08-30 在 `/home/admin1/workspaces/Dipole` 使用用户态 Node `22.12.0` 完成 Agent Runtime 与 Frontend 验证；Agent `125` 个测试文件/`665` 个测试通过并完成 typecheck/build，Frontend `29` 个测试文件/`114` 个测试通过并完成 typecheck/Vite build。首次 `npm ci --ignore-scripts` 缺少 rolldown optional binding，补装 optional dependencies 后恢复；集成测试按环境条件跳过，未启动 Docker。
- **最新基线复核证据：** 2026-08-30 在提交 `37d5f1b3` 上重新完成 Remote GPU Go canonical、服务布局和架构文档门禁，退出码 `0`；Node 验证生成的两个锁文件环境差异已反向清理，远端 detached 工作目录恢复干净，未启动容器。
- **最新资源保护证据：** 2026-08-30 Remote GPU preflight 通过，报告 224 vCPU、约 161 GiB 可用内存和约 1 TiB 可用磁盘；构建前保护检测到 `users=23`、`gpu_processes=5`，退出码 `3` 并未创建镜像/容器。继续等待维护窗口，禁止通过 `DIPOLE_REMOTE_ALLOW_ACTIVE=1` 绕过。
- **资源保护策略校正：** 2026-08-30 用户明确授权开发阶段在 GPU 任务存在时启动本项目；仅允许使用隔离的 `/home/admin1/workspaces/Dipole`、Compose project、证书、网络和非生产凭据，并禁止操作其他项目或 GPU 进程。`DIPOLE_REMOTE_ALLOW_ACTIVE=1` 只在该授权和隔离条件同时成立时使用，默认 fail-closed 行为保持不变。
- **启动保护证据：** 2026-08-30 执行 `scripts/remote-dev.sh build`，代码同步成功后因 Remote GPU 观测到 `users=23`、`gpu_processes=5` 在构建前退出，未创建镜像或容器；保护逻辑有效。
- **正式基线复核：** 2026-08-30 通过 `DIPOLE_REMOTE_BRANCH=master scripts/remote-dev.sh sync` 将管理员工作目录更新至 `27138a32`；只完成代码同步，活动用户/GPU 保护继续阻止构建与部署。
- **正式基线同步证据：** 2026-08-30 已通过管理员 alias 将远端工作目录更新到 `master` 最新已同步提交；未启动容器，Docker 权限与 Compose 插件已就绪，Go 版本缺口和活动用户保护仍阻止完整测试/构建。
- **远端 Smoke 证据：** 2026-08-30 在明确授权允许 GPU 任务并行的情况下运行隔离 `smoke-lite`；preflight、证书生成和项目隔离通过，随后因 Docker registry mirror 对缺失的 `dipole-core:latest` 返回 `403 Forbidden` 退出，trap 已清理隔离容器/卷，未触碰其他项目或 GPU 进程。该失败暴露远端需要先构建提交绑定服务镜像的流程缺口。
- **远端构建修复：** `scripts/remote-dev.sh build` 现先执行 `scripts/docker-build.sh backend` 生成提交绑定 Go 二进制，再执行逐服务镜像构建；入口契约测试 `5/5`、shell 语法和 diff 检查通过。下一次 Smoke 仍需验证基础镜像缓存/registry 可用性，真实负载和故障矩阵保持未完成。
- **最新远端构建与 Smoke 证据：** 2026-08-30 在提交 `c09334f0` 使用用户态 Go 1.27.0 完成 Go 二进制和 8 个微服务镜像构建；镜像构建上下文约 `688MB`。随后隔离 `smoke-lite` 完成资源 preflight、内部证书生成和项目隔离，但首次拉取 MySQL 基础镜像耗时过长后安全中止；Compose trap 已清理本项目容器/卷，远端 GPU 进程仍保持运行。完整 Smoke、真实服务 readiness 和基线压测仍待镜像缓存或受控下载条件。
- **Smoke 完成证据：** 2026-08-30 将 `mysql:8.4` 通过一次性 SSH 流式导入远端后，在提交 `f227401a` 对隔离 Compose project 重跑 `smoke-lite`；MySQL、Redis、Kafka、MinIO、Core、Message、Sync、Gateway 全部 healthy，Gateway readiness、认证代理和可选服务隔离通过。退出清理核验无该 project 容器或卷残留，完整基线压测、故障注入和共享环境证据仍待完成。
- **入口只读基线：** 2026-08-30 在同一提交和独立 Compose project 上执行 1000 次 Gateway `/health`、并发 16；成功 `1000`、失败 `0`，P50/P95/P99 为 `0.000521/0.000791/0.001960s`，退出后无容器或卷残留。该结果只覆盖入口健康请求稳定性，不支持消息吞吐、WebSocket 投递、Kafka lag 或成员 fan-out 结论。
- **C1 工具边界：** 当前 `scripts/bench/run_bench.sh` 面向旧三节点单体候选拓扑，需要 revision 匹配的 `dipole-server` 镜像和远端 `k6`；当前微服务 Smoke 使用独立 Compose 拓扑。后续必须先完成候选拓扑构建/工具链前置，再采集完整 C1 基线，避免混用两种部署模型。
- **C1 候选构建入口：** 2026-08-30 `scripts/remote-dev.sh build` 增加默认关闭的 `DIPOLE_REMOTE_BUILD_CANDIDATE=1` 开关；开启后按当前提交构建 `dipole-server:c1-<commit>` 并写入 revision、创建时间和 `dirty=false` provenance，默认微服务构建路径保持不变。远端 k6 和三节点候选 Compose 启动仍待验证。
- **C1 候选入口修复：** 2026-08-30 首次开启候选构建时发现 heredoc 将候选变量错误地在本地展开，脚本在 Docker 构建前 fail closed；现已转为远端运行时展开，并由契约测试锁定，未产生候选镜像或容器副作用。
- **C1 候选拓扑路径修复：** 2026-08-30 首次启动发现旧候选脚本的 `--project-directory` 使 `../../configs` 挂载到错误目录，且未准备 Nginx `local` 证书；现已按 Compose 文件目录解析相对路径，并按需生成短期开发证书，生产 Compose 不受影响。
- **C1 可选组件隔离：** 2026-08-30 远端启动时确认 Kafdrop/Nginx 的外部镜像下载会阻塞核心基准；候选拓扑默认仅启动 MySQL、Redis、Kafka、MinIO、迁移器和三节点服务，Kafdrop/Nginx 通过 `C1_ENABLE_OPTIONAL_SERVICES=1` 显式开启，避免把观测 UI 或反向代理耦合进消息链路基线。
- **C1 低负载基线：** 2026-08-30 在 Remote GPU 对提交 `160d2cc6` 完成 20 用户、50 条 direct message 的候选链路验证；接受、持久化和投递均为 `50/50`，消息端到端 P50/P95/P99 为 `49/162.10/165ms`，Kafka lag 采样为 `0`。该证据尚未覆盖容量上限、长连接规模、热群 fan-out 或故障恢复，扩展矩阵仍待执行。
- **C1 群广播基线：** 2026-08-30 在同一提交绑定候选镜像上完成 20 成员、10 条群消息验证；`190/190` 预期回执收到，消息端到端 P50/P95/P99 为 `83/89.54/107ms`，Kafka lag 采样为 `0`。该证据覆盖小规模成员 fan-out，尚未证明热群容量、背压或故障恢复。
- **C1 并发在线基线：** 2026-08-30 在同一提交绑定候选镜像上完成 20 个在线用户、80 条消息验证；接受、持久化和投递均为 `80/80`，消息端到端 P50/P95/P99 为 `91.5/103.05/104.41ms`，Kafka lag 采样为 `0`。该证据覆盖低规模跨节点并发，尚未证明容量上限、连接耗尽、背压或自动回切。
- **C1 100 用户容量观察：** 2026-08-30 在同一候选镜像上完成 100 个在线用户、400 条消息验证；接受、持久化和投递均为 `400/400`，消息端到端 P50/P95/P99 为 `149/178.04/243.01ms`，Kafka lag 采样为 `0`。相较 20 用户并发延迟上升，仍需更高并发、资源拐点、节点故障、Kafka 延迟和自动回切证据。
- **C1 recovery drill 路径修复：** 2026-08-30 发现 recovery drill 仍使用旧的 `--project-directory`，会使候选 Compose 相对挂载路径与启动脚本不一致；已移除该参数并加入契约测试，节点 stop/start 故障演练仍需在新提交绑定候选镜像上重新采集。
- **C1 单节点恢复证据：** 2026-08-30 在提交 `dd46e35b` 候选镜像上停止并恢复 `dipole-node2`；约 `505ms` 观察到健康端点不可用，约 `16.0s` 恢复，consumer group 稳定成员数为 `72`，恢复后 40/40 消息完成接受、持久化和投递且 Kafka lag 为 `0`。PID 已变化、镜像和 revision 未漂移；该证据覆盖单节点 stop/start，Kafka broker、Redis、热群和自动回切仍待单独演练。
- **C1 Kafka/Redis 组件故障证据：** 2026-08-30 在 Remote GPU 隔离环境完成三 broker Kafka consumer rebalance 和三节点 Redis Sentinel failover；Kafka member 退出后 6 个 partition 接管且 lag 为 `0`，Redis master 停止后约 4 秒完成切换，客户端读写、Pub/Sub、Presence、热群和限流状态恢复，旧 master 重新加入为 replica。该证据属于组件级验证，候选业务拓扑的 broker/Redis 自动回切、背压和端到端恢复仍待完成。Redis 探针镜像已改为可配置，默认使用远端已有的 `alpine:3.22`，详见 `benchmarks/c1-remote-2026-08-30/c1-component-fault-evidence.md`。
- **C1 有效基线证据：** 2026-08-30 在 Remote GPU、`master` 提交 `b9281eaa`、隔离 `dipole-c1` 三节点候选拓扑上完成 k6 基线；Dockerized k6 通过 UID/GID 映射成功写出汇总文件，450/450 消息接受、持久化和投递，端到端 P50/P95/P99 为 `88/109/181ms`，Kafka 峰值与结算 lag 均为 `0`，候选拓扑清理后无 `dipole-c1` 容器残留，外部 GPU 进程保持 `2` 个。群组阶段在 `20s` 运行窗口内完成 `30/50`，报告 HTTP failure rate 为 `21.35%`，该比例主要反映优雅停止覆盖限制；仍需拆分群组 workload、提高并发、采集资源拐点并完成候选业务拓扑的 broker/Redis 故障和自动回切证据。
- **C1 群组基线证据：** 2026-08-30 在 Remote GPU、`master` 提交 `67a4aa1a`、隔离 `dipole-c1` 三节点候选拓扑上使用 `SCENARIO_FILTER=group_blast` 完成 50/50 VU；10/10 群消息接受、持久化，群 Inbox 为 500 行，490/490 预期回执和 100% 投递，端到端 P50/P95/P99 为 `106/118/132ms`，Kafka lag 从 `1` 收敛到 `0`，候选拓扑清理后无残留容器，外部 GPU 进程保持 `2` 个。`group_blast` 默认窗口已由固定 `20s` 改为可配置 `GROUP_MAX_DURATION`，当前结果仅覆盖 50 成员群组。
- **C1 100 成员规模证据：** 2026-08-30 在 Remote GPU、`master` 提交 `9595b0ef`、隔离 `dipole-c1` 三节点候选拓扑上使用 `SCENARIO_FILTER=group_blast` 完成 100/100 VU；10/10 群消息接受、持久化，群 Inbox 为 `1000` 行，990/990 预期回执和 100% 投递，端到端 P50/P95/P99 为 `121/222/226ms`，Kafka lag 为 `0`，Node1 CPU 峰值约 `46.42%`，候选拓扑清理后无残留容器。该结果用于规模趋势观察，仍需热群、大规模成员、背压、broker/Redis 故障和自动回切证据。
- **C1 报告场景元数据修复：** 2026-08-30 修正 `run_bench.sh` 在 `SCENARIO_FILTER` 下仍写入 `SCENARIO` 的问题；现在过滤场景会准确进入 operations、JSON 和 Markdown 报告，并由 benchmark contract test 锁定，避免将 group-only 结果误标为 mixed。该修复不改变服务端行为或容量结论。
- **C1 远程 workload 参数入口：** 2026-08-30 为 `remote-dev.sh bench` 增加受控的 `DIPOLE_BENCH_*` 参数转发，覆盖场景过滤、群组窗口、用户数、群组规模和运行 ID，并由入口契约测试锁定。此前需要在 Remote GPU 内部手工调用 `run_bench.sh`，容易引入嵌套 SSH 与参数漂移；后续应从本地入口触发正式 group-only/热群/高并发 workload，并继续保留候选 revision、项目和容器隔离门禁。
- **构建上下文优化：** Go 服务镜像 Dockerfile 改为从 `dist/` 上下文复制指定二进制，构建脚本不再为每个镜像发送根目录上下文；契约测试覆盖上下文和 COPY 关系。Agent/C++ 镜像上下文保持独立，远端实际构建需进一步确认上下文大小与耗时收益。
- **变量修复证据：** 首次远端实测发现上下文切换代码引用未定义的大写变量 `ROOT_DIR`，在镜像构建前 fail-closed；已改为脚本实际定义的 `root_dir`，6 项入口契约/语法测试通过，未产生错误镜像或容器。下一次远端 build 负责确认最小上下文实测值。
- **TencentCloud 占用证据：** 同次只读核验发现已有 `nkdoing-app` 容器占用公网 `80`、`nkdoing-postgres` 绑定本机 `5432`，宿主 MySQL 监听 `3306`；因此 TencentCloud 只能在明确端口、Compose project、卷和业务影响隔离后执行轻量 smoke，不能视为干净测试主机。
- **C1 参数入口修复证据：** 2026-08-30 发现 SSH `bash -s --` 会丢弃空位置参数，导致 `DIPOLE_BENCH_GROUP_MAX_DURATION=35s` 被误识别为 `SCENARIO_FILTER`；现已为所有可选参数增加显式哨兵和远端解码，入口契约 `10/10` 通过。旧候选 revision 被 provenance 门禁拒绝后已重建同版本镜像，避免混用证据。
- **C1 200 成员规模证据：** 2026-08-30 使用本地正式入口在 Remote GPU 完成 `group_blast` 200/200 VU；10/10 群消息 accepted/persisted，Inbox `2000` 行，1990/1990 预期回执，投递率 `100%`，HTTP failure `0%`，P50/P95/P99 `121/167/169ms`，Kafka lag `1 -> 0`。Node1/2/3 CPU 峰值 `72.14%/20.99%/19.85%`，峰值 RSS `77.14/74.22/67.79 MiB`；热群读扩散、背压和业务拓扑故障回切仍未完成。
- **C1 热群 notify/pull 观察证据：** 2026-08-30 使用 `bench_group.js`、200 成员群和 `HOT_GROUP_WARMUP_MESSAGES=60` 完成 20 条正式消息；`3980/3980` 预期回执、投递率 `100%`、HTTP failure `0%`，群 Inbox 写入 `0`，Conversation message projection `80`，Kafka peak/settled lag `54/0`，P50/P95/P99 `296.5/2241.55/2521ms`。该结果支持 hot-group 读扩散行为观察，但当时报告阈值字段为空，不能单独证明具体阈值配置；后续入口已支持显式转发阈值，背压和故障回切仍待完成。
- **C1 热群入口修复：** 远程 benchmark 入口现支持 `BENCH_SCRIPT`、独立 `PHONE_PREFIX`、warm-up、激活等待和成员/消息阈值转发，默认行为保持兼容；setup 曾因复用已有 `138` 号码空间失败，已通过独立号码空间回归并将账号冲突风险纳入运行手册。
### AD-059 Multipart cleanup scan completeness

- **状态：** 改进已合入；真实 MinIO/Redis 故障矩阵仍待 Remote GPU 维护窗口。
- **约束：** MinIO listing error 必须使 cleanup report `complete=false` 并 fail closed；dry-run 默认保持只读，执行模式仍要求显式确认。
- **证据：** `internal/operations/storage/multipart_cleanup_test.go` 覆盖列举错误、部分候选和失败返回语义。
- 2026-08-30：Eino alpha spike 的总计划状态已与 `EINO-V010-ALPHA-SPIKE.md` 对齐；Session/Checkpoint/Background Task/Notification/Tool execution 仅作为 adapter 参考，Temporal、Dipole Capability、owner scope 和审计仍保持权威边界。
- 2026-08-30：Multipart cleanup 输入边界已加固：未初始化客户端和 MinIO listing error 均 fail-closed，错误详情有界且保留总数；真实 MinIO/Redis 故障矩阵仍待共享维护窗口。
- 2026-08-30：Remote GPU 隔离 MinIO Multipart smoke 已在 `b8b27a76`、Go `1.27.0` 下通过，基础生命周期证据已归档到运行手册；客户端中断、服务重启、预签名默认切流和跨存储故障矩阵仍为未完成项。
- 2026-08-30：Remote GPU Multipart restart smoke 已在 `10cccdd3`、Go `1.27.0` 下通过，确认持久卷可跨 MinIO 重启恢复分片并完成对象；客户端中断、预签名默认切流和跨存储故障矩阵仍未关闭。
- 2026-08-30：Remote GPU 对 `master` 提交 `bd7283d1` 完成远程 canonical 验证：Go 全量测试、服务布局与架构文档门禁通过；Agent Runtime `125` 个测试文件/`665` 个测试通过并完成 typecheck/build，Frontend `29` 个测试文件/`114` 个测试通过并完成 typecheck/Vite build。该证据覆盖 CPU/Node 验证和远程降载路径，未启动业务 Compose；Node `22.12.0` 与部分依赖声明的 `22.22.2+` engine 警告仍应在后续工具链升级中收敛。
- 2026-08-30：真实 MinIO Multipart 集成契约新增客户端上传流中断后同 part 重试覆盖；中断尝试返回受控错误，重试可复用 upload session，且失败尝试未进入完成结果。完整浏览器断网、过期会话、网关限流和跨存储故障矩阵仍待 Remote GPU 验证。
- 2026-08-30：真实 MinIO Multipart 集成契约进一步覆盖中断流同 part 重试后的 Complete 与对象内容校验；失败尝试未进入最终对象，客户端断网、过期会话、网关限流和跨存储故障矩阵仍待 Remote GPU 验证。
- 2026-08-30：Web Multipart 调度器新增可选 `AbortSignal`，取消会传播到 presigned/relay 请求并在退避前停止重试；页面卸载保留服务端 session 以支持后续恢复。浏览器真实断网、预签名服务异常和网关限流矩阵仍待 Remote GPU 验证。

### AD-060：Multipart Redis TTL 与 MinIO upload 生命周期缺少联合故障证据

- **优先级：** P1
- **状态：** 进行中
- **发现日期：** 2026-08-30
- **影响范围：** 大文件续传、Redis 会话过期、MinIO 未完成 upload 清理和跨存储对账
- **现状：** Redis session store 为 metadata、parts 和 completion receipt 设置 TTL；服务层已对缺失 session fail-closed，但 Redis 到期、MinIO 未完成 upload 残留和 cleanup reconciliation 尚未在同一真实故障矩阵中验证。
- **本轮进展：** `TestRedisMultipartSessionTTLExpiresMetadataAndPartsTogether` 验证 metadata/parts 同步过期与分片续期，`TestRedisMultipartSessionCompletionUsesIndependentTTL` 验证完成收据到期；测试使用确定性 Redis 时钟推进，不改变默认上传或清理路径。
- **本轮进展：** 新增 `smoke-multipart-reconciliation.sh` 与真实 MinIO/Redis 集成测试，覆盖匹配状态、Redis metadata 缺失和 Redis orphan drift；脚本使用隔离端口和自动清理，未触碰业务 Compose。
- **真实验证：** Remote GPU 在 `8940bff1` 使用 Go `1.27.0` 通过联合 smoke，退出码 `0`，GPU 进程保持 `0`，临时容器为 `0`；当前 MinIO 版本要求 reconciliation 使用完整对象键并等待 listing 收敛，目录前缀行为继续保留为兼容性约束。
- **本轮进展：** 真实集成 smoke 增加可选 Redis restart 注入窗口：匹配状态建立后暂停测试，重启隔离 Redis，恢复后确认 Redis metadata 缺失、MinIO incomplete upload 可 Abort，随后继续验证 Redis orphan drift；默认路径不变。
- **本轮进展：** cleanup 对 `NoSuchUpload` 做幂等收敛分类 `already_gone`，降低 list/Abort 并发竞态的误报；未知或实际 Abort 错误仍增加 `Failed` 并保持 fail-closed。
- **本轮进展：** 指标 textfile 原子发布增加故障测试，目标发布失败时保留原目标并清理临时文件；现有低基数指标和告警语义保持不变。
- **本轮进展：** HTTP Gateway Multipart 初始化入口增加限流 fail-fast 回归，确认限流请求不会触发 Core/MinIO；文件上传窗口仍复用 Redis-backed `AllowFileUpload`，默认配置保持兼容。
- **本轮进展：** 预签名 Gateway 代理增加显式上游响应超时配置与 `502` 超时回归，避免对象存储挂起长期占用 Gateway；默认 `30s`，仅启用代理时生效。
- **本轮进展：** 预签名 Gateway 代理接入按客户端地址的文件上传限流，限流发生在代理调用前并返回 `429`；允许请求才进入 MinIO，原签名与 relay 回退边界不变。
- **本轮进展：** 新增 fault-matrix 聚合脚本；Remote GPU 确定性 Go 门禁、真实 MinIO/Redis 基础 reconciliation 与 Redis restart smoke 通过，promtool 依赖镜像拉取因 registry 无进展中止，完整矩阵保持未关闭。
- **本轮收口：** Remote GPU 使用通过临时反向隧道取得并校验的官方 Prometheus `3.5.0` `promtool` 完成告警规则、firing timeline、确定性 Go 门禁、真实 MinIO/Redis reconciliation 与 Redis restart smoke；矩阵退出码为 `0`，GPU 进程前后均为 `0`，Dipole/Multipart 容器为 `0`，远程工作树干净。
- **本轮验证：** 在 `7601e78e` 上复跑完整矩阵：6 个确定性 Go package、7 条 Prometheus 规则、真实 MinIO/Redis reconciliation 与 Redis restart 注入均通过，脚本退出后无 `dipole-multipart-reconciliation-*` 容器残留。该证据仍是隔离开发期验证，未提供 24 小时 presigned 流量、生产 Alertmanager receiver 或默认模式切换授权。
- **本轮进展：** 新增 `multipart-presigned-rollout/v1` evidence/policy/report 契约与只读 evaluator。候选切流必须绑定精确策略 SHA-256，在最少 24 小时窗口内同时满足直传样本、fallback/failed/expired/checksum 比率、P95、clear alert、已演练 relay 回退和独立 reviewer；输出哈希 receipt，`blocked` 返回退出码 `2`。该工具没有修改运行时策略，默认仍为 `relay`。
- **本轮进展：** `check-multipart-policy.mjs` 现以 versioned policy 为基准，同时校验 release manifest、示例配置、Go 默认配置和 Web 离线回退值，避免候选切流前发生参数跨层漂移；环境级覆盖和默认 `presigned` 切换仍受独立 receipt 门禁约束。
- **下一步：** 在受控共享环境生成同版本真实 evidence receipt，并完成 active/expired/abort/retry 生命周期指标、真实 Prometheus/Alertmanager 路由验收；receipt 通过后仍需经过受审策略变更才可切换预签名默认值。

### AD-061：Agent Memory active promotion 仍缺少共享环境 authority 证据

- **优先级：** P1
- **状态：** 进行中
- **发现日期：** 2026-08-30
- **影响范围：** Agent Memory candidate、review、promotion、五类 Memory 生命周期
- **现状：** Runtime 已统一声明 `working`、`episodic`、`semantic`、`procedural` 和 `observational` 五类类型；Gateway owner 控制入口可请求持久长期类型，Core 在事务中重读 candidate/review、拒绝 task-scoped `working` 并保证幂等。独立 active executor 已由 `CommitMemoryPromotionReceipt` Core RPC、TypeScript client 和 Temporal `promotion_active` Activity 组成；它只提交低敏 receipt，Core 从持久 Task/Run 恢复主体并重查 active admission、promotion grant、candidate/review 与 target type。
- **本轮进展：** Core bootstrap 仅在显式 receipt commit 开关与 mTLS 同时存在时注入服务；Runtime profile 还要求 active mode、Temporal、Capability RPC mTLS、`operator_approved` authority 和 Runtime/Core 双开关，基础 Worker 固定拒绝提交。Gateway 已提供认证 `POST /api/v1/agent/memories/:memory_id/revoke`，以 `dipole-gateway` mTLS caller 和服务端恢复的 owner principal 调用 Core `RevokeOwnedMemory`，并复核审计回包。隔离联合演练已覆盖首次 Activity 失败后的稳定 receipt 重试、grant 撤销后的拒绝与 owner-scoped Memory revoke；active Kafka group 也独立于 shadow group。
- **证据：** [active executor 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)、[Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)、`scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh`、`services/agent-runtime/src/temporal/agent-memory-promotion-mtls-mysql.integration.test.ts`、`internal/services/agent/infrastructure/mysql/agent_memory_promotion_temporal_fixture_test.go`。
- **下一步：** 在受控共享环境完成 Kafka trigger、Gateway-to-Core owner revoke 的真实 mTLS 运行证据、promotion overlay 回滚和 24 小时观测证据。缺少任一项时保持 Shadow/Remote 默认关闭，不能宣称生产自动长期 Memory 写入。

- **2026-08-30 兼容性补充：** promotion receipt v2 将 observational candidate 与显式目标类型一起绑定至 canonical hash；历史 v1 receipt 保持原语义可读，但因没有目标类型而在 replay 阶段 fail-closed。External MCP Shadow 对 partial enablement 增加零进程启动回归，默认关闭路径继续不构造 Worker、RPC 或网络资源。
