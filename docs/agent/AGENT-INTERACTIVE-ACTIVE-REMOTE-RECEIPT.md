# Interactive Active Remote Receipt

## Scope

This receipt records a development-only acceptance run on 2026-09-02. The
isolated Remote GPU Compose project `dipole-agent-active-430c9e38` used source
revision `430c9e3891b26d49358465de98885b4b3f209f5f`. Core, Gateway, Message,
Sync, Search, migration, timeline repair, and Agent images carried that exact
revision with `dirty=false` provenance.

Gateway listened only on `127.0.0.1:18112`. The run did not change host
networking, the Docker daemon, or any other Compose project. The loaded
profiles included the isolated MySQL compatibility overlay, Temporal runtime,
active Runtime, `interactive_active` Activities, and the DeepSeek V4 Flash
provider profile. The test queue and Kafka consumer group were unique to this
project.

## Acceptance

The development fixture created one owner and one distinct Agent identity. A
development promotion grant was installed only for this run and revoked after
the checks completed.

| Scenario | Controlled input | Durable result | Side-effect check |
| --- | --- | --- | --- |
| Deny replay | Two concurrent owner `denied` control requests | One `approval_denied` cancellation; approval revoked | Zero Tool Invocations and zero messages |
| Approve replay | Two concurrent owner `approved` control requests | Workflow completed at revision 4; approval consumed once | One completed Tool Invocation, one action command/reference, and one Message with one client message ID |

Both control requests in each scenario returned `202`; Temporal and the
persisted approval state serialized the duplicate requests. The approved
scenario used the active Message Service path and persisted the action
reference before the resulting message was counted.

At the time of this historical `430c9e38` receipt, `agent_tasks.status`
remained `running` as a policy lifecycle projection. Later revision
`d591bc75` converged a parent Task to the same terminal state as its Run; see
the [terminal convergence receipt](../../benchmarks/agent-terminal-convergence-2026-09-02/).
This historical acceptance therefore records `workflow_status=completed` and
a completed Run, without claiming that its earlier Task projection had already
used the newer terminal contract.

## Fixture And Security Boundaries

The owner and Agent must be distinct identities. An earlier agent-to-self
fixture was rejected by the direct-conversation boundary after consuming its
approval and created no message; it is retained as a fixture precondition,
not an acceptance result.

Task identifiers, request identifiers, user identities, message text,
credentials, and promotion secrets are deliberately excluded from this
receipt. The temporary development grant was revoked after verification, so
the retained project has no ongoing active-write authority.

## Limits And Follow-up

This proves a clean same-revision, loopback-only development path for real
Core/Message/Temporal persistence with owner deny and duplicate approve/deny
inputs. It does not establish a public browser experience, a shared tenant
deployment, Worker/Core/Message fault recovery, rollback under a partial
effect, MCP expansion, capacity, latency, availability, or a success-rate
claim. Those gates remain tracked by `AD-009`.
