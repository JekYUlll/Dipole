# Web Sync Observability Smoke Receipt

- Source revision: `8cecb0ef28ca72bb509e800861290e3da868409c`
- Executed at: 2026-09-04
- Environment: Remote GPU `LAB113-OPS`, isolated loopback-only Compose project
  `dipole-web-sync-preflight-20260904-v4`
- Result: passed Core, Message, Sync and Gateway metrics targets plus Prometheus
  and Alertmanager readiness.
- Remote log: `/data/admin1/dipole-evidence/web-sync-observability-8cecb0ef-20260904/smoke.log`
- Remote log SHA-256: `a73020c31b72817c5f178e62a70cb7c2bbc00676f93e128e0983de541515cbfd`
- Cleanup: candidate project containers `0`; public `dipole-experience` containers
  remained `12`.

This is a development preflight receipt. It does not include real incoming-direct
traffic, a 24-hour client observation window, immutable object-storage evidence,
or a Web Sync promotion decision.
