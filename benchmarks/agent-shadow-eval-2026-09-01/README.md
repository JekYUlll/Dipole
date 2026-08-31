# Read-Shadow Eval Evidence

`report.json` records one owner-reviewed, synthetic `read_shadow` execution
observed on 2026-09-01 in a disposable Remote GPU Compose project. The
execution used Kafka, Temporal, MySQL, Go Core mTLS capability RPC and the
DeepSeek V4 Flash development route. The associated Core-restart smoke also
confirmed one EventLedger, Task, Run, model call and `conversation_digest`
Artifact after Core recovery.

The report evaluates candidate `agent-runtime@b808d18c` with the corrected
`45cb1da3` Eval adapter. It passes one case in each required category:
outcome, trajectory, permission, retrieval and cost. The report only includes
a synthetic trace ID, stable case digests and aggregate metrics; it contains no
message, prompt, model output, tool parameters or credential.

The cost metric uses a deliberately conservative development evaluation price
of 1,000,000 micro-USD per million input and output tokens. It is a budget
threshold for this controlled test, not a DeepSeek bill or a production cost
claim. This sample is `N=1`; it validates the real adapter path and must not be
presented as an Agent success rate, shared-environment result or active
authority evidence. A reviewed multi-sample window remains required before any
success-rate claim.
