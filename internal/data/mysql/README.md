# MySQL Compatibility Boundary

`internal/data/mysql/` is retained only for historical repository package paths. Transaction ownership lives in `internal/platform/mysql/`; SQLC repositories live under `internal/services/<service>/infrastructure/mysql/`; one-shot operation adapters live under `internal/operations/<service>/<operation>/mysql/`.

Do not add new schema, repository, transaction, or migration implementation here. Message, Sync, and Store compatibility facades have been retired; the remaining Core, Agent, and Search aliases require a caller and rollback audit before removal.
