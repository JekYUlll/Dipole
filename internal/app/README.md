# Embedded Compatibility Facade

`internal/app/` only contains the embedded Composition Root facade and its compatibility tests. Long-lived service composition belongs to `internal/bootstrap/` and `internal/services/<service>/`.

New service code must not be added here. When an embedded caller needs a migrated type, keep a narrow forwarding alias and document the removal condition in `docs/architecture/ARCHITECTURE-DEBT.md`.
