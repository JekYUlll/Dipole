# Agent Subscription Shadow 观察

该模式在 `direct_target` 主路径进入 EventLedger 前，对同一消息调用 Core 的确定性 Subscription matcher，并只记录对照指标。它不会创建 Subscription binding、Task、Run、Temporal Workflow 或模型调用。

## 启用前提

- 保持 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`。
- Kafka Runtime 与 Agent Capability RPC 必须启用并通过现有 mTLS/readiness 检查。
- 显式设置 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED=true`；Compose 默认固定为 `false`。
- 先确认 Prometheus 的 `dipole-agent` target 正常，并能读取 `dipole_agent_subscription_shadow_enabled`。

## 观察指标

- `dipole_agent_subscription_shadow_comparisons_total{direct_target,subscription}`：固定 `accepted|ignored` 与 `match|miss|error` 六种组合。
- `dipole_agent_subscription_shadow_candidates_total`：匹配候选总数，不含 Subscription、Definition、principal 或 event 标识。
- matcher 错误只累计 `subscription="error"`，不阻断 direct-target 主路径；EventLedger 与下游调用次数不因此增加。

至少观察完整业务周期，并联查 `DipoleAgentSubscriptionShadowErrors` 与 `DipoleAgentSubscriptionShadowAdmissionDrift`。对照数据只能用于形成后续晋级证据；真实 reviewed corpus、precision/recall、成本和漏触发门槛仍由 AD-034 管理。

## 回滚

1. 设置 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED=false` 并滚动 Agent Runtime。
2. 确认 `dipole_agent_subscription_shadow_enabled 0`，Core matcher 调用停止。
3. 保持 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`；无需数据库迁移或 EventLedger 清理。

若 matcher error 告警触发，优先执行上述回滚。不得通过切换到 `subscription` 主模式规避告警。
