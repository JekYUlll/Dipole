# 一次性运维操作

`internal/operations/` 收纳回填、对账、归档和受控切换等一次性操作的装配代码。

这些操作不属于长期运行服务的 Composition Root。长期服务启动代码保留在
`internal/bootstrap/`，命令行入口保留在 `cmd/tools/`，两者通过明确的操作包连接。

## 当前操作包

- `search/`：Search 索引回填、归档、对账、Alias 切换和 Outbox 清理。

操作包必须保持幂等、可审计，并为高风险写操作提供预览、确认和回滚路径。
