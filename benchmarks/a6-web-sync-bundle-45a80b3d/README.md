# A6 Web Sync Shadow Bundle - 2026-08-31

## Result

| Field | Value |
| --- | --- |
| Candidate version | `web-sync-shadow-45a80b3d` |
| Git commit | `45a80b3d475f4ba0317addab9d11ee0cb93397f2` |
| Mode | `shadow` |
| Remote output | `/tmp/dipole-dev-horeb-web-sync-shadow-45a80b3d.tar` |
| SHA-256 | `0c458602868170dbb45933f1c48fa0f9ba22c5978d6d79cb2d007cb0344bfdd5` |
| Host role | Remote GPU development host |

## Command

```bash
DIPOLE_REMOTE_BRANCH=dipole-dev/horeb \
DIPOLE_REMOTE_PROJECT=dipole-dev-horeb \
scripts/remote-dev.sh web-sync-bundle
```

The command synchronized the exact commit to the per-developer candidate ref,
then ran `package-web-sync-bundle.sh --mode shadow` on the Remote GPU host.

## Boundary

This is an immutable candidate input for the [Web Sync rollout](../../docs/operations/WEB-SYNC-ROLLOUT.md). It does not establish any of the following:

- Prometheus target or alert readiness.
- Incoming-direct comparison samples.
- A complete 24-hour observation window.
- Permission to switch the default client mode.

At collection time the host had active login sessions, so the guarded Compose
smoke and any traffic-bearing observation remained intentionally unstarted.
