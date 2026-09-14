# Benchmark 产物脱敏

2026-09-14 对 9 份历史 k6 summary 中的完整 JWT 替换为 `[REDACTED_JWT]`。
统计指标保持原值；涉及文件的 SHA256SUMS 已更新。因此这些文件是脱敏后的实验记录。

`scripts/bench/run_bench.sh` 现在通过 `SUMMARY_JSON` 调用两个压测入口的
`handleSummary`，仅导出指标和阈值，排除包含登录数据的 setup 内容。
不要额外使用 k6 的 `--summary-export` 导出完整执行数据。

此变更只处理当前工作区。历史 Git 提交仍可能包含旧令牌；未连接原运行环境验证有效性，
也未改写 Git 历史。公开分发前需评估历史暴露与必要的凭据失效处理。
