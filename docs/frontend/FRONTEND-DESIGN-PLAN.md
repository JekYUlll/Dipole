# Pencil 前端设计与维护计划

本文档定义 Dipole 前端设计稿、Vue 实现和视觉回归的持续协作流程。Pencil `.pen` 文件是产品视觉基线，运行代码仍以可访问性、响应式行为和自动化测试为发布门禁。

## 1. 当前基线

- Vue 3 + TypeScript + Pinia + Vue Router + Vite。
- 当前路由包含 Login、Chat，以及由 feature flag 保护的 Agent Elicitation、Approval、Task Timeline、Artifact、Subscription、Definition 和 Memory 页面；Search/Sync 作为 Chat 工作区能力接入，复杂交互仍主要集中在 `ChatView.vue`。
- 已建立 canonical `design/dipole-ui.pen`、设计日志和批准预览目录。
- Search 已完成 desktop/mobile 四态、可复用组件和 Vue 工作区；Sync 已完成状态矩阵、desktop/mobile 恢复稿、IndexedDB Sync Engine 与标题栏状态；Contact 已完成联系人管理 desktop/mobile、申请/拉黑状态矩阵、可复用组件与受认证只读 Vue 目录；Group 已完成目录 desktop/mobile、五态矩阵、复用组件和受认证只读 Vue 目录，范围由当前用户会话投影派生，详情读取失败即清空旧状态，热群仅表达 `notify + pull`；File 已完成 owner-scoped 只读目录及逐项授权下载，存储位置、校验值、上传会话和删除控制继续关闭；Device Security 已完成低敏 session projection、desktop/mobile/七态设计与认证 `/devices` 页面；Settings 已完成认证 `/settings`、Pencil desktop/mobile/四态设计、批准导出、Chromium 视觉基线与 Firefox 认证流程，复用签名、同步状态、设备安全和退出边界。WebKit 需要共享宿主系统库维护窗口；跨浏览器视觉回归保留为独立切片。Agent Workflow Repair 已完成 evidence review、六态矩阵和双人审批设计，普通 Elicitation 已完成来源披露、desktop/mobile Form、七态设计及默认关闭的 Vue 实现，Event Subscription 已完成 owner 管理、六态和 mobile 撤销设计；Login、完整 Chat 的完整设计基线仍待补齐。
- 已建立 Vitest + Vue Test Utils + jsdom 基线，以及 Chromium/Firefox/WebKit Playwright IndexedDB 验收；页面流程与视觉回归持续补齐。Agent Task Timeline v1 已通过真实 Pencil CLI 小批次补齐 canonical desktop/mobile/state matrix 和批准导出，并新增 Chromium 受控 fixture 截图基线；完整页面与未覆盖浏览器视觉回归仍待建立。
- Pencil CLI 已认证；2026-08-27 本地版本为 `0.3.5`，设计时使用 CLI 内置 skill 读取最新编辑约束。

## 2. 设计资产

```text
design/
├── dipole-ui.pen              canonical editable design
├── README.md                  pages, flows, CLI workflow
├── DESIGN-CHANGELOG.md        rolling visual decisions
├── exports/                   approved reference previews
└── references/                user-provided visual references
```

`.pen` 文件和设计说明纳入 Git。导出图只保存评审基线，避免每次自动生成大量二进制文件；大文件达到阈值后再启用 Git LFS。

## 3. 完整设计范围

### Foundations

- 色彩、字体、间距、圆角、阴影、图标、网格和动效原则。
- Desktop、Tablet、Mobile 三个断点。
- Light 主题优先，Dark 主题依据产品需要进入后续迭代。

### Components

- Button、Input、Form、Avatar、Badge、Menu、Dialog、Drawer、Toast、Skeleton、Empty、Error 和 Offline 状态。
- Conversation item、Message bubble、Composer、Attachment、Read state、Sync status、Agent status、Approval card 和 Artifact card。
- Default、hover、focus、disabled、loading、error 和 destructive 状态。

### Pages and flows

- 登录、注册和认证错误。
- 会话列表、私聊、群聊、联系人、群管理和文件消息。
- 全局搜索、离线恢复、多端同步和断线重连状态。
- Agent 创建、订阅配置、Task timeline、审批、输入请求、Memory 与 Artifact。
- 管理后台、空状态、权限拒绝和服务降级。

每个核心页面同时提供 desktop 与 mobile 设计，并覆盖 loading、empty、error、offline 和 permission denied。

## 4. Pencil 工作流

1. 从产品需求和现有页面生成或更新 `design/dipole-ui.pen`。
2. 使用 `pen --in` 增量修改同一 canonical 文件，禁止功能迭代重新创建无关联设计。
3. 导出 2x PNG 进行视觉评审，并在 `DESIGN-CHANGELOG.md` 记录页面、组件和 token 变化。
4. 设计通过后，将 token 和组件映射到 Vue；实现 PR 链接对应 frame 或导出图。
5. Playwright 截图与批准的设计基线比较，人工检查响应式、键盘操作和动态内容。

