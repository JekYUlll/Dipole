# Multipart Fault Matrix Receipt

## Scope

This receipt records an isolated Remote GPU run from source revision
`0724408dcb0bd0f08dc0bca4b14c296f42bd28de` using Go `1.27.0`.
`scripts/smoke-multipart-fault-matrix.sh` created randomly named MinIO and
Redis containers on loopback ports only. It did not modify the public
`dipole-experience` Compose project or GPU workloads.

## Verified Behavior

- Six deterministic Go package suites passed: Multipart cleanup, storage
  reconciliation, Gateway HTTP, Gateway server, Gateway bootstrap, and config.
- Prometheus `multipart-alerts.yml` passed `promtool` validation with seven
  rules and the checked firing timelines.
- The real MinIO/Redis reconciliation smoke detected matched state, missing
  Redis metadata, and Redis orphan drift.
- The Redis restart variant restarted only its isolated Redis container, then
  verified missing metadata handling, MinIO cleanup, and orphan drift.
- The matrix exited with status `0`; its cleanup removed the temporary
  MinIO/Redis containers and worktree.

## Boundary

This is a disposable development fixture. It does not provide a 24-hour
presigned-upload traffic window, a production Alertmanager receiver result,
browser network evidence, or approval to change the default `relay` mode.
