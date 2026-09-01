# Dipole UI Design

`dipole-ui.pen` 是 Dipole 前端的 canonical 可编辑设计文件。产品交互、响应式状态或视觉 token 变化时，应增量修改同一文件，并同步更新 `DESIGN-CHANGELOG.md`。

`export-manifest.json` 是批准评审导出的清单。新增或替换评审图时，先更新清单和设计更新日志，再运行 `npm run test:design`；清单路径相对于 `design/`，目录条目至少需要包含一个 PNG 文件。

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
| Group Directory desktop/mobile/state matrix | `exports/group-v1/` |
| Settings desktop/mobile/state matrix | `exports/settings-v1/` |
| Device Security desktop/mobile/state matrix | `exports/device-security-*-review.png` |
| Agent Task Create desktop/mobile/state matrix | `exports/agent-task-create-v1/` |

## 当前 Frame

### Brand Signal v2

`brand-signal-v2-brief.md` 记录新版标识的评审约束。Pencil CLI 在本轮增量写入超过安全超时前已导出一张方向性评审图，但没有完成 canonical `.pen` 保存，因此该文件尚未形成一个已批准的 Canvas Frame。仓库 SVG 使用同一套 Signal Link 方向交付；后续 Pencil 小批次只补充品牌评审区和导出，不能重建现有页面。

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

`shadow` 协议对照和 Prometheus 晋级门禁不增加用户可见状态，继续复用上述状态；灰度操作与回切步骤维护在 [`WEB-SYNC-ROLLOUT.md`](../docs/operations/WEB-SYNC-ROLLOUT.md)，只有交互语义变化时才新增 Pencil frame。

显式退出、HTTP 401、WS kick 和账号切换统一复用现有登录跳转与 Sync 状态，不新增视觉分支；终止过程先撤销会话，再在后台完成账号级本地数据清理。

### Contact v1

- `Contact/Desktop/Manage`：在深色导航轨与暖白工作区中展示可信联系人目录、关系筛选、备注、拉黑/删除入口和待审申请。
- `Contact/Mobile/Manage`：在 390px 宽度内保留联系人、申请和拉黑筛选，申请处理与关系操作均维持可见安全边界。
- `Contact/State Matrix`：覆盖 loading、empty、request pending 和 safety blocked；状态只表达既有认证联系人 API 的加载、申请审核与拉黑关系语义。
- `Component/Contact Row` 与 `Component/Contact Request`：供后续 Contact 页面、会话侧栏和审批入口复用。

批准的 2x 预览位于 `exports/contact-v1/`。当前切片仅建立 Pencil 视觉与状态基线，尚未新增 Contact Vue 路由或更改 Gateway API。后续实现必须从认证会话派生 principal，接受、忽略、拉黑、删除或修改备注后重新读取权威联系人状态；页面不得暗示自动审批或跨用户关系操作。

### Group Directory v1

- `Group/Desktop/Directory`：深色导航轨与暖白工作区中的认证群目录，只展示服务端权威投影。
- `Group/Mobile/Directory`：390px 单列布局，保留群状态、成员摘要和只读边界。
- `Group/State Matrix`：覆盖 loading、empty、unavailable、dismissed 和 hot group；热群明确使用 `notify + pull`。
- `Component/Group Row`、`Component/Group Status`、`Component/Group Member Summary`：供目录、会话侧栏和群摘要复用。

批准的 2x 预览位于 `exports/group-v1/`。当前切片只允许认证读取当前用户会话中的群投影；成员邀请、群资料修改、移除成员、解散和退出群继续使用既有聊天内管理入口，不在目录中新增写入能力。读取失败必须清空旧投影，避免旧群状态被误认为权威状态。

### Device Security v1

- `Device/Desktop/Sessions`：认证设备会话页，展示当前会话、其他会话与登出其他设备确认。
- `Device/Mobile/Sessions`：390px 堆叠会话卡片和清晰的批准确认区，避免窄屏横向表格。
- `Device/State Matrix`：覆盖 loading、无其他会话、读取失败重试、单设备确认、全部其他设备确认、成功反馈和动作失败。
- `Device Session Row`、`Device Trust Status`、`Session Sign-out Confirmation`：页面与状态矩阵共用的组件。

