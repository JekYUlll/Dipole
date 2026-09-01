# 设计更新日志

本文档记录 Dipole canonical Pencil 设计的用户可见变化，格式遵循 Keep a Changelog。日常修改统一写入 `Unreleased`。

## [Unreleased]

- 归档 Contact Directory 与 Group Directory 的 Pencil canonical desktop/mobile 导出，补齐 `design/README.md` 页面索引；导出来自现有 canonical frame，未复制第二份 `.pen` 文件。

- 增加 Device Directory authenticated desktop/mobile 页面与 Chromium canonical 基线；明确 owner-scoped 会话、设备撤销操作和服务不可用清空策略。

- V3 Logo 追踪新增颜色量化与二值 alpha mask 预处理，修正原始 VTracer 结果中的栅格边缘残影；IM/Agent 完整锁定稿与紧凑标志均已重新生成并保留透明背景。

- 增加 Group Directory authenticated desktop/mobile 页面与 Chromium canonical 基线；页面复用 V3 IM 紧凑 mark，固定群组只读边界、热群 `notify + pull` 状态和服务不可用清空策略。

- V3 品牌套件新增由 `LOGO_V3.png` 经 VTracer 追踪生成的 IM 紧凑 mark；完整锁定稿、IM mark 与 Agent mark 均记录独立裁剪范围和可复现生成脚本，页面继续禁止使用手工重绘版本。

- 增加 Contact Directory 的 authenticated desktop/mobile 页面实现与 Chromium canonical 截图基线；页面使用 V3 IM Logo，明确 server-owned、read-only 关系边界和不可用清空策略。

- V3 Logo 资产改为透明背景的 PNG→SVG 追踪结果；完整锁定稿与紧凑 Agent 标志均采用原图可见边界，避免将概念标题和色板带入产品页面。

- Agent Definition Catalog 已完成 authenticated owner-scoped 只读页面，补齐 desktop/mobile Chromium canonical 基线；页面不提供编辑、创建或能力授予动作。

- Agent Approval 与 Elicitation 的 Mobile/Form 设计帧已获得受控 Chromium 视觉基线，固定 390x844 单列断点、安全披露和操作区顺序。

- V3 品牌资产改用 VTracer 从 `docs/images/LOGO_V3.png` 的独立区域生成；完整 Agent 锁定稿保留顶部轨道节点，紧凑标志保留完整轨道构图，追踪参数统一由 `scripts/trace-brand-assets.sh` 管理。

### 验证

- V3 前端视觉基线已完成一次逐页复核并刷新 9 个 Chromium canonical 快照，覆盖 Agent Elicitation、Artifact、Definition Catalog、Governance、Task Timeline、Chat、File Directory 和 Settings；Chromium 40 项通过、2 项按测试设计跳过，Firefox 与 WebKit 独立验收均为 18 项通过、12 项按测试设计跳过。快照只记录当前 V3 设计实现，不替代 Pencil 源文件。

- 增加本地 `.pen` 结构门禁，校验 canonical 设计变量、核心 desktop/mobile frame、可复用组件和 placeholder/未命名节点；该检查不修改设计文件，也不替代 Pencil 视觉评审。

### 变更

- Agent Task 审批页完成 Vue 首个实现切片：沿用既有 Agent Approval 设计基线，展示任务/请求绑定、风险提示、过期和不可用状态；入口由 `VITE_AGENT_APPROVAL_ENABLED` 默认关闭，未扩展 canonical `.pen`。
- 将 `.pen` Foundations 的颜色、字体、间距和圆角变量映射为 `frontend/src/styles/design-tokens.css`，应用壳层与 Search 工作区复用同一组 `--dp-*` token；新增 Vitest 契约测试，后续设计稿更新需同步调整该文件。

### 验证

- Agent Task Timeline v1 增量设计已建立可复用 brief `design/agent-task-timeline-v1-brief.md`；Pencil CLI `0.3.5` 在两次受限模型调用中均未在超时窗口内完成，safe-edit wrapper 保留 canonical `.pen`，未生成导出图。F3/F4 视觉交付继续保持待处理状态。

