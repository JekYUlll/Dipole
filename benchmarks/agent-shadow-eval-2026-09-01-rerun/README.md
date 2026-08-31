# Read-Shadow Eval Rerun

This directory contains one low-sensitivity, disposable Remote GPU observation
for `agent-runtime@064568d9`. The isolated Compose project exercised Kafka
event admission, Temporal read-shadow execution, Core mTLS capabilities,
MySQL audit projection and the dedicated `dipole_agent_eval` read-only
evaluation account.

[`report.json`](report.json) records a passing five-category Shadow Eval:

- Outcome, trajectory, permission, retrieval and cost each passed once.
- Retrieval precision and recall were both `1` after control-plane context was
  excluded from the retrieval denominator.
- The single observed execution recorded one model call, two tool calls, 1765
  tokens and 7032 ms aggregate measured latency.

The report is an isolated `N=1` development observation. It must not be used
as a production task-success rate, shared-environment reliability result,
active-authority approval, write-capability safety claim or latency SLO. A
claimable window requires version-consistent, reviewed, unique-trace reports
through the Shadow summary contract.
