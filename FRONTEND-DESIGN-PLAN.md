# Pencil 前端设计与维护计划

本文档定义 Dipole 前端设计稿、Vue 实现和视觉回归的持续协作流程。Pencil `.pen` 文件是产品视觉基线，运行代码仍以可访问性、响应式行为和自动化测试为发布门禁。

## 1. 当前基线

- Vue 3 + TypeScript + Pinia + Vue Router + Vite。
- 当前路由包含 Login 与 Chat，复杂交互主要集中在 `ChatView.vue`。
- 仓库内尚无 `.pen` 文件、设计 token、组件规范和视觉回归资产。
- 本机 `pen-design` skill 可用，Pencil CLI 已认证；2026-08-26 本地与 npm 最新版本均为 `0.3.5`。

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

- 建立 canonical `.pen`、foundations 和核心组件。
- 完成 Login、Chat desktop/mobile 及关键状态。
- 抽取 Vue design tokens，保证现有功能不变。

### F2：现代 IM 完整流程

- 设计并实现 Contact、Group、File、Search、Sync、Device 和 Settings 流程。
- 将大型 ChatView 渐进拆成可测试组件。
- 建立 Playwright 路由、交互和视觉回归。

### F3：Agent Experience

- 设计 Agent definition、subscription、Task timeline、approval、elicitation、memory 和 artifact。
- UI 状态与 Temporal AgentTask 状态机保持一一映射。
- 写操作展示风险、影响对象、幂等状态和审计信息。

### F4：持续维护

- API 或状态机变化先更新设计稿和设计日志，再实现页面。
- 每个发布检查 `.pen`、Vue token、Story/fixture 和截图基线是否同步。
- 每季度清理失效 frame 与重复组件，保留已发布版本标签。

## 6. 验收标准

- 所有公开页面和核心状态在 `.pen` 中可定位。
- Desktop 与 mobile 主流程可串联演示。
- Vue 实现通过类型检查、组件测试、E2E、视觉回归和基础可访问性检查。
- 设计 token 无散落重复值，组件状态与设计稿一致。
- 每次用户可见变更同步更新 `CHANGELOG.md` 和 `design/DESIGN-CHANGELOG.md`。
