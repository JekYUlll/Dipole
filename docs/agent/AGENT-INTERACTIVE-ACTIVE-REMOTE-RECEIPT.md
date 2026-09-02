# Interactive Active Remote Receipt

## Same-Revision Runtime Status And Active Smoke (2026-09-02)

This development-only Remote GPU run used source revision
`9884b84886b19916ae87a5421c90c46e64b5c589` for the isolated Compose project
`dipole-agent-status-9884b848`. The Gateway was bound only to
`127.0.0.1:18130`; the project used unique Kafka and Temporal queues together
with the candidate-only MySQL native-AIO compatibility overlay.

The smoke registered one temporary owner, obtained a session through Core, and
called the public Gateway route. It required a `200` response from
`GET /api/v1/agent/status` with the low-sensitive active Runtime contract:
`dipole.agent.runtime_status.v1`, active mode, enabled Task control, and
enabled interactive message writes. The test then retained the existing
deterministic approval checks.

| Scenario | Result |
| --- | --- |
| Authenticated Runtime status | Gateway returned the expected active control-plane flags |
| Deny replay | Zero Tool Invocations and zero messages |
| Approve replay | One completed Tool Invocation, one message, and two Sync Inbox entries |
| Cleanup | Candidate containers, volumes, and loopback port were absent after exit |

The fixture uses the compose smoke's deterministic provider path and does not
evaluate model quality. It also does not establish a public browser experience,
shared-tenant authority, capacity, latency, availability, or a task-success
rate.

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