批准的 2x 预览位于 `exports/device-security-*-review.png`。设计只展示设备标签、粗粒度浏览器或设备说明、相对活动时间和当前/信任状态；IP、节点、连接 ID、用户 ID、Token、精确位置与原始 User-Agent 不进入 UI。后续 Vue 页面从认证会话调用既有设备列表与登出 API，单设备和全部其他设备动作均需要明确确认并以权威响应收敛。

### Settings v1

- `Settings/Desktop/Account`：深色导航轨和暖白账户工作区，分为个人资料、同步状态与会话退出边界。
- `Settings/Mobile/Account`：390px 单列布局，保留资料、低敏设备安全入口、本机同步状态和退出操作。
- `Settings/State Matrix`：覆盖 loading、保存成功、服务不可用和同步异常；失败时保留本地草稿并提供重试。
- `Component/Settings Profile`、`Component/Settings Sync Status`、`Component/Settings Logout Boundary`：资料、低敏同步与危险会话操作的共享组件。

批准的 2x 预览位于 `exports/settings-v1/`。页面仅复用认证 profile API、本机 safe cursor、设备安全入口和退出会话；IP、节点、连接 ID、消息正文、对象存储信息和设备原始标识不进入设置页。

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

### Agent Event Subscription v1

- `Agent Subscription/Desktop/Manage`：展示 owner 的订阅历史、精确 Definition version、conversation scope、确定性过滤器、当前状态和撤销/审计入口。
- `Agent Subscription/State Matrix`：覆盖 `loading`、`empty`、`unavailable`、`definition_stale`、`revoking` 和 `revoked` 六态。
- `Agent Subscription/Mobile/Revoke`：以单列订阅摘要和底部确认层展示精确撤销原因与审计边界。
- `Agent Subscription/Desktop/Create`：从 owner active Definition 与 Core 计算的 `principal readable ∩ Definition scope` 中选择精确绑定，并配置确定性过滤器。
- `Agent Subscription/Create State Matrix`：覆盖 catalog/conversation loading、empty scope、Definition stale、scope denied、submitting 和 unavailable。
- `Agent Subscription/Mobile/Create`：在 390x844 单列布局中保留精确 authority 摘要、过滤器校验与 Runtime 边界。
- `Component/Subscription Status`、`Component/Subscription Filter`、`Component/Subscription Row`、`Component/Subscription Create Option`、`Component/Subscription Authority Summary`：供后续 Agent Definition、Task timeline 和 Trigger 管理页面复用。

批准的管理预览位于 `exports/agent-subscription-v1/`，创建预览位于 `exports/agent-subscription-create-v1/`。当前稿固定展示 `OWNER CONTROL / DIRECT_TARGET` 边界：订阅控制状态可以持久化，Runtime 仍保持默认关闭，页面不得暗示已经启动共享事件触发或语义模型预筛。创建订阅必须选择经过 Gateway 鉴权的 active Definition version 与 Core 返回的可读 conversation scope；用户无需也不能手填内部 Definition ID、event type、resource 或 principal。撤销要求精确原因并保留审计信息，撤销动作不改变模型 Runtime 生命周期。

公开 Gateway owner list/create/revoke HTTP adapter、Vue 管理页、owner-scoped active Definition 目录和 authenticated conversation chooser 已按本设计默认关闭接入，分别由 `gateway.agent_subscription_enabled=false|true` 和 `VITE_AGENT_SUBSCRIPTIONS_ENABLED=false|true` 控制。服务端从认证会话派生 principal、固定 tenant，并在创建前后由 Core 复核 Definition、可读会话与 scope；页面查询失败时清空旧候选，创建和撤销均以权威响应收敛。本设计和管理页面不能用于声明 `subscription` Runtime 模式已经可用。

### Agent Task Timeline v1

