# Dipole UI Design

`dipole-ui.pen` 是 Dipole 前端的 canonical 可编辑设计文件。产品交互、响应式状态或视觉 token 变化时，应增量修改同一文件，并同步更新 `DESIGN-CHANGELOG.md`。

## F1 Frame 与评审导出

| Frame | 评审导出 |
| --- | --- |
| Dipole Foundations + Components | `exports/foundations.png` |
| Login Desktop | `exports/login-desktop.png` |
| Login Mobile | `exports/login-mobile.png` |
| Chat Desktop | `exports/chat-desktop.png` |
| Chat Mobile | `exports/chat-mobile.png` |
| Design Review Checklist | `exports/review-checklist.png` |
| 完整画布总览 | `exports/dipole-ui-overview.png` |

## 当前 Frame

### Foundations 与组件

- `00 Foundations`：颜色、字体、圆角和间距基线。
- `Component/Search Field`：搜索输入与快捷键提示。
- `Component/Search Result`：带会话、序号、发送者和时间的消息结果。
- `Component/Search Skeleton`：结果加载占位。
- `Component/Search State`：空态与错误态的共享容器。

### Search v1

- `Search/Desktop/Results`
- `Search/Desktop/Loading`
- `Search/Desktop/Empty`
- `Search/Desktop/Error`
- `Search/Mobile/Results`
- `Search/Mobile/Loading`
- `Search/Mobile/Empty`
- `Search/Mobile/Error`

批准的 1x 预览位于 `exports/search-v1/`。文件名采用 Pencil node ID，frame 名称以本清单和 `.pen` 图层为准。

Vue 实现位于 `frontend/src/components/SearchWorkspace.vue`，状态控制器位于 `frontend/src/composables/useMessageSearch.ts`。入口由 `VITE_SEARCH_ENABLED=true` 控制，并要求 Gateway 同时启用 `search.enabled`。

### Sync v1

- `Sync/State Matrix`：Restoring、Current、Offline、Error、Storage Full 五态。
- `Sync/Desktop/Restoring`
- `Sync/Mobile/Restoring`
- `Component/Sync Status`：页面和标题栏共享的同步状态语义。

批准的 1x 预览位于 `exports/sync-v1/`。Vue Sync Engine 位于 `frontend/src/sync/`，使用 IndexedDB 原子保存消息和安全游标；入口由 `VITE_SYNC_ENGINE_MODE=off|shadow|primary` 控制，默认关闭。

`shadow` 协议对照和 Prometheus 晋级门禁不增加用户可见状态，继续复用上述状态；灰度操作与回切步骤维护在 [`WEB-SYNC-ROLLOUT.md`](../WEB-SYNC-ROLLOUT.md)，只有交互语义变化时才新增 Pencil frame。

显式退出、HTTP 401、WS kick 和账号切换统一复用现有登录跳转与 Sync 状态，不新增视觉分支；终止过程先撤销会话，再在后台完成账号级本地数据清理。

### Agent Workflow Repair v1

- `Agent Repair/Desktop/Proposed`：对照 MySQL Task projection 与 Temporal Workflow 历史，展示 canonical evidence SHA-256、提案依据和双人审批链。
- `Agent Repair/State Matrix`：覆盖 `proposed 0/2`、`proposed 1/2`、`approved`、`rejected`、`expired` 和 `unavailable` 六类持久审计状态。
- `Agent Repair/Mobile/Approval`：在窄屏中以底部审批层保留证据摘要、首位审批记录和安全边界。
- `Component/Repair Status`、`Component/Repair Evidence Diff`、`Component/Repair Approval Step`：供后续 Task timeline 和运维审计页面复用。

批准的 2x 预览位于 `exports/agent-repair-v1/`。当前设计只允许在一小时证据窗口内由两位独立审批人批准或任一审批人拒绝；提案人不能审批。`approved` 仅表示审计门槛满足，界面不得暗示 projection 已修改，也不提供 apply/execute 操作。Worker 或 Temporal 不可用时禁止创建和批准提案，并引导恢复依赖后重新采证。

### Agent Elicitation v1

