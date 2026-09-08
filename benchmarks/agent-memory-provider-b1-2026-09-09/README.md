# Agent Memory B1 Provider Receipt

该目录保存 2026-09-09（Asia/Shanghai）在 Remote GPU 隔离 Compose 项目生成的低敏 B1 Provider smoke 回执。运行时通过 `docker compose --env-file` 读取既有 Provider 环境文件；仓库、日志和本目录均不保存凭据、会话文本、助手回复、owner 或原始 Task/Memory ID。

[`receipt.json`](./receipt.json) 绑定运行时 revision、`provider` 模型源和两个 SHA-256 Task 标识，并记录以下可复核事实：

- 首个入站 Task 完成一次模型调用并写入一次已审核 Memory 的 pre-model lineage。
- owner revoke 后的第二个新 Task 仍完成一次模型调用，已撤销 Memory 的 lineage 为零。
- B1 smoke 与随后 Interactive Active 审批幂等演练均完成，隔离候选容器清理为零；公共 `dipole-experience` 保持 11 个容器。

该回执证明真实 Provider 链路可用和撤销授权边界。它不保留自然语言输出，没有语义 gold label，也没有多样化样本集，因此不能用于主张召回质量、任务成功率或默认持久 Memory 已启用。
