# 设计更新日志

本文档记录 Dipole canonical Pencil 设计的用户可见变化，格式遵循 Keep a Changelog。日常修改统一写入 `Unreleased`。

## [Unreleased]

### 新增

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

- Subscription owner list/revoke 已映射到默认关闭的 Gateway HTTP 与 Vue 页面；实现必须同时启用服务端和前端开关，创建入口继续等待 authenticated Definition 目录，Runtime 继续使用 `direct_target`。
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
