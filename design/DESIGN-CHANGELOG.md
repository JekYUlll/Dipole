# 设计更新日志

本文档记录 Dipole canonical Pencil 设计的用户可见变化，格式遵循 Keep a Changelog。日常修改统一写入 `Unreleased`。

## [Unreleased]

### 新增

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

- 本地缓存采用高低水位淘汰；容量压缩属于本地保留策略，不改变服务端设备 Cursor 或已提交的安全 `sync_seq`。
- 同步恢复固定采用“读取安全游标 → 写入本地消息 → 提交设备游标”的可见顺序，避免界面暗示尚未持久化的消息已经安全同步。
- 同步故障局部降级；本地消息继续可读，错误状态提供重试入口，显式退出时清理当前账号本地数据。
- Search v1 仅展示 principal 有权访问的会话范围，并持续显示权限提示。
- Search 故障采用局部降级，不遮挡或禁用聊天主链路。
- 结果同时展示会话身份和 `message_seq`，为后续精确定位消息保留稳定交互语义。
