# 设计更新日志

本文档记录 Dipole canonical Pencil 设计的用户可见变化，格式遵循 Keep a Changelog。日常修改统一写入 `Unreleased`。

## [Unreleased]

### 新增

- 建立 `design/dipole-ui.pen`，定义浅色画布、深色导航数据面、绿色强调色、Manrope/Noto Sans SC 字体和基础间距圆角 token。
- 增加 Search Field、Search Result、Search Skeleton 与 Search State 四个可复用组件。
- 增加消息搜索 desktop/mobile 的 Results、Loading、Empty、Error 四态设计。
- 增加 `exports/search-v1/` 批准预览，作为 Vue 实现和后续视觉回归的首个基线。

### 设计决策

- Search v1 仅展示 principal 有权访问的会话范围，并持续显示权限提示。
- Search 故障采用局部降级，不遮挡或禁用聊天主链路。
- 结果同时展示会话身份和 `message_seq`，为后续精确定位消息保留稳定交互语义。
