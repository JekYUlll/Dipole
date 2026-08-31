# Withdrawn Read-Shadow Eval

This directory records a withdrawn 2026-09-01 development observation. The
disposable Remote GPU Compose smoke did establish Kafka, Temporal, MySQL, Go
Core mTLS capability RPC, one model call and a `conversation_digest` Artifact.
Its first Eval report was removed from the current evidence set.

At that revision, `agent_shadow_steps` persisted capability ID and terminal
status but did not persist the resolved resource scope. The Permission case
could bind the capability and status while obtaining `resourceType`,
`resourceId` and `action` from a reviewer manifest. It therefore cannot prove
that the evaluated scope matched the Runtime authorization decision.

The next evidence version must persist the resolved resource request and policy
decision under the Step lease, require the Eval adapter to compare every field,
then rerun the isolated observation. This withdrawn sample must not be used for
task success rate, permission safety, cost, shared-environment or active
authority claims.