- `Agent Timeline/Desktop/Events`：只读任务事件历史，展示 Task、revision、event sequence、kind、status、time、Capability 与低敏 provenance label。
- `Agent Timeline/Mobile/Events`：单列窄屏布局，事件按序分组且不依赖横向滚动。
- `Agent Timeline/State Matrix`：覆盖 loading、empty、unavailable/retry 与 older-events pagination；不可用状态明确说明历史未加载。
- `Component/Agent Timeline Event`、`Component/Agent Timeline Revision Badge`、`Component/Agent Timeline Provenance Label` 与 `Component/Agent Timeline Unavailable State`：供后续 Timeline 与审计页面复用。

批准的 2x 预览位于 `exports/agent-timeline-overview/`，全画布记录位于 `exports/agent-timeline-v1/overview.png`。Timeline 只展示持久化低敏元数据，不提供编辑、删除或任务执行控制，也不回放外部 evidence 正文。

Vue 实现位于 `frontend/src/components/AgentTaskTimeline.vue`，路由为 `/agent/tasks/:taskId/timeline`，由 `VITE_AGENT_TASK_TIMELINE_ENABLED=true` 显式启用。设计稿不表示 active Agent authority、MCP continuation 或写 Capability 已开放。

当前 Vue 只读页面的 Chromium visual baseline 位于 `frontend/e2e/agent-task-timeline.visual.spec.ts`；它使用受控低敏 fixture 固定 revision、Capability、等待审批、分页入口与 event kind 展示边界，不能替代全页面或跨浏览器视觉验收。

### Agent Task Create v1

- `Agent Task Create/Desktop`：认证后的聚焦任务创建器，展示本地 request identity、任务目标、只读会话访问边界与未校验时禁用的提交动作。
- `Agent Task Create/Mobile`：390px 单列版本，保留任务范围与提交状态，避免横向溢出。
- `Agent Task Create/State Matrix`：覆盖 idle、validation error、submitting、accepted/redirecting 与 unavailable；只有严格 accepted 回包才允许跳转 Timeline。
- `Component/Agent Task Goal Field`、`Component/Agent Task Request Badge` 与 `Component/Agent Task Submit State`：创建页及后续受控入口复用的低敏组件。
- `frontend/e2e/agent-task-create.visual.spec.ts`：以 Chromium 认证 fixture 固定初始空表单的 canonical 截图；Playwright 启动进程才显式开启创建页开关，常规构建继续关闭入口。
- 聊天主界面仅在创建页和 Timeline 双开关同时启用时显示创建入口；入口只导航到已批准的创建页，不传递身份、配置或任务参数。

批准的 2x 预览位于 `exports/agent-task-create-v1/`。Vue 实现位于 `frontend/src/components/AgentTaskCreate.vue`，路由为 `/agent/tasks/new`；只有 `VITE_AGENT_TASK_CREATE_ENABLED=true` 且 Timeline 开关同时开启时才可访问。页面只提交本地 `client_request_id` 与目标文本，principal、tenant、Agent、Tool、Memory 和 Runtime 控制均由服务端恢复或固定；设计与页面均不表达 active authority、Compose、Kafka 或 Temporal 已启用。

### Agent Definition Catalog v1

- `Agent Definition/Desktop/Catalog`：owner-scoped 的只读 Definition 目录，展示精确版本、会话 scope 与 Runtime 关闭边界。
- `Agent Definition/Mobile/Catalog`：窄屏单列目录，保留版本、scope、`CATALOG ONLY` 与 `RUNTIME DISABLED` 信息。
- `Agent Definition/State Matrix`：覆盖 loading、empty、unavailable/retry 与分页后的精确版本目录。
- `Component/Agent Definition Row`、`Component/Agent Definition Scope Chip` 与 `Component/Agent Definition Status`：供 Definition、Subscription 和治理页复用。

批准的 2x 预览位于 `exports/agent-definition-overview/`，全画布记录位于 `exports/agent-definition-v1/overview.png`。目录没有 create、edit、activate、delete、model 或 Tool 控制；页面也不披露 owner、tenant、内部 provenance 或参数。订阅创建继续以 Core 权威 scope 重新校验。

