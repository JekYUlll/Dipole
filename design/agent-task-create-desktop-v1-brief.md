# Agent Task Create Desktop v1

在现有 `design/dipole-ui.pen` 中只增量加入以下两个命名节点，不得改动或删除任何既有 frame、component、token 或 export：

1. `Agent Task Create/Desktop`
2. `Component/Agent Task Goal Field`

页面是经过认证的低敏任务创建器，沿用既有 Agent Timeline 的浅色画布、深色 rail、绿色强调、Manrope/Noto Sans SC/Geist Mono 和现有 spacing token。

Desktop frame 应包含：

- 标题“创建 Agent 任务”，说明“任务提交后进入只读时间线”。
- 一个目标 textarea，说明文本为“描述希望助手完成的工作”，含清晰 label 和字符提示。
- 一个 request badge，示例为 `REQUEST / LOCAL-001`，表达请求身份仅由浏览器本地生成。
- 一条只读访问边界：“只读取当前认证账号已授权的会话；提交不会启用 Runtime、Tool 或外部服务。”
- 一个默认 disabled 的“提交任务”按钮，用于表达内容校验前不能提交。
- 小型状态注释：`IDLE · VALIDATE BEFORE SUBMIT`。

不新增导航、激活、凭据、模型提示、原始事件、Tool、Memory、身份输入、写 Capability 控件或真实数据。根 frame 不得保留 placeholder。
