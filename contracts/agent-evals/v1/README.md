# Agent Evaluation Contract v1

`go-eino-baseline.json` 固化当前 Embedded Go/Eino Agent 的可观察行为。TypeScript Runtime、Prompt、Tool Schema 或权限模型迁移时复用这些 case，并通过 adapter 产出相同的 outcome、trajectory 和 permission 结果。

首批覆盖四类评测：

- `event`：只处理发给助手的 direct created Message，并按触发 Message 幂等。
- `reply`：普通回复通过 Message sender 回写，Tool 已发送消息时不再重复回写。
- `trajectory`：记录 Agent 与消息动作的顺序。
- `permission`：验证会话存在性拒绝，并持续以恶意身份参数检查服务端 principal 是否强制生效。

`known_gap` 表示当前 baseline 中已知且有债务编号的安全差距。缺口关闭时修改 case 预期和 adapter，同时保留攻击输入作为回归，不能静默删除用例。

`projection-reconcile.json` 固化 Temporal Workflow 与 Core/sqlc 投影的六类离线对账结果。该数据集覆盖一致、缺失、落后、超前、同 revision 状态冲突和 Temporal 不可用；报告 schema 固定为 `dipole.agent.projection-reconcile.v1`，任何非 `match` 结果均禁止作为 active 晋级证据。

## 五类离线评测

`offline-suite.json` 是 deterministic synthetic foundation，输入和输出遵循 `offline-suite.schema.json` 与 `offline-report.schema.json`。每个 Suite 必须同时包含 outcome、trajectory、permission、retrieval 和 cost，case ID 在 Suite 内唯一；Runtime 对规范化 Suite 计算 SHA-256，并且报告只保留稳定 ID、失败原因和数值指标，不回显消息或模型正文。

```bash
cd agent-runtime
npm run eval:offline -- --suite=../contracts/agent-evals/v1/offline-suite.json
```

退出码 `0` 表示全部 case 通过，`2` 表示有效 Suite 存在评测失败，`1` 表示参数或契约无效。当前 fixture 用于验证 Harness 语义，真实 Shadow Task adapter、人工标注 corpus 和检索相关性基线由 `AD-038` 跟踪。

`security-suite.json` 将五条真实 Runtime 结构性安全探针映射到同一 Harness：MCP 敏感参数外发对应 outcome，同源 Agent 循环对应 trajectory，越权 Capability 对应 permission，Prompt Injection provenance 对应 retrieval，重复事件的 planner 调用上限对应 cost。该 Suite 验证代码路径与稳定证据，不代表模型语义已免疫 Prompt Injection；真实 adversarial model corpus 仍需在 Shadow 晋级前归档。

## 真实 Shadow Task 适配

`shadow-manifest.schema.json` 将评审标签、Task/Run 绑定、检索阈值和版本化模型价格与数据库 observation 分离。`shadow-manifest.example.json` 只展示格式，其中的 Task、Evidence 和价格均为占位值，不能作为晋级证据。

运行前为独立只读账号应用 `configs/mysql/agent-eval-grants.dist.sql`，并以受控 Secret 配置连接：

```bash
cd agent-runtime
export DIPOLE_AGENT_EVAL_MYSQL_URL='mysql://dipole_agent_eval:...@mysql:3306/dipole'
npm run eval:shadow -- --manifest=../path/to/reviewed-shadow-manifest.json
```

Adapter 从真实 `Task/Run/Plan/Step/Artifact/ModelCall/ToolCall` 生成 observation。五个 case ID 携带 Task/Run SHA-256 摘要，Suite SHA-256 因此绑定候选、评审标签、派生 observation 与来源执行。Context provenance 仅输出 `evidence:<sha256>`，报告不回显 Task/Run、消息、Prompt、模型输出、Tool 参数或 Artifact 正文。集合查询读取约定上限外一条哨兵记录，超过 256 个 Step/Artifact/MCP 调用或 64 次模型调用时拒绝评测，避免无界读取和静默截断。

评审清单中的 `permission.stepNo` 绑定真实 Step；capability 必须与持久记录一致，decision 由 Step 终态派生。模型调用缺少 Token/延迟、Step 或 MCP Tool 调用缺少延迟、路由没有单价、Task/Run/Step 尚未终止时，命令返回 `1` 并 fail closed。成本按每百万 Token 的微美元整数单价计算并向上取整；Tool 次数统计 Shadow Step 与独立 MCP 调用，延迟为模型、Step 和 MCP 调用的记录耗时之和。当前 Step 表只保留最后一次 attempt 的区间，`attempt_count != 1` 时拒绝生成成本证据，后续逐 attempt 审计完成前不会低估重试。退出码 `0` 表示五类通过，`2` 表示有效证据未达阈值，`1` 表示清单、持久证据或连接无效。
