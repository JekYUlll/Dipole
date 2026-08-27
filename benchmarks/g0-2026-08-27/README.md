# G0 End-to-End Performance Baseline

本目录归档 2026-08-27 在 `ec979d4e237ccf7d0158bf8ce01c96e896118a93` 上采集的标准化报告。

| Scenario | JSON | Markdown |
| --- | --- | --- |
| Direct pairs | `direct.json` | `direct.md` |
| Concurrent ring | `concurrent.json` | `concurrent.md` |
| Regular 20-member group | `group-regular.json` | `group-regular.md` |
| Hot 20-member group | `group-hot.json` | `group-hot.md` |

所有场景都通过以下门禁：消息接受率和持久化率均为 100%，拒绝数为零，投递率至少 95%，Kafka 最终采样 lag 为零。原始 k6 summary、运行日志和诊断运行保留在被忽略的 `scripts/bench/results/`，避免将高噪声运行产物长期纳入仓库。

热群场景将成员阈值和消息阈值临时设为 `20/1`，先发送一条不计入测量的预热消息，再测量稳态 notify + pull；运行结束后，三个节点均已恢复默认 `200/50`。
