# 前端 Agent 体验缺口

整理日期：2026-09-03。以前端页面为主线，打开 `.env.agent-experience` 后按真实点击路径走一遍，反推后端缺什么。默认生产 `VITE_*` 仍为 false。

本地走查：`cd frontend && npm run dev:agent`。

## 本轮已补的前端可达性

- Settings「Agent 控制」按开关列出定义 / 订阅 / 记忆 / 创建任务。
- Chat 左侧在任意目录页打开时多一个 Agent 入口。
- 定义 / 订阅 / 记忆侧栏的静态标签改成可跳转。
- Definition 目录接上已有 `POST /api/v1/agent/definitions`（`subscription_autoreply` profile）。

## 按页面

| 体验 | 前端页面 | 后端已有 | 走不通的原因 |
| --- | --- | --- | --- |
| 创建任务 → 时间线 | `/agent/tasks/new` → `/timeline` | `POST /tasks`、`GET /tasks/:id`、`GET /timeline` | 创建后只能靠本次返回的 `taskId` 继续。没有「我的任务」列表，关页面就丢。`ListTaskWorkflowProjectionSnapshots` 拒绝带 principal，是 Runtime 服务身份分页，不能给用户当收件箱。 |
| 处理审批 | Timeline 链接 → `/approval` | `GET /tasks/:id` + `POST /approvals/:id` | 页面要 `status=waiting_approval` 且 `pending.kind=approval`。没有 waiting 收件箱，也没有 Chat/WS 推送。必须已经拿着 taskId。 |
| 补充输入 | Timeline 链接 → `/input` | `GET /tasks/:id` + `POST /inputs/:id` | 同上，依赖 `waiting_input` + `pending.kind=input`（含 form/requestId）。Timeline 的 `input_request` 事件没有 requestId，只能再 GET task。 |
| 查看产物 | Timeline → `/artifacts/:id` | `GET /artifacts/:id` + `/content` | 没有「我的产物」列表。Artifact 只能从时间线带 `artifactId` 的事件进去。 |
| 浏览 / 创建 Definition | `/agent/definitions` | `GET` + `POST` profile | 创建已接线。目录仍不暴露模型/Tool。空目录时订阅创建会停在「没有 active Definition」。 |
| 创建 / 撤销订阅 | `/agent/subscriptions` | list / options / create / revoke | API 齐。订阅要先有 Definition + 可读 conversation。Runtime 默认 `direct_target`，列表 active 不会自动开共享事件触发；要另开 subscription overlay。 |
| 查看 / 撤销 / 纠正记忆 | `/agent/memories` | list / revoke / correct | 页面不能写入 Observation。自动写入关着时列表会一直空。 |
| 晋升记忆候选 | 无页面 | 仅 `POST /memory-candidates/:id/promote` | 没有 `GET /memory-candidates`，前端无法列出待审候选，也就无法点晋升。 |
| 取消任务 | 时间线页无按钮 | `POST /tasks/:id/cancel` | 后端有，前端没入口。 |
| 运行状态灯 | 无页面 | `GET /api/v1/agent/status` | 运维/装配探测，不是产品收件箱。 |
| MCP / Repair / OAuth | 无 Vue 入口 | MCP HTTP、Repair Execute RPC、OAuth consume | 有意不进 SPA。Repair 是 mTLS 内部 RPC。 |

## 后端缺的能力（按阻塞排序）

1. **Owner 任务列表 / HITL 收件箱**  
   需要 `GET /api/v1/agent/tasks?status=&after=`，按当前 principal 返回 `taskId/status/revision/pendingKind`。现有 projection snapshot RPC 按 runtime 扫全表且禁止 principal，不能复用。没有这个 API，审批、elicitation、产物、取消都只能靠创建当场记住的 id。

2. **Waiting 任务通知**  
   Chat 没有「有任务在等你」的入口。即便有列表 API，用户仍要自己刷新。需要会话内通知或 WS 事件（task waiting_input / waiting_approval），把人从消息页带进 Timeline。

3. **记忆候选列表**  
   Promote 已挂 Gateway，缺 owner 可读的 candidate 分页。没有列表就做不出「审一条、晋升一条」。

4. **产物列表（次要）**  
   有 metadata/content，缺 `GET /artifacts`。时间线能深链时够用；关页面后找不到历史产物。

## 后端已有、前端仍走不全的（开关 / 运行时，不是缺 RPC）

- 任务创建与 HITL 写：`gateway.agent_control_enabled` + Interactive `/send` overlay。
- 订阅真正触发回复：`agent-subscription-autoreply` / active overlay，默认关。
- 记忆自动写入：`DIPOLE_AGENT_MEMORY_ENABLED`，默认关。
- Timeline 创建后短暂 404：前端已重试一次；投影延迟仍可能让用户看到「时间线暂不可用」。

## 不在本轮做的

不改默认 Compose / 生产 `VITE_*`。不把 MCP Server、Workflow Repair、OAuth 做成 Vue 页。
