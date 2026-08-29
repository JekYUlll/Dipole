# Legacy Repository Aliases

This package contains type aliases and constructor forwarding for callers that still use the historical `internal/data/mysql/repository` import path. Contract database helpers and repository tests belong to the owning service packages. The implementations are owned by the corresponding service packages:

- Core: `internal/services/core/infrastructure/mysql/`
- Search: `internal/services/search/infrastructure/mysql/`
- Agent: `internal/services/agent/infrastructure/mysql/`

Message and Sync repository compatibility facades have been retired after a repository-wide caller audit. New runtime, operation, or service tests must import the owner package directly. Keep the remaining aliases only until their documented rollback and compatibility windows are retired.
