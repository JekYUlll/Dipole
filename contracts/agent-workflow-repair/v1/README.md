# Agent Workflow Repair Execution Plan v1

该契约描述一次可审计的 Workflow projection 修复预演。v1 的 `mode` 固定为 `dry_run`，仓库当前没有 apply、execute 或 rollback RPC，也没有会修改 projection 的 executor 实现。

预演必须满足以下边界：

- Proposal 已由 v25 审计服务标记为 `approved`，并携带恰好两位不同审批人的快照。
- Executor 使用独立授权版本，身份需与 proposer 和两位 approver 全部不同；授权仍由未来控制面校验，模型不能提供身份。
- `expectedCurrentProjection` 与 proposal 的原投影一致，`targetProjection` 来自重新查询的同一 Temporal Workflow/Run，执行前必须再次按 SHA-256 做 compare-and-swap。
- `rollbackProjection` 固定为修改前状态；原投影缺失时使用 `null`，未来 rollback 需使用另一个显式授权命令和当前状态 CAS。
- Plan 最长存活 15 分钟；超时、Proposal/审批变化、Temporal evidence 漂移或当前 projection 漂移都会要求重新生成。

后续如需引入实际修改，应发布新的契约版本，并分别完成最小权限 executor 身份、持久 execution ledger、dry-run 与 apply 的二次确认、rollback 演练、Pencil 审批界面和生产注册评审。v1 文件不得通过放宽 `mode` 直接加入写操作。