### 新增

- 增加 Agent Event Subscription desktop/mobile 创建流程和七类创建状态，归档 `exports/agent-subscription-create-v1/` 的 2x 评审基线。
- 增加 Subscription Create Option 与 Authority Summary 两个可复用组件；canonical 文件扩展为 44 个顶层 Frame 和 21 个可复用组件。
- 增加 Agent Event Subscription desktop owner 管理页、六态契约矩阵和 mobile 撤销确认层，并保存 `exports/agent-subscription-v1/` 的 2x 评审基线。
- 增加 Subscription Status、Filter 和 Row 三个可复用组件；canonical 文件扩展为 41 个顶层 Frame 和 19 个可复用组件。
- 增加 Agent Elicitation desktop 普通 Form、七态契约矩阵和 mobile 表单，并保存 `exports/agent-elicitation-v1/` 的 2x 评审基线。
- 增加 Elicitation Source、Field 和 Status 三个可复用组件；canonical 文件扩展为 35 个顶层 Frame 和 16 个可复用组件。
- 增加 Agent Workflow Repair desktop 审计页、六态契约矩阵和 mobile 审批底部层，并保存 `exports/agent-repair-v1/` 的 2x 评审基线。
- 增加 Repair Status、Evidence Diff 和 Approval Step 三个可复用组件；canonical 文件扩展为 29 个顶层 Frame 和 13 个可复用组件。
- 建立 canonical Pencil F1 基线，增加 foundations、可复用 IM 组件、Login/Chat desktop/mobile、离线恢复、只读权限、空态、加载、错误、Agent Approval 和设计评审清单。
- 合并 F1 与后续 Search/Sync 画板为单一 canonical 文件，统一同名 token，并保留 23 个顶层 Frame 和 10 个可复用组件。
- Sync 状态矩阵增加 Storage Full，明确浏览器配额不足时本地消息仍可读、安全游标不会前移，并提供释放空间后的重试入口。
- 增加 Sync Status 可复用组件，以及同步 Restoring、Current、Offline、Error 状态矩阵。
- 增加消息恢复 desktop/mobile 页面和 `exports/sync-v1/` 批准预览，展示安全游标、本地落库、设备 ACK 与断网可读状态。
- 建立 `design/dipole-ui.pen`，定义浅色画布、深色导航数据面、绿色强调色、Manrope/Noto Sans SC 字体和基础间距圆角 token。
- 增加 Search Field、Search Result、Search Skeleton 与 Search State 四个可复用组件。
- 增加消息搜索 desktop/mobile 的 Results、Loading、Empty、Error 四态设计。
- 增加 `exports/search-v1/` 批准预览，作为 Vue 实现和后续视觉回归的首个基线。

### 设计决策