Vue 实现位于 `frontend/src/components/AgentDefinitionCatalog.vue`，路由为 `/agent/definitions`，由 `VITE_AGENT_DEFINITIONS_ENABLED=true` 显式启用。Chromium visual baseline 位于 `frontend/e2e/agent-definitions.visual.spec.ts`，受控 fixture 只固定低敏 Definition metadata；它不能替代 active Runtime、写 Capability、跨浏览器或真实共享环境验收。

### Agent Artifact Metadata v1

- `Agent Artifact/Desktop/Metadata`：owner-scoped 的只读 Artifact metadata 页面，展示类型、版本、标题、媒体类型、大小、Task/Run、创建时间和内容寻址摘要。
- `Agent Artifact/Mobile/Metadata`：390x844 单列布局，保留 Timeline 返回入口、metadata 状态和正文披露边界。
- `Agent Artifact/State Matrix`：覆盖 loading、ready、unavailable/retry 和 disclosure closed；读取失败必须清空旧 metadata。
- `Component/Agent Artifact Disclosure` 与 `Component/Agent Artifact Integrity`：固定“正文、对象键与下载保持关闭”和 SHA-256 content address 的只读语义。

批准的 2x 预览位于 `exports/agent-artifact-v1/`。页面不会显示正文、对象键、metadata JSON、公开 URL、下载或写入控制；未来正文/下载需要独立的对象访问授权、披露策略和设计切片。

Vue 实现位于 `frontend/src/components/AgentArtifactMetadata.vue`，路由为 `/agent/artifacts/:artifactId`，由 `VITE_AGENT_ARTIFACTS_ENABLED=true` 显式启用。Timeline 只在 `kind=artifact` 和 64 位内容寻址 ID 同时成立时提供跳转；认证读取流程已在 Chromium、Firefox、WebKit 复核，Chromium visual baseline 只固定受控 metadata fixture，不能替代共享环境或下载授权验收。

### Agent Artifact Digest Reader v2

`conversation_digest` 且媒体类型为 `text/markdown` 的 Artifact 可通过认证 owner 的受限正文接口进入阅读区。metadata、完整性摘要与正文分别加载；正文不可用时保留已确认的 metadata 并提供重试。对象键、metadata JSON、公开 URL、通用下载、写控制和其他 Artifact 类型的正文仍不进入浏览器。Pencil 画布增量和导出以 `agent-artifact-digest-v2-brief.md` 为输入，完成后再登记至 export manifest；实现继续使用 `VITE_AGENT_ARTIFACTS_ENABLED=true`。

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

自动化增量任务优先使用仓库内的安全包装器。它将 pen 输出写入临时文件，默认 120 秒超时，校验 `.pen` 结构和导出文件后再原子替换目标；失败时保留原设计：

```bash
node scripts/pencil-safe-edit.mjs \
  --input design/dipole-ui.pen --output design/dipole-ui.pen \
  --export design/exports/review.png --timeout-ms 120000 -- \
  --prompt-file /path/to/brief.md --agent claude
```

进入 interactive session 后遵循以下顺序：

1. 调用 `read_skill()` 阅读当前 Pencil skill；CLI 升级后重新确认 schema 和执行约束。
2. 调用 `get_app_state()` 检查当前文件和顶层 frame。
3. 通过 `execute` 增量更新 token、可复用组件和页面，新增节点必须使用可读名称。
4. 编辑期间设置根 frame `placeholder: true`，完成后恢复为 `false`。
5. 使用 `Get` 检查 clipping、未命名节点和残留 placeholder，并用 `TakeScreenshot` 做视觉检查。
6. 调用 `Export` 更新已批准预览，随后执行 `save()`。

禁止为单个功能重建整份 `.pen`，也不要复制出多个互相漂移的 canonical 文件。页面实现完成后，应补充键盘操作、焦点可见性、语义标签、缩放和窄屏测试；设计预览不能替代可访问性与组件测试。
