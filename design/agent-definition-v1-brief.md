# Agent Definition Catalog v1

在现有 `design/dipole-ui.pen` 中增量加入只读 Agent Definition Catalog，不得改动或删除既有 frame、token、组件和 export。

新增命名明确的 Frame：

1. `Agent Definition/Desktop/Catalog`
2. `Agent Definition/Mobile/Catalog`
3. `Agent Definition/State Matrix`

新增可复用 Component：

1. `Component/Agent Definition Row`
2. `Component/Agent Definition Scope Chip`
3. `Component/Agent Definition Status`

视觉语言沿用既有浅色画布、深色 Agent Control rail、绿色低敏只读强调、Manrope/Noto Sans SC/Geist Mono 和 `--dp-*` token。Desktop 使用三栏密度：左侧控制导航，中间 owner-scoped Definition list，右侧 authority panel。Mobile 使用单列，不依赖横向滚动。

页面应包含：

- 标题“Agent 定义”和说明“只展示当前认证 principal 可用的精确版本”。
- 只读边界：`CATALOG ONLY · RUNTIME DISABLED`，明确列表、版本和 scope 不会启用 Runtime、订阅或 Tool。
- 行项展示 Agent identity、Definition ID、精确 version、scope chips、valid from、optional expires at 和 `ACTIVE / CATALOG` 状态。
- state matrix 覆盖 loading、empty、unavailable/retry、expired 与 next-page pagination。
- 不提供新增、编辑、删除、启用、执行或 permission mutation 控件；避免展示 owner ID、tenant、内部 provenance、模型或 Tool 参数。

将这三个画板的 2x PNG 导出至 `design/exports/agent-definition-overview/`，并导出完整画布 `design/exports/agent-definition-v1/overview.png`。完成后保留可读 node name，根 frame 不得留下 placeholder。
