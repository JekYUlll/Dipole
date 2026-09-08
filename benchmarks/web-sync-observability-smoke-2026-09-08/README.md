# Web Sync Observability Smoke Receipt

- Source revision: `3f6398aa3`
- Executed at: 2026-09-08
- Environment: Remote GPU `LAB113-OPS`, isolated loopback-only Compose project
  `dipole-web-sync-observability`
- Configuration: an operator-owned, mode-`0600` environment file was supplied
  only through `docker compose --env-file`; no secret values were copied or
  printed by the smoke.
- Result: passed Core, Message, Sync and Gateway metrics targets plus
  Prometheus and Alertmanager readiness.
- Remote log: `/data/admin1/dipole-evidence/web-sync-observability-3f6398aa-20260908/smoke.log`
- Remote log SHA-256: `1911a7b08b3a0abfd06886a658a6b79a53b9a1a14a8eb3724c9222c6cf9c6964`
- Cleanup: candidate project containers `0`; public `dipole-experience`
  containers remained `11`.

This is a development preflight receipt. It does not include real incoming-direct
traffic, a 24-hour client observation window, immutable object-storage evidence,
or a Web Sync promotion decision.
