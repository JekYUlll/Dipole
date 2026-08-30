# 设计更新日志

本文档记录 Dipole canonical Pencil 设计的用户可见变化，格式遵循 Keep a Changelog。日常修改统一写入 `Unreleased`。

## [Unreleased]

- 增加本地 `.pen` 结构门禁，校验 canonical 设计变量、核心 desktop/mobile frame、可复用组件和 placeholder/未命名节点；该检查不修改设计文件，也不替代 Pencil 视觉评审。
- 增加 `design/export-manifest.json` 评审导出清单；设计门禁现在同时校验批准的单文件和导出目录存在且包含非空 PNG，避免设计稿与评审资产发生静默漂移。

### 变更

- Agent Task 审批页完成 Vue 首个实现切片：沿用既有 Agent Approval 设计基线，展示任务/请求绑定、风险提示、过期和不可用状态；入口由 `VITE_AGENT_APPROVAL_ENABLED` 默认关闭，未扩展 canonical `.pen`。
- 将 `.pen` Foundations 的颜色、字体、间距和圆角变量映射为 `frontend/src/styles/design-tokens.css`，应用壳层与 Search 工作区复用同一组 `--dp-*` token；新增 Vitest 契约测试，后续设计稿更新需同步调整该文件。

### 验证

- Agent Definition Catalog v1 使用 Pencil CLI `0.3.5` 与 `scripts/pencil-safe-edit.mjs` 完成真实增量编辑；canonical 文件原子替换后通过结构门禁，新增 desktop/mobile/state matrix、三个复用组件和 2x 导出。Vue 目录页的 Chromium visual baseline 只覆盖受控低敏 metadata 与只读边界，active Runtime 和写 Capability 继续关闭。
- Agent Task Timeline v1 使用 Pencil CLI `0.3.5`、`scripts/pencil-safe-edit.mjs` 和既有 brief 完成真实增量编辑；canonical 文件原子替换后通过结构门禁，新增 desktop/mobile/state matrix、四个复用组件和 2x 导出。F2/F3 未完成页面、完整截图级视觉回归与未覆盖平台场景继续保持待处理状态。
- Agent Task Timeline Vue 页面新增 Chromium canonical screenshot，使用受控低敏 fixture 固定只读 metadata、Capability、等待审批和分页入口；该验证不涵盖其余浏览器或完整页面基线。

### 新增

- 增加 Agent Artifact metadata desktop/mobile 页面、loading/ready/unavailable/disclosure-closed 状态矩阵和 `exports/agent-artifact-v1/` 批准预览。
- 增加 Artifact Disclosure 与 Integrity 两个可复用组件；设计固定只披露 owner-scoped metadata 和内容寻址摘要，正文、对象键、metadata JSON、下载与写控制继续关闭。
- 增加 Agent Definition Catalog desktop/mobile 目录、loading/empty/unavailable/pagination 状态矩阵和只读 Runtime 边界，并归档 `exports/agent-definition-overview/` 和 `exports/agent-definition-v1/overview.png`。
- 增加 Agent Definition Row、Scope Chip 和 Status 三个可复用组件；目录设计不提供创建、编辑、激活、删除、模型或 Tool 控制，也不披露 owner、tenant、内部 provenance 或参数。
- 增加 Agent Task Timeline desktop/mobile 事件历史和四态矩阵，明确 revision、序号、Capability、状态与低敏 provenance 的只读边界，并归档 `exports/agent-timeline-overview/` 和全画布 `exports/agent-timeline-v1/overview.png`。
- 增加 Agent Timeline Event、Revision Badge、Provenance Label 和 Unavailable State 四个可复用组件；canonical 文件扩展为 61 个顶层 Frame 和 27 个可复用组件。
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
- Search 故障采用局部降级，不遮挡或禁用聊天主链路。
- 结果同时展示会话身份和 `message_seq`，为后续精确定位消息保留稳定交互语义。
