# Legacy Store Entry Points

`internal/store/` retains the historical MySQL and Redis initialization symbols for compatibility. New code must use `internal/platform/mysql/` and `internal/platform/cache/` directly so connection ownership remains explicit.

Do not add domain state or new platform clients here. These forwarding variables and functions can be removed after the compatibility window and rollback validation are closed in the architecture debt ledger.
