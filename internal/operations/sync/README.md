# Sync Operations

本目录收纳 Sync baseline、replay 和 reconciliation 等一次性操作。长期运行的 Sync
Service 仍位于 `internal/services/sync/`，其启动与生命周期管理位于
`internal/services/sync/bootstrap/runtime.go`。