每次首次设计运行前检查 `pen status`、本地版本与 npm 最新版本。CLI 或 skill 升级单独提交，避免与视觉变更混合。

## 5. 里程碑

### F1：设计系统与现有页面复刻

- 已建立 canonical `.pen`、首组 foundations 和 Search 核心组件；通用组件库继续随页面切片补齐。
- 完成 Login、Chat desktop/mobile 及关键状态。
- 抽取 Vue design tokens，保证现有功能不变。

### F2：现代 IM 完整流程

- Search desktop/mobile 四态和 Vue 工作区已完成；Sync 本地恢复与状态反馈已完成首个切片；Contact 已完成 Pencil desktop/mobile/状态矩阵及受认证只读 Vue 目录，继续按独立权限切片实现备注、拉黑、删除与申请处理；Group 已完成 Pencil desktop/mobile/五态矩阵及 `/groups` 认证只读目录，成员与群管理写操作继续关闭；File 已完成 Pencil desktop/mobile/状态矩阵及 `/files` 认证只读目录，上传仍留在会话编辑器，目录只允许逐项重新授权下载；Device Security 已完成 Pencil desktop/mobile/七态矩阵及 `/devices` 认证会话控制；Settings 已完成 `/settings` 认证实现、Pencil desktop/mobile/状态矩阵、批准导出、Chromium 视觉基线与 Firefox 功能验证。WebKit 依赖共享宿主系统库维护窗口；下一项继续补齐完整 Chat 基线。
- 将大型 ChatView 渐进拆成可测试组件。
- 建立 Playwright 路由、交互和视觉回归。
- IndexedDB 的三浏览器持久化、账号清理与页面中断事务契约已进入 Playwright；继续补齐完整页面路由和截图基线。

### F3：Agent Experience

- Agent Workflow Repair proposal/evidence/approval、普通 Elicitation Form 与 Event Subscription owner 管理已完成 desktop、mobile 和状态契约设计；Elicitation Vue 已接入 authenticated Task query/input/cancel、来源披露、普通字段校验和 fail-closed 不可用状态，入口默认关闭。Agent approval Vue 已接入 authenticated Task query/decision、过期与 fail-closed 状态，入口由 `VITE_AGENT_APPROVAL_ENABLED` 默认关闭。Subscription owner list/revoke 已通过默认关闭的 Gateway HTTP 与 Vue 页面交付，并完成三浏览器路由验收。经过鉴权的 Agent Definition 目录已按 canonical desktop/mobile/state matrix 交付，页面严格只读、查询失败清空旧目录；认证读取流程已在 Chromium、Firefox、WebKit 通过，视觉基线继续只固定 Chromium。订阅创建继续独立复核 scope。Runtime 继续使用 `direct_target`。Agent Task Timeline v1 已完成 canonical desktop/mobile frame、State Matrix、批准导出和 Chromium 只读页面视觉基线。Artifact 已完成 canonical desktop/mobile/state matrix、默认关闭的 owner-scoped metadata 页面与 Timeline 条件跳转；失败清空旧 metadata，正文、对象键、metadata JSON 和下载继续关闭。完整 memory、MCP continuation 与其余浏览器视觉回归继续按 AD-036 推进。
- UI 状态与 Temporal AgentTask 状态机保持一一映射。
- 写操作展示风险、影响对象、幂等状态和审计信息。

### F4：持续维护

- [x] 将 canonical `.pen` Foundations token 映射到 Vue 全局 `--dp-*` CSS token，并以 Vitest 契约测试锁定颜色、字体、间距和圆角值；App 壳层与 Search 工作区已接入。
- [x] Agent Task Timeline 以受控低敏 fixture 固定 Chromium 截图，覆盖 revision、Capability、等待审批与“读取更早事件”入口；原始 event kind 不进入用户可见页面。
- API 或状态机变化先更新设计稿和设计日志，再实现页面。
- 每个发布检查 `.pen`、Vue token、Story/fixture 和截图基线是否同步。
- 每个用户可见切片同步复核 [学习与面试入口](../guides/PROJECT-LEARNING-AND-INTERVIEW.md) 中对应 IM 或 Agent 材料的演示步骤、证据链接、状态标签和限制。
- 每季度清理失效 frame 与重复组件，保留已发布版本标签。

## 6. 验收标准

- 所有公开页面和核心状态在 `.pen` 中可定位。
- Desktop 与 mobile 主流程可串联演示。
- Vue 实现通过类型检查、组件测试、E2E、视觉回归和基础可访问性检查。
- 设计 token 无散落重复值，组件状态与设计稿一致。
- 每次用户可见变更同步更新 `CHANGELOG.md` 和 `design/DESIGN-CHANGELOG.md`。
