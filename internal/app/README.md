# Compatibility Facades

`internal/app/` only contains narrow compatibility facades and their tests. Embedded composition belongs to `internal/bootstrap/embedded/`; long-lived service composition belongs to `internal/services/<service>/`.

New service code must not be added here. When an embedded caller needs a migrated type, keep a narrow forwarding alias and document the removal condition in `docs/architecture/ARCHITECTURE-DEBT.md`.
