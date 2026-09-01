# Controlled Shadow Eval Window: 2026-09-02 N=4

This archive records an isolated Remote GPU `read_shadow` functional cohort.
The evaluated runtime was `agent-runtime@f72e47cf83825169f02ce6014f233a0830831854`.
The V2 evaluator image was built from a clean source revision and re-summarized
the existing protected reports without changing the running Agent, database, or
Kafka topics.

All four reviewed terminal samples exercised the same no-discovery path:
`conversation.list` completed and the following `conversation.read` was
persisted as `not_required/no_discovered_conversation`. Each passed outcome,
trajectory, permission, retrieval, and cost checks. `manifest-set.json` binds
the fixed four-reviewer-manifest set; `summary.json` contains only aggregate
statistics and Suite hashes.

The reported `100%` is a controlled regression result for this one safe-skip
path. It does not measure general task quality, failure recovery, multi-turn
retrieval, active authority, external write behavior, shared-environment
behavior, or user impact. It must not be used as the resume task-success
placeholder.
