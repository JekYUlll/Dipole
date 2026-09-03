# 前端 Agent 体验缺口

整理日期：2026-09-03。以前端页面为主线，打开 `.env.agent-experience` 后按真实点击路径走一遍，反推后端缺什么。默认生产 `VITE_*` 仍为 false。

本地走查：`cd frontend && npm run dev:agent`。

## 债务审计（2026-09-03）

Owner SPA 控制面已经齐：定义、订阅、记忆（含候选审核与晋升）、建任务、任务收件箱、时间线、审批、输入、产物列表/详情、Chat 等待提示。台账早期「Vue 未接」指的是当日尚未合入的切片，HEAD 已有页面。

剩余分两类：

1. **默认关**：生产 `VITE_*`、Gateway `agent_*_enabled`、Memory 自动写入、订阅 overlay。页面在，默认路径进不去。
2. **有意不做**：MCP / Repair / OAuth 不进 SPA；Pencil「审批记录」没有路由和 API，已从四页侧栏删除。单次审批仍走 `/approval`。

侧栏开关关闭时显示的灰色标签是 flag fallback，不是死入口。任务/产物收件箱 CSS 相近，是各自页面副本，本轮不抽公共组件。

## 本轮已补的前端可达性

- Settings「Agent 控制」按开关列出定义 / 订阅 / 记忆 / 创建任务。
- Chat 左侧在任意目录页打开时多一个 Agent 入口。
- 定义 / 订阅 / 记忆侧栏的静态标签改成可跳转。
- Definition 目录接上已有 `POST /api/v1/agent/definitions`（`subscription_autoreply` profile）。

## 按页面

| 体验 | 前端页面 | 后端已有 | 走不通的原因 |
| --- | --- | --- | --- |
| 创建任务 → 时间线 | `/agent/tasks/new` → `/agent/tasks` → `/timeline` | `POST /tasks`、`GET /tasks`、`GET /tasks/:id`、`GET /timeline` | Owner 收件箱已接上。关页面后可从 `/agent/tasks` 找回。`ListTaskWorkflowProjectionSnapshots` 仍是 Runtime 服务身份分页，不给用户用。 |
| 处理审批 | Inbox / Timeline → `/approval` | `GET /tasks`、`GET /tasks/:id` + `POST /approvals/:id` | 收件箱能列出 `pendingKind=approval` 的行。页面仍要 `status=waiting_approval` 且 `pending.kind=approval`。Chat 已消费 `agent_task_waiting` locator，按 Task/revision 去重后提示并链到收件箱；详情仍以列表/GET 为准。 |
| 补充输入 | Inbox / Timeline → `/input` | `GET /tasks`、`GET /tasks/:id` + `POST /inputs/:id` | 收件箱能列出 `pendingKind=input` 的行。进入页面后仍要 GET task 拿 form/requestId。 |
| 查看产物 | Inbox `/agent/artifacts` → `/artifacts/:id` | `GET /artifacts`、`GET /artifacts/:id` + `/content` | 收件箱已列出低敏 metadata 并链到摘要页。列表不拉正文；digest 内容仍只在详情页按既有限制读取。 |
| 浏览 / 创建 Definition | `/agent/definitions` | `GET` + `POST` profile | 创建已接线。目录仍不暴露模型/Tool。空目录时订阅创建会停在「没有 active Definition」。 |
| 创建 / 撤销订阅 | `/agent/subscriptions` | list / options / create / revoke | API 齐。订阅要先有 Definition + 可读 conversation。Runtime 默认 `direct_target`，列表 active 不会自动开共享事件触发；要另开 subscription overlay。 |
| 查看 / 撤销 / 纠正记忆 | `/agent/memories` | list / revoke / correct | 页面不能写入 Observation。自动写入关着时列表会一直空。 |
| 审核记忆候选 | `/agent/memories` 候选区 | `POST /memory-candidates/:id/review` | pending 行可接受/拒绝。理由固定为 owner accepted/rejected。 |
| 晋升记忆候选 | `/agent/memories` 候选区 | `GET /memory-candidates` + `POST /memory-candidates/:id/promote` | 记忆页已列出摘要并晋升 accepted+reviewId 行。 |
| 取消任务 | 时间线页取消按钮 | `POST /tasks/:id/cancel` | 已挂到 Timeline 头。终态任务再点会走 Runtime 既有错误。 |
| 运行状态灯 | 无页面 | `GET /api/v1/agent/status` | 运维/装配探测，不是产品收件箱。 |
| MCP / Repair / OAuth | 无 Vue 入口 | MCP HTTP、Repair Execute RPC、OAuth consume | 有意不进 SPA。Repair 是 mTLS 内部 RPC。 |

## 后端缺的能力（按阻塞排序）

当前前端可走的 owner 控制面缺口已接上。剩下的是默认关的运行时 overlay、以及有意不进 SPA 的 MCP / Repair / OAuth。

Chat 已消费 `agent_task_waiting`：会话栏提示、按 Task/revision 去重、`connected` 后以 `GET /tasks` 覆盖 locator。产物收件箱已接 `GET /artifacts`，详情页 digest 限制未改。

## 后端已有、前端仍走不全的（开关 / 运行时，不是缺 RPC）

- 任务创建与 HITL 写：`gateway.agent_control_enabled` + Interactive `/send` overlay。
- 订阅真正触发回复：`agent-subscription-autoreply` / active overlay，默认关。
- 记忆自动写入：`DIPOLE_AGENT_MEMORY_ENABLED`，默认关。
- Timeline 创建后短暂 404：前端已重试一次；投影延迟仍可能让用户看到「时间线暂不可用」。

## 不在本轮做的

不改默认 Compose / 生产 `VITE_*`。不把 MCP Server、Workflow Repair、OAuth 做成 Vue 页。
