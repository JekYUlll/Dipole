# Frontend BI Redesign — Foundations v1 (Pencil brief)

Author: session-fad3064c · Date: 2026-09-03. Scope: **只添加一个新 frame** `BI Foundations v1`。不改任何已有 frame。

Canonical file: `design/dipole-ui.pen`. Config doc: `docs/notes/frontend-bi-redesign.md`.

---

## 0. 前置约束

- 本次是**增量**：不重建 `.pen`；不复制现有 frame；不修改现有 `Design Review Checklist`、`00 Foundations` 等 frame 里的节点。
- 只新增一个 top-level frame，名字必须叫 `BI Foundations v1`（大小写与空格一致），放在现有所有 frame **之后**。
- 编辑期间根 frame 设 `placeholder: true`，完成后恢复 `false`。
- 所有颜色、字体、圆角、间距只能使用现有 token（`$rail`、`$accent`、`$agent`、`$ink`、`$ink-soft`、`$ink-faint`、`$line`、`$canvas`、`$surface`、`$surface-muted`、`$danger`、`$danger-soft`、`$warning`、`$warning-soft`、`$success`、`$success-soft`、`$text-inverse`、`$font-display`、`$font-body`、`$font-data`、`$radius-sm`、`$radius-md`、`$space-xs..lg`）。**禁止硬编码颜色和字体**。
- 圆角规则（本次的核心变化，必须遵守）：
  - **所有矩形元素**（按钮、输入、下拉、表格、Card、Panel、Toolbar、Toast、Tooltip、Modal、Drawer、DetailPanel）：`cornerRadius: 0`
  - Pill / Chip / Badge：`cornerRadius: 999`
  - Avatar / Dot / Circle indicator：`cornerRadius: 999`（完整圆）
  - `$radius-sm` 与 `$radius-md` 的当前值已重定义为 0，可以引用它们，但推荐直接写 `0` 以显示意图。
- 图标风格：Feather Icons（stroke 1.75、圆头、`viewBox 0 0 24 24`、`currentColor`）。若需要图标节点，用 `type: "vector"` 或 `type: "icon"` 并显式声明 stroke。**禁止 unicode 图形符号**（`▣ ⌁ ◉ ☷ ▦ ＋`）。
- 该项目定位是内部 BI 工作台，密度偏紧：卡片 padding 12，表格行高 32，`h1` 22px，段落行距 1.45。

---

## 1. 新 frame 的大结构

`BI Foundations v1` 顶层：

```
type: frame
name: "BI Foundations v1"
width: 1440
height: 2200
fill: "$canvas"
layout: vertical
gap: 32
padding: 40
placeholder: false  (完成后)
```

内部按以下顺序纵向排列 8 个 section frame，每个 section 是一个 `type: frame` 子节点，width `fill_container`，`gap: 16`。

### 1.1 Header

- eyebrow：`DIPOLE / BI FOUNDATIONS V1`，`$accent-strong`，`$font-display`，12px，`letterSpacing: 1.8`。
- title：`高密度控制面，直角、Feather 图标、PrimeVue 骨架`，`$ink`，`$font-display`，22px，`fontWeight: 700`。
- description：`覆盖按钮、输入、表格、状态、Pill、Toolbar、Toast、空/加载/失败态的直角基线。所有页面沿此基线组装。`，`$ink-soft`，`$font-body`，14px。

### 1.2 Buttons

标题 `Buttons  ·  rectangular, 3 sizes`。子布局横向 gap 12。

必须给出以下 6 个按钮，全部 `cornerRadius: 0`：

| variant | 高度 | 背景 | 文字 | 边框 |
| --- | --- | --- | --- | --- |
| Primary | 32 | `$accent` | `$text-inverse` | none |
| Primary large | 40 | `$accent` | `$text-inverse` | none |
| Secondary | 32 | `$surface` | `$ink` | 1px `$line` |
| Ghost | 32 | transparent | `$ink` | none |
| Danger | 32 | `$danger` | `$text-inverse` | none |
| Icon-only | 32×32 | `$surface` | `$ink` | 1px `$line` |

每个按钮 padding 水平 14、垂直 6；字体 `$font-body` 13px、`fontWeight: 600`。Icon-only 内部放一个简笔 SVG（例如 `+`，用 `type: "line"` 或 `type: "vector"` 组合两条 12×2 的线交叉，颜色 `$ink`）。

### 1.3 Form fields

标题 `Form  ·  0 radius, 32 height`。三个字段横向排列：

- InputText：宽 240，高 32，`fill: $surface`，边框 1px `$line`，`cornerRadius: 0`，占位文本 `Search agents…`（`$ink-faint`）。
- Select：宽 200，高 32，右侧 12×12 chevron-down（Feather）。
- Textarea：宽 320，高 88，两行占位文本。

### 1.4 DataTable row

标题 `Table  ·  32 row, sticky header, striped`。

