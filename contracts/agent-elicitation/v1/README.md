# Agent Elicitation v1

该契约固定 Agent Task 进入 `waiting_input` 时向客户端公开的受限表单。表单只支持 `text`、`select`、`multiselect` 和 `boolean`，最多 16 个字段，序列化后不得超过 16 KiB；当前版本不支持富文本、文件、URL 授权或敏感凭据。

响应是以字段 ID 为键的 JSON object，序列化后不得超过 16 KiB。服务端必须根据当前 Temporal Workflow Query 返回的表单逐字段校验：拒绝未知字段、缺少的必填字段、重复或越界选项及类型错误。JSON Schema 固定静态表单结构；响应中的动态键约束由各语言实现根据该表单执行。

提交路径必须同时绑定 Task ID 和当前 `request_id`。Gateway 从 JWT 派生 principal，Agent Runtime 向 Core 复核 Task 所有权后才发送 Temporal Signal；旧 request、跨用户请求和终态 Task 均拒绝。Workflow history 保存等待与恢复事件，服务重启后继续使用同一个 pending request。

Task Query 的 pending input 同时公开受限来源元数据：本地请求使用 `kind=agent`；MCP 请求使用 `kind=mcp`，并绑定 `serverId`、`toolName`、`invocationId` 与固定的 `trust=untrusted`。来源字段不承载凭据、URL 或可执行授权；旧 directive 缺少来源时按本地 Agent 处理。
