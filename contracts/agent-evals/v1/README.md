# Agent Evaluation Contract v1

`go-eino-baseline.json` 固化当前 Embedded Go/Eino Agent 的可观察行为。TypeScript Runtime、Prompt、Tool Schema 或权限模型迁移时复用这些 case，并通过 adapter 产出相同的 outcome、trajectory 和 permission 结果。

首批覆盖四类评测：

- `event`：只处理发给助手的 direct created Message，并按触发 Message 幂等。
- `reply`：普通回复通过 Message sender 回写，Tool 已发送消息时不再重复回写。
- `trajectory`：记录 Agent 与消息动作的顺序。
- `permission`：验证会话存在性拒绝，并显式记录模型可提交身份或写入目标的 `AD-008` 缺口。

`known_gap` 表示当前 baseline 中已知且有债务编号的安全差距。迁移后的目标实现应通过修改 case 预期和 adapter 测试关闭缺口，不能静默删除用例。
