# Agent OAuth Token Lifecycle MySQL/mTLS Restart Receipt

- **Date:** 2026-09-04
- **Revision:** `de3f8974`
- **Environment:** Remote GPU development host; disposable MySQL Compose project
- **Toolchain:** Go `1.27.0`
- **Command:**

```text
DIPOLE_GO_BIN=/home/admin1/.local/go-1.27.0/bin/go \
DIPOLE_AGENT_OAUTH_LIFECYCLE_MYSQL_PORT=23326 \
./scripts/drill-agent-oauth-token-lifecycle-mysql-mtls-restart.sh
```

## Result

The script created one disposable MySQL 8.4 container, applied the full SQLC
migration set to a per-test schema, and passed
`TestAgentOAuthTokenLifecycleMySQLMTLSRestartContract`.

The contract verified:

1. A live `exchange_claimed` handoff accepts one opaque `active` lifecycle write
   over TLS 1.3 mTLS as `dipole-agent`.
2. A Core listener restart preserves the row; an exact request replay succeeds.
3. A replay with altered lifecycle metadata is rejected.
4. The final SQLC-backed table contains exactly one lifecycle row.

The disposable project was removed after the test. The public
`dipole-experience` stack was observed at 11 containers before and after this
run; the drill did not target or restart public containers.

## Evidence Boundary

This proves the Core/MySQL persistence and restart boundary only. It does not
register the public Gateway callback route, start a Runtime provider exchange,
open a token envelope, run Kafka or Temporal, or provide long-lived
refresh/revoke/retention authority. Default OAuth and Compose profiles remain
disabled.