- Pencil 增量编辑必须通过 `scripts/pencil-safe-edit.mjs`：真实 CLI 调用写入临时 `.pen`，同时校验文档结构和导出资产，成功后才原子替换 canonical 文件；CLI 超时、进程异常或导出缺失均保留现有设计并进入 `AD-044` 记录。
- Agent Task Timeline v1 后续编辑优先按 brief 拆分为单 frame 小批次；每批必须同时验证节点命名、canonical JSON 结构和 2x 导出，失败时只保留 brief 与证据，不修改现有设计基线。
- `sync.item.notify.v1` 的 primary 交互只负责按会话序号补拉并合并已验证消息，不能直接把通知正文当作事实；目标序号或 UUID 校验失败时不展示、不推进客户端状态，服务端 Cassandra 灰度仍由独立运行证据控制。
- Subscription create 只允许选择 owner active Definition 和 Core 返回的 readable/scope 交集；principal、tenant、event type 与 resource 均由认证上下文和 conversation key 派生，页面不提供手填入口。
- 创建成功只表示控制记录已持久化。Runtime 与共享环境继续固定 `direct_target`，语义预筛与事件消费晋级需要独立证据。
- Subscription owner list/create/revoke 已映射到默认关闭的 Gateway HTTP 与 Vue 页面；实现必须同时启用服务端和前端开关，Runtime 继续使用 `direct_target`。
- Subscription 管理固定披露 owner、精确 Definition version、conversation scope、确定性 filter 和审计状态；列表中的 `active` 只表示控制记录有效，不能表达 Runtime 已消费事件。
- Runtime 继续显示 `direct_target` Shadow 边界。公开 Definition 目录交付前关闭创建入口，禁止要求用户手填内部 Definition ID；撤销必须提交精确原因并保留审计记录。
- `definition_stale`、依赖不可用和撤销中的状态均 fail closed；界面不启用共享事件触发，也不声称关键词过滤已经具备语义等价召回能力。
- Elicitation Form 固定披露 Server、Tool、Invocation 与不可信来源，提交前以当前 Workflow Query 的 schema 和 `request_id` 再校验；缓存 Form 不能在依赖不可用时继续提交。
- 普通 Elicitation 只渲染 `text|select|multiselect|boolean`；密码、Token、支付、Cookie、文件和 URL 授权进入独立安全设计，不复用当前 Form。
- `submitting` 不提前显示恢复成功；只有同一 Temporal Signal 被接受并进入 `running` 后才移除旧表单。取消或过期均进入可审计终态。
- Repair 审批固定采用双人控制：提案人不能审批、两位审批人必须互异、任一拒绝立即终止，证据最长一小时后过期。
- Repair `approved` 只表达不可变审计结论；在独立、可回滚且再次授权的 executor 落地前，界面不提供执行入口，也不声称 projection 已修复。
- Repair evidence unavailable 时禁止创建或批准提案，先恢复 Worker/Temporal 并重新采集 canonical evidence。
- 本地缓存采用高低水位淘汰；容量压缩属于本地保留策略，不改变服务端设备 Cursor 或已提交的安全 `sync_seq`。
- 同步恢复固定采用“读取安全游标 → 写入本地消息 → 提交设备游标”的可见顺序，避免界面暗示尚未持久化的消息已经安全同步。
- 同步故障局部降级；本地消息继续可读，错误状态提供重试入口，显式退出时清理当前账号本地数据。
- Search v1 仅展示 principal 有权访问的会话范围，并持续显示权限提示。

# 2026-09-01 Brand asset trace rollout

- Agent Memory 与 Event Subscription 控制台的窄侧栏改用从 `LOGO_V3.png` 经 VTracer 生成的 `dipole-v3-agent-mark-traced.svg`；完整 IM/Agent 锁定标仍按页面语义分别使用。
- 页面测试与生产构建通过，远程候选已验证 SVG 通过 Gateway 静态资源路由返回。

# 2026-09-01 Chat Desktop V3 surface pass

- Chat Desktop 首轮实现已将导航 rail、会话侧栏、空状态、消息气泡、输入区和详情区映射到共享 Pencil tokens；交互与数据请求保持原有契约。
- 新增受控 Chromium Chat screenshot baseline，页面级视觉测试通过，远程候选已验证新 CSS 资源返回 `200`。
- Search 故障采用局部降级，不遮挡或禁用聊天主链路。
- 结果同时展示会话身份和 `message_seq`，为后续精确定位消息保留稳定交互语义。

# 2026-09-01 Pencil V3 path asset import

- 在活动 Pencil canvas 中将 `LOGO_V3.png` 的 VTracer 原始四路径几何导入 Group、Contact、File 和 Device rail，保留源 viewBox、品牌颜色与比例；导入过程未使用人工重绘或外部 image fill。
- Group、Contact、File、Device 节点级截图通过，确认标志无 checkerboard、裁切、溢出或折叠；已将活动 Pencil canvas 的完整设计基线安全同步到当前分支 canonical `.pen`，保留完整页面 frame 与复用组件。

### 验证

- Pencil canonical 结构门禁通过 `107 frames / 3329 nodes / 36 variables / 49 reusable`；前端页面继续通过 `scripts/check-brand-assets.mjs` 校验页面引用与路径资产完整性。
