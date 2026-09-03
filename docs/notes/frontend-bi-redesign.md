# 前端信息架构与 BI 化改造设计

日期：2026-09-03。作用范围：`dipole/frontend`。目标：把当前 17 view / 9 处复制两栏、按页面维度切碎的 SPA，改成 3–4 个高密度 BI 工作台。默认生产 `VITE_*` 仍关。

本文档是方案草稿，先过设计再落地。落地按"引入 primitives → 迁工作台 → 删旧 view"分阶段做，每阶段独立可回滚。

---

## 1. 现状与问题

### 1.1 页面清单（来自 [Explore Survey](20c3332e-5dc3-40bc-91bf-9bafc98fe12b)）

17 个 view，命名与深度分散：

| 分组 | View | 路由 | 深度 | Flag |
| --- | --- | --- | --- | --- |
| 入口 | LoginView | `/login` | – | – |
| IM | ChatView | `/` | 0 | – |
| 目录（孤儿，仅 URL 可达） | ContactDirectory / GroupDirectory / FileDirectory | `/contacts` `/groups` `/files` | – | – |
| 账号 | DeviceSecurityView | `/devices` | 2 | – |
| 账号 | SettingsView | `/settings` | 1 | – |
| Agent 列表 | AgentTaskInboxView | `/agent/tasks` | 1 | `timeline` |
| Agent 列表 | AgentArtifactInboxView | `/agent/artifacts` | 1 | `artifacts` |
| Agent 列表 | AgentSubscriptionsView | `/agent/subscriptions` | 1 | `subscriptions` |
| Agent 列表 | AgentDefinitionsView | `/agent/definitions` | 1 | `definitions` |
| Agent 列表 | AgentMemoriesView | `/agent/memories` | 1 | `memories` |
| Agent 详情/表单 | AgentTaskTimelineView / AgentElicitationView / AgentApprovalView / AgentArtifactView | `/agent/tasks/:id/{timeline,input,approval}` `/agent/artifacts/:id` | 2 | 逐个 flag |
| Agent 建任务 | AgentTaskCreateView | `/agent/tasks/new` | 1 | `taskCreate && timeline` |

### 1.2 具体问题

1. **命名与深度不一致**：Agent 用 `/agent/tasks/:id/timeline` 一路展开到 3 层；联系人/群组/文件是平级顶层却根本没入口。
2. **两栏侧栏复制 9 遍**：`grid-template-columns:256px 1fr` 在 `AgentTaskInbox / AgentMemoryManager / AgentArtifactInbox / AgentSubscriptionManager / AgentDefinitionCatalog / ContactDirectory / GroupDirectory / FileDirectory / DeviceSecurity` 各写一份，rail 只在 5 个 Agent 页共享（本轮抽出的 `AgentControlRail`），IM 侧的 4 个目录页仍各写一遍。
3. **状态视图重复**：`viewState + spinner + EMPTY + unavailable danger card` 在 13 个组件里独立拷贝，Class 名不一致（`state-card` / `.spinner` / `.state-code`）。
4. **色号硬编码**：`AgentMemoryManager.vue:403` 直接写 `"Noto Sans SC"` 等字体名；IM 目录 rail 用 `#b7c7c0` 硬色；其余都走 `--dp-*` token。
5. **详情=新页面**：任务/产物详情、审批、输入都是独立路由，从 Inbox 点进去要再进两屏，不利于 BI 场景下的横向对比。
6. **孤儿路由**：`/contacts /groups /files` 没有任何 `RouterLink` 指向；IM 里的联系人/群组是 `ChatView` 内的 tab，与外部路由重复实现。
7. **代码风格分层不清**：所有页面都写 `<style scoped>`，无 shared layout primitive；Vue 端已经有 design tokens，但被 rail-shell 之类的局部变量再次映射。
8. **测试口味两分**：16 个 mount 测试 + 15 个"读源文件断言字符串"的 design 测试，后者对样式重构不友好。

---

## 2. 目标与非目标

### 2.1 目标（本轮）

- 顶层 workspace 从"17 view / 12 top routes"收敛到 **4 workspace + 1 login**：`Chat / Agent / Directory / Settings`。
- 详情、审批、输入、时间线一律**收进 workspace 内部的右侧 detail panel 或 drawer**，click depth 从 3 降到 1。
- 抽出 8 个 layout / state primitives，禁止各页面再重写两栏、状态卡、状态条。
- 视觉密度按 BI 风格：**行高降到 34px、卡片压扁、表格优先、留白仅在 workspace 之间**。
- 代码风格统一：`<script setup lang="ts">`、`defineProps` 内联 type、Pinia 只在跨 workspace 状态用、颜色/字体一律 token。

### 2.2 非目标（不在本轮）

