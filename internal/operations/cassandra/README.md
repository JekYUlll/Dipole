# Cassandra Operations

本目录收纳 Message Timeline 的 Cassandra backfill、archive 和 reconciliation 操作。
Cassandra projector 与通用 Timeline 适配器继续分别位于 Message service 和 platform
目录，运维操作必须保留校验、幂等和回滚证据。
