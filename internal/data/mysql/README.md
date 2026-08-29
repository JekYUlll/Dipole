# MySQL Compatibility Boundary

`internal/data/mysql/` is retained only for historical package paths. Transaction ownership lives in `internal/platform/mysql/`; SQLC repositories live under `internal/services/<service>/infrastructure/mysql/`; one-shot operation adapters live under `internal/operations/<service>/<operation>/mysql/`.

Do not add new schema, repository, transaction, or migration implementation here. Remove compatibility files only after all callers and rollback procedures have been migrated and verified.
