# B1 Provider Memory Window: 399af6f7

This directory is the low-sensitivity Remote GPU evidence for the B1 Provider
Memory window run with the committed Agent image `399af6f7`.

| Canary | Recall | Revoke lineage |
| --- | --- | --- |
| `ORBIT-91` | pass | 0 |
| `NOVA-42` | pass | 0 |
| `QUARTZ-17` | pass | 0 |

`window.json` binds the three independent suites to one candidate.
`window-report.json` is the fail-closed CLI result: three recall cases passed,
the recall rate is `10000` bps, and there are no revoke-boundary or invariant
failures. Each receipt stores hashes, counters, and booleans only; it omits
credentials, owners, conversations, prompts, Memory bodies, and replies.

The isolated Compose projects were removed after the runs. This validates the
synthetic recall gate only. Default persistent Memory remains disabled pending
owner-reviewed real corpus evaluation, multi-turn tasks, and a cross-provider
comparison.