- **不自造轮子**：Table / Dialog / Dropdown / Toast / Popover / Tabs 等复杂组件走开源库（详见 §5.1），不再手写。
- 不改 API 层（`src/api/`）、不改 store 契约。
- 不动 Login 和 ChatView 主体（Chat 有 4000+ 行 CSS，另外一轮再收敛）。
- 不改后端 Gateway/Core 契约、不改默认 flag、不改默认 Compose。
- 不做 mobile：BI 风格仍面向桌面 ≥ 1280px。原 `@media(max-width:820px){display:none}` 保持。
- 不新增图标风格：图标统一走 [Feather Icons](https://feathericons.com/)（`components/icons/index.ts` 已是这套），unicode 符号 `▣ ⌁ ◉ ☷ ▦ ＋` 全部清掉。

---

## 3. 信息架构（IA）

### 3.1 顶层导航 —— Agent 是 Chat 的伴生抽屉，不是独立 workspace

产品定位：**Chat 是主活动，Agent 是旁观察侧**。参考 Cursor（Composer 主区 + 生成结果侧栏）、Codex Web UI（chat 主区 + task drawer）、WorkBuddy（IM 主区 + 任务面板）。用户不该在打字打到一半时"切走去另一个 tab 处理 Agent"。

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│ DIPOLE   Chat   Directory   Settings                    🔍  🤖 3●  ⚙  user ▾       │  ← 顶栏 48px
│                                                              ↑                        │
│                                                      Agent 图标常驻，红点 = pending    │
├───────────────────────────────────────────────────────────┬───────────────────────────┤
│ CONVERSATIONS               │ Chat main area              │ AGENT DRAWER              │
│                             │                             │ (可展开/折叠，默认折叠)   │
│  ─ Alice                    │  ...消息流...               │  Live | Tasks | Artifacts │
│  ─ Bob                      │                             │  ─────────────────────────│
│  ─ project.roomA            │  [Composer]                 │  当前会话活动：           │
│                             │                             │   • task 854… RUNNING     │
│                             │                             │   • 待我处理 (2)          │
│                             │                             │   • 最近产物 (5)          │
└───────────────────────────────────────────────────────────┴───────────────────────────┘
```

- 顶级 route 只有 **4 个**：`/login /` `/directory` `/settings`。Agent 没有独立 URL 主路径。
- 顶栏右侧一个 `🤖` 图标（Feather `Cpu` 或 `Zap`）+ 红点数量徽标，永远存在；点击 = 展开右侧 Agent Drawer。
- Agent Drawer 用 URL query 表示开合与视图：`/?agent=1&view=live`；关闭 = `router.replace` 去掉 `agent`。
- 目前的 `AgentControlRail` 退化为 Agent Drawer **内部**的 tab 栏。

### 3.2 路由结构（目标）

```
/login                                            未登录
/                                                 Chat 主区，Agent Drawer 折叠
/?agent=1                                         打开 Drawer，默认 view=live
/?agent=1&view=<V>                                V ∈ live | tasks | artifacts | definitions | subscriptions | memories
/?agent=1&view=tasks&task=<id>&panel=<P>          P ∈ timeline | input | approval
/?agent=1&view=artifacts&artifact=<id>            产物详情内嵌在同一个 Drawer 里
/?agent=1&view=tasks&drawer=create                创建任务表单（Drawer 内部形态切换）
/directory                                        Directory workspace（默认 tab=contacts）
/directory?tab=<T>                                T ∈ contacts | groups | files | devices
/settings                                         账号 + 会话 + 集成
```

要点：

- 全部子视图**都在 `/` 一个 URL 底下**用 query 控制，深链可分享。
- Drawer 展开时占右侧 44%（可拖拽 320–560px 范围），关闭时 0px；Chat 主区自适应。
- 产品语义：Drawer 打开时 Chat 主区**仍然可交互**（继续打字、切会话），Agent 视图只是"看着旁边"，不 modal 阻塞。
- 旧路由 `/agent/tasks*` `/agent/artifacts*` `/agent/subscriptions` `/agent/definitions` `/agent/memories` 全部**直接删除**（用户拍板 5：无外链，无 redirect）。
- 联系人/群组：`ChatView` 内的 tab 保持不动（IM 消息发送需要）；Directory workspace 提供只读全景，不做重复的 IM 语义。

### 3.2.1 Agent Drawer 内部视图

Drawer 顶部一条 tab bar，**顺序固定**：

| view | 用途 | 默认打开时看到什么 |
| --- | --- | --- |
| `live` **（默认）** | "跟当前 chat 会话相关" | 当前会话活跃任务时间线 + 待我处理红点 + 最近 5 条产物；无当前任务时 fallback 到 owner 全局 pending |
| `tasks` | 任务运行管理 | DataTable：Task ID / Status / Pending / Updated；点行 → Drawer **内部**右侧再展开 sub-panel 显示 timeline/input/approval |
| `artifacts` | 产物 | DataTable + sub-panel 详情 |
| `definitions` | Agent 定义目录 | DataTable + sub-panel 详情 |
| `subscriptions` | 事件订阅 | DataTable + toolbar `+ Create` Dialog + 行内 revoke ConfirmDialog |
| `memories` | 长期记忆 + 候选审阅 | DataTable + accept/reject 行内操作 |

关键：**`live` 是默认页**，用户点顶栏 🤖 就看到"当前跟我聊的这个上下文里 agent 在干嘛"，而不是甩他一个空的任务列表。这是与 Cursor/Codex 一致的产品直觉。

### 3.3 深度对比

| 场景 | 现在 clicks (from `/`) | 目标 |
| --- | --- | --- |
| 打开待审批任务并审批 | Chat → Agent icon → Inbox → 行 → Approval | 1（在 Agent workspace 里表格点行，右侧 panel 直接展 Approval 表单） |
| 打开产物并查看正文 | Chat → Artifact icon → Inbox → 行 → 详情 | 1 |
| 创建订阅 | Chat → Settings → Subscriptions link → 页 → 按钮 → 抽屉 | 1（Agent workspace `Subscriptions` tab 顶部 toolbar 直接开 modal） |
| 撤销设备 | Chat → Settings → Devices | 1（Directory workspace `Devices` tab） |

### 3.4 空态与 Flag 组合

- Agent workspace 顶部**始终显示 5 个 tab**；对应 flag 关闭时该 tab 呈 `disabled + tooltip "in this deployment"`，点不动。
- workspace 内部若默认 tab 被关，自动 fallback 到列表里第一个启用的 tab；全关时 workspace 层由 `router` 拦截跳 `/`。
- Directory 同理。这样避免"tab 会随 flag 消失导致锚点抖动"。

### 3.5 Kill list（每个旧页面 / 旧状态的下场）

用户视角每一次跳转必须变成同屏动作。下表列每个"独立页面 / 空占位"在新 IA 里的落点。Agent 全部塞进 **Chat 右侧 Drawer**：

| 旧路径 / 旧 UI | 现在的问题 | 新落点（都在 `/` 一个 URL 下的 Agent Drawer 里） |
| --- | --- | --- |
| `/agent/tasks` | 独立列表页 + 自绘 sidebar | Drawer `view=tasks` 的 DataTable |
| `/agent/tasks/new` | 独立表单页，进去后无返回按钮 | Drawer `view=tasks&drawer=create` 内部形态切换，不打开新 Drawer |
| `/agent/tasks/:id/timeline` | 深路径 3 层 | Drawer `view=tasks&task=:id&panel=timeline` sub-panel |
| `/agent/tasks/:id/input` | 独立表单 + 用户自己回 inbox | 同一 sub-panel 的 `Input` tab；pending 时自动激活；提交后自动折叠回列表 |
| `/agent/tasks/:id/approval` | 同上 | 同一 sub-panel 的 `Approval` tab |
| `/agent/artifacts` + `/agent/artifacts/:id` | 两页两级 | Drawer `view=artifacts` + `artifact=:id` sub-panel |
| `/agent/subscriptions` + create 弹层 + revoke 弹层 + **4 种 State Matrix 独占页** | 你点名的就是这个 | Drawer `view=subscriptions`；create / revoke 走 PrimeVue `<Dialog>` / `<ConfirmDialog>`；4 种 state 全变成**行内 Banner + Skeleton row**（§4.6） |
| `/agent/definitions` | 独立页 | Drawer `view=definitions` |
| `/agent/memories` | 独立页 | Drawer `view=memories`，accept / reject 行内按钮直接触发（无 confirm dialog） |
| `.state-card { margin:72px auto }` 在 8 个组件里独占主区中央 | 空占空间 | 一律删除，见 §4.6 |
| `dialog-backdrop + sheet-handle`（从底部弹起的 sheet） | 移动端手感、不能横向对比 | 桌面用 `<Drawer position="right" :modal="false">`；确认删除类走 `<ConfirmDialog>` |
| unicode 图标 `▣ ⌁ ◉ ☷ ▦ ＋` | 不成套 | Feather Icons（§5.3） |
| Chat 内的联系人/群组 tab | 和 `/contacts /groups` 重复实现 | Chat 保留 IM 语义；只读全景搬到 `/directory?tab=contacts` |
| `AgentControlRail.vue`（Agent 页面左侧 rail） | 与 Chat 顶栏冲突 | 删除；rail 里那 5 个入口变成 Drawer 顶部的 tab bar |

**结果**：Agent 由 10 route → **0 个独立 route**（全部收进 `/?agent=1&view=...`）；Directory 由 4 个孤儿 route → **1 个 `/directory`**；总 route 数 12 → **4**（`/login /` `/directory` `/settings`）。

### 3.6 关闭 / 返回 语义（所有子面板必须遵守）

用户当前从 create/timeline/input/approval/artifact 页返回需要按浏览器后退 —— 我们没提供 UI 层返回。新规则：

- **每个 Drawer / Dialog 顶栏都必须有** `IconX` 关闭按钮（右上角）、`ESC` 关闭、`click backdrop` 关闭三件套。
- Drawer 关闭 = URL 移除 `task=` / `drawer=` / `artifact=` 等 query 参数（`router.replace`，不重载）；不改变 `tab=`。
- Dialog 提交成功后自动关闭并 flash `<Toast>`；不需要用户再点关闭。
- workspace 内部 tab 切换是**平级**动作，不产生历史；用 `router.replace` 而不是 `push`，避免"后退按钮回到上一个 tab"这种反直觉行为。
- 深链场景（分享 URL）打开时如果 query 里带的 `task=xxx` 无权限或不存在，Drawer 弹一个 inline error 而不是跳走。

---

## 4. 布局与视觉规范（BI 风格）

### 4.1 网格与断点

- 目标分辨率 **1440×900** 基准，最小 1280 宽度。
- 采用 12 列 fluid grid：`display:grid; grid-template-columns:repeat(12, minmax(0,1fr)); column-gap:16px;`。
- 高度按 BI 布局：**Toolbar 48px + Content 剩余空间 + StatusBar 28px**。工作台不再有中央外边距，边到边贴。
- 内部两栏（表格 + detail）用 `grid-template-columns: minmax(0,1fr) minmax(320px,420px);`；detail 可折叠。
- 左窄栏 48px；顶栏 48px；不再有 256px 侧栏。

### 4.1.1 形状（圆角规则）

BI 场景**全直角**。圆形只保留两处强语义：Pill / Chip 的椭圆胶囊、Avatar / Dot 的圆形指示物。其他一切矩形。

| 场景 | 圆角 |
| --- | --- |
| 按钮 / 输入 / 下拉 / 文本域 / 表格 / 表头 / 单元格 | **0** |
| Toolbar / StatusBar / 面板背板 / Divider | **0** |
| Card / SectionCard / MetricStrip / Modal / Drawer / DetailPanel / Toast / Tooltip | **0** |
| StatusPill / Chip / Badge | `--dp-radius-pill`（999px） |
| Avatar / dot indicator / brand dot | 50% |

Token 覆盖：`--dp-radius-sm` 从 `8px` 改为 **`0px`**，`--dp-radius-md` 从 `14px` 改为 **`0px`**；新增 `--dp-radius-pill: 999px;`。同步项：`design/dipole-ui.pen` 的 `variables.radius-sm/md/pill`、`design-tokens.test.ts` 的 `expect` 断言、`.pen` 内所有节点的 `cornerRadius`（573 处硬编码圆角一次性归零，Pill 和 Circle 语义除外）。

Grep 兜底：`assert !/border-radius:\s*(?!0|0px|var\(--dp-radius-pill\)|50%)/.test(css)`；命名不叫 Pill / Avatar / Dot / Circle 的 `.vue` 组件出现非零 radius，即视为回退。

### 4.2 密度

| 元素 | 现值（多数页面） | BI 目标 |
| --- | --- | --- |
| 表格行高 | 44–52px | **32px**（compact 34px） |
| 卡片 padding | 20–24px | **12px** |
| header eyebrow + h1 | 10px + 38px | eyebrow 10px + h1 **22px** |
| 段落行距 | 1.7 | 1.45 |
| 主要列表左右留白 | 42px | 16px |
| Section 之间垂直留白 | 26px | 12px |
| 图标尺寸 | 20px | 16px |

### 4.3 颜色

沿用 `src/styles/design-tokens.css` 的 27 个 token，本轮**不新增颜色**，改动如下：

- 把 `AgentMemoryManager.vue:403–406` 里的字体硬编码删掉，改用 `--dp-font-*`。
- IM 目录里的 `#b7c7c0` 换成 `--dp-ink-faint`。
- 新增两个语义色别名（在同一文件里追加，不新画色）：
  - `--dp-bg-workspace: var(--dp-canvas);`
  - `--dp-bg-panel: var(--dp-surface);`
  - `--dp-bg-panel-muted: var(--dp-surface-muted);`

### 4.4 版式

- 显示字号：`--dp-font-display` for h1/h2；`--dp-font-body` for 段落；`--dp-font-data` for 数据/id/时间戳。
- 全站禁止 `<h1>` 大于 22px。BI 场景里"页面 title"不占黄金位。
- 数据 id / hash / mono 值一律走 `.mono` class（新加到 `design-tokens.css` 底部，10px + `--dp-font-data`）。

### 4.5 状态色与语义

沿用已有 accent/agent/warning/danger/success 五组配对，规则：

| 语义 | 用途 | Token |
| --- | --- | --- |
| Neutral | 默认表格行、默认按钮 | ink / line |
| Pending | 待处理、running | agent + agent-soft |
| Blocked | 待用户输入/审批 | warning + warning-soft |
| Failed | 失败终态 | danger + danger-soft |
| Success | 成功终态 | success + success-soft |
| Brand | 主 CTA | accent |

BI 页面**不用大色块**，语义只出现在 pill/dot/leading-column-border。表格背景保持 `--dp-surface`。

### 4.6 加载 / 空 / 失败 / 陈旧 状态的统一规则（禁用整屏卡片）

**核心原则：状态永远不占据 hero 位置**。数据没就绪，也要让 workspace 骨架（toolbar、tab、状态栏、列/表头）**先渲染**，用户至少知道自己身处何地。

| 状态 | 现状（`.state-card { margin:72px auto }`） | 新规则 |
| --- | --- | --- |
| Loading | 主区中央一个转圈 + 两行字 | Toolbar 右侧 `<ProgressSpinner size="12">` inline；DataTable 内 5 行 `<Skeleton>`；不阻塞用户切 tab |
| Empty | 主区中央 "还没有 xxx" | DataTable 内一行 `<tr class="empty-row">` 显示 `暂无 xxx · <PrimaryCTA>`，CTA 是最高频动作（如 `+ 创建订阅`） |
| Unavailable / Failed | 整屏红卡 + Retry | Toolbar 下方一条 `<Banner tone="danger" collapsible>` + Retry；表格保留上次成功数据（灰化并加 stale watermark），不清空 |
| Stale（Definition drift / conflict） | 整屏橘卡 | Toolbar 下方一条 `<Banner tone="warning" collapsible>` + Refresh；表格数据不清空 |
| Success flash（创建/撤销后） | 无 | `<Toast>` 右下角 3s 自动消失，不阻塞 |
| Blocking action confirm | 底部 sheet | PrimeVue `<ConfirmDialog>` 中央 modal，仅"删除/撤销"用，其它 CTA 不确认直接执行 |

`<StatePanel>` primitive 只在**首屏冷启动、workspace 完全没数据**时短暂显示（DataTable 未装载前的空占位），一旦数据结构确定就交给 DataTable 自己处理 skeleton / empty row。

**grep 兜底**：`assert !/state-card|margin:\s*\d+px auto/`；任何 `.state-*` 独占主区的写法都要被 CI 拒绝。

### 4.7 Wireframe —— Chat + Agent Drawer

**场景 A：Drawer 折叠**（默认）—— 全屏 Chat：

```
┌───────────────────────────────────────────────────────────────────────────────────────────────────┐
│ DIPOLE   Chat   Directory   Settings                              🔍   🤖3●   ⚙   user ▾        │  顶栏 48px
├────────────────┬──────────────────────────────────────────────────────────────────────────────────┤
│ CONVERSATIONS  │ Alice · online · e2ee                                                            │
│  ─ Alice       │  ────────────────────────────────────────────────────────────────────────────── │
│  ─ Bob         │                                                                                  │
│  ─ projectA    │   ...消息流...                                                                   │
│  ─ agent-uai   │                                                                                  │
│                │                                                                                  │
│                │  ┌──────────────────────────────────────────────────────────────────────────┐  │
│                │  │ Composer                                                       [Send]     │  │
│                │  └──────────────────────────────────────────────────────────────────────────┘  │
└────────────────┴──────────────────────────────────────────────────────────────────────────────────┘
```

**场景 B：Drawer 展开 · view=live**（点顶栏 🤖 后默认视图）：

```
┌───────────────────────────────────────────────────────────────────────────────────────────────────┐
│ DIPOLE   Chat   Directory   Settings                              🔍   🤖3●   ⚙   user ▾        │
├──────────┬─────────────────────────────────────────────┬───────────────────────────────────────────┤
│ CONV.    │ Alice                                        │ AGENT · Alice 上下文                    ×│  Drawer header
│  ─ Alice │  ────────────────────────────────────────── │ ─────────────────────────────────────────│
│  ─ Bob   │  ...消息流...                                │ Live │Tasks│Artifacts│Def│Sub│Mem        │  内部 tab bar
│  ─ …     │                                              │ ─────────────────────────────────────────│
│          │                                              │ ⚠ Definition v3 失效  [刷新]       ×  │  inline Banner
│          │                                              │ ─────────────────────────────────────────│
│          │                                              │ 当前任务 · task:854035b7… · RUNNING 34s │
│          │  [Composer]                        [Send]    │  • cap.assign     10:02:03              │
│          │                                              │  • cap.execute    10:02:11              │
│          │                                              │  • cap.reply      10:02:34              │
│          │                                              │ ─────────────────────────────────────────│
│          │                                              │ 待我处理 (2)                             │
│          │                                              │  ● INPUT   task:71fa2e91…  [处理 →]     │
│          │                                              │  ● APPROV. task:9820…      [审批 →]     │
│          │                                              │ ─────────────────────────────────────────│
│          │                                              │ 最近产物 (5)                             │
│          │                                              │  • report-2026-09-03.pdf                 │
│          │                                              │  • diff-summary.md                       │
└──────────┴─────────────────────────────────────────────┴───────────────────────────────────────────┘
                                                            ↑ 44% 可拖拽 320–560px
```

**场景 C：Drawer 展开 · view=tasks · 选中一行**（sub-panel 内嵌）：

```
│ AGENT · Tasks                                                                       [+ Create]  ×│
│ ────────────────────────────────────────────────────────────────────────────────────────────────│
│ Live │[Tasks]│Artifacts│Definitions│Subscriptions│Memories                                        │
│ ────────────────────────────────────────────────────────────────────────────────────────────────│
│ ID              Status         Pending  Updated   ⋯    │ task:854035b7… · RUNNING              ×│
│ ────────────────────────────────────────────────────── │ ──────────────────────────────────────│
│ task:854035b7…  RUNNING        —        10:02   ▸     │ [Timeline] Input Approval             │
│ task:71fa2e91…  AWAIT_INPUT    ●        09:58         │ ──────────────────────────────────────│
│ task:9820…      AWAIT_APPROV.  ●        09:12         │ • cap.assign      10:02:03            │
│ task:5b1c…      SUCCESS        —        09:44         │ • cap.execute     10:02:11            │
│ ⋯                                                     │ • cap.reply       10:02:34            │
│                                                       │                                        │
```

要点：
- Drawer 有**两级 tab**：外层 6 个 view（Live/Tasks/…），内层 sub-panel（Timeline/Input/Approval）。全部平级、无历史堆栈。
- 关闭按钮 `×` 永远在 Drawer header 右上角；点击 = `router.replace` 去掉 `agent` 参数；同时响应 `ESC` 和 backdrop 点击（Chat 主区不算 backdrop，因为 modal:false）。
- Live view 是 Codex/Cursor 那种"你现在跟 Alice 聊天，agent 在 Alice 这个会话里干着什么"的视角；无关会话时 fallback 到 owner 全局 pending。
- 所有 loading / empty / error 都在 Drawer 内以 inline Banner + Skeleton 呈现，**永远不占据 Chat 主区**（这解决了"State Matrix 空洞占屏"）。
- 移动端 (<820px) 本轮不做，Drawer 直接隐藏、Chat 全屏；顶栏 🤖 图标改成"点击跳提示：请用桌面版处理 Agent"。

---

## 5. 组件规范

### 5.1 开源组件库选型

**结论：引入 PrimeVue 4，Aura theme + `--p-*-border-radius: 0` 全局覆写。**

评估过的候选：

| 库 | 版本 | License | 适配度 | 关键理由 |
| --- | --- | --- | --- | --- |
| **PrimeVue** | 4.x | MIT | ★★★★★ | DataTable 是 Vue 生态最强；Aura theme 全 CSS var 驱动，`--p-content-border-radius`、`--p-form-field-border-radius` 直接设 0；90+ 组件全套；design-agnostic，不强制自带图标；`unstyled` 模式可完全自绘 |
| Naive UI | 2.x | MIT | ★★★★ | Vue 3 native、TS-first；`n-config-provider` 传 `common.borderRadius:'0'` 就直角；DataTable 也强。次选，只是 DataTable 与主题定制能力略逊 |
| Ant Design Vue | 4.x | MIT | ★★★ | 组件最全、企业级；但默认圆角+间距偏松，改成 BI 密度需要覆写量大；文化上更偏审批型后台 |
| TDesign Vue | 1.x | MIT | ★★★ | Tencent 出品、中文文档好；密度友好但 DataTable 弱于 PrimeVue |
| Shadcn-Vue | 2.x | MIT | ★★ | copy-based，你拥有源码；但等于自己写，本轮想减重就别走这条 |
| Element Plus | 2.x | MIT | ★★ | 老牌但 Vue 2 味重，DataTable 一般 |
| Vuetify / Quasar | 3.x / 2.x | MIT | ★ | Material Design，气质与本项目冲突 |

**PrimeVue 的用法约束**：

- 只引 needed components，用 `unplugin-vue-components/resolvers` 的 `PrimeVueResolver` 自动 tree-shake。
- 主题走 `@primevue/themes/aura` 的 `definePreset`，把 `primitive.borderRadius`、`semantic.formField.borderRadius`、`semantic.content.borderRadius` 全设 `'0'`，`primitive.borderRadius.pill = '9999px'` 保留 Pill。
- 颜色 preset 使用 `--p-primary-color: var(--dp-accent)`、`--p-surface-*: var(--dp-surface)` 桥接到 Dipole tokens，不引 PrimeVue 自己的调色板。
- **不使用** PrimeIcons（`primeicons` 包），组件 `icon` prop 一律传我们的 Vue icon 组件或 slot。

我们只用 PrimeVue 提供的：`DataTable`、`Column`、`Dialog`、`Drawer`、`Select`、`MultiSelect`、`InputText`、`Textarea`、`Checkbox`、`RadioButton`、`Button`、`Tabs`、`Popover`、`Tooltip`、`Toast`、`ConfirmDialog`、`Divider`、`ProgressSpinner`、`Skeleton`。

### 5.2 自建的 shared primitives（缩到 4 个）

外壳类别 PrimeVue 没有涵盖，仍需自建，但只做**布局壳子**，内容全部塞给 PrimeVue 组件：

| 组件 | 责任 | 内部用什么 |
| --- | --- | --- |
| `AppShell.vue` | 顶栏 + 左窄栏 48px + 主区，Toast/ConfirmDialog 挂载点 | 自绘 + PrimeVue `<Toast>` `<ConfirmDialog>` |
| `WorkspaceToolbar.vue` | Workspace 顶部：title、tab、右侧 action | PrimeVue `<Tabs>` + `<Button>` |
| `WorkspacePanel.vue` | 主 + 副两栏，副栏可折叠 | 纯 CSS grid |
| `StatePanel.vue` | Loading / Empty / Unavailable 状态 | PrimeVue `<ProgressSpinner>` `<Message>` |

**表格**：直接用 `<DataTable :value="rows" size="small" stripedRows>` + `<Column>`，不再自造 `DataTable.vue`。  
**详情**：`<Drawer position="right" :visible="open" modal="false">`，不再自造 `DetailPanel.vue`。  
**弹窗**：`<Dialog>`。  
**下拉**：`<Select>`。  
**KV 列表**：直接手写 `<dl>`，够简单；不做组件。  
**MetricStrip / SectionCard**：`<Card>` + 自绘 CSS 就够，不做组件。  
**Pill / Badge**：项目里 `.status-pill` 已经存在，不引 PrimeVue `<Tag>`（后者可选，但样式不够统一）。抽成 `<StatusPill>` 一个纯 CSS 组件保留。

### 5.3 图标：Feather Icons，一套到底

现有 `frontend/src/components/icons/index.ts` 是标准 [Feather Icons](https://feathericons.com/)：`viewBox 0 0 24 24`、`stroke-width:1.75`、`stroke-linecap:round`、`currentColor`、22 个已提取。规则：

- **本轮清除**所有 unicode 图标（`▣ ⌁ ◉ ☷ ▦ ＋` 出现在原 rail 和 memory 页），全部换成对应的 Feather：`Grid`、`Radio`、`Cpu`（or `Layers`）、`Inbox`、`Package`、`Plus`。
- 补齐当前缺的 Feather：`Bell`、`Filter`、`ChevronDown`、`ChevronRight`、`MoreHorizontal`、`X`、`Play`、`Pause`、`Trash2`、`Edit3`、`Copy`、`ExternalLink`、`RefreshCw`、`Shield`、`Database`、`Layers`。凡在 [feathericons.com](https://feathericons.com/) 官网可查即可加，SVG 路径手工从官网拷贝到 `icons/index.ts` 沿用现有 `icon(...)` 工厂。
- 禁止：Ant Icons、PrimeIcons、Material Icons、Element Icons、emoji 图标、Unicode 图形符号。
- 用法：`<template><IconPlus :size="16" /></template>`；PrimeVue 组件需要 icon 时通过 slot 传入，例如 `<Button><template #icon><IconPlus /></template></Button>`。

### 5.4 已有可复用

- `AgentControlRail.vue` → **本轮弃用并删除**，被 `WorkspaceToolbar` 内 PrimeVue `<Tabs>` 完全取代。
- `SearchWorkspace.vue` → ChatView 保留使用，本轮不迁。
- `icons/index.ts` → 保留并扩充。

### 5.3 每个 workspace 的组件构成

```
AppShell
 └─ WorkspaceToolbar (title, tabs, actions)
     └─ WorkspacePanel
        ├─ MetricStrip (可选)
        ├─ DataTable   ← 每个 tab 的主视图
        └─ DetailPanel ← 行选中后展开
```

**Agent workspace**：`tabs = tasks|artifacts|definitions|subscriptions|memories`，右侧 `DetailPanel` 内嵌 `tabs = timeline|input|approval`（后两者按 pending 类型出现）。
**Directory workspace**：`tabs = contacts|groups|files|devices`，右侧 `DetailPanel` 内嵌行详情。
**Chat workspace**：本轮只做外壳统一（AppShell 顶栏 + tokens），内部不动。
**Settings workspace**：单栏，`SectionCard` 堆叠：账号、Agent 集成、同步状态、退出。

---

## 6. 代码规范

### 6.1 目录与命名

```
src/
  api/                # 已有：每个域一个文件，导出 typed client
  components/
    layout/           # AppShell, WorkspaceToolbar, WorkspacePanel, DetailPanel
    data/             # DataTable, MetricStrip, StatusPill, StatePanel, KeyValueList, SectionCard
    agent/            # Agent workspace 内容组件（原 AgentTaskInbox 等收敛到这里）
    directory/        # Directory workspace 内容组件
    chat/             # 后续 ChatView 拆分预留
    icons/
  views/              # 只留 5 个：LoginView, ChatView, AgentWorkspaceView, DirectoryWorkspaceView, SettingsView
  stores/             # auth, chat；本轮不加新 store
  config/             # agentFlags 保留；不再新增 flag 模块
  styles/             # design-tokens.css, primitives.css（可选）
  router/             # index.ts 五条 route
```

命名：

- 组件文件 `PascalCase.vue`；测试 `PascalCase.test.ts` 同目录。
- Workspace 组件后缀 `Workspace`（`AgentWorkspaceView.vue`）。
- Route name 全小写、kebab-case：`agent`, `directory`, `settings`, `chat`, `login`。
- Store 全小写：`auth`, `chat`。
- CSS class 全小写、kebab-case，`.data-table__row--selected` 双短横 BEM，不用蛇形。

### 6.2 TypeScript

- 一律 `<script setup lang="ts">`；SFC 顶层只导出组件。
- Props 用 `defineProps<{ ... }>()` inline type。
- 事件用 `defineEmits<{ (e:'select', row:Row):void }>()`。
- 禁用 `any`；跨模块类型走 `src/api/*.ts` 或 `src/types/*.ts`（新增）。
- 数据校验（`parse*`）保留在 API 层，view 层不再重复类型断言。

### 6.3 样式

- 所有色/间距/字体 100% token；ESLint 之外补一个 grep 测试：`assert !/(#[0-9a-f]{3,8}|"Noto Sans SC"|"Manrope"|"Geist Mono")/.test(component)`。
- **圆角只能取 `0`、`0px`、`--dp-radius-pill`(999px)、`50%` 四值之一**（`--dp-radius-sm/md` 现值就是 0，可与 `0` 互换但推荐直接写 `0`）；grep 测试 `assert !/border-radius:\s*(?!0|0px|var\(--dp-radius-pill\)|50%)/`。
- 全站禁止 `style="..."` 内联样式（Chat 弹窗除外，暂缓）。
- `<style scoped>` 内引用 primitive class 用 `:deep(...)`；shared primitive 自带样式，页面组件只写自己的布局。
- 状态色只允许出现在 `StatusPill` 或者行左侧 4px accent border。
- **图标只用 `components/icons/`**（Feather 一套）；禁止 unicode 图形符号、emoji、其他 icon 库；grep 测试 `assert !/[▣⌁◉☷▦＋]|@fortawesome|primeicons|@element-plus\/icons/`。
- PrimeVue 主题 preset 集中在 `src/config/primevueTheme.ts`；页面组件不许 override `--p-*` 变量。

### 6.4 状态视图

`viewState` 一律使用四态枚举：`'loading' | 'ready' | 'empty' | 'unavailable'`。Agent 特殊状态（`correcting/revoking/conflict/definition_stale`）迁移到 `busyKind` / `errorKind` 单独字段，不参与主状态机。渲染统一走 `<StatePanel :state="viewState" />`，页面组件只在 `ready` 分支里画内容。

### 6.5 测试

- 保留 mount 测试（`@vue/test-utils`），要求覆盖：默认 tab、tab 切换、行点击展开 detail、pending action 迁移到 detail tab、错误回退到 StatePanel。
- **废弃"读源文件断言字符串"的 design 测试**（15 个 `.test.ts`）。这些测试对本轮重构非常不友好，用 primitives 之后应该只测 primitive 本身的合规。改造清单：
  - `LoginView.test.ts` / `ChatView.*.test.ts` / `AgentActionFormsDesign.test.ts` / `SearchWorkspaceDesign.test.ts` / `design-tokens.test.ts`：本轮全部改成 mount 或者 token snapshot；保留一个 `no-hardcoded-color.test.ts` 全局 grep 兜底。
- 新增 `layout-primitives.test.ts`：只测 `AppShell / WorkspaceToolbar / DetailPanel / DataTable / StatePanel / StatusPill` 六个基石组件的可访问性、focus、tab 切换、行选中。

### 6.6 Feature Flag

- `src/config/agentFlags.ts` 保留 runtime `window.__DIPOLE_FLAGS__` 注入路径。
- workspace 级别 flag 保留原语义，但**只用 flag 决定 tab 是否可点**，不再用 flag 生成路由 fallback（route 只保留 workspace 级别 guard）。
- `agentTaskCreatePageEnabled` / `agentTaskInboxEnabled` 这类衍生量迁到组件内部计算，避免继续膨胀。

---

## 7. 迁移计划

按四个阶段做，每阶段独立发布，不阻塞后端。

### Phase 0 — 基础设施（1 PR）

- 引入 PrimeVue 4：`npm i primevue @primevue/themes`；在 `main.ts` 挂载 `PrimeVue` 与 `ToastService/ConfirmationService`；`src/config/primevueTheme.ts` 定义 preset（radius 全 0、颜色桥接 Dipole tokens）。
- 建 `components/layout` 目录，落地 4 个自建 primitives：`AppShell / WorkspaceToolbar / WorkspacePanel / StatePanel`；`StatusPill` 单独放在 `components/data/`。
- design tokens 改动：`--dp-radius-sm/md` → `0px`（**已落地**），新增 `--dp-radius-pill: 999px;` 与 `--dp-bg-workspace/panel/panel-muted` 别名和 `.mono` class；同步 `design/dipole-ui.pen` 的 `radius-sm/md/pill` 变量、573 处节点 `cornerRadius`、`design-tokens.test.ts` 断言（脚本：`scripts/pencil-flatten-corners.mjs`）。
- 加 `no-hardcoded-color.test.ts`、`no-unicode-glyph.test.ts`、`layout-primitives.test.ts`。
- 补 Feather 图标：从 feathericons.com 拷 `Grid / Radio / Cpu / Inbox / Package / Bell / Filter / ChevronDown / ChevronRight / MoreHorizontal / X / RefreshCw / Shield / Database / Layers` 到 `icons/index.ts`。
- 不改任何 view / route。

### Phase 1 — Agent Drawer 挂到 Chat（1 PR，本轮重点）

产出：Agent 从"10 个独立 route"变成"Chat 页顶栏 🤖 图标 + 右侧 Drawer + 6 个 view + inline banner/skeleton"。这一步就能让你亲眼看到 State Matrix 消失、无返回按钮问题消失、页面深度从 3 变 0。

- 顶栏 `AppShell` 挂 `<AgentToggleButton>`（🤖 图标 + 红点徽标，红点数 = owner 全局 pending input+approval 数）。
- 新增 `components/agent/AgentDrawer.vue`：PrimeVue `<Drawer position="right" :modal="false">`；内部 6 个 view 用 PrimeVue `<Tabs>`。
- 新增 `components/agent/views/`：
  - `AgentLiveView.vue`（**默认视图**）：三段式，当前任务时间线 + 待我处理 + 最近产物。数据来自 `agentTaskClient.listOwned()` 过滤当前 `chatScope`。
  - `AgentTasksView.vue`：DataTable + sub-panel（Timeline/Input/Approval）。
  - `AgentArtifactsView.vue`：DataTable + sub-panel。
  - `AgentDefinitionsView.vue`：DataTable + sub-panel。
  - `AgentSubscriptionsView.vue`：DataTable + `<Dialog>` create + `<ConfirmDialog>` revoke，**移除 4 个 `.state-card` 独占**。
  - `AgentMemoriesView.vue`：DataTable + accept/reject 行内按钮。
- Drawer 开合走 URL query（`agent=1&view=...&task=...`），不产生历史（`router.replace`）；`ESC` 关闭；点 `×` 关闭。
- **删除** 10 个旧 Agent route 和对应的 view 文件：`AgentTaskInboxView / AgentTaskCreateView / AgentTaskTimelineView / AgentElicitationView / AgentApprovalView / AgentArtifactInboxView / AgentArtifactView / AgentSubscriptionsView / AgentDefinitionsView / AgentMemoriesView`。
- **删除** `AgentControlRail.vue`。
- **删除** 每个原页面里的 `.state-card / .dialog-backdrop / .sheet-handle` CSS 和对应模板；状态一律走 `<StatePanel>` 或 inline `<Banner>` + `<Skeleton>` row。

### Phase 2 — Directory workspace（1 PR）

- 新增 `views/DirectoryWorkspaceView.vue`，路由 `/directory`。
- Contacts / Groups / Files / Devices 四个原孤儿路由归拢到这里 tab。
- 旧路由 `/contacts /groups /files /devices` **直接删除**（用户拍板 5：无 redirect）。
- 迁移期 ChatView 中的联系人/群组 tab 不动。

### Phase 3 — Chrome + Settings（1 PR）

- ChatView / SettingsView 外壳换成 `AppShell`；Chat 内部布局本轮不动（用户拍板 6：只统一 chrome）。
- SettingsView 用 `SectionCard` 重排。
- 顶栏 workspace 切换器（Chat / Directory / Settings + 🤖 Agent toggle）落地。

### Phase 4 — 收尾（1 PR）

- 删除 15 个"读源文件"设计测试并替换为 primitive 测试。
- CHANGELOG + AGENTS.md session 状态更新。

### Phase P — Pencil 设计同步（贯穿）

- 每个 Phase 完成前更新 `design/dipole-ui.pen`：
  - Phase 0：新增 frame `BI Foundations v1`（token + 按钮 + 输入 + 表格行 + StatusPill + 空/加载/失败态）。
  - Phase 1：新增 frame `Agent Workspace v1`（tasks/artifacts/definitions/subscriptions/memories 五个 tab + Detail panel）。
  - Phase 2：新增 frame `Directory Workspace v1`。
  - Phase 3：新增 frame `App Chrome v1`（顶栏 + 左窄栏 + Settings）。
- 一律走 `scripts/pencil-safe-edit.mjs --timeout-ms 240000`，配对 brief 文件在 `design/frontend-bi-redesign-*-brief.md`。
- 每帧导出 `design/exports/bi-redesign/*.png` 并登记到 `export-manifest.json` 与 `DESIGN-CHANGELOG.md`。

---

## 8. 已拍板决策（2026-09-03）

1. **组件库** → PrimeVue 4（DataTable/Drawer/Dialog/Toast/Tabs），Aura theme 全直角，图标继续用 Feather。
2. **IA 骨架** → Chat 主活动 + Agent Drawer 伴生；顶级 route 4 个（`/login /` `/directory` `/settings`），Agent 无独立 route，全部走 `/?agent=1&view=...`。参考物：Cursor / Codex / WorkBuddy。
3. **详情面板** → 右侧 `<Drawer position="right" :modal="false">`，可拖拽宽度、可折叠。
4. **状态呈现** → `.state-card` 独占中央全部废弃；Loading = Skeleton row + toolbar spinner；Empty = 表格内 empty row + CTA；Failed/Stale = toolbar 下方 Banner + Retry；不再有整屏卡片。
5. **旧路由** → 直接删除，无 redirect。
6. **ChatView 本轮** → 只统一顶栏 + tokens，内部 4000 行 CSS 留下一轮。

## 9. 开工顺序（对齐 §7）

Phase 0 → Phase 1（本轮的核心）→ Phase 2 → Phase 3 → Phase 4。Phase 1 完工后你能亲眼看到：

- 顶栏出现 🤖 图标 + 红点数 = pending 数
- 点击 🤖 展开右侧 Agent Drawer，默认 `Live` view 显示当前会话的 agent 活动
- Drawer 内部 6 tab + 行内 sub-panel，无独立 URL，无返回按钮问题
- Subscriptions 的 4 个 State Matrix 消失，换成 inline Banner + Skeleton row
- 12 route → 4 route，10 个 Agent view 文件被删

---

## 10. 参考

- Explore 调研：[frontend structure survey](20c3332e-5dc3-40bc-91bf-9bafc98fe12b)
- 现有 design tokens：`frontend/src/styles/design-tokens.css`
- Pencil 源：`design/dipole-ui.pen`
- 前一轮 rail 抽出：`frontend/src/components/AgentControlRail.vue`（Phase 1 删除）
- 已知 flag 与 gap：`docs/notes/agent-frontend-experience-gaps.md`
- 产品参考：Cursor Composer 右侧 diff/plan 面板、Codex Web UI 三栏、WorkBuddy IM + 任务面板
