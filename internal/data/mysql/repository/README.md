# Legacy Repository Aliases

This package contains type aliases and constructor forwarding for callers that still use the historical `internal/data/mysql/repository` import path. The implementations are owned by the corresponding service packages:

- Core: `internal/services/core/infrastructure/mysql/`
- Message: `internal/services/message/infrastructure/mysql/`
- Sync: `internal/services/sync/infrastructure/mysql/`
- Search: `internal/services/search/infrastructure/mysql/`
- Agent: `internal/services/agent/infrastructure/mysql/`

New runtime, operation, or service tests must import the owner package directly. Keep this package only until the documented rollback and compatibility window is retired.
