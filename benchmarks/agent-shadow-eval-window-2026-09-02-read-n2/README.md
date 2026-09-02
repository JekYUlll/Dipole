# Controlled Shadow Eval Window: 2026-09-02 Actual Read N=2

This archive records an isolated, loopback-only Remote GPU `read_shadow`
functional window. The evaluated Agent image was built from clean revision
`d591bc7592b3974c6ec33425371f66fd9d3e29ea`.

The disposable fixture exposed exactly one direct conversation to the Task
owner and published two independent Kafka events. Each Task used trusted
`conversation.list` discovery followed by an authorized, actual
`conversation.read`; both then produced one `conversation_digest` Artifact.
Message write Capability, active authority, MCP, Memory promotion and public
network exposure remained disabled.

## Evidence

- [`manifest-set.json`](manifest-set.json) binds two reviewer manifests with a
  SHA-256 set digest and `minimumManifestCount=2`.
- [`summary.json`](summary.json) contains the low-sensitivity aggregate: two
  terminal samples passed outcome, trajectory, permission, retrieval and cost
  checks.
- [`review-pack-a.json`](review-pack-a.json) and
  [`review-pack-b.json`](review-pack-b.json) contain hashed Task/Run/Trace and
  resource bindings, plus the actual read trajectory and complete metering.

The reviewer labels were controlled fixture labels. The protected Remote GPU
workspace retains the raw manifests and per-sample reports; they include
Task/Run/Trace bindings and are intentionally excluded from this repository.

## Boundary

The `100%` in `summary.json` applies only to these two fixed, single-
conversation functional fixtures. It does not establish a general task success
rate, model quality, multi-conversation selection behavior, failure recovery,
active write safety, MCP behavior, shared-environment reliability, latency
SLOs, production authority or user impact. A resumable claim requires a larger
and independently reviewed multi-path corpus with explicit failure cases.
