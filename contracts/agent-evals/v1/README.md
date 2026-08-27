# Agent Evaluation Contract v1

`go-eino-baseline.json` 固化当前 Embedded Go/Eino Agent 的可观察行为。TypeScript Runtime、Prompt、Tool Schema 或权限模型迁移时复用这些 case，并通过 adapter 产出相同的 outcome、trajectory 和 permission 结果。

首批覆盖四类评测：

- `event`：只处理发给助手的 direct created Message，并按触发 Message 幂等。
- `reply`：普通回复通过 Message sender 回写，Tool 已发送消息时不再重复回写。
- `trajectory`：记录 Agent 与消息动作的顺序。
- `permission`：验证会话存在性拒绝，并持续以恶意身份参数检查服务端 principal 是否强制生效。

`known_gap` 表示当前 baseline 中已知且有债务编号的安全差距。缺口关闭时修改 case 预期和 adapter，同时保留攻击输入作为回归，不能静默删除用例。

`projection-reconcile.json` 固化 Temporal Workflow 与 Core/sqlc 投影的六类离线对账结果。该数据集覆盖一致、缺失、落后、超前、同 revision 状态冲突和 Temporal 不可用；报告 schema 固定为 `dipole.agent.projection-reconcile.v1`，任何非 `match` 结果均禁止作为 active 晋级证据。
