# Dipole UI Design

`dipole-ui.pen` 是 Dipole 前端的 canonical 可编辑设计文件。产品交互、响应式状态或视觉 token 变化时，应增量修改同一文件，并同步更新 `DESIGN-CHANGELOG.md`。

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

## Sync 交互契约

- 客户端先展示已持久化的本地消息，再从本地安全 `sync_seq` 请求增量页面。
- 每页消息与本地游标在同一 IndexedDB 事务中提交；只有事务成功后才能更新内存并 ACK 服务端设备 Cursor。
- 网络中断时保留本地可读消息；同步失败采用局部状态和显式重试，不遮挡聊天主链路。
- 单用户消息数超过高水位时，在同一事务内淘汰到低水位；优先保留每个会话的最新消息，淘汰不推进安全 `sync_seq`。
- 浏览器拒绝 IndexedDB 写入时展示“本地空间不足”，用户释放浏览器空间后可重试，失败页面不暗示游标已经安全前移。
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
