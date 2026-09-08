# B1 Provider Multi-Canary Window

本窗口在 2026-09-09（Asia/Shanghai）于 Remote GPU 的三个独立 Compose 项目中，使用真实 DeepSeek V4 Flash Provider 运行三个 synthetic B1 Memory canary。每次运行均只保存 canary、Task 和回复的 SHA-256，以及完成状态、模型调用计数、Memory lineage 计数和 canary 命中布尔值；没有归档 owner、会话、提示词、回复正文或凭据。

| Canary | Recall | Revoke lineage | Eval |
| --- | --- | --- | --- |
| `ORBIT-91` | pass | 0 | [既有 2/2 基准](../agent-memory-b1-synthetic-eval-2026-09-09/) |
| `NOVA-42` | pass | 0 | 2/2, 10000 bps |
| `QUARTZ-17` | fail | 0 | 1/2, 5000 bps |

因此，三个 recall 样本中有 `2/3` 命中，三个 owner revoke 后的新 Task 均没有目标 Memory lineage。`QUARTZ-17` 的失败已完整保留为 `synthetic_canary_not_recalled`，没有通过修改门槛或重试覆盖。revoke 回复文本仍只作观察，因为直接会话的短期历史可能包含先前答案；Core lineage 是撤销读取边界的权威依据。

[`window.json`](window.json) 将三份 suite 绑定到同一 candidate，并由 `eval:memory-b1-window` 复算。窗口报告将 `2/3` recall 命中表示为 `6666` bps，并以 `recall_below_minimum` 失败；三条 revoke 边界和所有其余执行不变量通过。

复算新增 suite：

```bash
cd services/agent-runtime
npm run eval:memory-b1-synthetic -- \
  --suite=../../benchmarks/agent-memory-b1-provider-window-2026-09-09/nova-42-suite.json

npm run eval:memory-b1-synthetic -- \
  --suite=../../benchmarks/agent-memory-b1-provider-window-2026-09-09/quartz-17-suite.json
```

第二条命令以退出码 `2` 返回，代表有效 Eval 未达到全通过门槛。此窗口只覆盖 synthetic token recall，尚未包含 owner-reviewed 真实语料、人工语义标注、多轮任务或跨 Provider 对照，因此不能用于宣称默认持久 Memory 的语义质量或启用条件已满足。
