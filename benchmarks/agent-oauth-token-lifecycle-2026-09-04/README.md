# Agent OAuth Token Lifecycle Remote Validation Receipt

- **Date:** 2026-09-04
- **Revision:** `85af1e88`
- **Environment:** Remote GPU development host, isolated source/toolchain validation
- **Toolchains:** Go `1.27.0`; Node `22.12.0`
- **Public service impact:** none; public `dipole-experience` remained at 12 running containers

## Commands

```text
CGO_ENABLED=0 go test ./internal/application \
  -run '^TestAgentOAuthTokenLifecycleWriteRequestValidation$' -count=1

npm test -- --run \
  src/mcp/oauth-token-lifecycle-envelope.test.ts \
  src/mcp/oauth-token-lifecycle-persistence-client.test.ts \
  src/mcp/oauth-callback-handoff-provider-processor.test.ts

npm run typecheck
```

## Result

- Go lifecycle contract: passed.
- Runtime envelope, Core persistence client and provider processor: 3 test files,
  11 tests passed.
- Runtime TypeScript typecheck: passed.

## Evidence Boundary

This receipt validates source/toolchain compatibility only. It does not start a
callback route, exchange against a real provider, connect a Core mTLS server,
write MySQL, or enable any default OAuth/Compose profile. A future restart
drill must prove lease-bound persistence against disposable MySQL and Core
mTLS before treating the lifecycle as deployment-ready.
