# Architecture Documentation Policy

Dipole 的长期架构文档需要随实现一起评审、提交和回滚。受管清单位于 `docs/architecture-docs.manifest`，检查入口为：

```bash
./scripts/check-architecture-docs.sh
```

清单当前覆盖平台演进、架构债务、sqlc 数据访问、微服务部署、Sync、Cassandra Timeline、Elasticsearch Search、Realtime Delivery、Agent Runtime、前端设计和性能基线。新增长期架构约束时，应同步更新清单、实现文档和 `CHANGELOG.md`。

`docs/` 下另有一组本地研究、面试和历史分析材料。它们通过文件级规则显式忽略，不承担当前实现契约：

- `architecture-reference.md` 维护 `acc/` 参考项目的本地摘录，持续保持未跟踪。
- `architecture-qa.md`、`interview-qa.md` 和 `load-test-report.md` 属于历史问答或报告。
- `development-roadmap.md`、`ai-readiness-checklist.md` 和 `cache-strategy.md` 属于早期方案草稿。
- `message-storage-and-sync-model.md`、`message-sync-strategy.md` 和 `tls-setup.md` 尚未完成当前实现对齐。

这组本地文件需要转为正式契约时，应先校验代码、配置和运行手册，再移除对应的单文件忽略规则并加入 manifest。禁止恢复 `docs/*.md` 通配忽略，以免新的关键文档脱离代码审查。