一张宽 1200 的示意表：

- Header 行高 36，`fill: $surface-muted`，`$ink-soft`，`$font-data` 11px，`letterSpacing: 0.06`，列名 `TASK · REVISION · STATE · UPDATED · OWNER`。
- 5 行 body 行高 32，交替填充 `$surface` 和 `$surface-muted`。
- 第 3 行 selected：整行 `fill: $accent-soft`，左侧 3px `$accent` 竖条。
- 每行第 1 列 `$font-data` 10px 显示 mock task id `task:abc12…c3f4`（用 `…` 缩略）。
- STATE 列放 StatusPill：分别是 `RUNNING`、`WAITING_INPUT`、`WAITING_APPROVAL`、`SUCCEEDED`、`FAILED`。
- UPDATED 列 `2026-09-03 20:41`，`$font-data`。
- OWNER 列文本，`$ink-soft`。

### 1.5 StatusPill matrix

标题 `Status pills  ·  6 tones`。横向 gap 8：

- neutral：`$surface-muted` bg，`$ink-soft` text，dot `$ink-faint`
- pending：`$agent-soft` bg，`$agent-strong` text，dot `$agent`
- blocked：`$warning-soft` bg，`$warning` text，dot `$warning`
- failed：`$danger-soft` bg，`$danger` text，dot `$danger`
- success：`$success-soft` bg，`$success` text，dot `$success`
- brand：`$accent-soft` bg，`$accent-strong` text，dot `$accent`

每个 pill：高 20、水平 padding 8、`cornerRadius: 999`、字号 10、`$font-data`、`letterSpacing: 0.06`；dot 6×6 `cornerRadius: 999`，label 大写。

### 1.6 Workspace Toolbar 样板

标题 `Workspace Toolbar  ·  48h, tabs + actions`。

- Toolbar frame 高 48，宽 1200，`fill: $surface`，底部 1px `$line`。
- 左侧：workspace title `Agent`，`$font-display` 16、`fontWeight: 700`。
- 中间：5 个 tab（`Tasks / Artifacts / Definitions / Subscriptions / Memories`），横向 gap 24，字号 13，`$ink-soft`；当前 tab `$ink`，下方 2px `$accent` 下划线；tab 前放对应 Feather icon 16×16（分别是 Inbox / Package / Grid / Radio / Cpu，本文件里用 `vector` 简单画出，只求形近即可）。
- 右侧：主按钮 `+ 新建`（Primary 32，含 `Plus` icon）+ icon-only `Filter` + icon-only `RefreshCw`。

### 1.7 State panels

标题 `State panels  ·  loading / empty / unavailable`。横向 gap 16，三张卡片各宽 380 高 180，`fill: $surface`，1px `$line`，`cornerRadius: $radius-sm`。

- Loading：中心 spinner circle 28（Feather `refresh-cw` 或简单圆环），下方 `LOADING`，`$font-data` 10、`letterSpacing: 0.12`，颜色 `$ink-faint`；再下方 `正在拉取权威列表…`。
- Empty：中心 Feather `Inbox` 32、`$ink-faint`；`EMPTY`；`没有符合条件的记录。`
- Unavailable：整卡边框 1px `$danger`；中心 Feather `AlertCircle` 32、`$danger`；`UNAVAILABLE`；`Gateway 未响应，稍后重试。` + 一个 Secondary 按钮 `重试`。

### 1.8 Toast + Dialog 头样板

标题 `Feedback  ·  toast + dialog header`。

- Toast 宽 320 高 56，`fill: $surface`，左侧 3px `$success`；Feather `CheckCircle` 20 `$success` + 文案 `操作成功`（`$font-body` 13）+ 关闭 X 图标。
- Dialog header 高 48 宽 480，`fill: $surface`，底部 1px `$line`；左：`Dialog title`（`$font-display` 15、700）；右：Feather `X` 16 `$ink-soft`。

---

## 2. 校验清单

完成后：

1. `Get` frame `BI Foundations v1`，确认：
   - 只有一个顶层子 frame 被新增，其他 frame 未改。
   - 内部 8 个 section 齐全。
   - 没有节点 `placeholder: true`。
   - 没有硬编码颜色（非 `$`-token）。
   - 没有出现 unicode 符号 `▣ ⌁ ◉ ☷ ▦ ＋`。
2. `TakeScreenshot` 目标 frame，做视觉自检。
3. `Export` PNG 到 `design/exports/bi-redesign/foundations-v1.png`（scale 1）。
4. `save()`。

---

## 3. 输出

- 修改后的 `.pen` 内含新 frame `BI Foundations v1`。
- 一张 PNG 导出：`design/exports/bi-redesign/foundations-v1.png`。
- 不修改任何现有 frame、组件、导出、`export-manifest.json`（登记留给后续人手动做，避免 CLI 覆盖手工整理的清单）。
