# Controlled Shadow Eval Window: 2026-09-01 N=2

This archive contains a Remote GPU, isolated `read_shadow` observation of two
completed direct-message tasks. The evaluated task runtime was
`agent-runtime@4fd2d1363a65c3f397d94330515a9c1e2e65dab8`; the clean evaluator
image revision was `dd726dbc0fe0042ddd95c5b6995e69d4ea731b19`.

`summary.json` records two unique terminal Task/Run reports. Both passed the
five deterministic categories: outcome, trajectory, persisted authorization,
retrieval evidence and reported cost. The resulting `100%` applies only to
this selected N=2 completed-task cohort. It is not an overall Agent task
success rate and must not be used as the resume `[XX]%` value.

One additional controlled event in the same disposable stack failed after the
provider returned an empty JSON-text response. Its model record lacks token
counts, so the current five-category evaluator correctly rejects it as
incomplete cost evidence. That failed event is excluded from this archive and
remains a tracked evaluator-coverage gap. This window establishes no active
authority, external write capability, shared-environment behavior or user
impact.
