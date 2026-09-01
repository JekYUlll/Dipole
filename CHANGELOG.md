# 更新日志

- 2026-09-01：品牌资产改为由 `scripts/generate-brand-assets.mjs` 单一来源生成，`docs/images/` 下的 IM、Agent、favicon 与 lockup 四个 SVG 全部按 V3 品牌板实测色值（海军蓝 `#0D2744`、信号红 `#EA2521`、轨道金 `#EFAD05`、象牙白 `#FBF2E7`）与同一套几何常量重建，两个标识共用完全镜像的构造。Agent 变体只在同一主体上叠加金色层：均匀 6 单位描边、带明暗渐变的倾斜星环、中心完全镂空的环与打通桥接胶囊的镂空节点，主色块保持平涂。lockup 的字标换为 Goldman Bold 轮廓：宽体方块字形、方正字腔与均匀粗字干呈现机加工硬件感，与标识圆盘形成刻意反差，取代原先偏圆润的字形；轮廓按 cap-height 归一化内联为 path，无字体依赖，并由新增的 `scripts/generate-brand-wordmarks.mjs` 可复现生成。版式改为自适应：列位置按标识包围盒与字标实测宽度计算，cap 高度取仍能留出内边距的最大值（上限为与标识匹配的尺寸），不再使用硬编码坐标——Goldman 比常规无衬线宽约 40%，因此自动缩小而不是撞上分隔线。新增 `npm run test:brand` 门禁校验 SVG 与生成器一致；favicon 与 Login 用的两个标识由生成器镜像进 `frontend/public/` 与 `frontend/src/assets/brand/`，前端构建不再跨出自身根目录引用 `docs/`。退役 4 个旧青绿 SVG 与 `#07c160` 占位 favicon。README 资产引用同步更新，技术 badge 按 go.mod/.nvmrc 修正为 Go 1.26 与 Node 22.12 并补齐 Vue、Temporal、Redis、MinIO，统一为品牌海军蓝单色板；红金按设计语言保留给未读、主操作与 Agent 状态。仅文档与资产改动，不影响运行时与服务 authority。

- 2026-09-01：read 路径的多会话读取范围改为由 Task owner 确认。`conversation.list` 发现两个及以上会话时，Run 在 claim 读取 Step 前暂停并返回 `wait_input`，用 `dipole.agent.elicitation.v1` 的 select Form 提供最多 8 个会话候选并显式披露截断数量；恢复后只读取被确认的会话。用户提交同样按不可信输入处理，必须命中 checkpoint 中的候选集合与确定性 request ID，越界选择、伪造 request、缺失 checkpoint 均 fail closed，Core 仍是最终读取授权。恢复期不重新规划：暂停前已验证的 `conversation.list` 到 `conversation.read` 结构由代码重建，避免二次模型规划改变 Step 编号与 trajectory 重放语义；多于一对 discovery 的 plan 在需要确认时 fail closed。单会话与零会话行为不变，Kafka Shadow 主路径不受影响。写 Capability、active authority 和外部 MCP 保持关闭。

- 2026-09-01：Remote GPU 在候选 `aec1b867` 归档读取范围确认的 [approve/deny/expire 三份 receipt](benchmarks/agent-temporal-fault-2026-09-01/)，集成套件 `10/10` 并经 CLI 独立复核。三条演练由生产 read Activity 驱动，receipt 直接记录 Run 实际读取的会话数：确认路径在伪造 request 被拒绝后只读取被确认的会话并收敛为 completed；owner 取消与确认到期均收敛为 `cancelled`（`user_cancelled` / `input_expired`），会话读取计数为零。故障 receipt 契约同步加入这三个 drill 与 `conversationReads`、`unconfirmedConversationReads` 计数，出现未确认读取直接判定 `ineligible`。运行仍使用内存 Temporal Test Server，不接入 Kafka、Core、MySQL、共享 tenant 或 active authority。

- 2026-09-01：Remote GPU 在 `6beab05d` 完成隔离 Temporal Worker replacement 演练并归档 [两份故障 receipt](benchmarks/agent-temporal-fault-2026-09-01/)。approval/input 恢复均在替换 Worker 上收敛为一次完成；approval 路径的注入终态重试仅持久化一次，input 路径的无效与过期 Signal 均未恢复任务。运行使用内存 Temporal Test Server，未启动 Compose，也未接入 Core、Kafka、MySQL、共享 tenant 或 active authority。

- 2026-09-01：Remote GPU 在 `f0dcf98a` 通过 Approval gate v2 disposable drill，并归档 [v2 receipt](benchmarks/agent-mcp-approval-shadow-2026-09-01-v2/)。denied grant、consumed replay 与 failed-operation replay 均被拒绝且不产生新增 effect；同轮 MCP 去重、过期 readiness 与 mTLS identity 检查继续通过。审批 UI、真实外部 MCP 与 active authority 保持关闭。

- 2026-09-01：Approval gate drill receipt 升级为 v2，将 denied grant、consumed replay 与 failed-operation replay 的拒绝结果显式绑定到零副作用基数；v1 归档保持可读，下一次 Remote GPU disposable drill 将生成 v2 evidence。该契约不开放审批 UI、IM 写入、真实外部 MCP 或 active authority。

- 2026-09-01：Remote GPU 已在 `3c1f3eba` 复跑 External MCP/approval disposable Shadow drill，并归档低敏 [MCP 与 approval receipt](benchmarks/agent-mcp-approval-shadow-2026-09-01/)。验证覆盖本地 MCP 单次 Tool/Artifact、重启去重、过期 readiness 拒绝、mTLS identity denial 与 approved fixture operation 的精确副作用基数；共享服务、审批 UI、真实外部 MCP 和 active authority 继续关闭。

- 2026-09-01：External MCP/approval Shadow drill 的一次性 MySQL 追加关闭 native AIO 的启动参数，并由 Compose gate 固定。该兼容项仅覆盖共享 Remote GPU 上的 disposable drill，避免宿主 AIO 配额不足导致初始化失败；基础微服务拓扑保持不变。

- 2026-09-01：Remote GPU 的正常 Git 同步不再依赖 bundle 上传成功；`scp` 上传失败仅禁用离线回退并给出明确提示，远端仍可通过正式 Git remote 获取候选 revision。

- 2026-09-01：修复 Remote GPU 候选在 Git bundle 回退 clone 后仍指向临时 bundle `origin` 的问题；现在会恢复正式 Git remote，后续候选同步可继续 fetch。回归测试覆盖 fallback clone 的 remote 重绑定。

- 2026-09-01：Interactive Agent Web profile 增加 `agent-interactive-shadow` 构建模式，仅开启任务创建、Timeline 与 Artifact 页面。Remote GPU 同 revision 候选已在独立 `18100` 端口验证前端生产构建、认证、Task 创建、Temporal 收敛与 `5` 条 Timeline 事件；公开入口为 `http://223.111.157.214:18100/app/`。该候选仍是 `shadow + read_shadow`，不开放消息写入、MCP、Memory、检索或 active authority。

- 2026-09-01：新增 DeepSeek V4 Flash 的隔离 Interactive Agent Shadow overlay，固定 JSON-text 输出与关闭 reasoning，并在 Compose gate 中校验该配置仍只开放 `shadow + read_shadow` 的任务控制面。Remote GPU 同 revision 候选已验证认证、Task admission、Temporal、模型、Timeline 与 `conversation_digest` Artifact 的完整只读链路；active authority、MCP、Memory、检索和写 Capability 保持关闭。

- 2026-09-01：Agent Artifact 的认证前端现可读取严格限定的 `conversation_digest` Markdown 正文。页面仅在 metadata 为该类型和 `text/markdown` 时请求受限正文接口，正文响应必须与 metadata 的内容寻址 ID 和媒体类型一致；其他 Artifact 保持 metadata-only。正文以纯文本阅读区显示，下载、对象键、Metadata JSON 与写操作继续关闭。Remote GPU Node 22 已通过定向 Vitest `7/7`、typecheck、生产构建及 Chromium 功能/视觉回归。

- 2026-09-01：Gateway 现支持默认关闭、owner-scoped 的 `conversation_digest` Markdown 正文读取。`GET /api/v1/agent/artifacts/:artifact_id/content` 复用 Core Artifact owner 校验与内容哈希校验，只返回 Artifact ID、媒体类型和正文；对象键、Metadata JSON 与通用下载仍不进入公开边界。Remote GPU 隔离候选已验证同一用户的 metadata/content 均为 `200`，另一用户读取正文为 `404`。

- 2026-09-01：修复 Agent Artifact Timeline 的持久化 ID 边界。Artifact 的内容寻址 ID 现直接作为 Timeline event ID，符合 MySQL `VARCHAR(64)` 契约；领域校验同步拒绝超长 event ID，避免投影写入失败后造成 Artifact 从 Timeline 缺失。Remote GPU 隔离候选已复验任务完成、Artifact metadata 与 Timeline Artifact 事件一致。

- 2026-09-01：修复隔离 Interactive Agent Task 的 Timeline 读取闭环。Core 现将 Timeline Store 装配到独立 Agent gRPC adapter，并允许受限 `dipole-agent` mTLS caller 调用只读 Timeline RPC；Runtime 在 HTTP 边界将 protobuf `bigint` 转为前端契约要求的序列字符串和安全数字，并保留每个事件的 Task 绑定。Remote GPU 候选验证创建 `202`、终态 `completed` 和 Timeline `200`；空会话任务只有 Task/Run 事件，不将其描述为 Artifact 产出、active authority 或写能力。

- 2026-09-01：Gateway 的 Agent Task Timeline 代理现在将 URL path 与分页 query 分开解析。此前 `?limit` 和 `after` 会被编码进路径，Runtime 返回 `404`；认证、owner 授权和既有控制端点保持不变。

- 2026-09-01：read-shadow Agent 在 `conversation.list` 为空时，将依赖发现结果的 `conversation.read` 记录为 `skipped/no_discovered_conversation` 并继续生成摘要。该分支不调用远端读取 Capability；伪造 ID、缺少紧邻 List 等越权输入继续被拒绝。

- 2026-09-01：read-shadow Agent 将模型计划 JSON Schema 收紧为两个只读步骤：`conversation.list` 与紧随其后的 `conversation.read`；读取步骤的 `conversationId` 在 schema 层固定为 `$discovered.previous`，执行层仍独立校验并解析真实发现结果。该修复避免 Provider 生成裸会话 ID 后触发 Durable Task 重试。

- 2026-09-01：read-shadow Agent 完成两阶段模型闭环：先持久化计划并执行受信只读 Tool，再将完成的 Tool 输出封装为不可信数据交给独立 `synthesis` Model stage，最终 Artifact 使用综合摘要。空 Tool 输出保留原计划摘要；写 Capability、active authority 与自动多轮选择仍关闭。

- 2026-09-01：Agent Model audit 将 durable run 按 `task_uuid + stage` 分隔，默认 `plan` 保持既有运行 ID 与重放语义，新增 `synthesis` 可拥有独立预算、调用记录和恢复结果。该迁移为读取结果后的模型综合准备基础，尚未启用第二次模型调用或改变 Shadow 默认行为。

- 2026-09-01：两步 Agent 读取的执行层新增独立失败关闭门禁。即使上游 Planner 被替换或其校验被绕过，直接携带会话 ID 的 `conversation.read` 仍在授权审计和远程 Capability 调用前被拒绝；只有 `$discovered.previous` 可解析为前一步 List 输出。

- 2026-09-01：Agent read-shadow 现支持受信的两步 `conversation.list → conversation.read`。模型只能在紧邻的读取步骤使用固定 `$discovered.previous` 标记；Runtime 从前一 List 的实际输出提取首个有效会话键后才授权读取，模型自造 ID、缺少前置发现或空发现结果均会在读取前失败。写 Capability、active authority、MCP 写入与多会话选择策略仍未开启。

- 2026-09-01：Agent OAuth callback handoff Runtime 新增跨实例恢复演练：前一 Runtime 仅在显式 `retryable_failure` 后释放 Core 条件租约，替换实例随后才能重新领取并完成。该回归只验证默认未装配执行器对 Core lease 的依赖；callback 路由、Provider 换码、token 生命周期与默认启用状态仍保持关闭。

- 2026-09-01：Remote GPU 的 `node-test` 现在仅在锁文件安装明确报 `ENOTEMPTY` 时，将该候选 app 的中断 `node_modules` 原子隔离后重试一次；网络、锁文件和权限错误保持失败。该恢复路径保留旧目录，避免清理其他工作树的依赖，并继续使用单次 `npm ci --include=optional`。

- 2026-09-01：Remote GPU 隔离交互 Shadow 以 DeepSeek V4 Flash 完成一条新用户只读 Task：Gateway 返回 `202` 后状态收敛为 `completed`，持久化审计为一条完成 Run、一次模型调用、一次 `conversation.list` Step 与一个 `conversation_digest` Artifact。该证据只覆盖隔离 read-shadow；任务策略行保持 `running`、Durable `workflow_status` 与 Run 共同表达终态，写 Capability、active authority 与多轮读取均未开启。

- 2026-09-01：Agent 单轮 Temporal read-shadow 规划器收紧为仅暴露 `conversation.list`。此前模型能在未获得会话列表结果时直接生成 `conversation.read`，对新用户会构造无效目标并触发 Durable Activity 重试；读取 Capability、事件预取与 MCP 边界保持可用，后续多轮编排需要将后续读取目标绑定到已完成的发现结果。

- 2026-09-01：OpenAI-compatible Agent Provider 增加显式 `DIPOLE_AGENT_MODEL_THINKING_MODE=disabled`。开关仅透传给已选择的 Provider；DeepSeek V4 Flash 的受控 Shadow 可借此关闭默认高强度思考，避免有限 JSON-text 输出预算被 `reasoning_content` 耗尽。未设置时继续使用 Provider 默认行为，active authority、写 Capability 与 MCP 均未开启。

- 2026-09-01：Remote GPU 隔离交互 Shadow 候选完成公开 API 的只读 Agent Task 验收：临时用户经 JWT 创建任务后均收敛为 `completed`，Timeline 首次读取和 cursor 续页均成功。候选 Gateway 使用 `4ab924b87` 专用镜像，Core/Agent 保持兼容的既有候选版本；验收记录见 [Agent Interactive Shadow Remote Receipt](docs/agent/AGENT-INTERACTIVE-SHADOW-REMOTE-RECEIPT.md)。该结果不构成同版本发布、任务成功率、active authority、写 Capability 或公开体验入口结论。

- 2026-09-01：新增 `remote-gpu-mysql-aio-compat.yml`。共享 Remote GPU 的 Linux AIO 配额接近上限时，候选 MySQL 可显式关闭 native AIO 完成隔离验证；基础 Compose 与既有 MySQL 栈保持不变。

- 2026-09-01：Remote GPU Node 门禁将每个应用的双阶段 `npm ci + npm install` 收敛为一次锁文件驱动的 `npm ci --include=optional`，避免第二次安装的目录重命名冲突，同时保留 optional 平台依赖。

- 2026-09-01：修正 Remote GPU bundle 生成引用：以 `HEAD` 创建可检出的完整归档，并继续将不可变 commit 单独传给远端做精确校验，避免裸 SHA 被 Git 判定为空 bundle。

- 2026-09-01：Remote GPU 的 origin clone/fetch 增加可配置的 20 秒超时；GitHub 出站异常会在受限时间内转入已上传的 commit bundle，避免开发验证被网络阻塞。

- 2026-09-01：Remote GPU 候选同步新增 commit-pinned Git bundle 回退。远端 GitHub clone/fetch 超时时，脚本通过既有 SSH 上传临时 bundle、校验目标 commit 后 checkout，并在退出时清理 bundle；正常网络路径仍优先使用 origin fetch。

- 2026-09-01：根 README 改用版本控制的 `dipole-v3-brand-lockup.svg`。该矢量 lockup 统一 V3 的海军蓝/信号红 IM 标识与金色 Agent 轨道，替换此前粗粒度 PNG 嵌入；运行时和 Agent authority 不受影响。

- 2026-09-01：微服务镜像构建改为每个 Go 服务使用独立的临时 Docker context，成功和失败路径均在子 shell 退出时清理。此前循环复用 context 会累积前序二进制并放大后续 Docker 上传；镜像内仍只复制目标服务二进制，revision 与 provenance 参数保持不变。

- 2026-09-01：前端开始 V3 品牌兼容切片：以用户提供的双极对话标识重绘 IM/Agent SVG，新增海军蓝、信号红、轨道金和暖象牙白语义 token，并将登录入口迁移到新视觉语言。现有 Pencil canonical 变量与其余页面保持兼容；Pencil CLI 的两次安全增量调用超时且未写入画布，完整 Chat/Agent 视觉基线继续待独立设计验收。

- 2026-09-01：Remote GPU 隔离环境完成受认证 Interactive Agent Task 回归：注册 `200`、创建 `202`、终态读取 `200`，只读 Shadow Workflow 收敛为 `completed`，receipt 归档于 [`agent-interactive-control-2026-09-01`](benchmarks/agent-interactive-control-2026-09-01/)。此次仅重建 Agent `c9f3f424` 验证终态读取修复；Gateway/Core 保持此前 clean candidate，因此不构成同版本发布、active authority、生产体验或成功率结论。

- 2026-09-01：Agent Task control 在已授权的 Durable Workflow 终态关闭后，使用 Core 持久化的终态投影响应读取请求。此前 Temporal 不再接受关闭工作流的 Query 会使已完成任务返回 `404`；运行中任务和非“工作流不可用”错误继续失败关闭，避免旧投影掩盖执行中断。

- 2026-09-01：Remote GPU 以同一 clean revision `676a6d93` 完成隔离微服务 smoke。Gateway/Core/Message/Sync 与 Agent 镜像来自同一源码版本；私聊持久化后重启 Core，最终 Message、Outbox 和目标 Inbox 均为单条，receipt 归档于 [`microservices-same-revision-smoke-2026-09-01`](benchmarks/microservices-same-revision-smoke-2026-09-01/)。该演练不构成 Agent active authority、Cassandra 主读或 A6 浏览器观察验收。

- 2026-09-01：微服务镜像构建将 Go Docker context 收敛为当前目标二进制。构建仍使用同一 `dist` 基线与版本化标签，但每个服务不再重复上传整套 `dist` 工具集，降低 Remote GPU 验收的 Docker I/O 与传输开销；缺少或不可执行的目标二进制会在 build 前失败关闭。

- 2026-09-01：C++ Realtime Delivery 对齐 Timeline 主模式的无正文投递契约。显式 `primary` 只投递 `sync.item.notify.v1` locator，`shadow` 继续为完整消息追加 locator，二者同时开启会失败关闭；热群聚合语义保持不变。Ubuntu 24.04 容器门禁完成 CMake Release 构建并通过 `14/14` CTest。C++ authority 继续默认关闭，Go 仍是当前默认投递路径。

- 2026-09-01：修正 Timeline notify primary 的投递语义。Gateway Kafka 与 embedded Dispatcher 现在只向接收方发送无正文 `sync.item.notify.v1` locator，发送者继续收到 `chat.sent` 回执；`shadow` 保持完整消息加 locator 对照，`off` 保持完整消息投递，热群继续走聚合 notify + pull。direct/group/embedded 回归测试通过，真实 Cassandra 主读窗口和回切验收仍未开启。

- 2026-09-01：Agent Runtime 的本地全量门禁恢复可复现。未显式开启的 Approval mTLS drill 不再在 suite 注册阶段读取远程环境变量；安全 Eval 与 Temporal 只读活动夹具同步当前 token availability 和授权审计契约。离线验证通过 `158` 个测试文件、`796` 项测试，另有 `10` 个显式外部依赖测试跳过，TypeScript typecheck 与生产构建通过；该结果不替代 Remote GPU 同版本 Compose smoke。

- 2026-09-01：Agent Shadow Runtime 增加显式 Provider thinking 控制。`DIPOLE_AGENT_MODEL_THINKING_MODE=disabled` 仅向已选 OpenAI-compatible Provider 传递专有选项，默认继续使用 Provider 行为；单次模型规划同时收紧为 `conversation.list`，后续多轮读取必须绑定已完成的发现结果。该切片不启用写 Capability、active authority 或 MCP。

- 2026-09-01：Web Sync Observation Session 现可绑定完整静态发布目录。目录摘要按稳定相对路径和文件 SHA-256 计算，新增、删除、改名、内容变化、空目录或符号链接都会使候选校验失败；既有单文件 bundle 调用保持兼容。该切片只强化真实浏览器观察的版本证据，未启动 24 小时窗口或改变客户端默认同步模式。

- 2026-09-01：Remote GPU 隔离交互 Shadow 候选复验了认证 Agent Task 链路：新临时用户的只读请求返回 `202`，同一 Task 随后经受限轮询收敛为 `completed`。Gateway/Core 使用 `406c3154`，Agent 使用 `thinking-4e9740a0`，该结果只证明版本兼容与 Shadow 读取路径，不构成同版本发布、active authority、写 Capability 或整体成功率结论。

- 2026-09-01：个人资料弹窗新增修改密码闭环。受保护的 `/api/v1/auth/password` 校验当前密码、以 bcrypt 更新新密码并撤销当前会话；前端在成功后清除本地会话并要求重新登录，密码和哈希均不写入响应、日志或本地存储。

- 2026-09-01：修复 Agent Task 创建页面的默认请求 ID。此前 Vue Function prop 默认值多包了一层函数，浏览器会在本地参数校验阶段拒绝函数对象并显示“任务创建暂不可用”；现在默认生成 UUID 字符串，回归测试覆盖无显式 request ID 的提交路径。

- 2026-09-01：微服务隔离 smoke 增加默认关闭的低敏 Agent Task/Run receipt。显式提供输出路径时，成功收敛后仅写入随机 event、Task、Run、trace 与运行状态，便于后续 Context Ablation runner 在销毁临时 Compose 项目前建立绑定；消息、模型正文和凭据均不导出。

- 2026-09-01：修正 AI SDK Shadow Planner 的会话 hydration 输入：Core `ReadConversation` 现在接收事件中的 `target_uuid`，与其按目标用户/群 UUID 的 RPC 契约一致；`conversation_key` 继续只用于 Memory 作用域。该修复为隔离 Context Ablation 的真实会话证据准备前置条件，不开启任何写能力。

- 2026-09-01：修复 Context Ablation migration `000056` 的外键字符集/排序规则兼容性。binding 表现显式使用与 Agent Task/Run 相同的 `utf8mb4_unicode_ci`，隔离 MySQL 8.4 预检不再因外键列不兼容失败。

- 2026-09-01：新增 Context Ablation 隔离 MySQL 预检。它应用 migration `000056`、核对 binding 表并验证 Eval 账号可读取且没有写权限；脚本不启动完整 Compose 或 Agent 运行时，退出后清理临时容器与网络。

- 2026-09-01：新增 baseline、retrieval、memory 三份默认不加载的 Context Ablation Compose overlay。它们使用独立 Temporal queue，并以开关互斥地选择只读 Context 来源；Memory 写入、消息写入、Control、MCP 和 External MCP 不因此开启。

- 2026-09-01：Context Ablation 增加版本化 manifest JSON Schema 与低敏示例，并将 `agent_context_ablation_bindings` 纳入 `dipole_agent_eval` 的最小 `SELECT` 授权。评测账号继续没有任何写权限，示例不能作为效果或晋级证据。

- 2026-09-01：新增 `eval:context-ablation` 只读 CLI。它以一个低敏评审 manifest 加载 experiment 的三条件绑定观测并输出聚合报告；参数、数据库、审计、版本或计量不完整时失败关闭，成功只表示输入可复算。

- 2026-09-01：Context Ablation 现提供只读 observation adapter：经人工评审的低敏 manifest 以 case SHA-256、Artifact/Evidence ID 和固定模型价格绑定三种条件，再将已持久化的 Task/Run 审计观测编译为统一 Eval 输入。缺少终态、授权、延迟、Token 计量或候选版本一致性时拒绝生成报告；不读取或输出消息正文、模型正文或原始资源 ID。

- 2026-09-01：新增 Agent Context Ablation 实验绑定表与 SQLC 查询。每条记录仅绑定 experiment、case SHA-256、baseline/retrieval/memory 条件、Task/Run 与候选版本；数据库唯一约束阻止同一 case 条件重复或同一 Run 被复用，正文不进入该表。

- 2026-09-01：新增低敏 Context Ablation Eval v1，固定 baseline、retrieval、memory 三种条件，并按任务输出命中、证据召回、权限安全与模型/工具/token/成本/延迟汇总对照；首版只接受脱敏 observation，尚未连接真实 Shadow 查询。

- 2026-09-01：简历 Claim 验收矩阵已同步 Shadow Eval 的缺失 Token 计量语义：失败调用保留为可分类的 `token_metrics_unavailable`，固定任务集与共享环境观察窗口仍是填写任务成功率前的 P0 门禁。

- 2026-09-01：Shadow Eval 窗口输入的发布 JSON Schema 现与 Runtime 对齐，同时接受 40 位 Git revision 与 64 位内容摘要；外部 Schema 校验不再拒绝真实 OCI Git provenance。

- 2026-09-01：Shadow Eval 现将失败模型调用的缺失 Token 计量编码为可审计的 `availability.tokenMetrics=unavailable`，仍输出五类报告并在 Cost case 固定为 `token_metrics_unavailable` 失败；已知调用数和延迟保留，缺失 Token 不再被当作零或导致整份证据无法分类。完整计量样本的既有契约保持兼容。

- 2026-09-01：Remote GPU 隔离 read-shadow 新归档受控完成子集 `N=2` 的五类 Eval
  [窗口](benchmarks/agent-shadow-eval-window-2026-09-01-n2/)，两份 Task/Run 的 Outcome、Trajectory、Permission、Retrieval、Cost 均通过。其 `100%` 仅适用于该完成子集；同一栈另有一次 Provider 空 JSON-text 失败因 token 计量缺失无法形成五类报告，整体任务成功率继续保留占位符。

- 2026-09-01：Shadow Eval 汇总的 `runtimeRevision` 同时接受 40 位 Git SHA-1 与 64 位内容摘要；此前 OCI 镜像的有效 Git revision 会在汇总阶段被错误拒绝，现已由回归测试覆盖。

- 2026-09-01：Shadow Eval 窗口收集器改为从运行中 `agent` 容器的 OCI revision 与 clean-source
  标签取得 Runtime provenance；缺失或 dirty 标签立即失败关闭，避免候选 checkout 与实际评测镜像漂移。

- 2026-09-01：新增 `collect-agent-shadow-eval-window.sh`，可在已运行的 read-shadow Compose 中逐份执行人工评审的 Shadow Eval manifest，并生成按候选 revision、唯一 Trace 与唯一 Suite 绑定的低敏窗口汇总。脚本不创建任务、不生成评审标签、不启动服务或修改开关；全通过返回 `0`，有效失败窗口返回 `2`。

- 2026-09-01：Temporal 的“模型结果后置确认丢失”集成夹具补齐 Step 授权审计依赖，并固定断言重试仅记录一次授权。Remote GPU 以 Node `22.12.0` 重跑 `test:temporal:integration`，两个测试文件共 `10` 项均通过；该演练覆盖隔离 Temporal 的模型/Step 重放，不扩大 active authority 或外部写能力。

- 2026-09-01：Remote GPU 的 disposable read-shadow Compose 在候选 `agent-runtime@064568d9` 生成新的五类 Shadow Eval 报告：Outcome、Trajectory、Permission、Retrieval 和 Cost 均通过；Retrieval precision/recall 均为 `1`，单次执行记录 `1` 次模型调用、`2` 次工具调用、`1765` tokens 与 `7032 ms` 聚合延迟。报告归档于 [`agent-shadow-eval-2026-09-01-rerun`](benchmarks/agent-shadow-eval-2026-09-01-rerun/)，只表示受控隔离 `N=1` 观察，不构成生产成功率、共享 authority 或写 Capability 结论。

- 2026-09-01：Shadow Eval 的 Retrieval 指标现排除 runtime policy、execution context、task 与 capability registry 等控制面基线，仅统计领域证据；它们仍保留在 Context/Trajectory 审计中，不影响 Capability 或运行时上下文。

- 2026-09-01：微服务 Compose 增加 `dipole_agent_eval` 专用 MySQL 只读账号，仅允许 Shadow Eval 查询 Task、Run、Plan、Step、Artifact、Model 与 Tool 审计投影；该配置不启用自动评测或任何运行时写入权限。
- 2026-09-01：Remote GPU 隔离 read-shadow Compose 已用该账号完成 `SELECT`，并确认零行 `UPDATE` 仍被 MySQL 拒绝；该证据只覆盖账号权限，不代表 Eval 质量或 active authority。

- 2026-09-01：Agent Shadow Step 在持有有效 lease 时持久化精确 `resourceType/resourceId/action/decision`；Shadow Eval 只接受完整且为 `allowed` 的持久授权记录，并逐项对照评审 manifest。旧、部分或无效授权记录均 fail closed，未新增 Capability、授权或默认运行路径；待在隔离真实环境重新生成五类报告。

- 2026-09-01：撤回 `agent-shadow-eval-2026-09-01` 的五类通过报告。复核发现 Permission case 尚未持久化实际 resource scope，`resourceType/resourceId/action` 仍可来自评审 manifest，无法证明其与 Runtime 授权决策一致。该 JSON 已从当前证据集移除；后续将以 Step lease 内持久 scope/decision 与精确比对重新生成报告。

- 2026-09-01：Agent Shadow Eval manifest 与五类离线评测现可保留 Runtime 策略中的资源类通配 scope `*`。此前评估契约只能表达具体 resource ID，会使默认 `conversation/*` read-shadow 授权无法进入权限评测；JSON Schema、TypeScript parser、通用 evaluator 与回归测试现共同限制为稳定标识符或唯一 `*`，不扩大任何 Capability 或运行时授权。

- 2026-08-31：Agent Shadow Eval 现区分策略 Task 状态与 Durable Workflow 状态。常规 Task 仍以 `agent_tasks.status` 判定终态；read-shadow 仅在 CAS `workflow_status` 和持久 Run 同时终态时生成五类报告，避免将运行中的 Shadow 策略记录误判为未完成，也拒绝非终态 Workflow 伪造成功率。

- 2026-08-31：Agent EventLedger 新增 `dipole.agent.event-lease-reclaim.v1` receipt，绑定过期 claim 回收、旧 owner completion 拒绝与最终 completed 行唯一性。Remote GPU 的 loopback-only MySQL 8.4 集成测试 `3/3`、receipt 测试 `4/4` 与 TypeScript typecheck 均通过；该证据仅覆盖消费租约，不外推至 Temporal Workflow 或 active authority。

- 2026-08-31：Remote GPU 在候选 `a7bc03ef` 的隔离 Compose 项目完成 `dipole.agent.core-restart-read-shadow.v1` 实测：事件发布后 Core 重启、Gateway 代理恢复，且同一事件的 Ledger、Task、Run、模型调用和 `conversation_digest` Artifact 均精确为 `1`。新镜像内正式 CLI 已复核 24 小时低敏 receipt；该证据不外推到共享环境、写 authority 或 lease expiry。

- 2026-08-31：微服务 read-shadow Core restart 演练新增 `dipole.agent.core-restart-read-shadow.v1` receipt。只有重启后的 Core/Gateway 恢复，且同一事件的 Ledger、Task、Run、模型调用与 `conversation_digest` Artifact 全部精确收敛时才生成；artifact 以 SHA-256 和 24 小时窗口绑定，明确 `production_authority=false`。

- 2026-08-31：`BUILD_IMAGE=1` 的微服务 smoke 现同时构建 TypeScript `dipole-agent` 镜像并注入候选 revision 元数据。此前仅重建 Go 镜像，导致 Agent Runtime 可能运行旧 `latest` 代码；构建门禁已覆盖该同版本要求。

- 2026-08-31：Approval gate drill 新增语言中立 `dipole.agent.approval-gate-drill.v1` receipt、canonical SHA-256 与 24 小时有效期校验。隔离 mTLS 演练输出 approved `1`、denied/replay `0`、failed `1`、failed-replay `0` 的低敏 artifact；该证据不代表 IM 消息已写入、生产 authority 或共享 Shadow。

- 2026-08-31：External MCP Shadow drill 新增真实 mTLS `AgentCapabilityRPCClient` 的 Approval gate 场景：精确 approved grant 只执行一次，denied 与已消费 grant 均零执行，执行失败后审批保持已消费并拒绝自动重放。脚本同时固定 Node/npm 同源，干净 Remote GPU worktree 不会以系统 Node 误装依赖；默认写 Capability 与 production authority 继续关闭。

- 2026-08-31：Core 的 `dipole-agent` mTLS Capability allowlist 现包含 `ResolveApprovalGrant` 与 `ConsumeApproval`。隔离认证测试覆盖审批请求、批准、精确 grant、单次消费，以及错误 service secret 和错误客户端证书拒绝；该改动不启用常驻写 Capability、外部 MCP 写入或生产 authority。

- 2026-08-31：External MCP Shadow drill 现在在干净候选 worktree 缺少 `vitest` 时，以 `DIPOLE_NODE_BIN` 相邻的锁定 npm 执行 `npm ci --ignore-scripts`；已有依赖不重复安装。该修复使 Remote GPU 演练使用 Node 22，默认运行时 authority 不变。

- 2026-08-31：Remote GPU 在 `bb1e43e8` 上重跑 External MCP Shadow drill：`2/2` EventLedger 收敛、一次 Tool 与一次 Artifact、重启重复投递被抑制、过期 readiness 被拒绝，并通过 Go internal gRPC mTLS identity denial 检查。receipt 为短时低敏隔离证据，明确 `production_authority=false`。

- 2026-08-31：Temporal fault receipt v1 追加 `worker_replacement_input_resume`，将无效/过期输入拒绝、精确输入恢复、Worker 替换和终态写入基数收敛为独立可验证场景；该隔离证据不外推到 Core restart、lease expiry 或 active authority。

- 2026-08-31：Agent Temporal 增加 `worker_replacement_approval_resume` fault receipt v1。它将隔离测试中的状态修订、Worker 替换、终态写入重试和副作用基数绑定为 SHA-256 receipt；任何状态或单次副作用漂移都会得到 `ineligible`。该 receipt 证明本地 Temporal 场景，不构成共享环境、Core restart、lease expiry 或 active authority 证据。

- 2026-08-31：Agent Shadow Eval 现在把受信任 admission 的 `trace_id` 持久化到 `agent_runs`，并由 `eval:shadow` 输出版本化低敏 Trace envelope。窗口汇总拒绝缺失/非法 Trace、Trace 复用、混版本、合成 Suite 与重复摘要，便于将受控 Shadow 的成功率和失败分类关联到 OTel；旧 Run 因缺 Trace 不可作为该证据。

- 2026-08-31：Agent Eval 新增 `reviewed_shadow` 窗口汇总契约与 CLI。它只接收同候选版本、唯一摘要且完整绑定的终态 Shadow 五类报告，输出脱敏的任务样本量、成功率、类别通过率和失败原因计数；合成 Suite、混版本、重复报告及非绑定 case 均失败关闭。报告仍不构成 active authority、生产质量或用户影响证据。

- 2026-08-31：README 现采用用户提供的 Dipole V3 品牌板作为主视觉，并增加 Go、TypeScript Agent Runtime、Kafka、sqlc 与 MIT 许可证 badge。`docs/images/dipole-brand-v3.png` 成为 IM/Agent 双标识、配色和后续产品视觉调整的参考资产。

- 2026-08-31：Remote GPU 在隔离项目 `dipole-message-recovery-53a4edf7` 完成 Message Service 持久化后重启演练：同一 `client_message_id` 重放后 Message、Outbox 与目标 Inbox 均为单条，候选资源自动清理。低敏 receipt 归档于 `benchmarks/microservices-message-recovery-2026-08-31/`；Kafka/broker/in-flight 故障矩阵仍待验证。

- 2026-08-31：候选消息恢复 smoke 的 readiness 和首次持久化失败现输出有限服务状态、容器日志及 `wscli` 尾部。失败仍自动清理隔离 Compose 项目；诊断信息用于区分拓扑未就绪与消息链路未收敛，不能替代成功 receipt。

- 2026-08-31：候选微服务 smoke 的服务内 health、数据库计数与恢复探针现统一通过有界 `SMOKE_EXEC_TIMEOUT_SECONDS` 包装。Remote GPU 出现停止态 `docker compose exec` 客户端时，演练会在默认 20 秒后发送 `TERM`，再于 5 秒后强制结束并清理隔离项目，不再无限等待。

- 2026-08-31：候选微服务消息 smoke 新增显式 `SMOKE_MESSAGE_RESTART_SERVICE` 持久化后恢复演练。脚本在首次 WebSocket 消息落库后重启 Core、Gateway、Message 或 Sync，使用同一 `client_message_id` 重放，并在权限为 `0600` 的 receipt 中核对 Message、Outbox、目标 Inbox 三类副作用各为一条；默认 smoke 路径不变。

- 2026-08-31：新增简历 Claim 验收矩阵，将 Dipole IM 与 Dipole Agent 的目标表述逐项绑定到实现、可重跑运行证据和剩余门禁。后续优先补齐消息/Agent 故障副作用 receipt、Sync/Cassandra/Search/热点群 P99 和 Agent Eval 成功率；无对应报告时继续保留指标占位符。

- 2026-08-31：微服务 smoke 现支持版本化 Compose overlay、受控 Compose 环境文件、事件发布后的可选 Core 重启，以及 Read Shadow 的模型调用和 `conversation_digest` Artifact 绑定断言。Remote GPU 已在隔离 Compose 项目完成事件发布后 Core 重启、Temporal 收敛与 Artifact 绑定演练，资源随后自动清理。该工具不改变默认基础 Shadow 路径和 authority。

- 2026-08-31：基础微服务 Compose 的 Agent Shadow profile 现显式清空 v2 模型 route context profile，避免宿主 `.env` 的 AI SDK/active 配置与固定的 v1 Context Compiler 冲突而导致 Agent 启动失败。专用 AI SDK 与 active overlay 继续显式启用 v2 并注入 profile。

- 2026-08-31：微服务 smoke 新增可选 `RESTART_CORE=1` 隔离 Core 重启阶段。重启后重新验证 Core readiness 与 Gateway 代理，再执行既有 Agent EventLedger/Task/Run 幂等检查；Remote GPU 已以独立 Compose 项目和端口完成该演练并自动清理。默认 smoke 行为不变，Capability RPC read-shadow 恢复仍需单独演练。

- 2026-08-31：Agent Capability RPC 的重连包装器现在在每次方法调用时解析当前 gRPC channel。Core 返回 `UNAVAILABLE` 后，即使上层保留了旧方法引用，下一次事件级调用也会使用新 channel；失败调用仍不在 transport 层重放，Kafka/EventLedger 保持幂等重试责任。

- 2026-08-31：默认关闭的 Agent OAuth callback handoff executor 现在在私钥解封前和 Provider processor 前两次复核 durable handoff 的 lease/expiry。过期检查失败属于副作用前故障并释放 lease；processor 或 completion 结果不确定时仍保留 lease。该改动不装配 callback HTTP、Provider token exchange 或 token 生命周期。

- 2026-08-31：Cassandra read-rollout 新增 Prometheus 窗口转换器与 CLI。它将 Message Service 的起止快照转换为 evidence v1，并拒绝 route/verification counter 回退、未知标签、histogram bucket 漂移及未覆盖最终路由的延迟数据；`mysql_fallback` 同时归入 MySQL 最终路径和 fallback 比例。转换过程只读取快照文件，不改变 Cassandra 读比例或 MySQL 回退。

- 2026-08-31：补齐主链路外部阻塞期间的并行治理规则。前端设计、只读体验、视觉回归、文档入口和图表可在独立分支推进；这些切片不改变服务 authority、默认 feature flag 或真实环境证据门槛。

- 2026-08-31：新增 Cassandra read-rollout 原始 Prometheus 窗口采集脚本，严格分离不可覆盖的 `start` 与 `end` 快照，并绑定部署 revision 与配置读比例。脚本只读取 Message Service `/metrics`，为后续 evidence v1 转换与共享灰度归档提供输入，不修改流量开关。

- 2026-08-31：修正 Cassandra read-rollout evidence 对运行时回退指标的计数契约：`mysql_fallback` 是 MySQL 最终路由的子集，评估器现按该关系校验。真实 Prometheus 窗口中的 Cassandra 失败回退不再被误判为无效 evidence，默认读比例与回退行为不变。

- 2026-08-31：Remote GPU 使用隔离 MySQL 容器完成 Agent Task Timeline repair 进程 smoke：worker 重放单个 intent 并幂等收敛。该脚本现支持 `DIPOLE_GO_BIN`，避免远端默认 Go 版本低于模块要求时误阻断演练；不改变默认 repair profile 或生产开关。

- 2026-08-31：Agent Timeline repair Compose smoke 改为从已版本化 migration 文件动态推导 schema 基线，并轮询正常退出的 `mysql-permissions` 一次性初始化容器。Remote GPU 使用 MySQL migration v53 完成最小权限、UTC、worker readiness、pending intent 恢复和 event UUID 幂等重放；随机 Compose 项目、卷和临时工作树均已自动清理。

- 2026-08-31：Agent Runtime 现在会拒绝 `DIPOLE_AGENT_OAUTH_CALLBACK_ENABLED=true` 的未批准部署 profile。当前镜像没有 Provider code-exchange processor，启动会在创建 listener、Kafka、Temporal 或 RPC 资源前失败；未设置该开关时保持默认关闭。

- 2026-08-31：Agent Runtime 新增 OAuth callback handoff 的注入式组合工厂，统一 claim、私钥解封、processor、complete/release 与 Gateway control adapter；默认启动路径仍不构造该工厂，且未实现 Provider code exchange 或 token 生命周期。组合工厂要求独立 Core 凭据，并支持仅用于受控测试的 key/envelope 注入。

- 2026-08-31：C++ Realtime Delivery 后续开发暂缓。现有协议、容器构建、故障回切和基准证据保持可复核，但 Go 继续作为唯一投递 authority；当前开发窗口优先完成 TypeScript Agent Runtime 的默认关闭 OAuth handoff 执行器组合验证与安全闭环。

- 2026-08-31：Remote GPU 的 `agent-temporal-read-shadow` 完成受控只读 Durable Task 演练：Kafka 事件经 Temporal 和 Core mTLS 投影后，持久 Run 完成、EventLedger 单次收敛，并生成 `conversation_digest` Artifact。开发环境将 Flash 单次输出预算设为 `1024`，用于避免推理过程耗尽正文预算；仍保持零内部重试、只读 Capability、schema 校验和默认关闭的写能力。

- 2026-08-31：微服务 Compose 的独立 Agent 服务现在显式加载受忽略的根目录 `.env`。这保证重建 `agent-temporal-read-shadow` 容器时保留受托管的 Provider 路由和凭据；缺失 `.env` 时基础 `metadata` 配置仍可启动，AI SDK overlay 继续因缺 Provider 配置失败关闭。

- 2026-08-31：修复独立 Core 在远程 Agent Temporal 运行时遗漏 Task approval、control、Workflow projection/repair 与 Artifact RPC 装配的问题。独立 Core 现在复用 embedded 回滚基线的持久服务组合；Artifact 仍要求既有存储开关。该修复仅使 `read_shadow` Durable Task 能完整投影和产出受控 Artifact，不开放消息写入、外部 MCP 或 active authority。

- 2026-08-31：修复 `agent-temporal-read-shadow` 开发 overlay 在容器网络中无法通过健康检查的部署缺口：Temporal 显式监听 `0.0.0.0`，保留 loopback `7233` 探针，并由 Compose 合同检查锁定该绑定。该 overlay 仍只提供内部网络的 `read_shadow` 执行，不开放默认写能力或外部端口。

- 2026-08-31：DeepSeek V4 Flash 的 `json_text` 输出恢复支持单个、受限包装的 JSON 对象：短前后说明可被剥离，第二个 JSON 对象、超长包装、无效 JSON 和 schema 不匹配继续失败关闭。解析结果仍须通过 Zod 与只读 Capability allowlist；该修复不改变任务权限、模型预算或副作用边界。

- 2026-08-31：新增显式 `agent-temporal-read-shadow` 开发 overlay 与运行手册。它将 Temporal/PostgreSQL 限定在 Compose 内部网络，并把 Agent 固定为 DeepSeek 等 AI SDK Provider 可用时的 `read_shadow`、v2 Context 与独立 task queue；消息写入、Memory、检索、Control、MCP、OAuth callback 和默认基础 Compose 均保持关闭。

- 2026-08-31：OAuth callback handoff 增加 Runtime 重启重复通知与 terminal completion 不可用的组合测试：进程内去重不会跨重启保留，重复请求重新交给 Core lease；完成终态不确定时不释放 lease。callback HTTP、key source、provider exchange 与默认启动配置仍保持关闭。

- 2026-08-31：Remote GPU 开发环境使用 `.env` 托管的 DeepSeek V4 Flash 完成真实私聊到 Kafka、Agent Capability RPC、单次 JSON-text 模型调用、持久化 Model Run/Call 与 Shadow Plan 的只读闭环验证；该证据仅覆盖 Shadow Runtime，不代表 active authority 或写能力已开放。

- 2026-08-31：`json_text` 解析支持受限的前置标签：仅提取响应末尾唯一、完整的 JSON 对象；对象后的任意文本仍拒绝。提取结果继续经过 Zod schema 与 Capability allowlist 验证。

- 2026-08-31：`json_text` 兼容解析额外剥离完整、封闭的 `<think>...</think>` 前缀，再验证剩余 JSON；普通解释性文本继续拒绝，推理正文不进入审计或计划数据。

- 2026-08-31：`json_text` 模式现接受完整的 `json` Markdown 代码围栏，再以同一 Zod schema 验证对象；带额外解释文字、无效 JSON 或 schema 不匹配仍会失败关闭。

- 2026-08-31：Agent AI SDK 输出协议新增 `DIPOLE_AGENT_MODEL_OUTPUT_MODE=json_text`。对于不支持 OpenAI JSON Schema response format 的兼容 Provider，Runtime 在单次、零内部重试调用中要求纯 JSON 并以原始 Zod schema 本地复核，失败仍由既有 ModelRouter 预算与审计处理。

- 2026-08-31：新增受版本控制的 `agent-ai-sdk-shadow.yml` 开发 overlay，统一从受忽略 `.env` 读取 OpenAI-compatible Provider、预算、Context profile 和可选 structured-output 声明；移除该 overlay 即回退为基础 metadata Shadow Runtime。

- 2026-08-31：OpenAI-compatible Agent Provider 增加显式 `DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS` 能力声明，默认关闭；声明为 `true` 时才请求 AI SDK/Zod JSON Schema structured output。该配置可由开发环境 `.env` 为已验证 Provider 启用，API key 始终只从环境读取。

- 2026-08-31：standalone Core 的 Agent Capability 在 `core.message.transport=grpc` 时改用惰性 Message History RPC reader，避免 Core 未持有本地消息仓储却参与 `conversation.read` 时发生空指针崩溃；reader 复用 Core service identity，并在 Runtime 关闭时释放连接。

- 2026-08-31：Agent Capability RPC transport 在收到 `UNAVAILABLE` 时会关闭并替换失效的底层 gRPC channel；失败调用不在客户端层重放，Kafka/EventLedger 保持原有幂等重试责任，使 Core 容器重建后的下一次事件尝试重新解析服务地址。

- 2026-08-31：微服务 Compose 的 Agent 现等待 Core `service_healthy` 后才启动，避免首次部署时 Agent gRPC transport 在 Core listener 就绪前固定失败连接。Core 运行中重启后的 retry/reconnect 演练继续作为独立可靠性切片推进。

- 2026-08-31：微服务 Compose 的 Core 现以 `ai.enabled=true` 和 `ai.runtime_mode=remote` 维护唯一 assistant identity；独立 TypeScript Agent Runtime 继续作为 Shadow consumer。Compose 门禁同时固定 Core remote mode 与 Agent UUID，避免运行时目标用户缺失。

- 2026-08-31：Core standalone 现在在内部 RPC 已启用且 mTLS 完整时始终装配基础 Agent Capability RPC，确保独立 TS Runtime 可以执行 `admit_run`、会话列表和会话读取。`conversation.search` 仍由 `internal_rpc.agent_conversation_search_enabled` 单独控制；关闭时 Core 不拨号 Search，调用搜索能力返回受控 `Unavailable`。

- 2026-08-31：增加 OAuth callback Runtime 默认关闭配置契约。只有显式 `enabled=true` 时才要求独立 control secret、固定 lease owner 与 Runtime key 映射；未启用时忽略残留值并保持零 callback surface。该配置尚未由 `index.ts` 消费。

- 2026-08-31：新增默认关闭的 OAuth callback Runtime control-to-executor 集成测试，覆盖 Gateway service-secret 认证、handoff-ID-only body、重复通知去重和固定 Runtime lease owner 转发。测试仅使用内存 fake executor，未构造 Runtime bootstrap、provider 或外部 callback。

- 2026-08-31：Agent Runtime 增加未装配的 OAuth callback control service adapter。它将 Gateway 已认证的 handoff ID 与 correlation 交给 executor，并使用进程固定且校验过的 Runtime lease owner；不接受 principal、authorization code 或配置自定义身份。`index.ts` 不构造该 adapter，默认网络行为不变。

- 2026-08-31：Agent Runtime 增加未装配的 OAuth callback handoff executor。它串联 claim、Runtime key source、AAD 解封与 complete/release：仅解封前失败或 processor 明确返回 retryable 时释放 lease；processor 异常和完成后 terminal 失败保留 lease，避免副作用不确定时重复换码。该 seam 不执行 provider exchange，`index.ts` 未构造它。

- 2026-08-31：OAuth callback handoff claim 响应增加 Runtime-only `owner_user_id` binding。该字段只在 `dipole-agent` mTLS 领取链返回，用于重建 AES-GCM envelope AAD；Gateway control HTTP、浏览器、Kafka、Temporal、日志和审计均不接触它。Runtime client 对字段缺失或格式异常失败关闭，默认 callback 配置保持关闭。

- 2026-08-31：Agent Runtime 增加未装配的 OAuth callback handoff terminal client。它用 `dipole-agent` mTLS metadata 仅提交 handoff ID 和 lease owner，以精确 ID 回包结束或释放租约；principal、授权码、密文、key 与 token 都不会进入该调用。无效输入、回包冲突或 Core 错误均 fail closed，`index.ts` 和 control handler 未注入它。同时为四个既有生成 gRPC 回调补充显式协议元素类型，恢复锁定 TypeScript 的完整 typecheck。

- 2026-08-31：Agent Capability 增加默认未装配的 OAuth callback handoff 终态 RPC。`CompleteOAuthCallbackHandoff` 与 `ReleaseOAuthCallbackHandoff` 仅接受 `dipole-agent` mTLS caller 的 handoff ID 和 lease owner，Core 以 SQLC 条件更新复核有效 lease 后返回无敏感字段的 ID；缺 Store、越权、无效或过期 lease 均 fail closed。TypeScript 生成脚本会优先复用已安装的锁定 `protoc`，确保隔离 Remote GPU 可重复生成。Runtime 终态 client、key open、code exchange、token 生命周期与 callback route 继续未装配。

- 2026-08-31：Agent Runtime 增加未装配的 OAuth callback handoff control handler。它仅认证 `dipole-gateway` service secret，拒绝任何 principal header 和非单字段 `{handoff_id}` body，成功返回 `202`；有界进程内去重避免重复通知的重复下游派发，失败会释放记录以支持重试。该 handler 未接入 `index.ts`、未读取环境变量、未启动 claim/exchange，因此默认网络 surface 不变。

- 2026-08-31：Gateway 增加未装配的 OAuth callback handoff notifier。它只向 Runtime control endpoint 发送严格的 `handoff_id` JSON 和 request/trace correlation，固定 `dipole-gateway` service identity 且不写 principal header；非 `202`、非 loopback HTTP target 或非法 ID 均 fail closed。该 client 未加入 bootstrap、callback route 或开关，因此默认仍无外部 OAuth 流量。

- 2026-08-31：Agent Runtime 增加未装配的 OAuth callback handoff claim client。它通过现有 Runtime-to-Core mTLS metadata 固定 `dipole-agent` caller、仅提交 handoff ID 和 Runtime lease owner，并对返回的 ID、HTTPS binding、SHA-256、envelope、key ID、lease 与授权 expiry 逐项 fail closed。该库未读入 Runtime config、未注册 Gateway notifier、未开启 callback 或 token exchange。

- 2026-08-31：Agent Capability 增加默认未装配的 OAuth callback handoff claim RPC。只有 `dipole-agent` mTLS caller 可领取，Core 固定 30 秒租约并从持久化记录恢复 transaction、issuer、redirect、摘要、Runtime key ID 与 Runtime-only 密文；Gateway、浏览器和用户主体均无法影响 owner binding。缺 Store 时固定返回 `Unavailable`，未注册 callback route、Runtime client、code exchange 或 token 生命周期。TypeScript proto 生成器同时改为使用锁定的 `@protobuf-ts/protoc`，避免依赖宿主 protobuf 安装。

- 2026-08-31：固定 Agent OAuth callback handoff 的双通道 transport contract：Gateway 到 Runtime 的私有 control HTTP 仅通知 handoff ID；Runtime 到 Core 使用 `dipole-agent` mTLS 领取、完成或释放 lease。契约明确禁止 code/state/verifier/ciphertext/key/token 进入 HTTP、Kafka、Temporal、日志或审计，并列出重复通知、重启、Core outage 和过期 lease 的验收矩阵。该项未注册任何 route 或 RPC。

- 2026-08-31：Agent Runtime 增加未装配的 OAuth callback private-key source。它仅接受显式 key ID 到绝对路径映射，每次使用检查目录/文件 owner、权限、链接和大小，确认 PKCS#8 RSA modulus 至少 2048 位后才在 callback 内短时提供 Buffer，并在结束时清零。Runtime 默认启动、Gateway、callback route 与 token exchange 均未读取该 source。

- 2026-08-31：增加 Agent OAuth callback Runtime envelope v1。Gateway 仅用 Runtime RSA public key 通过 OAEP-SHA256 封装每次 handoff 的 AES-256-GCM data key；授权码密文以完整 handoff binding 作为 AAD，Runtime 只用私钥解封并重算 code SHA-256。Go/TypeScript 各自对版本、base64url、长度、RSA/OAEP、AAD、摘要和毫秒时间 fail closed。该原语未接入 callback route、Store writer、Runtime claim、code exchange 或 token 持久化。

- 2026-08-31：Agent OAuth callback handoff 增加 SQLC durable persistence foundation：migration `000053` 保存 Runtime-key 标识、授权码 SHA-256、Runtime-only 密文、transaction/owner/issuer/redirect binding 与有限状态。领取以条件更新实现；租约不得跨越授权过期时间，完成和失败释放均绑定尚有效的 Runtime lease，重启后可从同一 handoff 恢复。该层尚未注册 browser callback、密钥封装、Runtime 领取 RPC、code exchange 或 token 写入，默认部署仍为零 OAuth callback 流量。

- 2026-08-31：补充 Agent OAuth callback handoff 的发布前设计门禁与故障矩阵。明确当前 Core 单次 consume 不支持 Runtime 不可达后的可靠重试，且 callback correlation 尚未定义；因此继续不注册 callback HTTP route。后续 handoff 需要 browser binding、issuer/redirect 核对、Runtime-key ciphertext、lease 状态与 controlled provider 演练。

- 2026-08-31：Gateway 增加未装配的 OAuth authorization transaction consume client foundation。它通过已有 Core mTLS 通道只提交 owner、transaction ID 与 state digest，严格复核返回的 HTTPS binding、expiry 与 base64url 密封 verifier；结果类型禁止作为 HTTP/审计/日志载荷。尚未注册 HTTP callback、Runtime handoff、解封、code exchange 或 token 写入。

- 2026-08-31：Core standalone bootstrap 为 OAuth authorization transaction consume 增加显式、默认关闭的 SQLC Store 注入门禁。只有 `internal_rpc.agent_oauth_authorization_transaction_consume_enabled=true` 且内部 RPC mTLS 已开启才装配受限 adapter；它可与 Memory receipt seam 共存，未增加 HTTP callback、verifier 解封、code exchange 或 token 写入。

- 2026-08-31：Agent Capability RPC 新增默认关闭的 OAuth authorization transaction consume seam。仅 `dipole-gateway` 可使用认证 `RequestContext` 派生 owner 并提交 transaction ID/state digest；Core 先核对 owner/state，再以 SQLC 条件更新单次消费，成功时只返回密封 verifier 和固定 issuer/callback binding。默认 composition 未注入 store，调用固定返回 `Unavailable`，未新增 callback HTTP、换码或 token 写入。

- 2026-08-31：Agent OAuth authorization transaction 已增加 SQLC persistence foundation：migration `000052` 保存密封 verifier、state 摘要、owner、issuer、callback、expiry 与单次 `consumed_at`，消费查询同时要求 transaction/owner/state digest/未过期/未消费。该层未注册 callback 或 Runtime writer，默认部署无 OAuth 事务写入。

- 2026-08-31：Agent OAuth 增加短时授权事务记录契约。记录只保存 state SHA-256 和 AES-256-GCM 密封的 PKCE verifier，并用 transaction、owner、issuer、redirect URI、state digest 与 expiry 作为 AAD；回调必须由后续 Core/SQLC Store 原子按 owner/state/expiry consume 后才能解封。当前没有内存 fallback、callback 路由、换码、令牌保存或 Runtime 默认接线。

- 2026-08-31：Agent Runtime 的 OAuth foundation 新增默认关闭、注入式的 RFC 8414 metadata discovery client。请求只访问由 canonical issuer 派生的 HTTPS URL，固定 `redirect: manual`、10 秒上限、64 KiB 响应上限和 JSON Content-Type；重定向、状态错误、网络失败、超限或无效元数据均 fail closed。该库未接入 Runtime 默认路径，未保存 state/verifier，未执行 code exchange 或 refresh token 操作。

- 2026-08-31：Agent Runtime 增加默认关闭的 OAuth discovery/PKCE foundation：严格派生 RFC 8414 authorization-server metadata URI，要求 exact issuer、HTTPS 端点和 `S256`，并生成不落 URL 的 code verifier、challenge 与 state。该切片不执行 discovery 网络请求、不保存授权材料、不交换 code、不注册客户端，也不改变 MCP Runtime 默认关闭状态。

- 2026-08-31：Remote GPU 在 `bed7a5d0` 重跑 C3 同契约 Go/C++ projection benchmark。Ubuntu 24.04 builder 的 CTest `14/14` 通过，但 C++/Go 吞吐比为 `0.239956`，低于 `1.0` 晋级门槛，判定 `blocked`；Go 继续作为投递 authority，未启动 Dipole 长驻容器或切换灰度。

- 2026-08-31：Remote GPU 在隔离 worktree 为当前 `2ca6b199` 生成 A6 Web Sync Shadow 候选包；Vue production build 与 14 项 observation contract 测试通过，归档以 `0600` 保存并固定 SHA-256。该工件只作为真实观察的不可变输入；主机有 25 个活动登录会话，未启动 Compose、Prometheus 或客户端流量窗口。

- 2026-08-31：Remote GPU 在 `7601e78e` 上完成 A7 Multipart 故障矩阵：确定性 File/Gateway/cleanup contract、Prometheus 7 条告警规则、真实 MinIO/Redis 对账及 Redis restart 注入均通过。矩阵使用随机名称、loopback 绑定的临时容器并在退出后确认清理；默认上传模式仍为 `relay`，预签名直传切流继续需要 24 小时受控证据。

- 2026-08-31：active Agent Runtime 将 retrieval 与 retrieval-to-Context 纳入运行时只读 surface gate。即使绕过 Compose 直接注入环境变量，active/read 或 `promotion_active` profile 也会拒绝扩张到 `conversation.search`；Shadow profile 继续可按独立门禁使用受控检索。

- 2026-08-31：`promotion_active` 现强制使用 `dipole-agent-memory-promotion-` 前缀的独立 Temporal task queue。Runtime profile 和 Compose 渲染门禁会拒绝通用或 read-active 队列，避免 reviewed Memory 提交 Worker 误消费其他任务；默认部署和写入开关不变。

- 2026-08-31：收敛 `promotion_active` 的永久提交失败路径。Memory receipt commit 在 Temporal 用尽重试后，Workflow 现在将错误转换为受限的 Agent Task `failed` 终态并调用既有 `finishAgentTask` 持久化 Run，避免孤立的运行中任务；默认 profile、双开关、mTLS 和共享环境写入状态不变。

- 2026-08-31：默认关闭的 Agent Task 创建页现提供 IM 主界面导航入口。入口同时要求创建页和 Timeline feature flag，点击只跳转受现有路由守卫保护的 `/agent/tasks/new`；不携带身份、配置或任务参数，也不启用 Runtime/Tool/外部服务。

- 2026-08-31：Agent Runtime 增加 `test:temporal:integration` 门禁，固定在内存 Temporal Test Server 复核幂等启动、Worker 恢复、审批、Elicitation、取消、步数预算、后效重放和 reviewed Memory receipt 重试。该门禁不连接 Compose、Kafka、Core、MySQL 或 active authority。

- 2026-08-31：收敛 Agent Task Timeline 单元测试的 RouterLink 依赖。统一挂载 helper 为全部时间线状态注入路由桩，消除认证页面测试的组件解析警告；不改变 Timeline 路由、Artifact 链接、Capability 或默认关闭状态。

- 2026-08-31：补齐默认关闭的 Agent Task 创建前端入口。认证 `/agent/tasks/new` 仅在 `VITE_AGENT_TASK_CREATE_ENABLED=true` 且 Timeline 开关同时开启时可访问；浏览器生成本地幂等 `client_request_id`，客户端严格校验 `{taskId,status:"accepted"}` 后才跳转只读 Timeline。页面不收集 principal、tenant、Agent、Tool、Memory 或 Runtime 配置。Remote GPU Node 22 定向 `15` 项 Vitest、typecheck 与 production build 通过；Pencil CLI 已完成 canonical desktop/mobile/五态创建画板与 2x 导出，并新增 Chromium 初始表单截图回归。active authority、Compose、Kafka 与 Temporal 均保持关闭。

- 2026-08-31：Agent Runtime 增加默认关闭的交互式 Task 创建链路。Gateway 仅从 JWT 注入 principal，向 Runtime 私有控制面转发 `client_request_id` 与目标；Runtime 固定 tenant/Agent 身份，以客户端幂等键派生确定性 Task/Event ID 并交由 Temporal dispatcher 启动。请求体中的身份字段不参与授权。Remote GPU Node 22 定向 Vitest `10` 项、typecheck、build 与 Gateway Go 测试均通过；未启动 Compose、Kafka、Temporal 或 active authority，用户界面与共享环境切流继续待办。

- 2026-08-31：Remote GPU 候选工作树在 `8e99bde7` 上以 Node `22.12.0` 完成完整 Agent Runtime 开发期门禁：`134 passed / 9 skipped` 测试文件、`703 passed / 30 skipped` 测试，以及 `typecheck` 和 production `build` 均通过；验证结束后工作树保持干净。该命令未启动 Compose、Kafka、Temporal 或 active authority，不能替代共享环境演练。

- 2026-08-31：校正 Agent Memory active promotion 的证据台账。Gateway 已具备认证 owner revoke HTTP 到 `dipole-gateway` mTLS Core RPC 的受约束传输链，并由 principal 绑定与审计回包测试覆盖；共享环境 Kafka trigger、该链路的运行记录、promotion overlay 回滚和 24 小时观测仍保持未完成。

- 2026-08-31：将 embedded 聚合运行时、Kafka 组合、消息/同步兼容传输及其测试收敛到 `internal/services/core/bootstrap/embedded/`；Core 的唯一兼容桥继续保留本地回滚能力，服务布局门禁拒绝 `internal/bootstrap/embedded` 回流，独立服务运行路径不变。

- 2026-08-31：Agent Context hydration 现并行读取已授权的会话、Memory 与受 Core 调解的检索证据，并记录低敏数量指标；任一读取失败仍在模型调用前 fail closed，默认检索与共享环境开关保持关闭。

- 2026-08-31：Multipart 策略门禁现以 `contracts/multipart-upload/v1` 为唯一默认值基准，自动比对 release manifest、`config.dist.yaml`、Go 默认配置和 Web 离线回退值。策略更新若遗漏任一层会阻断校验；运行时默认仍为 `relay` 并保留即时回退。

- 2026-08-31：登录入口现复用 canonical Signal Link 紧凑标识，与 README 的深青绿双极、橙色事件脉冲保持一致；新增源代码设计契约，阻止入口页退回无标识的文字品牌。

- 2026-08-31：Remote GPU 在 `6f15f887` 完成不启动 Compose 的完整 Node 门禁：Agent Runtime 通过 `134` 个测试文件、`702` 项测试（另有 `9/30` 项预期跳过），前端通过 `41` 个测试文件、`165` 项测试；两侧 typecheck 与 production build 均通过，生成 Web 产物退出后恢复，候选工作树保持干净。

- 2026-08-31：Remote GPU 候选同步会先拒绝已跟踪的远端修改；对于与目标 Git blob 的 SHA-256 完全一致的未跟踪文件冲突，仅清理可由提交恢复的生成物。内容不同的文件保持原样并 fail closed，避免视觉快照等测试产物阻塞后续已提交 revision。

- 2026-08-31：Settings 的认证页面检查现运行于全部 Playwright 项目，Chromium 保留像素基线，Remote GPU Firefox 已通过真实路由与披露断言。WebKit 二进制已安装，但共享宿主缺少 `libgstreamer-plugins-bad1.0-0` 与 `libavif16`，系统依赖安装留待维护窗口。

- 2026-08-31：完成 Settings 的 canonical Pencil desktop/mobile/四态设计、批准 PNG 导出和 Chromium 受控截图基线。认证账户页继续只呈现签名、本机同步状态、Device Security 入口和当前会话退出边界，跨浏览器视觉回归保持后续独立切片。

- 2026-08-31：Core 新增默认关闭的受控 `conversation.search` assembly。启用需 mTLS，Core 以独立身份调用 Search，再以持久 Task/Run 恢复 principal、scope 和 capability policy；Search RPC 仅允许 Gateway 与 Core 服务身份。默认 Compose 固定关闭，生产检索流量未启用。

- 2026-08-31：Compose 部署层显式固定 Agent retrieval 与 retrieval-to-Context 为关闭：基础 Shadow、active read 和 External MCP Shadow overlay 均写入 `false`，`check-compose.sh` 复核渲染结果，防止宿主环境变量意外扩张默认只读能力面。

- 2026-08-31：将默认 Signal Link 品牌从偏蓝变体恢复为深青绿与橙色事件脉冲方案，统一主字标、IM 标识与 Agent 标识。保留 SVG 文件名和现有 README 引用，运行时行为不受影响。

- 2026-08-31：Agent Runtime 新增默认关闭的 `DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED`。在已启用受权 retrieval 的 Shadow/Temporal read composition 中，Planner 仅从当前事件正文派生有界 query，经 Core 返回最多 8 条命中并按 Context budget 写入带 provenance 的 `untrusted` evidence；检索失败在模型调用前 fail closed。生产 Search、共享流量和默认路径未改变。

- 2026-08-31：修复 Remote GPU 候选代码同步在 squash 合并后产生的陈旧 tracking ref 警告。`dipole-dev/<user>` 现在只在远端使用受限强制 refspec 刷新，`master` 等共享引用继续普通更新，fetch 失败会直接中止同步；部署与活动用户保护策略未改变。

- 2026-08-31：修复 Agent `conversation.search` 扩展后的 legacy Eino 测试桩漂移。共享桩现记录并返回受控搜索证据，并由编译期 `AgentCapabilityV1` 断言覆盖新增方法；运行时检索开关和生产搜索路径未改变。

- 2026-08-31：新增 `multipart-presigned-rollout/v1` 可执行晋级门禁。候选将 Multipart 默认模式从 `relay` 改为 `presigned` 前，必须提交绑定策略 SHA-256 的 24 小时同版本 evidence，并满足直传样本、fallback/failed/expired/checksum 比率、P95、clear alert、relay 回切演练和独立 reviewer；工具输出可审计 receipt，阻断结果返回退出码 `2`。默认上传策略未改变。

- 2026-08-31：Agent Runtime 增加默认关闭的 `DIPOLE_AGENT_RETRIEVAL_ENABLED`。仅在 AI SDK 与受认证 Capability RPC 同时就绪时，Shadow/Temporal read composition 才注册并向模型公开 `conversation.search`；关闭时模型 allowlist、Registry 与执行 Context 均保持 `conversation.list/read`。Core 继续从 Task/Run 恢复身份并约束权限、scope 与有界 untrusted evidence；Elasticsearch、共享环境检索灰度和生产默认路径未改变。

- 2026-08-31：恢复上一版 Signal Link 品牌资产。主字标与紧凑应用图标回到深青双极和橙色事件脉冲，IM 标识表达消息投递链路，Agent 标识表达受控任务能力；保留既有 SVG 文件名和 README 引用，运行时行为不受影响。

- 2026-08-31：新增认证 `/settings` 页面，提供签名更新、低敏 Device Security 入口、当前客户端同步状态与退出登录。设备详情继续由独立隐私页面负责，设置页不显示 IP、节点或连接标识；路由与 Agent 路由测试解除总数硬编码，后续普通页面增加不会削弱 Agent 的认证/开关断言。

- 2026-08-31：校正 Agent Memory promotion 债务口径：Core receipt commit RPC、TypeScript client、Temporal `promotion_active` Activity、mTLS/双开关与隔离重试、grant 撤销、owner revoke 演练已完成；共享环境 Kafka 触发、Gateway revoke 传输、overlay 回滚和观测证据仍未完成，默认写入路径保持关闭。

- 2026-08-31：Remote GPU 为 `45a80b3d475f4ba0317addab9d11ee0cb93397f2` 生成不可变 Web Sync Shadow bundle `/tmp/dipole-dev-horeb-web-sync-shadow-45a80b3d.tar`，SHA-256 为 `0c458602868170dbb45933f1c48fa0f9ba22c5978d6d79cb2d007cb0344bfdd5`。过程未启动 Compose、Prometheus、客户端流量或 GPU 任务；它仅为后续 24 小时观察提供可复核输入。

- 2026-08-31：更新为 Signal Link 品牌体系：实心端点、空心端点与连接轨迹取代旧的蓝色成对极形，统一用于主字标、IM、Agent 和紧凑应用图标；运行时行为不受影响。

- 2026-08-31：新增默认关闭的 Agent `conversation.search` 契约。Core Agent Capability 从 Task/Run 恢复身份后，经可注入 Search port 查询，要求独立 permission 与 `conversation/*/read` scope；RPC/TS 双端拒绝客户端 principal、超限输入和冲突 evidence，并限制 query、结果数及正文。默认 composition 未注入 Search port，生产 Elasticsearch 与 Runtime 注册继续关闭。

- 2026-08-31：恢复蓝色双极 SVG 品牌体系。README 和文档入口重新采用更高识别度的成对信号标记、冷色画布与深蓝字色；历史单色 PNG 继续弃用。本项只影响品牌呈现。

- 2026-08-30：固定 Agent 检索 Context 的安全契约：Runtime 不直连 Search，后续由 Core 以 Task/Run 恢复的身份、permission 与 scope 调解查询；命中只能作为有界 `untrusted` evidence 进入 Context Compiler。生产 Elasticsearch、默认 Runtime 和跨会话检索继续关闭。

- 2026-08-30：校正平台计划与已合并实现的交付口径：Context Compiler v1/v2 已编译预算、可信度、会话证据、Memory、Capability 元数据和 route tokenizer，完整检索编排仍未完成；F2 仅保留 Settings，F3 仅保留 MCP 多轮、敏感授权、产品入口与未覆盖视觉回归。

- 2026-08-30：修复 Remote GPU 临时候选引用在 squash 合并后的同步失败。默认 `dipole-dev/<user>` 现以远端 tip 的精确 lease 更新，允许复用单一候选引用并拒绝并发覆盖；显式 `master` 或其他共享分支继续只接受快进推送。

- 2026-08-30：将 Dipole Signal 主标及 IM/Agent 标记从偏蓝的渐变校正为深青绿色，保留双极信号、橙色事件脉冲和既有 SVG 文件名，避免 README 与文档入口产生蓝色产品标识的观感。

- 2026-08-30：F2 Device Security 完成 Pencil desktop/mobile/七态设计、认证 `/devices` 路由与严格低敏会话解析。公共设备投影移除 IP、节点和原始 User-Agent；新增 `logout-others` 精确动作，以认证请求的稳定设备 ID 排除当前设备，单设备与批量登出均要求前端明确确认并在结果后重读权威列表。Remote GPU Node 22 已在候选切片通过前端 `40` 个测试文件、`162` 项测试、typecheck 和 production build；Playwright 的 Chromium/Firefox/WebKit binaries 当前不可用，交互执行与 Chromium 像素快照保留为独立环境准备切片。

- 2026-08-30：校正 External MCP Shadow 的运行口径：`external_mcp_shadow` 已作为独占、默认关闭的 TypeScript Runtime mode 接入受控 Kafka/Temporal/Capability RPC 生命周期；基础 Compose 仍为 `foundation`，未配置完整 Shadow 依赖时零外部连接或消费。文档、架构债务和 Agent 面试材料现明确区分该本地/隔离能力与尚未完成的共享环境、DNS/TLS、凭据和生产授权证据。

- 2026-08-30：F2 File Directory 完成 Pencil desktop/mobile/状态矩阵、批准导出和认证只读 `/files` 路由。Core 经 SQLC owner-scoped 文件 UUID cursor 和版本化 gRPC 返回低敏目录投影；HTTP、Swagger 与 Vue 严格排除对象键、存储 URL、校验值和上传会话，下载逐项重新授权，读取失败清空旧状态。Remote GPU Node 22 在提交 `a29d9927` 通过 38 个前端测试文件、157 项测试、typecheck 和 production build。上传仍由会话编辑器和既有 MinIO Multipart 数据面处理；删除、分享、跨浏览器视觉回归和预签名直传默认切流继续作为独立切片。

- 2026-08-30：修复 Remote GPU `node-test` 的 Vite 构建清理：原先对 Web 产物反向应用 diff 会误删基线 hashed assets，现改为仅对受控 `internal/services/core/server/webapp` 目录恢复固定 `HEAD` 后清理新文件。Remote GPU 在 `3f1f3936` 复跑后工作树干净，未启动 Compose 或 GPU 任务。

- 2026-08-30：Agent Project Guardian 增加版本化低敏 subscription 预筛语料：四类关注项目状态、四类干扰事件、双 reviewer 100% agreement 与共享 deterministic evaluator 已纳入回归。规则 evidence 直接复用 production `matchEventSubscriptions`，避免测试手写 decision 造成语义分叉。Remote GPU Node 22 验证 Agent Runtime `133` 个测试文件、`695` 项通过，typecheck/build 通过；素材固定使用 synthetic `fixture:` 标识，不含真实会话、用户或模型输出。真实 corpus、retrieval relevance、候选模型成本阈值和 shared shadow 观察仍由 `AD-038` 管理。

- 2026-08-30：F2 Group Directory 完成 Pencil canonical desktop/mobile 目录、loading/empty/unavailable/dismissed/hot-group 状态矩阵、三个复用组件和批准导出，并交付认证只读 `/groups` 路由。目录从会话投影派生范围后读取权威群详情，异常时清空旧状态；Remote GPU Node 22 通过 36 个前端测试文件、152 项测试、typecheck 与 production build。热群继续采用 `notify + pull`，群成员与管理写操作未在目录中开放。

- 2026-08-30：收紧 SQLC-only 数据访问门禁：`check-sqlc.sh` 现作为唯一权威入口，拒绝 GORM module、任意 Go import/selector 和运行时 `AutoMigrate`，并以临时 Git fixture 覆盖 SQLC-only 基线及三类回流场景，防止后续微服务与多语言演进重新引入第二套 ORM 模型。

- 2026-08-30：Remote GPU 为 `253cf3d29ec79a0f58bcc06c58f5fbad20974b45` 生成不可变 Web Sync Shadow bundle `/tmp/dipole-dev-horeb-web-sync-shadow-253cf3d2.tar`，SHA-256 为 `f4a3a90c5ed5d7d04575a9b939ca738b7e6bd92f53fe3ef818a7249941725f9d`。该动作未启动 Compose、Prometheus 或客户端流量，只提供后续 Observation Session 的候选输入。

- 2026-08-30：收紧 Go/Eino Agent 迁移边界：服务布局门禁要求 production legacy import 只能来自 embedded Kafka composition，`github.com/cloudwego/eino` 只能位于 `internal/services/agent/legacy`。独立 TypeScript Runtime、Gateway 与服务入口无法绕过 Capability、Temporal 与 promotion 门禁回接旧链路。

- 2026-08-30：F2 Contact 增加受认证只读目录 `/contacts`：前端严格解析 `/api/v1/contacts` 的联系人投影，读取失败会清空旧数据并提供重试入口；Remote GPU Node 22 通过 `34` 个前端测试文件、`147` 项测试、typecheck 与 production build。备注、拉黑、删除、申请处理及跨浏览器视觉回归继续按独立权限切片推进。

- 2026-08-30：F2 Contact 完成 Pencil canonical desktop/mobile 管理稿、申请与安全状态矩阵、可复用 Contact Row/Request 组件及 2x 评审导出。该切片只建立视觉基线，未新增前端路由、权限或 Contact API 行为。

- 2026-08-30：修正 IM 深度面试问答的历史叙事：当前 `messages`/`user_sync_inbox`/设备 Cursor 已分别承担 Message Store、Sync Store 与多端位点；Redis 保持实时状态与可丢弃加速职责。Cassandra 迁移、A6 真实 Web Sync 观察和旧 Offline 兼容窗口仍按既有门禁推进。

- 2026-08-30：Remote GPU 在提交 `37d02383` 上拉取 `quay.io/prometheus/alertmanager:v0.28.1` 并通过 `amtool check-config`；配置包含全局配置、路由和一个 discard receiver。该证据只验证 Alertmanager 配置，未启动常驻 Compose 服务或外部通知。

- 2026-08-30：开发 observability profile 新增 loopback-only Alertmanager，并由 Prometheus 配置转发告警。仓库 receiver 为 discard，用于配置与投递链路验证；生产通知凭据和目标继续通过受控部署层配置。

- 2026-08-30：Web Sync 开发观察新增隔离 Prometheus smoke 与远程入口；Gateway/Prometheus 宿主端口默认限制为 loopback，验证必要服务 metrics target 后自动清理。该 smoke 不启动 24 小时观察会话，也不构成晋级证据。

- 2026-08-30：移除无引用的旧蓝色 PNG 主标；项目入口统一使用深青绿与橙色 Signal SVG 视觉系统。

- 2026-08-30：Remote GPU 已为 `b0e4f2523392796837af6130f6c3b27c5b5400de` 生成不可变 Web Sync Shadow bundle `web-sync-shadow-b0e4f2523392-20260830`，SHA-256 为 `7d815e968dfd489b8c6ec43f7ebe27a46cac15e28b593bb414db904a945151a5`。构建与归档准备完成，尚未连接 Prometheus 或启动 24 小时真实流量观察。

- 2026-08-30：修正 Web Sync 候选 bundle 默认来源至 Vite 实际生产输出 `internal/services/core/server/webapp`，避免发布归档错误读取已废弃的 `frontend/dist` 并在远端构建后失败。

- 2026-08-30：修复前端 Agent 路由安全契约遗漏 Artifact 页面的问题：测试现覆盖 `agent-artifact` 的认证与独立 feature flag，并按当前 8 个受认证页面校验，恢复 Shadow Web 候选构建门禁。

- 2026-08-30：按项目视觉方向恢复 README 的深青/橙色 Signal 品牌：双极主标表达事件脉冲，IM 与 Agent 分别表达消息投递和受控能力。蓝色 SVG 版本不再作为项目入口视觉，运行时行为不受影响。

- 2026-08-30：Remote GPU 一次性 worktree 通过 External MCP Shadow 全栈演练：隔离 MySQL/Kafka、Temporal、mTLS Core fixture 与本地 MCP 共同验证 2 个事件、持久 ledger、1 次 allowlisted Tool/Artifact、同 group 重启去重和过期 readiness 拒绝。证据固定为 `production_authority=false`，未连接共享服务或外部 MCP。

- 2026-08-30：统一 README SVG 品牌资产与既有蓝色 Dipole 主标：主标、IM 与 Agent 图形共享双极信号母版，并新增资产用途说明。此次调整只影响开源项目介绍与文档呈现，不改变服务、协议或 Agent 权限状态。

- 2026-08-30：修复 Agent active overlay 的 Kafka consumer group 启动边界：配置层与消费器层均要求 active 使用独立 `dipole-agent-active-*` group，shadow 继续限定 `dipole-agent-shadow-*`。专项测试验证 active 直投消息进入 Temporal dispatcher；Remote GPU 隔离 Node 22 worktree 在独立 `npm ci` 后通过 `132` 个测试文件、`693` 项测试、typecheck 与 build。该证据不包含真实 broker、Core mTLS、Temporal Worker 或 Memory promotion 提交。

- 2026-08-30：主 README 收敛为成熟开源项目入口：新增可维护 SVG 主标、IM/Agent 标记、产品概览、可验证的本地启动与检查入口、文档导航和贡献规范；架构细节、更新日志与面试材料继续归档到对应文档。

- 2026-08-30：Temporal/Core/MySQL receipt 联合演练补齐 owner-scoped Memory rollback：首个 receipt 经 durable retry 收敛为同一条 Memory，撤销 active grant 后预 admission receipt 被拒绝且零写入，owner application control 随后撤销已写入 Memory。Gateway owner revoke 的网络传输、Kafka trigger、共享 overlay rollback 仍待验证。

- 2026-08-30：学习与面试材料拆分为 `Dipole IM` 与 `Dipole Agent` 两个独立项目口径。入口页只负责选择材料和说明受控集成边界；双文档门禁、README、文档目录和架构清单已同步，避免在投递和现场介绍中混用 IM 与 Agent 成果。

- 2026-08-30：Temporal/Core/MySQL receipt 联合演练新增 admission 后 grant 撤销场景：同一有效 grant 预先 admission 两个 Task/Run；首个完成持久重试，撤销后第二个 receipt 经 mTLS 被 Core 拒绝，且候选没有产生 Memory。该隔离证据仍不包含 Kafka 或业务级 Memory rollback。

- 2026-08-30：Remote GPU 隔离联合演练将 Temporal、实际 Core receipt adapter、TCP+mTLS 与临时 MySQL 8.4 串联。首次提交后故意使 Worker Activity 失败，重试复用同一 receipt 并返回同一持久 Memory；同时修复候选晋级在重试时误将首次 `ValidFrom` 视为冲突的问题。Kafka、共享环境 grant 撤销和 overlay 回滚证据仍待完成。

- 2026-08-30：将 Agent receipt 的隔离 MySQL 合约提升为 loopback TCP+mTLS：临时 CA、Core 方法白名单、`dipole-agent` 证书身份、protobuf adapter 和 MySQL 持久事务在同一测试链通过。Remote GPU MySQL 8.4 验证通过；Temporal Worker 同组运行仍待完成。

- 2026-08-30：扩展 Agent Memory receipt 的隔离 MySQL 合约至实际 Core receipt adapter：经 Agent service 身份拦截器执行完整 migration、首次提交、同 receipt 幂等重放与 grant 撤销后拒绝。Remote GPU MySQL 8.4 通过；mTLS 网络握手与 Temporal Worker 联合演练继续待完成。

- 2026-08-30：Remote GPU 一次性 worktree 使用内存 Temporal test server 验证 Agent Memory promotion workflow：prepared receipt 可持久返回，受控首次 commit 失败后会重试且保持同一低敏 receipt binding。该测试未启动 Core、MySQL、Kafka 或 active Compose，联合演练仍待完成。

- 2026-08-30：收敛 Web Multipart 故障重试：浏览器断连及 `408`、`429`、`5xx` 保持指数退避；确定不可恢复的预签名 `4xx` 立即上抛，避免重复 PUT。专项 28 项测试、类型检查与生产构建通过；默认 `relay` 路径和预签名切流开关保持不变。

- 2026-08-30：Remote GPU 隔离验证 MinIO Multipart lifecycle 与 restart smoke：乱序/替换分片、完成、内容校验、重复 Abort，以及服务重启后的继续上传均通过；预签名默认切流和浏览器网络故障矩阵继续关闭。

- 2026-08-30：修正 active receipt authority 的默认关闭结构门禁，要求 production Bootstrap 同时保留显式 receipt 开关和 mTLS 前置条件；默认 Agent Runtime 写 Capability 继续禁止注册。

- 2026-08-30：Active Agent 部署手册补充隔离 MySQL、mTLS RPC 与 Temporal retry 三层验证顺序，并明确三者不能替代 Core/Temporal/MySQL 联合演练。

- 2026-08-30：补齐 embedded Agent active Run admission 的 promotion grant authorizer 注入，使 admission 与 receipt invocation resolver 使用同一持久授权边界；默认 active Worker 仍关闭。

- 2026-08-30：Core 与 embedded receipt commit composition 现注入持久 active Runtime promotion authorizer，使已开启且已授权的 receipt 可通过 Invocation Resolver 复核；缺失或失效 grant 继续 fail-closed，默认 active 路径未改变。

- 2026-08-30：新增隔离 MySQL receipt promotion contract。它经完整 migration 验证持久 Task/Run、active grant、accepted candidate/review、首个晋级、同 receipt 幂等重放和撤销 grant 后拒绝；同时修复 candidate promotion 未 canonicalize Memory lineage 而触发 MySQL 约束回滚的问题。默认 active Worker 路径继续关闭。

- 2026-08-30：学习、简历与面试主文档新增合并前复核清单；门禁要求项目定位、简历描述、现场介绍、工程故事和独立问答入口保持完整。后续架构、Agent、前端与性能切片须按清单同步叙事、证据与限制。

- 2026-08-30：新增 `drill-agent-memory-promotion-rpc.sh`，以临时 CA、loopback Go fixture 和 TypeScript generated gRPC client 演练 reviewed receipt 的跨语言 mTLS 提交。Fixture 覆盖 `dipole-agent` 身份、错误 secret/证书拒绝、prepared receipt 序列化及低敏回包绑定，并支持 `DIPOLE_GO_BIN` 固定远端工具链；不启动 Docker、Temporal、Kafka 或 MySQL，不写入真实 Memory。

- 2026-08-30：Remote GPU 的隔离 Node 22 worktree 已复核 Agent Memory promotion 基线：`promotion_active` profile 6 项、in-memory Temporal receipt preparation/retry 2 项与 TypeScript typecheck 全部通过。该验证未启动 Compose、未占用 GPU，且不连接共享 Core、Kafka 或真实 grant，因此默认写路径保持关闭。

- 2026-08-30：`check-compose.sh` 新增 Agent Memory promotion overlay 门禁：叠加 active 与 promotion 配置时，验证 Core receipt commit、`promotion_active` Worker、显式 `operator_approved` authority 与只读能力边界；缺少 authority 时 Compose 渲染必须失败。该检查只验证静态输入，不构成共享环境提交、grant 撤销或回滚证据。

- 2026-08-30：学习、简历与面试主文档加入可执行维护门禁。架构文档检查现验证主文档受版本控制、README/文档目录入口、核心章节和能力卡片模板字段；每个改变服务边界、默认路径、用户流程、性能结论或 Agent 权限的合并切片均须同步叙事与证据。

- 2026-08-30：Temporal Memory promotion 集成测试新增 `commit=true` 路径：首次 receipt commit Activity 临时失败后由 Temporal 重试，重试复用相同 receipt hash 并收敛到同一低敏 Memory binding。该测试使用隔离 Temporal test server，不连接 Core、Kafka 或共享环境，默认 Worker 写路径继续关闭。

- 2026-08-30：新增 Memory receipt `promotion:memory-worker-drill` evidence 契约与 CLI，将共享环境的候选版本、manifest/configuration/evidence 摘要、grant、首个提交、重试幂等、失效 grant 拒绝和回滚结果收敛为低敏 decision。CLI 不连接 Runtime、Temporal、Core 或数据库，也不提供写入授权；缺少任一演练结果固定为 `blocked`。

- 2026-08-30：新增默认不加载的 `agent-memory-promotion.yml` overlay 与 `promotion_active` Temporal Worker profile。提交 Activity 只有在 active Runtime、Temporal、Capability RPC mTLS、显式 `operator_approved` authority、Runtime 开关和 Core receipt commit 开关同时成立时才会装配；Profile 继续拒绝 Control、MCP、自动 Memory、subscription 与消息写入。共享环境重放、失效 grant 和回滚证据仍待完成。

- 2026-08-30：Core bootstrap 为 receipt commit 增加 `internal_rpc.agent_memory_promotion_receipt_commit_enabled` 显式开关，默认关闭；仅在内部 RPC 已启用 mTLS 时才构造并注入 commit service。Compose 同步保持 `DIPOLE_AGENT_MEMORY_PROMOTION_RECEIPT_COMMIT_ENABLED=false`，Temporal Worker 仍未装配写 Activity。

- 2026-08-30：为 reviewed Memory receipt 增加可注入的 Temporal commit Activity。Workflow 仅在显式 `commit=true` 的受控输入完成后提交 prepared receipt，基础 Worker 固定拒绝；Activity 只转发 receipt 与 correlation 至 active RPC client，持久化结果只保留低敏 binding。Worker 组合、Core bootstrap、Compose profile 和自动写入保持关闭。

- 2026-08-30：收紧 `CommitMemoryPromotionReceipt` 的返回契约为低敏 receipt response，移除 `AgentOwnedMemory` 中的正文、资源与 owner 字段；TS active client 会先校验 receipt v2，再复核 Memory ID、类型、状态、provenance 和 receipt hash。默认 Worker、Temporal Activity、Core bootstrap 和自动写入继续关闭。

- 2026-08-30：新增 `CommitMemoryPromotionReceipt` 内部 protobuf/gRPC seam，仅允许认证的 `dipole-agent` 调用并由 Core caller policy 放行；未注入 commit service 时 fail closed 为 `Unavailable`。Gateway owner RPC 不获得该方法，Temporal Activity、Runtime client、Core bootstrap 和自动写入开关继续关闭。

- 2026-08-30：新增默认未接线的 Core Memory promotion receipt commit service。它从持久化 Task/Run Invocation 恢复 owner，要求 active `dipole-agent` Runtime，并以 TS/Go 共用的毫秒级 ISO canonical SHA-256 向量复核 receipt 后委托既有 candidate/review 幂等事务；RPC、Temporal Activity、启动链和自动写入开关继续关闭。

- 2026-08-30：补充 Agent Memory promotion active executor v1 契约，固定 `dipole-agent` 的专用内部提交边界、Core 侧 Task/Run 身份恢复、active admission/promotion grant 验证、receipt 重算与幂等重放要求。当前仅完成设计与测试矩阵，默认配置继续为 receipt-only，未启用 Runtime 自动写入。

- 2026-08-30：Agent Artifact metadata 的受认证只读流程已在 Chromium、Firefox、WebKit 三浏览器完成本地功能复核，继续固定为 metadata-only 且无下载面。视觉快照仍仅以 Chromium 受控 fixture 为基线；共享环境、正文和下载授权保持关闭。

- 2026-08-30：新增默认关闭的 Agent Artifact metadata 页面：Pencil canonical 已补齐 desktop/mobile/state matrix 与批准导出，Vue 仅按经认证的精确 64 位内容寻址 ID 读取类型、版本、标题、媒体类型、大小、Task/Run、创建时间和 SHA-256。Timeline 仅对 `artifact` event 提供条件跳转；失败清空旧 metadata，正文、对象键、metadata JSON、下载和写控制继续关闭，Chromium visual fixture 仅覆盖低敏本地基线。

- 2026-08-30：Agent Memory candidate promotion 新增显式持久化 `target_memory_type`，贯通 Gateway、版本化 gRPC、Core 与 MySQL 事务。已接受 review 的 owner 可写入 episodic、semantic、procedural 或 observational Memory；空字段保持 observational 兼容，task-scoped working 在 Gateway 与 Core 双层拒绝，重复请求保持幂等。TS receipt v2 到 active executor 的接线仍默认关闭。

- 2026-08-30：Agent Task Timeline v1 增加可选的内容寻址 `artifact_id`，通过 MySQL 主投影与失败修复队列、Core gRPC、TypeScript Runtime 和前端解析贯通。新 `artifact` 事件要求精确小写 SHA-256 ID，其他事件携带该字段会拒绝；该能力只支持 owner-scoped metadata 关联，正文、对象键、下载与 Artifact 页面仍未开放。

- 2026-08-30：学习、简历与面试主文档新增合并切片维护记录模板，要求针对可讲能力同步维护对外表述、演示、证据、追问、限制和复核条件；根 README 增加直接入口，文档叙事继续以代码、契约、测试和运行记录为准。

- 2026-08-30：Gateway 新增默认关闭的 owner-scoped Artifact metadata 读取 seam，复用认证 Core gRPC 绑定 principal，并要求精确 64 位 SHA-256 Artifact ID、校验正文长度与 SHA-256 后只返回身份、Task/Run、类型、版本、标题、媒体类型、摘要和大小；Artifact 正文、对象键与 metadata JSON 继续不暴露给浏览器，下载与 Web 页面等待独立契约。

- 2026-08-30：Agent Definition Catalog 的认证读取流程已在 Chromium、Firefox、WebKit 三浏览器通过；视觉快照仍只以 Chromium 固定，未将该功能验收外推为 active Runtime、写 Capability 或共享环境证据。

- 2026-08-30：新增默认关闭的 Agent Definition Catalog：Pencil canonical 设计包含 desktop/mobile/state matrix 与复用组件，Vue 页面通过 authenticated Gateway 查询精确 Definition version 和 scope，异常时清空旧目录；Chromium visual regression 固定只读边界。页面不提供 Runtime、激活、编辑或写 Capability 控制，学习与面试主文档同步增加能力卡片索引。

- 2026-08-30：新增 `agent-external-mcp-shadow.yml` 受控 Compose overlay，强制外部 MCP Profile、I/O/route manifest、只读 secrets、独立 Kafka group 与 Temporal 输入；Compose 门禁覆盖完整渲染和缺 Profile 拒绝，关闭开关时 Runtime 不读取残留 Profile。基础 Compose 继续关闭外部 MCP，真实公网与共享环境证据仍待完成。

- 2026-08-30：新增 Agent Task Timeline Chromium visual regression，以受控低敏 fixture 固定 revision、Capability、等待审批、分页入口和 raw event kind 展示边界；学习与面试主文档同步加入滚动维护契约、设计证据与对应追问，其他浏览器和全页面视觉基线仍待完成。

- 2026-08-30：通过真实 Pencil CLI 与原子 safe-edit 包装器完成 Agent Task Timeline desktop/mobile/state-matrix 增量设计，新增只读 metadata/provenance 组件和批准导出；完整页面视觉基线与其余前端流程继续按设计计划推进。

- 2026-08-30：项目学习与面试主文档增加服务边界、SQLC、Temporal Durable Approval、远程验证和 C++ 数据面证据速查与高频追问，明确区分已验证、默认关闭和规划能力。

- 2026-08-30：Agent Runtime 在 active 启动链执行只读 profile 校验，Control、MCP Server、External MCP、Memory 或 subscription Shadow 任一开启都会在构造运行资源前停止；Shadow 运维路径保持可配置。

- 2026-08-30：新增项目学习与面试主文档，集中维护简历描述、60 秒/3 分钟介绍、工程故事、高频问答、学习路线与面试前检查；详细题库保留为扩展阅读，状态标签要求区分已验证、默认关闭和规划能力。

- 2026-08-30：Agent active overlay 强制固定 direct-target、Memory、Control、MCP Server 与 External MCP 为关闭；Compose 门禁覆盖 host 环境试图开启 Control/MCP 的回归，user-gray 继续只允许只读 Temporal 路径。

- 2026-08-30：新增 Agent Active 部署运行手册，明确 user-gray 的 Provider、Kafka、Temporal、Capability RPC、五类 Eval、operator grant 与维护窗口证据边界，并提供静态渲染和移除 override 的回滚步骤。

- 2026-08-30：Agent active Compose overlay 现要求独立 Kafka consumer group、OpenAI-compatible Provider、v2 Context profile 与 Temporal endpoint/namespace/task queue，并强制 `ai_sdk` 和 `read_active`；缺少任一输入即在 Compose 渲染阶段拒绝。基础微服务 Compose 继续固定 Shadow，移除 override 可立即回滚。

- 2026-08-30：Agent Runtime 的 `ai_sdk` 模式改为显式 OpenAI-compatible Provider adapter；Provider name 绑定全部模型 route 前缀，base URL、API key 与 route 在启动前校验，避免把字符串 route 作为模型对象使用。默认 `metadata` 路径保持不创建 Provider，真实 active 仍需独立完成 Temporal、Kafka、Capability RPC 和 authority 证据。

- 2026-08-30：Agent Memory promotion receipt 新增 `v2`：将 observational candidate 与显式目标 Memory 类型共同纳入确定性哈希和 Temporal replay 绑定；历史 `v1` receipt 继续可读，但缺少类型绑定时停止 replay 并要求重新审批。External MCP Shadow 组合配置不完整时新增零进程启动回归测试，默认关闭路径保持不创建 Worker、RPC 或网络资源。

- 2026-08-30：Agent Runtime 新增五类 Memory 类型策略边界，明确 working 为任务级临时记忆，其余类型需经过 review；候选生成继续限定为 observational，目标类型必须显式指定且不会凭借类型校验获得写入权限。补充 Agent 文档与跨服务契约导航，并通过 Memory 测试、TypeScript typecheck 和文档索引门禁。

- 2026-08-30：新增 `scripts/check-doc-indexes.sh` 并接入架构文档门禁，校验项目、Agent 和跨服务契约索引中的本地相对链接，降低文档重排后的断链风险。

- 2026-08-30：新增 `contracts/README.md` 契约总索引，并将 Agent Capability、Task、Memory、MCP、发布、修复和评估契约按领域归类；Agent 文档入口与项目 README 同步链接，明确版本兼容、证据哈希和 authority 边界。

- 2026-08-30：新增 `docs/agent/README.md` 作为 Agent Runtime 专属文档入口，统一阅读顺序、运行时职责、默认开关和真实环境证据边界；总文档目录与架构文档清单同步收录，便于后续滚动维护。

- 2026-08-30：为 Multipart `upload_part` retry counter 增加 Prometheus 连续重试告警与 promtool firing 时序测试；告警仅聚合 operation，保持低基数和业务路径不变。

- 2026-08-30：Multipart Core 为同一 session 重复上传同一 `partNumber` 增加低基数 `upload_part{outcome="retry"}` 计数；retry 与最终结果分开记录，耗时只采样一次，旧 session store 可通过可选接口兼容，上传默认路径和回滚语义保持不变。

- 2026-08-30：Multipart cleanup 的 Prometheus textfile 输出新增低基数生命周期状态指标，覆盖 active、expired、aborted、failed、扫描完成状态和清理耗时；`--metrics-output` 可在仅 cleanup 场景使用，reconciliation 指标保持兼容，默认 dry-run 和 relay 回滚路径不变。

- 2026-08-30：Remote GPU 真实联合 reconciliation smoke 通过：隔离 MinIO+Redis 依次识别匹配 session、missing Redis metadata 和 Redis orphan drift，退出清理无残留；测试使用完整对象键等待 MinIO listing 收敛。

- 2026-08-30：新增隔离 MinIO+Redis Multipart reconciliation smoke，真实验证匹配 session、Redis metadata 缺失和 Redis 孤儿三种跨存储状态；临时容器、bucket 和未完成 upload 自动清理。

- 2026-08-30：新增 Redis Multipart session TTL 回归测试，验证 metadata 与 parts hash 同步过期、分片续传刷新 TTL，以及完成收据按独立 TTL 到期；覆盖 Redis 状态过期后安全停止续传的持久化边界。

- 2026-08-30：补充 Multipart 过期 session fail-closed 回归测试；status、presign、register、upload、complete 和 abort 入口均在 session 不存在时停止，避免过期会话继续调用 MinIO。

- 2026-08-30：新增真实 MinIO Multipart cleanup 生命周期验证：创建带 part 的未完成 upload，按 cutoff 选择并 Abort，随后重新列举确认 upload 清除；针对 MinIO listing 收敛使用有界等待，隔离桶自动清理。

- 2026-08-30：Remote GPU `remote-dev.sh sync` 与 `multipart-smoke` 在最新 `master` 上验证通过；未显式设置 Go 根目录时自动选择用户态 Go `1.27.0`。此前偶发的小写 SSH 别名问题在当前环境无法复现，暂不修改主机 SSH 配置。

- 2026-08-30：真实 MinIO cleanup 生命周期 smoke 已通过：未完成 upload 的 listing、cutoff 选择、Abort 和清理后无残留均完成验证；测试对当前 MinIO listing 收敛采用有界等待与完整对象键隔离，生产清理前缀保持不变。

- 2026-08-30：修复直接调用的 MinIO Multipart smoke 脚本忽略 `DIPOLE_REMOTE_GO_ROOT` 的问题；显式用户态 Go 现在会经过可执行性校验并优先加入 `PATH`，默认固定 `GOTOOLCHAIN=local`，避免远端测试意外下载工具链。14 项远程入口契约测试、Remote GPU 两项真实 MinIO smoke 均通过。

- 2026-08-30：确认 Remote GPU 并行启动策略：开发阶段允许在已有 GPU 任务运行时启动隔离的 CPU/容器型 Dipole 构建、Smoke、集成测试和压力测试；活动登录会话仍默认保护，确需 GPU 的任务必须单独声明设备、显存预算和冲突检查。该授权不允许停止、重启、迁移或修改其他 GPU 任务。

- 2026-08-30：Web Multipart 上传增加 `AbortSignal` 传播，覆盖 presigned PUT、relay API、part 重试和页面卸载；取消后停止新请求并保留可恢复 session，避免页面销毁后的无效重试。默认 relay/presigned 策略保持不变。

- 2026-08-30：强化 Multipart 中断恢复集成验证：中断流使用同一 part 编号重试后完成上传并校验最终对象内容，确认失败尝试不会污染 Complete 结果；完整浏览器断网、过期会话和网关限流矩阵仍待完成。

- 2026-08-30：扩展真实 MinIO Multipart 集成契约，模拟客户端在 part 读取过程中断后使用同一 part 编号重试，并验证失败尝试不污染后续会话；默认 relay/presigned 开关和生产路径保持不变。完整浏览器断网、过期会话和网关限流矩阵仍待完成。

- 2026-08-30：在 Remote GPU 对 `master` 提交 `bd7283d1` 完成远程 canonical 验证：Go 全量测试、服务布局与架构文档门禁通过；Agent Runtime `125` 个测试文件/`665` 个测试通过并完成 typecheck/build，Frontend `29` 个测试文件/`114` 个测试通过并完成 typecheck/Vite build。验证仅使用 CPU/用户态工具链，未启动业务 Compose 或触碰 GPU 任务；Node `22.12.0` 对部分依赖要求 `22.22.2+` 仅产生非阻断警告。

- 2026-08-30：在 Remote GPU 完成 MinIO Multipart 服务重启恢复 smoke；分片跨重启保留并成功完成对象，临时容器和数据卷已清理，客户端断网与预签名切流仍待验证。

- 2026-08-30：在 Remote GPU 使用现有 Go `1.27.0` 完成隔离 MinIO Multipart smoke，验证乱序/替换/完成/内容校验/重复 Abort；临时容器已清理，完整故障矩阵继续待完成。

- 2026-08-30：对齐 Eino 版本评估状态：`v0.10.0-alpha.26` 只读 spike 报告已归档，当前稳定回滚基线为 `v0.9.17`；未将 alpha API 加入默认构建、Compose 或 Agent authority。
- 2026-08-30：Multipart cleanup 对 nil MinIO client 和批量 listing error fail-closed；错误总数完整保留，错误详情最多 32 条并显式标记截断。

- 2026-08-30：对齐平台总计划、Kafka 集群说明和架构债务摘要，明确业务高可用依赖组合已可渲染，组件级演练与真实业务故障收敛仍分开计证。
- 2026-08-30：修正业务 MySQL Router Compose 门禁对 healthcheck 参数位置的错误假设，改为按端口值语义检查；相关业务拓扑契约和全量 Compose 校验通过。
- 2026-08-30：业务集群 override 接入 MySQL Router 和三节点 InnoDB Cluster，应用继续通过稳定的 `mysql:3306` writer endpoint 连接；单节点微服务 Compose 保持默认回滚路径。已加入 Compose 渲染与业务拓扑契约门禁，真实业务故障切换仍待 Remote GPU 独立演练。
- 2026-08-30：为微服务 Gateway 增加可配置宿主端口，业务集群演练默认使用 `18080`，允许多个隔离 Compose project 并行运行；默认端口仍保持 `8080`。
- 2026-08-30：新增隔离业务集群生命周期入口 `scripts/bench/business_cluster_topology.sh`，提供 `config/up/status/down`、活动会话保护、GPU 并行资源提示和无卷删除回滚；真实业务故障演练继续要求显式批准与独立 project。

- 2026-08-30：新增业务集群 Compose override，将 Kafka 三节点和 Redis Sentinel 接入微服务业务服务的客户端配置；默认单节点 Compose 继续作为回滚路径，完整业务故障切换仍需 Remote GPU 验收。

- 2026-08-30：新增业务依赖拓扑契约，明确单节点微服务 Compose、Kafka/Redis 组件级集群演练和未来业务集群的证据边界；`check-compose.sh` 在缺少契约或出现业务集群误宣称时 fail closed，默认部署路径保持不变。

- 2026-08-30：新增 `remote-dev.sh recovery` 远程节点恢复入口，自动绑定 `dipole-c1` 候选端口、`/tmp` 报告目录和 Dockerized k6 fallback，减少手工 SSH 参数漂移；入口继续经过活动用户门禁和候选镜像 provenance 校验。

- 2026-08-30：远程 Dockerized k6 wrapper 额外挂载 `/tmp`，允许 benchmark/recovery 报告使用宿主临时目录并被容器正常写回；仓库挂载、UID/GID 映射和隔离网络保持不变。

- 2026-08-30：完成 C1 候选业务拓扑节点恢复演练：在 `d8b0e4a9`、隔离 `dipole-c1` 三节点环境中停止并恢复 `dipole-node2`，不可用观测 `518ms`、恢复健康 `16093ms`；consumer group 稳定成员 `72`，恢复后 `40/40` 消息接受/持久化/投递，HTTP failure `0%`，Kafka lag `0`。证据归档于 `benchmarks/c1-node2-recovery-d8b0e4a9/`，外部 GPU 任务保持运行。

- 2026-08-30：修复 Remote GPU 活动会话批准参数未跨 SSH 透传的问题；`DIPOLE_REMOTE_ALLOW_ACTIVE=1` 现在由远端启动门禁显式接收，GPU 任务并行策略和活跃用户默认保护保持不变。

- 2026-08-30：统一 Remote GPU 并行启动行为：CPU/容器型 Dipole 构建、Smoke 和压力测试在检测到既有 GPU 任务时继续执行并记录资源快照；活跃登录用户仍默认阻断，`DIPOLE_REMOTE_ALLOW_ACTIVE=1` 仅用于明确批准的活动会话。隔离 Compose project、资源边界和自动清理策略保持不变。

- 2026-08-30：改进 Remote GPU benchmark 入口，新增受控的 `DIPOLE_BENCH_SCENARIO_FILTER`、`DIPOLE_BENCH_GROUP_MAX_DURATION`、`DIPOLE_BENCH_USER_COUNT`、`DIPOLE_BENCH_GROUP_SIZE` 和 `DIPOLE_BENCH_RUN_ID` 转发；后续可从本地统一触发 group-only、规模和可比性 workload，避免手工远端脚本参数漂移。
- 2026-08-30：修复 benchmark 报告场景标识：设置 `SCENARIO_FILTER` 时，operations 与最终报告现在使用实际过滤场景，避免 group-only workload 被错误标记为 `mixed`；新增契约测试，默认 mixed/direct/concurrent 行为保持兼容。
- 2026-08-30：完成 C1 100 成员群组 fan-out 观察：使用 `SCENARIO_FILTER=group_blast` 在 `master` 提交 `9595b0ef` 的隔离候选拓扑上完成 100/100 VU；10/10 群消息持久化、1,000 条群 Inbox 行、990/990 预期回执和 100% 投递，P50/P95/P99 为 121/222/226ms，Kafka lag 为 0。Node1 CPU 峰值约 46.42%，该结果用于规模趋势观察，热群故障回切和 C++ 灰度仍待完成。
- 2026-08-30：补齐 C1 群组基准覆盖：`group_blast` 运行窗口改为可配置，默认 `35s`，并在 Remote GPU 以提交 `67a4aa1a` 完成 50/50 VU；10/10 群消息持久化、500 条群 Inbox 行、490/490 预期回执和 100% 投递，P50/P95/P99 为 106/118/132ms，Kafka lag 从 1 收敛到 0。该结果只代表 50 成员群组基线，不代表热群容量或故障回切已完成。
- 2026-08-30：修复 Remote GPU Dockerized k6 runner 的宿主文件写入权限，使用调用者 UID/GID 映射确保基准汇总文件可落盘；在 `master` 提交 `b9281eaa` 的隔离 C1 候选拓扑上完成有效基线：450/450 消息接受、持久化和投递，端到端 P50/P95/P99 为 88/109/181ms，Kafka 峰值与结算 lag 均为 0。群组阶段在优雅停止窗口内完成 30/50，HTTP 失败率包含该覆盖限制，后续仍需独立群组容量和故障矩阵。
- 2026-08-30：改进 Remote GPU C1 基准工作流：`run_bench.sh` 支持显式 `K6_BIN`，远端缺少宿主机 k6 时自动使用固定 `grafana/k6:0.57.0` 容器和 host network；修复 SSH 可选空参数导致的镜像参数左移，并在 `dipole-c1` project 下自动映射 `18081/18082` 候选端口。新增远程入口、Shell 和 Python 契约测试，默认 Go authority 与候选拓扑隔离保持不变。
- 2026-08-30：完成 Eino `v0.10.0-alpha.26` 隔离 spike：核对 ADK Session/Checkpoint/Resume、background task lease/CAS 和 notification outbox，并形成与现有 Temporal + MySQL Task/Run authority 的映射；默认 `v0.9.17` 依赖保持不变。
- 2026-08-30：为大文件 Multipart session 增加浏览器 Web Locks 独占租约，同一文件在多个标签页中会串行执行，避免重复接管；无 Web Locks 的浏览器保持兼容回退，并新增串行/回退测试，上传测试 `15/15`、Frontend typecheck 通过。
- 2026-08-30：补充预签名服务不可用回归测试：刷新签名失败时保留原错误、只发起一次失败分片 PUT，不误报上传成功；Multipart 上传测试 `13/13`、Frontend typecheck 通过。
- 2026-08-30：新增 `multipart-restart-smoke` 远程故障验证：首个分片上传后重启隔离 MinIO 容器，再继续上传并完成对象，使用独立持久卷并在退出时清理；新增 Go smoke tool、远程入口和操作说明，不申请 GPU。
- 2026-08-30：将预签名过期恢复提炼为可复用的 `uploadPresignedPartWithRefresh` 原语，并新增 403 -> 刷新签名 -> 重试测试；上传测试 `12/12`、Frontend typecheck 通过，页面层只负责签名 API 与 URL 映射。
- 2026-08-30：Multipart 预签名分片上传新增 `401/403` 过期恢复：保留 HTTP 状态、按分片重新获取签名并交给既有指数退避重试；失败 session 继续保留，成功 Complete 仍清理本地状态。上传测试 `11/11`、Frontend typecheck/build 通过。
- 2026-08-30：修复大文件 Multipart 上传失败后的断点续传行为：分片或 Complete 失败时保留服务端 session 与本地文件身份，后续重试通过 status 跳过已完成分片；成功 Complete 仍清理本地 session，服务端已完成记录保持幂等。Frontend `29/114`、typecheck 和生产构建通过。
- 2026-08-30：远程开发入口新增 `multipart-smoke`，统一注入远端 Go toolchain 并固定 `GOTOOLCHAIN=local`；该 CPU/容器型动作允许与 GPU 任务并行，使用脚本自有临时 MinIO 容器和自动清理，新增入口契约测试 `7/7` 通过。
- 2026-08-30：Remote GPU 在 `master` revision `67235080` 使用已验证 Go 1.27 本地 toolchain 完成 MinIO Multipart 生命周期 smoke；乱序分片、同编号替换、按序 Complete、对象内容校验和重复 Abort 全部通过。首次尝试因 Go 自动工具链下载超时，随后固定 `GOTOOLCHAIN=local` 重试成功；测试容器已自动清理，默认 relay 路径未改变。
- 2026-08-30：A6 新增真实 Chromium Sync Timeline 恢复验收，覆盖 IndexedDB 持久化、浏览器重开、从已提交 cursor `2` 继续请求、幂等重 ACK `2` 后推进到 `4`，并确认本地恢复消息先于远端增量交付；Chromium `6` 项通过、`2` 项按条件跳过，未改变 `/sync` 默认路由或切流开关。
- 2026-08-30：C3 性能基准新增显式 `DIPOLE_REALTIME_BENCH_CONTAINER=1` 模式，使用当前 revision 的 Docker builder 产物运行 C++ benchmark，并在报告记录 runner 来源；解除宿主机 `clang-tidy` 缺失与性能测量之间的耦合，默认路径和 Go authority 保持不变。
- 2026-08-30：C3 性能基准支持 `DIPOLE_GO_BIN` 注入远端已安装的 Go toolchain，避免 `go run` 因自动下载 toolchain 超时而阻断同版本 C++/Go 对比；默认仍使用 PATH 中的 `go`。
- 2026-08-30：Remote GPU 在存在 2 个 Python GPU 任务期间完成提交 `7eb11de7` 的容器 C++ benchmark 与 Go 对比，C++ builder CTest `14/14` 通过，C++/Go ratio `0.119227`，按阈值 `1.0` fail closed；报告归档到 `benchmarks/c3-cpp-projection-benchmark-2026-08-30/`，Go 继续作为 authority。
- 2026-08-30：将 C3 Go 对照改为同一 `DeliveryEnvelope`/item projection 后，在 Remote GPU 完成 revision `8a87cc44` 的 10,000 次等价契约 workload；C++/Go ratio `0.247269`，C++ builder CTest `14/14` 通过但仍低于 `1.0` 门槛，报告归档到 `benchmarks/c3-cpp-projection-benchmark-2026-08-30-equivalent/`。
- 2026-08-30：补充 C3 5 次稳定性采样，C++ 约 `30.76k-31.58k ops/s`、Go 约 `122.12k-125.59k ops/s`，ratio 稳定约 `0.25`；确认性能阻断来自稳定差距，不是单次抖动，采样说明归档在同一 benchmark 目录。
- 2026-08-30：Remote GPU C3 长时 profiling 尝试完成 1,000,000 次 C++ 投影，吞吐 `31,598.15 ops/s`；宿主 `perf` 因缺少内核 `6.14.0-36` 对应 linux-tools 返回 `2`，未生成伪造热点数据。profiling 容器已清理，GPU 任务未受影响。

- 2026-08-30：补充开发与远程资源工作流：Remote GPU 存在其他 GPU 任务时允许启动 CPU/容器型 Dipole 构建、Smoke 和压力测试；新增独立 Compose project、资源快照、GPU 显式申请和禁止触碰他人任务的边界。GPU 忙碌本身不再作为开发启动阻断条件。

- 2026-08-30：新增隔离 MinIO Multipart 生命周期 smoke，真实验证乱序分片、同编号分片替换、按序 Complete、对象内容校验和重复 Abort；脚本使用临时容器并自动清理，不改变默认 relay 路径。

- 2026-08-30：复核 Agent Runtime 独立门禁：Vitest `125` 个测试文件通过、`665` 个测试通过，TypeScript typecheck 与生产 build 均通过；误用 Jest 的 `--runInBand` 仅记录为命令兼容性提示，项目标准入口保持 `npm test`。

- 2026-08-30：修复 `smoke-sync-write-ownership.sh` 在服务目录重排后仍指向 `internal/bootstrap` 的三个测试选择器，改为 Sync/Message 服务实际拥有的测试包，并保留“selector 无匹配即失败”保护。

- 2026-08-30：为 C3 灰度发布增加独立 `RolloutPolicy` 契约，支持按节点/用户作用域使用稳定盐值确定性选择 `go|shadow|cpp` 目标；默认百分比为 Go，配置或 subject 异常 fail closed。当前仅提供纯策略和测试，未接入 Gateway 投递副作用，性能收益与可执行回切门禁保持不变。
- 2026-08-30：修复 `RolloutPolicy` 未拒绝 `101..255` 百分比的边界问题；非法灰度比例现在在选择前 fail closed，并增加回归测试。

- 2026-08-30：校正阶段计划状态：C3 authority、自动回切、Redis/Kafka 故障注入和 C++ primary 隔离证据已完成；C1/C2/C3 主阶段继续保持进行中，剩余门禁为可复现的 C++ 性能收益和按节点/用户灰度发布。避免将已完成故障演练重复排队，也避免提前宣称 C++ 已替换 Go。

- 2026-08-30：补充 Remote GPU C1 组件故障证据：独立三 broker Kafka consumer rebalance 在 member 退出后接管全部 6 个 partition 且 lag 恢复为 `0`；独立 Redis Sentinel 在 master 停止后约 4 秒完成切换，客户端读写、Pub/Sub、Presence、热群和限流状态恢复，旧 master 重新加入为 replica。Redis 探针镜像支持 `DIPOLE_REDIS_FAILOVER_PROBE_IMAGE`，避免远端固定镜像未缓存造成阻塞；候选业务拓扑的 Kafka/Redis 自动回切仍待验证。

- 2026-08-30：完成 Remote GPU C1 单节点恢复演练：`dipole-node2` stop/start 后约 `505ms` 观察到不可用、约 `16.0s` 恢复健康，consumer group 稳定恢复为 `72` 个成员；恢复后 40/40 消息接受/持久化/投递，Kafka lag 为 `0`，PID 更换且 revision 未漂移。完整 evidence/report 已归档。

- 2026-08-30：修复 C1 节点 recovery drill 的 Compose 相对路径解析，移除旧的 `--project-directory`，使 stop/start 故障证据与候选拓扑使用同一配置挂载语义；新增契约断言，生产 Compose 不受影响。

- 2026-08-30：补充 Remote GPU C1 100 用户并发容量观察：400/400 消息接受、持久化和投递，投递率 `100%`，HTTP 失败率 `0%`，消息端到端 P50/P95/P99 为 `149/178.04/243.01ms`，Kafka lag 采样为 `0`。相比 20 用户并发延迟上升；该结果仍不代表容量上限或故障恢复能力。

- 2026-08-30：补充 Remote GPU C1 并发在线基线：20 个在线用户、80 条消息全部接受/持久化/投递，投递率 `100%`，消息端到端 P50/P95/P99 为 `91.5/103.05/104.41ms`，Kafka lag 采样为 `0`。结果已归档；容量上限、故障恢复和自动回切仍待验证。

- 2026-08-30：补充 Remote GPU C1 群广播基线：20 个成员、10 条群消息，`190/190` 预期回执、投递率 `100%`，消息端到端 P50/P95/P99 为 `83/89.54/107ms`，Kafka lag 采样为 `0`。结果已与 direct message 基线一并归档；容量上限和故障恢复仍待验证。

- 2026-08-30：归档 Remote GPU C1 低负载 direct message 基线：提交 `160d2cc6`、20 用户、50 条消息全部接受/持久化/投递，HTTP 失败率 `0%`，消息端到端 P50/P95/P99 为 `49/162.10/165ms`，Kafka lag 峰值与稳定采样均为 `0`。该结果仅覆盖候选链路可运行性，不代表容量上限。

- 2026-08-30：C1 候选拓扑默认只启动消息基准所需的核心服务；Kafdrop/Nginx 改为通过 `C1_ENABLE_OPTIONAL_SERVICES=1` 显式启用，避免可选镜像下载阻塞核心健康检查和负载测试。

- 2026-08-30：修复 C1 候选拓扑的 Compose 相对挂载路径和证书前置：移除会把 `../../configs` 解析到仓库外的 `--project-directory`，并由候选脚本按需生成短期开发自签名 Nginx 证书；失败拓扑已清理，未改变生产 Compose。

- 2026-08-30：修复远程 C1 候选构建 heredoc 的变量展开问题；候选 revision、创建时间和镜像标签现在均在远端脚本中计算，避免本地未定义变量导致构建提前退出。新增契约断言并通过 Shell、脚本、架构文档和 diff 门禁。

- 2026-08-30：远程构建入口新增默认关闭的 `DIPOLE_REMOTE_BUILD_CANDIDATE=1`，按当前提交额外生成带 OCI revision、创建时间和 `dirty=false` provenance 的 `dipole-server:c1-<commit>` 候选镜像；默认微服务构建路径与回滚行为保持不变，为完整 C1 三节点基线补齐可验证前置。

- 2026-08-30：归档 Remote GPU 微服务入口只读基线：提交 `f227401a` 的隔离微服务栈执行 1000 次 Gateway `/health` 请求、并发 16，成功率 `100%`，P50/P95/P99 为 `0.521/0.791/1.960ms`；退出后无容器或卷残留。该证据只覆盖入口稳定性，不代表消息吞吐或 WebSocket 容量，完整 C1 k6 基线仍待候选三节点拓扑和远端工具链。

- 2026-08-30：远端基础镜像经一次性流式导入后，隔离 `smoke-lite` 在提交 `f227401a` 上完整通过；MySQL、Redis、Kafka、MinIO、Core、Message、Sync、Gateway 均 healthy，Gateway readiness、认证代理和可选服务隔离检查通过。脚本退出后自动清理，无本项目容器或卷残留；现有 GPU 任务未被操作，完整基线压测仍单独排队。

- 2026-08-30：在用户明确授权 GPU 任务并行的前提下，Remote GPU 对 `c09334f0` 完成提交绑定 Go 编译和 8 个微服务镜像构建；Go 镜像构建上下文实测约 `688MB`，未启动或影响现有 GPU 任务。隔离 `smoke-lite` 已完成 preflight/证书阶段，但首次拉取 MySQL 基础镜像耗时过长后安全中止；Compose trap 清理后无本项目容器或卷残留，完整 Smoke 与负载测试继续待镜像缓存或受控代理条件。

- 2026-08-30：修复 Go 微服务镜像最小上下文切换中的变量引用错误，并以脚本契约测试锁定 `root_dir/dist`；首次远端实测的 fail-closed 结果已记录，未启动容器。

- 2026-08-30：将 Go 微服务镜像构建上下文收窄为生成的 `dist/` 目录，Dockerfile 仅复制指定服务二进制；新增脚本契约测试，减少 Remote GPU 构建时重复发送根目录数据。

- 2026-08-30：修复 Remote GPU 构建入口，`scripts/remote-dev.sh build` 会先编译提交绑定的 Go 服务二进制再构建逐服务镜像；同步修正文档中的 `planning-with-files` 模式，并记录隔离 Smoke 因 registry mirror 对缺失自定义镜像返回 `403` 的可回滚证据。

- 2026-08-30：Multipart Prometheus 规则新增 reconciliation 漂移、扫描不完整和指标过期告警，并补充 `promtool` 触发时序测试；修复动作仍保持人工确认和原有删除保护。

- 2026-08-30：Remote GPU 在用户态 Go 1.27.0 下对 `master` 提交 `9c0f2702` 完成远端 canonical 门禁；Go 白名单测试、服务布局和架构文档检查全部通过，未启动 Compose 或创建容器，构建/Smoke/Benchmark 继续遵守活动用户保护。

- 2026-08-30：Multipart reconciliation 工具新增可选 `--metrics-output`，以原子替换方式输出固定名称、低基数 Prometheus textfile gauges，覆盖扫描状态、漂移数量和最后运行时间；默认不创建指标文件，JSON、退出码和删除保护语义保持不变。

- 2026-08-30：新增 [开发工作流与提速规则](docs/operations/DEVELOPMENT-WORKFLOW.md)，将活动计划、历史记录、快速门禁、共享环境演练、Epic 同步和 worktree 生命周期分层；确认当前瓶颈来自计划上下文膨胀与验证串行化，后续按 30 分钟切片和一次性 Epic 同步执行。

- 2026-08-30：A6 Web Sync Observation 工具新增 5 分钟未来时间偏差门禁，`start/status/finalize` 均拒绝未来时间查询，并补充可注入时钟测试和操作手册说明；该改动只强化证据完整性，未改变客户端默认同步路径或任何切流开关。

- 2026-08-30：本机 `planning-with-files` 更新至上游稳定版 `v3.11.2`；移出重复的 `.agents` skill 安装，仅保留 `.codex` canonical 来源，并在 Codex 适配器层加入同 session/项目根短窗口去重。真实并行 Bash 验证确认三次触发只注入一份计划上下文，旧版本与 hook 配置已保留在日期备份目录。

- 2026-08-30：新增 `scripts/remote-dev.test.mjs` 远程开发入口契约测试，覆盖提交绑定同步、Node 锁文件保护、`webapp` 构建产物清理、Node 版本门禁及活动主机下的构建/Smoke/Benchmark 保护；`4/4` 通过。

- 2026-08-30：在 Remote GPU 对 `master` revision `b96403b0` 重新执行 Go canonical 门禁；全部白名单 Go 包测试、服务布局和架构文档检查通过，未启动 Compose，远端源码工作树保持干净。

- 2026-08-30：修复 Remote GPU `node-test` 的构建产物污染：测试前检查 `internal/services/core/server/webapp` 是否干净，退出时仅恢复该目录的 tracked diff 并清理本次生成的 untracked 资产；Agent Runtime 与 Frontend 验证可持续运行且不留下远端源码变更。

- 2026-08-30：同步 `epic/01-microservices`、`epic/02-storage-architecture`、`epic/03-agent-runtime`、`epic/04-cpp-realtime` 和 `epic/05-frontend-experience` 到最新 `master` 基线；核心三阶段分支快进同步，C++/Frontend 扩展分支保留独有提交后完成合并并推送。

- 2026-08-30：Remote GPU 最新 preflight 通过（224 vCPU、约 161 GiB 可用内存、约 1 TiB 可用磁盘），但构建门禁检测到 `users=23`、`gpu_processes=5` 并安全拒绝启动；未创建镜像或容器，继续等待维护窗口。

- 2026-08-30：Remote GPU 已在提交 `37d5f1b3` 上完成最新 Go canonical 复核；Go 测试、服务布局和架构文档门禁全部通过，Agent/Frontend Node 验证产生的锁文件临时差异已清理，远端工作目录恢复干净。

- 2026-08-30：Remote GPU 用户态 Node `22.12.0` 完成 Agent Runtime 与 Frontend 验证；Agent 通过 125 个测试文件/665 个测试，typecheck/build 通过；Frontend 通过 29 个测试文件/114 个测试，typecheck/Vite 构建通过。集成测试按环境条件跳过，未启动 Docker。

- 2026-08-30：为 Codex `planning-with-files` 的 `PostToolUse` 增加同会话/工作目录短窗口去重；并行 Bash 完成事件只保留一条提醒，超过窗口仍恢复提示，避免开发工作流噪声累积。

- 2026-08-30：Remote GPU 已同步 `master` 提交 `3dfaf53d`，使用用户态 Go 1.27.0 与 `GOPROXY=off` 完成最新离线 canonical 测试；Go test、服务布局和架构文档门禁全部通过，未启动容器。

- 2026-08-30：新增 `scripts/drain-local-dipole.sh`，支持迁移成功后的本机降载预览与显式执行；仅停止 `dipole*` 容器，保留卷/镜像并避开无关项目，补充脚本契约测试和恢复说明。

- 2026-08-30：Remote GPU 在断开临时 module proxy 后使用本地缓存、用户态 Go 1.27.0 完成完全离线 `scripts/remote-dev.sh test`；Go test、Compose、服务布局和架构文档门禁全部通过，退出码为 `0`。为降低本机负载，Dipole 本地 Compose、隔离 smoke 和观测拓扑已停止，未删除卷或镜像。

- 2026-08-30：Remote GPU 在用户态 Go 1.27.0、受控只读 module proxy 和提交 `a92b9a8c` 上完成远端 canonical 验证；Go test、Compose、服务布局与架构文档门禁全部通过，退出码为 `0`。本机 Dipole Compose 拓扑已停止，远端未启动容器。

- 2026-08-30：远端测试入口增加可选 `DIPOLE_REMOTE_GOPROXY`，允许通过短期受控缓存代理补齐远端缺失 Go modules；代理地址只存在于运行环境，不写入仓库或持久配置。

- 2026-08-30：为 Remote GPU 增加 `DIPOLE_REMOTE_GO_ROOT` 用户态 Go 工具链入口；已同步 Go 1.27.0 到 `/home/admin1/.local/go-1.27.0`，不修改系统 Go，后续可在远端执行完整 canonical 测试。

- 2026-08-30：远端开发测试流程增加 Go 工具链预检并固定 `GOTOOLCHAIN=local`，在 Remote GPU 仅有 Go 1.22.2、项目要求 Go 1.26.0 时快速失败，避免因隐式下载工具链造成网络超时；当前未启动容器，待远端维护窗口补齐 Go 1.26+ 后继续执行 canonical 测试。

- 2026-08-30：Multipart 策略接口加入三份静态 Swagger 文档；当前 Go 1.27 环境下 `swag` 生成器仍受历史注释/标准库解析兼容影响，运行时路由与静态文档已保持一致。

- 2026-08-30：Multipart 策略接入运行时：Core 新增认证的 `/api/v1/files/uploads/policy`，前端按服务端版本策略执行阈值、并发、重试和预签名候选模式；策略异常或接口不可用时回退到 `v1/relay`，默认生产流量保持不变。

- 2026-08-30：新增 `contracts/multipart-upload/v1` 版本化上传策略、默认 `relay` 策略和 SHA-256 release manifest，并增加 `scripts/check-multipart-policy.mjs` 及 Node 测试；大文件上限、分片大小、并发、重试和预签名 URL TTL 统一进入可审计契约，生产仍保留旧路径回切。

- 2026-08-30：Multipart Web 上传增加可见的暂停/继续控制；暂停只停止新分片调度并保留 Redis/MinIO 会话，继续时复用原 `upload_id` 和已确认分片，前端上传专项测试、类型检查和生产构建通过。

- 2026-08-30：重新执行 `scripts/smoke-microservices.sh`，隔离验证 Core、Gateway、Message、Sync、Agent 及 MySQL/Redis/Kafka/MinIO readiness、metrics、mTLS、远程 WS ownership 和 Agent 幂等；临时拓扑自动清理，KafkaJS 分区器 warning 未影响验收。
- 2026-08-30：重新执行 `scripts/smoke-cassandra-read-routing.sh`，隔离验证 migration v50、Cassandra Seq 页面读取，以及 payload 损坏和缺失行按同一 cursor 回退 MySQL；临时资源自动清理，生产 Cassandra 主读保持关闭。
- 2026-08-30：在最新 `master` 重新执行 `scripts/check-go.sh`，全部 Go 包 test/vet 通过；直接 `go test ./...` 仍受本地忽略的旧 `agent-runtime` 构建目录影响，规范验收继续使用包白名单入口。
- 2026-08-30：在最新 `master` 重新执行 `scripts/check-go.sh`，全部 Go 包 test/vet 通过；直接 `go test ./...` 仍受本地忽略的旧 `agent-runtime` 构建目录影响，规范验收继续使用包白名单入口。
- 2026-08-30：补充 Multipart P95 延迟告警的正向 promtool 触发测试，确保 30 秒阈值和 `operation` 标签在规则变更后仍可验证。
- 2026-08-30：增加 Multipart Prometheus 告警规则与 promtool 测试，覆盖操作错误、整文件 checksum mismatch 和高延迟；修正 Core 指标将 checksum mismatch 以专用 outcome 暴露，规则挂载保持可回滚。
- 2026-08-30：Multipart reconciliation 增加 `--reconcile-fail-on-drift` 告警门禁，显式开启时发现 MinIO/Redis 跨存储漂移返回退出码 `3`；默认仍只读且不修改数据。
- 2026-08-30：Multipart cleanup 增加只读 `--reconcile` 模式，对照 MinIO 未完成 upload 与 Redis session metadata，识别跨存储漂移并以测试保证默认不修改任何数据。
- 2026-08-30：Multipart 增加短期完成收据与重复请求保护：成功响应丢失后重复 Complete 返回同一文件记录，重复 Abort 不重复调用对象存储，已完成会话拒绝 Abort；补充 Core 服务测试与 A7/AD-055 台账记录。
- 2026-08-30：校准 AD-055 与 A7 的大文件上传状态：记录整文件 SHA-256 绑定与可选强制校验已完成，明确 Redis 孤儿扫描已有基础能力，并将剩余工作收敛为 Multipart reconciliation、生命周期告警、Complete/Abort 幂等、暂停/继续和真实故障矩阵。
- 2026-08-30：同步 `PLATFORM-EVOLUTION-PLAN.md` 的实际质量基线和 F4 状态：Agent Runtime 更新为 `125` 个测试文件/`665` 个通过、`27` 个按条件跳过，Frontend 更新为 `28` 个文件/`104` 个测试，并明确 token 映射、核心流程和跨浏览器功能回归已完成；全页面截图视觉基线、真实 Pencil CLI 增量编辑及未覆盖平台仍保留待办。
- 2026-08-30：完成主线综合门禁复核：架构文档、服务布局、SQLC 和脚本包白名单下的 Go 全量 test/vet 均通过；同时确认根目录 Markdown 与 `docs/` 分类保持收敛，未新增散落文档。
- 2026-08-30：修正 `docs/architecture/DEVELOPMENT-ROADMAP.md` 中对已退役 `internal/service` 的过时表述，改为描述共享兼容适配器与 `internal/services/<service>/` 的持续收敛，避免路线图误导新服务开发。
- 2026-08-30：修复 `smoke-sync-cassandra-hydration.sh` 对已退役 `internal/service` 的路径引用，改用 `internal/services/message/domain`；修复后真实隔离 hydration smoke 通过 shadow comparison、重复响应恢复、Legacy ID 恢复和 Metadata backfill。
- 2026-08-30：在当前 `master` revision `801e69ce` 上复跑完整微服务 Compose smoke；逐服务 health/readiness、metrics、Core 代理 401、Core WS 边界、mTLS 启动、远程 WS ownership 及 Agent EventLedger/Task/Run 幂等均通过，隔离拓扑自动清理，生产 Kafka ownership 与回滚切换继续关闭。
- 2026-08-30：在当前 `master` revision `69055e87` 上复跑 Cassandra primary Compose smoke；Cassandra schema init、Sync primary profile、依赖 readiness 和 Sync `readyz` 全部通过，临时资源已清理。本次仅证明启动与配置门禁，真实 Inbox hydration、共享环境观测、责任人批准和可执行回切继续关闭。
- 2026-08-30：复核 Agent Runtime Context Compiler 校准命令：fixture evidence 覆盖中文、英文、代码、Emoji 和 Tool Schema 五类样本，`eligible=true`、无低估，report SHA-256 为 `d5bce2090f8d4b4c6af786d75dee656fd9dd33554ecaf8026e5880abe4863562`；同时全量回归 `665` 项通过、`27` 项按条件跳过，真实候选模型校准仍保持生产前置门禁。
- 2026-08-30：在当前 `master` revision `d2507377` 上重建微服务候选镜像并复跑 Agent Timeline repair Compose smoke；migration `v50`、UTC、专用最小权限、worker readiness、pending intent 恢复和 event UUID 幂等均通过，隔离栈已清理，共享环境 operator 灰度与默认生产开关继续关闭。
- 2026-08-30：补齐前端 `npm run typecheck` 标准脚本，并由 Vite 工具链契约测试锁定 `vue-tsc --noEmit` 命令；后续类型检查可使用统一入口执行。
- 2026-08-30：收敛 Chat 初始化阶段的认证恢复异常：HTTP 401 仍由统一拦截器执行会话清理和跳转，Vue 生命周期不再产生未处理 Promise rejection；共享设备认证 E2E 在 Chromium、Firefox、WebKit 共 `6/6` 通过。
- 2026-08-30：完成前端全浏览器 Playwright 回归：Chromium、Firefox、WebKit 共 `90` 项配置测试中 `64` 项通过、`26` 项按平台/条件跳过；Agent 表单、Task Timeline、IndexedDB 恢复、Search 视觉状态和设备会话均通过适用场景。
- 2026-08-30：修正 Agent Memory Chromium E2E 的非精确标题断言，避免 `长期记忆` 与 `正在读取长期记忆` 触发 Playwright strict mode；目标场景 `3/3`、完整 Chromium E2E `28` 项通过（`2` 项按条件跳过）。
- 2026-08-30：修正 Vite 生产构建输出边界，前端产物统一写入 Core-owned `internal/services/core/server/webapp/`，避免构建重新生成已退役的 `internal/server/webapp/`；工具链 `3/3`、Vitest `28` 个文件/`104` 个测试、`vue-tsc` 和生产构建均通过。
- 2026-08-30：补充 Agent Active Compose 负向门禁，缺失 release manifest 或 candidate version 时在插值阶段直接失败；默认 Shadow 配置和 Active 回滚路径保持不变。
- 2026-08-30：核验 CloudWeGo Eino 依赖升级状态：`go list -m -u` 未发现高于当前 `v0.9.17` 的可用升级；公开 `v0.10.0-alpha` 仍属于预发布路线，暂不引入 Go/Eino 回滚基线，避免与 TS Agent Runtime 接管和微服务切换同时改变。
- 2026-08-30：复核 Agent Active 晋级前置门禁：TypeScript Runtime Vitest `125` 个文件、`665` 个测试通过，`typecheck`、生产构建和 Compose active overlay 契约均通过；默认 Shadow、显式 `user_gray` manifest 和 candidate 绑定保持 fail-closed，共享 Temporal/Kafka/Active authority 联调仍待完成。

- 2026-08-30：修复 Inbox projector 隔离 smoke 的失败拓扑保留逻辑，并补齐 Message Service 的 `DIPOLE_SYNC_PROJECTOR_ENABLED` 前置配置；在 projector ownership 模式下完成真实消息流程验证，receipt 归档于 `benchmarks/ad048-projector-message-flow-2026-08-30/receipt.json`，回滚仍为移除 overlay 并恢复 `atomic`。
- 2026-08-30：完成逐服务候选镜像的真实消息流程验证，覆盖注册登录、好友关系、WebSocket 发送、Message/Outbox/Inbox 幂等和 Seq 历史/增量读取；receipt 归档于 `benchmarks/ad048-message-flow-2026-08-30/receipt.json`，生产切换保持关闭。
- 2026-08-30：收紧 Memory candidate 解析边界，正文和 compact 摘要均拒绝凭据模式；新增绕过 ObservationWorker 的 fail-closed 测试，保持自动 Memory 写入关闭。
- 2026-08-30：收紧 Agent Task Timeline 的 TS RPC 客户端响应校验，拒绝跨任务、空事件 ID、重复或倒序 `event_seq`，新增 fail-closed 契约测试；不改变服务端协议和默认关闭状态。
- 2026-08-30：完成逐服务镜像候选拓扑验证：基于干净 `master` revision 重建 Go 服务镜像，并在 Search profile 隔离 Compose 中验证 Core、Message、Sync、Gateway、Search 和 Search Indexer 的 health/readiness；证据归档于 `benchmarks/ad048-independent-images-2026-08-30/receipt.json`，生产切换仍保持关闭。
- 2026-08-30：收紧 embedded 聚合的服务边界：仅允许 Core 的显式 `embedded_compat.go` 作为本地兼容/回滚桥接，新增结构门禁与 Core bootstrap 回归测试，阻止独立服务重新依赖 `internal/bootstrap/embedded`。
- 2026-08-30：修正 C++ Realtime Delivery CMake 根目录探测并通过标准容器门禁；Ubuntu 24.04 容器构建、14/14 CTest 和镜像打包均通过，本机 host gate 因缺少 `grpc++` 依赖暂不能运行，C++ primary 仍保持关闭。
- 2026-08-30：修正 C++ Realtime Delivery 的 CMake 根目录探测：同时支持源码仓库和独立容器构建上下文，按 canonical delivery proto 与 fence testdata 定位 `api/`、`contracts/`，避免本地配置误解析到 `services/api`。
- 2026-08-30：Agent Runtime 对 `DIPOLE_AGENT_RUNTIME_MODE` 增加显式枚举校验，除 `shadow`/`remote` 外的值现在 fail closed，避免拼写错误静默回退到 Shadow。
- 2026-08-30：增加 Agent 前端路由安全契约测试，锁定 5 条 Agent 页面均需认证、各自受独立 feature flag 保护且关闭时回到 Chat；未改变默认关闭配置。
- 2026-08-30：同步前端设计计划与实际 Router：补充 5 条受 feature flag 保护的 Agent 页面路由，并明确 Search/Sync 属于 Chat 工作区能力；Contact、Group、File、Device 和 Settings 仍保持待完成状态。
- 2026-08-30：收紧 Web Sync Observation Session 的状态时间边界：`status` 对早于 `started_at` 的采样时间 fail closed，并增加回归测试，避免观测证据出现倒序时间。
- 2026-08-30：复核主线前端与 Web Sync 契约：Pencil 设计门禁通过（54 个 Frame、2036 个节点、36 个变量、23 个可复用组件），前端 Vitest 通过 27 个文件/102 个测试，Web Sync 观测契约通过 9 个测试；真实客户端 24 小时观测仍保持未启动。
- 2026-08-30：将微服务 Compose 的 Agent Runtime 默认模式显式固定为 `shadow`，并由 Compose 门禁阻止默认配置漂移到 `active`；运行行为保持原有安全默认值。
- 2026-08-30：明确 Agent Runtime 默认部署语义：微服务 Compose 默认启动独立容器并消费 Kafka Shadow 流，但 `active` authority、模型调用和写能力保持关闭；同步更新服务目录说明，避免“默认启用”被误解为已完成生产接管。
- 2026-08-30：诊断 Pencil Gemini 增量路径：CLI 因 selected model 缺少 API key 在执行前退出，safe-edit wrapper 清理临时输出并保持 canonical `.pen` 不变；Claude 路径的执行超时仍单独记录于 `AD-044`。
- 2026-08-30：按正确的 `--prompt` 与 `--prompt-file` 参数重试 Agent Task Timeline Pencil 增量编辑；CLI 进入 Agent 会话后在 90 秒窗口内未完成，safe-edit wrapper 清理临时输出并保持 canonical `.pen` 不变，继续记录 `AD-044`。
- 2026-08-30：明确忽略根级遗留 `agent-runtime/` 构建产物，保留 `services/agent-runtime/` 作为唯一 TypeScript Agent 源码目录；Go 项目继续使用显式包白名单门禁，源码布局不受本地构建输出影响。
- 2026-08-30：在兼容服务测试根退休后的最新 `master` 上完成规范 Go 门禁复核；`scripts/check-go.sh` 中的全量 `go test` 与 `go vet` 均通过，裸 `go test ./...` 仅受本地忽略构建目录影响，不作为项目门禁。
- 2026-08-30：完成 `internal/compat/service` 兼容测试根退休：跨版本 domain-event 契约测试迁入 `internal/platform/events/contract` 外部测试包，兼容目录仅保留说明，结构门禁阻止旧路径回流。
- 2026-08-30：完成 `internal/app` 聚合测试壳退休：迁移 11 个 Agent application 边界测试至 `internal/services/agent/application` 外部测试包，删除空聚合目录，并增加结构门禁防止其回流。
- 2026-08-30：审计确认 Gateway 旧 `NewServer` 仅被测试使用，已迁移测试到显式依赖注入入口并删除隐式 Core Auth 兼容包装；结构门禁锁定 Gateway 只接受 Composition Root 提供的 `TokenResolver`。
- 2026-08-30：为 `gateway.NewServerWithDependencies` 增加 `TokenResolver` 必填校验和回归测试；独立 Gateway 组合缺少 verifier 时启动前失败，旧 `NewServer` 继续提供兼容注入。
- 2026-08-30：Gateway Server 新增显式 `TokenResolver` 注入构造函数，独立 bootstrap 负责提供 Core verifier；旧 `NewServer` 保留兼容包装，认证行为和回滚路径保持兼容。
- 2026-08-30：将 WebSocket Authenticator 的 `ResolveSession` 和 TokenSession 依赖提取为 `internal/application.TokenSessionResolver`；WS transport 不再绑定 Core Auth 具体类型，新增结构门禁与跨包回归测试。
- 2026-08-30：将 Agent token session contract 提取到 `internal/application`，middleware 改为依赖最小 token resolver 接口；Core Auth 保留具体 JWT 实现，Gateway 后续可注入独立 verifier，相关测试和结构门禁通过。
- 2026-08-30：将 Agent MCP 默认 resource 与配置 resource 解析下沉到 `internal/application`；Core Auth 保留兼容入口并继续负责 token issuer/verifier，Gateway bootstrap、proxy 和 middleware 统一使用跨服务 contract，相关回流门禁与测试通过。
- 2026-08-30：将 Gateway Agent MCP 代理使用的 resource identifier、只读 scope 和安全 URL 校验下沉到 `internal/application`；Core 继续持有 token 签发/解析实现，Gateway 代理仅依赖跨服务认证 contract，并新增回流门禁与安全校验测试。
- 2026-08-30：将 Gateway Kafka 消费所需的群组、会话强制退出、联系人删除和会话已读事件 payload/decoder 下沉到 `internal/application`；Gateway 不再编译期依赖 Core domain，新增服务布局门禁防止该依赖回流，事件 JSON 与投递语义保持兼容。
- 2026-08-30：调用审计确认 embedded `NewMessageProcessRepositories` 包装无独立语义，已删除并让 embedded aggregate 直接调用 Message-owned SQLC constructor；inbox 写入开关和回滚配置保持显式传递。
- 2026-08-30：审计确认 embedded `Repositories.Search` 无生产或测试调用者，删除 embedded Search SQLC 构造和冗余字段；Search Service 继续由 Elasticsearch-owned runtime 独立装配。
- 2026-08-30：embedded repository composition 将 Core 的 User、Group、Contact、File、Conversation、Admin 仓储统一收回 `CoreProcessRepositories`，移除聚合根的 Core 扁平字段；Core HTTP 适配、assistant 初始化和 embedded 回滚语义保持兼容。
- 2026-08-30：embedded repository composition 将 Message、Outbox、Sync 仓储访问统一收回 `MessageProcessRepositories` 与 `SyncProcessRepositories`，移除聚合根的 Message/Sync 扁平字段；Outbox 启动、Inbox 组合和回滚语义保持兼容。
- 2026-08-30：embedded repository composition 将 Agent policy、task、memory、approval、artifact、tool audit 等仓储统一收回 `AgentProcessRepositories`，移除聚合根的 Agent 扁平字段；Agent 初始化、SQLC 实现和 embedded 回滚语义保持兼容。
- 2026-08-30：Core server 与 standalone bootstrap 已改用 Core-owned repository、messaging 和 application 端口；embedded 聚合通过边界适配继续提供本地回滚组合，Core 服务自有代码不再直接依赖 `internal/bootstrap/embedded`。
- 2026-08-30：Core Auth 的输入、结果、错误和 MCP grant 调用已统一迁移到 Core-owned Auth domain，删除无调用者的 `internal/compat/service/auth_compat.go`；认证 HTTP contract 保持兼容。
- 2026-08-30：Core Conversation 的视图、已读回执、错误和事件契约调用已统一迁移到 Core-owned Conversation domain，删除无调用者的 `internal/compat/service/conversation_compat.go`；Conversation HTTP/Kafka contract 保持兼容。
- 2026-08-30：Core Contact 的输入、响应、错误和事件契约调用已统一迁移到 Core-owned Contact domain，删除无调用者的 `internal/compat/service/contact_compat.go`；联系人 HTTP/Kafka contract 保持兼容。
- 2026-08-30：Core User 的输入、响应、错误和 User HTTP 调用已统一迁移到 Core-owned User domain，删除无调用者的 `internal/compat/service/user_compat.go`；用户管理和头像 HTTP contract 保持兼容。
- 2026-08-30：Core Session 的设备会话、Session Kick 事件和错误契约调用已统一迁移到 Core-owned Session domain，删除无调用者的 `internal/compat/service/session_compat.go`；设备会话 HTTP/Kafka contract 保持兼容。
- 2026-08-30：Core Admin 的 Overview、错误契约和 HTTP/DTO 调用已统一迁移到 Core-owned Admin domain，删除无调用者的 `internal/compat/service/admin_compat.go`；User/Auth 等仍有实际调用者的兼容入口继续保留。
- 2026-08-30：Core Auth 的 TokenService、TokenSession 与 MCP 资源校验调用已统一迁移到 Core-owned Auth domain，删除无调用者的 `internal/compat/service/token_compat.go`；Auth grant contract 兼容入口继续保留。
- 2026-08-30：执行 `scripts/smoke-kafka-rebalance.sh`，隔离验证双 consumer 加入、成员退出后的六分区接管和 lag 归零；临时 Kafka 集群自动清理，生产 consumer ownership、offset 和切换配置保持不变。
- 2026-08-30：完成 TypeScript Agent Runtime 独立交付回归：Vitest 通过 125 个测试文件（662 个测试），`typecheck` 与生产构建均通过；真实共享 Kafka/Temporal/MCP 联调和 active authority 仍按发布门禁保持关闭。
- 2026-08-30：执行 `scripts/smoke-search-service.sh`，隔离验证 Elasticsearch 9.5.2 查询路径、Core 派生 scope 和 Search Service 内部 RPC contract；临时资源自动清理，生产 Search Alias 与索引切换保持关闭。
- 2026-08-30：执行 `scripts/smoke-cassandra-read-routing.sh`，验证 Timeline 页面使用 Cassandra 读取，并在 payload 损坏或记录缺失时按同一 cursor 安全回退 MySQL；隔离资源自动清理，生产 Cassandra 主读开关保持关闭。
- 2026-08-30：执行 `scripts/smoke-sync-cassandra-primary-compose.sh`，在隔离 Compose 中验证 Cassandra schema init、MySQL migration、Sync Cassandra primary 配置和 `/readyz`；临时资源自动清理，生产 Cassandra 主读、共享环境观测与回切开关保持关闭。
- 2026-08-30：删除仅被 legacy 测试使用的 shared `newInternalRPCServer` 与 `dialInternalRPC` 转发层，测试和 Core embedded 组合直接使用 `internal/platform/rpc`；RPC transport、认证、TLS 和回滚语义保持兼容。
- 2026-08-30：将 Core Agent RPC caller-to-method 权限策略从 shared `internal/bootstrap` 下沉到 `internal/services/core/rpcpolicy`；embedded Core server 与 MCP drill fixture 复用 Core-owned policy，保留 Agent/Search/Sync 的方法白名单、mTLS caller 校验和拒绝语义。
- 2026-08-30：在最新 `master` 上再次执行 `scripts/smoke-microservices.sh`，隔离验证 Core、Message、Sync、Gateway、Agent 及 MySQL、Redis、Kafka、MinIO 的 readiness、metrics、Core proxy、mTLS、远程 WS ownership 和 Agent EventLedger/Task/Run 幂等；临时拓扑自动清理，生产流量与 ownership 配置保持不变。
- 2026-08-30：修正架构债务台账重复编号：将“服务入口已拆分但共享实现区仍缺少服务级物理边界”统一编号为 `AD-054`，保留已完成的运维目录整理为 `AD-050`，不改变债务状态或运行行为。
- 2026-08-30：删除经全仓调用审计确认仅被测试使用的 shared `NewCoreRPCServerWithAgent` facade；Agent RPC contract 测试直接构造 adapter 并验证 Core server 组合，运行时使用的 control/projection/artifact 构造路径保持不变。
- 2026-08-30：删除经全仓调用审计确认无调用者的 shared `RegisterCoreProjectionKafkaHandlers` facade；Core standalone runtime 继续直接使用 Core-owned projection 注册器，Conversation projection、Kafka ownership 和回滚路径不变。
- 2026-08-30：删除经全仓调用审计确认无调用者的 shared Kafka 注册 facade `RegisterKafkaHandlersWithRepositories` 与 `RegisterCoreKafkaHandlersWithRepositories`；embedded runtime 继续使用私有装配，Core projection 保持由 Core-owned 入口注册，Kafka topic、消费语义和回滚路径不变。
- 2026-08-30：在最新 `master` 上重新执行 `scripts/smoke-microservices.sh`，真实验证 Core、Message、Sync、Gateway、Agent 及 MySQL、Redis、Kafka、MinIO 冷启动 readiness、metrics、Core proxy、mTLS、远程 WS ownership 和 Agent EventLedger/Task/Run 幂等；隔离拓扑自动清理，生产流量与 ownership 配置保持不变。
- 2026-08-30：将 Core RPC 测试 helper 切换到 Core-owned bootstrap，删除无生产调用者的 `internal/bootstrap.NewCoreRPCServer` 公开 facade；Core capability 的认证、mTLS 和协议行为保持兼容。
- 2026-08-30：将 Delivery Observation RPC 的测试调用切换到 Gateway-owned bootstrap，删除无生产调用者的 `internal/bootstrap` facade；Realtime 服务身份、mTLS transport 和 backpressure contract 保持兼容。
- 2026-08-30：收敛 embedded runtime 的 metrics 入口，删除无生产调用者的 `internal/bootstrap` 转发层，测试迁入 `internal/platform/runtime` 并直接验证平台 API；指标启停、地址校验和 typed-nil collector 语义保持不变。
- 2026-08-30：修复 `scripts/smoke-mysql-cluster.sh` 的隔离配置注入，使用带 YAML 后缀的一次性 Router 配置并显式禁用宿主机 cgo DNS；MySQL 8.4.8 三节点 migration v50、Router writer 故障转移、已提交数据可见和停止节点 AdminAPI rejoin smoke 全部通过，临时资源自动清理。
- Agent 增加独立 `deploy/microservices/agent-active.yml` 部署 override：显式要求 candidate 和 release manifest 文件，并以只读方式挂载；默认 Compose 仍为 shadow，移除 override 即可回滚。
- 2026-08-30：重新执行 Sync Cassandra primary Compose smoke，验证 Cassandra schema init、Core/Message/Sync 依赖 readiness、primary hydration 配置和 Sync `/readyz`；临时拓扑自动清理，生产 Cassandra 主读保持关闭。
- Agent 微服务 Compose 增加显式 Runtime mode、candidate 和 release manifest 路径契约：默认固定 `shadow` 且不挂载 manifest，active override 必须只读挂载 `user_gray` 清单；回滚恢复 shadow 配置即可。
- 2026-08-30：重新执行 Kafka rebalance 隔离 smoke，验证双 consumer 成员、成员退出后的六分区接管和 lag 归零；临时集群自动清理，生产 offset、retry/DLQ 和 consumer group 配置保持不变。
- 2026-08-30：重新执行 Kafka observability 隔离 smoke，验证 Prometheus 规则、consumer lag、retry/DLQ、ISR 缺口及 broker 恢复；临时三节点集群自动清理，生产 Kafka ownership 和 topic 配置保持不变。
- 2026-08-30：重新执行 Redis Sentinel 三节点故障转移 smoke，真实验证客户端重连、Pub/Sub、Presence、Hot Group 和限流语义恢复，以及原主节点重新加入为副本；隔离栈自动清理，生产 Redis 配置保持不变。
- 2026-08-30：重新执行 Elasticsearch Search Service 隔离 smoke，真实验证 Elasticsearch 9.5.2 查询路径、Core 派生 scope 和 Internal RPC 契约；临时存储栈自动清理，生产 Search Alias 与索引切换保持不变。
- 2026-08-30：重新执行 Cassandra read-routing 隔离 smoke，真实验证 migration v50、Cassandra Seq 页面主读，以及 payload 损坏/缺行按同一 cursor 回退 MySQL；临时 Compose 资源自动清理，生产主读开关保持关闭。
- Agent Runtime active 启动增加 release manifest 绑定：必须提供 manifest 文件、candidate 必须一致且阶段必须为 `user_gray`；默认 shadow 路径不变，缺失/读取失败/阶段或版本漂移均 fail closed。
- Go/Eino 兼容 Agent 基线已从 `internal/modules/ai/` 收敛到 `internal/services/agent/legacy/`；bootstrap import 与相关文档同步更新，保留 TS Runtime 接管前的回滚路径。
- 服务布局门禁已固定 Agent legacy 目录归属，并阻止 `internal/modules/ai/` 回流。

本文档记录 Dipole 的重要功能、行为变化、兼容性说明和修复。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。项目引入正式版本后，版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## 维护约定

- 日常开发统一追加到 `Unreleased`，不要直接创建临时版本章节。
- 条目描述用户或系统可观察到的变化，并标明影响范围；纯格式调整和无行为变化的重构通常无需记录。
- 分类按需使用：`新增`、`变更`、`修复`、`安全`、`弃用`、`移除`、`迁移说明`、`验证`、`已知问题`。
- 正式发布时，将 `Unreleased` 内容移动到 `## [X.Y.Z] - YYYY-MM-DD`，随后保留一个空的 `Unreleased` 章节继续滚动更新。
- 数据结构、接口兼容性或部署步骤发生变化时，必须补充 `迁移说明`；未解决但会影响开发或发布的问题写入 `已知问题`。
- 可在条目末尾附关联 Issue、PR 或提交，例如 `(#123)` 或 ``(`abc1234`)``。

## [Unreleased]

- 新增默认关闭的 `agent-interactive-shadow` Compose overlay：仅启用经认证的 Agent Task 创建、查询、取消、输入与审批代理，固定 `shadow + read_shadow` 执行路径，并关闭 MCP、外部 MCP、Memory、检索和任何写入 authority。

- 恢复 Emerald Signal Link 品牌调色：README、IM、Agent 与紧凑入口标记统一采用深青信号场、浅色画布和橙色事件脉冲，替换分支中遗留的偏蓝青渐变；本项只影响品牌呈现。

### Added

- Added a Chromium visual baseline for the owner-scoped File Directory. The fixture locks the metadata-only disclosure boundary and per-file authorization entrypoint without contacting object storage.

- 远程开发入口在未指定 `DIPOLE_REMOTE_GO_ROOT` 时会自动选择 `/home/admin1/.local/go-*` 中的最高版本，显式路径仍优先；继续禁止隐式 Go toolchain 下载，减少 Remote GPU 默认 PATH 漂移导致的测试阻断。
- Web Sync 观察证据现在强制绑定对象存储归档收据：opaque URI、object version、ETag 和未来 retention 截止时间；缺少或过期收据时只能生成 `blocked`，不会误判为可晋级。
- 远程开发策略更新：Remote GPU 存在 GPU 任务时允许启动 Dipole 的 CPU、Docker、集成测试和压力测试任务；新增 GPU 进程保护、Compose/端口/目录隔离、资源检查和自有资源清理约束，避免把 GPU 占用误判为全局启动阻断条件。

- 新增 Remote GPU 测试入口 `scripts/remote-dev.sh test`，远端执行 Go canonical 测试、Compose、服务布局和架构文档门禁；测试阶段不启动容器，继续保留部署动作的活动用户保护。
- Remote GPU 管理员工作目录已重新同步到正式 `master` 提交 `27138a32`；当前主机仍有活动实验，未启动构建或服务。
- Remote GPU 构建入口已完成一次安全阻断验证：同步成功后检测到 23 个登录用户和 5 个 GPU 任务，在构建前退出且无容器/镜像副作用；维护窗口开启后可直接重试。
- Remote GPU 主机前置修复完成：`admin1` 已加入 Docker 组并安装 Compose v2，管理员工作流 preflight 通过；由于仍有活动实验，服务构建与启动继续由 fail-closed 保护阻止。
- Remote GPU 管理员工作目录已成功切换到正式 `master` 提交 `b9035b66`；同步链路可用，Docker 权限与 Compose 插件前置现已修复，构建与启动继续等待活动实验释放。
- Remote GPU 管理员工作流已验证 SSH、资源和代码同步；当前 `admin1` 无 Docker socket 访问且主机缺少 Compose v2 插件，preflight 会阻止构建/部署/压测，待维护窗口完成最小权限和插件修复。
- Remote GPU 工作流默认切换为 SSH alias `LAB113-OPS` 的 `admin1` 账号和 `/home/admin1/workspaces/Dipole`，与实际可用的管理员连接配置一致；凭据仍不写入仓库。
- preflight 现在区分 Docker Compose 插件缺失和 Compose 文件无效，Remote GPU 缺少 `docker compose` 时会以 `compose=plugin-missing` fail-closed；不会自动安装主机组件。
- Remote GPU 代码同步已更新至 `c3739971`；资源 preflight 通过，但因主机缺少 Docker Compose v2 插件返回 `compose=plugin-missing`，未启动容器，部署与压测继续等待维护窗口。
- 修正 Remote GPU 默认工作目录为实际账号可用的 `/home/zhangzhuyu/workspaces/Dipole`，并补充首次同步自动创建目录的说明；不改变现有主机保护和隔离 project 规则。
- 将开发部署、代码同步、镜像构建和完整压测统一收敛到 Remote GPU 工作流，新增 `scripts/remote-dev.sh`；本机不启动完整 Compose，远端动作绑定已提交 revision、隔离 Compose project，并在活动用户或 GPU 任务存在时默认 fail-closed。
- 新增低资源只读 HTTP 负载探针 `scripts/bench/http-read-load.sh`，固定 GET 并输出请求成功率与 P50/P95/P99，可用于 TencentCloud_01 的健康检查和认证边界回归；完整吞吐、WebSocket、Kafka lag 与故障证据仍使用既有 k6 基准。

- 2026-08-30：补充 TencentCloud_01 只读占用核验：现有 `nkdoing-app` 使用公网 `80`，`nkdoing-postgres` 使用本机 `5432`，宿主 MySQL 监听 `3306`；Dipole 轻量 smoke 仍需独立 Compose project、端口和卷，并等待业务影响确认后执行，当前未启动容器。

- 2026-08-30：完成两台开发主机的只读 preflight：Remote GPU（224 vCPU、可用内存约 163510 MiB、可用磁盘约 1084340 MiB）和 TencentCloud_01（2 vCPU、可用内存约 1172 MiB、可用磁盘约 34347 MiB）均通过对应 profile；未启动容器，实际部署和负载测试仍等待维护窗口。

- 2026-08-30：完成 Eino 上游复核：当前稳定依赖保持 `v0.9.17`，官方最新预发布为 `v0.10.0-alpha.26`，其 v0.10 方向增加可恢复 Session、可重放中间件状态、后台任务和 Automemory；由于仍处于 alpha 且可能存在破坏性变化，暂不升级生产回滚链路，新增隔离 spike 评估项。

- 2026-08-30：新增 `scripts/smoke-microservices-lite.sh` 与依赖闭包契约测试，以 Gateway 依赖闭包验证 TencentCloud 轻量拓扑的 Gateway/Core/Message/Sync readiness、认证代理和可选服务隔离，默认不启动 Agent、Search、Cassandra、可观测性或 C++；完整 `smoke-microservices.sh` 继续用于 Remote GPU。

- 2026-08-30：改进开发主机 preflight 的内存判定，默认读取 `MemAvailable` 而非物理总内存，避免已有实验造成内存压力时误放行；保留显式覆盖值和原有 fail-closed profile 门禁。

- 2026-08-30：新增远程开发部署与压测 runbook，明确 Remote GPU 完整拓扑、TencentCloud 轻量 smoke、本机资源限制、独立 Compose project、提交绑定镜像、证据采集、停止条件和回滚要求；记录 Remote GPU 当前存在活动会话与 GPU 任务，实际部署需等待维护窗口。

- 2026-08-30：新增开发主机 preflight `scripts/check-dev-host.sh` 与 Node 测试：Remote GPU profile 用于完整微服务和负载测试，TencentCloud profile 仅用于轻量 smoke，本机资源不足时 fail closed；检查支持资源覆盖、Docker daemon 和 Compose 配置校验，当前仅完成门禁实现，尚未执行远程部署。

- 2026-08-30：完成开发期部署环境评估：Remote GPU（224 vCPU、188 GiB 内存、约 1.1 TiB 可用磁盘、4 张 RTX 4090）作为完整微服务、存储实验、Agent Runtime 和分级负载测试环境；TencentCloud_01（2 vCPU、2 GiB 内存、50 GiB 磁盘）收敛为轻量 smoke 与低资源兼容性环境；本机暂不运行完整集群压测。远程部署门禁、资源快照、不可变镜像、隔离 Compose project、故障停止和回滚要求已写入平台演进计划，当前尚未执行远程部署。

- 2026-08-30：Multipart 初始化支持可选 `file_sha256`，会话绑定整文件摘要；开启 `storage.multipart_require_checksum` 后，Complete 会读取已完成对象校验 SHA-256，缺失或不匹配时拒绝并清理对象，前端会在初始化阶段提交 Web Crypto 摘要，默认仍保持兼容模式。
- 2026-08-30：为 Redis Multipart 清理增加幂等与截断保护回归：重复执行不会重复删除，达到 `--redis-max-keys` 时报告 `complete=false`，避免把部分扫描结果误判为全量清理。
- 2026-08-30：扩展 `dipole-multipart-cleanup` 的可选 Redis 生命周期扫描：`--redis-orphans` 以有界 SCAN 检测无 TTL 的 Multipart meta 与 meta 已过期的孤儿 parts，默认 dry-run，只有 `--execute --confirm` 才删除孤儿 parts；保留原 MinIO 报告字段，完整过期 upload reconciliation 和告警仍待 A7/AD-055。
- 2026-08-30：Core File Service 接入低基数 Multipart 指标 `dipole_multipart_operations_total` 与 `dipole_multipart_operation_duration_seconds`，覆盖 initiate、presign、register、upload_part、complete、abort 的结果和耗时；不记录用户、会话或对象标识，Redis 过期扫描与清理指标仍待 A7/AD-055。
- 2026-08-30：新增可选真实 MinIO 预签名代理集成测试，验证 Multipart `UploadPart` 经 Gateway 同源代理转发后仍通过 S3 Host 签名校验，并完成 ETag、Complete 和对象内容核对；测试通过后自动清理测试对象，完整故障矩阵与默认切流仍待 A7/AD-055。
- 2026-08-30：为开源 MinIO Bucket CORS 不可用场景增加默认关闭的 Gateway 同源 S3 PUT 代理：仅转发带完整签名的 Multipart 分片，固定上传 bucket、限制 PUT 方法和分片体积，并保留 Core 中转路径作为回切方案；真实代理启用和预签名端到端切流继续由 A7/AD-055 跟踪。
- 2026-08-30：真实 MinIO 验收确认开源 MinIO `RELEASE.2025-04-22T22-12-26Z` 不支持 Bucket CORS API，`mc cors set` 返回 `501 NotImplemented`；移除三套 Compose 中会导致初始化失败的 CORS 命令，预签名 Multipart 默认切流继续暂停，待补 Gateway 同源代理 CORS 或切换支持 Bucket CORS 的对象存储实现。
- 2026-08-30：补充平台存储 CORS 策略 XML 作为支持 Bucket CORS 的对象存储部署参考；当前开源 MinIO 不支持该 API，生产域名与浏览器直传仍需通过 Gateway 同源代理或兼容实现落地。
- 2026-08-30：Web Multipart 接入默认关闭的预签名直传试运行：批量获取绑定 `uploadId + partNumber` 的 MinIO URL，浏览器直接 PUT 后向 Core 登记 ETag/尺寸，失败沿用 Abort 回滚；新增 `VITE_MULTIPART_PRESIGNED_ENABLED=false` 配置，默认仍走 Core 中转路径，真实 MinIO/CORS 验收和默认切流继续由 A7/AD-055 跟踪。
- 2026-08-30：新增 Multipart 预签名 part URL 契约：Core 按用户归属和合法 part 编号批量签发绑定 `uploadId + partNumber` 的短期 MinIO URL，并同步 HTTP/Swagger contract 与回归测试；当前仍保持 Core 中转上传为默认路径，客户端直传切流、ETag 登记和真实 MinIO 验收继续由 A7/AD-055 跟踪。
- 2026-08-30：新增默认 dry-run 的 `dipole-multipart-cleanup` 运维工具，按 MinIO 发起时间筛选 `message-files/` 下的未完成 Multipart，输出可审计 JSON；只有显式 `--execute --confirm` 才执行 Abort，单个清理失败会保留结果并返回失败状态，Redis session 扫描、指标和真实 MinIO 集成仍由 A7/AD-055 跟踪。
- 2026-08-30：Web Multipart 上传接入客户端断点恢复基础：按文件指纹持久化 session，恢复前通过 Core 状态接口校验文件元数据并跳过已确认 part，完成或失败取消后清理本地 session；新增 helper 恢复测试，暂停/继续 UI 和预签名直传继续由 A7/AD-055 跟踪。
- 2026-08-30：新增受所有权保护的 `GET /api/v1/files/uploads/{session_id}` Multipart 会话状态接口，返回已完成 part 的编号、ETag 和实际尺寸；该 contract 为后续暂停/断点恢复提供基础，当前仍保持现有 Core 中转上传路径。
- 2026-08-30：Multipart part 增加 `X-Part-SHA256` 校验链路：现代 Web Crypto 可用时 Web 客户端发送摘要，Core 在保存 ETag/Size 前校验实际读取长度并恒时比较，校验失败不会登记可完成 part；旧客户端缺少该头时保持兼容，并同步更新 Swagger contract 与回归测试。
- 2026-08-30：Multipart 完成阶段新增 part 实际大小校验；新 Redis 会话保存 `ETag + Size`，前置分片和最后分片必须匹配初始化文件尺寸，旧 ETag-only 会话在无法证明完整性时安全拒绝完成，并通过 Core File/Storage 定向测试。
- 2026-08-30：Web 大文件 Multipart 上传新增可测试的有界并发与分片重试：默认 3 路并发、每个 part 最多 2 次指数退避重试，永久失败停止继续调度并沿用现有会话 Abort 回滚；暂停/断点恢复、预签名直传和 checksum 继续由 A7/AD-055 跟踪。
- 2026-08-30：确认文件上传已支持 MinIO 原生 S3 Multipart Upload：Web 端超过 `4 MiB` 自动进入初始化、5 MiB 分片和完成流程，Core 使用 `NewMultipartUpload`、`PutObjectPart`、`CompleteMultipartUpload`，失败时执行 Abort；当前默认上限为 `50 MiB`，后续 A7 计划增强预签名直传、并发重试、断点恢复、checksum 和未完成 upload 清理。
- 2026-08-30：将 Group、Conversation、Contact、Session domain-event decoder 下沉到对应 Core domain，删除生产代码对 `internal/compat/service` 的依赖，并新增门禁阻止兼容目录回流；事件校验和 Kafka 投递 contract 保持兼容。
- 2026-08-30：同步修正服务边界文档中已过期的 Message/Sync 兼容入口描述，明确剩余兼容目录仅承担跨版本 domain-event decoder 辅助；不改变运行时 contract。
- 2026-08-30：Message HTTP/WS 错误和 service 构造调用已统一迁移到 Message-owned contract，删除无调用者的 `internal/compat/service/message_compat.go`；兼容目录仅保留跨版本 domain-event decoder 辅助。
- 2026-08-30：Message event payload、mutation 和 Search/Sync projection 调用已统一迁移到 Message domain，兼容文件重命名并缩减为仍有调用者的 Message service/错误 contract；Kafka、Search、Sync 和 Gateway 事件行为保持兼容。
- 2026-08-30：Core Group 的 HTTP、DTO、Gateway Kafka 和 embedded 解码调用已统一迁移到 Core-owned Group domain contract，删除无调用者的 `internal/compat/service/group_compat.go`；群组 HTTP/Kafka contract 保持兼容。
- 2026-08-30：Core File 的 HTTP、DTO 和测试调用已统一迁移到 Core-owned File domain contract，删除无调用者的 `internal/compat/service/file_compat.go`；文件上传、分片会话和下载内容 HTTP contract 保持兼容。
- 2026-08-30：删除经全仓调用审计确认无调用者的 `internal/compat/service/sync_compat.go`，Sync domain 继续由 `internal/services/sync/domain` 唯一持有，其他兼容入口和回滚路径保持不变。
- 2026-08-30：为 `internal/application` 增加 contract ownership 说明和架构测试，禁止跨服务契约层依赖服务实现、旧数据层及运维目录，为 SQLC 与多语言协议演进固定边界。
- 2026-08-30：加强微服务 Compose 镜像隔离门禁，覆盖默认 Agent Runtime 和 Timeline repair profile 的独立镜像、构建上下文与服务入口；不改变默认 Go authority 或回滚路径。
- 2026-08-30：前端 Pencil 设计门禁新增批准导出清单校验，覆盖基础页面、Search、Sync 和 Agent 评审资产；缺失或空导出会在 `npm run test:design` 阶段 fail closed，不改变运行时行为。
- 2026-08-30：复核 SQLC-only 数据访问边界：`scripts/check-sqlc.sh`、服务布局门禁和 Go 全仓回归均通过；生产 Go 源码与 `go.mod`/`go.sum` 未发现 GORM/`AutoMigrate` 回流，`AD-010` 继续保持已解决。
- 2026-08-30：复跑 `scripts/smoke-runtime-dependency-readiness.sh`，确认 readiness 探针超时修复在完整微服务拓扑中生效；Core/Message/Sync/Gateway 冷启动、Gateway assignment、Elasticsearch 故障降级/恢复和核心服务不重启均通过，临时资源自动清理。
- 2026-08-30：为 `scripts/smoke-runtime-dependency-readiness.sh` 增加 `docker compose exec` 10 秒硬超时，避免 readiness 探针在 CLI/容器异常时无限等待；修复后完整 smoke 通过 Elasticsearch 停止/恢复、Search readiness 降级恢复、Gateway assignment 和核心服务容器不重启校验。
- 2026-08-30：运行 `services/agent-runtime` 的 `context:calibrate` fixture，报告 `eligible=true`，覆盖中文、代码、Emoji、英文和 Tool schema 5 类样本，生成稳定的 evidence/report SHA-256；该结果仅验证校准器链路，fixture tokenizer 和合成语料不能替代真实候选模型切流证据。
- 2026-08-30：执行 `scripts/check-agent-timeline-repair-alerts.sh` 与 `scripts/smoke-agent-timeline-repair-compose.sh`，验证 2 条 Prometheus 告警规则、migration v50、最小权限、worker readiness、启动前 pending intent 恢复和 event UUID 幂等重放；临时 Compose 资源自动清理，operator 灰度和默认生产开关保持关闭。
- 2026-08-30：在 `master` 当前基线复跑 Agent Runtime 与服务级回归门禁：Vitest 通过 125 个测试文件（662 个测试，7 个文件/27 个测试按契约跳过），`typecheck`、生产构建、Go 全仓、服务布局和 Compose 契约均通过；本次验证仍只覆盖 shadow/协议边界，不改变 active authority、Temporal 共享联调或生产灰度开关。
- 复跑 `scripts/smoke-sync-cassandra-primary-compose.sh`：隔离微服务 Compose 真实验证 Cassandra schema init、MySQL migration、Core/Message/Sync readiness 与 `primary=true` 配置；临时资源自动清理，生产 Cassandra 主读和共享环境灰度保持关闭。
- 复跑 `scripts/smoke-cassandra-read-routing.sh`：在隔离 MySQL/Cassandra 与 migration v50 环境验证 Timeline 页面使用 Cassandra，并在 payload 损坏或记录缺失时按同一 locator 安全回退 MySQL；临时资源自动清理，生产主读开关保持关闭。
- 将 Gateway HTTP/WS server、Agent 控制代理和 Search 边缘适配迁入 `internal/services/gateway/server/`，Gateway bootstrap 直接使用服务自有 server；共享 Gin handler 保留在 `internal/gateway/http/` 供兼容 Core server 复用。
- 将 Core HTTP/WS server、静态资源和通知适配器迁入 `internal/services/core/server/`，独立 Core 与 embedded 兼容入口统一使用 Core-owned server；旧 `internal/server/` 路径退役，HTTP、WS 和回滚语义保持兼容。
- 服务布局门禁新增 shared `internal/bootstrap` 根目录生产 Go 文件回流检查，embedded runtime 必须位于 `internal/bootstrap/embedded/runtime/`，独立服务 runtime 必须位于对应服务 bootstrap。
- 将 embedded 聚合 runtime 的 Message persistence ownership 策略迁入 `internal/bootstrap/embedded/`，runtime 通过 embedded-owned API 判断 local/gRPC/remote 组合；独立服务的 ownership 与回滚语义保持不变。
- 将 embedded 聚合 runtime 迁入 `internal/bootstrap/embedded/runtime/` 子包，避免 server 与 embedded composition 形成 import cycle；Core embedded 兼容入口、生命周期和回滚语义保持兼容。
- 将 embedded Kafka managed topic 清单及其契约扫描测试迁入 `internal/bootstrap/embedded/`；聚合 runtime 通过 embedded-owned API 确保 topic，独立服务的 consumer ownership 不变。
- 清理服务布局门禁中针对已删除 `internal/bootstrap/internal_rpc.go` 的无效检查，避免通过门禁时产生误导性 IO 警告；Core RPC 组合和 embedded Kafka 组合的当前归属与实际目录保持一致。
- 将 embedded Kafka 组合及其兼容测试迁入 `internal/bootstrap/embedded/`，聚合 runtime 通过 `RegisterKafkaHandlers` 调用；Conversation projection、群初始化、旧 Eino 触发、日志 handler 和实时投递注册顺序保持兼容。
- 迁移 `internal/bootstrap` RPC contract 测试至 `internal/services/core/rpc`，删除 Core RPC 组合的旧 bootstrap wrapper、类型别名和生产服务名常量；embedded runtime 直接持有 Core-owned RPC 类型，测试常量仅保留在测试辅助文件。
- 将 Core RPC 组合逻辑迁入 `internal/services/core/rpc/`；embedded runtime 直接使用 Core-owned composition，旧 `internal/bootstrap/internal_rpc.go` 已完全移除，RPC 协议、mTLS、caller allowlist 和 Agent 方法权限保持兼容。
- 修正文档中的 SQLC 仓储目录描述，使 `REPOSITORY-STRUCTURE.md` 与 `SERVICE-BOUNDARIES.md` 和结构门禁一致：`internal/data/mysql` 已退役，业务仓储由各服务 infrastructure 独占，`internal/platform/mysql` 仅保留共享数据库基础设施。
- 复测 `master` 上 1000 成员 Conversation SQLC 批量 upsert：四个固定 workload 子项通过，batch 相比 serial 约降低 46.2 倍，并发对照约降低 286.9 倍；SQL 层锁等待增量为零。证据见 `benchmarks/ad005-conversation-batch-2026-08-30/`，端到端 P95、多轮统计和共享拓扑容量仍待完成。
- 将仅供 embedded 聚合运行时使用的 Sync transport/shadow 实现迁入 `internal/bootstrap/embedded/`，共享 bootstrap 只保留生命周期编排；local/grpc/shadow 回退、设备 checkpoint 和同步查询语义保持不变。
- 删除经调用审计确认无调用者的 shared `NewMessageRPCServer`、`DialMessageApplication` 和 `DialCoreMessageApplication` facade；Message RPC server/client 统一由 Message/Gateway/embedded 自有 bootstrap 持有。
- 删除经调用审计确认仅被 contract 测试使用的 shared `NewSearchRPCServer` 与 `NewSyncRPCServer` facade；测试和生产入口统一使用 Search/Sync 自有 bootstrap。
- 删除经调用审计确认无生产调用者的 shared `DialSearchApplication`、`DialSyncApplication` 与 `DialCoreSyncApplication` facade；测试改用 Search bootstrap 或 embedded-owned client，RPC 认证和协议语义保持兼容。
- 删除经调用审计确认无生产调用者的 shared `DialSearchCoreCapability` 与 `DialSyncCoreCapability` facade；RPC contract 测试改用 Search/Sync 自有 bootstrap，Core capability 身份限制保持不变。
- 删除经调用审计确认无生产调用者的 shared `DialGatewayCoreCapability` facade；Gateway contract 测试和生产 runtime 统一使用 Gateway 自有 bootstrap。
- 将 Gateway Agent capability client 与 Message Core capability client 迁入对应服务 bootstrap，删除 shared `DialGatewayAgentCapability`、`DialCoreCapability` 及其通用拨号实现；身份和权限 contract 保持兼容。
- 删除经调用审计确认无调用者的 shared `internal/bootstrap.RunServer`；Core、Gateway 服务入口继续使用各自 bootstrap，embedded 聚合只保留初始化和生命周期组合。
- 删除无调用者的导出 `RestrictCoreServiceMethods` 包装；Core RPC server 继续使用内部私有策略实现，Agent/Search/Sync 权限规则保持不变。
- 将仅供 embedded 聚合运行时使用的 Message transport/shadow 实现迁入 `internal/bootstrap/embedded/`，共享 bootstrap 只保留生命周期编排；transport 行为、local/grpc/shadow 回退和测试语义保持不变。
- 删除经调用审计确认无生产或测试调用者的 shared Core Agent RPC control 包装 `NewCoreRPCServerWithAgentControl`；仍在 embedded contract 和运行时使用的 Agent RPC 装配保持不变。
- 将 Cassandra Projector runtime 从共享 `internal/bootstrap` 迁入 `internal/services/message/bootstrap`；独立工具改用 Message-owned bootstrap，Cassandra Timeline projection、Kafka consumer group 和回滚语义保持不变。
- 删除经全仓调用审计确认无调用者的 Core、Agent、Search repository alias 及 `internal/data/mysql` 历史兼容目录；各服务 SQLC repository 现在完全由服务 infrastructure 持有。
- 删除经全仓调用审计确认无调用者的 `internal/data/mysql/store_compat.go`；MySQL 事务边界统一由 `internal/platform/mysql` 持有，剩余历史 repository alias 继续按实际调用者治理。
- 删除经全仓调用审计确认无调用者的 `internal/store` MySQL/Redis 兼容入口；所有生产服务和运维工具统一使用 `internal/platform/mysql` 与 `internal/platform/cache`，旧共享 store 目录不再作为回滚入口。
- 删除经全仓调用审计确认无生产或测试调用者的 Message/Sync 历史 repository facade；Message 与 Sync SQLC repository 现在仅由各自服务 infrastructure 持有，Core、Agent、Search 兼容入口及 embedded 回滚边界保持不变。
- 删除无调用者的 `internal/bootstrap.RegisterGatewayKafkaHandlers` 兼容 facade，embedded Kafka 装配直接使用 Gateway infrastructure 注册器；Gateway Kafka 注册所有权完成收口。
- Gateway runtime 已直接使用 `internal/services/gateway/infrastructure/kafka.RegisterHandlers`，移除对共享 `internal/bootstrap` Kafka 注册兼容入口的生产依赖，并新增架构回流测试。
- Gateway Kafka 注册器与 realtime authority handler factory 已迁入 `internal/services/gateway/infrastructure/kafka`，共享 bootstrap 降为兼容转发；Gateway 的订阅注册、热群 detector、Notifier 和 fence 组合由服务边界统一持有。
- Gateway group message delivery handler 已迁入 `internal/services/gateway/infrastructure/kafka`，普通群逐用户 fan-out、hot-group notify 聚合、文件映射和 Timeline notify 均由服务自有实现持有；Gateway Kafka 共享 handler 实现已清理完毕。
- Gateway direct message delivery handler 已迁入 `internal/services/gateway/infrastructure/kafka`，保留文件消息映射、Timeline notify 三种模式和 WS 上下文传播；group message delivery 继续独立迁移。
- Gateway 群事件 Kafka handler（`group.created`、`group.updated`、成员变更和解散）已迁入 `internal/services/gateway/infrastructure/kafka`，新增泛型 fan-out 契约测试；Core 的会话初始化解码保持原有归属。
- Gateway `session.force_logout` Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka`，通过服务自有 `ConnectionController` 保持指定连接和全量连接断开语义，并补充契约测试。
- Gateway `contact.friend.deleted` Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka`，新增用户范围 WS 契约测试并保持 malformed event 重试语义。
- Gateway `conversation.direct.read` Kafka handler 已迁入 `internal/services/gateway/infrastructure/kafka`，新增契约测试并保持 WS read receipt 与 malformed event 重试语义。
- Gateway realtime delivery authority fence 已迁入 `internal/services/gateway/infrastructure/kafka`，embedded 装配改用服务自有实现；消息 delivery handler 仍按依赖闭包继续迁移。
- Gateway 热群通知聚合器及其测试已迁入 `internal/services/gateway/infrastructure/kafka`，共享 Kafka handler 改用服务自有 `Notifier` 与默认窗口；完整消息投递 handler 继续按依赖闭包分阶段迁移。
- 删除已无生产调用者的共享 readiness 实现与重复测试，Kafka assignment、authority fence 和依赖监控统一由 `internal/platform/runtime` 持有并验证。
- 时间线通知模式校验已下沉到 `internal/platform/runtime.ValidateTimelineNotifyMode`，Gateway、Core 和 embedded runtime 统一使用平台启动校验；删除共享 bootstrap 的重复实现与无调用者兼容入口。
- TLS 证书与私钥路径校验已下沉至 `internal/platform/runtime.ValidateTLSFiles`，Core、Gateway 和 embedded runtime 统一使用平台实现；删除共享 bootstrap 的重复 helper，TLS 启动失败语义保持一致。
- 删除已无调用者的 `internal/bootstrap.VerifyMessageDatabaseBoundary` 兼容转发；Message 数据库权限探针继续由 `internal/services/message/infrastructure/mysql` 唯一持有，embedded 与独立 runtime 均不改变权限语义。
- Embedded runtime 已直接持有并创建 `messagekafka.Relay`，删除仅供旧 bootstrap 内部使用的 Outbox 类型 alias/构造包装；Outbox relay 实现、启动条件和回滚行为保持兼容。
- Embedded Kafka 装配已直接注册 Message-owned persistence handlers，删除仅供共享 bootstrap 内部使用的 `RegisterMessageKafkaHandlers` 包装，避免继续扩散 Message Kafka 实现入口。
- Message bootstrap 的惰性 Core 重试测试已改用本地最小 gRPC adapter，不再反向导入共享 `internal/bootstrap` 测试夹具，解除 Message 服务测试包循环依赖。
- Embedded 兼容入口 `internal/bootstrap.NewMessageRPCServer` 已改为转发 Message bootstrap 的服务自有实现，删除共享 RPC 文件中的重复注册逻辑；旧调用方的协议、认证和回滚行为保持兼容。
- Message 独立 runtime 的 RPC server 字段已切换为 `internal/platform/rpc.Server`，移除对共享 `internal/bootstrap` RPC 类型别名的生产依赖；embedded 兼容入口继续保留，协议、认证和回滚行为不变。
- Message MySQL least-privilege database permission probe 已迁入 `internal/services/message/infrastructure/mysql`；独立 Message runtime 直接校验服务自有边界，embedded 入口保留兼容转发，权限探针单元测试与真实账号集成测试随服务 infrastructure 归属。
- Message `send_requested` 持久化 Kafka handler 已迁入 `internal/services/message/infrastructure/kafka`；独立 Message runtime 直接注册服务 handler，embedded 保留兼容注册包装。
- Message Outbox relay 已下沉至 `internal/services/message/infrastructure/kafka`；独立 Message runtime 直接使用服务实现，embedded 回滚路径保留薄兼容包装。
- Message shadow runtime 的 Query-only adapter 及契约测试已迁入 `internal/services/message/bootstrap/`，删除共享 bootstrap 中对应兼容 alias/构造转发；shadow 写拒绝和查询透传语义保持不变。
- Message bootstrap 已接管惰性 Core Capability adapter 及其重试测试，runtime 不再通过共享 `internal/bootstrap` facade 获取 Core capability；旧共享拨号能力仍为其他兼容调用者保留。
- 五条主要 Epic 分支已同步到当前 `master` 基线并推送：`epic/01-microservices`、`epic/02-storage-architecture`、`epic/03-agent-runtime`、`epic/04-cpp-realtime`、`epic/05-frontend-experience`；后续里程碑开发可继续保持阶段隔离。
- 删除已无实现的 `internal/app/agent_application_compat.go`，同步收紧服务布局门禁和仓库边界文档，避免通过空兼容文件维持过时结构。
- Agent Execution Policy 测试已直接使用 Agent application 的持久策略构造器；删除 `internal/app` 中无调用的策略 alias 与构造转发，进一步收敛 embedded compatibility facade。
- 清理 `internal/app` 中无调用的 `StaticAgentExecutionPolicyV1` 和 `AgentMemoryTaskReaderV1` 兼容符号；生产装配继续直接使用 Agent application 边界。
- Agent Execution Policy 测试已直接使用 Agent application 的 Resolver/Run Admission 构造器；删除 `internal/app` 中对应兼容类型与转发。
- Message Command Execution 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Workflow Repair Executor 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Workflow Repair Prepare 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Workflow Repair Audit 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Runtime Promotion Control 测试已直接使用 Agent application 的时钟注入构造器；删除 `internal/app` 中无调用的类型与构造转发。
- Agent Approval Service 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Task Workflow Projection 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Task Control 测试已直接使用 Agent application 构造器；删除 `internal/app` 中仅供该测试使用的兼容类型与转发。
- Active Run Promotion Authorizer 测试已迁入 Agent application 包并直接验证服务实现；删除 `internal/app` 中无调用的类型与构造转发。
- Agent Capability 测试已迁入 Agent application 包并直接验证服务实现；删除 `internal/app` 中仅供该测试使用的类型与构造转发。
- Agent Command 测试已迁入 Agent application 包并直接验证服务实现；删除 `internal/app` 中仅供该测试使用的类型与构造转发。
- Agent Event Subscription 控制面测试已迁入 Agent application 包并直接验证服务实现；删除 `internal/app` 中仅供该测试使用的类型与构造转发。
- Agent Memory Resolver 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent Runtime Promotion Evidence Review 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent Memory Owner Control 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent Artifact Service 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent Memory Candidate Promotion 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent MCP Tool Round 与 Terminal 测试已整体迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的 Round/Terminal 构造转发。
- Agent MCP Tool Round 测试已补齐本地最小 invocation reader stub，解除对 `internal/app` 共享测试辅助的隐式依赖。
- Agent MCP Readiness Evidence 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent Definition Catalog 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发。
- Agent Approval Grant Resolver 测试已迁入 Agent application 包并直接使用服务实现，删除聚合 `internal/app` 中对应的类型与构造转发；审批主服务测试因共享 policy stub 继续保留兼容边界。
- Agent application compatibility facade 已删除两个无调用者的未导出转发函数；仍被兼容测试使用的执行策略和任务辅助入口继续保留，服务 application 实现保持唯一来源。
- Core、Sync 和 Agent repository compatibility facade 已完成调用者清理并退休；生产与 embedded composition 直接使用各服务 infrastructure，服务布局门禁同步移除三项历史登记及过时的存在性断言。
- 聚合 `internal/app/composition_compat.go` 已退休；composition 测试已归属 `internal/bootstrap/embedded` 并直接验证 embedded repository/service 装配，`internal/app` 当前仅保留仍在迁移期的 Agent application 兼容边界。
- Core embedded compatibility facade 已移除无调用者的 Inbox 写入开关转发和旧 Message application 构造入口；Inbox projector 与 Message application 继续由服务专属/embedded composition 直接装配，保留仍有调用者的兼容 API。
- 服务布局门禁已同步移除已退休的 `internal/app/core_capability.go` 必需登记项，避免已删除的孤立 facade 阻断后续结构检查。
- Core 已删除无调用者的 `internal/app/core_capability.go` 兼容构造入口；Core 能力继续由服务自身 application 与 embedded composition 装配。
- Core 已移除无调用者的旧 `validateStandaloneCoreMode` compatibility facade，并将模式校验测试归属 `internal/services/core/bootstrap`；embedded 组合逻辑与回滚入口保持不变。
- Core bootstrap 已将 embedded 初始化兼容入口隔离到独立 `embedded_compat.go`；独立 Core entrypoint 文件不再直接依赖旧 bootstrap，embedded 回滚 API 保持兼容。
- Core 独立服务入口已自有 HTTP/TLS 启动与证书文件校验，embedded 模式仍通过兼容入口保留原有回滚路径；新增架构测试锁定服务入口不再转发旧 bootstrap 的 `RunServer`。
- 微服务 smoke 新增真实 `message.direct.created` 事件注入：通过 Kafka 连续发布同一事件两次，并在 MySQL 核对 EventLedger、Shadow Plan、Task 和 Shadow Run 的完成/幂等结果；当前仍保持 Agent shadow 与 Task `running` 生命周期。
- 微服务隔离 smoke 新增 Agent Runtime `/livez` 和 `/readyz` 检查；真实 Kafka/MySQL/Redis/MinIO 栈验证 Agent 加入 `dipole-agent-shadow-v1` consumer group 并获得主/retry topic 分区，事件触发仍保持 shadow 默认路径。
- Agent Runtime 独立进程 smoke 已验证默认安全配置下的 `/livez`、`/readyz` 和 SIGINT 优雅退出；Kafka、Temporal、MCP 与 Task Control 保持关闭，避免将本地启动验证误作外部依赖联调。
- Agent Runtime TypeScript protobuf generated files 已按当前 `api/proto` 重新生成，补齐 Message system-message RPC 并通过 proto drift 检查；Agent Runtime `661 passed / 27 skipped`，typecheck 和 build 均通过。
- AI assistant 用户 seed 已下沉到 `internal/services/core/application`，独立 Core 与 embedded 回滚路径共享 Core-owned 初始化能力；Core bootstrap 不再依赖旧 bootstrap 的 assistant seed facade，并新增缺失 Store 的测试覆盖。
- Core Conversation Kafka projection 已迁入 `internal/services/core/infrastructure/kafka`，独立 Core 直接注册 group/message projection；旧 `internal/bootstrap` 入口降为兼容转发，事件版本解码、Conversation Seq 映射和错误传播保持不变。
- SQLC/GORM 迁移复核已加入服务布局门禁：生产 Go 代码和 Go module 禁止重新引入 GORM；当前生产路径统一使用 `database/sql + sqlc`，兼容测试仍可保留历史命名语义。
- Core 独立服务的 runtime、system-message sender 和 RPC adapter 已迁移到 `internal/services/core/bootstrap`；embedded 启动仍保留为回滚路径，Kafka projection 与 assistant seed 通过显式兼容 facade 复用。
- Gateway 生产 RPC bootstrap 已直接使用 `internal/platform/rpc` 管理 Message、Sync、Core、Search 客户端和 realtime delivery observation server；Kafka handler、TLS 与时间线校验兼容边界保持不变，支持后续独立迁移。
- Sync 生产 RPC bootstrap 已直接使用 `internal/platform/rpc` 注册 Sync query adapter 和拨号 Core capability；旧 `internal/bootstrap` RPC wrapper 不再参与 Sync 生产装配，调用方白名单和回滚语义保持不变。
- Message 生产 RPC bootstrap 已直接使用 `internal/platform/rpc` 注册 Message adapter，并由 Message runtime 自有入口启动；旧 `internal/bootstrap` RPC server wrapper 不再参与 Message 生产装配，其他兼容依赖继续保持可回滚。
- Search 生产 RPC bootstrap 已直接使用 `internal/platform/rpc` 注册 Search adapter、执行服务认证拨号并管理 transport 生命周期；Core RPC legacy helper 限定在测试 fixture，Search 生产代码不再依赖 `internal/bootstrap`。
- Internal RPC 通用 transport 已下沉到 `internal/platform/rpc`，统一承载 gRPC listener、服务认证、TLS 1.3 mTLS、health check、拨号超时和优雅关闭；`internal/bootstrap` 保留薄兼容转发，协议 adapter 与方法权限继续按服务边界逐步迁移。
- 修复 Agent MCP RPC drill fixture 对已迁移 protobuf 生成目录的旧引用，统一使用 `api/gen/go/agent/v1`；master 全量 Go 测试恢复可执行。
- 修复 Gateway 服务入口 `RunServer` 递归调用自身的迁移回归，改为委托服务自有 `RunGatewayServer`；新增架构测试锁定入口委托关系，HTTP/WS 与 TLS 启动路径已通过验证。
- Gateway runtime 已从共享 `internal/bootstrap` 迁入 `internal/services/gateway/bootstrap`，服务入口直接拥有 HTTP/WS、Redis Presence/限流、Kafka 和实时投递 authority 装配；RPC、TLS 与 Kafka handler 兼容入口保留，旧 runtime 路径由结构门禁阻止回流。
- Message Service runtime 与配置校验测试已从共享 `internal/bootstrap` 迁入 `internal/services/message/bootstrap`，服务入口直接组合 Message-owned SQLC repository 和现有 Kafka/Cassandra/Outbox 能力；旧共享 runtime 路径由结构门禁阻止回流。
- Sync Service runtime、数据库权限边界校验及相关测试已从共享 `internal/bootstrap` 迁入 `internal/services/sync/bootstrap`，保留 Cassandra hydration、Kafka projector 和 Local 回滚语义；Internal RPC 暂由窄 compatibility adapter 承接。
- Search Service runtime 已从共享 `internal/bootstrap` 迁入 `internal/services/search/bootstrap`，Search 测试与 Elasticsearch/Core capability 装配同步归属服务边界；Internal RPC 暂由窄 compatibility adapter 承接，旧 runtime 路径由结构门禁阻止回流。
- Search Indexer 的 runtime 实现已从共享 `internal/bootstrap` 迁入 `internal/services/search-indexer/bootstrap`，直接拥有 Kafka consumer、Elasticsearch index 和 metrics/readiness 启动编排；旧路径由结构门禁阻止回流，Kafka/Elasticsearch 回滚语义保持不变。
- 运行时 readiness 编排已下沉到 `internal/platform/runtime`，统一提供依赖探针、gRPC health 检查、Kafka consumer 初始分配检查、Cassandra schema 检查和 RPC serving 绑定；服务特有启动条件继续留在各自 runtime，旧 `internal/bootstrap/dependency_readiness.go` 保留兼容出口。
- 新增 `internal/platform/runtime` 共享运行时平台，Core、Gateway、Message、Sync、Search、Search Indexer 和 Cassandra projector 统一使用平台 metrics 生命周期；旧 `internal/bootstrap/metrics.go` 降级为兼容出口，运行行为和回滚路径保持不变。
- Search Indexer 服务新增 `internal/services/search-indexer/bootstrap/` 入口边界，`cmd/services/search-indexer` 已停止直接依赖共享 `internal/bootstrap`；Kafka、Elasticsearch、metrics 和 readiness 运行时暂保留兼容 facade，支持后续分步抽离与快速回滚。
- Core 服务新增 `internal/services/core/bootstrap/` 入口边界，`cmd/services/core` 已停止直接依赖共享 `internal/bootstrap`，并显式区分独立 Core 与 embedded 回滚模式；RPC、Kafka、storage 和 readiness 运行时暂保留兼容 facade。
- Gateway 服务新增 `internal/services/gateway/bootstrap/` 入口边界，`cmd/services/gateway` 已停止直接依赖共享 `internal/bootstrap`；实时投递 authority、Kafka、Redis、RPC 和 WS/TLS 运行时暂保留兼容 facade，支持后续分步抽离与快速回滚。
- Sync 服务新增 `internal/services/sync/bootstrap/` 入口边界，`cmd/services/sync` 已停止直接依赖共享 `internal/bootstrap`；Kafka projector、Cassandra hydration、数据库和 gRPC 运行时暂保留兼容 facade，支持后续分步抽离与快速回滚。
- Message 服务新增 `internal/services/message/bootstrap/` 入口边界，`cmd/services/message` 已停止直接依赖共享 `internal/bootstrap`；Kafka、Outbox、Cassandra routing、gRPC 和 readiness 运行时暂保留兼容 facade，支持后续分步抽离与快速回滚。
- Search 服务新增 `internal/services/search/bootstrap/` 入口边界，`cmd/services/search` 已停止直接依赖共享 `internal/bootstrap`；底层运行时保留兼容 facade，便于后续独立抽离 gRPC、metrics 和 readiness 基础设施并支持快速回滚。
- 新增 `deploy/microservices/inbox-projector.yml` 可移除的 Inbox ownership 切换 overlay：绑定 Message projector 模式、`dipole_message_projector` 最小账号和 Sync projector 开关，并由 `scripts/check-compose.sh` 校验配置一致性；默认 atomic 回滚路径保持不变。
- 重新通过 `scripts/smoke-sync-write-ownership.sh`：真实 MySQL 8.4 验证 atomic/projector 最小权限、Inbox 写责任切换和 rollback contract；共享候选环境切换仍需维护窗口 receipt。
- 扩展隔离微服务消息 smoke，支持显式加载 Inbox projector overlay 并等待 Kafka/Sync 异步物化目标用户 Inbox；默认 atomic smoke 和自动清理行为保持不变，候选 projector 拓扑可独立验收。
- 通过 `SMOKE_INBOX_PROJECTOR=1 SMOKE_MESSAGE_FLOW=1 scripts/smoke-microservice-isolated-images.sh` 完成候选 projector 端到端验收：Gateway WebSocket 发送、Message/Outbox 持久化、Sync 异步 Inbox 物化、重复消息语义和 Seq 查询均通过；共享环境切换仍需维护窗口 receipt。
- 候选微服务 smoke 成功后新增 `dipole.microservices.smoke-receipt.v1` JSON receipt，绑定源码 revision、Compose project、projector 模式、dirty 状态和无数据迁移回滚动作；默认输出到 `/tmp` 并限制为 `0600`，可通过 `SMOKE_REPORT_FILE` 归档。
- receipt contract 实际通过：候选 projector 拓扑再次完成端到端消息验收，JSON schema、projector/message-flow 标志、无数据迁移回滚字段和 `0600` 权限均校验通过；共享环境 Kafka ownership 仍需维护窗口确认。
- Kafka 三节点故障与消费 ownership 演练通过：RF=3/min ISR=2 下验证单 broker 存活、低于 quorum 拒绝确认写入、consumer member 丢失后的 6 分区接管和 lag 归零；Prometheus 观测演练覆盖 lag、retry、DLQ、ISR 缺口及 broker 恢复。
- 修复 Kafka cluster observability profile 的 Prometheus rule-file 挂载漂移，补齐 duplicate hydration 和 Agent Timeline repair 规则，并在 `scripts/check-compose.sh` 增加挂载门禁；生产 Kafka ownership 切换和可执行回滚 receipt 仍按 AD-048 跟踪。
- Redis Sentinel 三节点故障演练通过：真实客户端完成 master 切换、Pub/Sub 重连、Presence、Hot Group 和限流恢复，旧 master 重新加入为 replica；可靠消息仍由 Kafka/Sync Timeline 提供补偿。
- 修复 Redis Sentinel 故障 smoke 使用旧 `internal/store` 测试包的问题，改为构建 `internal/platform/cache` 的真实故障测试，保持兼容目录仅作回滚出口。
- 修复 storage-lab 在受限宿主机上的 Elasticsearch 启动与磁盘水位问题：支持 `COMPOSE_PROJECT_NAME` 隔离调试，实验栈使用仅限 lab 的 `90%/99%/99.5%` 磁盘阈值，并为 API 版本探针增加有界重试；storage-lab Cassandra 5.0.9、Elasticsearch 9.5.2、MinIO CRUD smoke 已通过，生产水位配置未改变。
- Sync Cassandra primary Compose smoke 通过：隔离微服务拓扑完成 Cassandra schema init，Core、Message、Sync 与基础依赖达到 healthy，Sync primary hydration 配置和 readiness 验收通过并自动清理；生产主读灰度继续保持关闭。
- Cassandra read-routing 隔离 smoke 通过：真实 Cassandra/MySQL 双存储验证 Seq 页面主读，payload 损坏和缺失行按同一 cursor 回退 MySQL；该验证不改变默认生产主读比例，生产观测和回切审批继续受 AD-043 约束。
- 服务布局门禁现在同时检查已跟踪和未忽略的未跟踪兼容目录文件；负向测试确认未登记文件会 fail closed，避免本地新文件绕过 `internal/app`、`internal/store` 和 `internal/data/mysql` 的物理边界约束。
- 收紧微服务仓库物理边界门禁：`internal/app`、`internal/store` 和 `internal/data/mysql` 现在仅允许登记的兼容 adapter、SQLC 别名、README 与兼容测试；未知文件会 fail closed，避免服务抽取后重新形成共享实现区。现有兼容入口和回滚行为保持不变。
- 微服务隔离部署 smoke 通过：在独立 Compose project 和候选服务镜像上验证 Core、Message、Sync、Gateway、Agent 及基础设施的冷启动、readiness、metrics、TLS 1.3 mTLS、Core 代理和 remote WS ownership，并自动清理临时拓扑；共享环境发布切换与回滚 receipt 仍按架构债务台账跟踪。
- Agent Runtime 新增容器交付门禁 `scripts/check-agent-runtime-container.sh`：镜像绑定 OCI revision/created/dirty provenance，自动验证非 root `node` 用户与 foundation `/readyz`，为独立制品和回滚路径提供可重复检查。
- Agent Runtime 完成独立制品验证：`services/agent-runtime/Dockerfile` 构建成功，生产镜像仅包含编译后的 `dist` 与裁剪后依赖；容器以 `node` 用户运行，关闭 Kafka/RPC 的 foundation 配置下 `/readyz` 返回 200。
- TS Agent Runtime 独立 module 完成当前基线验证：`npm test -- --run` 通过（125 个测试文件、661 个测试），`npm run typecheck` 与 `npm run build` 通过；Compose 仍保持 shadow、metadata、foundation 默认回滚模式。
- 完成当前仓库结构基线验证：全量 `CGO_ENABLED=0 go test ./...` 通过，覆盖服务入口、运维工具、兼容包、平台层和 RPC/WS transport；根级目录白名单与服务边界门禁继续通过。
- 新增根级源码目录白名单门禁，明确 `api/`、`benchmarks/`、`cmd/`、`configs/`、`contracts/`、`db/`、`deploy/`、`design/`、`docs/`、`frontend/`、`internal/`、`scripts/` 和 `services/` 的归属；本地 `logs/`、`tmp/`、`dist/`、`certs/` 继续由忽略规则隔离。
- Agent bootstrap 已改用 Agent-owned application constructors，移除 runtime/kafka 对 `internal/app` 聚合 facade 的最后两处生产引用；服务布局门禁现禁止外部生产代码依赖该兼容入口。
- 校正平台演进计划的 Message transport 基线，区分 M3 历史 `local` 默认值与当前微服务 Compose 的远程 `grpc` 默认路径，避免把回滚配置误读为生产默认配置。
- 清理 MySQL 共享 repository 目录中的无调用者 contract test helper；各服务继续在自身 infrastructure 测试边界维护 contract database helper，历史兼容包仅保留别名和构造转发。
- 修正文档中的目录基线：明确共享 `internal/handler` 已清空，当前仅保留 `internal/store`、`internal/app` 和历史 SQLC 兼容入口，避免服务边界清单继续引用已删除的共享 Handler 目录。
- 补齐兼容目录的结构说明：`internal/app`、`internal/data/mysql`、历史 repository aliases 和 `internal/store` 均增加 ownership/迁移出口 README，服务布局门禁将其作为仓库导航约束；未改变兼容入口和运行时行为。
- 收敛 MySQL repository 调用边界：Sync 运维、Message/Sync/Cassandra 集成测试和 embedded composition 测试改用对应服务自有 SQLC repository；历史 `internal/data/mysql/repository` 兼容别名继续保留，但结构门禁禁止新的运行时代码依赖该路径。
- Compose 结构门禁新增 Core/Message 默认拓扑循环依赖检查：默认微服务配置禁止双方互相 `depends_on`，并继续要求 Core 使用远程 gRPC transport；Cassandra primary 的 embedded/local 回滚覆盖层保持兼容。
- Message Service 对 Core Capability RPC 改用惰性连接与就绪探针：Core 未监听时 Message 仍可完成启动，失败连接不缓存并在后续请求或探针中重试；关闭和 embedded/local 回退语义保持兼容。
- 更新平台演进计划的当前基线，使部署入口、Gateway/Message/Sync ownership、MySQL/Kafka/Cassandra/Redis 分层和 Go/Eino 到 TS Runtime 的过渡状态与仓库现状一致；保留 embedded 与 shadow/primary 回滚边界。
- 删除已无调用者的 `internal/service/event_publisher.go` 旧接口，并由服务布局门禁阻止 `internal/service/` 重新承载实现；跨服务事件契约继续使用 `internal/application` 和版本化事件包。
- Message RPC 新增 Core-only system message command，Core standalone 通过懒连接 adapter 将联系人/群组系统消息交给 Message Service 持久化；默认微服务配置启用远程路径，embedded 模式保持本地回滚。
- 将 Message 与 Core 共享的文件错误提升到 `internal/application` 契约，解除 Message domain 对 Core domain 实现的直接依赖；保留 Core/兼容入口的错误身份和 HTTP 错误映射。
- 重新整理一次性运维代码：将 Agent、Cassandra、Search、Sync 的回填、基线、清理、切换、证据和对账实现统一收纳到 `internal/operations/<service>/`，移除 `internal/backfill`、`internal/baseline`、`internal/cleanup`、`internal/cutover`、`internal/reconcile` 和 `internal/evidence` 横向遗留目录；补充目录索引与结构门禁，运行行为和工具入口保持兼容。
- 将 MySQL migration runner、DSN 配置迁入 `internal/platform/mysql/`，将 Agent/Cassandra/Search/Sync 的 MySQL 运维 adapter 与 contract test 迁入对应 `internal/operations/<service>/<operation>/mysql/`；`internal/data/mysql` 仅保留兼容入口，服务调用路径和回滚语义保持兼容。

- 将 Search 回填、归档、对账、Alias 切换和 Outbox 清理装配从 `internal/bootstrap/` 收纳到 `internal/operations/search/`，明确长期服务启动与一次性运维操作的目录边界；命令行入口、回滚语义和操作参数保持兼容。
- 将 Sync baseline/replay/reconcile 与 Cassandra backfill/archive/reconcile 装配从 `internal/bootstrap/` 分别收纳到 `internal/operations/sync/`、`internal/operations/cassandra/`；长期服务运行时、命令参数和回滚语义保持兼容。
- 将 Agent Memory lineage backfill 装配从 `internal/bootstrap/` 收纳到 `internal/operations/agent/`；dry-run、审批绑定、manifest 校验和回执语义保持兼容。
- 将 embedded 聚合 `Repositories`、`MessagingServices` 及其构造实现从 `internal/app/` 收纳到 `internal/bootstrap/embedded/`，`internal/app` 保留兼容 facade；服务启动行为和 Agent 兼容构造语义保持兼容。
- 将 protobuf Go 生成物从 `internal/transport/grpc/gen/` 收纳到 `api/gen/go/`，同步更新 `go_package`、生成脚本和全部 RPC 适配引用；RPC 方法、版本和 wire 兼容性保持不变。
- 将 MySQL 全局连接初始化从 `internal/store` 收敛到 `internal/platform/mysql`，生产启动入口、Bloom 初始化和 Agent 维护工具统一使用新平台边界；旧 MySQL 入口保留为兼容转发，Redis 迁移保持独立节奏。
- 将 Redis 客户端初始化和全局状态从 `internal/store` 收敛到 `internal/platform/cache`，同步迁移 Core、Gateway、Message、Presence、Hot Group、限流和 realtime 运维工具；旧 Redis 入口保留为兼容转发，单节点/Sentinel 配置和实时状态语义保持兼容。
- Hot Group Detector 新增显式 Redis 客户端注入，Core、Message、embedded 和 Kafka 投影装配统一传入平台客户端；无参数构造函数继续保留为兼容入口，检测阈值和热群策略保持不变。
- Presence 新增显式 Redis 客户端注入，Gateway、embedded Server 和 WebSocket 路由统一使用平台客户端；无参数构造函数继续保留为兼容入口，连接 TTL、节点状态和故障语义保持兼容。
- Rate Limiter 新增显式 Redis 客户端注入，Gateway、embedded Server 和 Agent MCP 入口统一使用平台客户端；普通业务限流的 fail-open 与 Agent MCP 的 fail-closed 语义保持兼容。
- Rate Limiter 执行路径改为只使用实例绑定的 Redis 客户端，全局 Redis 仅保留在兼容构造函数；多实例限流共享同一注入客户端的行为由集成契约覆盖。
- 整理多语言微服务目录：将 TypeScript Agent Runtime 和 C++ Realtime Delivery 从根目录收敛到 `services/`，同步更新 Compose、Docker、生成脚本、测试门禁和运行文档；Go 长期服务继续统一使用 `cmd/services/` 入口，根目录不再承载多语言服务源码。
- 将 Sync Kafka Projector 从 `internal/projector/sync` 收敛到 `internal/services/sync/infrastructure/kafka`，直接复用 Message domain 事件解码与 Inbox projection contract；新增目录说明和结构门禁，运行时注册、Kafka topic 和 atomic/projector 回滚语义保持兼容。
- 将 Search Indexer Kafka Projector 从 `internal/projector/search` 收敛到 `internal/services/search/infrastructure/kafka`，直接复用 Message domain 事件解码与 Search mutation contract；新增目录说明和结构门禁，Kafka retry/DLQ 与 Alias 回滚语义保持兼容。
- 将 Cassandra Message Projector 从 `internal/projector/cassandra` 收敛到 `internal/services/message/infrastructure/cassandra`，直接复用 Message domain 事件解码；保留 `cmd/tools/cassandra-projector` 可选入口，Cassandra shadow/primary 和 MySQL 回退语义保持兼容。
- 将 SQLC MySQL 事务 Store 和测试迁入 `internal/platform/mysql`，旧 `internal/data/mysql/store_compat.go` 仅保留类型别名与构造转发；事务语义、SQLC 生成输出和现有调用方保持兼容。
- 将 SQLC generated 输出和 MySQL mapper 从 `internal/data/mysql` 收敛到 `internal/platform/mysql`，同步更新 `sqlc.yaml`、漂移检查和全仓引用；旧 `internal/data/mysql` 不再承载 generated/mapper 目录，查询行为与兼容入口保持稳定。

### 变更
- 将 Search/Indexer 共用的 Elasticsearch client、版本化 schema、Alias 和 projection adapter 从 `internal/data/elasticsearch` 收纳到 `internal/platform/elasticsearch`，新增目录职责说明和结构门禁；Search 权限边界、Indexer 写入职责及 Alias 回滚语义保持兼容。
- 将 Cassandra 灰度读取、消息对照和 Sync hydration fallback 从 `internal/data/{routing,shadow}` 收纳到 `internal/platform/storage/{routing,shadow}`，新增存储平台目录说明和结构门禁；MySQL 主路径、shadow 指标和回退开关保持兼容。
- 将跨 Message/Sync 复用的 Cassandra Timeline、连接和 hydration 适配器从 `internal/data/cassandra` 收纳到 `internal/platform/cassandra`，新增目录职责说明和结构门禁；服务业务 projection 与编排保持原有边界。
- 将 Core、Sync 和 Message 的旧服务兼容入口从 `internal/service` 收纳到 `internal/compat/service`，新增兼容层说明与结构门禁；旧 `internal/service` 实现已清空，业务行为和回滚入口保持兼容。
- Core 文件分片会话的 Redis 访问已收敛到 `internal/platform/cache`，domain 保留会话协议并移除对聚合 `internal/store` 的直接依赖；事务写入、缺失 Redis 和上传回滚语义保持兼容。
- Core Auth TokenService 的 Redis 撤销状态访问已收敛到 `internal/platform/cache`，移除对 `internal/store` 全局客户端的直接依赖；写入和校验在 Redis 不可用时保持 fail-closed。
- Agent infrastructure contract tests 已改用 `internal/services/agent/application/` 的 Agent-owned application constructors，结构门禁新增 Agent 服务禁止依赖聚合 `internal/app` 的检查；embedded 兼容入口保持不变。
- 修复 Cassandra primary Compose override 的仓库文件挂载路径：schema 与 Sync primary 配置改用相对于 `deploy/compose/` 首个 Compose 文件的根目录路径，避免容器内将目标文件解析为目录；隔离 primary smoke 已重新通过。
- Agent Timeline repair 隔离 Compose smoke 通过：验证 v50 migration、UTC 时间基准、专用最小权限、worker readiness、启动前 pending intent 恢复和 event UUID 幂等；临时栈自动清理，默认 production profile 仍关闭。
- Agent Workflow repair 已完成 operator grant 版本与 CAS executor 的本地实现验证：migration v50 提供 `grant_version`/`can_execute`，执行与回滚均要求 fresh grant、projection hash 和事务性 CAS；公开生产控制面与共享环境演练继续关闭。
- 更新 `docs/architecture/DEVELOPMENT-ROADMAP.md`：将长期路线图收敛为 G0、微服务、分层存储、Sync、TypeScript Agent Runtime 和 C++ Realtime Delivery 轨道，移除旧 Cgo 必做主线叙述。
- 对齐面向当前读者的架构材料：更新面试问答、消息存储与同步策略，统一描述 sqlc、服务边界、Message Store、User Inbox、`message_seq`、`read_seq` 和 `sync_seq`；保留 `after_id` 与 `/messages/offline` 的兼容语义。
- 更新 `docs/architecture/ARCHITECTURE-QA.md`：同步 Message Store、User Inbox Timeline、Conversation Seq/read_seq、sqlc、微服务和分层存储现状，修正早期无 Inbox/GORM/纯模块化单体描述。
- Sync repository composition 已迁入 `internal/services/sync/infrastructure/mysql/`；embedded 聚合入口保留兼容别名，独立与聚合启动均通过 Sync-owned composition 构造 repository 集合。
- Agent SQLC repository composition 已迁入 `internal/services/agent/infrastructure/mysql/`；`internal/app` 仅保留 embedded 兼容别名，聚合入口通过 Agent-owned composition 构造 repository 集合。
- Core SQLC repository composition 与 User/Group/Contact cache adapter 已迁入 `internal/services/core/infrastructure/mysql/`；独立 Core Runtime 不再依赖聚合包中的 Core 实现，`internal/app` 仅保留 embedded 兼容别名。
- Agent Capability RPC 增加 remote authority 传输契约：Admission 支持 `candidate_version`，Run Complete/Finish 绑定显式 `runtime_id + mode`；TS client 默认保持 shadow，active client 必须提供 candidate version，Go Core 继续通过 promotion authorizer 决定是否允许 active。旧省略字段按 shadow 兼容处理，尚未改变生产 Agent 默认开关。
- Temporal 增加显式 `read_active` Activity profile：active Task 使用 Core RPC 返回的权威 ExecutionContext，并以同一 `runtime_id + mode` 完成终态绑定；Artifact 与写 Capability 继续只在 shadow/关闭路径可用。
- Agent Execution Policy、Invocation Resolver 和 Run Admission 实现已迁入 `internal/services/agent/application/`；兼容入口保留 deterministic clock 构造，结构门禁阻止旧策略实现回流。
- Agent application 剩余的 MCP tool terminal、Memory、Message command execution、Runtime promotion control 和 Runtime promotion 实现已迁入 `internal/services/agent/application/`；Bootstrap 已直接使用服务包，兼容入口与 deterministic clock 测试构造保持可回切。
- Workflow Repair Prepare 和 Executor application 实现已迁入 `internal/services/agent/application/`；测试通过兼容入口保持 embedded 回滚能力，结构门禁阻止旧实现回流。
- Agent Capability 与 Command application 实现已迁入 `internal/services/agent/application/`；消息、会话依赖改为服务接口，Bootstrap 直接使用服务包，结构门禁阻止旧实现回流。
- Agent Event Subscription application 实现已迁入 `internal/services/agent/application/`；定义读取和会话可见性依赖改为显式服务接口，Bootstrap 直接使用服务包，结构门禁阻止旧实现回流。
- Agent application 的 Artifact 和 Memory Owner 实现已迁入 `internal/services/agent/application/`；Artifact policy 依赖改为显式服务接口，Bootstrap 直接使用服务包，结构门禁覆盖已迁移文件。
- Agent application 的 MCP readiness、MCP tool round、tool invocation audit、Runtime promotion evidence 和 Workflow repair audit 实现已迁入 `internal/services/agent/application/`；Bootstrap 与 SQLC 契约测试直接使用服务包，结构门禁覆盖全部已迁移文件。
- Agent application 的审批、审批授权、任务控制、Definition Catalog、Memory Candidate Promotion 和 Task Workflow Projection 实现已迁入 `internal/services/agent/application/`；embedded 入口保留兼容转发，结构门禁阻止已迁移实现回流。
- Agent application 的审批、审批授权和任务控制实现已迁入 `internal/services/agent/application/`；embedded 入口通过兼容别名保持回滚能力，Bootstrap 与 Agent SQLC 契约测试已改用服务专属包。
- 清理共享 MySQL repository 中已无调用者的事务别名和 UUID 辅助文件，并将共享目录约束收紧为兼容入口集合。
- Search Index SQLC repository 及契约测试已迁入 `internal/services/search/infrastructure/mysql/`；共享 repository 仅保留兼容别名和构造入口，服务布局门禁已阻止 Search 数据访问实现回流。
- Core 专属 sqlc MySQL repository 及契约测试已迁入 `internal/services/core/infrastructure/mysql/`；共享 repository 仅保留兼容别名和构造入口，服务布局门禁已阻止 Core 数据访问实现回流。
- Agent 专属 sqlc MySQL repository 及契约测试已迁入 `internal/services/agent/infrastructure/mysql/`；共享 repository 仅保留兼容别名和构造入口，服务布局门禁已阻止 Agent 数据访问实现回流。
- 修正 Sync/Message ownership smoke 在 repository 迁移后仍指向旧测试包的问题；新增测试 selector 命中校验，避免 `go test` 无匹配时误报成功，并重新验证真实 MySQL atomic/projector/rollback 权限边界。
- Compose 编排完成目录收敛：默认本地入口保留在根目录，其余微服务、分发、集群和存储实验拓扑统一迁入 `deploy/compose/`；脚本、文档和归档引用已同步，新增目录索引，Compose 配置门禁覆盖迁移后的全部拓扑。
- 保留 TypeScript Agent Runtime 的独立 `go.mod` 扫描边界，并修正 repair worker Compose 契约测试，使其校验统一 `/app/service` 入口和 Dockerfile 制品路径。
- 收紧 Inbox ownership 配置：`message.inbox_write_mode=projector` 现在必须同时启用 `sync.projector_enabled` 和 Kafka；配置不完整时 Message runtime fail closed，`atomic` 回滚路径保持不变。
- Sync 独立 runtime 已直接装配 Sync-owned repository、hydrator、projection 和 process composition，移除对 `internal/app` 聚合 Composition Root 的依赖；embedded 模式兼容入口保持可回滚。
- Sync 专属 MySQL repository、hydrator、projection 和 process composition 已迁入 `internal/services/sync/infrastructure/mysql/`；Sync 独立 runtime 直接装配服务边界，embedded 聚合兼容入口继续保留。
- Message 独立 runtime 已直接装配 Message-owned repository composition 和 application factory，移除对 `internal/app` 聚合 Composition Root 的依赖；embedded 模式兼容入口保持可回滚。
- Message 专属 sqlc MySQL repository 及 contract tests 已迁入 `internal/services/message/infrastructure/mysql/`；共享 `internal/data/mysql` 仅保留基础 Store、生成代码和其他服务 repository，旧构造入口继续兼容。
- Message 核心 domain 实现及测试已迁入 `internal/services/message/domain/`；旧 `internal/service` 仅保留兼容类型、错误和构造入口，消息发送、历史查询、幂等、Outbox、Seq、文件授权和热群策略 contract 保持兼容。
- Message event contract 与 Sync projection 实现及测试已迁入 `internal/services/message/domain/`；旧 `internal/service` 仅保留类型、错误和函数兼容入口，事件版本、Mutation、Search 和 Inbox locator contract 保持兼容。
- Sync domain 实现及测试已迁入 `internal/services/sync/domain/`；旧 `internal/service` 仅保留兼容错误和构造入口，User Inbox Timeline、设备 Cursor 和群组 checkpoint contract 保持兼容。
- Core 新增独立 Composition Root `InitializeCoreService`；remote 模式只装配 Core-owned repository、Core HTTP、Core projection Kafka consumer 和 Capability RPC，embedded 模式保留聚合启动器作为本地兼容与回滚路径。
- Core Conversation domain 实现及测试已迁入 `internal/services/core/domain/conversation/`；旧 `internal/service` 仅保留兼容别名和构造入口，会话列表、群组会话投影、已读回执和备注 contract 保持兼容。
- Core Contact domain 实现及测试已迁入 `internal/services/core/domain/contact/`；旧 `internal/service` 仅保留兼容别名和构造入口，好友关系、联系人申请、备注、拉黑和删除事件 contract 保持兼容。
- Core User domain 实现及测试已迁入 `internal/services/core/domain/user/`；旧 `internal/service` 仅保留兼容别名和构造入口，用户资料、头像、用户搜索和管理员用户状态 contract 保持兼容。
- Core Session domain 实现及测试已迁入 `internal/services/core/domain/session/`；旧 `internal/service` 仅保留兼容别名和构造入口，设备会话查询、强制下线和 Token 撤销 contract 保持兼容。
- Core Admin domain 实现及测试已迁入 `internal/services/core/domain/admin/`；旧 `internal/service` 仅保留兼容别名和构造入口，后台概览 HTTP contract 与 `ErrAdminRequired` 语义保持兼容。
- Core Auth/Token domain 实现及测试已迁入 `internal/services/core/domain/auth/`；旧 `internal/service` 仅保留兼容类型、常量、错误和构造入口，现有认证、MCP 授权、Middleware、Gateway 与 WS contract 保持兼容。
- Gateway 全部 HTTP handler 及测试已从 `internal/handler/http` 迁入 `internal/gateway/http/`；保留现有路由、认证、错误响应和 application port contract，旧共享目录由结构门禁阻止回流。
- Search HTTP handler 已迁入 `internal/gateway/`，Search application 继续位于 `internal/services/search/application/`；公共 API 和错误响应保持兼容，结构门禁会阻止旧通用路径回流。
- Sync application 装配已迁入 `internal/services/sync/application/`；`internal/app` 仅通过 `SyncApplication` port 组合，embedded Core 兼容路径和独立 Sync runtime 的行为保持一致。
- Message application 装配已迁入 `internal/services/message/application/`；保留 Agent command、Outbox 和消息持久化扩展能力，embedded Core 与独立 Message runtime 继续使用同一服务专属 factory。
- Core capability 实现已迁入 `internal/services/core/application/`；factory 改用用户、联系人、群组、文件和会话查询所需的最小接口，embedded 兼容构造入口保留，便于后续 Core 服务独立部署。
- Core Conversation application 装配已迁入 `internal/services/core/application/`；Core 的 embedded 与独立 runtime 共用服务专属 local adapter，底层 ConversationService 保持兼容并列入后续物理迁移。
- Core User application 装配已迁入 `internal/services/core/application/`；Server 继续使用原 User HTTP contract，同时将 User/File store 和对象存储依赖收敛到 Core 服务 factory。
- Core Contact application 装配已迁入 `internal/services/core/application/`；Server 继续使用原联系人 HTTP contract，同时将联系人 store、事件、通知和系统消息依赖收敛到 Core 服务 factory。
- Core Group application 装配已迁入 `internal/services/core/application/`；Server 继续使用原群组 HTTP contract，同时将群组 store、事件、热群、文件、对象存储和系统消息依赖收敛到 Core 服务 factory。
- Core File application 装配已迁入 `internal/services/core/application/`；Messaging composition root 继续使用原文件 HTTP contract，同时将 File metadata、Message store 和对象存储依赖收敛到 Core 服务 factory。
- Core Auth/Admin/Session application 装配已迁入 `internal/services/core/application/`；Server 继续使用原 HTTP contract，同时将认证、后台统计和设备会话依赖收敛到 Core 服务 factory。
- Core Group domain 实现及测试已迁入 `internal/services/core/domain/group/`；旧 `internal/service` 仅保留兼容类型和错误别名，现有 HTTP、DTO 与 Kafka contract 保持兼容。
- Core File domain、Redis 分片会话实现及测试已迁入 `internal/services/core/domain/file/`；旧 `internal/service` 仅保留兼容类型和错误别名，现有文件 HTTP 与 DTO contract 保持兼容。
- 聚合 repository composition 已显式保存 Core、Message、Sync、Agent 四类 process-owned repository 分组，并由 embedded 兼容入口复用；为后续独立启动链切换保留可回滚边界。
- 新增 `CoreProcessRepositories`，集中装配 Core 所有的 SQLC repository，并由聚合 `NewRepositories` 复用；现有 embedded 入口保持兼容，便于后续 Core 独立 runtime 切换。
- 新增 `AgentProcessRepositories`，集中装配 Agent-owned SQLC repository，并由聚合入口复用；Core 兼容 Capability 继续通过 port 使用，便于后续 TS Agent Runtime 独立接管。
- Search application 已从共享 `internal/app` 迁移到 `internal/services/search/application/`，Search runtime 保持原 application port 不变；结构门禁会阻止旧实现路径回流。
- 微服务远程模式下 Gateway 直接拥有消息历史与 Sync HTTP 路由，新增 Sync gRPC 连接和 readiness 依赖；Core 仅在 embedded 模式注册消息/同步数据路由，减少 Core HTTP 反代对服务 ownership 的绕行。
- Gateway 现在直接拥有消息历史和 Sync HTTP 路由，并通过 Message/Sync gRPC 客户端访问；Core 在 `gateway.mode=remote` 下不再注册消息/同步 HTTP 与 WebSocket 数据路由，embedded 模式继续保留兼容入口。
- 增加 Gateway/Core 路由 ownership 测试和 Sync RPC readiness/连接清理，避免远程部署通过 Core HTTP 反代绕过 Message/Sync 服务。
- 新增 `docs/architecture/SERVICE-BOUNDARIES.md` 和 `cmd/services/README.md`，明确服务入口、数据所有权、允许共享层与渐进迁移例外；结构门禁现在校验服务边界清单存在且已纳入版本控制。
- 明确 `internal/` 当前是迁移中的共享实现区，后续按 Core、Message、Sync、Search 和 Agent 责任逐步收敛，避免把入口拆分误判为业务实现已经完全自治。
- 微服务 Compose 为 Core 增加独立的 `DIPOLE_CORE_MESSAGE_TRANSPORT` 启动配置，默认使用本地消息实现完成 Core readiness；全局 `DIPOLE_MESSAGE_TRANSPORT=grpc` 继续保留给 Gateway/远程调用方，解除 Core/Message 冷启动环。
- 远程 Gateway 模式下 Core 不再注册消息持久化 Kafka handlers 或负责消息 topic 初始化，避免 Core 的本地启动兼容实现与 Message Service 形成双重 owner。
- Sync 微服务 Compose 补齐 `DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION`、`DIPOLE_CASSANDRA_ENABLED` 和 `DIPOLE_CASSANDRA_HOSTS` 配置契约；primary hydration 仍默认关闭，启用时保留 shadow 互斥和 MySQL 即时回退。
- 在干净提交 `fe84b7b` 上重建并验证七个候选微服务镜像，确认 revision、`dirty=false`、服务二进制标签一致；独立消息流程再次通过 Inbox、`before_seq` 历史读取和 `after_seq` 增量读取。
- 候选微服务消息 smoke 增加 Seq Timeline 验收：同一消息写入后分别通过 `before_seq=0` 和 `after_seq=0` 查询，并校验 `message_seq` 持久化结果，补齐历史读取与增量读取证据。
- 修正候选拓扑发布前的源码脏状态判定，仅阻断已跟踪文件变更，允许仓库中的本地 `.planning/` 和 `.codex/` 记录存在而不污染候选镜像发布门禁。
- 增加 Go 微服务单镜像候选路径：`core`、`gateway`、`message`、`sync`、`search` 和 `search-indexer` 可分别构建只包含自身二进制的镜像，并通过 Compose override 灰度；override 同步覆盖旧 entrypoint，默认仍使用共享 `DIPOLE_IMAGE`，移除 override 即可回滚。
- 隔离镜像 smoke 已固化为 `scripts/smoke-microservice-isolated-images.sh`，覆盖迁移、服务 readiness、Gateway health 和独立 project 清理；可使用临时 Gateway 端口验证候选栈，不干扰已有 Dipole 实例。
- 隔离镜像 smoke 增加 `SMOKE_SEARCH_PROFILE=1` 可选门禁，可在不改变默认核心 smoke 的情况下启动并检查 Search、Search Indexer 与 Elasticsearch 候选路径。
- 隔离镜像 smoke 增加 `SMOKE_MESSAGE_FLOW=1` 可选端到端消息验收：经候选 Gateway 注册/登录和 WebSocket 发送后，核对 Message、Outbox 与目标用户 Inbox 持久化。
- 完成仓库文档归档：`docs/` 顶层仅保留索引、清单和架构图；Agent、架构、数据、运行、性能、指南与参考材料分别归档到对应子目录，并将参考链接从根目录 `acc.txt` 收纳到 `docs/references/`。文档门禁现在检查顶层文件白名单和旧路径回流。
- Agent Workflow Repair 增加跨 Go/TypeScript 对齐的 projection hash precondition guard，执行前校验 active executor grant、grant version、Task 绑定和当前/目标 projection 哈希；该 guard 无副作用。
- 重整仓库文档布局：根目录 README 聚焦项目介绍、架构概览、快速开始和验证入口；架构、数据、运行、前端和性能文档统一归档到 `docs/` 分类目录，并由 `docs/README.md` 集中导航。
- 将长期运行的 Go 服务入口统一归档到 `cmd/services/`，保留一次性迁移、回填和对账工具在 `cmd/` 顶层，降低微服务部署边界与运维工具的混淆。
- 将一次性迁移、回填、对账、证据采集和诊断工具统一归档到 `cmd/tools/`，并同步构建脚本、运行手册和测试引用。
- Workflow Repair operator grant 增加可验证的 `grant_version` 和独立 `can_execute` 权限字段。历史授权默认不获得执行权限，后续 CAS executor 必须同时校验执行能力、版本和有效期。
- Workflow Repair execution ledger 增加 `prepared -> executing -> failed` 的执行人、授权版本和状态 CAS 边界；当前仅提供持久状态认领与失败终止，projection 写入、commit 和 rollback 仍保持关闭。
- Workflow Repair 增加事务化 projection commit：在同一 MySQL 事务内完成 projection CAS 与 execution `committed` 更新，任一条件失败都会回滚。
- Workflow Repair 增加事务化 rollback：支持恢复已提交 projection 或清空 projection，并将 execution 原子标记为 `rolled_back`。
- Workflow Repair 增加默认未接线的应用层 Executor：执行前重新读取执行人 grant、Task projection 和 canonical hash，完成 claim、precondition、事务 commit；失败记录固定 failure code，rollback 要求原执行人携带 fresh grant 并校验 rollback hash。
- Workflow Repair Executor 在执行入口校验计划声明的 rollback projection 与 `rollback_sha256`，缺失或漂移时在 claim 前 fail closed。
- Agent Workflow repair 增加 Gateway-only 的 Execute/Rollback gRPC 契约与执行器注入点；接口默认保持关闭，未配置执行器时返回 `Unavailable`，继续禁止未经组合根接线的真实 projection mutation。

### 新增
- 增加可选 `cassandra-primary` 微服务 Compose profile：启动 Cassandra 5.0.9、一次性 Timeline schema init，并将 Sync 接入 Cassandra-first hydration；profile 默认关闭，移除 profile 或关闭开关即可回退 MySQL。
- 增加 `smoke-sync-cassandra-primary-compose.sh`，验证真实容器网络中的 Cassandra schema init、Sync primary readiness 和自动清理边界；该 smoke 不改变默认生产开关。
- 微服务 Compose 默认切换为 `migrate/core/gateway/message/sync/search/search-indexer` 各自的单服务镜像，并补充可选 Timeline repair worker 镜像；legacy Compose 和逐服务镜像变量保留回滚路径。
- Context Compiler 增加 provider-neutral `RouteTokenizerAdapter`：按模型 route 注入稳定 tokenizer ID、上下文窗口和 token 计数，跨 route 仍取保守最大估算；未配置 tokenizer 时继续使用校准 UTF-8 fallback，避免未经证据直接绑定 provider。
- Agent promotion publication 增加受保护的 release manifest 入口：CLI 检测 manifest 后强制校验 `shadow` 阶段、candidate、offline Eval Suite SHA-256 和四类组件哈希，并将 manifest 哈希写入发布 Artifact 与低敏 receipt；旧证据回放入口保持兼容。

- Agent Runtime 增加 `dipole.agent.release-manifest.v1`：将模型、Prompt、Capability Schema、Memory Policy 与 offline Eval Suite SHA-256 绑定到候选版本，晋级校验只接受 `shadow` 阶段并拒绝版本或评测哈希漂移；该清单不改变生产开关。

- Agent Approval 与 Elicitation 表单已迁移到共享 `--dp-*` Pencil token，并增加设计契约测试，避免交互页面重新引入独立主题变量。
- Agent Approval 页面增加认证浏览器验收，覆盖精确审批绑定、权威查询失败时的 fail-closed 状态和移动端单列布局。
- Agent Approval 与 Elicitation 增加 Chromium canonical 截图回归，固定 Pencil 共享 token 的主要桌面布局；功能行为继续由三浏览器 E2E 覆盖。
- Agent Subscription 与 Memory 管理页增加 Chromium canonical 截图回归，覆盖已迁移共享 token 的 Agent 治理控制面。
- Search Workspace 清理残留硬编码主题值并统一映射到 Pencil `--dp-*` token，新增设计契约测试覆盖搜索、错误和骨架状态。
- Search Workspace 增加 E2E visual harness，建立 Chromium canonical 的 Idle、Loading、Results、Empty、Error 五态截图基线。
- Timeline Repair 增加 `agent-timeline-repair-rollout` v1 只读灰度门禁：绑定 worker readiness、operator、部署 revision、告警状态、回滚演练和 outcome 比例，输出低敏 `eligible|blocked` 报告；门禁不会自动启停 worker 或打开生产开关。
- Timeline Repair rollout 契约补充 eligible/blocked 脱敏示例，明确示例仅用于 CLI 回归，不能替代共享环境灰度证据。
- Event Subscription 增加可复用的 `off/shadow/enforced` 预筛运行时门禁：强制模式精确绑定 rollout decision、候选配置、语料、评审与 evidence 哈希，证据缺失或漂移时 fail closed；默认不接入 Kafka、模型或生产 Task 创建。
- C++ projection benchmark 在候选 revision `c063594` 上完成 100,000 次固定 workload 复跑，C++/Go ops ratio 为 `0.0976283897`，低于 `1.0` 门槛，继续保留 Go projection 并归档可复现报告。
- Conversation Projection 增加 sqlc 批量群消息 upsert：在保持 sender `read_seq`、成员未读计算、Seq 单调更新和幂等语义的前提下，将普通群一次消息的数据库写入收敛为单条 `INSERT ... SELECT`；旧 Repository/test double 继续兼容逐成员路径。
- Conversation Projection 归档真实 MySQL 8.4.8 的 1000 成员 SQL 对照：serial、batch、并发 serial、并发 batch 均通过行数/序号校验，batch 数据库层耗时约降低 37.3-353.8 倍，InnoDB row-lock wait 增量为零；该证据仍不替代端到端 P95 容量测试。
- 修正 MySQL migration integration baseline 与实际 v49 schema 漂移：补齐 v49 到 v44 的逐步回滚/表数断言，并恢复 Metadata backfill 测试窗口；隔离 MySQL 8.4.8 + Cassandra hydration smoke 全部通过。
- Cassandra read-routing 隔离 smoke 已在迁移 v49、临时 MySQL 与 Cassandra 环境通过：Cassandra 页面读取、payload 损坏回退和缺失行回退均符合契约，临时资源已自动清理。
- Kafka Shadow Runtime 增加可选 Subscription gate 注入点；enforced blocked 会在订阅匹配和 EventLedger claim 前停止，默认未注入以保持现有路径兼容。

- Agent Runtime 增加受认证的只读 `conversation.read` Capability：Go Core 通过新增 gRPC RPC 执行 Task/Run 身份解析与精确资源复核，TypeScript 注册同名 Capability 并将会话消息作为受 provenance 约束的上下文证据候选；协议为向后兼容新增，无数据库迁移。
- `conversation.read` 输入统一采用 canonical `conversationId`（`direct:<user>:<user>` 或 `group:<group>`）；Runtime 先执行精确 scope 检查并解析目标，Core 继续以 principal 的会话关系作为最终授权依据。
- ModelShadowPlanner 现在可在模型调用前按 event conversation key 读取最多 20 条授权消息，将 full/compact 消息作为 `untrusted` evidence 编译并记录来源/sequence；Temporal read activity 与普通 shadow registry 统一注册 `conversation.read`，读取失败保持 fail-closed。
- 修正会话 evidence 中 protobuf Timestamp 的 `bigint` 序列化：`seconds` 统一转为字符串，避免真实消息在 Context 编译时触发 JSON 序列化异常。
- 会话 evidence 增加防御性边界：Planner 最多编译 20 条远程消息，单条正文最多 8 KiB，并通过 `contentTruncated` 标记截断，避免异常响应放大内存和上下文预算。
- TypeScript Agent Capability RPC 客户端新增 `conversation.read` 跨语言契约测试：固定 group/direct canonical key 的 target 解析、可信 principal 请求边界、非法 scope 拒绝和响应 target 冲突 fail-closed 行为。
- `conversation.read` RPC 客户端在边界处拒绝超过请求 `limit` 的消息响应，并对 `found=true/false` 统一校验 target，避免异常响应绕过 Planner 上限或造成资源范围漂移。
- Context Compiler 的 capability section 现在可消费已注册 Capability Descriptor，向模型提供排序稳定的 `id`、`risk` 和 `requiredPermission` 元数据；运行时从 Registry 注入，旧调用仍兼容 ID-only 表示。
- Context Compiler 为每个选中的 full/compact fragment 生成 `contentSha256` 并写入 Shadow Plan manifest，支持跨进程重放与 descriptor/context 漂移核验；审计仍不保存 prompt 正文。
- `conversation.list` 与 `conversation.read` 注册 descriptor 增加受限输入 Schema 摘要（类型、范围、默认值和额外字段策略），模型可按契约生成参数，执行时仍由 Zod 进行最终校验。
- Capability Registry 注册时校验输入 Schema 摘要：限制可投影关键字、`properties` 嵌套结构和 4 KiB 大小，未知字段或异常膨胀会在进入模型上下文前失败。
- Capability Registry 注册后深度冻结 descriptor snapshot，防止外部对象修改风险、权限或输入 Schema 后造成执行策略与模型上下文漂移。
- Event Subscription matcher 增加 256 条候选上限，超限集合在 Schema 解析和关键词匹配前 fail-closed，避免异常订阅配置放大单条 Kafka 事件的 CPU/内存开销。
- 修正 Subscription Shadow 观测的候选计数：指标现在记录 Core 返回的原始候选数，不再使用匹配结果数低估 miss 场景的预筛选成本；matcher 错误会保留已取得的候选计数。
- Subscription Shadow metrics observer 增加运行时 outcome 枚举校验，仅接受 `match`、`miss`、`error`，拒绝未注册的低基数标签进入 Prometheus 输出。
- Subscription Shadow 的 HTTP Prometheus Collector 增加 256 KiB 响应体上限，使用流式读取并在超限、截断或 JSON 无效时返回固定错误，避免外部响应放大 Collector 内存占用。
- 微服务 Compose 增加默认关闭的 `agent-timeline-repair` profile：独立运行 Timeline repair worker，使用专用 MySQL 账号和最小表级权限，提供可选 readiness/Prometheus 端口；未显式启用 profile 时，默认服务拓扑保持不变。
- Go 发布链路现在构建并打包 `dipole-agent-task-timeline-repair`，避免运维进程仅存在源码而无法进入服务镜像。

### 迁移说明

- 本次新增 Agent Capability RPC 不要求数据库迁移；升级时需同时部署匹配的 Go gRPC 服务和 TypeScript Runtime 生成代码，旧 Runtime 可继续使用既有 RPC。`conversation.read` 仍受 Task/Run 解析、Core 资源授权和 Runtime `conversation.read` 权限三重约束。
- 启用 repair worker 前执行 `docker compose --profile agent-timeline-repair up -d agent-timeline-repair`；Compose 会先等待 `mysql-permissions` 完成。共享环境应覆盖 `DIPOLE_AGENT_TIMELINE_REPAIR_MYSQL_PASSWORD`，并在发布前替换授权 SQL 中的示例密码。

### 验证
- 2026-08-29 使用随机 Compose project 和 `18180` 隔离端口完成候选微服务端到端 smoke：Core、Message、Sync、Gateway、Agent 均 healthy；注册/登录、好友关系、WebSocket 发送、Message/Outbox/Inbox 幂等以及 `before_seq`/`after_seq` Timeline 读取均通过，测试资源自动清理。
- 通过 `scripts/smoke-sync-write-ownership.sh`：真实 MySQL 8.4 最小权限、Message atomic/projector 写入边界和 rollback 测试均执行并通过。
- 通过 `scripts/smoke-sync-projector.sh`：三节点 Kafka backlog/实时事件收敛、retry/DLQ 可观测性和热群 fanout 禁用契约均通过。
- 2026-08-29 使用独立 Compose project 实测 `SMOKE_SEARCH_PROFILE=1`：Elasticsearch、Search Indexer、Search、Core、Message、Sync、Gateway 和 Agent 均通过 health/readiness，Gateway health 通过，临时资源自动清理。
- 2026-08-29 `smoke-sync-write-ownership.sh` 与 `smoke-sync-projector.sh` 通过：真实 MySQL 验证 Message atomic/projector 权限和 Inbox ownership 迁移/回滚，三节点 Kafka 验证 backlog、实时事件、retry/DLQ 与 Sync Projector 收敛；证据仍不等同于候选镜像经 Gateway 的完整消息发送验收。
- 2026-08-29 使用 `SMOKE_MESSAGE_FLOW=1` 通过候选镜像端到端消息验收：经 Gateway 注册/登录、好友关系和 WebSocket 发送后，Message、Outbox 与目标用户 Inbox 均正确落库；重复请求幂等、Kafka authority 和生产回滚仍保持后续门禁。
- 2026-08-29 端到端消息 smoke 增加同一 `client_message_id` 重发，确认候选 Message Service 对 Message、Outbox 和目标 Inbox 保持幂等单条结果；Kafka authority 深度核对和生产回滚继续保持后续门禁。
- 2026-08-29 `ISOLATED_IMAGES=1 scripts/smoke-runtime-dependency-readiness.sh` 通过候选镜像运行时演练：Kafka assignment、Search/Indexer readiness、Elasticsearch 故障降级与恢复及核心服务容器身份稳定性均通过；生产切换与回滚 receipt 仍待完成。
- 运行时依赖 readiness smoke 增加 `ISOLATED_IMAGES=1` 候选镜像模式，复用 Kafka assignment 与 Elasticsearch 故障恢复门禁，避免候选部署只能通过静态配置验证。
- C++ Realtime Delivery 在当前 `master` 基线通过 Ubuntu 24.04 容器门禁：依赖安装、CMake Release 构建和 14/14 CTest 成功，镜像 provenance 标记 `dirty=false`；Go/C++ projection 性能对照仍为 `blocked`，因此继续保留 Go projection 和默认 Go authority。

- Cassandra hydration 与 read-routing smoke 已支持动态宿主机端口并行执行；2026-08-29 两条真实隔离 MySQL 8.4/Cassandra 5.0.9 验证同时通过，覆盖 shadow hydration、重复响应恢复、Legacy ID 恢复、Metadata 回填、Cassandra 页面读取及损坏/缺失行 MySQL fallback。该证据仍不授权生产主读灰度。

- 前端增加 `test:design` Pencil 结构门禁：对 canonical `design/dipole-ui.pen` 校验核心 desktop/mobile frame、设计变量、可复用组件和 placeholder/未命名节点；测试 fixture 同时覆盖缺失画板与占位节点拒绝，保持真实 Pencil CLI 失败时的安全边界。

- C++ Realtime Delivery 增加 `scripts/check-cpp-realtime-container.sh` 容器门禁：复用 Ubuntu 24.04 Dockerfile，自动绑定 revision/created/dirty provenance，在宿主机缺少 gRPC C++ 开发包时仍可复现依赖、编译和 CTest 验证；该门禁不改变 Go 默认投递 authority。

- 2026-08-29 复核平台静态与协议门禁：Go 全量测试、sqlc、Go/TS Proto、Compose、架构文档、Web Sync 观察和 Agent OTel 检查均通过；C++ Realtime Delivery 通过仓库自带 Ubuntu 24.04 构建镜像完成编译与 14/14 CTest。宿主机缺少 `grpc++ >= 1.51` 时应使用 `services/realtime-delivery/Dockerfile` 或显式依赖根目录，不能将宿主机失败误判为源码失败。

- 平台级门禁与 Agent 观测链路复核通过：`scripts/check-go.sh`、`check-sqlc.sh`、`check-compose.sh`、`check-architecture-docs.sh` 和 `check-agent-otel-observability.sh` 全部通过；独立 OTel smoke 验证 trace 经 Collector 写入并可由 Tempo 查询。

- 微服务部署 smoke 在独立 Compose project 和新构建镜像上通过：MySQL、Redis、Kafka、Core、Message、Sync、Gateway、Agent 均 healthy，且 readiness、Prometheus、Core 代理、TLS 1.3 mTLS 和 remote WS ownership 验收通过；脚本 HTTP 探针增加有界重试/超时，失败可回收。

- 修复 Go 根模块递归扫描 `services/agent-runtime/node_modules` 内嵌 Go 源码的问题：新增 TS 服务目录的 Go module boundary 后，`CGO_ENABLED=0 go test ./...` 全仓通过；Agent Runtime 仍单独通过 Vitest、typecheck 和 production build。

- Agent Runtime 独立服务完成全量回归：Vitest 124 个测试文件通过、650 个测试通过，TypeScript typecheck 与生产构建通过；同时 `CGO_ENABLED=0 go test ./internal/...` 全部通过，确认 TS Runtime 的 shadow/协议边界未破坏 Go Core、存储和微服务路径。

- 存储架构隔离 smoke 已通过 Cassandra 5.0.9、Elasticsearch 9.5.2 和 MinIO 的健康检查及 CRUD 验证；Elasticsearch lab 编排显式使用仅测试环境的磁盘水位参数，健康检查要求 yellow/green，生产配置保持不变。

- Go 微服务测试显式绑定版本化 `configs/config.dist.yaml`，修复干净 worktree 缺少隐式 `config.yaml` 导致的测试失败；`CGO_ENABLED=0 go test ./internal/...` 全部通过，生产配置搜索路径未改变。
- 新增 `scripts/smoke-agent-timeline-repair-compose.sh` 部署级隔离演练：先校验 v49 migration、Timeline 表和 MySQL `+00:00/+00:00` 时间基准，再在 repair worker 启动前写入 pending intent，验证 opt-in profile 的 `readyz`、启动恢复、持续 replay 和 event UUID 幂等；使用源码构建镜像后完整通过。演练同时发现并修复 MySQL `Asia/Shanghai` 与 Go UTC lease/retry 的 DATETIME 偏移，将微服务 Compose MySQL 固定为 UTC，并避免把一次性 migration job 交给长期服务等待语义。
- Agent Task Timeline v1 增量设计维护已建立 `design/agent-task-timeline-v1-brief.md`；Pencil CLI `0.3.5` 使用受限增量调用两次均在超时窗口内未完成，safe-edit wrapper 保持 canonical `.pen` 不变且未生成导出图，记录到 `AD-044`，未提前开放视觉基线。
- Agent Task Timeline Vue 组件已接入共享 `--dp-*` Pencil token，统一使用设计基线中的颜色、字体、间距和圆角；组件契约测试会阻止核心时间线样式回退为旧硬编码值。
- Login 页面已接入共享 Pencil token，统一画布、表面、边框、字体、交互色和错误色；新增设计契约测试阻止旧绿色/灰色硬编码回流。
- Agent Task Timeline 路由页面外壳已接入共享画布、字体、间距和文字 token，确保页面级背景与 Timeline 组件使用同一 Pencil 设计基线。
- Agent Event Subscription 管理页已将主题变量、字体、状态色、表面和边框映射到共享 Pencil token，并增加组件设计契约测试，减少管理控制面视觉漂移。
- Agent Memory 管理页已覆盖旧主题变量，统一使用共享 Pencil 画布、表面、字体、状态色和数据字体，并增加设计契约测试。
- Agent Task Timeline 增加 Playwright 浏览器验收：在认证 mock 会话下验证路由开关、Bearer 传递、默认 `limit=50`、稳定 `after` cursor、低敏事件展示和内部 capability ID 不泄露。
- 新增 Compose 静态契约测试，校验 repair profile、镜像二进制、构建脚本和持久化权限依赖；`docker compose -f docker-compose.microservices.yml config --quiet` 在注入 `DIPOLE_INTERNAL_RPC_SHARED_SECRET` 后通过。
- 新增 `conversation.read` gRPC/TypeScript 契约测试：验证 Core 从可信 Task/Run 解析身份、拒绝客户端伪造 principal、映射消息字段，并验证 Runtime 权限缺失时不会发起远程调用；`scripts/check-proto.sh`、`node scripts/check-agent-proto-ts.mjs`、Go 定向测试与 Agent Runtime typecheck/测试通过。
- 新增 Subscription Shadow Collector 响应体边界回归测试，覆盖超过 256 KiB 的 Prometheus 响应 fail-closed；Agent Runtime 定向测试与 typecheck 通过。
- 新增 Context Compiler 会话 evidence 测试：验证消息 provenance、sequence、full/compact 表示和远程读取调用边界；Agent Runtime 全量测试此前通过，当前 Planner/Capability 定向测试 `15 passed`。
- 新增 `scripts/smoke-agent-timeline-repair.sh` 进程级隔离演练：使用临时 MySQL、真实迁移和独立 repair 二进制，验证 repair intent 被 claim/replay、状态收敛为 `completed` 且 Timeline 事件保持单份；worker 由 timeout 有界停止，演练不会启用共享环境服务。
- `smoke-agent-timeline-repair.sh` 真实运行通过：隔离 MySQL、migration、repair process 和幂等事件计数均通过，失败时会保留状态诊断并自动清理临时资源。
- 增加 Timeline repair 的 Prometheus 告警规则和 promtool 测试：区分短窗口失败与持续 projection retry，profile 启用时由 observability 配置按可选服务抓取；默认拓扑和 repair 开关保持关闭。
- 新增正式运维手册 `docs/agent/AGENT-TIMELINE-REPAIR-OPERATIONS.md`：统一记录 repair worker 的前置检查、隔离启用、指标验收、暂停回切和低敏证据归档要求；明确禁止 down migration、手工修改状态和删除 Timeline 事件。
- repair worker 增加显式 `-once` 有界执行模式，常驻轮询与 CronJob/发布验证复用同一 claim/replay/retry 语义；进程级 smoke 已切换为一次性真实执行，减少外部 timeout 对运行结果的干扰。
- 修正 repair 专用 MySQL 密码覆盖边界：`mysql-permissions` 现在使用同一环境变量执行 `ALTER USER`，并拒绝无法安全嵌入初始化 SQL 的单引号/反斜杠，避免 Compose 配置与授权账号密码漂移。

- Gateway/WS 新增 `message.timeline_notify_mode=primary`：与客户端 `VITE_TIMELINE_NOTIFY_MODE=primary` 对齐，继续发送无正文 `sync.item.notify.v1` locator，支持客户端按会话序号向 Cassandra 主读路径补拉；`off|shadow` 行为保持兼容，回切只需恢复原模式。
- Sync Web 客户端支持 `VITE_TIMELINE_NOTIFY_MODE=primary`：收到经过严格校验的 `sync.item.notify.v1` 后，按会话 `message_seq` 串行补拉缺口，只有目标序号和 `message_uuid` 完整匹配才合并消息；事件去重、失败隔离和 shadow/off 兼容行为保持不变。服务端 Cassandra 主读仍需独立灰度证据才能启用。
- Agent Memory 增加 reviewed corpus v1 语言中立 Schema、双 reviewer/第三方 adjudicator 评测器和 `eval:memory-corpus-review` 离线 CLI。语料只保存候选类型、资源范围、证据数量与内容哈希；CLI 仅输出低敏哈希/计数报告，退出码 `0/2/1` 分别表示通过、门禁失败和输入错误，当前仍需真实脱敏语料与人工签署后才可用于灰度。
- Agent Memory reviewed corpus 增加 owner-only source manifest loader：加载前校验绝对规范路径、不可跟随符号链接、regular/single-link 文件、owner 权限、2 MiB 大小、审批有效期及 corpus/review SHA-256；失败不会进入评测或晋级。该 loader 仍只服务离线证据，生产自动写入保持关闭。
- Agent Memory 增加 provider-neutral prefilter evidence v1：embedding/small_model 候选绑定 reviewed corpus SHA-256、revision、configuration SHA-256 和 score/threshold，离线评测输出混淆矩阵、precision/recall、nearest-rank p95、平均/总成本及 fail-closed 门禁原因；新增 `eval:memory-prefilter` 与 policy/evidence/report Schema。该证据不访问模型、Kafka 或数据库，真实语料与在线灰度仍关闭。
- Agent Memory 增加 prefilter rollout decision v1 与 `eval:memory-prefilter-rollout`：重新计算双 reviewer/adjudicator 结果和候选 evidence 门禁，绑定 corpus/review/final-label/evidence 哈希后输出 `eligible|blocked`；该决策仍只作为离线发布前置证据，不开启 Runtime、Kafka 或自动 Memory 写入。
- Agent Memory 增加 provider-neutral Runtime binding v1：`off/shadow/enforced` 三态 gate 精确校验 rollout decision、candidate/configuration/corpus/review 哈希；只有 `enforced + eligible` 才允许后续任务创建，所有模式均固定无 Memory 写权限，默认不接入生产 Runtime。
- Cassandra Message Store 增加 read rollout evidence v1 与 `cassandra-read-rollout-evidence` CLI：绑定服务部署 revision、窗口、配置比例、Cassandra/MySQL/fallback/verification 聚合计数和 p95 延迟，按样本、fallback、核验和延迟门槛输出 `eligible|blocked`。报告不含会话或消息标识，仍只支持离线归档，不改变默认 Cassandra 路由。
- Sync Service 增加 Cassandra hydration evidence v1 与 `sync-cassandra-hydration-evidence` CLI：分别记录 `shadow|primary` 窗口的 Cassandra 命中、MySQL fallback、缺失/冲突/错误和 p95，按统一策略输出低敏 `eligible|blocked`；该证据不改变 `sync.cassandra_primary_hydration` 默认关闭状态。
- Sync Service 为 Cassandra primary/fallback hydration 增加运行时 Prometheus 证据：按低基数 `hit|fallback|error|cancelled` 记录请求计数和耗时，保留原有日志观测接口；指标只在显式构造 Cassandra primary 路径时注册，不改变默认关闭和即时回退行为。
- 新增 `sync-cassandra-hydration-snapshot`：严格解析 Sync Service Prometheus 文本快照，绑定显式服务、revision、模式与时间窗口，聚合路由计数并以有限 histogram 桶保守计算 hit p95，输出既有 hydration evidence v1；缺失命中 histogram、未知 outcome、重复 histogram 或无请求均 fail closed。
- 修正 hydration snapshot 的窗口语义：CLI 现在要求起止 Prometheus 快照，对生命周期累计 counter/histogram 做单调差分；counter reset、histogram 缺失或桶漂移均拒绝生成 evidence，避免把进程累计值误归因到 rollout 窗口。
- 加固 hydration snapshot 完整性：拒绝重复 route outcome、重复 metric family、错误 metric 类型、额外标签、未知 outcome 和非单调 histogram 桶，避免错误或篡改的 Prometheus 文本被聚合成有效证据。

- 前端新增默认关闭的 Agent Task 审批页 `/agent/tasks/:taskId/approval`，通过认证 Task Query 展示 `waiting_approval` 请求，并调用审批决策接口；严格保留过期、不可用和终态的 fail-closed 行为。完整 Run/Step 时间线仍待后端只读契约。
- 新增 `contracts/agent-task-timeline/v1/`：定义 Agent Task 增量时间线的低敏事件、稳定游标、principal 复核和 fail-closed 边界；当前只建立契约，Core/Gateway 聚合 adapter 与前端完整时间线仍关闭并由 `AD-045` 跟踪。
- Agent Task Timeline 增加 migration v48 append-only 事件表及 sqlc append/list repository，使用数据库生成的 `event_seq` 保证 Task 内顺序；状态变更事务接入和 Core/Gateway 聚合 API 仍未开放。
- Agent Policy 生产事务装配已将 Task/Run 创建与状态迁移和 Timeline 事件写入绑定在同一 MySQL 事务中；事件写入失败会回滚对应状态变化，旧兼容构造保持可用。
- Core Agent RPC 新增 owner-scoped `ListAgentTaskTimeline` v1：服务端复核 Task principal，按 `event_seq` 提供低敏增量事件与 `next_cursor`，Timeline 仓储已接入生产 Core 装配；Gateway 代理和前端完整时间线仍待后续阶段。
- Agent Task Timeline 已贯通 Runtime/Gateway 只读链路：Runtime 通过认证控制接口校验 `after/limit` 并调用 Core，Gateway 暴露 `GET /api/v1/agent/tasks/:task_id/timeline`；旧控制实现保持兼容，未提供 Timeline 能力时显式返回不可用。
- 前端新增默认关闭的 Agent Task Timeline 页面 `/agent/tasks/:taskId/timeline`，支持 v1 低敏事件展示、稳定 cursor 分页、空态、失败清空和重试；开关为 `VITE_AGENT_TIMELINE_ENABLED=true`，未开启时继续回到聊天页。
- Agent Tool Invocation 成功开始/结束后追加 Timeline v1 低敏事件，事件 ID 按 invocation 和阶段确定性生成；Timeline 插入支持幂等重放，认证失败与外部 round 未完成不会生成伪事件。
- Agent Approval 请求与决策成功后追加 Timeline v1 事件，按 approval 和阶段生成幂等事件 ID；越权、绑定冲突和失败决策不会追加事件。
- Core 新增受认证的 Agent Timeline append RPC；TypeScript Runtime 的持久化 Model Router 现在为模型调用写入 `model_call` begin/finish 低敏事件，模型输出和 prompt 不进入 Timeline，投影失败不会阻断模型主流程。
- Agent Artifact 创建成功后追加低敏 `artifact` Timeline 事件，事件键绑定 artifact ID 并支持幂等重放；正文、对象存储 URI 和 Metadata 继续留在 Artifact 专属读取链路。
- Agent Task Timeline 增加 MySQL repair ledger v49：Timeline 投影失败时持久化低敏事件意图，按 event UUID 幂等入账，并提供带租约的 claim、完成和 retry 状态操作；当前仅落账，不自动启用修复 worker。
- Agent Task Timeline 增加显式 repairer：批量领取 repair intent 后按原事件重放，成功标记完成，失败按退避重新调度；worker 保持显式构造和关闭默认，便于先进行故障注入与灰度。
- 新增独立运维进程 `cmd/agent-task-timeline-repair`：通过 MySQL transaction store 运行 Timeline repairer，支持批量、租约、退避和轮询参数；主服务默认不自动启动该进程。
- Timeline repair runtime 增加可选 Prometheus collector：暴露有限 outcome 的 repair 计数与耗时，独立进程通过 `--metrics-address` 显式开启，默认不监听指标端口。
- 新增 MySQL repair recovery contract：在真实 MySQL 上创建 Task/Run，注入 Timeline 投影失败，验证 repair intent 进入 retry，随后真实重放收敛为 `completed` 且 Timeline 事件保持单份；测试独立于客户端时区。

### 验证
- Agent Runtime 完成 `125` 个测试文件、`657` 个测试和 TypeScript 构建；Frontend 完成工具链契约、`27` 个测试文件、`102` 个测试及 `vue-tsc`/Vite 生产构建，并同步刷新嵌入式 `internal/server/webapp` 资源。

- Agent Runtime G2 foundation 验收状态与实现对齐：`agent-runtime` 已提供 Node 22、Fastify、Zod、KafkaJS、AI SDK adapter 和独立 shadow consumer；Runtime 核心通过 `ModelRouter` 与具体模型 SDK 解耦。现有测试、类型检查和构建门禁保持通过，真实模型、真实语料和生产写入仍按 G3/G4 独立控制。
- C++ projection microbenchmark 使用同一 `message.direct.created` v1 事件和 100,000 次 JSON 解码/投影迭代，Go/C++ 结果计数一致；C++ 吞吐约为 Go 的 `0.10x`，低于默认晋级门槛，报告为 `blocked`。当前保留 Go projection 作为默认实现并停止 C++ projection 替换，证据归档于 `benchmarks/c2-cpp-projection-benchmark-2026-08-29/`。
- 新增 sqlc `UpsertGroupConversationMessageBatch` 查询及服务层可选批量路径；`CGO_ENABLED=0 go test ./internal/service ./internal/data/mysql/repository`、sqlc 生成和 diff 检查通过，真实 MySQL 8.4 contract 已验证 sender/recipient Seq、未读计算和重复写入幂等。
- C++ Realtime Delivery C3 真实隔离故障演练通过：14/14 C++ build/CTest、5/5 对比测试，以及 Controller 进程替换、Redis outage、Kafka rebalance、过期 freeze 自动回切和 C++ primary 停止恢复均通过；报告绑定当前 Git revision、Redis/Kafka 镜像、C++ 二进制、observation 与 journal 哈希。C++ primary、双 group checkpoint 与自动回切证据已具备，生产灰度和性能收益门槛仍保持关闭。
- Redis Sentinel 三节点真实隔离 smoke 通过：停止当前 master 后约 4 秒完成切换，同一客户端恢复读写与 Pub/Sub，Presence、Hot Group 和限流语义保持可用，旧 master 重新加入为 replica；切主窗口的 Pub/Sub at-most-once 边界仍由 AD-017 跟踪。
- Elasticsearch Search Service 真实隔离契约通过，验证 Core-derived scope、内部 RPC 和 Elasticsearch 9.5.2 查询路径；三节点 Kafka + Elasticsearch Search Indexer smoke 通过，created、recalled tombstone 与迟到 edited 事件最终收敛为 revision 3 且 `searchable=false`。
- Cassandra Message Store 真实隔离读路由 smoke 通过：migration v47、Cassandra Timeline 主读、payload 损坏回退和缺行回退均完成验证；Sync hydration smoke 同时通过 Metadata 回填、重复消息恢复和 Legacy ID 恢复。
- 修正 MySQL migration 集成测试版本基线：迁移已扩展至 v47，测试此前仍按 v44 计算回滚步数，导致 Metadata v12 未被重新执行、回填断言读不到记录；当前按实际最高迁移版本验证回滚和重放。
- 新增 `scripts/pencil-safe-edit.test.mjs`，用 fake Pencil CLI 覆盖有效 `.pen` 与导出原子提交、超时清理临时文件并保持 canonical 不变两条回归路径；测试 `2/2` 通过。
- 在当前 master 基线完成 Agent Runtime 与前端质量验证：Agent Runtime `122` 个测试文件、`627` 个测试通过；Frontend `22` 个测试文件、`87` 个测试通过，`vue-tsc` 与 Vite 生产构建通过。7 个 Agent 测试文件、27 个测试按既定条件跳过。
- Pencil CLI `0.3.5` 认证和版本检查通过；Agent Task Timeline 增量任务在画布调用阶段超时终止，未产生 `.pen` 或导出图，canonical 设计文件保持不变，记录为 `AD-044`。
- 增加 `scripts/pencil-safe-edit.mjs`：Pencil 增量编辑具备默认超时、临时输出、`.pen` JSON 结构校验、导出文件校验和成功后原子替换，失败不会覆盖 canonical 设计。
- Agent Runtime 追加 `npm run typecheck` 与生产 `npm run build` 验证通过；`ModelRouter` 继续通过 `StructuredModelClient` 隔离具体 AI SDK，保持后续 Eino/provider 替换的适配边界。
- Core Agent Timeline RPC 契约测试通过：覆盖 schema/revision/cursor、foreign Task 隐藏和 Timeline 未配置时的 `FailedPrecondition`；Proto Go/TypeScript 生成检查通过。受当前环境配置文件缺失影响，`internal/app` 全包测试仍在既有配置读取处 panic，未归因于本次改动。
- Agent Runtime Timeline control 通过 TypeScript 类型检查和原生 Vitest：`122` 个测试文件、`627` 个测试通过，7 个文件、27 个测试按既定条件跳过；Go Gateway 受当前环境缺失 `configs/config` 影响未能完成全包运行验证。
- 前端 Timeline 验证通过：`24` 个测试文件、`96` 个测试通过，`vue-tsc` 与 Vite 生产构建通过；构建产物已同步到嵌入式 Web 服务目录。
- Agent Tool Timeline 写入专项验证通过：`internal/transport/grpc/agent` 测试通过，`scripts/check-sqlc.sh` 通过，覆盖 begin/finish 事件和重复调用边界。
- Agent Approval Timeline 专项测试通过：覆盖 pending/approved 生命周期事件，继续保留此前 `internal/transport/grpc/agent` 与 sqlc 门禁结果。
- Model Router Timeline sink、Core append RPC 的 Proto/Go 生成、Core Agent 测试和 Runtime 类型检查通过；Model Router 回归测试 `12/12` 通过。
- Artifact Timeline 接入通过 Core Agent 专项编译与测试，保留默认关闭的 Artifact 生产链路和现有授权边界。
- Timeline repair ledger 专项验证通过：sqlc 生成检查、Application/Repository/Agent transport focused tests 通过；真实 repair worker、共享环境故障注入和默认生产开关仍未开启。
- Timeline repairer 单元验证通过：覆盖成功重放完成、投影失败 retry_count 递增和非法配置拒绝；当前仍缺少进程装配、真实 MySQL 故障注入与运行时指标。
- Timeline repair 进程编译验证通过；当前仍需真实数据库故障注入、运行时指标和 operator 灰度记录。
- Timeline repair collector 与运行时专项验证通过：覆盖 outcome 白名单、指标注册、成功重放和失败 retry；真实数据库故障注入与 operator 灰度记录仍未完成。
- MySQL repair recovery contract 通过（`DIPOLE_TEST_MYSQL_ADMIN_DSN`）；完整 repository contract 因既有外部依赖等待未完成，不作为本轮通过证据。

- Agent Runtime 增加 `dipole.agent.memory-promotion-receipt.v1` 与 Temporal preparation Activity：为候选晋级生成不含正文的确定性 receipt，绑定 Task/Run、owner、candidate/review 哈希和最多 15 分钟租约；精确重放可恢复，过期、状态或绑定漂移 fail closed。该 receipt 仍只形成 durable promotion intent，不触发 Core Memory 写入，Temporal worker 与自动晋级保持默认关闭。

- Agent Memory 增加受认证的 `PromoteMemoryCandidate` Core gRPC 与 Gateway HTTP 控制入口：仅允许已认证 Gateway 绑定 owner principal，服务端重新校验候选/审核哈希并返回幂等的 observational Memory；该入口不启动 Temporal、不消费 Runtime 旁路，也不打开自动写入。

### 安全

- Agent Workflow repair 增加受控 `prepared` 准备服务：仅接受已批准、未过期且满足审批门槛的提案，复核 proposal/task/executor 绑定后幂等写入执行意图；当前不推进状态、不修改 projection，也未开放公开执行入口。`executor_grant_version` 仍只作为账本绑定值，待 operator grant 版本化后接入运行时授权复核。
- Agent Workflow repair 增加 migration v44 prepared execution ledger 与 sqlc 访问边界：以唯一 plan 记录执行意图、提案/任务/执行人绑定、CAS 摘要和回滚摘要；当前仅允许 `prepared` 创建/读取，未提供状态推进、apply、execute 或 rollback RPC，现有运行流量不受影响。
- Agent Workflow repair 增加默认无副作用的 `repair:preflight`：在未来执行器前重新核对 plan 摘要、批准提案证据、executor grant 版本和当前投影 CAS，仅输出低敏 `ready|blocked` 收据；过期、漂移和绑定不一致统一阻断，继续不提供 projection 写入。
- 收紧 Agent Workflow repair dry-run 计划绑定：当前投影与目标投影必须属于同一 Workflow/Run，跨任务证据在生成 plan 前 fail closed；不改变 v1 仅 dry-run、无 projection 写入的边界。
- Agent Memory lineage rollout 增加 deployment evidence v1 与只读 `agent-memory-lineage-deployment-evidence` CLI：外部共享环境记录必须绑定 rollout review receipt、运行版本、配置摘要、migration 43、健康检查和回滚演练 ID；输出仅保存 deployment ID 摘要与通过标志，`executionAuthority`、`contentRead`、`deletionAuthority`、`runtimeAuthority` 固定为 false。该工具不连接共享环境、不执行回填，缺少真实部署与回滚记录时继续 fail closed。
- Agent Memory 增加有界历史 lineage backfill v1 的语言中立 manifest/receipt 与 Go runner：游标固定为 Shadow Plan ID high-water mark，引用仅允许 exact `memory:<id>` 与 `full|compact`，runner 在目标成功后推进 checkpoint，重复写返回 duplicate 并保持可收敛。Receipt 仅保存 hash、游标和计数，固定不读取正文、不授予删除或 Runtime 权威；MySQL checkpoint/source/target 尚未接线。
- Agent Memory 增加纯离线派生数据 retention policy 决策：严格覆盖 Model Call、Shadow Plan/Step、Artifact、Tool Invocation、Message Action 与 Temporal potential Task，输入绑定已验证的低敏 lineage report，输出绑定 policy/report/decision 三个 SHA-256。parser 会从 lineage 完整性和受影响的人工复核域重新推导阻断原因；CLI 不连接数据库或网络，固定不读取正文、不执行删除且不授予删除或 Runtime 权威。
- Agent Memory migration v42 将 direct lineage 外键从 Shadow Plan 提前到权威 Agent Task；受管 Model planner 在 Context 编译后、模型调用前写入 `context_pre_model`，失败时模型零调用。Plan-time repair 保持幂等且不能降级来源；只有同时缺少 Plan 与任何 Memory lineage 的 owner 模型结果才进入未归因缺口。
- Agent Memory migration v41 增加保守的派生影响边界：Shadow Plan 事务同步保存 `Memory -> Task` 直接引用，精确重放补齐索引，representation 漂移 fail closed；只读审计按 root 统计模型调用、Plan/Step、Artifact、Tool、Message Action 与 Temporal 潜在 Task。报告只含 root SHA-256、有界计数和完整性标志，固定不读取内容、不授予删除或 Runtime 权威；历史未索引 Context 或已完成模型调用缺少 Plan 的未归因 Task 都会返回 `lineageComplete=false`。
- Agent Memory migration v40 增加 root-wide 内容擦除基础：内部 sqlc/Core 事务锁定完整纠正链，撤销 active 版本，并清除正文、compact、URI、resource binding、root 原始来源与自由文本审计原因；只保留 owner、root/version/predecessor、擦除时间和枚举原因码。语言中立 policy/receipt 明确无自动执行或公开 API 权威，当前没有 Proto、Gateway、Vue 或 retention Worker 入口。
- Agent Memory correction 增加纯离线五类 Eval 门禁：严格 manifest/observation 绑定 predecessor/successor、完整 lineage、精确重放、漂移冲突、owner/foreign 权限与 successor-only retrieval，并要求模型、Tool、Token 和模型成本全部为零；输入各限 64 KiB，错误与标准报告不回显 Memory、principal 或正文，也不连接生产数据库或写入 Memory。
- Agent Memory owner correction 继续沿用认证派生权限：公开请求只提交 Memory ID、期望版本、纠正内容与原因，tenant/principal/corrector 由 Gateway/Core 认证链绑定；响应省略内部 provenance URI，并同时返回权威 predecessor 与 successor，客户端不推断并发结果。
- Agent Memory 增加 owner-scoped 治理边界：公开 HTTP 不接收 tenant/principal，Gateway 从已认证会话构造 Core RequestContext；Core 只允许认证 `dipole-gateway` 调用并再次按 tenant、principal 与 Memory ID 约束查询和撤销。公开 DTO 省略内部 provenance URI，撤销必须提交有界原因并保存 revoker、原因和时间；不同原因的终态重放返回冲突。
- Gin HTTP 访问日志统一脱敏敏感查询参数：WebSocket 兼容的 `token`/`access_token` 及 refresh/id token、Authorization、API key、client secret、密码和签名类键按大小写无关识别，重复值全部替换为 `REDACTED`；非法 query 编码整段关闭，普通参数继续规范化记录。真实 WebSocket 握手日志捕获测试同时证明查询凭据与 Authorization Header 不进入结构化字段。
- Agent Runtime 增加默认跳过的外部 MCP encrypted credential 生命周期演练：临时 owner-only 文件以独立 key/ref/version 完成 v3 到 v4 轮换，Catalog 通过原子 rename 发布并在旧版本与当前版本吊销后于 Transport 构造前拒绝；Runtime 重建继续解析 v4，三次成功 Transport 全部关闭。语言中立 v1 证据与 `mcp:credential-drill:check` 固定三开三关、双吊销、canonical SHA-256、24 小时有效期及 `inflight_revocation_authority=false`、`production_authority=false`，不记录凭据、身份、路径或 endpoint。该离线演练不提供在途 socket 主动撤销或共享 provider authority。
- Agent Runtime 新增默认关闭且独占的 `external_mcp_shadow` Temporal activity mode，并将已验证的外部 MCP Kafka/Temporal process owner 接入 `index.ts`。startup policy 要求 Profile 开关、Temporal、Kafka subscription trigger 与 Capability RPC 同时启用，任一部分配置或 mode 漂移均在 runtime 构造前 fail closed；该 mode 跳过旧 Kafka runtime 与旧 Temporal Worker，防止同 task queue 出现不同 Activity catalog。关闭时统一 process 先停 Kafka admission，再 drain Client/Worker/Core。Compose 继续固定 external MCP disabled + `foundation`。本地隔离演练使用 in-memory Temporal 与现代 MCP Client/Server，通过 17 项恢复/取消/只读 discovery 测试且 `tools/call=0`；真实凭据、DNS/TLS 与公网 evidence 仍未启用。
- 外部 MCP deployment route manifest 增加可选的可信 subscription trigger binding：每条 route 可绑定 exact Agent Definition ID/version 与静态业务参数，加载时同时通过代码 Capability schema、route egress 字段和规范 JSON 字节上限，并拒绝重复 Definition version。trigger binding 纳入 deployment SHA-256，参数或 Definition 漂移会改变 Temporal route history authority；Worker composition 在创建 RPC 前校验全部 mapping 属于 exact route catalog，并冻结快照供受管 Client selector 使用。旧 manifest 仍可省略 mapping，但无法进入后续 production subscription process。`index.ts`、Compose 与外部网络保持关闭。
- Agent Runtime 增加默认关闭的 Kafka + 外部 MCP Temporal Shadow process owner：只接受 subscription trigger，先启动完整 Temporal Worker/Client owner，再启动 Kafka consumer；停止、回滚和取消固定先关闭 Kafka admission，再 drain Client 并停止 Worker/Core resource，任一阶段失败仍继续且重复 stop 复用同一 Promise。Worker RPC resource 将同一个认证 client 作为借用 subscription matcher 沿 startup/lifecycle/process 透传，Kafka dispatcher 不再创建第二条 RPC 连接或分裂身份视图。disabled Kafka/Temporal、matcher 缺失和 Kafka 构造/启动失败均 fail closed。该 owner 尚未接入 `index.ts`、Compose、生产 route registration 或外部网络。
- Agent Runtime 增加默认关闭的外部 MCP Shadow Temporal process owner：按 `Worker bootstrap -> managed Client` 顺序启动并传递同一 AbortSignal、Temporal config 与 exact Worker catalog；disabled Worker 不创建 Client。Client 启动失败或交接后取消会回收 Worker，清理失败固定分类；正常停止固定先 drain/关闭 Client，再停止 Worker/connection/Core resource，任一阶段失败仍继续且重复 stop 复用同一 Promise。该 owner 只公开 exact deployment/worker/config、可信 dispatch 与 stop，尚未接入 Kafka、`index.ts`、Compose、生产 route registration 或外部网络。
- Agent Runtime 增加默认关闭的外部 MCP Temporal Client lifecycle：Worker lifecycle 现冻结并公开实际 Temporal config，Client owner 直接复用同一 address/namespace/task queue 与 exact Workflow route catalog，调用方无法另传漂移配置。disabled Worker 零 selector/连接调用；enabled 时惰性构造代码 route selector 和独立 Client resource，取消/启动失败回滚关闭，错误固定脱敏。stop 先拒绝新 dispatch、等待已接受的 Workflow start 收敛，再幂等关闭连接；Client 不接管 Worker/Core resource。该 owner 尚未接入 `index.ts`、Compose、Kafka runtime 或生产 route registration。
- Agent Runtime 增加受信 subscription-to-MCP route binding：subscription mode 现在把 Core 已校验的 winning subscription ID、definition ID/version、tenant 与 Agent 作为可选 `AgentEvent.subscriptionBinding` 一并投影，并保持旧 direct-target/仅 subscription ID 事件兼容。`TemporalMcpSubscriptionRouteSelector` 仅按代码注册的 exact definition version 选择 route，先复核事件 subscription ID 与运行 tenant/Agent，再调用参数 resolver；空集、重复 definition binding、未知版本、附加 route authority 和非对象参数均 fail closed。生产尚未注册任何 definition route/resolver，也未接入 `index.ts`、Workflow Client、Compose 或网络。
- Agent Runtime 增加默认关闭的可信外部 MCP Shadow dispatcher：先复算事件、tenant 与 Agent 绑定的确定性 Task ID，再从已验证 `AgentEvent`/`AgentIdentity` 固化 admission 与固定 goal；宿主 route selector 只能返回严格的 route ID 和业务参数，无法提交 tenant、principal、Agent、admission 或 goal。selector 输入在调用前冻结，输出拒绝附加字段；专用 Temporal client 继续由 host catalog 注入 route version/manifest digest。该原语尚未接入 `index.ts`、Kafka subscription route mapping、Workflow Client 或 Compose，外部 Worker 与网络仍关闭。
- Agent Runtime 将外部 MCP Worker 收敛为 single-RPC Activity snapshot：Agent Capability RPC resource 现从同一个认证 client 派生 persistent admission/finish/projection/approval Activities，同时继续把该 client 用作 MCP Core 与 Artifact writer；host 提供的 `executeAgentTaskStep` 保持不变。managed startup 在创建资源前用 host base Activities 静态校验，资源后再用冻结的实际 snapshot 执行 composition 与 collision/binding 校验；通用 resource 仍可省略 snapshot。Shadow bootstrap 显式传递 base Activities，未来生产接线无需另建 `temporalRPC`。`index.ts`/Compose 与 Workflow client 仍未修改。
- Agent Runtime 增加默认关闭的外部 MCP Shadow Worker bootstrap：用代码 seal 的只读 definitions 加载 exact deployment snapshot，在 enabled 路径内惰性创建 Agent Capability RPC resource，再交给 managed Worker startup 与 Temporal lifecycle。disabled deployment 只构造 definitions 并读取 Profile 开关，不创建 RPC factory/transport 或 Worker；交接前取消由 bootstrap 关闭 startup，交给 lifecycle 后不再重复回收，构造与清理错误固定脱敏。成功结果保留 exact deployment、Worker composition、Workflow route catalog 与有序 stop。该 root 尚未接入 `index.ts`/Compose，也不创建 Workflow client；若未来启用，外部 egress 仍逐请求要求 fresh readiness evidence。
- Agent Runtime 增加默认关闭的外部 MCP Agent Capability RPC resource factory：仅在 managed startup 请求资源时创建认证 RPC transport，并把同一个 `AgentCapabilityRPCClient` 同时投影为 MCP Core 与 Artifact writer，避免身份或连接视图分裂。factory 在建连前要求 RPC 已启用、deployment 至少一个 Profile 且全部 tenant 与 Shadow config 精确一致；构造后取消会回滚关闭，显式关闭成功或失败均幂等，构造/清理错误固定脱敏。该层不修改 Proto、`index.ts` 或 Compose，外部 Worker 与网络仍未启用。
- Agent Runtime 增加首个代码拥有的外部 MCP 只读 Capability definition `repository.issue.read`：参数严格限制为规范化的 `owner/repo/issue_number`，资源 authority 固定为单个 `repository_issue`，权限与 Capability ID 对齐，egress ceiling 仅允许三个参数且最多 1 KiB，deployment route 只能进一步收窄。Definition Registry 新增显式 seal，并冻结 descriptor/ceiling/resolver snapshot，代码 factory 注册后立即封口，部署或启动层无法追加 write/未知定义。该 factory 不读取环境或 manifest，不创建 RPC/网络资源，仍未接入生产启动链。
- Agent Runtime 增加默认关闭的外部 MCP Temporal Worker lifecycle owner：disabled startup snapshot 不创建 Worker；enabled 时仅用同一 snapshot 的 Activities 启动现有 Temporal Runtime。disabled Temporal config、Runtime factory/启动失败都会回收 startup resource；正常停止严格执行 Worker/Temporal connection shutdown 后再关闭 Core/Artifact resource，任一阶段失败仍继续清理，重复停止复用同一 Promise，并将启动、清理和停止错误固定脱敏。该 owner 尚未接入 `index.ts`/Compose，也不创建真实 RPC 或开启外部网络。
- Agent Runtime 增加受管且默认关闭的外部 MCP Temporal Worker startup plan：以同一 AbortSignal 串行执行 deployment load、无副作用静态 composition validation、受管 Core/Artifact resource factory 与 Worker composition；disabled 或静态无效的 deployment 均不创建 resource。资源创建后的取消、composition 异常或空结果会先执行 rollback close，构造和清理错误固定脱敏；成功计划返回 exact deployment/worker snapshot 与幂等 close，底层关闭成功或失败都只调用一次。该计划不创建 Temporal Worker/Workflow Client，不执行 preflight、DNS 或外部连接，且尚未接入 `index.ts`/Compose。
- Agent Runtime 增加默认关闭的外部 MCP Temporal Worker composition：undefined deployment plan 直接返回且不解析 Core/Artifact 端口；enabled 时先复算 exact Runtime binding、验证全部 route bindings、Capability egress policy 与 Activity 名称唯一性，再惰性构造 multi-route runtime。输出将通用 lifecycle Activities、唯一 `executeMcpDispatch`、匹配的 host Workflow catalog、route bindings 和低敏 Runtime digest 收敛为一个不可变 bundle；Runtime 工厂返回任何 binding 漂移都会拒绝。Temporal Worker 注册类型已允许该 additive Activity，但 `index.ts`、生产 Worker、RPC 创建、Temporal Client、preflight、DNS 与网络仍未接线。
- Agent Runtime 增加版本化 `external_mcp_v1` Temporal Workflow history envelope 与专用启动 client：调用方只选择宿主 catalog 中的 route ID 并提交 16 KiB 内 JSON 业务参数，route version/manifest digest 由部署计划返回的 binding 注入，Profile/Server/Tool 不进入启动 API。Workflow 有 envelope 时调用唯一 `executeMcpDispatch`，首次从持久 admission 派生 Task/Run/principal/correlation，恢复时只传 durable checkpoint 与已验证用户输入；普通 goal/event 路径继续使用 `executeAgentTaskStep`。真实 Temporal 测试覆盖 Worker 替换与 Elicitation resume。生产 Worker、`index.ts`、Compose 和网络仍未注册该 Activity。
- Agent Runtime 增加默认关闭的外部 MCP multi-route Temporal Activity dispatcher：部署计划中的每条 route 只构造一次 route-scoped runtime，统一的 `executeMcpDispatch` 入口仅从 begin 输入或 durable checkpoint 提取 route ID，再把完整 payload 交回对应 Activity 校验 route version、manifest/deployment digest、Context 与 checkpoint。空路由、重复或未知 route 以及残缺 routing payload 均在 Core/Transport 前 fail closed；调用取消和 route-local 错误保持原样传播。该 dispatcher 已由 `external_mcp_v1` Workflow 分支引用，但未注册到生产 Temporal Worker、`index.ts` 或 Compose，也不启动网络。
- Agent Runtime 增加默认关闭的外部 MCP deployment composition plan：先固定 owner/取消边界并加载一次 Profile、production I/O manifest 与 deployment route manifest，全部交叉校验成功后才构造无文件依赖读取、无 DNS/socket 的 production runtime。计划将同一 config、I/O、raw Registry 和 readiness binding options 同时提供给 readiness collector 与 gated Worker，并公开可复核的低敏 Runtime binding；输出不含 RPC、Temporal Worker、start/stop、自动 preflight/drill 或网络启动。disabled 模式不读取 Profile/manifest 残留配置，任一 manifest 漂移或取消均不返回部分计划；`index.ts` 与 Compose 仍未接线。
- Agent Runtime 增加 credential-free 外部 MCP deployment route manifest v1：安全 loader 仅在 enabled Profile 下读取 owner-only、canonical parent、`O_NOFOLLOW`、regular/single-link 的有界 UTF-8 JSON；manifest 固定 route/version、Capability、Workflow step/ordinal、Profile/Server/Tool 与 egress policy，并与代码定义的 schema/resource resolver/egress ceiling 及 Profile allowlist 精确 join。重复 route、Capability 或 Workflow 坐标、Tool/Server 漂移及 policy 扩权统一 fail closed。完整部署绑定摘要进入 Temporal route history/checkpoint，漏升版本的 Profile/Tool/egress 漂移也会在 Core 前拒绝。该 loader 未接入 `index.ts`、Temporal Worker 或网络启动。
- Agent Runtime 增加外部 MCP fresh-readiness egress gate：MCP Worker 组合必须提供 host-owned Profile、production I/O 与 raw Registry，Runtime 在每次 `connect` 前自行派生 exact Profile/Runtime binding，并通过 Core 服务端时钟解析 fresh evidence；空结果、回执漂移、underlying Profile 漂移、取消或解析异常均在 Catalog/Transport/网络访问前 fail closed。回执不缓存，调用方无法提交 binding 或查询时间；readiness 采集继续使用独立 raw Registry，避免以现有 evidence 授权自身采集。该组合仍未注册到 `index.ts`、Compose 或自动调度，也不授予 Run admission、Runtime promotion、Profile activation 或外部网络启动权限。
- Agent Capability 增加 additive `ResolveFreshMcpReadinessEvidence` 只读 RPC：只允许 transport 认证且无 principal 的 `dipole-agent`，请求仅含 tenant 与 exact Profile/Runtime binding，查询时间由 Core 服务端时钟确定。应用 Resolver 通过 sqlc freshness 查询后重新验证 canonical evidence、确定性身份、双 binding、非未来采集时间与严格 expiry；响应只返回 `found`、摘要和时间，不包含 evidence JSON、operator 或 request/trace。TS client 对空响应和低敏收据逐字段复核；该 RPC 不读取或改变 admission、activation、promotion 状态。
- Agent Runtime 增加显式单次 `mcp:readiness:publish` 运维入口：命令必须提供 tenant、exact Profile、60 至 3600 秒有效期及 request/trace，先通过安全 manifest 构造 production I/O runtime，串行执行 local preflight 与只读 Shadow Tool discovery，再调用一次 Core Publisher。参数、采集、清理或响应校验失败均 fail closed 且不会发布；收据只含绑定摘要、时间与 created 标志。该入口未接入常驻 `index.ts`、Compose、自动调度、重试或 admission，执行命令是唯一触发外部 DNS/TLS/MCP discovery 的方式。
- Agent Capability 增加 additive `PublishMcpReadinessEvidence` RPC：仅接受 transport 认证且无用户 principal 的 `dipole-agent`，operator 由服务端绑定为认证 service identity，request/trace 从已验证 RequestContext 派生；请求只能提交 tenant、Profile binding、v2 evidence JSON 与 expiry，无法注入状态或 activation。Core 严格解析 16 KiB 内 evidence 后经 migration v37 Publisher 追加；TS client 复算 canonical content hash 与确定性 Evidence ID，并拒绝响应绑定漂移。RPC 已装配但没有自动采集、调度或 admission 调用。
- Agent Core 增加外部 MCP readiness evidence 追加式归档：保留 evidence v1 契约，新增 v2 exact Profile binding；migration v37 建立无 Task/Run 假 lineage、无更新列和无 activation 副作用的独立控制面表。Go Publisher 严格拒绝额外/敏感 JSON 字段，复算 canonical content SHA-256，并用 tenant、Profile/Runtime binding、operator、request/trace 和最长一小时有效期派生确定性 Evidence ID。exact replay 幂等，收据或 binding 漂移保留新历史；fresh 查询同时约束 tenant、双 binding 与 expiry。自动采集调度、admission、签名 attestation 和真实公网 Shadow 归档仍未启用。
- Agent Runtime 增加外部 MCP readiness evidence v1：production runtime 将同一次 local preflight 与 exact Profile 的只读 Shadow drill 收敛到最长 5 分钟窗口，并以 canonical SHA-256 绑定全部排序后 Profile policy、credential/key/secret/CA opaque ref、文件路径拓扑和生效上限。归档 bundle 只含 binding hash、四个时间、Profile/Credential/CA/Tool 计数，语言中立 schema 禁止附加字段；配置、路径或上限漂移会改变摘要，收据时间/计数漂移、依赖失败和清理失败统一拒绝。注入自定义 Transport builder 的测试组合不能生成 evidence。摘要不包含 secret/key/certificate bytes，也没有签名 attestation；`index.ts`、自动 admission 与真实公网 Shadow 归档仍未启用。
- Agent Runtime 增加默认关闭的外部 MCP Shadow connectivity drill：production runtime 只新增受约束演练函数，不公开 DNS/TLS/Secret adapters。调用方必须显式给出 exact Profile/tenant；演练经正式 Registry 完成 Catalog、Secret、public-only DNS、pinned TLS 和 modern MCP discover/list，要求全部 allowlisted Tool 可见后立即关闭 Client，且没有 `tools/call` 表面。成功收据只含 schema、时间和 Tool 数量，Profile、tenant、Server、Tool 与网络细节不落收据；失败与清理异常固定脱敏，取消传播。该函数未注册到 `index.ts` 或自动启动，真实公网证据仍需在隔离只读 Shadow tenant 中归档。
- Agent Runtime 增加外部 MCP production I/O readiness preflight：`createExternalMcpProductionIoRuntime` 只公开 tenant-bound Registry 与预检函数，使用同一固定逻辑时间解析全部 enabled Profile，并对 exact Credential binding 与 CA ref 去重后验证 Catalog 状态、AES-GCM/AAD/key、正式 Bearer 语法及 PEM/X509 文件。成功收据只含 schema、开关、时间和聚合计数；吊销、身份漂移、key/envelope/CA 损坏统一低敏失败。预检不创建 Transport、DNS client 或 socket，统一的 `maximum_secret_bytes` 同时约束 Provider、AuthProvider 与预检。`index.ts` 和外部网络仍未启用，真实连通性由后续隔离 Shadow 演练验证。
- Agent Runtime 增加外部 MCP production I/O manifest v1 与 default-off 安全 loader：语言中立 schema 只允许 Catalog、key/secret/CA opaque ref 的规范绝对路径及有界参数，运行时进一步要求引用和全部路径唯一、secret key 关联存在。主开关关闭时不读取残留 manifest 环境变量；开启后仅接受 owner-only、canonical parent、`O_NOFOLLOW`、regular/single-link 的 UTF-8 JSON 文件，每次加载重新读取且失败不回退旧快照，错误固定脱敏。loader 输出可直接构造 production Registry，但尚未注册到 `index.ts`。
- Agent Runtime 增加默认关闭的外部 MCP production I/O composition：`createExternalMcpProductionIoRegistry` 统一拥有文件 Credential Catalog、AES-GCM Secret Provider、request-local DNS、文件 CA、pinned TLS Dispatcher 与 Streamable HTTP Factory，只公开 tenant-bound Transport Registry。disabled 路径不触达残留 I/O 配置；enabled 构造仅验证映射，不读取文件、查询 DNS 或建连。Catalog 每次 connect 重载，Secret 每次 token 重载；revoke 在 Transport 前阻断，provider identity 漂移在 DNS/TLS 前以固定错误失败。`index.ts` 尚未注册该组合，外部网络开关继续关闭。
- Agent Runtime 增加默认关闭的外部 MCP AES-256-GCM 文件 Secret Provider：Catalog 的 exact credential binding 与版本化 `key_ref` 一起进入 AAD，密文篡改、错 key、tenant/ref/version/provider 漂移和旧映射移除均 fail closed。Provider 每次请求重新打开 32 字节私有 key 与有界二进制 envelope，要求 canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode，并在成功外的路径擦除 key/plaintext buffer；错误固定脱敏。该本地 envelope 适配器尚未装配到 `index.ts`，不等同于独立 KMS 托管，外部网络开关继续关闭。
- Agent Runtime 增加默认关闭的外部 MCP pinned TLS Dispatcher 与文件 CA provider：每次请求重新读取并验证 certificate-only CA bundle，通过自定义 lookup 只连接 Network Guard 批准的 DNS answer，禁用代理和连接复用，并同时校验 TLS chain、ServerName 与实际 remote peer。请求/响应 body 保持流式传输，TLS connect timeout 与取消会销毁独立 socket，3xx 原样返回给上层拒绝；CA 文件要求 canonical 安全父目录、`O_NOFOLLOW`、regular/single-link、owner/mode 与大小上限。适配器尚未注册到 `index.ts` 或 Worker startup，加密 Secret backend 与外部网络开关继续关闭。
- Agent Runtime 增加外部 MCP production DNS Resolver：每次 guarded fetch 创建独立 `node:dns/promises Resolver`，并行获取完整 A/AAAA 集合且不缓存；单族 `ENODATA/ENOTFOUND` 可由另一族补足，任何瞬时失败或畸形 family/TTL 证据均整体 fail closed。AbortSignal 只取消当前 Resolver，预取消不会创建 DNS client，上游错误统一为低敏消息。Resolver 尚未接入启动链，pinned TLS Dispatcher、加密 Secret backend 和外部网络开关继续关闭。
- Agent Runtime 增加单一 default-off Temporal MCP Runtime factory：由同一 Capability Route Registry 同时构造 stable Invocation producer 与 route-scoped Worker egress policy，再组合 fresh Core Context resolver、receipt-derived terminal Worker 和 untrusted Artifact projector。调用方不再重复提交 Profile/Tool egress map，factory 只返回版本化 route binding 与专用 Activity；端到端测试覆盖 completion-loss receipt/Artifact 重放、durable input 双轮恢复和预取消。生产 Worker、`index.ts`、Activity mode 与外部网络启动仍未接线。
- Temporal MCP dispatch route 增加完整 manifest 摘要：`temporalMcpDispatchRouteBinding` 对 route ID/version、Capability、Workflow step 与 ordinal 生成 canonical SHA-256，受信调度输入及 durable checkpoint 均持久化该摘要。替换 Worker 即使遗漏 route version 提升，只要 Capability 或 Step 坐标发生变化，也会在 Core context/producer 前 fail closed，避免 Activity completion 丢失后派生第二个 Invocation。摘要仍须来自 host-owned 静态 route manifest，模型、goal、事件和客户端无权提供。
- Agent Runtime 增加默认关闭的外部 MCP Artifact projector：完成路径只接受 ExecutionContext、Invocation/Round ID 与 terminal Worker 结果，随后从 Core 重新解析 completed Tool command，逐项核对 tenant/principal/Agent/Task/Run 及 Profile/Server/Tool/Capability。结果经 MCP 标准 schema、canonical JSON 和 128 KiB 上限验证后写入 Invocation-derived type 的 content-addressed Artifact，metadata 标记 `trust=untrusted` 并保存低敏 lineage；重放及 Artifact 已提交后的取消均收敛到同一收据。当前仅允许 running shadow Run，active policy、生产路由和启动接线仍关闭。
- Agent Runtime 增加独立且默认关闭的 Temporal MCP dispatch Activity：Workflow 历史固定 host-owned route ID/version 与业务参数，Activity 每次 begin/retry/resume 都从 Core 重新解析 ExecutionContext、精确重放稳定 Invocation producer，并仅把 Task/Run/Invocation 三个 ID 交给 terminal Worker。等待点以完整性 checkpoint 绑定路由、Step 坐标和 Worker 状态；完成后先把不可信外部结果投影为幂等 Artifact，Workflow 只接收 Invocation/Round/Artifact 收据。取消会在 Core、producer、Worker 和投影边界之间 fail closed；生产 Worker、启动入口、Activity mode 与外部网络仍未接线。
- Agent Core 增加 receipt-derived MCP Invocation 终态所有者：additive RPC 只接收 Task/Run/Invocation/Round 四个持久 ID，Core 重新加载并核对 read-risk Invocation 与 terminal Round，服务端派生结果摘要、字节数、错误码和首次延迟；完成后的重试直接核对已存终态，不由 Runtime 重传易漂移证据。旧 Finish RPC 拒绝带 Profile 的外部 Invocation，避免出现第二终态权威。默认关闭的 terminal Worker composition 在成功或稳定失败 Round 后调用新 RPC，`input_required`、`executing`、绑定漂移和 write Capability 均 fail closed；现有启动入口、Temporal 调度与外部网络仍未接线。
- Agent Runtime 增加默认关闭的受信外部 MCP Invocation producer：strict 输入仅接受 Workflow step/ordinal、注册 Capability ID、业务参数和可选 Approval；tenant/principal/Agent/Task/Run 来自 ExecutionContext，Profile/Server/Tool 来自注入式 Capability 路由。稳定 64-hex Invocation ID 只绑定 host-owned Workflow 坐标，参数或路由漂移复用同一 ID并由 Core exact begin 拒绝，避免配置变化生成第二个执行意图。producer 复用 PolicyEngine、schema 与 MCP egress guard，在 Core begin 前阻断越权资源、未声明参数和敏感字段。
- Agent Core 为外部 MCP Tool Invocation 增加精确 begin/finish 重放与终态 receipt 恢复：重复 begin 仅在全部权威身份、Capability、Profile/Server、canonical 参数、请求关联和审批绑定一致时返回原记录；重复 finish 逐项核对状态、结果摘要/大小/延迟、错误码与 action reference。`ResolveMcpToolCommand` additive 返回 Invocation 状态，terminal Invocation 的 round claim 只能读取并重放已存在 receipt，缺失或漂移时 fail closed，禁止创建新 round 或重新发网。
- Agent Runtime 增加默认关闭的 MCP Worker Runtime 组合器：将认证 Core command resolver/round receipt、tenant Profile Transport Registry、allowlisted modern Client、Activity-safe continuation 与三 ID dispatcher 组装为单一依赖注入边界。已取消请求会在 Core resolve/receipt claim 前停止；本地完成后替换 Runtime 只回放 canonical receipt，`ambiguous` 不创建 Client/Transport。Temporal 自动调度仍等待受信 Agent Step 创建持久 Invocation，不从 goal、模型输出或事件正文选取命令。
- Agent Runtime 增加外部 MCP Streamable HTTP Transport Factory：精确复核 Profile 与 Catalog 的 tenant/ref/version 绑定，为每次连接创建独立 AuthProvider、Network Guard 和官方 SDK Transport；每个请求重新读取 Bearer、重新解析全部公共 DNS 地址并核对 pinned peer，同时关闭 401 自动刷新、403 扩权和 SSE 自动重连。生产 Secret Provider、DNS Resolver、TLS pinned Dispatcher 与专用 Worker 仍未装配，外部连接开关继续 fail closed。
- Agent Runtime 增加默认关闭的 MCP Worker command dispatcher：初始输入严格只接受 Task/Run/Invocation ID，Profile、Server、Tool、Capability、参数和开始时间每次从 Core 持久 Tool Invocation 解析；稳定 request ID 与输入截止时间由 Invocation 派生，恢复前重新核对完整命令和 Activity checkpoint。连接 Session Factory 仅接收 tenant/profile/server/tool 四字段，不再可见 Task、Run、Invocation 或参数。生产 Worker 与外部网络开关继续关闭。
- migration v36、Core/TS RPC 与 MCP Activity 增加 durable Tool round receipt：确定性 Round ID 绑定 Invocation、轮次和请求摘要，MySQL 原子认领后仅原 owner 可一次写入规范结果或稳定失败；Temporal 丢失 Activity completion 时重放已存结果，遗留 `executing` 明确返回 `ambiguous` 并禁止自动 reclaim/retry。外部 Provider、网络开关和 Worker composition 继续关闭；远端已执行但本地收据尚未提交的窗口采用 at-most-once 失败策略。
- Agent Runtime 增加默认关闭的 MCP `2026-07-28` 手工 `input_required` 续接基础：仅接受一个普通 Form，持久 checkpoint 精确绑定原 Tool 参数、请求键、不透明 `requestState` 与 Server/Tool/Invocation lineage，恢复时由新请求回传同一状态；凭据字段、状态漂移、多请求和超限载荷均 fail closed。生产 Activity 编排、外部连接、多轮与敏感 URL 授权仍未启用。
- Agent Runtime 增加默认关闭的 MCP Activity-safe round runner：每个首次/恢复轮次均从 tenant-owned Profile 打开全新现代 Client/Transport，并在成功、取消或握手失败后关闭资源；外层 checkpoint 绑定 tenant/profile/server 与内层 continuation，credential 仅在 Registry `connect` 时解析。现有 Worker 模式与外部连接开关保持关闭。
- migration v35 与 Core/TS RPC 增加持久外部 MCP Tool command：`BeginMcpToolInvocation` 可按 all-or-none 保存 Profile、Server 和 16 KiB canonical 参数，Core 重算 SHA-256 并递归拒绝凭据字段；`ResolveMcpToolCommand` 只向认证 Agent Runtime 返回同一 running Task/Run/Invocation 的权威命令。Worker 切流继续等待外部 Tool round receipt 对模糊执行窗口的治理。
- Agent Elicitation Task Query 新增受限来源绑定：本地请求标记为 Agent，MCP 请求固定携带不可信的 Server/Tool/Invocation 元数据；Runtime 与 Web 双重拒绝凭据类字段，查询失败时 Web 清空缓存 Form 并隐藏上游错误，避免向失效 request 提交数据。
- 修复内部开发证书生成脚本的权限覆盖顺序：公开证书保持 `0644`，CA 与服务私钥最终固定为 `0600`；新增临时目录回归测试，防止后续演练或本地部署生成可被其他用户读取的私钥。
- 修复 C3 首次冻结直接回切的节点证据缺口：`rollback_requested` 现在必须先对当前 frozen lease 使用 source-node manifest 生成独立 `rollback_frozen_confirmed` artifact，再允许 source activation；覆盖 freeze 后尚未完成 target-node confirmation 即超时的路径，避免复用缺失或面向候选节点的旧 proof。

### 新增

- Context Compiler v2 增加可选 `maxInputTokens` 窗口门禁；route-aware runtime 按最小候选模型窗口扣除最大输出预算，超出时在编译前 fail closed，v1/旧构造保持兼容。
- Agent Memory Observation/Reflection worker 将幂等键扩展为 tenant、principal、Agent、资源与事件/窗口的完整 scope，避免多租户或多资源复用 ID 时错误丢弃候选；新增跨 scope 回归测试。
- Agent Memory 增加 v47 Core-owned accepted candidate promotion seam：服务端重新加载并校验候选、owner review、exact hash、范围与 30 天证据窗口，在同一 sqlc/MySQL 事务中创建摘要型 observational Memory 并记录 promotion receipt；重复 promote 可恢复同一 Memory，漂移与缺失均回滚。当前没有公开 Runtime 旁路或自动写入开关。
- Agent Memory 增加 v46 append-only candidate review ledger：`accepted|rejected` 审核绑定候选哈希、reviewer、有限理由、时间和 review hash，候选状态与审核记录在同一事务中更新；精确重放返回 duplicate，哈希漂移、候选缺失和重复决策冲突均回滚。该阶段仍不将候选投影到 `agent_memories`。
- Agent Memory 增加 v45 candidate ledger：持久化 Observation/Reflection 候选的摘要、来源/证据 ID、策略版本、规范 SHA-256 和待审状态；候选唯一 ID 重放时执行哈希冲突校验，完整对话正文不会写入 ledger，且不会自动投影到 `agent_memories`。Migration 可回滚，后续 accepted 投影仍需 reviewer、策略和 durable receipt 门禁。
- Agent Runtime 增加默认 shadow-only 的 Observation/Reflection Memory worker：按事件生成有界、确定性、可去重的 `observational` candidate，再按唯一 evidence window 聚合 reflection candidate；输入超限或凭据模式 fail closed，候选不自动写入 Memory、不调用模型或外部系统。详见 `docs/agent/agent-memory-observation.md`。
- Agent Workflow repair 增加 `repair:plan` dry-run 执行计划编译器：仅接受已批准提案、双人审批、独立 executor grant 和重新采集的当前/目标/回滚投影，生成带三组 CAS SHA-256、15 分钟有效期和确定性 plan ID 的语言中立 v1 计划。计划生成不连接 MySQL/Temporal、不提供 apply/execute/rollback 字段，身份复用、回滚证据漂移和窗口外重放均 fail closed。
- 增加默认关闭的 `realtime-cpp` Compose profile：显式配置 `cpp` authority、Primary RPC、Redis fencing epoch 和维护窗口后，才会启动独立 C++ Realtime Delivery；默认 Compose 继续使用 Go，profile 未启用时不创建 C++ 服务。C++ 进程通过 Kafka primary group、Redis authority 和 Gateway mTLS node transport 工作，回滚恢复 Go 配置并移除 profile。
- 增加语言中立 `dipole.agent.memory-derived-lineage` v1 manifest/report、严格 Zod 解析器和 `audit:memory-derived` CLI。owner 授权 manifest 保持本地敏感输入，标准输出省略 tenant、principal、Memory ID 与全部正文；MySQL 审计账号仅新增 Memory 与 lineage 两张表的只读权限。
- Agent Memory 增加 append-only owner correction：migration v39 为每条记录保存 root/version/predecessor/corrector/reason，唯一 predecessor 与 `(tenant, root, version)` 约束阻止分叉；sqlc transaction 在同一事务中撤销前序版本并追加 successor，稳定 correction ID 支持精确重放，payload 或期望版本漂移返回冲突。additive gRPC、Gateway 与 Vue 已形成完整闭环，`VITE_AGENT_MEMORY_CORRECTION_ENABLED=false` 默认关闭纠正入口，Pencil 文件维护 desktop/mobile 与六类纠正状态。
- Agent G3 增加默认关闭的 Memory owner 管理闭环：migration v38 与 sqlc 提供稳定 cursor 分页、owner 隔离的 authoritative get/revoke 和完整撤销审计；additive Agent gRPC 由 Gateway-only 控制面调用，公开 list/revoke API 与 canonical Pencil desktop/mobile 设计、Vue 页面覆盖 loading、ready、empty、inactive、expired、unavailable、revoking 和 conflict 状态。长期 Memory 始终显示 `UNTRUSTED MEMORY`、owner provenance 与自动写入关闭状态；纠正入口等待 append-only 版本模型后再开放。
- 增加语言中立 `dipole.agent.subscription-shadow-collection.v1` 与只读 Prometheus Collector：从无凭据 origin 执行固定 19 次历史查询，要求单 Agent series、全窗口 Shadow enabled、vector 单值和非负安全整数，自动生成 evidence v1 所需的起止 counter、抓取覆盖与 reset 输入；Collector 不修改共享状态、不输出 Prometheus URL，也明确保留部署 revision 的发布记录核验门槛。
- 增加语言中立 `dipole.agent.subscription-shadow-evidence.v1` 与独立 CLI：Prometheus 起止快照绑定 24 小时以上窗口、Runtime/config SHA-256、query revision、抓取覆盖率、六类 comparison、candidate 和 counter resets；至少 95% 抓取、100 个事件、零 reset、零 matcher error 才生成最多有效 24 小时的 canonical-hashed passing evidence。输入/证据 Schema 均拒绝附加字段，收据固定 `production_authority=false` 与 `runtime_change_authority=false`，Runtime 启动链不读取该文件。
- 增加默认关闭的 Agent Subscription 在线 Shadow 对照：`direct_target` Kafka handler 在 EventLedger 前调用同一 Core matcher，只记录固定 `accepted|ignored × match|miss|error` 矩阵和候选总数；matcher 异常不阻断主路径，且不会创建第二个 Task、Workflow 或模型调用。Agent `/metrics` 暴露低敏零值/开关状态，Prometheus 新增 matcher error 与 admission drift 告警；Compose 固定关闭，启用与回滚见 `docs/agent/agent-subscription-shadow.md`。
- 增加默认关闭的 Agent Event Subscription owner create 闭环：Core additive RPC 将 authenticated readable conversation 与精确 Definition scope 求交集并派生 direct/group event type；Gateway 只接受 Definition、conversation 与确定性 filter，从 JWT/配置派生 principal/tenant，并在创建时重新派生 resource。Vue 从 active Definition 目录和权威候选中选择绑定，关键词超限显式阻止提交，成功后以 Core 结果更新列表；canonical Pencil 增加 desktop/mobile 创建稿、七类状态和两个复用组件。Runtime 与 Compose 继续固定 `direct_target`。
- 增加默认关闭的 Agent Definition catalog：sqlc 按 tenant/owner、服务端有效期、`conversation.read` 与可读 conversation scope 筛选 active 版本，应用层二次复核 authority；additive Core RPC 与 Gateway `/api/v1/agent/definitions` 从认证 principal 派生 owner，并以 opaque 复合 cursor 分页。公开投影不含原始 permissions、owner 或非 conversation scope；前端严格 API parser 已就绪，conversation chooser、create UI 与 Runtime 切换继续关闭。
- 增加默认关闭的 Agent Event Subscription owner list/revoke Web 闭环：Gateway 复用现有 Core Agent gRPC 连接，从认证会话派生 principal 并固定 tenant；Vue 以严格响应解析展示 Definition version、conversation scope、确定性 filter 与撤销审计，查询失败清空旧状态，撤销要求精确原因并以权威响应收敛。desktop/mobile 路由已通过 Chromium、Firefox 与 WebKit 验收。该阶段公开 Definition 目录与 create 尚未交付，现由同一 `Unreleased` 中的 owner create 闭环补齐；Runtime `subscription` 继续关闭。
- canonical Pencil 增加 Agent Event Subscription v1：desktop owner 管理页、`loading|empty|unavailable|definition_stale|revoking|revoked` 六态矩阵、mobile 精确撤销确认层及三类可复用组件；设计固定披露 Definition version、conversation scope、确定性 filter、审计原因与 `direct_target` Shadow 边界，2x 评审图归档于 `design/exports/agent-subscription-v1/`。owner list/revoke 已由默认关闭的 Gateway HTTP/Vue 管理页实现；Definition 目录和 create 设计由后续同节条目补齐，Runtime `subscription` 模式仍未启用。
- Agent G4 全栈 Shadow 演练增加测试专用 Go Core RPC fixture：复用生产 TLS 1.3、双向证书验证、shared-secret metadata、caller allowlist 和证书 CN 绑定；脚本生成临时 `dipole-core`/`dipole-agent` 身份并验证错误 secret、错误 CN 与无客户端证书均失败关闭。
- 外部 MCP Shadow 演练证据升级为 v2，新增 `core_rpc_type=go_internal_grpc_mtls`、认证成功和身份拒绝验证门禁；v1 Schema 保留用于解释历史文件，当前 CLI 只接受带完整 Core RPC 证据的 v2。
- Agent G4 隔离全栈 Shadow 证据增加语言中立 JSON Schema、严格 Zod 解析器和 `mcp:shadow-drill:check` CLI；契约固定成功计数/布尔门禁、canonical `content_sha256` 与最多 24 小时有效期，并拒绝额外字段、未同步 hash 的内容漂移、未来或过期文件。
- Agent G4 增加默认跳过的隔离全栈 Shadow 演练：随机 Compose 项目启动独立 MySQL 8.4 与 Kafka 3.9，测试进程启动内存型 Temporal、owner-only route manifest、可信 Core 夹具和本地只读 MCP Server；演练输出 owner-only、无标识符/正文/凭据的 v1 JSON 证据并自动清理容器、网络和卷。
- 增加默认关闭的 Agent Elicitation Web 闭环：`/agent/tasks/:taskId/input` 根据 authenticated Task Query 渲染 `text|select|multiselect|boolean`，精确绑定 Task/request 提交并在提交、取消或过期后重新查询权威状态；desktop/mobile 响应式页面与 3 项组件行为测试已接入，入口由 `VITE_AGENT_ELICITATION_ENABLED=false|true` 控制。
- Agent Elicitation Web 补齐浏览器验收与可访问性语义：Chromium、Firefox、WebKit 覆盖 authenticated Task/request 精确绑定、外部 MCP 来源披露、首次查询失败后的 stale Form 清理与恢复、校验失败聚焦首个无效字段，以及 390x844 单列布局；Form 同步暴露 busy、alert、`aria-invalid` 和错误描述关系，敏感字段与 URL mode 继续关闭。
- canonical Pencil 增加 Agent Elicitation v1：desktop/mobile 普通 Form、外部来源披露、四类受限字段、七态矩阵及三类可复用组件；设计明确旧 request、敏感字段、URL mode 和依赖不可用时 fail closed，2x 评审图归档于 `design/exports/agent-elicitation-v1/`。
- canonical Pencil 增加 Agent Workflow Repair v1：desktop evidence review、`proposed|approved|rejected|expired|unavailable` 六态矩阵、mobile 双人审批层及三类可复用组件；界面明确批准只形成审计结论，不执行 projection repair，2x 评审图归档于 `design/exports/agent-repair-v1/`。
- 增加 canonical Pencil 前端设计，覆盖 foundations、可复用 IM 组件、Login/Chat desktop/mobile 与关键异常状态，并保存 2x 评审导出图。
- C3 增加持续 cutover controller 并关闭 `AD-041`：`dipole-realtime-cutover -operation run` 在一个同步循环中统一拥有状态推进、冻结超时回切、阻塞重试与临近到期的 authority lease 续期。Redis attempt-scoped ownership 通过 owner token 的 acquire/renew/release Lua 比较阻止并发 controller；control lease 至少覆盖两倍 action timeout。当前 authority deadline 只从 initial input 或最新 journal-bound transition artifact 恢复，拒绝采用未入 journal 的孤儿 transition；`rollback_requested` 续租保留回切意图和二次冻结要求。隔离 Docker Redis + race 演练证明 Controller A 无 release 退出后，B 在 5 秒 TTL 前被阻断、到期后从 sequence 1 继续到 completed sequence 6，证据归档于 `benchmarks/c3-cutover-controller-2026-08-28/`。
- C3 隔离故障演练接入真实 C++ Primary authority：演练改用 canonical message topics 与 `dipole-realtime-primary-*` group，目标激活后停止 Go Primary 夹具并启动当前源树构建的 `dipole-realtime-delivery primary`。目标 checkpoint 的 `services/realtime-delivery/cpp-a` observation 禁止由测试夹具代写，必须由 C++ 进程在校验 CPP active lease 后写入 Redis，并同时通过真实 librdkafka assignment 与 `/readyz`；报告绑定 C++ 二进制、observation payload、consumer group 和 journal 的 SHA-256。持续 controller 所有权仍由 `AD-041` 跟踪。
- C3 增加 production cutover executor：启动时校验 attempt 对初始 lease、三阶段节点清单和双组 checkpoint 清单的 SHA-256 绑定；每个动作固定执行 `artifact lookup -> Redis receipt recovery -> new side effect`，只有明确缺失 receipt 才允许新的 CAS。source/target/rollback checkpoint 复用节点聚合器和双组 collector，正常切换、冻结期源恢复及目标激活后二次冻结回退均验证 authority、epoch、phase、lease 与 manifest。完成、回退意图和回退完成使用版本化 decision artifact。真实 Redis writer + 模拟 observation/Kafka collector 集成覆盖 forward、两条 rollback 和 transition 成功但 artifact 缺失的恢复。恢复 CLI、租约续期和真实 crash/rebalance/Redis 故障演练仍待完成。
- C3 增加自包含 cutover attempt workspace：创建时 canonicalize 并不可覆盖保存 initial transition、source/frozen/target 节点清单与 checkpoint 清单，由代码生成精确绑定 initial lease 和全部输入摘要的 `attempt.json`；重试创建只接受完全相同的 canonical 输入。恢复加载会严格解码并重算每个绑定，再打开独立 artifacts 目录，避免操作员在续切时重新提供已漂移的外部文件。
- C3 增加 `dipole-realtime-cutover` 恢复命令：`create` 在 initial lease 有效期内生成 workspace，`status` 无需 Redis/Kafka 即可回放状态，`advance` 每次只执行一个外部动作并落盘一个事件，`rollback` 从合法状态记录回退意图。变更操作要求显式确认与 operator，单动作 30 秒超时；模糊失败后重复调用会恢复同一 artifact/receipt，禁止一次进程跨多个未持久化副作用。
- C3 增加证据链内的单步 lease 续期：`renew` 使用最新 transition hash 执行可恢复 Redis CAS，将 receipt/artifact/event 依次持久化且不重置冻结中断预算。续期后，绑定旧 lease 的 source checkpoint、frozen observation、target checkpoint 或 rollback observation 会回退至前一采集状态，后续切换必须重新生成当前 lease 证据；真实 Redis 测试验证续期后的 checkpoint 精确绑定新 receipt。持续调度与真实故障演练仍由 `AD-041` 跟踪。
- C3 增加可复跑的隔离故障演练并归档 `benchmarks/c3-cutover-faults-2026-08-28/`：随机 loopback Kafka/Redis、两个真实 consumer group、生产 Redis observation/checkpoint 组件及 TCP fault proxy 在 race 下验证 controller artifact/journal 崩溃恢复、Redis outage 阻断与恢复、primary group member 丢失/重建，三次故障均未错误推进 journal，forward 最终完成 sequence 7。第二 attempt 以 500 ms 中断预算验证首次 freeze 超时后自动回切，经 source-node frozen proof、Go epoch 2 激活和 source checkpoint 收敛为 `rolled_back` sequence 7。脚本自动校验低敏报告并清理 Compose project/volume；持续 controller 与 C++ primary 切流仍由 `AD-041` 跟踪。
- C3 增加持久化 cutover attempt 状态机、单步恢复 orchestrator 与 action artifact store：不可变 manifest 绑定源/目标 authority、初始 epoch、中断预算、三阶段节点清单和双组 checkpoint 清单摘要；哈希链事件日志严格约束顺序、时间与 artifact 摘要。每一步使用确定性 action ID，执行失败不推进 journal，替换进程会重试同一动作；首次 freeze 超过中断预算后自动转入源 authority 回退。artifact envelope 保存并分别绑定完整 canonical action 与严格 JSON transition/checkpoint payload，可脱离控制器状态独立复核；同一 action 重放返回原文件，任何 binding 漂移 fail closed。Redis writer 可按稳定 action ID 严格恢复既有 receipt，覆盖 transition 成功后进程崩溃的模糊窗口，并统一拒绝 receipt 重复字段。正常续切、冻结期直接回退，以及目标激活后二次冻结回退均有确定性归约。文件采用 `0600`、不可覆盖写入与目录 fsync。恢复 CLI、租约续期和真实自动回切演练仍由 `AD-041` 跟踪。
- C3 归档首个隔离双组 checkpoint 实证到 `benchmarks/c3-cutover-checkpoint-2026-08-28/`：真实 kafka-go compatibility member 与 C++ librdkafka member 均为 `Stable`，direct/group 两分区的 committed/log end 都为 `1/1`、lag 0，active epoch 1 的两节点 proof 与 lease hash 完整进入 mode `0600` bundle。删除预期 C++ observation 和停止候选组分别被 fail closed，且未创建输出文件；共享三节点未被修改。该证据验证收据链与跨客户端 assignment，C++ 进程使用 shadow authority，完整 primary 切换和自动回切仍待后续演练。
- C3 增加预期节点聚合与持久双消费组 checkpoint bundle：版本化 manifest 显式列出 Gateway/C++ 实例及其本地 expected authority，控制面逐键读取未过期 Redis observation。active 要求全部节点指向目标 authority 且 authorized；frozen 允许 Go/shadow/C++ 准备节点并存，但全部必须对同一 transition lease 报告 `denied/frozen`。proof 精确绑定 observed authority、epoch、phase、deadline 与 `next_sha256`。只读 Kafka collector 同时验证 compatibility/primary group 均为 `Stable`、assignment 完整、逐分区 committed=read-committed log end、两组高水位一致，并在采集结束后复验短 TTL 节点 proof。原始 DescribeGroups adapter 严格解析 ConsumerProtocol assignment v0-v3 的有界 topic/partition 前缀，并容纳 kafka-go、librdkafka 等客户端的 opaque 扩展尾部；真实隔离 kafka-go + C++ librdkafka 双组收据通过。`dipole-realtime-cutover-checkpoint` 将完整节点聚合与哈希绑定的分区 receipt 以 `0600`、不可覆盖、文件及目录 fsync 的 JSON bundle 落盘。自动续切/回切和真实共享中断演练仍由 `AD-041` 跟踪。
- C3 增加默认关闭的共享 delivery fence Go/C++ 数据面：版本化 Redis JSON lease 固定 `epoch + authority + active|frozen + expiry`，跨语言 golden vectors 统一合法、冻结、过期、ABA、未知、重复字段、非法 phase 及非正 lease 的授权结果和 reason code。Gateway 在每条 message-created 写入/checkpoint 前读取；被拒 handler 停留在当前 Kafka record，不进入业务 retry/DLQ。C++ shadow/primary 在每个 pending record 投影前读取，拒绝时不写 evidence/commit。新增本地运维 CLI，以 Redis Lua CAS 强制 `bootstrap-go -> freeze(epoch+1) -> activate(same epoch) -> renew`，原子保存有 TTL 的低敏幂等 receipt；真实隔离 Redis 生命周期通过。Go Gateway 以稳定 Presence 节点 ID 在启动、5 秒空闲心跳和 readiness 写 15 秒 observation；C++ 要求显式 instance ID，在 evidence/Kafka 初始化前及空 poll heartbeat 写同一契约。两端均绑定预期/实际 authority、epoch、phase、lease deadline 与原始 lease SHA-256，写失败 fail closed；消息热路径保持只读或节流，避免 observation 写放大。真实 C++ Redis 演练在 Kafka 不可用期间持续刷新 authorized epoch 18 记录。预期节点聚合、持久 checkpoint receipt 和自动回切尚未实现，`AD-041` 继续处理中。
- C3 归档本地互斥 delivery authority 实测到 `benchmarks/c3-delivery-authority-2026-08-28/`：隔离 `go` 模式对目标消息只产生一条无 delivery ID 的客户端 frame；隔离 `cpp` 模式只产生一条带稳定 C++ delivery ID 的 frame。两种模式的 Gateway authority 指标均与配置一致，`cpp` 下 Go checkpoint group 与 C++ primary group 同时追至 log end/lag 0，terminal evidence 为 `ENQUEUED(1)/commit`。自动报告 12/12 通过，隔离资源已清理且共享三节点持续运行。本证据完成本地单帧门槛；共享动态 fencing、双 group receipt 和自动回切仍由 `AD-041` 阻断。
- C3 增加默认 `go` 的 `realtime.delivery=go|shadow|cpp` 本地 authority 契约：Gateway 在启动副作用前校验 observation/primary 能力组合，并以有界 Prometheus 标签暴露当前 ownership；`go` 与 `shadow` 保留 Go 消息客户端写入，`cpp` 对消息 Topic 仅验证 v1 事件并推进原 Go consumer group checkpoint，继续处理踢下线、已读和群变更等非消息事件。C++ `shadow`/`primary` 命令分别要求 `shadow`/`cpp` authority，配置错配会在连接 Kafka 前 fail closed。该切片尚未提供跨副本共享 fencing、双 group 切换 receipt 或自动回切，`AD-041` 保持处理中且 tracked Compose 继续为 Go authority。
- C2 归档显式 primary runtime 的真实证据到 `benchmarks/c2-primary-runtime-2026-08-28/`：干净同 revision Go/C++ 镜像在隔离 topology 中完成 `ENQUEUED(1)` 后 offset commit；600 KiB C++ probe 将真实 WebSocket queue 压至 16/16 并返回 `PARTIAL/BACKPRESSURED/QUEUE_FULL`；错误 gRPC target worker 对同一 Kafka 坐标写 deferred/retain，`SIGKILL` 前 lag 为 1，正常 worker 重放后 commit 且 lag 归零。报告 8/8、校验和与低敏扫描通过。并行运行的 Go consumer 同时产生 legacy frame，暴露双投递并登记 P0 `AD-041`，因此 C2 runtime evidence eligible，C3 cutover blocked。
- C2 增加默认关闭的显式 C++ primary runtime：`primary` CLI 要求命令入口、`DIPOLE_REALTIME_PRIMARY_ENABLED=true`、Presence primary 与 node transport primary 同时成立，并使用独立 `dipole-realtime-primary-*` Kafka group。Runner 固定 `poll -> project -> Presence -> DeliverNodeBatch -> primary-evidence.v1 -> commit`；只有完整 terminal `ENQUEUED|OFFLINE` ACK 集合或全部 Presence offline 才提交，partial/backpressure/rejected/failed、身份漂移、RPC 或 evidence 故障均保留同一 pending record。shadow v3 schema、命令和默认 Compose 保持不变；真实 consume-to-ACK commit、queue saturation 与进程崩溃重放仍待归档。
- C2 冻结 primary runtime 的 Kafka offset 决策边界：C++ 对完整 `NodeDeliveryBatch/DeliveryAck` 集合复核 batch/delivery identity，仅当所有结果均为 terminal `ENQUEUED|OFFLINE` 时给出 `commit`；`PARTIAL/BACKPRESSURED`、`REJECTED`、`FAILED` 与任何身份/数量漂移均给出 `retain` 或 fail closed。该纯分类器同时约束 one-shot probe 与显式 primary runner。
- C2 归档默认关闭的 primary seam 跨进程证据到 `benchmarks/c2-primary-delivery-seam-2026-08-28/`：clean revision 的 C++ one-shot probe 经 mTLS 精确投递真实 WS connection，首次 ACK 为 `ACCEPTED/ENQUEUED(1)` 且客户端只收到一条带稳定 `delivery_id` 的帧；相同批次模拟 ACK 丢失后重放，ACK 保持一致且无第二帧；Redis TTL 尚存但 Hub 已关闭的 connection 返回 `ACCEPTED/OFFLINE`。自动报告 7/7 检查通过，并明确保留真实部分 queue saturation 与 primary runtime Kafka offset/crash replay 两项后续门槛。演练同时发现 WS query JWT 进入访问日志，归档已脱敏并登记 `AD-040`。
- C2 增加默认关闭的 Gateway primary Delivery seam：语言中立 `NodeDeliveryService.DeliverNodeBatch` 返回逐项 `DeliveryAck`；Hub 按 `user_id + connection_id` 精确入队并区分 enqueued/offline/backpressured，稳定 `delivery_id` 进入 additive WebSocket envelope。Go dispatcher 对同一 batch 的 payload/target 漂移 fail closed，按 delivery/connection 保存有界 terminal 状态，部分成功重试只触达未完成 connection；Web 以账户隔离的 IndexedDB v4 原子 claim 和 4096 项有界 FIFO 去重重连及页面重载重放，登出时清理对应账户。C++ transport 已验证认证调用和 ACK，ShadowRunner 继续固定 `ObserveNodeBatch`，配置 `delivery_primary_enabled=false`，尚未进行生产切流。
- C2 关闭 `AD-039`：Gateway readiness 新增消费组级 `kafka-assignment` 探针，首次成功前保持不可就绪，并要求 group 为 `Stable` 且所有已注册 base/retry topic 均有分区；现有依赖探针默认启动语义保持兼容。独立 clean revision 冷启动从 `Empty 0` 开始，首样本 `/readyz=not-ready`、assignment=0，约 10.2 秒后收敛为 `Stable 20`、`/readyz=ready`、assignment=1；证据归档到 `benchmarks/c2-gateway-assignment-readiness-2026-08-28/`。微服务 Compose 的内部证书目录支持覆盖，readiness smoke 使用临时目录并校验 assignment 指标。
- C2 归档同一 clean revision 的 Go/C++ 实时投递对照到 `benchmarks/c2-cpp-comparison-2026-08-28/`：Go concurrent workload attempted/accepted/persisted/received 为 40/40/40/40；C++ 观察 80 个 Kafka 坐标，按 `message_type=0` 选中 40 条并将 40 条初始化系统消息保留为 filtered-out，选中记录 projected 与 node requested/observed 均为 40/40，最终拒绝/背压为 0，comparison 决策为 eligible。真实 TCP Gateway queue saturation race 测试验证 `BACKPRESSURED/QUEUE_FULL`；该轮演练发现并登记了后续关闭的 Gateway assignment readiness 缺口 `AD-039`。
- C2 增加语言中立的 Go/C++ 同 workload 对照门禁：Go baseline 显式声明低敏 `message_type` selector，C++ v3 evidence 保留全部 Kafka 坐标并按 selector 排除好友初始化等系统事件；报告同时记录 observed、filtered-out 与 workload 计数。选中记录按 Kafka 坐标折叠 deferred 重试，要求最终 projected、全部节点请求 observed、最终拒绝/背压为零，并与 Go baseline v4 的接受、持久化、接收和 settled lag 精确匹配。报告绑定双端完整 revision 与输入 SHA-256，结构错误、blocked、eligible 使用独立退出码；canonical C++ gate 持续运行其单测与 schema 检查。
- C2 C++ ShadowRunner 在 transport、Presence、evidence 或 commit 失败后保留至多一条未提交 Kafka record，并沿用 Runtime 的有界错误退避在同进程重试；只有 commit 成功才清除 pending record，deferred 尝试不再重复累计 projected 统计。真实 backpressure 和同 workload Go/C++ 对照仍未启用客户端写入。
- C2 归档首个 C++ node delivery 跨进程 shadow 证据到 `benchmarks/c2-cpp-node-delivery-2026-08-28/`：C++ 经 Kafka 与 Redis Presence 聚合节点批次，使用 `dipole-realtime` mTLS 身份调用默认关闭的 Go Gateway observation receiver；Gateway 故障时 offset 保持未提交，恢复并重启 worker 后重放成功，回拨已提交 offset 命中稳定 batch 去重，最终 lag 为 0，全程客户端写入为 0。证据同时记录当前 deferred record 需要 worker 重启才能重放，以及发布镜像前必须刷新预构建 `dist/` 的边界。
- C2 增加默认关闭的 C++ node gRPC transport shadow：显式 `node=target` 路由、10 ms 至 30 s deadline、`dipole-realtime` 服务认证、correlation metadata 和可选 mTLS；明文 target 仅允许 loopback。ShadowRunner 顺序升级为 Presence 投影、节点观察、`shadow-evidence.v3`、Kafka commit；部分成功后的重试依靠稳定 batch ID 与 Gateway 去重，RPC 拒绝/背压/故障会写低敏 `deferred` 证据、保留 offset 并撤销 readiness。真实本地 gRPC、runner 顺序、`-Werror`、clang-tidy 与 11 项 CTest 通过，生产 Compose 仍未启用。
- C2 增加 Gateway 节点投递观察接收端：默认关闭的独立 gRPC listener 只允许认证的 `dipole-realtime` 调用方，按 Presence node ID 校验 `NodeDeliveryBatch`，通过有界队列、稳定 batch 去重和明确 backpressure 返回观察结果；消费端仅累计低敏批次/条目/连接计数，不调用 WebSocket Hub，也不改变现有 Go Delivery。配置样例固定 loopback、容量和重试提示，race 与完整 Go 门禁通过。
- C2 增加 `NodeDeliveryService.ObserveNodeBatch` 跨语言观察合约：`NodeDeliveryObservation` 独立表达 observed/rejected/backpressured、节点批次聚合计数、QueuePressure 与 duplicate 去重，不复用客户端 `DeliveryAck`；Go/C++ validator 与第四组 golden vector 已固定。
- C2 归档 Presence Shadow 与 Sentinel 恢复证据到 `benchmarks/c2-cpp-presence-2026-08-28/`：同一 hiredis reader 在停止当前 master 后完成 80 次读取中的 75 次成功、5 次有界错误，并自动从 `redis-2` 恢复到 `redis-3`，无需进程重启；隔离项目、网络和卷已清理。
- C2 将 Presence 注入 Kafka ShadowRunner，并升级低敏证据为 `shadow-evidence.v2`：记录节点批次及 malformed/observed/eligible/stale/offline 聚合计数；Redis 读取失败不提交 offset，身份漂移记录 `invalid_presence` 后提交。真实联合回放 206 条，205 projected、1 poison rejected、20 个非空节点批次，最终 lag 为 0。
- C2 增加 hiredis 1.2 Presence 只读 adapter：支持 direct 或 Sentinel master discovery、AUTH/SELECT、批量 `HGETALL` pipeline、命令失败后断连重发现，以及 Go Hash 兼容解析；真实 Redis 隔离 fixture 验证通过，尚未注入 Kafka ShadowRunner。
- C2 增加 C++ Presence 纯投影边界：兼容解析 Go Presence Hash JSON/RFC3339 时间，将连接快照按 TTL 过滤后确定性分组为 `NodeDeliveryBatch`，显式统计 malformed/observed/eligible/stale/offline，并对用户身份漂移和跨节点重复 connection 所有权 fail closed；当前尚未连接 Redis 或写 Gateway。
- C2 增加可独立运行的 C++ Kafka shadow：`shadow` 命令在 canonical golden 校验后启动 consumer worker 与动态健康面，只有实际 partition assignment 且最近一次 evidence-before-commit 链健康时 ready；broker 不可达时 live 保持 200、ready 返回 503，SIGTERM 有界退出。`sync_fanout=false` 可在无 Redis 阶段选择热群通知，进程仍不写 Gateway/客户端且未进入生产 Compose。
- C2 增加 Ubuntu 24.04 多阶段 Realtime Delivery 镜像：builder 显式安装 C++/Protobuf/nlohmann-json/librdkafka 并运行全部 CTest，runtime 只保留二进制、共享库与 Delivery golden contracts，同时写入 OCI revision/created/dirty 标签；镜像尚未加入生产 Compose。
- C2 增加 C++ Kafka shadow 消费边界：librdkafka C API 强制独立 `dipole-realtime-shadow-*` group、earliest、手动 offset 与 round-robin assignment；runner 仅在低敏 NDJSON evidence 刷盘后同步提交 offset，poison event 记录固定类别，evidence/commit/poll 失败撤销 readiness。
- C2 增加 C++ Kafka message-created 纯投影层：严格解码 v1/minor-additive envelope，将 direct、普通群、热群、Timeline shadow 和文件消息映射为 canonical `DeliveryEnvelope`，固定稳定 batch/delivery ID、Kafka source coordinates、用户 ordering key 与 Go WebSocket payload 形状；兼容缺少 mutation/revision/actor/Seq 的 legacy created 完整消息，重复 recipient、channel/target 漂移和未知 major version fail closed。当前 executable 仍为 `contract_only`，未连接 broker、Redis、Gateway 或客户端。
- C2 增加首个独立 C++20 Realtime Delivery foundation：CMake 在 build 目录从 canonical `dipole.delivery.v1` 生成 C++ Protobuf 类型，手写 validator 与 Go 共用三组 golden vectors，并以 `contract_only` CLI/进程验证 envelope、节点批次和背压 ACK。进程启动前完成契约校验，只暴露 `/livez`、`/readyz`、`/health`，host/port/mode 非法时 fail closed；当前未消费 Kafka、查询 Redis、写客户端或进入 Compose。统一门禁使用系统 GCC 13、Protobuf 3.21、`-Werror`、clang-tidy 和 CTest。
- C2 建立语言无关的 `dipole.delivery.v1` 实时投递契约：`DeliveryEnvelope` 固定 Kafka source coordinates 与用户级投递项，`NodeDeliveryBatch` 固定 Presence 解析后的节点/connection 批次，逐项 ACK 覆盖 enqueued、offline、backpressured、rejected 和 failed，并用饱和队列 retry hint 表达背压。三个 Protobuf JSON golden vectors 与 Go fail-closed validator 约束枚举、时间戳、批次上限、ID 唯一性和 ACK 一致性；legacy adapter 可映射现有 Go Hub 返回值但暂不接管流量，默认仍为 Go Delivery。
- C1 增加隔离且可回滚的 Go 候选基准拓扑：`docker-compose.dist.yml` 在保持旧默认值的同时支持 image、容器前缀、宿主端口和网段覆盖；`candidate_topology.sh` 只接受与干净工作树同 revision 的镜像，固定 image SHA、关闭 embedded Agent，并依次等待基础设施、执行 MinIO 初始化和 one-shot migration 后启动独立 project，`down` 保留候选卷。迁移编排不进入基础 Compose 服务依赖，既有共享拓扑继续支持 `compose start`。canonical Compose gate 同时验证默认和候选渲染。
- C1 归档同一候选镜像下 20/50/100 并发连接、每连接 2 条消息的 Go 实时数据面梯度证据：三档接收、持久化和投递率均为 100%，Kafka lag 最终归零；吞吐从 2.51 增至 4.85 msg/s 的同时 P95 从 1.07s 增至 8.08s，提示下一步需用故障恢复和分段剖析定位等待/串行化路径。
- C1 增加版本化单节点 stop/start 恢复门禁：恢复 evidence 绑定前后 container/image/revision/PID、单调健康时间线和故障前稳定 consumer group 成员数，要求确实观察到 unavailable、新 PID，并在恢复到相同成员数且连续稳定 5 秒后才运行负载；恢复后复用 baseline v4 验证消息接收、持久化、投递和 Kafka lag，并以精确 baseline SHA-256 生成 recovery report。EXIT trap 在演练异常时尝试恢复目标节点，稳态资源门禁继续拒绝意外 PID 漂移。
- C1 基准健康检查支持自定义节点 URL，operations/baseline v4 归档无凭据的 API 与 WebSocket 实际端点，避免隔离端口与报告环境描述漂移。
- C1 为 Go 实时数据面基准增加运行镜像来源门禁：Docker 镜像写入 OCI revision/created 与 source dirty 标签，`run_bench.sh` 从运行容器解析不可变 container/image ID，并要求被测服务、采集器和干净源码树绑定同一完整 Git revision；operations/baseline v4 归档逐服务来源证据，缺标签、dirty 构建、提交偏差或重复容器绑定均在负载启动前 fail closed。
- C1 增加 Go 实时数据面资源基准采集：`process_metrics.py` 从固定服务 PID 的 `/proc` 多点采样 CPU、RSS、线程与 context switch，`run_bench.sh` 将结果绑定到 operations/baseline v4；进程重启、服务集合漂移和计数异常 fail closed，v1-v3 历史报告继续可读并显式标记资源证据不可用。
- Agent G4 增加 Event Subscription rollout evidence gate：CLI 同时读取 corpus、双评审 review 和 candidate evidence，重新执行 review/prefilter evaluator 后才产出低敏 `eligible|blocked` 决策，避免信任调用方预聚合报告。决策绑定 corpus、review、final-label、candidate evidence/configuration 哈希与 agreement、precision/recall、p95、成本指标；任一门槛失败返回 2，哈希/结构/逐 case 绑定无效返回 1。该门禁不修改 Trigger/Runtime mode 或 Capability authority。
- Agent G4 增加 Event Subscription corpus review v1：语言中立 review/report schema 将两个独立 reviewer 的完整逐 case 标签绑定到 prefilter corpus SHA-256；有分歧时要求第三个独立 adjudicator 精确裁决全部分歧 case。纯离线 evaluator 对身份复用、case 覆盖、裁决集合和最终 corpus 标签执行 fail-closed 校验，CLI 以 `0/2/1` 区分达标、未达标和无效输入。低敏报告只含 review/final-label 哈希、agreement bps、计数和异常 case ID，不回显消息正文或 reviewer 身份。
- Agent G4 增加 provider-neutral Event Subscription prefilter Eval v1：语言中立 corpus/evidence/report schema 将受控事件标签与 `rule|embedding|small_model` 候选决策分离，绑定 corpus、strategy revision 和 configuration SHA-256。纯 TypeScript evaluator 不访问模型、数据库或网络，输出低敏混淆矩阵、保守整数 precision/recall bps、nearest-rank p95、微美元平均/总成本及误判 case ID；缺失/重复 case、hash 漂移、分数/阈值决策漂移和单类 corpus fail closed。首个 rule adapter 直接复用生产 matcher，CLI 使用 `0/2/1` 区分达标、未达标和无效证据；生产仍固定 `direct_target`。
- Agent G4 增加 Gateway-only Event Subscription 管理控制面：migration v34 为 v28 订阅补充 creator/revoker、撤销原因与更新时间，并从绑定的不可变 Definition owner 回填历史审计。认证 `dipole-gateway` 从 RequestContext 派生 owner，可创建、分页查看和撤销精确 Definition version 订阅；Core 重新校验 tenant、Agent、有效期、`conversation.read` 与 resource scope。`message_contains_any` 以 trim/lowercase/去重/排序生成规范 JSON 和稳定 SHA-256 Subscription ID，等价创建及同原因撤销可安全重放，漂移返回冲突。公开 list/create/revoke 已进入默认关闭的 HTTP/Pencil/Vue 页面；语义预筛和生产触发切换仍未启用。
- Agent G4 增加 Gateway-only 晋升证据 review projection：已获 tenant-scoped promotion operator 权限的 reviewer 可按 Proposal ID 读取其精确绑定的 `promotion_evaluation` Artifact 元数据和正文；应用层先复用控制面 `Get` 授权，再校验 Artifact ID/type/version/media/content SHA-256 并从专用对象存储复算正文证据。未授权调用不会触达 Artifact reader，普通 Task-principal 下载权限不变，专用存储未装配时 RPC unavailable；当前未暴露公共 HTTP、active authority 或 write Tool。
- Agent G4 补齐真实 Shadow 晋升证据发布链：`promotion:publish` 只接受语言中立 v1 publication 输入，重新解析完整 promotion v2 证据并要求决策为 `eligible`，随后以 canonical JSON 创建 completed Shadow Run 绑定的不可变 `promotion_evaluation` Artifact。Core 对该终态例外严格校验固定类型/版本/媒体类型、pinned Definition、candidate、Suite SHA-256、正文 envelope 与 metadata 一致性；普通 Artifact 继续仅允许 running Shadow Run。CLI 只输出 Artifact/content/Suite hash 绑定的低敏收据，不自动创建 Proposal、Grant、active Run 或 write Tool。
- Agent G4 增加默认关闭的 approved Capability projection：Core 只在 durable promotion authorizer 已通过的 active admission/context resolve 后，从 pinned Definition 的 `message.write` 与 conversation/write scope 投影显式 allowlist 中唯一的 `message.system.send`；shadow、未知 ID、重复项及 Registry 后增 Tool 均无法扩权。Go RPC 出口与 TS ExecutionContext 双重校验，TS write Policy 仍要求同一 Capability，后续还需精确 Approval consumption、Tool audit 和 Message Command。生产 Bootstrap 未注入 authorizer，Runtime 入口未注册 write projection/executor。
- Agent G4 增加默认关闭的 Runtime promotion control plane：migration v33 与 sqlc 建立 tenant-scoped operator Grant、不可变 Proposal、唯一 Review 和追加式 Revocation 审计。提案必须绑定 `promotion_evaluation` Artifact 的内容哈希及 metadata 中的 Runtime candidate、pinned Definition、Eval Suite SHA-256，并与权威 Task/Run 逐项一致；不同 reviewer 审批时在同一 MySQL 事务生成 durable Grant，授权 revoker 在同一事务追加审计并撤销。四个 additive gRPC 仅接受认证 `dipole-gateway` 并从可信 RequestContext 派生 operator。生产 Bootstrap 只装配控制面审计，仍未向 admission/resolver 注入 active authorizer，也未注册 write Tool。
- Agent G4 增加 durable Runtime promotion grant：migration v32 与 sqlc 持久绑定 tenant、Runtime candidate、pinned Definition、promotion v2、evidence SHA-256 和完整离线 Eval Suite SHA-256，并要求不同 grantor/reviewer 双人签署、生效窗口与可撤销状态。active Run 保存 candidate version；admission 和每次 persistent MCP context 解析都会重新校验 grant，撤销后已有 active Run 立即失去 context authority。生产 Bootstrap 未注入 authorizer，也没有 grant 签发 API、active admission 或 write Tool 注册。
- Agent G4 建立 fail-closed active ExecutionContext promotion seam：`PersistentAgentRunAdmission` 只有在注入的 promotion authorizer 对 Runtime candidate、Task 和 pinned Definition 精确授权后才允许创建 active Run；缺少 authorizer、candidate version 或授权拒绝均不会创建 Task/Run。Core MCP context 现在返回持久 Run 的权威 `runtime_id/mode`，Go Server 与 TS Client 双重拒绝伪造 Runtime 或非法 mode。生产 Bootstrap 未注入 authorizer，公开 admission 继续固定 shadow，write Registry/executor 仍关闭。
- Agent G4 增加 active-only MCP Approval grant resolution：sqlc 按 Task/Capability/Scope/Arguments 查询最多两条 approved、未消费、未撤销且未过期记录，应用层要求 active `dipole-agent` Run、运行中 Task、principal 审批人与唯一 exact binding；零匹配和多匹配统一拒绝。认证 RPC 只返回 Approval ID、Resource Scope、三个 SHA-256 摘要与过期时间，不消费审批。TS client 复算 scope/arguments 并校验响应，grant adapter 直接连接现有 write gate。`nonce_sha256` 明确作为一次性绑定摘要，避免依赖无法持久恢复的原始 nonce。生产 MCP context 继续为 shadow。
- Agent G4 增加默认关闭的第一方 MCP Message write projection：显式 write executor 只接受 `mode=active`、`risk=write`、`approvalRequired=true` 且声明 Command kind 的 Capability。调用先校验 direct conversation，再由 Approval gate 消费精确绑定，Tool runner 生成同一个 Invocation ID 依次完成 audit begin、Message Command RPC 和带 action reference 的 audit finish；Command kind 或返回证据漂移会记录失败终态。现有生产 `index.ts` 未注册 write Capability、未注入 executor/grant resolver，Core MCP context 继续为 shadow。
- Agent G4 增加认证的 MCP Message Command RPC：`dipole-agent` 只能在已持久化、仍为 running、带已消费 Approval 且 Task/Run/Capability/身份完全匹配的 Tool Invocation 中请求消息动作。Core 从可信 Invocation 派生 sender/target，按标准化 `{content, conversationId}` canonical JSON 重算审批参数 SHA-256，并根据 `invocation_id + command kind` 生成稳定 Command ID；RPC 只返回 Message UUID、Command 引用和可复算 `client_message_id`。TS Runtime 独立验证返回证据，Tool runner 与 Approval gate 统一使用排序 canonical JSON。生产 MCP Server 仍只注册 read Tool。
- Agent MCP ToolCall 增加可验证的消息动作血缘：migration v31 与 sqlc 在 Begin 记录已消费 `approval_id`，在完成记录中只保存 `message` 资源 UUID、Command kind/id 与既有结果摘要，不复制消息正文。Core 对写 Capability 重新校验 active Agent Run、Task、参数摘要、审批终态与资源授权，再按权威 Agent sender 和稳定 Command ID 查询 Message receipt；只有 committed Message 的 UUID、sender、target、conversation 和 message type 全部一致时才能完成审计。protobuf 与 TS runner 支持可选 action reference，读 Tool 携带审批/动作引用、失败终态携带动作引用或引用漂移均 fail closed；生产 MCP Server 继续只投影 read Tool。
- Agent Message Command 增加 sender-scoped receipt/status 查询：Message v1 新增 `GetMessageCommandReceipt`，从认证 principal 派生 sender，并复用现有 sqlc `(sender_uuid, client_message_id)` 查询返回 `ABSENT|COMMITTED`，不新增 Command 表或 pending 状态。Agent 发送失败后使用脱离原取消信号、保留 trace values 的独立 2 秒窗口查询 receipt；仅 sender、target、conversation、message type、content 和稳定幂等键全部匹配时恢复成功，查询故障保留发送与查询两条错误因果链。
- Agent G4 增加默认关闭的 MCP durable Elicitation adapter：只接受 `elicitation/create` form mode 的严格子集，将最多 16 个 text/select/multiselect/boolean 字段转换为现有 `dipole.agent.elicitation.v1` 和 Temporal `wait_input` directive。checkpoint 绑定 host-owned Request、Server、Tool、Invocation、deadline、完整 Form 与 `trust=untrusted` SHA-256；仅精确且未过期的 durable resume 可转换为 MCP `accept`，`decline/cancel` 也要求同一 Request。URL、number/integer、default、format、description、自由扩展、敏感字段和 checkpoint 漂移均拒绝。当前外部 MCP Client 未声明 Elicitation capability，也未注册 request handler。
- Agent G4 增加默认关闭的 MCP write Approval gate：Core 新增 authenticated、active-only 的原子 `ConsumeApproval` RPC，使用现有 MySQL/sqlc 条件更新精确绑定 Task、Run、Capability、Resource Scope、canonical Arguments 与 nonce 摘要，并让重放、过期、吊销或任一哈希漂移返回拒绝。TS `McpWriteApprovalGate` 先通过 Capability Registry 完成 schema、Policy 和 Resource 校验，再按与 Go 一致的 scope v1/canonical JSON 计算 claim；只有原子消费成功才执行副作用。生产 active authority 尚未装配，启动链继续只投影 read Tool。
- Agent G4 增加外部 MCP Result-to-Context adapter：只接收成功、可 JSON 序列化且默认不超过 64 KiB（1 KiB 至 128 KiB 可配）的 Tool 结果，生成固定 `section=evidence`、`trust=untrusted` 的可选 Context fragment，并用 Profile/Server/Tool/Invocation 绑定 provenance。完整正文保留为不可变 JSON 快照；compact 版本只包含 Server、Tool、content type 集合和 structured-content 标记，不复制可能含 Prompt Injection 的外部文本。该 adapter 尚未接入生产外部调用链，外部网络继续关闭。
- Agent G4 增加默认关闭的外部 MCP Network Guard：每个 SDK HTTP 请求都重新校验 Profile 的 exact HTTPS Host/Port/TLS ServerName，使用 2 秒默认 DNS deadline（100 ms 至 60 秒可配），要求 1 至 32 个解析结果全部为合法公网地址且无重复。批准地址集合和 opaque CA ref 被交给 pinned Dispatcher，返回后复核实际 peer；混合私网答案、DNS rebinding、非批准 peer、跨边界 URL 和任何重定向均以固定脱敏错误 fail closed。当前只有 Resolver/Dispatcher 接口与测试实现，生产 DNS/TLS Dispatcher、CA Secret backend 和外部连接开关继续关闭。
- Agent G4 增加 provider-neutral 外部 MCP `AuthProvider` adapter：官方 SDK 每次请求前按 exact Credential binding 调用 Secret Provider，默认 2 秒超时（100 ms 至 60 秒可配）和 4 KiB 上限（16 B 至 8 KiB 可配），只接受严格 UTF-8 与 RFC 6750 Bearer 字符。Provider failure、timeout、非法内容分别收敛为 `secret_unavailable`、`secret_timeout`、`secret_invalid`，不携带底层异常或 credential 标识；成功、校验失败及超时后迟到的 fresh byte buffer 均被覆盖。Adapter 不缓存 token 且不实现 `onUnauthorized`，避免 SDK 自动刷新；JavaScript token/Header 字符串仍由 GC 管理，生产 Secret backend 与外部网络继续关闭。
- Agent G4 增加外部 MCP Credential Catalog 的受约束文件源：只接受规范绝对路径，要求 canonical、root/预期 Runtime owner 且 group/other 不可写的父目录，并在每次解析时以 `O_NOFOLLOW` 重新打开 single-link regular file；文件 owner/mode 同样受检，配置上限为 32 B 至 1 MiB（默认 256 KiB）。固定缓冲读取可阻断检查后增长绕过，原子 rename 后下一次建连立即使用新 lifecycle manifest；任意路径 symlink、目录、错误 owner/mode、超限和畸形 JSON 均 fail closed，旧 Catalog 不缓存。该 source 尚未装配到生产启动链，也不包含 secret material。
- Agent G4 增加外部 MCP Credential Catalog v1：语言中立 lifecycle manifest 仅保存 tenant、credential ref/version、生效/过期/吊销状态、provider ID 和 opaque provider secret ref，拒绝附加 secret 字段、重复绑定与非法时间状态。Catalog 在每次建连前重新加载，精确版本已吊销、未生效、过期或跨租户时均在 Transport Factory 前 fail closed；轮换可通过新增版本并更新 Profile 完成，Task、Workflow、Context 和审计不接触秘密正文。生产 Catalog source、Secret Provider 与真实外部连接继续关闭。
- Agent G4 增加默认关闭的外部 MCP 凭据与网络边界：语言中立 Profile v1 仅允许 tenant、HTTPS endpoint、Server/Tool/Host/Port allowlist、TLS ServerName、CA opaque ref 和版本化 credential opaque ref；拒绝 URL 凭据、query/fragment、IP/localhost/内部域名及附加 secret 字段。租户 Registry 只有在精确 owner 匹配后才调用注入式 Transport Factory，并要求 Factory 每次建连拒绝非公网 DNS 解析；当前生产 Provider 尚未实现，误开 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED` 会直接 fail closed，不产生外部连接。
- Agent G4 为 MCP 执行补充有界取消/超时：Runtime Tool invocation 默认 5 秒并限制在 100 ms 至 60 秒，超时触发 cooperative `AbortSignal`、持久化稳定 `tool_timeout` 且关闭 OTel span；外部 MCP Client 的 connect/list/call 默认使用 10 秒 request/total timeout并接受调用方取消信号。Gateway 断连经 Runtime Streamable HTTP Request signal 传播，DELETE Session 清理与长流入口不受统一代理超时破坏。配置保持默认关闭路径无行为变化。
- Agent G4 增加默认关闭的第一方 MCP 授权交换与资源边界：受认证 session 需对唯一 canonical resource 和 `dipole.agent.mcp.read` 显式 consent，才能取得 15 分钟专用 JWT；令牌以 `aud`、`scope`、`token_use` 防止跨资源和 session/MCP 混用。Gateway 验证后剥离客户端凭据，仅向 Runtime 传递可信 principal/resource/scope，Runtime 再构造只读 `AuthInfo`。Compose 支持统一覆盖 canonical URI，发布与回滚见 `docs/agent/agent-mcp-authorization.md`；通用 OAuth 2.1 discovery/PKCE/客户端注册继续由 `AD-037` 跟踪。
- Agent G4 增加默认关闭的 OpenTelemetry 运维 profile：Collector `0.159.0` 通过 128 MiB memory limiter、有界 batch/queue 和重试向 Tempo `2.10.5` 写入 trace，Tempo local backend 固定 24 小时保留且端口只绑定 localhost；Prometheus 新增 Collector down、export failure 和 refused span 三类低基数告警。配置 gate 验证 Collector 与规则，真实 smoke 生成 Agent span、核对 accepted/sent 指标并按 trace ID 从 Tempo 查询；运维说明补充 Task/Run 审计联查和回滚。生产对象存储与通知链继续由 `AD-037` 跟踪。
- Agent G4 增加默认关闭的生产 OpenTelemetry 装配：Node trace SDK 通过 OTLP/HTTP protobuf 导出既有低敏 span，复用标准 `OTEL_*` endpoint、protocol、ParentBased trace-id ratio sampler 与超时参数，限制单 span 属性/事件/link 数量，并在 Runtime 逆序关闭末尾 flush。关闭 `DIPOLE_AGENT_OTEL_ENABLED` 时不实例化 SDK且忽略残留 OTel 配置；Collector、保留和告警继续由 `AD-037` 跟踪。
- Agent G4 增加统一低敏 OpenTelemetry 全链路：Foundation Event Processor 记录 Task/Run，Durable Activity 记录 Task admission/finish、Run、Approval 和 Artifact，Context Compiler 记录版本与预算统计，Model Router 为每个真实 provider attempt 记录 ModelCall，native Capability 与 MCP 均记录 ToolCall。Temporal Workflow 保持确定性；span 禁止 Prompt、消息、Memory、Tool/Artifact 正文和底层异常文本，SDK/exporter 装配由后一里程碑承接。
- Agent G4 增加真实 Shadow Task 离线评测 adapter：sqlc 与 TypeScript 从同一只读 SQL 源提取 Task/Run、Context provenance、Step、Artifact、ModelCall 和 ToolCall，结合版本化人工评审 manifest 生成五类 Suite；Task/Run 摘要进入 case ID，报告继续保持低敏。独立评测账号仅授予八张审计表 SELECT；非终态、capability 绑定漂移、缺失 Token/延迟或路由单价均 fail closed。当前仍需归档 Project Guardian 人工语料、reviewer agreement 与生产阈值后才能满足 `AD-038` 关闭条件。
- Agent G4 增加五路径 deterministic security suite：真实 ContextCompiler 验证 Prompt Injection 保持 `untrusted` provenance，Capability Registry 验证越权资源在执行前拒绝，EventLedger/lineage 验证重复事件单次规划与同源循环在 Ledger 前抑制，MCP Client/Server 验证敏感外发在网络调用前阻断。外部 MCP Tool 现在必须配置与 allowlist 完全匹配的 egress policy，限制顶层参数、JSON 大小和嵌套深度，并递归拒绝常见凭据字段；值级 DLP 与模型语义攻击继续作为 Shadow 切流门禁。
- Agent G4 增加 outcome、trajectory、permission、retrieval、cost 五类 deterministic 离线评测：严格 Suite/Report 契约要求五类 case、唯一 ID 和有界输入，以 canonical SHA-256 绑定候选数据集；CLI 输出不含消息正文的类别结果与 precision/recall、调用、Token、成本和延迟指标，并以 `0/2/1` 区分通过、评测失败和输入错误。Shadow promotion v2 强制携带同一候选版本的完整报告并逐类别阻断，v1 证据继续兼容；当前 synthetic fixture 只验证 Harness，真实 Task/corpus 由 `AD-038` 跟踪。
- Agent G4 为认证 MCP 入口增加 Redis principal 限流：Gateway 在 JWT principal 解析后对 GET/POST 使用统一 `rate:agent_mcp:{principal}` 固定窗口，跨 Task、Run、方法和 Gateway 实例共享额度；超限返回 429 与向上取整的 `Retry-After`。Redis 缺失、调用失败或额度配置非法时 fail closed，DELETE 继续允许释放 Streamable HTTP Session；旧登录、消息和文件限流的兼容语义不变。
- Agent G4 增加 migration v30 与 MCP ToolCall 持久审计：Core/sqlc 绑定权威 tenant、principal、Agent、Task 和 Run，仅保存参数/结果 SHA-256、结果字节数、耗时、稳定错误码与单次终态；TypeScript Runtime 只有在 durable begin 成功后才执行只读 Capability，并使用 `@opentelemetry/api` 创建不含正文的 ToolCall span。结果超过 64 KiB、工具异常或审计不可用均 fail closed；默认未装配 OTel SDK/exporter。
- Agent G4 增加默认关闭的认证 MCP 网络入口：Gateway 仅在 JWT 成功后转发 `GET/POST/DELETE` Streamable HTTP，并覆盖客户端伪造的服务身份与 principal；Runtime 通过 additive `ResolveMcpContext` RPC 让 Core 复核 Task、Run、固定 Definition、grant 和 resource scope，再构造只读 `conversation.list` ExecutionContext。服务密钥、Cookie 与公开 Authorization 不进入 MCP handler，SSE 和 Session header 保持流式转发。
- Agent G4 增加官方 MCP TypeScript SDK v2 foundation：MCP Server 将显式 allowlist 的只读 Capability 投影为 Tool，并由宿主注入经校验的 ExecutionContext；MCP 参数无法提供 principal。MCP Client 同时校验配置 allowlist、握手 Server identity、已发现 Tool、256 项发现上限及 128 KiB 响应上限。进程内与 Streamable HTTP 契约均通过，HTTP handler 要求宿主提供已验证 `AuthInfo`；write/destructive Tool 保持关闭。
- Agent G3 为 `WAITING_INPUT` 与 `WAITING_APPROVAL` 增加持久 deadline 和 Temporal Timer：Input Activity 与已持久 Approval binding 提供绝对截止时间，Workflow history 固定该值；到期且没有精确 Signal 时确定性进入 `cancelled`，分别记录 `input_expired` 或 `approval_expired`，投影终态并完成持久 Run，避免无限等待占用执行资源。
- Agent G3 增加语言中立 `dipole.agent.elicitation.v1` 与持久输入恢复链：Temporal `WAITING_INPUT` 固定受限 Form 和 request ID，Gateway 公开 JWT 认证输入 API，Runtime 经 Core 复核 Task principal 后校验精确字段、类型、选项和 16 KiB 上限，再发送 Signal。无效值、跨用户、旧 request 与终态请求 fail closed；Worker 替换后仍可从 Workflow history 恢复同一等待点。Pencil 客户端、敏感输入与 MCP Elicitation adapter 继续关闭。
- Agent G3 增加 migration v29 与默认关闭的 scoped Memory foundation：MySQL/sqlc 以 tenant、Task principal、Agent 和 conversation resource 保存不可变 Working/Episodic/Semantic/Procedural/Observational Memory，记录 full/compact content、priority、有效期和 provenance。受认证 `dipole-agent` 只能用运行中的 Task/Run 请求 Core；Core 从固定 Definition 解析 read permission/scope，并以 Task 创建时间固定可见记录上界，撤销/过期立即失效。显式启用后，TypeScript ModelShadowPlanner 为实际命中的 Memory 分配独立 500-token 预算并以 `untrusted` provenance fragment 注入 Context；空结果保留原 evidence 预算和路径。
- Agent G3 增加 migration v28 与 Event Subscription 确定性触发基础：Core/sqlc 按 Definition version、tenant、Agent、事件和 conversation resource 持久化 `all|message_contains_any` 订阅，受认证 `dipole-agent` RPC 仅返回当前有效 Definition 且具备 read scope 的候选。TypeScript Runtime 可显式选择 `subscription` 模式，在 EventLedger、Temporal 和模型调用前完成严格 schema、资源、事件及 Unicode 关键词过滤；零匹配直接退出，多匹配按 Subscription ID 稳定选择，并把 ID 固定到 Task 防止重放漂移。默认与 Compose 保持 `direct_target`，现有流量不切换。
- Agent G3 增加 Message v1 事件 lineage：可选 `origin`、`causation_event_id` 与 `agent_task_id` 从 Embedded Agent 命令经 Kafka `send_requested`、consumer context 和 Transactional Outbox 传播到 confirmed Message；Agent origin 强制绑定 Task，非法 lineage 在 Go/TS decoder 中 fail closed。TypeScript Shadow Runtime 在领取 EventLedger、启动 Temporal 或调用模型前返回 `suppressed`，阻断同一 Agent 因果链的循环触发；不带 lineage 的 legacy v1 事件保持兼容。
- Conversation State 增加低基数成功写 Counter 和 `dipole_conversation_projection_write_duration_seconds{projection,outcome}` Histogram，区分 direct message、group message 与 group init 的 Repository 调用耗时和错误；端到端基准升级为兼容 v1/v2 的 operations/baseline v3，逐节点保存前后 Prometheus 快照并拒绝 Counter 回退或成功次数漂移。20/100 人普通群和热群写放大证据位于 `benchmarks/ad005-2026-08-27/`，projection timing 与原始快照位于 `benchmarks/ad005-projection-timing-2026-08-27/`。
- Go 长运行服务增加缓存型动态依赖 readiness：按服务关键依赖矩阵周期探测 MySQL、Redis、Kafka、内部 gRPC、Elasticsearch 与 Cassandra，使用超时和失败/恢复双阈值避免瞬时抖动；HTTP readiness 与 gRPC health 同步摘流和恢复，Prometheus 可按服务/依赖告警。微服务 Compose 默认启用，Elasticsearch 隔离演练验证 Search 局部摘流且应用容器不发生级联重启。
- Agent Context 校准增加语言中立的 evidence/report v1 与离线 `context:calibrate` 命令：输入仅接受 synthetic corpus，要求每个 model route 覆盖中英文、代码、Emoji 与 Tool schema 五类语料，并绑定候选提交、provider/model/tokenizer revision 和 reference Token；报告逐 route 输出正文 SHA-256、估算误差、fallback 与 underestimate，原文不回显，evidence/report 均可复算哈希。完整 profile 且零低估返回 0，校准阻断返回 2，契约错误返回 1；命令不访问模型、网络或运行时配置。
- TypeScript Agent Context Compiler 增加显式启用的 route-aware Token estimator v1：候选模型 route 可声明 context window、UTF-8 bytes/token 校准值和 basis-point 安全余量，编译前按全部 fallback route 取最大 Token 估算与最小窗口；缺少声明时使用固定 `8192 / 2 bytes / 25%` 保守 fallback。Compiler v2 将配置导出的 SHA-256 estimator ID 写入不可变 Context manifest，窗口无法容纳固定 4096 输入预算与单次最大输出时在配置阶段 fail closed；默认与 Compose 继续固定 Compiler v1，避免在途不可变 Plan 重放发生哈希漂移，模型路由和流量权威不变。
- Agent Artifact 增加语言中立的 maintenance authorization/receipt v1 与离线命令：授权绑定 reconcile SHA-256、单个对象证据、两位独立审批人、独立 proposer/executor、grant version 和最长 15 分钟有效期；evaluate 重新执行对象 Stat 与 sqlc 元数据查询，输出 `would_delete` 或四类阻断 receipt。Schema 拒绝附加字段，授权固定 `delete_adapter_available=false`，receipt 固定 `delete_attempted=false`、`deleted=false`。
- Agent Artifact 增加 `dipole.agent.artifact.reconcile.v1` 离线 dry-run：独立只读 MinIO 身份固定列举 `agent-artifacts/v1/`，对象满 24 小时后才查询 sqlc 元数据并形成孤儿候选；异常键只产生不可清理告警。报告固定 `delete_authorized=false`、有界样例和可复算 SHA-256，命令不包含删除参数或删除 client。
- Agent G3 增加 `dipole.agent.artifact.v1` 与 migration v26：Temporal `read_shadow` 将持久模型摘要输出为确定性 Markdown Artifact，Core 校验 Task/Run、Shadow 模式、版本与跨语言 SHA-256 后把不可变元数据写入 MySQL、正文写入内容寻址 MinIO；精确重试重新验证对象并收敛到同一 Artifact。`dipole-agent` 仅能创建，Gateway 仅能按 Task principal 读取，协议不提供更新、删除、公开 URL、消息发送或 active 写权限。
- Agent G3 增加语言中立的 Workflow repair execution plan v1：当前只允许 `dry_run`，固定 approved Proposal、proposer、两位 approver、独立 executor grant version、当前/目标/回滚投影和三组 SHA-256 CAS 证据，并以 15 分钟有效期约束重新采证。生产 protobuf 继续没有 apply/execute/rollback 方法，实际写执行器需发布新契约版本并单独评审。
- Agent G3 增加 migration v25 与 Workflow repair 审计控制面：Core 以 Gateway 认证 principal 和默认空授权表接收服务端重算 SHA-256 的一小时修复提案，MySQL 不可变保存工单、投影/Temporal 证据和操作员身份；提案人无权审批，每位审批人仅能提交一票，至少两位独立授权操作员批准后才进入 `approved`，任一拒绝进入 `rejected`。API 未提供 apply/execute 方法，`dipole-agent` 服务身份被方法级拒绝，批准结果仍不能改变 Workflow projection。
- Agent G3 增加版本化 Shadow 晋级策略与修复提案 Artifact：候选版本需提供连续 24 小时、至少 24 个观察点、最大 90 分钟间隔、累计至少 100 个 Task 的零差异/零 unavailable 对账，并通过六项 projection Eval 与 outcome/trajectory/permission Eval；`promotion:check` 仅输出 eligible/blocked 证据。`repair:propose` 只生成绑定操作员声明、工单、Temporal 证据、一小时有效期和 SHA-256 的不可执行提案，不开放自动修复或 active 切换。
- Agent G3 增加只读 Workflow projection 离线对账：Core/sqlc 通过服务身份限定的 keyset RPC 枚举 `dipole-agent/shadow` Task 与可空投影，TS Runtime 同时读取 Temporal Query 和 Describe 的实际 Workflow/Run 绑定，输出版本化 `match|missing|stale|ahead|conflict|unavailable` JSON 报告；不一致时命令返回退出码 2，不执行自动修复。版本化 Eval 数据集固定六类结果，真实 MySQL 与 Temporal Server 覆盖分页和完成态对账。
- Agent G3 增加 migration v24 与 Temporal Workflow 状态投影：`agent_tasks` 以独立 nullable workflow binding/status/revision 保存 shadow 观察状态，保留原 Task `status` 权威语义；Core/sqlc 仅接受同一 Workflow/Run 的更高 revision，完全相同写入幂等，旧 revision、同 revision 漂移和身份漂移均 fail closed。Workflow 通过版本化 `patched` Activity 投影 running/waiting/terminal 状态，Gateway Task Query 返回 `match|missing|stale|ahead|conflict` 对账证据且不自动修复。
- Agent G3 增加默认关闭的 Gateway Task 控制桥：公开 `GET /api/v1/agent/tasks/:task_id`、取消与 Approval resolution API 由 Gateway JWT 派生 principal，经固定服务身份与共享密钥调用私有 Runtime；Runtime 每次操作都通过 additive `AuthorizeTaskControl` RPC 向 Go/sqlc Core 校验 Task 所有权，再执行 Temporal Query/Signal。审批同时绑定当前 pending request/approval ID，跨用户、旧审批、终态取消和模型/请求体伪造 principal 均 fail closed；TS 继续无权直接读取 `agent_tasks`。
- Temporal G3 增加默认关闭的 `read_shadow` 执行模式与 migration v23：Kafka Shadow 保留独立 consumer/EventLedger 和事件触发责任，成功启动稳定 Workflow 后由 Temporal Activity 执行 ContextCompiler、持久 ModelRouter、不可变 Plan 与首条只读 Capability 轨迹。成功模型调用保存 Zod 校验后的结构化输出，Activity 在 provider 返回、Plan 或 Step 完成后丢失 ACK 时可恢复同一输出和已完成 Step，不能重复付费调用或 Tool 副作用；Task/Run/admission/event 任一绑定漂移均 fail closed。
- Temporal G3 增加持久 Approval Signal：`wait_approval` 在进入等待前通过 additive `RequestApproval` RPC 保存 capability、resource scope、arguments、nonce 与过期时间绑定；Signal 必须同时匹配 request/approval ID，并由 `ResolveApproval` 校验运行中的 Task/Run 和持久 Task principal，Core 完成 approved/revoked 后 Workflow 才恢复。创建、批准、拒绝和并发网络重放均收敛，默认 `foundation` 与现有 Kafka/模型/Capability 流量保持不变。
- Temporal G3 增加默认关闭的持久 shadow 生命周期 Activity：Workflow 在 Step 前通过受认证 Capability RPC 建立确定性 Task/Run admission，并在 completed、failed、cancelled 后提交精确终态证据；Core 新增 additive `FinishRun` RPC，以 Task/Run/runtime/mode 绑定、compare-and-set 和终态证据实现网络重试幂等。`DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=foundation|persistent_shadow` 默认保持 `foundation`，Kafka、模型、Capability、Approval 与权威 Task 状态均未切流。
- TypeScript Agent Runtime 增加 Temporal G3 foundation：以持久 Agent Task ID 生成稳定 Workflow ID，运行中重复启动使用 `USE_EXISTING` 收敛、终态使用 `REJECT_DUPLICATE` 阻止重放；框架中立状态机覆盖 running、waiting_input、waiting_approval、completed、failed、cancelled，Workflow 提供输入/审批/取消 Signal 与状态 Query，模型、Capability、存储和副作用统一隔离到带三次有界重试的 Activity。Worker 配置默认关闭，Compose 显式保持零流量切换，foundation Activity 不产生外部副作用。
- TypeScript Agent Runtime 增加框架中立 `ContextCompiler` v1 与 migration v22：策略、身份、Task、事件证据和 Capability 以 system/trusted/untrusted JSON record 编译，按全局及 section Token 预算确定性选择 full/compact/omit，必需上下文超预算时 fail closed；Plan 审计持久化 compiler version、估算 Token、selected/omitted ID 和 provenance，不保存额外上下文正文。ModelShadowPlanner 只接收编译后的 prompt，模型 adapter 与上下文策略保持隔离。
- Agent Runtime 增加 migration v21、独立 `agent_runs` 生命周期与首个受认证远程 Capability：Core 通过 `dipole.agent.v1.AgentCapabilityService` 完成确定性 Task/Run admission、持久 Definition 快照解析和 `conversation.list` 授权，TS Runtime 使用静态生成的 protobuf gRPC client、mTLS `dipole-agent` 身份与 Capability Registry 执行只读 Step。Step 通过 lease/token 持久 claim/result/error，旧 owner 无法覆盖重领结果；Run 完成接口支持网络重试幂等，模型仍不能提交 principal，Agent 身份也无法调用未授权 Core RPC。
- Agent Runtime 增加 migration v20 与持久 Shadow Plan/Step 轨迹：模型输出升级为有序 `steps[]`，每步固定 capability ID 与结构化输入；Task Plan 以 SHA-256 不可变快照保存，Kafka 并发重放幂等收敛，计划漂移 fail closed。MySQL ledger 模式在 Kafka readiness 前探测轨迹表，最小账号仅获两表 `SELECT, INSERT`。
- Agent Runtime 增加 migration v19 与 MySQL ModelAuditStore：Task 唯一绑定持久 Run 和不可变预算快照，ModelRouter 在调用 provider 前事务预留 call slot，16 路并发及跨 Kafka 重投均严格受 `max_calls` 限制；调用完成/失败记录 route、Token、finish reason、latency 和错误，崩溃遗留 slot 在 Run 终止时收敛为 `abandoned`。AI SDK 模式强制持久 Store，Agent 最小账号仅新增两张审计表的 SELECT/INSERT/UPDATE。
- TypeScript Agent Runtime 增加 provider-neutral `ModelRouter` 与 AI SDK 结构化输出 adapter：按有序 route 降级，失败调用计入每 Run 调用上限，总 deadline 与单次输出 Token 上限由 Runtime 强制传递，AI SDK 隐式 retry 关闭；`metadata` 仍为默认模式，模型 plan 仅允许只读 capability 白名单并记录 route/attempt/token usage。
- Agent Runtime 增加有界 Kafka 失败转移：`dipole.<topic>.retry/.dead` 显式创建并校验拓扑，无效 envelope/tombstone 直接死信，处理错误最多尝试三次且保留原始 key/value/header；publisher 失败时 handler 拒绝完成。真实 Kafka 3.9 已验证 poison、retry→dead、offset LAG 归零和双副本 rebalance。
- Agent Runtime 增加 migration v18 与 MySQL EventLedger：Event ID/Task ID 双唯一、事务 claim、lease crash recovery、attempt 和精确 token 终态；微服务默认使用最小权限 `dipole_agent`，真实 MySQL 8.4 与 Kafka 3.9 验证并发单 owner、失败重领、旧 owner 拒绝及 Runtime 重启后重复事件收敛。
- TypeScript Agent Runtime 增加 `message.direct.created` v1 decoder、KafkaJS 独立 shadow consumer、稳定 Task ID 和 EventLedger port；冷启动 metadata 未收敛时会断开旧客户端并有界重连，微服务 Compose 可独立启动只读 Agent，真实 Kafka 3.9 重放同一事件只产生一条 metadata plan。
- 增加独立 `services/agent-runtime/` TypeScript foundation：Node 22+、Fastify 5、Zod 4、AI SDK 7、KafkaJS 2，提供 trusted ExecutionContext、Go 兼容 Task ID、Capability Registry、resource-scope Policy Engine、shadow 写隔离和 `/livez`/`/readyz`；模型路由与持久审计留待 G2 后续切片。
- Embedded Agent 增加持久执行策略：`ai.policy_mode=persistent` 默认从版本化 Definition 创建确定性 AgentTask、固定并重读精确 policy version，Invocation 携带 permission/resource scope；`static` 保留显式回滚，Task 以 compare-and-set 进入 completed/failed。
- 增加版本化 Web Sync 真实观察 Session/Evidence 与 `web_sync_observation.py`：`start/status/finalize` 将候选版本、完整 Git commit、实际发布 bundle SHA-256、初始/最终 Prometheus 原始响应和 24 小时门禁绑定为不可覆盖证据；窗口不足、候选漂移、告警、差异或溢出均 fail closed，blocked 窗口仍保留审计结果且不会自动切换客户端或 Cassandra 路由。
- 增加 `dipole.agent.policy.persistence.v1`、migration v16 与 sqlc `AgentPolicyStoreV1`：版本化 Definition 保存 permission/scope/有效期/撤销状态，AgentTask 固定 Definition version 与 principal 并以 compare-and-set 迁移状态，Approval 支持 pending→approved、撤销及绑定 capability/canonical scope hash/arguments hash/nonce/有效期的一次性消费；真实 MySQL 8.4 并发测试要求 16 个竞争者仅一个成功。
- 增加 `dipole.agent.command.v1` 跨语言消息写契约与本地 adapter：普通回复和系统 Tool 统一经过 AgentPolicy、Message Service、Kafka/Outbox；sender/target 取自可信 Invocation，Command ID 以固定 SHA-256 canonical form 映射到 64 字符 Message 幂等键，并以黄金向量约束未来 TypeScript 实现。
- 增加 `AgentPolicyV1` 与跨语言 capability descriptors：可信 Invocation 携带 tenant、principal、Agent、delegator、permissions、approvals 和 correlation IDs；read/write 需要显式 permission，destructive/敏感能力需要审批，Embedded Tool 与本地 Capability adapter 双层 fail closed，并以 `AD-027` 跟踪持久授权和审批状态。
- 增加 `ai.runtime_mode=off|embedded|shadow|remote`：未配置时兼容 `ai.enabled`，非法值 fail fast；shadow 保持 Go/Eino 权威并预留 TS 独立 consumer group，remote/off 在模型与 Capability 依赖构造前停止 Embedded consumer，微服务 Core 默认显式 `off`。
- 增加 `dipole.agent.capability.v1` 跨语言契约、Go application port 与本地 adapter；Agent ContextBuilder/Tool 停止持有 User/Message/Conversation repository-shaped 依赖，资料、上下文、会话读取和系统消息统一经过 Core/Conversation/Message 应用边界，写命令保留 correlation context。
- 增加 Embedded Agent `ExecutionContext`：由触发 Message 和 correlation context 注入 principal、Agent、会话及 request/trace/event ID；五个 Tool schema 移除模型可控 `user_uuid`，缺少可信上下文或发送 Agent 不匹配时拒绝执行，并以原 `AD-008` 越权输入持续回归。
- 增加语言中立 `dipole.agent.eval.v1` 与 Go/Eino baseline adapter 测试，覆盖 Agent 事件过滤/幂等、普通与 Tool 回复轨迹、会话读取授权，并将模型身份覆盖行为显式绑定到 `AD-008`。
- 增加 canonical 架构文档 manifest 与 Git 跟踪门禁，移除 `docs/*.md` 通配忽略；历史和本地参考资料改为显式单文件忽略，避免旧文档被误当作当前实现契约。
- 增加 G0 可复现端到端性能门禁与版本化报告，覆盖 direct、concurrent、普通群和热群的接受/持久化/投递率、P50/P95/P99、Kafka lag 及 Inbox 写放大；Compose 热群阈值可在基准期间受控覆盖，默认仍为 `200/50`。
- 增加统一服务健康面：Core、Gateway、Message、Sync、Search、Search Indexer 与 Cassandra Projector 通过 metrics listener 暴露 `/livez`、`/readyz`、兼容 `/health`、`dipole_service_info` 和 `dipole_service_ready`；微服务 Compose 使用 readiness 探针，Prometheus 增加必需服务 down/not-ready 告警及 promtool 时序测试。
- 增加统一关联上下文：HTTP Core/Gateway 生成并回传 `X-Request-ID`、`X-Trace-ID`，gRPC metadata/protobuf、WebSocket 命令与 ACK、Kafka Envelope/headers、consumer handler 和 Transactional Outbox 保持同一 request/trace 因果链；每个领域事件独立生成 `event_id`，旧接口和旧事件仍保持兼容。
- 增加 Group、Conversation Read、Contact 与 Session v1 语言中立事件 schema 和公共 decoder；Gateway 停止自行解码这些 payload，并新增门禁保证所有 `kafkaManagedTopics()` 都有唯一版本化契约。
- 增加 `contracts/events/message/v1` 语言中立事件契约，分别描述 pre-persistence `send_requested` 与 confirmed Message fact；统一 Gateway、Cassandra、Sync、Search、Backfill 和 Replay 的 v1 decoder，并验证 legacy created 默认值、minor additive 字段、事件通道与 producer schema drift。
- 增加真实 Vue 应用共享设备 E2E：在生产默认 IndexedDB 同时保存 U1/U2，通过 Axios 401 和 WebSocket `session.kicked` 两条生产链路终止 U1；Chromium、Firefox、WebKit 均验证凭据与 U1 被清理、U2 Message/Cursor 保留。
- 增加 `scripts/check-web-sync-real-quota.sh`：在无特权 128 MiB tmpfs 中运行独立 Chromium profile，以随机不可压缩正文触发真实 IndexedDB 容量拒绝；释放 reserve file 后验证失败页未推进安全 Cursor，Message 与 manifest 保持一致。
- Web IndexedDB 验收增加真实 Chromium 主进程强退场景：独立 persistent profile 在生产 `commitPage` 仍 pending 时触发 `Browser.crash`，以同一 profile 重启后验证 Message、manifest 与安全 Cursor 只能整页提交或整页回滚。
- 增加版本化 `sync.item.notify.v1` WebSocket 轻量通知，固定只携带 event、Message UUID、会话 key/Seq 和目标 locator；`message.timeline_notify_mode=off|shadow` 默认关闭，shadow 保留现有完整消息投递并附加通知，热群继续只使用聚合 notify + pull。
- Web 增加默认关闭的 Timeline notification shadow verifier，按会话串行使用 `after_seq` 补拉，覆盖通知丢失、重复、乱序、缺行和 UUID 冲突；仅上报 `match|missing|mismatch|error|invalid` 有界聚合指标，并配套 24 小时、至少 100 次 match、零失败的 Prometheus 晋级门禁。
- Direct Message Timeline 增加 `after_seq` 增量查询，HTTP、Message v1 gRPC、Local/Remote/Shadow adapters 与 Cassandra cohort 路由保持同一语义；字段以 protobuf 追加方式兼容旧调用方，为在线 `sync.item.notify` 拉取路径提供前置契约。
- 增加 Playwright Chromium/Firefox/WebKit IndexedDB 验收，直接运行生产 Store 与 Session Terminator，覆盖容量淘汰、关闭重开、账号隔离、延迟清理和页面中断事务原子性。
- Web Sync 聚合遥测增加 `dipole_web_sync_client_errors_total{outcome}`，仅允许 `storage_full|sync_error`，并新增浏览器存储不足、客户端恢复错误告警及 promtool 固定时序测试。
- 增加 `message.mysql.*` 专用数据库配置、atomic/projector 两套最小权限账号与操作级启动门禁；微服务 Compose 在 migration 后初始化授权，并默认使用专用 Message/Sync 凭据。
- 增加 `dipole-cassandra-archive` 与不可变完整消息快照：按固定 MySQL Message 高水位导出完整字段 NDJSON、SHA-256 manifest，并支持 MinIO object-lock 发布、固定 Version ID 恢复。
- Cassandra Backfill/Reconcile 增加 `mysql|archive` source selector；migration v15 将 Job 绑定到 source kind、snapshot ID 与 hash，同名 Job 续跑、完成后复核均拒绝换源或篡改。
- Storage 配置增加 `message_archive_bucket` 与 `message_archive_retention_days`，Compose 初始化独立 `dipole-message-archives` bucket、版本控制和 30 天默认 Governance 保留。
- 增加默认关闭的 `message.cassandra_duplicate_hydration`：重复发送先用 Metadata locator 从 Cassandra Timeline 恢复原响应，命中时不读取 MySQL 正文；缺行、查询错误、位置冲突或历史 Seq 缺失时回退 MySQL。
- 增加 `dipole_message_duplicate_hydration_total{outcome}`，预注册 `hit|fallback|skipped_no_seq` 三种有界结果，用于评估幂等响应正文退役条件。
- 增加重复消息 Cassandra hydration 的 24 小时 recording rules、fallback/no-seq 告警、promtool 固定时序测试与灰度手册；晋级要求至少 100 次 hit 且 fallback/no-seq 均为零。
- 增加 `dipole-search-outbox-cleanup` 受控清理命令：默认 dry-run，只接受已验证对象归档 receipt、一致 Reconcile 报告和匹配 Backfill Job；执行模式强制维护窗口确认与 operator，并输出对象版本、高水位和删除统计审计字段。
- 发布镜像增加 `dipole-search-archive` 与 `dipole-search-outbox-cleanup` 运维二进制，归档恢复和受控清理可直接在同一版本镜像中执行。
- 增加 `search.mysql.*` 专用维护连接与最小授权模板；账号仅可访问 migration ledger、Search Backfill Job 和 Outbox，Core 业务表保持拒绝。
- `dipole-search-archive` 增加 `publish|restore`：归档发布到启用 versioning/object lock 的独立 MinIO bucket，receipt 固定 object version ID、ETag 和 Governance 保留截止时间；恢复只读取指定版本并重新校验 hash。
- Storage 配置增加 `search_archive_bucket` 与 `search_archive_retention_days`，Compose 初始化独立 `dipole-search-archives` bucket、版本控制和 30 天默认保留。
- 增加 `dipole-search-archive` 与不可变 Search snapshot：按固定 Outbox mutation 高水位流式导出最终状态 NDJSON 和 SHA-256 manifest；Backfill、Reconcile、Alias 可统一选择 `mysql|archive` 源。
- 增加 migration v13，将 Search Backfill Job 绑定到 source kind、snapshot ID 与 hash；恢复、对账和 Alias 操作拒绝换源、篡改归档或高水位不一致。
- 增加 migration v12 `message_metadata`：在 Message、Inbox 与 Outbox 同一事务保存幂等 locator、会话 Seq、发送目标、文件绑定、过期时间和版本化 payload SHA-256；历史消息回填 locator，旧 payload hash 明确为空。
- 重复发送优先按 Metadata 的 `(sender_uuid, client_message_id)` 定位并校验目标与 payload hash；文件下载/内容授权改查 Metadata，完整 `messages` 行删除后仍可执行访问判断。
- Web 热群恢复增加独立 `GroupMessageSyncEngine` 与 IndexedDB v3 群位点：补拉消息和 `message_seq` 原子落库后才展示并 ACK 设备群 checkpoint；刷新后优先恢复本地群消息，并可补交丢失的 ACK。
- Web IndexedDB 升级到 v2，增加逐用户缓存 manifest、5000/4000 默认高低水位和按会话保底的最近消息淘汰；超量页面在同一事务内压缩到低水位，同时保留完整安全 `sync_seq`。
- Web Sync 增加 `storage_full` 状态和 Pencil 批准预览；浏览器拒绝持久化时提示释放空间后重试，未落库页面不会确认设备 Cursor。
- 增加默认关闭的 Web Sync Engine：按用户将 Sync 消息与安全 `sync_seq` 原子写入 IndexedDB，启动和 WebSocket 重连时从本地恢复并分页追平 `/sync`，仅在本地事务完成后确认设备 Cursor。
- 增加同步恢复的 Pencil 状态矩阵、desktop/mobile 恢复页面和 Vue 标题栏状态，覆盖恢复中、已同步、离线可读与可重试中断语义。
- 增加 Web `off|shadow|primary` 同步模式；shadow 保持旧 Offline 驱动界面，同时持久化 `/sync` 并按收到的私聊 Message UUID 做带宽限期的有界对照。
- 增加认证聚合遥测 `POST /api/v1/sync/comparison` 与 `dipole_web_sync_comparison_total{scope,outcome}`，只接收 baseline、match、pending、legacy_only、sync_only、overflow 计数，不接收消息 ID 或正文。
- 增加 Web Sync 24 小时 rolling recording rules、终态差异/比较器溢出 critical 告警和 promtool 固定时序测试；六种 outcome 启动时暴露零值并拒绝未知标签，晋级门槛为至少 100 个 match、零 `legacy_only/sync_only` 与零 overflow。
- 增加 Web Sync 灰度与观测手册，明确 Core metrics 抓取前提、停止条件、真实证据归档、primary 晋级和无数据回切步骤。
- 增加 Sync 专用 MySQL 配置覆盖、`dipole_sync` 最小授权模板和启动权限门禁，精确验证 Inbox/Checkpoint/恢复表读写及 Message/Core 越权拒绝。
- 增加 `message.inbox_write_mode=atomic|projector`；独立 Message owner 可停止 Inbox 写入并保留 Message、Conversation Seq、群高水位与 Transactional Outbox 原子提交，模块化单体继续固定使用 atomic 路径。
- 增加 `dipole_message_projector` 最小授权模板、Sync mTLS 证书身份和 Compose 运行时定义。
- 增加 Sync Projector 专用 Prometheus lag、retry、DLQ 告警及 promtool 时序测试，精确绑定 `dipole-sync-consumer` 与 created-event 失败 topic。
- 增加 `dipole-sync-baseline` 运维命令与 migration v11，按固定 Inbox `sync_seq` 高水位归档所有缺少 created Outbox 的历史 recipient/locator，保存规范化 SHA-256，并支持精确 Reconcile 与 missing-only Restore。
- Sync 历史基线恢复保留原始 `sync_seq`，发现新增 legacy 行、recipient/locator 偏移或 Cursor 序号冲突时拒绝自动修复，避免按当前群成员关系推断早期收件人。
- 增加真实 MySQL 8.4 Sync baseline smoke，覆盖重复 Capture、差异退出码 2、原 Cursor 恢复、冲突退出码 1 和最终收敛。
- 增加默认关闭的 `sync.projector_enabled`；`dipole-sync` 可使用独立 Kafka consumer group 将私聊和普通群 `message.created` 物化到 Durable Inbox，热群继续遵循 `sync_fanout=false` 的 notify + pull 路径。
- 增加 storage-neutral `SyncProjectionStore` 与 sqlc 事务适配器，按固定用户顺序锁定 Sync state，并以 `message_uuid + conversation_key + message_seq` 实现双运行精确重放和冲突回滚。
- 增加真实 Kafka 三节点与 MySQL 8.4 Sync Projector smoke，覆盖 Message 预写、重复事件收敛和热群无 Inbox 写扩散。
- 增加 `dipole-sync-replay` 与 `dipole-sync-reconcile` 运维命令，以 created Outbox 最大 ID 固定恢复快照，使用 lease/checkpoint 分批重放，并精确核对每条消息的收件人和 locator 集合。
- 增加 migration v10 `sync_replay_jobs`、机器可读一致性报告和退出码 2 差异门禁；真实 MySQL 8.4 演练覆盖部分状态补齐、completed no-op、删行检测和新 job 修复。
- 增加独立 `cmd/sync-service` 查询运行时、`dipole-sync` 内部身份和构建产物，进程只组合 sqlc Sync Store、MySQL schema readiness、Core 群成员授权与 Sync v1 gRPC。
- Core 内部 RPC 为 Sync 身份增加方法级最小权限，`dipole-sync` 只能读取群成员关系；Core/Gateway 可通过绑定自身身份的客户端调用 Sync API。
- 增加默认 `local` 的 `sync.transport=local|grpc` 切流开关；Core HTTP 可通过受认证 gRPC 使用独立 Sync Service，并保留进程内即时回滚路径。
- 增加默认关闭的 `sync.shadow_queries`，异步比较 Inbox、设备 Cursor 和群 checkpoint 只读结果；两类 checkpoint advance 始终只调用选定主实现一次。
- Sync Item 增加存储中立的 `conversation_key + message_uuid + message_seq` 定位契约，HTTP 与 Sync v1 gRPC 以追加字段暴露 locator，同时保留完整 Message 快照兼容旧客户端。
- 增加默认关闭的 `sync.cassandra_shadow_hydration`；Sync Service 继续由 MySQL 补全并返回消息正文，同时按 Inbox locator 异步读取 Cassandra Timeline，记录 match、mismatch、error 和容量跳过结果。
- 增加 `dipole_sync_hydration_shadow_total{outcome}` 与 `dipole_sync_hydration_shadow_duration_seconds{outcome}`，用于评估 Sync 正文补全切换条件；异步比较上限固定为 32，Cassandra 异常不增加主响应等待。
- `user_sync_inbox` 增加会话 Seq 回填和 `(user_uuid, conversation_key, message_seq)` 唯一约束；相同消息或位置的冲突重放会失败，正确重放保持幂等。
- 增加 Vue 消息搜索工作区，支持 desktop/mobile 的结果、加载、空态和局部故障态，以及会话入口、`Cmd/Ctrl+K`、300ms 防抖、乱序响应淘汰和重试。
- 增加 Vitest、Vue Test Utils 与 jsdom 前端测试基线，首批覆盖 Search 状态控制器和工作区交互。
- 增加 canonical Pencil 设计基线，包含消息搜索 desktop/mobile 的结果、加载、空态、错误态和可复用组件，并提供批准预览与持续维护说明。
- 增加 transport-neutral `MessageApplication`、`SyncApplication`、`CoreCapability` 和 `EventPublisher` 端口、单体 Local adapter 及数据层依赖架构测试。
- 增加用户同步 Inbox Timeline：通过 `user_sync_inbox` 按用户维护持久化 `sync_seq`，支持离线和多端增量同步。
- 增加 `GET /api/v1/sync` 接口，支持 `after_seq` 游标、分页上限、`next_seq` 和 `has_more`。
- 消息持久化、Inbox 写入与 Transactional Outbox 进入同一数据库事务，避免消息事实与同步状态分离提交。
- 增加同步链路的 repository、service 和 HTTP handler 测试，并覆盖私聊、普通群聊、热群和分页场景。
- 增加分阶段平台演进计划与架构债务台账，明确微服务、存储架构和 Agent Runtime 的分支、验收及回滚策略。
- 增加 TypeScript Agent Runtime 设计，明确 Durable Task、Capability、Context、Memory、MCP、评测和渐进迁移方案。
- 增加 GORM 到 sqlc 的渐进迁移计划，以及基于 Pencil `.pen` 的前端设计与视觉回归维护规范。
- 增加版本化 MySQL migration、独立 `cmd/migrate` runner、schema ledger 与真实 MySQL drift 测试。
- 增加固定 sqlc `v1.31.1` 的生成配置与漂移门禁、`database/sql` 事务 Store、错误映射及首组 AICallLog 类型安全查询。
- 增加 `AICallLogStore` application port、GORM 可注入 adapter、sqlc adapter 及共享 MySQL contract test。
- 增加 `AdminOverviewStore` application port 与 Admin sqlc adapter，以单条聚合查询替代九次独立统计查询，并通过 GORM/sqlc 共享契约验证。
- 增加 `FileMetadataStore` application port 与 File sqlc adapter，保持创建后的 ID/时间戳回填和缺失查询语义，并纳入统一回切开关。
- 增加 `UserStore` application port、共享 Redis/Bloom 缓存装饰器与 User sqlc adapter，覆盖创建、助手 upsert、资料更新、筛选和批量查询。
- 增加 `ContactStore` application port、共享关系缓存装饰器与 Contact sqlc adapter，覆盖双向好友关系和联系人申请生命周期。
- 增加 `GroupStore` application port、共享元数据/成员缓存装饰器与事务型 Group sqlc adapter。
- 增加 `ConversationStore` application port 与 Conversation sqlc adapter，覆盖投影 upsert、列表、初始化、备注和未读状态。
- 增加 `OutboxRelayStore` application port 与事务型 sqlc adapter，覆盖有序批量领取、过期租约回收、重试退避、发布终态和 Header 解码。
- 增加 `MessageStore`、`SyncStore` application port 与 sqlc adapters，完整覆盖 Message Store 查询、Inbox Timeline 读取和 Message/Inbox/Outbox 原子写入。
- 增加 `dipole.message.v1.MessageService` protobuf/gRPC 契约、固定版本生成门禁、结构化领域错误详情，以及实现同一 `MessageApplication` 的本地 server 与远程 client adapters。
- 增加共享 `dipole.common.v1.RequestContext`、Core Capability 与 Sync Query v1 契约，以及实现现有 application ports 的 Local server/Remote client adapters。
- 增加 Kafka schema version 兼容校验与事件契约，legacy 空版本和 v1 minor 保持兼容，未知主版本跳过业务 Handler 并直接进入 DLQ。
- 增加内部 gRPC 服务身份认证基元，客户端注入服务名与共享凭据，服务端以常量时间比较校验凭据并执行调用方 allowlist。
- 增加独立 `cmd/message-service` 运行时，拥有 Message RPC、消息持久化 Kafka consumer、Transactional Outbox Relay 和远程 Core Capability client。
- Core Capability v1 增加最小文件所有权快照，文件消息通过受认证 RPC 获取 File ID、名称、大小、类型和 URL，不暴露对象存储内部键。
- 增加仅组合 Message 与 Outbox adapters 的 `MessageProcessRepositories`，独立进程停止构造 User、Contact、Group、File、Conversation、Admin 和 AI Repository。
- 增加 `message.shadow_queries` 查询影子开关，可在 Local/Remote 任一方向异步比较历史、群增量和离线消息结果。
- 增加 `message.runtime_mode=shadow|owner`；shadow 进程拒绝全部命令且不启动 Kafka/Outbox，owner 进程承担完整消息写入职责。
- 内部 RPC 支持 TLS 1.3 双向证书认证，并将认证服务身份绑定到 protobuf `caller_service`；明文 listener/target 强制限制在 loopback。
- 增加 `message.enforce_db_permissions` 启动验收和 Message MySQL 最小授权模板，检查自有表可访问且 Core 表全部拒绝。
- 增加 Message Service 渐进部署手册，覆盖影子阶段、consumer group 交接、mTLS、验收和无数据回滚流程。
- 增加可滚动维护的性能基线，首组记录 Local 与 TLS 1.3 mTLS gRPC Message History adapter 的三轮 benchmark。
- 增加独立 `cmd/gateway`，承担公开 HTTP/WS、认证上下文、限流、连接管理、Redis Presence 与 Kafka Realtime Delivery，运行时不初始化 MySQL 或 Repository。
- 增加 IM Gateway 渐进部署手册，覆盖三进程边界、mTLS 身份、灰度验收和 `embedded` 无数据回滚。
- 增加最小微服务开发拓扑：统一镜像打包 Core、Message、Gateway 与 migration，Compose 默认启用内部 mTLS 和依赖健康门禁。
- 增加内部开发 CA/三服务证书生成脚本及可自动清理的微服务 cold-start smoke。
- 增加会话内单调 `message_seq` 与 `conversation_sequences` allocator；消息唯一 ID 继续承担全局身份，Seq 专门承担会话内排序。
- Message v1 protobuf、Kafka created event 和 WS chat payload 以追加字段暴露会话 Seq，旧 producer/client 缺字段时保持兼容。
- 增加 Conversation `last_message_seq/read_seq`、设备级 Sync checkpoint 查询与显式 ACK API，并将 checkpoint RPC 加入 Sync v1 契约。
- Web 客户端生成持久稳定的设备 ID，同时用于 HTTP `X-Device-ID` 与 WebSocket Presence 身份。
- 增加存储中立的会话 Seq 历史查询与 `SearchIndex` 契约，MySQL 搜索投影实现支持幂等更新、删除和限定会话范围检索。
- 增加热群持久高水位、设备群 checkpoint 和 `after_seq` 补拉；Web 重连后可发现并追平离线期间跳过用户 Inbox 的热群消息。
- 增加消息 mutation 事件契约；created Outbox 固化 `mutation_type/revision/actor_uuid`，并为未来 edited/recalled/deleted 预留单调 revision 语义。
- 增加三节点 Kafka KRaft cluster profile、显式 Topic min ISR/retention、可配置 producer ACK 策略与自动清理的 quorum 故障 smoke。
- 增加显式 Kafka consumer rebalance policy、处理/提交/retry/DLQ snapshot，以及双 member 到单 member 的 partition 接管故障 smoke。
- 增加进程级 Kafka Prometheus Collector、独立 metrics listener、Kafka exporter、Prometheus 告警规则与自动故障 smoke，覆盖 lag、ISR、retry 和 DLQ。
- 增加 MySQL 8.4 三成员 InnoDB Cluster、MySQL Router writer endpoint、AdminAPI 初始化/恢复脚本与连接池主切换故障 smoke。
- 增加 Redis Sentinel 连接模式、三节点 Redis/三 Sentinel 隔离拓扑与自动故障 smoke。
- 增加 Cassandra 5.0.9 与 Elasticsearch 9.5.2 隔离 Storage Lab、资源基线与自动 CRUD smoke。
- 增加 Cassandra Conversation Timeline 版本化 CQL、10,000 Seq bucket 规则和 LWT 幂等投影 primitive。
- 增加默认关闭的独立 Cassandra Projector Runtime、专属 Kafka consumer group、schema readiness 与端到端 smoke。
- 增加独立 Cassandra 历史 Backfill、固定 MySQL 高水位、owner lease、批次 checkpoint 和失败恢复烟测。
- 增加独立 Cassandra Reconciler，以 JSON 报告固定快照的数量、全量 payload hash、确定性字段样本和会话 Seq 连续性；确认差异时返回退出码 2。
- 增加默认关闭的 `message.cassandra_shadow_reads`，Message Service 可异步比较 MySQL 历史页与 Cassandra Timeline，客户端响应继续取自 MySQL。
- Cassandra Timeline 增加跨 bucket Seq 区间读取，并按升序合并结果供影子比较使用。
- 增加 `message.cassandra_read_percentage`，按会话稳定 cohort 将群 `after_seq` 增量读取灰度到 Cassandra，异常或 Seq 缺口自动回退 MySQL。
- 增加 Cassandra 读路由 Prometheus 请求计数与延迟直方图，以及真实 MySQL/Cassandra 缺行回退 smoke。
- Direct/Group 历史增加 `before_seq` HTTP 与 Message v1 RPC 游标，支持按会话 Seq 获取最新页和向前分页。
- Cassandra cohort 主读扩展到 Direct/Group `before_seq`，连续性校验失败时按同一游标整页回退 MySQL。
- 增加 `message.cassandra_read_verify_percentage` 主读抽样核验；按同一 Seq cursor 比较 MySQL 公开字段，payload mismatch 自动整页回退。
- 增加 `dipole_message_read_verification_total{operation,outcome}`，区分主读核验 match、mismatch 与 MySQL error。
- 增加 Cassandra 主读 Prometheus 告警与 promtool 规则测试，覆盖 payload mismatch、核验依赖失败和持续高 fallback 比例。
- 增加 MySQL 消息正文退役门禁，覆盖 Sync 补全、幂等回放、文件授权、搜索重建、备份与事件回放责任。
- 增加 Elasticsearch `dipole-messages-v1` strict mapping、read/write Alias、schema readiness 与原子 Alias 切换契约。
- 增加 Elasticsearch Search adapter，以 Message UUID、external revision 和 payload hash 识别幂等重放、旧事件与同版本冲突，并强制 conversation scope 查询。
- 增加版本化 `SearchIndex.Apply` 与持久 tombstone，MySQL/Elasticsearch 共享更高 revision 覆盖、旧 revision no-op、相同重放和同版本冲突语义。
- 增加默认关闭的独立 `cmd/search-indexer`、Elasticsearch 配置/认证、专属 Kafka consumer group 与八类 direct/group mutation Topic 投影。
- 增加 Outbox 固定快照 Search Backfill、目标索引绑定的 owner lease/checkpoint、最终 mutation 状态折叠与独立 Reconcile JSON 报告。
- 增加显式 Elasticsearch 物理构建目标；维护写入不绑定生产 Alias，在线 Indexer 继续强制 `require_alias=true`。
- Core Capability v1 增加由认证 principal 派生的 Search 会话范围；调用方无法提交 user ID 或 conversation keys，陈旧群会话投影无法绕过成员关系校验。
- 增加独立 `cmd/search-service`、`dipole.search.v1.SearchService` 与只读 Elasticsearch readiness；查询进程通过 Core scope 限定结果且不初始化 MySQL、Redis 或 Kafka。
- 微服务 Compose 增加可选 `search` profile、`dipole-search` mTLS 身份及 Search Service 隔离存储契约。
- Core 内部 RPC 增加 Search 身份的方法级最小权限，`dipole-search` 只能读取 Search scope，其他 Core capability 被拒绝。
- Storage Lab 架构门禁收窄为检查 Core、Message、Gateway 直连，允许独立 Search 运行时使用 Elasticsearch。
- 增加默认关闭的 Gateway 认证搜索 API `GET /api/v1/messages/search`；principal 来自 JWT 会话，查询参数只包含 `q` 与 1..100 的 `limit`。
- 增加 `dipole-search-alias` 受控切换/回滚命令，要求维护窗口确认、新鲜快照三重检查、现场 Reconcile、Alias owner CAS 与切换后自动补偿。

### 变更
- Agent release manifest 增加单步阶段转移校验，仅允许 `offline <-> shadow <-> user_gray` 相邻推进或回滚，禁止跨阶段跳转并保持旧 manifest 不可变；阶段转移不会自动开启生产流量。


- Sync 增加默认关闭的 `sync.cassandra_primary_hydration`：启用后按同一 `conversation_key + message_seq + message_uuid` locator 优先从 Cassandra 补全消息，查询失败立即回退 MySQL；与 `cassandra_shadow_hydration` 互斥，默认配置、旧 Offline 和 MySQL 主读行为保持不变。
- 前端 F4 增加 `.pen` Foundations 到 Vue 的 token 映射：全局 `--dp-*` CSS token 覆盖颜色、字体、间距和圆角，App 壳层与 Search 工作区开始复用；Vitest 直接读取 canonical `.pen` variables 校验实现值，避免设计稿与页面样式静默漂移。
- Eino 从 `v0.9.15` 升级至 `v0.9.17`；保持 OpenAI 扩展 `v0.1.13` 不变。Go 全量测试、sqlc drift、Agent Runtime 测试、typecheck 与 build 均通过，未发现 API 兼容性回归。
- 更新正式技术架构图以匹配当前实现：补充 Core/Message/Gateway/Sync/Search/Agent Runtime 边界、`user_sync_inbox` Sync Timeline、sqlc 数据访问及 Cassandra/Elasticsearch 影子投影；移除 `AutoMigrate`、单体无 Inbox 和旧 Eino 主链路等过时描述。该图仅记录当前已实现或明确默认关闭的能力，不改变运行配置。

- Core RPC Agent 方法 allowlist 补齐 readiness evidence publish/resolve；此前真实 RPC 部署会在 MCP egress freshness 查询时返回 `PermissionDenied`。全栈演练中的 subscription、Run、Workflow projection、MCP Invocation/Round、readiness 和 Artifact 均改由正式 TS `AgentCapabilityRPCClient` 访问隔离 Go mTLS fixture。
- 外部 MCP 全栈演练脚本不再内嵌两字段 JSON 判断，统一调用 Runtime 契约校验 CLI；证据创建和离线复核共享相同 canonical hash、时间与成功不变量。
- 外部 MCP fresh-readiness egress gate 现在除双 binding、hash 和时间结构外，还要求 `expiresAt` 严格晚于 Worker 当前时钟；Worker 复用已有可注入时钟完成每次连接前校验，过期证据在 raw Registry、Catalog 和网络访问前拒绝。
- 前端开发工具链升级到 Vite 8.2.2、Vitest 4.1.11、plugin-vue 6.0.8 和 Rolldown 1.2.6，并固定 Node 22.12+ LTS；配置改用 `import.meta.dirname`，隔离测试可通过 `DIPOLE_WEB_PROXY_TARGET` 覆盖 HTTP/WS 代理目标。
- 用户状态固定为语言中立 `dipole.user.status.v1`：`normal=1`、`disabled=2`，Go 领域常量改为显式值，避免其他常量或多语言实现改变持久化语义。
- Web 会话终止统一覆盖显式退出、HTTP 401、WS kick 和账号切换：先撤销本地凭据，再等待在途 Sync 收敛并清理该用户 IndexedDB；并发终止复用 singleflight，快速重登等待旧清理完成。

- Web 同步以 `message_uuid + message_seq + sync_seq` 作为本地身份、排序和恢复契约；MySQL 自增 `id` 继续只服务旧 Offline 兼容路径。
- A6 将 Durable Inbox 新增写入责任切换到 Sync Projector；默认 `atomic` 仍是即时回滚路径，切换需同时启用 Sync Projector、最小权限账号和启动权限验收。
- `dipole-sync` 的新 Kafka consumer group 从 earliest retained offset 建立，已有 group 继续使用已提交 offset；Outbox Replay 与历史 baseline 负责覆盖 Kafka retention 之外的数据。
- A6 首个切片将 Durable Inbox、设备 Cursor 和群 checkpoint 的查询所有权抽入 Sync Service；Message 事务继续原子写入 Inbox，事件消费投影将在具备回放与一致性门禁后切换。
- 普通群消息按成员写入 Inbox；热群沿用 notify + pull，跳过成员级 Inbox 写扩散。
- HTTP、Kafka 与 Agent 启动路径通过统一 Composition Root 创建 Repository 与消息域 Service，消除进程内重复实例和分散的具体依赖构造。
- Runtime 在 HTTP、Kafka、Outbox 和 AI 助手初始化之间复用同一 sqlc Repository 集合，所有构造入口必须显式提供 `*sql.DB`。
- Kafka Topic 初始化显式覆盖主 Topic、`.retry` 与 `.dead`；Publisher 和 Transactional Outbox 同时写入 `version` 与 `schema_version` headers。
- Kafka 单节点默认继续使用 RF=1/min ISR=1/acks=one；cluster profile 使用 RF=3/min ISR=2/acks=all，低于 quorum 时拒绝确认写入。
- 增加 `message.transport=local|grpc` Composition Root 开关；默认 Local，grpc 模式通过进程内 channel 执行同一 MessageApplication 契约并支持安全回切。
- Messaging Composition Root 支持注入远程兼容的 `CoreCapability`，为独立 Message Service 停止直接读取 Core Repository 建立切换边界。
- `message.transport=grpc` 升级为受认证的真实 TCP channel；Core 先开放 Capability listener 再连接 Message，避免冷启动循环等待，`local` 继续作为默认回切路径。
- Kafka handler 按 Core 与 Message 进程拆分；远程模式下 Core 停止消费 `message.*.send_requested` 并停止运行 Outbox Relay。
- 本地与远程 MessageService 统一通过 Core Capability 校验文件所有权；非所有者与缺失文件返回相同不可用语义。
- Outbox Relay 停止时等待 worker 退出，避免 Kafka publisher 关闭后仍有并发发布。
- Shadow comparison 只比较 Message v1 对外字段，忽略 `CreatedAt/UpdatedAt` 等内部字段，并按时间瞬时语义处理时区差异。
- Cassandra 存储影子比较限制为 32 个并发任务；容量耗尽、空页、主查询失败或无效 Seq 仅记录跳过原因，不增加主查询等待时间。
- Cassandra 主读首批仅覆盖 Seq cursor；MySQL `id` cursor、Offline Inbox 和写链路保持原存储职责，百分比设为 0 可立即回切。
- MySQL 完整消息写入保留到 A5/A6 替代读契约双跑完成；`metadata_only` 写模式仅作为门禁后的未来开关，不在 A4 实现。
- Web 历史首屏和加载更多统一使用 `before_seq`，热群在线补拉使用 `after_seq`；消息 UUID 负责去重，Seq 负责排序和分页。
- Runtime 创建的同一套 Messaging Services 现在注入 Server 与 Kafka handlers，消除 grpc 模式下重复的 Local MessageService 和 singleflight 实例。
- 增加 `gateway.mode=embedded|remote`；默认保持单体行为，`remote` 时 Core 停止注册 WS 路由和实时投递 handler，其余 HTTP/Swagger/静态 Web 由 Gateway 代理到私网 Core。
- Kafka 实时投递 handler 与 Core 领域投影拆分，独立 Gateway 固定使用 `dipole-gateway-consumer`，避免与 Core 和 Message 的消费职责竞争。
- Core 与 Gateway 调用 Message RPC 时分别声明 `dipole-core`、`dipole-gateway`，内部证书和审计主体保持独立。
- 服务启动只读校验 migration 版本，已移除运行时 schema mutation 和 `AutoMigrate` 配置。
- Message、Sync Inbox、Conversation Seq allocator 与 Transactional Outbox 在同一 MySQL 事务中提交；Outbox payload 在 Seq 分配后构造，保证事件与消息事实一致。
- Migration Up/Down 使用 schema-scoped MySQL advisory lock，多个 migration owner 并发启动时按锁串行执行并重新读取 ledger。
- Redis 单节点配置保持兼容；Sentinel 模式通过 go-redis Failover Client 自动发现当前 master，业务组件继续共享同一客户端。
- Kafka Topic 创建后使用有界 metadata 收敛重试，避免冷启动期间短暂的 `Unknown Topic Or Partition` 造成服务退出。
- Conversation 投影按 Seq 拒绝重复和过期消息回退；`unread_count` 继续作为兼容投影，由 `last_message_seq - read_seq` 语义维护。
- Composition Root 统一使用 sqlc，已移除 `data.mysql_adapter` 兼容开关和 legacy GORM adapters。
- User Repository 的 Redis/Bloom 策略从数据库适配器中抽离，SQLC 后端复用统一缓存装饰器。
- Contact Repository 的 Redis 关系缓存从数据库适配器中抽离，SQLC 后端复用统一缓存装饰器。
- Group Repository 的 Redis/Bloom 与成员排序策略从数据库适配器中抽离，SQLC 后端复用统一缓存装饰器。
- Conversation 的消息预览规则收敛到 domain model，SQLC 投影复用统一的文本、文件、AI 和系统消息摘要语义。
- Eino 从 `v0.8.8` 升级至 `v0.9.17`，`eino-ext/components/model/openai` 保持 `v0.1.13`。
- 更新 OpenAPI/Swagger 文档，加入同步接口及其请求、响应模型。

### 修复
- 修正 Timeline repair Compose smoke 与运维手册的迁移基线至 v50，避免 v50 schema 在部署前置检查中被误判为旧版本。
- 修正 MySQL migration integration 的当前版本基线至 v50，并校正 Metadata 回填测试的回退步数；隔离 Cassandra/Sync hydration smoke 现可真实覆盖 v12 `message_metadata` legacy-message backfill。
- Go 质量门禁默认采用 `CGO_ENABLED=0`，与服务镜像的静态构建保持一致；需要平台原生依赖的检查仍可显式设置 `CGO_ENABLED=1`。

- 存储实验 Compose 支持 Cassandra 宿主机动态端口；hydration 与 read-routing smoke 启动后反查实际映射，修复并行运行共享固定 `19042` 导致的假失败。默认使用随机端口，也可通过 `DIPOLE_CASSANDRA_LAB_PORT` 显式覆盖。

- 修复 Agent Task Timeline 内部自动事件 ID 由长 Task/Run UUID 拼接导致超过 `VARCHAR(64)` 的问题；现在使用固定 64 位 SHA-256 十六进制 ID，兼容最大长度身份并保持事件校验边界。

- 前端 Agent Task 响应解析器现在严格解析 `waiting_approval` 的 request/approval/summary/expiry，并提供绑定 Task/approval ID 的 approved/denied API；审批状态不会再被静默丢弃，`waiting_input` 和旧状态行为保持兼容。
- 修复 `realtime-cpp` 镜像构建阶段遗漏 authority fence 测试契约目录的问题，确保 CMake/CTest 在独立 builder 中读取完整 golden vectors。
- 修复 Agent TypeScript Proto 生成物滞后：重新生成 `CorrectOwnedMemory` RPC、Memory root/version/correction 字段及后续方法索引，使 TS 客户端与已发布的 additive Go Proto 定义恢复一致；未改变 Proto schema 或 Runtime 开关。
- 修复 C1 Kafka lag 采样将无 committed offset 的分区静默当作零积压：独立解析器对 `current-offset=-` 且存在 log end 的行保守计入 retained backlog，并在找不到目标 consumer group 或字段不可解析时 fail closed。真实 node2 首轮恢复演练因此识别出 HTTP 健康早于 72-member consumer group 稳定、`LastOffset` 跳过 40 条 send-requested 的窗口，失败报告未被接受。
- 修复 canonical Go gate 在干净 checkout 中隐式依赖被忽略的 `configs/config.yaml`：配置加载器支持显式 `DIPOLE_CONFIG_FILE`，`scripts/check-go.sh` 默认使用跟踪的 `configs/config.dist.yaml`，调用方仍可覆盖；未设置环境变量的生产/本地启动继续沿用原有 `config.yaml` 搜索行为。
- 修复分布式与微服务 Compose 在干净 checkout 中强制依赖未跟踪 `.env`：本地 env file 改为 optional，关键内部 RPC secret 的 `${VAR:?}` 校验保持不变；新增 `scripts/check-compose.sh` 对全部 Compose 文件执行统一静态解析。
- migration v17 将 Agent Definition、Task 与 Approval 的身份列从 20 字符 expand-only 扩至 24 字符，修复默认 21 字符 `assistant_uuid` 无法初始化 persistent policy 的启动失败。
- HTTP Handler 测试在包级 `TestMain` 统一初始化 Gin TestMode，移除并行测试中的重复全局写入，使整包 `go test -race ./internal/handler/http` 可作为稳定门禁。
- 修正 MySQL migration 集成测试仍将已存在的 v10 当作未来版本的问题，并将真实上下迁移与并发 owner 门禁推进到 v11。

- 修复 WS 暂态回声先占用 `message_id` 时，后续持久化历史消息无法回填正 `message_seq` 的问题；消息合并现在优先保留持久化等级更高的版本。
- 修复并发消息事务可能造成同一用户 Sync Cursor 永久跳过迟提交消息的问题，Inbox 写入现在按用户锁行串行化。
- 修复旧群消息 Kafka 事件缺少 `sync_fanout` 时被误判为热群的问题，滚动部署期间默认保留普通群 Inbox fanout。
- 修复幂等冲突复用已有消息时可能沿用新目标收件人的问题，路由身份不一致时拒绝 Outbox/Inbox 修复。
- 修复重复创建已存在好友关系时可能用新建默认值覆盖缓存、造成缓存状态与数据库状态暂时不一致的问题；建交成功后统一失效双向缓存。
- 修复群成员追加按输入切片长度递增 `member_count` 导致重复或空成员虚增的问题，改为按数据库实际插入行数计数。
- 修复 MySQL 8.4 下同一批次重复追加新成员可能触发 `Error 1869` 的问题，追加入口先按群和用户去重。
- 固定 Conversation upsert 的 SQL 赋值顺序，先基于旧 `last_message_uuid` 计算未读，再更新最新消息字段，避免依赖 GORM map 排序。

### 安全

- MCP request budget 独立于旧 `rate_limit.enabled`，防止关闭传统业务限流时同时移除模型驱动入口门禁；JWT principal 是唯一计数身份，客户端无法通过请求正文或更换 Task/Run 选择限流键。
- MCP Tool 参数、结果和内部异常正文禁止进入数据库审计及 span 属性；Core 重新解析运行中的 Task/Run 和固定 Definition，只允许 read-risk Capability，Runtime 无法提交 principal。重复 begin、重复终态、伪造 principal 与审计失效均拒绝执行或返回稳定泛化错误。
- MCP Gateway 代理移除公开 Authorization、Cookie、客户端注入的 `X-Dipole-*` 身份头和 hop-by-hop/body 长度头，再注入固定 `dipole-gateway` 服务凭据与 JWT principal；Core 对外部 principal 不匹配统一返回 unavailable，模型和 Tool 参数无法选择 tenant、Agent、Task 权限或 resource scope。
- 移除 Vite 5/esbuild 与 Vitest 2 开发依赖链中的 1 个 critical、1 个 high 和 3 个 moderate 公告；完整依赖及生产依赖 `npm audit` 均为零漏洞。
- 新增第三个离线 `dipoleartifactmaintenance` 检查身份，MinIO policy 仅允许固定 Artifact 前缀 GetObject 证据权限；Go adapter 只暴露 Stat，配置拒绝复用 Runtime 或 audit access key。该身份无法 List、Put、Delete，Core、Agent 与 Gateway 均不持有其凭据。
- 新增 `dipoleartifactaudit` 身份，policy 只有专用 bucket 的定位和 List 权限；CLI 显式拒绝复用 Artifact Runtime access key，Core、TS Agent 和公开 Gateway 均不接收 audit 凭据。候选发现不授予 Get/Put/Delete，未来清理身份继续与 audit/Runtime 分离。
- Agent Artifact 改用独立 `dipole-agent-artifacts` bucket 与专用 Core 身份，policy 仅允许 bucket 定位/列举和 `agent-artifacts/v1/*` 的 Get/Put，明确不授予删除权限；通用文件/归档身份同步从 MinIO root 降为限定三个既有 bucket 的 `dipoleplatform` 用户，两个运行时身份无法跨 bucket 写入。TS Agent Runtime 不接收对象存储凭据。
- Message atomic/projector 账号仅保留 sqlc 实际使用的表操作；显式拒绝 Message UPDATE/DELETE、Outbox DELETE、migration 写入、Core 表访问，以及 projector 模式的 Inbox 权限，关闭 AD-015。
- Sync 与 projector-mode Message 账号按表和操作拆分；启动时拒绝 Sync 修改 Message/Outbox/群高水位或读取 Core 数据，也拒绝 Message projector 访问 Inbox 状态。
- 更新前端生产依赖锁定版本，修复 Axios、form-data、nanoid 和 PostCSS 的高危公告；生产依赖审计恢复为零漏洞。

### 移除

- 移除 Message Service 测试替身中遗留的 `Create`、`StoreWithOutbox` 兼容写入口，并以方法集测试固定生产 `MessageRepository` 仅暴露 Sync-aware 原子写契约。
- 移除 legacy GORM repositories、model persistence tags、运行时 `AutoMigrate`、SQLite 方言测试以及 `gorm.io/*` 依赖。
- 移除 `data.mysql_adapter`、`mysql.auto_migrate` 和无依赖 Repository/Server/Kafka 便捷构造入口。

### 迁移说明

- v42 把 `agent_memory_task_lineage.task_uuid` 外键从 `agent_shadow_plans` 改为 `agent_tasks`，并增加 `context_pre_model` 来源。发布顺序为 migration、Runtime writer、模型流量；旧 Runtime 的 Plan-time `runtime_write` 继续兼容。Down 会删除尚无 Plan 的 pre-model lineage，并把其余来源归一为 `runtime_write` 后恢复 v41 Plan 外键，因此回滚前必须保留低敏影响报告并停止模型 admission。
- v41 新增 `agent_memory_task_lineage`，以 Memory/Task 主键和 Shadow Plan 外键保存 direct Context reference；发布顺序为 migration、sqlc/TS query、Agent Runtime Shadow writer，旧 Runtime 可继续写 Plan 但会被审计标记为历史未索引。回滚前停止新 Runtime 写入；Down 只删除 direct-reference 索引，不修改 Memory、Plan、Step 或其他派生数据。该迁移不执行历史回填，也不启用删除 Worker、公开 API 或生产 Runtime。
- v40 为 `agent_memories` 增加内容擦除审计列与固定 tombstone 约束。Down migration 只删除 v40 列和约束，已被擦除的正文、来源及自由文本不会恢复；回滚仍保留 revoked 状态和 v39 纠正链，因此执行擦除前必须按隐私策略确认不可逆影响。
- v39 在 `agent_memories` 上追加 lineage 与 correction 审计列，并把历史记录回填为 `root=self/version=1`。发布顺序为 migration/sqlc 与 Core transaction、additive gRPC/Gateway、最后显式开启 `VITE_AGENT_MEMORY_CORRECTION_ENABLED`；回滚前先关闭前端入口，Down 会删除纠正 lineage 字段，执行前需确认是否保留已产生的版本链审计。
- migration v38 为 `agent_memories` 增加 `revoked_by_uuid` 与 `revoke_reason`，并约束 active 记录审计字段为空、revoked 记录同时具备时间、revoker 和原因。历史 revoked 记录以原 principal 和固定 `legacy internal revocation` 回填。发布顺序为 Core migration/sqlc 与 additive gRPC、Gateway、最后显式开启 `gateway.agent_memory_enabled` 和前端 `VITE_AGENT_MEMORIES_ENABLED`；回滚先关闭两个入口，Down 会移除新增撤销审计列，需先确认审计保留要求。
- Agent Subscription owner create 使用 additive `ListEligibleSubscriptionConversations` RPC，无数据库迁移。先确认 Core 已应用 migration v34，再滚动 Core/Go Proto、Gateway/Web 和 TS 生成客户端；旧 Runtime 不调用该 RPC。`gateway.agent_subscription_enabled=false` 与 `VITE_AGENT_SUBSCRIPTIONS_ENABLED=false` 仍需同时显式启用。回滚先关闭前端入口，再关闭 Gateway adapter；该流程不修改 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`。
- `PublishMcpReadinessEvidence` 是 additive Agent Capability RPC，依赖 migration v37。先迁移并滚动 Core，再发布 TS Runtime；旧 Runtime 不调用该方法。回滚时先确保没有证据 Publisher 调用，再回退 Runtime/Core；当前没有 startup scheduler 或 admission consumer，部署后不会自动建立外部连接或激活 Profile。
- migration v37 新增 `agent_mcp_readiness_evidence` 追加式控制面表及 tenant/Profile/Runtime binding freshness 索引。先迁移 Core，再发布 additive Publisher RPC client；当前没有自动采集或 admission 启动接线。回滚前停止证据写入并完成审计留存，v37 Down 会删除全部 readiness evidence 历史，不影响 Agent Task、Run、Artifact 或 Runtime promotion 表。
- `ResolveMcpToolCommandResponse.status` 是 additive 字段，无数据库迁移。先滚动 Core，再滚动 Agent Runtime；旧 Runtime 忽略未知字段。回滚 Runtime 后可继续由新 Core 服务旧调用方；回滚 Core 前须确保新 Runtime 不再执行 terminal receipt recovery，否则空状态会 fail closed。生产 Worker 与外部网络开关仍保持关闭。

- migration v34 为 `agent_event_subscriptions` 增加 owner/revocation 审计列和一致性约束。先迁移 Core，再滚动发布 additive Go/TS Proto；迁移会从固定 Definition version 回填 creator，历史 revoked 行使用 `legacy_v28_migration` 标明来源。回滚前停止新的管理 RPC 写入；v34 Down 只删除新增审计列，保留 v28 订阅与 Task 绑定。生产 `subscription` 模式仍需完成 AD-034 的前端管理与语义基线门禁。
- 数据库新增 migration v33：创建 `agent_runtime_promotion_operator_grants`、`agent_runtime_promotion_proposals`、`agent_runtime_promotion_reviews` 和 `agent_runtime_promotion_revocations`。升级不会自动创建 operator Grant；部署者需通过受控运维流程预置 tenant-scoped proposer/reviewer/revoker，且应保持职责分离。回滚 v33 会删除控制面提案、复核和撤销审计表，不影响 v32 durable Grant 表，但应先停用控制 API 并归档审计证据。

- 新增 MySQL migration v32：创建 `agent_runtime_promotion_grants`，并为 `agent_runs` 增加可空 `candidate_version`。现有 embedded/shadow Run 无需回填；历史 active Run 缺少 candidate 或 production authorizer 时会 fail closed，回滚会先移除 Run candidate 字段再删除 grant 表。
- `ResolveApprovalGrant` 是 additive Agent Capability RPC，无数据迁移；先滚动 Core 和 sqlc 查询，再发布 TS Runtime。查询只在持久 active Run 中返回唯一 exact grant，旧 Runtime 不调用。回滚先保持生产 write Tool 未注册，再回退 Runtime/Core。该 RPC 可用不代表 active context 已晋级。
- MCP Message write projection 没有数据迁移或生产开关。`createDipoleMcpServer` 只有同时收到 active ExecutionContext、审批必需的 write descriptor、Command kind 和显式 executor 才注册写 Tool；当前启动链不提供这些条件。未来启用前还需完成 active authority 晋级、UI 风险摘要，并演练 Command RPC 超时后通过 receipt 收敛 action lineage。
- `ExecuteMcpMessageCommand` 是 additive Agent Capability RPC，无数据迁移。先滚动 Core，再发布 TS Runtime；旧 Runtime 不调用该方法。回滚时先保持生产 MCP write Tool 未注册或关闭，再回退 Runtime/Core。该 RPC 依赖 migration v31 的 Tool Invocation Approval 字段，不能在 v31 之前启用。
- migration v31 为 `agent_tool_invocations` 增加 nullable Approval 与 action reference 字段及约束。先迁移 Core，再发布 additive Agent Proto 与 Runtime；旧只读调用继续写入空引用。回滚前必须关闭所有未来 write Tool、等待 `running` 调用收敛并确认不再需要 Approval/Command/Message 联查证据，v31 Down 会移除这些字段和索引。
- `ConsumeApproval` 是一次性、active-only 的安全门槛。审批在 Capability 执行前消费，执行失败后不会自动恢复或重放；调用方必须创建新审批。Message Command 已具备稳定业务幂等键和 sender-scoped receipt 查询，可收敛 RPC 超时后的提交状态；migration v31 已提供 Tool-to-Message lineage，生产 write projection 仍需 active authority、UI 风险摘要和故障演练。当前不要向 `createDipoleMcpServer` 注册 write/destructive Tool。
- 外部 MCP Client 的原始 `CallToolResult` 不得直接拼接到 system/trusted prompt、Memory 或 Agent instruction。未来接入必须先通过 `externalMcpResultToContextFragment`，保留 `trust=untrusted` 与 provenance；若结果要形成 Artifact 或 Memory，需继续携带原始 Invocation lineage，不能因人工摘要或模型改写提升信任等级。
- Network Guard 尚未装配生产 Resolver/Dispatcher。后续 Dispatcher 必须用守卫提供的某一个批准地址直接建连，使用 Profile `tlsServerName` 做 SNI/证书主机校验、通过 `caBundleRef` 从可信 Secret backend 取 CA，并返回 socket 实际 peer；禁止在 Dispatcher 内再次按 hostname 自由解析或自动跟随重定向。当前接口存在不代表可启用 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED`。
- Secret Provider adapter 没有生产 backend 或启动配置。未来 Provider 必须为每次 `read` 返回独占、可写的 fresh `Uint8Array`，遵守 AbortSignal 且不得把 secret 写入异常；复用共享 buffer 会在首次请求后被 adapter 清零。SDK 所需 token string 无法强零化，部署时应使用短期、最小权限凭据并依靠 Catalog 版本/吊销和 Server 端失效控制暴露窗口。
- Catalog file source 没有默认路径或启动装配。后续受控环境使用时必须挂载 root/Runtime UID 拥有、group/other 不可写的 regular file；默认 Kubernetes ConfigMap/Secret projected volume 使用 symlink，会被 `O_NOFOLLOW` 拒绝，可由受信 init/sidecar 写入私有 tmpfs regular file 并以同目录原子 rename 更新。不要降低 symlink/mode 校验来迁就挂载方式。
- Credential Catalog v1 没有数据库迁移或生产配置入口。后续 Catalog source 必须返回完整、受信的 `contracts/agent-external-mcp/v1/credential-catalog.schema.json` manifest，并在轮换时先发布新版本、更新 Profile、确认新建连成功，再吊销旧版本；Catalog 中只允许 opaque `provider_secret_ref`，禁止 Secret 值或 provider credential。
- 外部 MCP Profile foundation 与 Transport Factory 没有数据迁移，Compose 固定 `DIPOLE_AGENT_EXTERNAL_MCP_ENABLED=false`。现阶段不要在部署环境开启该变量；Runtime 会在发现启用配置后拒绝启动，直到后续里程碑注入加密 Secret Provider、真实 DNS Resolver 与 TLS pinned Dispatcher。Profile JSON 禁止保存 Token、密码、私钥或 CA 正文。
- MCP 限流没有数据迁移。微服务 Gateway 默认配置为 60 次/60 秒；发布后可调整 `DIPOLE_RATE_LIMIT_AGENT_MCP_LIMIT` 与 `DIPOLE_RATE_LIMIT_AGENT_MCP_WINDOW_SECONDS`。两个值必须为正且 Redis 必须可用，否则认证 MCP GET/POST 返回 429。回滚先关闭 `DIPOLE_GATEWAY_AGENT_MCP_ENABLED`，保留限流配置不会影响其他入口。
- `BeginMcpToolInvocation` 与 `FinishMcpToolInvocation` 是 Agent Capability v1 的 additive RPC。先执行 migration v30 并滚动 Core，再滚动 Runtime；入口默认关闭，确认 Core 审计可用后再按既有双开关启用。回滚先关闭 Gateway/Runtime MCP 开关并等待 `running` 调用收敛，再回退 Runtime/Core；v30 down 会删除 `agent_tool_invocations` 审计历史，应先完成合规留存判断。
- `ResolveMcpContext` 是 Agent Capability v1 的 additive RPC。先滚动 Core 与 Runtime，再同时显式设置 `DIPOLE_AGENT_MCP_SERVER_ENABLED=true` 和 `DIPOLE_GATEWAY_AGENT_MCP_ENABLED=true`；任一开关保持 `false` 时公网链路不可用。回滚先关闭 Gateway 开关，再关闭 Runtime 开关，无数据迁移。共享环境仍需完成 `AD-037` 的 OAuth resource indicator、外部 Server 凭据和 OTel exporter/告警门禁。
- migration v29 创建 `agent_memories`。先迁移 Core，再滚动发布 additive Proto 两端，最后在受控 Shadow 环境设置 `DIPOLE_AGENT_MEMORY_ENABLED=true`；默认与 Compose 保持 `false`。回滚先关闭开关，Down 会删除全部 Memory 内容和来源记录，只适用于停止 Agent Runtime、确认无需保留记忆且已完成必要导出的环境。
- migration v28 创建 `agent_event_subscriptions` 并为 `agent_tasks` 增加 nullable `trigger_subscription_uuid` 外键。发布新 Runtime 前先迁移 Core，再滚动发布 additive Proto 两端；启用 `DIPOLE_AGENT_TRIGGER_MODE=subscription` 前必须通过受控 Store 写入绑定有效 Definition 的订阅。回滚先恢复 `direct_target` 并等待在途 Task 收敛，再执行 v28 Down；Down 会删除订阅表及 Task 订阅绑定，仅适用于确认不再需要相关审计数据的环境。
- migration v27 将历史 `users.status=0` 归一为 `normal=1`，把默认值改为 `1`，并增加仅允许 `1/2` 的数据库约束。Down 会移除约束并恢复默认值 `0`，已归一的行继续保留 `1`，避免破坏合法账号状态。
- `dipole-agent-artifact-maintenance -action authorize` 从已签名 reconcile JSON 生成短期授权；`-action evaluate` 需单独注入 `storage.artifact_maintenance_access_key/secret_key` 并连接只读 MySQL。`would_delete` 仅表示当前证据满足条件，不能转换为删除动作；其余阻断结果退出码为 2。
- 运行 `dipole-agent-artifact-reconcile` 前通过环境变量单独注入 `storage.artifact_audit_access_key/secret_key`，并保持最短 `-minimum-age=24h`；命令输出 JSON，发现候选或异常键时退出码为 2。当前报告仅用于审查和容量治理，不能直接作为删除授权。
- 通用存储默认凭据由 MinIO root 改为 `dipoleplatform`，Artifact 存储通过 `storage.artifact_*` 独立配置且默认关闭；更新后的 `minio-init` 会幂等创建两个受限用户、policy 和专用 bucket。微服务 Compose 显式启用 Artifact 存储；自定义部署需先创建等价 policy，再设置独立 access key/secret，回滚时关闭 `storage.artifact_enabled` 即可停止 Artifact blob 装配，现有元数据与对象保持不变。
- 启用 `read_shadow` 前先执行 migration v23，并同时配置 MySQL ledger、`ai_sdk` model routes、Agent Capability RPC 与 Temporal；启动顺序为 Store 探针、Worker、Temporal dispatcher、Kafka consumer。回滚先将 Activity mode 恢复为 `persistent_shadow`/`foundation` 或关闭 Temporal，再回滚 v23；v23 仅增加 nullable `agent_model_calls.output_json`，旧 Runtime 可继续运行。
- `FinishRun`、`RequestApproval` 和 `ResolveApproval` 是 Agent Capability v1 的 additive RPC，旧 Runtime 可继续调用 `CompleteRun`。持久 shadow Worker 需同时显式设置 `DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE=persistent_shadow`、启用 Agent Capability RPC 并配置既有共享密钥或 mTLS；回滚时先恢复 `foundation` 或关闭 Temporal Worker，无需数据迁移。
- Agent 镜像新增精确锁定的 Temporal TypeScript SDK `1.23.0`，构建和运行基底由 Node 22 Alpine 切换为 Node 22 Bookworm slim，以满足 Temporal Native Core 的 glibc ABI。现有部署无需启动 Temporal Server；`DIPOLE_AGENT_TEMPORAL_ENABLED` 默认为 `false`，仅在独立 foundation 验证环境设置为 `true`，并配置 address、namespace 与 task queue。
- 发布 Context Compiler v1 前先执行 migration v22；该 migration 只向 `agent_shadow_plans` 追加 nullable compiler/Token/manifest 字段，旧 Runtime 可继续写入空值。回滚新 Runtime 后可执行 v22 down 删除 manifest 字段，不影响 Plan/Step 主数据。
- 启动 TS Agent Capability 执行前先应用 migration v21 和更新后的 `agent-service-grants.dist.sql`，再为 `dipole-agent` 配置 Core target、共享密钥与 mTLS CA/cert/key/server name。微服务证书脚本已生成 Agent 双用途证书；回滚时先关闭 `DIPOLE_AGENT_CAPABILITY_RPC_ENABLED` 或恢复 metadata-only Runtime，再执行 v21 down。v21 down 会删除 Run 审计及未完成 Step lease，只应在确认 Agent consumer 停止后执行。
- 启动前执行 migration v17。该变更仅扩宽 Agent policy 身份列；应用回滚时保留 24 字符宽度以兼容已有身份，因此 Down 为安全 no-op。需要临时回退策略来源时设置 `DIPOLE_AI_POLICY_MODE=static`。
- 独立 Message Service 部署先应用 `message-service-atomic-grants.dist.sql` 或 `message-service-projector-grants.dist.sql`，再配置 `message.mysql.*` 并启用 `message.enforce_db_permissions`；atomic/projector 模式与账号必须匹配。微服务 Compose 已通过一次性 `mysql-permissions` 服务自动执行开发授权。
- 发布 Cassandra 完整消息归档能力时先执行 migration v15；已有 Job 按固定高水位回填 `mysql-messages:<id>`。归档 Backfill/Reconcile 必须使用同一 manifest 和 Job 名；需要换源时创建新 Job，禁止覆盖归档或复用旧 checkpoint。
- 发布归档重建能力时先执行 migration v13；已有 Search Job 会按其固定高水位回填 `mysql-outbox:<id>` 身份。归档模式要求三条命令使用同一 manifest 和新 Job 名，回滚可显式恢复 `--source=mysql`，不得覆盖已存在归档文件。
- 发布 Message Metadata 时先执行 migration v12 并应用更新后的 Message atomic/projector GRANT，再滚动新 Message 节点；回滚先恢复旧节点，再执行 down migration。历史 `payload_sha256=''` 表示迁移前未记录指纹，不据此拒绝重复请求。
- migration v14 为 `message_metadata` 回填 `legacy_message_id`；先执行 migration，再灰度开启 `message.cassandra_duplicate_hydration`。回滚开关后可继续使用 MySQL 正文，随后再执行 v14 down migration。
- 写责任切换顺序固定为：完成 baseline/Replay Reconcile 和 lag/DLQ 门禁，应用两套最小授权，启用 `sync.enforce_db_permissions`，再设置 `message.inbox_write_mode=projector`；回滚时先恢复 `atomic` 及原 Message 授权，再停用 Projector。
- migration v9 会从 `messages.seq` 回填现有 Inbox，并创建位置唯一索引；存在无法关联 Message 的孤立 Inbox 时迁移会失败，部署前应先完成一致性检查。表级 DDL 建议在维护窗口执行。
- 切换 Sync 查询前先启动 `dipole-sync` 并验证 health/RPC；随后将 Core 的 `sync.transport` 改为 `grpc`。回滚只需恢复 `local` 并重启 Core，不涉及数据回滚。

- 消息搜索入口默认关闭；部署时需要同时设置 Gateway `search.enabled=true` 与前端构建变量 `VITE_SEARCH_ENABLED=true`，任一侧关闭都会保持现有聊天行为。
- Message、Inbox 与 Outbox Producer 已作为同一 sqlc 事务边界运行，避免跨连接提交。
- MySQL 运行时连接池由 `database/sql` 直接初始化，migration、sqlc repositories 与 Bloom Registry 共享同一连接池。
- Bloom Registry 改用 `database/sql` 读取用户和群 UUID，停止依赖全局 GORM 查询。
- 部署或本地启动服务前执行 `go run ./cmd/migrate -direction up`；`000001_baseline` 创建或接管基础业务表，`000002_conversation_sequence` 回填历史会话序号并建立 allocator。
- `000002` 按 `conversation_key + id` 为历史消息回填 `1..N`；先完成 migration，再滚动发布新 Message 节点。旧 `before_id`/`after_id` 接口在客户端迁移期间继续保留。
- `000003` 通过现有最后消息与未读计数回填会话读位置，并创建 `device_sync_checkpoints`；只有启用具备 IndexedDB 事务提交能力的 Web Sync Engine 后，客户端才会自动 ACK Sync 页面。
- Web Sync Engine 通过构建变量 `VITE_SYNC_ENGINE_MODE=off|shadow|primary` 灰度；默认 `off` 保持现有客户端行为，`shadow` 以旧 Offline 为界面主路径，`primary` 才由 `/sync` 恢复界面。旧布尔开关 `VITE_SYNC_ENGINE_ENABLED=true` 暂时映射到 primary 兼容已构建环境。
- `000004` 创建可重建的 `message_search_documents` 逻辑索引；当前不自动回填，A5 Search Indexer 上线前搜索入口保持关闭。
- `000005` 回填群消息高水位并创建设备群 checkpoint；Message 账号新增 `group_sync_states` 写权限，继续拒绝设备 checkpoint 与其他 Core 表。
- `000006` 创建 `cassandra_backfill_jobs`；先确认实时 Projector 已接管增量，再运行 `dipole-cassandra-backfill` 固定历史快照并补齐 Cassandra。
- `000007` 为 `message_search_documents` 增加 revision、searchable 与 payload hash，并允许最小 tombstone 使用空正文元数据；迁移前后搜索入口仍保持关闭。
- Cassandra 固定快照对账通过后，可同时启用 `cassandra.enabled` 与 `message.cassandra_shadow_reads`；回滚时关闭后者并滚动重启 Message Service，无需迁移数据。
- 启用 `message.cassandra_read_percentage` 前先发布支持 `before_seq` 的服务端；新版 Web 不再把 MySQL `id` 用作历史游标，旧客户端的 ID cursor 继续固定访问 MySQL。
- baseline migration 会创建 `user_sync_inbox` 与 `user_sync_states`；所有消息持久化节点完成升级后，并发提交顺序保证正式生效。
- Inbox 只覆盖升级后新产生的消息；升级前历史消息继续通过现有历史/离线消息接口读取。
- 现有 `/messages/offline` 接口继续保留，客户端可以渐进迁移到 `/sync`。
- 兼容缺少 `sync_fanout` 字段的旧私聊和群聊 Kafka 事件，避免滚动部署期间漏写 Inbox。
- 独立部署先启动启用 `internal_rpc` 的 Core，再启动 `cmd/message-service`；两端通过 `DIPOLE_INTERNAL_RPC_SHARED_SECRET` 注入同一服务凭据，Gateway/Core 节点随后将 `message.transport` 切换为 `grpc`。
- Gateway 独立部署要求 `gateway.mode=remote`、`message.transport=grpc`、Kafka 和内部 RPC 同时启用；Core 改为私网 HTTP 目标，公开流量只进入 Gateway。

### 验证

- 真实 MySQL 验证通过：`AgentPolicy`、Runtime Promotion 和 Timeline repair recovery contract 均通过，覆盖最大长度身份、故障 retry、恢复 completed 与单事件收敛。
- Agent MySQL contract 分组验证通过：`TestAgent*` 与 `TestAICallLogRepositoryContract` 共 13 个 contract 在真实 MySQL 上完成，耗时约 149 秒；完整 repository 分组仍单独受数据库清理等待影响。
- Repository contract 分领域验证通过：基础实体、消息/同步/序列、Outbox/Search 三组在真实 MySQL 上分别通过；与 Agent 组结果合并后覆盖全部 repository contract。共享 MySQL 下的串行整包仍受测试数据库清理耗时影响，未作为单次整包证据。
- Eino 能力核对：当前锁定的 `v0.9.17` 已包含 ADK Runner、AgenticMessage、AgenticModel 和 AgenticToolsNode 相关 API；Dipole 暂保持现有 `schema.Message`/`adk.Runner` 兼容链路，未直接启用实验性 Agentic provider，后续通过独立 adapter、契约测试和灰度开关评估。

- Agent Workflow repair v44 repository 合同测试在隔离 MySQL 8.4.8 中验证 `prepared` execution 创建、精确重放和同 plan 目标哈希漂移拒绝；当前测试不推进状态，也不修改 Workflow projection。
- MySQL migration integration baseline 更新至 v44，并覆盖 v44 execution ledger、v43 lineage backfill、v42 pre-model lineage 的连续回滚与表数量断言；避免新迁移已发布但集成测试仍停留在 v42 的验证盲区。
- 发布级隔离门禁通过：`scripts/check-compose.sh` 校验全部根目录 Compose 与 Agent Shadow 配置；`scripts/smoke-microservices.sh` 使用独立项目启动并清理 Core、Message、Sync、Gateway、Agent、MySQL、Kafka、Redis 和 MinIO，验证 readiness/metrics、mTLS、Gateway 认证代理与远程 WS ownership。Agent 镜像在隔离构建中通过 `npm ci`、构建和零漏洞审计。
- 发布级回归重新通过：Go 全量包、Agent Runtime `580 passed / 26 skipped`、前端 `85 passed` 与生产构建、sqlc 漂移和架构文档门禁均通过；修正 Agent Runtime README 对 Subscription 管理 API 的过时描述。Vitest 使用项目原生命令运行，未使用不兼容的 Jest `--runInBand` 参数。

- Agent Memory lineage backfill CLI 通过默认 dry-run、审批/manifest hash 绑定、owner 身份匹配和输入大小边界测试；隔离 MySQL 8.4 通过 v43 migration up/down/reapply、失败后恢复、owner 隔离和精确重放验证。
- Backfill execute approval 收紧为独立 operator/approver 双身份，二者必须不同并共同绑定 job、manifest hash 与 source high-water；CLI 不提供自审批路径。
- 增加只读 `agent-memory-lineage-rollout-review` CLI，验证 v43、runtime/config SHA-256、未来维护窗口、回滚/备份检查和双人审批；eligible receipt 固定 `executionAuthority=false`，不会触发 backfill。
- 增加 `scripts/agent-memory-lineage-rollout-input.sh`，只读采集 OCI revision/dirty label、配置 SHA-256、manifest/approval 绑定和人工维护窗口参数；支持现有 40 位或 64 位 revision，缺失或 dirty provenance 直接 fail closed。

- Agent Memory lineage backfill runner/contract 测试通过 `7/7`，覆盖固定 manifest/receipt hash、unknown field、authority/counter drift、resume、duplicate convergence、目标失败不推进 checkpoint、非法引用和配置边界；JSON Schema examples 通过 Draft 2020-12 校验。v43 migration、sqlc 生成物和隔离 MySQL 8.4 已验证固定 high-water、跨 owner 引用阻断、失败后恢复与精确重放幂等。
- Agent Memory 派生 retention policy 聚焦测试通过 `6/6`，与 lineage 回归合计 `11/11`；覆盖完整/缺失 lineage、受影响与零影响人工复核域、policy/report/decision hash 漂移、authority 提升、64 KiB 双输入和低敏失败。完整 Agent Runtime 为 `586 passed / 26 expected skipped`；TypeScript typecheck/build、Draft 2020-12 strict Schema 示例、canonical Go test/vet、sqlc、Go/TS Proto、Compose、架构文档、观测规则和生产依赖零漏洞门禁通过。
- Agent Memory pre-model lineage 通过 Planner 顺序/失败单测与真实 MySQL 20/20 contract：Context 来源优先、Plan repair 不降级、未知 Task 原子拒绝、foreign owner 隔离、无 Plan 模型结果先 fail closed 再由 lineage 恢复 root attribution；v1→v42 与 v42→v41 回滚删除无 Plan 行均通过。完整 Agent Runtime 为 580 passed / 26 expected skipped，Go test/vet、typecheck/build 与生产依赖零漏洞门禁通过。
- Agent Memory 派生血缘测试覆盖 Context 引用排序/去重、非法 ID、representation 冲突、报告 hash/不变量、CLI 脱敏、并发 Plan 重放、历史缺口、下划线相似 ID 与无 Plan 模型结果；真实 MySQL 8.4 通过 12/12 Runtime contract，并验证 migration v1→v41、48 张表及 v41/v40/v39 分步回滚。完整 Agent Runtime 为 579 passed / 24 expected skipped，Go test/vet、Go/TS Proto、sqlc、Compose、架构文档、观测规则、typecheck/build 与生产依赖零漏洞门禁通过。
- Agent Memory privacy retention 通过领域/Core/sqlc 聚焦测试和真实 MySQL 8.4 contract：两版本 root 全量 tombstone、原字段清除、Context 零召回、越权拒绝、精确重放，以及 v40/v39 分步回滚均通过。
- Agent Memory correction 通过应用/事务/传输/Gateway/Vue 聚焦测试、canonical Go 全仓测试、前端 85 项 Vitest、工具链 3 项与生产构建；真实 MySQL 8.4 完整 migration v1→v39、v39→v38→后续逐级回滚及 owner repository 并发精确重放通过。真实测试同时修复显式 migration 目标版本、SQL NULL Tool arguments 与 MySQL JSON compact 的兼容基线。
- Agent Memory owner 治理测试覆盖稳定分页、并发精确撤销重放、不同原因冲突、owner/tenant 隔离、Gateway 服务身份、客户端 principal 注入拒绝、公开 URI 省略、严格前端响应解析和 authoritative row replacement；Vue 21 个文件共 83 项 Vitest、生产构建及 Chromium、Firefox、WebKit 共 6 项 desktop/mobile E2E 通过。真实 MySQL 8.4 验证 migration v1→v38、v38→v37 回滚、生命周期 CHECK、历史审计回填和 owner sqlc contract；canonical Go test/vet、完整 Agent Runtime 566 项通过且 22 项按环境预期跳过，双端构建与官方 npm audit 零高危漏洞。
- Agent Subscription Shadow Collector 与 evidence 聚焦测试通过 `13/13`，完整 Runtime 通过 `566 passed / 22 expected skipped`；覆盖固定查询/时间、单 series、持续启用、URL 凭据、缺失/多值/非整数/error envelope、CLI 低敏失败、三类严格 Schema 及 Collector-to-evidence 兼容。typecheck/build、官方 npm 源 `0 vulnerabilities`、canonical Go、TS/Go Proto、sqlc、Compose、架构文档和 Agent/服务观测门禁通过。
- Agent Subscription Shadow evidence 合同通过聚焦 `7/7` 与完整 Runtime `560 passed / 22 expected skipped`，覆盖 CLI create/verify、Schema 字段对照、部分窗口、低覆盖、reset、matcher error、低样本、counter 回退、过期和 canonical hash 篡改；typecheck/build、官方 npm 源零高危审计、TS/Go Proto、sqlc、Compose、架构文档和 Agent/服务观测门禁通过。
- Agent Subscription Shadow observation 通过聚焦 `15/15`、完整 Runtime `553 passed / 22 expected skipped`、typecheck/build 和官方 npm 源零高危审计；Prometheus 五条 Agent 规则及测试、Compose、服务观测与架构文档门禁通过。测试固定 matcher match/miss/error、direct-target accepted/ignored、EventLedger 单一路径和默认关闭指标面。
- Agent Subscription create 权限链通过 canonical Go test/vet、Agent Runtime `549 passed / 22 expected skipped`、前端 `78/78`、工具链 `3/3`、两端生产构建与官方 npm 源零高危审计；Go/TS Proto、sqlc、Compose、架构文档和全部维护中的观测规则门禁通过。Chromium、Firefox、WebKit 完整矩阵为 `34 passed / 8 expected skipped`，覆盖认证 Definition/options/create、精确瘦请求、权威响应、撤销与不可用/mobile 状态；WebKit 使用临时用户态兼容库，未修改系统或仓库配置。
- Agent Subscription owner list/revoke 通过 Gateway/config/bootstrap 聚焦 Go 测试、5 项 Vue API/组件测试和 Chromium/Firefox/WebKit 共 6 项路由 E2E；WebKit 在本机通过临时用户态兼容库运行，未修改系统包或仓库配置。服务端覆盖认证派生、游标/limit、撤销原因、nil/错误映射与默认关闭，浏览器覆盖 Bearer、精确撤销、不可用状态清理和 390x844 mobile sheet。
- Go Core mTLS fixture 认证测试通过：正确 Agent 身份可调用，错误 shared secret 返回 unauthenticated，证书 CN 与 caller 不一致返回 permission denied，无客户端证书无法建立可用 RPC；联合演练随后通过真实 mTLS 完成全部 12 类 Core RPC，并输出 v2 低敏证据。
- 外部 MCP 证据契约聚焦测试通过 7 项，并由真实隔离演练生成包含 `collected_at`、`expires_at` 与 `content_sha256` 的文件后经独立 CLI 验证；篡改计数/布尔值/hash、附加字段、未来时间和过期时间均失败关闭。
- 外部 MCP 全栈演练通过：2 个 subscription 事件经 Kafka、MySQL EventLedger 与 Temporal 收敛；首个事件完成 1 次 allowlisted read Tool 和 1 个 Artifact，同事件在 Runtime 重启后由持久 ledger 抑制，第二个事件使用过期 readiness 后 Workflow failed 且 Tool 调用仍为 1。证据固定 `production_authority=false`，共享 Docker 服务未触达。
- C2 C++ shadow 在 Kafka 3.9/librdkafka 2.3.0 上完成首次真实 earliest replay：205 条合法 group event、1 条 poison event 均在 evidence 后提交，最终 lag=0；双实例分担 12 个 partition，停止一例后另一例完成接管并保持 ready=200。归档明确 direct topic 当时为空，尚未证明 direct broker 样本、节点路由或性能收益。
- C1 node2 恢复演练保留一组 fail-closed 与一组 passing 证据：首轮 `f657100` 在 HTTP 恢复后立即负载，40 条 accepted 消息均未持久化，暴露 consumer group 尚未稳定及旧 lag 零值误判；修复后 `ce4b600` 在 fresh project 中验证 PID `887973→898410`、72 members 前后稳定、完整 readiness 13.53s，恢复后 40/40 持久化与投递、峰值 lag 4、settled lag 0。原始证据和 SHA-256 清单归档于 `benchmarks/c1-go-recovery-2026-08-28/`。
- Subscription rollout gate 测试覆盖源证据重算、同 corpus 绑定、review/candidate 独立阻断、hash 漂移、CLI 三态退出码和低敏输出。synthetic 三件套得到 `eligible`，绑定 candidate evidence SHA-256 `2809bbcc5318cb41af6b86f09625abf9ccf05b0f178507459c47ef2e2afbbae3`；完整 Agent Runtime 为 291 passed / 19 expected skipped，该结果只验证 Harness。
- Subscription corpus review 测试覆盖双 reviewer 身份/Review ID 分离、完整 case 绑定、第三方精确裁决、最终标签漂移、review 顺序规范化、黄金哈希、CLI 退出码与正文/身份不回显。synthetic 示例达到 10000 bps agreement；完整 Agent Runtime 为 286 passed / 19 expected skipped，真实 Project Guardian review 仍待受控归档。
- Subscription prefilter Eval 测试覆盖 TP/TN/FP/FN、保守 basis-point 舍入、p95/成本门槛、候选 score/threshold 一致性、case 完整/唯一绑定、corpus hash 漂移、生产规则 matcher 复用、CLI 参数/退出码与正文不回显。synthetic 示例规则基线达到 10000 bps precision/recall、零成本，完整 Agent Runtime 为 280 passed / 19 expected skipped。
- Agent Event Subscription 控制测试覆盖大小写无关集合规范化、稳定 ID、等价创建重放、owner/tenant/Definition/scope 越权、分页隔离、撤销审计与冲突重放、Gateway/Agent 服务身份分离；真实 MySQL 8.4 验证 migration v1→v34、v33→v34 历史回填、sqlc owner 查询、创建/revoke CAS 和显式回到 v27 后重建。
- Runtime promotion 测试覆盖 v2 policy、SHA-256、双人签署、candidate/Definition 漂移、有效期、撤销、冲突重放、active admission 与逐次 context 重查；真实 MySQL 8.4 验证 sqlc grant contract，以及完整 migration v1→v32→v1 和全部历史回填集成测试。
- Approval grant 测试覆盖 active Run/Task、Core scope hash、唯一 exact binding、零/多匹配、过期/消费/撤销过滤、RPC 服务身份与 NotFound 收敛、TS scope/arguments/nonce 摘要复核及 write gate adapter；sqlc 查询固定 `LIMIT 2`，不静默选择候选。
- MCP Message write projection 测试覆盖审批消费、Tool begin、同 Invocation ID 的 Command RPC、action finish 的严格顺序，以及 direct conversation、active mode、显式 executor、Command kind 和返回引用漂移的 fail-closed 行为；现有 read Tool 回归保持通过。
- MCP Message Command 测试覆盖 Core 派生 Command ID、canonical 参数黄金向量、Tool/Approval/Task/Run/Capability/身份漂移、Agent event lineage、RPC 服务身份与 TS 返回证据复算；Tool runner 和 Approval gate 对同一参数使用统一排序 canonical JSON。
- Tool action lineage 测试覆盖已消费审批、Task/Capability/参数摘要与资源 scope 漂移、read/write 引用隔离、Command kind/capability 对应、sender-scoped receipt、Message UUID/身份/会话/类型冲突、protobuf/TS 映射及 sqlc 持久化；生产 MCP Server 的 read-only 投影回归保持通过。
- MCP 限流测试覆盖 JWT principal 派生、跨 Task 共享额度、principal 隔离、429/`Retry-After` 向上取整、被限请求不进入代理、DELETE 清理旁路、Redis 缺失 fail closed，以及消息发送 fail-open 兼容性；真实 Redis 验证两个 Limiter 实例共享计数和 TTL 到期恢复。
- MCP ToolCall 测试覆盖 durable begin、权威身份绑定、只读风险门禁、参数/结果哈希、64 KiB 上限、异常脱敏、OTel span 终止、RPC principal 拒绝、重复 begin 与单次终态；真实 MySQL 8.4 验证 migration v30 全量 `up→down` 链、FK/CHECK 约束及 sqlc contract。Go 聚焦测试、128 项 TS 测试、typecheck/build 和双语言生成物通过。
- MCP 认证挂载测试覆盖默认零路由、错误服务密钥、JWT principal 覆盖、Task/Run 绑定、异步 Core 上下文解析、foreign principal NotFound、内部凭据与 body 长度头清理、SSE/Session header 透传及无效标识拒绝；Go 全量 test/vet、定向 race、125 项 TS 测试、typecheck/build、双语言 Proto 漂移、Compose、架构文档和生产依赖零漏洞门禁通过。
- Agent Memory 单元与传输测试覆盖内容/生命周期校验、Task 固定身份、conversation scope 越权、伪造 principal、provenance 映射、稳定排序和 `untrusted` Context 注入；真实 MySQL 8.4 验证主体隔离、sqlc 创建/查询/撤销、类型/状态/priority CHECK 及 migration v29 `up→down→up`。聚焦 Go/TS 测试、typecheck 和 build 通过。
- Agent Event Subscription 单元与契约测试覆盖未知过滤器字段、控制字符、Definition 撤销/过期/漂移、resource scope 越权、RPC 服务身份、零匹配零台账/零计划、多匹配稳定选择和 Task 绑定冲突；真实 MySQL 8.4 验证默认 21 字符 Agent ID、sqlc 创建/查询/撤销、外键/CHECK 约束及 migration v28 `up→down→up`。Go/TS Proto、sqlc 生成物、112 项 TS 测试、typecheck 和 build 通过。
- Node 22.12.0/npm 10.9.0 干净容器通过冷安装、3 项 Vite 工具链契约、53 项 Vitest、`vue-tsc`、Vite 8 生产构建和双重 audit；HTTP/WS 开发代理、`/app/` 静态资源路径及 Chromium/Firefox/WebKit 全部适用 E2E 场景通过。
- User status 契约测试固定 JSON Schema ID、版本、默认值、枚举与 Go 常量一致；真实 MySQL 8.4 migration 测试覆盖历史 `0` 回填、默认写入、非法值拒绝和保留归一数据的降级边界。
- maintenance 单元/契约测试覆盖双审批与职责分离、15 分钟有效期、候选绑定、24 小时语义复核、元数据回补、对象缺失/漂移/过期和可复算 authorization/receipt 示例；真实 tmpfs MinIO 验证 inspect 身份可 Stat 且无法 List/Put/Delete，生产 Agent protobuf 继续没有 Artifact 清理方法，第 21 个后端二进制及当前源码镜像构建通过。
- 真实 MySQL 8.4 验证对象键存在/缺失查询与 migration v26 回滚重建，真实 tmpfs MinIO 验证 audit 用户可列举固定前缀且无法 Get/Put/Delete；联合环境运行 CLI 后输出年轻对象隔离和有效 evidence SHA-256。单元测试覆盖过期孤儿、已引用对象、年轻对象、异常键、24 小时门槛、样例上限与报告复核，新增第 20 个后端二进制的当前源码镜像构建通过。
- 真实 tmpfs MinIO 验证 Artifact 专用身份可幂等 Put/Get，无法删除、写入通用文件 bucket 或逃逸对象前缀；`dipoleplatform` 可正常写入/清理文件对象且无法写 Artifact bucket。三份 Compose 配置渲染和两次连续 `minio-init` 均通过，Go 全量包 test/vet、定向 race、TS 90 项测试、typecheck/build、sqlc/Go+TS Proto 漂移及 19 个当前源码后端二进制镜像构建门禁通过。
- 真实 Temporal CLI 1.8.2 / Server 1.31.2 验证 read Activity 在模型、Plan 与 Capability 完成后丢失 ACK 并重试时，provider 和 Capability 均只执行一次；真实 MySQL 8.4 验证结构化输出恢复、持久预算漂移拒绝、已完成 Run 精确重放，以及完整 migration v23 `up→down` 链。TypeScript 绑定测试覆盖 Kafka 仅启动稳定 Workflow、inline planner 不再执行和伪造 Task/Run/event 拒绝。
- 真实 Temporal CLI 1.8.2 / Server 1.31.2 验证 Approval 在 Worker 替换前仅持久创建一次、恢复后保持同一等待点、精确 Signal 只解析一次且解析完成后才继续 Step；真实 MySQL 8.4 验证 pending 精确创建重放、绑定冲突拒绝、16 路并发批准收敛、approve/deny 竞争仅一个 CAS 获胜、终态重放和伪造 principal/cross-Task 拒绝。
- Temporal 状态机、稳定 Workflow ID、重复启动策略、默认关闭配置和 Worker 启停单测通过；真实 Temporal CLI 1.8.2 / Server 1.31.2 验证 admission 仅执行一次、Step 两次失败后第三次成功、替换 Worker 从历史恢复审批等待、Signal 后完成、terminal Activity 失败后重试，以及超步数精确写入 failed。真实 MySQL 8.4 验证 failed/cancelled Run 首次提交、精确重放、持久读取与冲突终态拒绝；Agent 全量为 62 passed / 13 skipped，TypeScript typecheck/build 和 Proto drift 门禁通过。
- 已通过 Context Compiler 的确定性顺序、section/global budget、full→compact、optional omit、required fail-closed、重复 ID 和不可信内容隔离测试；真实 MySQL 8.4 验证 v22 manifest 持久化与 `up→down→up`，最终 schema version 为 22。
- 已通过 Go/TypeScript Task/Run ID 黄金向量、Agent application/transport/bootstrap 测试、真实认证 gRPC 最小权限测试、真实 MySQL 8.4 Agent Policy Repository 与 Step claim 合同，以及 migration v21 `up→down→up`；最终 schema version 为 21，`agent_runs` 和 Step claim 字段均存在。真实 Go Core 与 Node grpc-js 完成 `AdmitRun→ListConversations→CompleteRun→replay`，重放返回同一 completed Run。
- 已通过真实 MySQL 8.4 Agent policy 合约：使用默认长度 Assistant/principal 初始化 Definition，持久 Task 固定版本并完成 `running→completed`；同时覆盖 Definition 撤销/过期、新版并发出现、重复触发和 resource scope 越权拒绝。
- 已通过 Chromium、Firefox、WebKit 共 12 项真实 IndexedDB/Session Playwright 验收；实验性 Chromium quota override 未拒绝 IndexedDB 写入并被明确标记为 skip，未作为 AD-025 完成证据。
- 已通过 MySQL 8.4/Cassandra 5.0.9 重复发送 hydration 演练：真实 Timeline 命中不读取 MySQL 正文，Metadata 恢复 legacy ID；缺失和历史无 Seq 回退，v14 历史回填成功；运行时要求显式启用 Cassandra，指标拒绝未定义标签。
- 已通过 MySQL 8.4、MinIO object lock 与 Elasticsearch 9.5.2 联合演练：专用账号按 2/2/1 清理 5 条水位内 Search mutation，保留无关 Outbox 并拒绝 Core 表访问；删除本地副本后按精确对象版本恢复，从空索引重建并完成 3/3 hash 对账、Alias 正向切换和回滚。
- 已通过 Search cleanup dry-run、安全证据、未发布事件阻断、维护窗口/operator 门禁和批次中断后可重入测试。

- 已通过 MinIO/MySQL 8.4/Elasticsearch 9.5.2 三存储恢复演练：对象发布返回固定版本，保留期内无 bypass 删除失败；删除本地归档后按 receipt 恢复，再删除历史 Message Outbox，最终重建、3/3 hash 对账、Alias 正反切换均通过。
- 已通过 MySQL 8.4/Elasticsearch 9.5.2 归档恢复演练：导出固定 mutation snapshot 后删除历史 Message Outbox，仍完成两个物理索引重建、3/3 hash 对账、Alias 正向切换和回滚；陈旧 MySQL snapshot 被阻断，目标篡改返回退出码 2。
- 已通过 MySQL 8.4 migration v12 空库、v11 历史回填、重复执行和回滚测试；Repository 合约覆盖 Metadata 双键定位、hash、Outbox 失败原子回滚，以及删除 Message 正文后的文件授权。最小权限 write-owner smoke 覆盖 projector/atomic 模式与 Metadata 表启动探针。
- 已通过 43 个前端测试；新增群同步引擎、IndexedDB v2→v3 升级、群位点单调性、逐账号清理和浏览器重开补交 ACK 契约。
- 已通过 29 个 Web Inbox Sync/Session 单元测试，覆盖本地优先恢复、Sync/Offline 多页推进、事务失败禁止 ACK、断点 ACK 补交、非推进页拒绝、分页上限、IndexedDB 重开与旧 schema 升级、账号隔离、游标单调性、高低水位淘汰、会话保底、大页面硬上限、quota 分类、首轮基线、宽限匹配、单边超时、状态溢出、损坏恢复、私聊语义过滤、终止 singleflight、先撤销后清理、存储/清理失败隔离、401 接线和跨账号登录。
- 已通过 Web Sync `off|shadow|primary` 三种生产构建、聚合遥测 Handler 边界测试、Prometheus Collector 测试、OpenAPI 生成和全量 Go 测试。
- 已通过 Web Sync Prometheus 七条规则静态检查与固定时序测试，覆盖窗口不足、样本不足、干净晋级、终态差异和比较器溢出。
- 已通过 Pencil 结构与截图检查，确认 Sync 状态矩阵、desktop 及 mobile frame 无 clipping 或残留 placeholder，并导出批准预览。
- 已通过 MySQL 8.4 与 Cassandra 5.0.9 真实 Sync hydration contract，覆盖一致结果、payload mismatch、缺少 Cassandra 投影及 MySQL 主结果隔离。
- 新增真实 MySQL 8.4 写责任烟测，覆盖 Sync/Message 最小权限启动探针、Message+Outbox 提交但不写 Inbox、Projector 收敛和 atomic 回退恢复。
- Sync Projector 三节点 Kafka/MySQL smoke 增加启动前 backlog、在线双运行、热群跳过、永久失败 retry/DLQ 和无脏行验证。
- 已通过锁文件冷安装、6 个前端 Search 单元/组件测试、`vue-tsc`、Search 开启/关闭两种 Vite 生产构建；生产依赖 `npm audit --omit=dev` 为零漏洞。
- 已通过 Pencil 全文档结构检查，确认 Search 八个 frame 无残留 placeholder、clipping 或未命名图层，并导出 desktop 1440x900 与 mobile 390x844 批准预览。
- 已通过 `go test ./...`、`go vet ./...` 和 `go mod verify`。
- 已通过新增同步 repository、service、handler 及消息 service 的定向 race 测试。
- 已通过 Kafka `sync_fanout` 新旧字段契约测试和幂等目标隔离测试。
- 已通过 MySQL 8.4 双事务提交顺序集成测试、`FOR UPDATE` 方言测试和 Sync 锁行回滚测试。
- 已通过 MySQL 8.4 空库升级、重复执行、未来 migration 兼容和 baseline 回滚测试。
- 已通过 sqlc Store 的 MySQL 8.4 提交、回滚与幂等插入集成测试。
- 已通过全部 sqlc Repository 的真实 MySQL 功能契约，覆盖幂等、状态转换、排序、权限、事务回滚、租约和同步顺序。
- 已通过 sqlc 同用户并发提交顺序测试，确认 Inbox `sync_seq` 与提交顺序一致。
- 已通过 Message gRPC bufconn 契约、结构化错误往返、全量 Go、vet、race 和模块完整性测试。
- 已通过 Core 五项能力与 Sync Timeline 页面的 bufconn 往返测试，覆盖调用身份、权限结果、成员快照和持久游标映射。
- 已通过 Cassandra MessageStore 影子读单元/race 测试和 Cassandra 5.0.9 跨 bucket Seq 范围真实 contract；差异、跳过与 Cassandra 错误均不改变 MySQL 响应。
- 已通过 MySQL 8.4 与 Cassandra 5.0.9 真实读路由 contract：before/after Seq 完整页由 Cassandra 返回，人工删除一行后按同一 Seq cursor 整页回退 MySQL。
- 已通过 Cassandra 主读 payload 篡改演练：Seq 保持连续时内容差异仍被抽样核验识别并返回 MySQL 完整页。
- 已通过 Kafka legacy/v1 minor/v2 兼容、永久 schema 错误隔离、DLQ 诊断 header 和 Outbox schema header 测试。
- 已对 Local 与 gRPC transport 运行同一套八项 MessageApplication 行为契约，覆盖文本/文件命令、历史、热群增量和离线查询。
- 已通过内部 gRPC 服务认证集成测试，覆盖合法凭据、缺失凭据、错误密钥、未授权调用方与无效启动配置。
- 已通过 Core File Capability 的 Local/Remote 契约、非所有者隐藏测试，以及 MessageService 远程文件消息测试。
- 已通过 shadow query 契约：primary 四类命令各执行一次、shadow 命令调用为零、四类查询均参与比较，差异不改变 primary 响应。
- 已通过 TLS 1.3 mTLS 真实网络测试、非 loopback 明文拒绝、认证 caller 与 payload caller 冲突拒绝测试。
- 已通过 MySQL 8.4 临时受限账号验证：五张 Message 所需表可读取，Core `users` 表返回权限拒绝，测试账号随后删除。
- Message History mTLS gRPC loopback 三轮为 `69,339-69,618 ns/op`，低于 M4 `<1 ms/op` adapter 门槛；完整端到端 P95/P99 继续由 G0 跟踪。
- 已通过 Gateway 本地 health、真实 HTTP 反向代理、依赖门禁、Gateway/Core 独立 RPC 身份和 WS Gin 全局状态 race 测试。
- 已通过本地 Core、Message、Gateway 三进程 smoke：Gateway 注入不可达 MySQL 仍正常启动，health 返回成功，Core HTTP 代理返回预期认证响应，Core remote WS 路由关闭。
- 已通过隔离 Compose 容器验收：migration 成功退出，Core/Message/Gateway 经 mTLS 冷启动并达到 healthy，公开代理与 WS 所有权检查通过。
- 已通过 MySQL 8.4 会话 Seq migration 回填/回滚、同会话事务锁顺序和 24 路并发连续性测试，并验证失败事务不消耗序号。
- 已通过 Kafka legacy payload、Message protobuf 往返、Sync RPC、WS 映射和 shadow comparison 的 Seq 兼容测试。
- 已通过 MySQL 8.4 部分已读、并发新消息隔离、乱序投影保护、24 路设备 ACK 单调性和多设备隔离测试。
- 已通过 MySQL 8.4 MessageStore/SyncStore/SearchIndex contract，覆盖会话 Seq cursor、索引幂等更新、范围隔离、排序和删除。
- 已通过 MySQL 8.4 热群高水位回填、消息事务原子回滚、设备群 ACK 单调性，以及 Sync 权限和 Message/Sync gRPC 契约测试。
- 已通过 created mutation 新旧 payload 归一化、event type 一致性、future mutation revision/actor 门禁和 direct/group 事件命名测试。
- 已通过三节点 Kafka 故障演练：单 broker 停止后继续确认写入，低于 min ISR 时拒绝 ACK，恢复 quorum 后消息完整可消费。
- 已通过 Kafka consumer group 演练：两个 member 各持有 3 个 partition，单 member 退出后剩余 member 接管 6 个 partition并将 lag 恢复为 0。
- 已通过 Kafka 可观测性演练：Prometheus 规则有效，consumer lag、retry/DLQ 增量和单 broker 故障造成的 ISR 缺口均可查询，broker 恢复后缺口归零。
- 已通过 MySQL Router writer 演练：停止 PRIMARY 后同一 `database/sql` 池约 4.1 秒内连接新 writer，切换前后已提交记录均可见，旧节点通过 AdminAPI 成功 rejoin。
- 已通过两个 migration runner 对空库并发执行测试，双方均成功且 migration ledger 保持唯一完整。
- 已通过 Redis Sentinel 演练：停止当前 master 后约 4 秒完成切换，同一客户端恢复读写与 Pub/Sub，Presence、热点和限流状态可用，旧 master 重新加入为 replica。
- 已通过隔离 Storage Lab 演练：Cassandra 与 Elasticsearch 健康启动并完成临时 CRUD，Core、Message、Gateway 和客户端生产读路径保持断开。
- 已通过 Elasticsearch 9.5.2 真实 contract：重复 Bootstrap 校验 mapping/Alias，external revision 支持重放、更新与旧事件 no-op，作用域搜索无隐藏会话泄漏。
- 已通过 MySQL 8.4 与 Elasticsearch 9.5.2 版本化 Search contract：同 revision 冲突可检测，tombstone 后旧正文事件无法恢复搜索结果。
- 已通过三节点 Kafka/Elasticsearch Search Indexer smoke：created r1、recalled r3 与迟到 edited r2 收敛为 revision 3 tombstone。
- 已通过 MySQL 8.4/Elasticsearch 9.5.2 Search 恢复演练：created/edited 与 created/recalled 折叠为 3 个最终状态，固定高水位对账一致，目标 hash 篡改返回退出码 2。
- 已通过 Elasticsearch Alias 正反切换演练：old→new 与 new→old 均保持双 Alias 原子所有权；新增 Outbox mutation 后陈旧快照被拒绝且 Alias 未漂移。
- 已通过 MySQL 8.4 Search 授权范围合约与 Core gRPC 往返测试：私聊和有效群成员关系可见，无成员关系或无效群状态均不可见，缺失 principal 被拒绝。
- 已通过 Search Application、内部 RPC 和 Composition Root 测试：空 scope 不访问 Elasticsearch，基础设施错误保持有界，启动 readiness 不产生索引写操作。
- 已通过 Elasticsearch 9.5.2 Search Service 真实契约：同关键词的授权与隐藏文档经 Core scope、内部 RPC 和 read Alias 查询后只返回授权结果。
- 已通过 Gateway Search 路由测试：缺失令牌返回 401，合法会话调用 Search Application，精确路由不进入 Core HTTP 反代，依赖错误不泄漏内部详情。
- 已通过 Cassandra 5.0.9 Timeline contract：bucket 边界与 Seq 倒序正确，重复 payload 安全重放，冲突 payload 拒绝覆盖。
- 已通过 Kafka/Cassandra projector 演练：独立 consumer group 获得 assignment 后消费两次相同 created event，最终只生成一条 Timeline 记录。
- 已通过 MySQL 8.4 Backfill lease 合约，以及 MySQL/Cassandra 恢复演练：失败批次 checkpoint 不前移，恢复时安全重放 duplicate，最终固定高水位全部完成。
- 已通过 Cassandra 对账演练：干净快照全量匹配；人工篡改后检测 hash 与样本差异并返回退出码 2，差异报告不包含消息正文。
- 已通过 MySQL 8.4、MinIO Object Lock 与 Cassandra 5.0.9 联合演练：发布并按固定对象版本恢复完整消息归档，删除 MySQL `messages` 正文后仍重建 3 条 Timeline 并 3/3 对账；换源、内容篡改和保留期内删除均被拒绝。
- 已通过真实 MySQL 8.4 Message/Sync 最小权限演练和完整微服务镜像 smoke：atomic/projector 写责任正确，禁止多余表操作与 Core 访问；migration 后自动授权，Message/Sync 权限门禁、mTLS、Gateway health 和 Core 代理全部通过。

### 已知问题

- 历史 lineage backfill 已具备 v43 MySQL checkpoint、sqlc source/target、owner-scoped adapter、隔离数据库测试和默认 dry-run 的审批 CLI；当前仍缺少共享环境执行审批记录与 production rollout 证据，不能在共享环境执行回填。sqlc v1.31.1 不支持 inline JSON_TABLE，当前采用 sqlc manifest retrieval、Go 严格展开和 owner-scoped sqlc lookup。
- Memory root 派生影响已具备逐域离线 retention policy 决策，但尚未实现 Shadow plan、Step、Artifact、Agent Message 或 Temporal history 的字段级擦除器；v42 已为受管 Model planner 建立模型前 root attribution，历史/旁路缺口继续由 owner-scoped 未归因计数阻断。公开 owner 擦除 API、自动 retention Worker 与账号级隐私删除继续关闭并由 `AD-035` 跟踪。
- Memory v1 已提供默认关闭的 owner list/revoke HTTP/Pencil/Vue 闭环和追加式撤销审计；自动写入、append-only 纠正/版本冲突、Observation/Reflection Worker、置信度策略及 hybrid/vector retrieval 仍待完成。共享 Shadow 仅在已有受控记录时读取，详见 `AD-035`。
- Event Subscription 已具备默认关闭的公开 Definition 目录、authenticated conversation chooser、owner list/create/revoke HTTP/Pencil/Vue 闭环、撤销审计、provider-neutral 离线预筛 Eval 和双评审 agreement 合同；尚未归档真实 Project Guardian corpus/review report、embedding/小模型 candidate evidence或 subscription Runtime 灰度证据。共享环境继续固定 `direct_target`，详见 `AD-034`。
- Sync Inbox、旧 Offline 与默认关闭的幂等 hydration 尚未完成替代链路观察；Cassandra 恢复工具已可独立使用不可变完整消息归档，正文退役其余条件继续由 AD-019 跟踪。
- `/messages/offline` 真实对照观察窗口仍待执行；Web 本地 Sync Engine 默认关闭，旧客户端继续使用数据库 ID cursor。
- Web IndexedDB 已统一会话清理和容量淘汰实现；真实浏览器配额、共享设备和进程强退验收仍是默认启用门禁，记录为 `AD-025`。
- Web 协议对照首批仅覆盖收到的私聊消息；群聊存在普通群 fanout 与热群 notify/pull 两套语义，需按群类型建立独立比较契约后再纳入。
- 独立 Message Service 已停止在代码中读取 Core Repository；当前仍与 Core 共用 MySQL schema 和数据库账号，最小数据库授权记录为 AD-015。
- M5 的非 WS HTTP、Swagger 与静态 Web 仍由私网 Core 提供，Gateway 通过反向代理暴露；后续服务拆分前必须保持 Core 不直接暴露公网。
- HTTP Handler 历史测试并行修改 Gin 全局模式，整包 race 门禁会产生数据竞争；新增 Sync Handler 定向 race 已通过，后续治理记录为 AD-016。

## 发布归档

当前尚未建立正式版本标签。首次发布时，从 `Unreleased` 下沉内容并使用以下格式：

```markdown
## [X.Y.Z] - YYYY-MM-DD

### 新增

- 描述新增能力及影响范围。

### 变更

- 描述行为、依赖或接口变化。

### 修复

- 描述已修复问题及触发条件。

### 安全

- 描述安全修复；敏感细节应链接到受控公告。

### 弃用

- 描述即将移除的能力、替代方案和计划移除版本。

### 移除

- 描述已移除能力及替代路径。

### 迁移说明

- 列出数据库、配置、API、消息协议或部署顺序要求。

### 验证

- 列出本次发布实际执行的测试和检查。

### 已知问题

- 列出尚未解决且可能影响使用、开发或发布的问题。
```

# 2026-08-30 Remote C1 200-member group fan-out baseline

- 修复 `remote-dev.sh bench` 通过 SSH 传递可选参数时的空值左移；所有可选工具链、代理和 workload 参数使用显式哨兵并在远端解码，入口契约测试 `10/10` 通过。
- Remote GPU provenance 门禁拦截旧候选镜像后，重建并绑定 `master` `959ac70d` 的 `dipole-server:c1-959ac70d`；正式入口完成 200 成员 `group_blast`，200/200 VU、10/10 消息 accepted/persisted、群 Inbox `2000` 行、1990/1990 回执、投递率 `100%`、HTTP failure `0%`。
- 端到端平均/P50/P95/P99/最大值为 `126.84/121/167/169/169 ms`，Kafka 峰值 lag `1`、结算 lag `0`，Node1/2/3 CPU 峰值 `72.14%/20.99%/19.85%`；候选拓扑清理后无 `dipole-c1` 残留。热群 notify/pull、背压阈值和 broker/Redis 故障回切仍待独立验收。

- 2026-08-30：扩展 `remote-dev.sh bench` workload 白名单，支持选择 `bench_group.js`、隔离 `PHONE_PREFIX`、warm-up、激活等待和 hot-group 阈值；默认仍使用原有 `bench.js` 和默认参数，入口契约测试 `10/10` 通过。
- 2026-08-30：使用 `bench_group.js` 和 `PHONE_PREFIX=157` 完成 200 成员热群观察：warm-up `60`、正式消息 `20`、`3980/3980` 预期回执、投递率 `100%`、HTTP failure `0%`；群 Inbox 写入 `0`，Conversation message projection `80`，Kafka peak/settled lag `54/0`，P50/P95/P99 `296.5/2241.55/2521ms`。报告当时的阈值字段为空，行为证据用于验证 notify + pull，阈值元数据由后续入口修复补齐。
## Unreleased

- 2026-09-01：Remote GPU 长驻 Agent Shadow 体验项目已将 Core 静态资源更新到 `6d274a54`；Core、Gateway 与 Timeline 路由健康，部署前端资产包含等待审批入口。复用候选 `.env` 的单服务更新现要求显式传入 `DIPOLE_INTERNAL_CERT_DIR`，避免 mTLS 证书 bind 路径漂移。
- 2026-09-01：Agent Task Timeline 对具有 approval ID 的 `waiting_approval` 事件提供审批页入口，使创建任务后的只读轨迹可进入既有 owner-scoped Human-in-the-loop 页面；已完成和无效事件保持无操作入口。
- 2026-09-01：开发工作流收敛为主轨道连续 worktree 与里程碑提交；Remote GPU 开发验证可直接更新本轨道已有 Compose project，缺失依赖可经 sudo 安装。仅在明确冲突时新建隔离项目，普通 Smoke、脚本试验和文档验证不再创建完整临时集群。
- 2026-08-30：Remote development 新增 `web-sync-bundle` 动作，提交同步后在 Remote GPU 生成绑定 revision 的 shadow bundle；该动作不启动 Compose、不申请 GPU，归档输出位于 `/tmp` 并保持不可覆盖和 `0600` 权限。
- 2026-08-30：A6 新增 `scripts/package-web-sync-bundle.sh`，将候选 Web 构建按完整 Git revision、显式 Sync 模式和稳定 tar 元数据打包为不可覆盖的 `web-sync-bundle.v1`；报告权限固定为 `0600`，源目录内输出会 fail-closed，便于后续观察会话复核 bundle 哈希。
- 2026-08-30：Sync/Message Inbox ownership smoke 新增可选 `SMOKE_REPORT_FILE` 机器可读 receipt，绑定源码 revision、dirty 状态、projector/atomic 模式、非破坏性回滚动作、退出状态和临时容器清理结果；报告以 `0600` 权限原子写入，默认路径与 GPU 并行策略保持不变。
- 2026-08-30：完成 Multipart fault-matrix 联合验收：Remote GPU 使用官方 Prometheus `3.5.0` `promtool` 通过告警规则与 firing timeline、确定性 Go contract、真实 MinIO/Redis reconciliation 和 Redis restart smoke；GPU 任务前后未变化，临时资源已清理。A7 默认预签名切流、生命周期指标和 Alertmanager 联调继续保留为后续工作。
- 2026-08-30：新增 Multipart fault-matrix 聚合入口，统一执行 Go contract、promtool、真实 MinIO/Redis reconciliation 和 Redis restart smoke；Remote GPU 已通过确定性与两组真实存储矩阵，promtool 首次镜像拉取因 registry 无进展中止，未伪造完整矩阵结论。
- 2026-08-30：预签名 Multipart Gateway 代理接入按客户端地址的文件上传限流；超限请求在进入 MinIO 代理前返回 `429` 与 `Retry-After`，允许请求保持签名校验和既有超时边界。
- 2026-08-30：预签名 Multipart Gateway 代理新增可配置上游响应超时，默认 `30s`；上游对象存储超时返回 `502`，避免长连接无限占用，配置异常 fail-closed，relay 回退路径保持不变。
- 2026-08-30：补充 Multipart HTTP Gateway 初始化限流回归：超过文件上传窗口时在 `initiate` 阶段返回 `429`，不调用 Core/MinIO，并保留 Retry 语义；普通上传和预签名代理默认路径保持不变。
- 2026-08-30：补充 Multipart reconciliation 指标发布失败测试：模拟原子 rename 目标冲突，确认旧目标不被替换、临时文件自动清理；promtool 告警规则与默认指标路径保持不变。
- 2026-08-30：Multipart cleanup 将 MinIO `NoSuchUpload` 竞态视为已收敛的幂等结果并记录为 `already_gone`；列举与 Abort 之间 upload 已被其他 worker 清理时不再误报失败，其他 Abort 错误仍保持 fail-closed。
- 2026-08-30：Multipart 真实对账 smoke 增加可选 Redis 重启故障注入：在匹配状态建立后重启隔离 Redis，验证 metadata 丢失被识别、MinIO 未完成 upload 仍可清理、孤儿 Redis drift 仍可报告；默认 smoke 路径不变，GPU 任务可并行运行且测试资源自动清理。
- **A7 Multipart cleanup fail-closed**：MinIO 未完成上传扫描错误现在会进入结构化报告并阻止清理命令成功返回，避免部分扫描被误判为完整生命周期证据。
