# Dipole 学习、简历与面试入口

Dipole 的 IM 与 Agent Runtime 采用独立项目口径维护，避免传统分布式 IM 经历与 AI Agent 工程经历在投递和现场介绍中混用。

## 选择材料

| 投递方向 | 使用材料 | 重点 |
| --- | --- | --- |
| 后端、分布式系统、IM | [Dipole IM 项目学习、简历与面试](DIPOLE-IM-LEARNING-AND-INTERVIEW.md) | Timeline、Outbox、Kafka、SQLC、微服务、Multipart、Realtime |
| Agent、AI 应用、平台工程 | [Dipole Agent 项目学习、简历与面试](DIPOLE-AGENT-LEARNING-AND-INTERVIEW.md) | ExecutionContext、Capability、Temporal、Memory、MCP、Eval、权限 |

两个项目共享仓库与 IM Capability contract：Agent 通过受控 RPC 使用 IM 领域能力，却不把 IM 的存储、同步或连接层实现写成 Agent 成果。面试中可根据岗位选择其中一份作为主项目；涉及另一份时，仅说明其提供的受控集成边界。

性能、可靠性和成功率 claim 统一以[简历 Claim 验收矩阵](RESUME-CLAIM-READINESS.md)为发布门禁。矩阵中的“可使用”表示可以带限定范围表述；“部分完成”和“缺口”只能作为开发计划，不能提前写入简历。

## 维护规则

1. IM 的消息、存储、同步、文件、微服务和性能切片更新 IM 材料。
2. Agent 的 Runtime、模型、Tool、Memory、Temporal、MCP、权限和评测切片更新 Agent 材料。
3. 共享的契约或部署改动需要分别更新两份材料中的影响与限制。
4. 状态必须区分已验证、默认关闭和规划中；以代码、测试、基准和归档运行记录为准。

深入 IM 问答见 [Dipole IM 深入问答](INTERVIEW-QA.md)。架构事实以 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 和对应运行手册为准。
