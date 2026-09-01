# Agent Temporal Fault Receipt - 2026-09-01

Remote GPU candidate `6beab05d` ran the isolated Temporal Worker replacement
drill with Node `22.12.0`. The integration suite passed `7/7`, then the
`receipt:temporal-fault` CLI independently verified both archived receipts.

| Drill | Result | Bound recovery path |
| --- | --- | --- |
| `worker_replacement_approval_resume` | `eligible` | approval Signal resumes on the replacement Worker; one injected terminal retry persists once |
| `worker_replacement_input_resume` | `eligible` | exact Elicitation input resumes once; invalid and stale inputs remain rejected |

The run used Temporal's in-memory test server. It did not start Compose or
connect to Kafka, Core, MySQL, a shared tenant, or active authority. Separate
Core restart and EventLedger lease receipts retain their own scope.

Reproduce from an environment with Node 22 and runtime dependencies:

```bash
cd services/agent-runtime
receipt_dir="$(mktemp -d)"
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true \
DIPOLE_AGENT_TEMPORAL_FAULT_EVIDENCE_DIR="$receipt_dir" \
npm test -- --run src/temporal/agent-task-workflow.integration.test.ts
for receipt in "$receipt_dir"/*.json; do
  npm run receipt:temporal-fault -- --receipt="$receipt"
done
```