- `Agent Elicitation/Desktop/Form`：展示外部 MCP Server/Tool/Invocation 来源、当前 Task/revision、deadline、四类受限字段与提交/取消操作。
- `Agent Elicitation/State Matrix`：覆盖 `waiting_input`、`validation_error`、`submitting`、`running`、`cancelled`、`expired` 和 `unavailable` 七态。
- `Agent Elicitation/Mobile/Form`：在窄屏保留来源披露、核心字段、安全提示和固定底部操作区。
- `Component/Elicitation Source`、`Component/Elicitation Field`、`Component/Elicitation Status`：供后续 Task timeline、MCP Form 和本地 Agent 输入请求复用。

批准的 2x 预览位于 `exports/agent-elicitation-v1/`。普通 Form 只允许 `text`、`select`、`multiselect` 和 `boolean`，提交必须绑定当前 Task、principal 与 `request_id`；旧请求、跨用户请求和终态 Task 均拒绝。界面明确将外部内容标记为不可信，并禁止密码、Token、API Key、支付、Cookie、文件上传和 URL 授权。当前设计不代表 MCP continuation、敏感输入或 URL mode 已接入。

Vue 实现位于 `frontend/src/components/AgentElicitationForm.vue`，路由为 `/agent/tasks/:taskId/input`，由 `VITE_AGENT_ELICITATION_ENABLED=true` 显式启用。页面只使用 authenticated Gateway Task query/input/cancel API；查询失败时清空缓存 Form，提交后重新查询权威 Workflow 状态。MCP continuation、敏感输入和 URL mode 仍未接入。

## Sync 交互契约

- 客户端先展示已持久化的本地消息，再从本地安全 `sync_seq` 请求增量页面。
- 每页消息与本地游标在同一 IndexedDB 事务中提交；只有事务成功后才能更新内存并 ACK 服务端设备 Cursor。
- 网络中断时保留本地可读消息；同步失败采用局部状态和显式重试，不遮挡聊天主链路。
- 单用户消息数超过高水位时，在同一事务内淘汰到低水位；优先保留每个会话的最新消息，淘汰不推进安全 `sync_seq`。
- 浏览器拒绝 IndexedDB 写入时展示“本地空间不足”，用户释放浏览器空间后可重试，失败页面不暗示游标已经安全前移。
- 热群补拉复用现有恢复状态；群消息与群 `message_seq` 原子落库后才展示并 ACK，刷新后从本地群位点继续追平。
- 本地数据按认证用户隔离，显式退出账号时清除此账号的消息与游标。
- `message_uuid` 负责稳定身份，`message_seq` 负责会话排序，`sync_seq` 负责设备增量恢复；页面不依赖 MySQL 内部自增 ID。

## Search 交互契约

- 用户从会话侧栏搜索入口或键盘快捷键进入全局消息搜索。
- 查询文本为 1..256 个 Unicode 字符，单次结果上限为 1..100。
- 搜索范围由服务端认证 principal 推导；客户端不能提交用户 ID 或任意会话 scope。
- 结果展示 `conversation_key`、`message_seq`、发送者和发送时间。点击结果的目标交互是打开对应会话并定位到该序号；围绕指定 Seq 拉取上下文的后端能力未接入前，前端不得伪造已定位状态。
- Search Service 不可用时展示有界错误态和重试入口，现有聊天、发送和同步能力保持可用。
- Loading、Empty、Error 与 Results 必须同时覆盖 desktop 和 mobile。

## 增量维护

首次运行前检查 CLI：

```bash
pen status
pen version
```

打开 canonical 文件并将修改保存回原路径：

```bash
pen interactive --in design/dipole-ui.pen --out design/dipole-ui.pen
```

进入 interactive session 后遵循以下顺序：

1. 调用 `read_skill()` 阅读当前 Pencil skill；CLI 升级后重新确认 schema 和执行约束。
2. 调用 `get_app_state()` 检查当前文件和顶层 frame。
3. 通过 `execute` 增量更新 token、可复用组件和页面，新增节点必须使用可读名称。
4. 编辑期间设置根 frame `placeholder: true`，完成后恢复为 `false`。
5. 使用 `Get` 检查 clipping、未命名节点和残留 placeholder，并用 `TakeScreenshot` 做视觉检查。
6. 调用 `Export` 更新已批准预览，随后执行 `save()`。

禁止为单个功能重建整份 `.pen`，也不要复制出多个互相漂移的 canonical 文件。页面实现完成后，应补充键盘操作、焦点可见性、语义标签、缩放和窄屏测试；设计预览不能替代可访问性与组件测试。
