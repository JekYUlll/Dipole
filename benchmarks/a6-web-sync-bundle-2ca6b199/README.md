# A6 Web Sync Shadow Bundle - 2026-08-31

## Result

| Field | Value |
| --- | --- |
| Candidate version | `web-sync-shadow-2ca6b199` |
| Git commit | `2ca6b1992d409ad4b4dab4fc86842cd28cc1e543` |
| Mode | `shadow` |
| Remote output | `/tmp/dipole-web-sync-shadow-2ca6b199.tar` |
| SHA-256 | `d72207d7c70a88d4dc0f11c348c2e545589c3d5dbfd056aab323f1aee78b3b18` |
| Permissions | `0600` |
| Host role | Remote GPU development host |

## Verification

The isolated worktree built the Vue production output with Node `22.12.0` and
then ran:

```bash
scripts/check-web-sync-observation.sh
scripts/package-web-sync-bundle.sh \
  --candidate-version web-sync-shadow-2ca6b199 \
  --mode shadow \
  --output /tmp/dipole-web-sync-shadow-2ca6b199.tar
```

The observation contract suite passed all 14 tests. The tar metadata confirms
that the candidate, revision, and Shadow mode are bound together.

## Boundary

This package is an immutable input to the
[Web Sync rollout](../../docs/operations/WEB-SYNC-ROLLOUT.md). It does not
prove Prometheus or Alertmanager readiness, client comparison samples, a
complete 24-hour observation window, or permission to change the default
client mode.

At collection time the Remote GPU host had 25 active login sessions. The
guarded Compose smoke and all traffic-bearing observation activities therefore
remained unstarted.
