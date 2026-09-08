# Agent Inbound Context E2E Receipt

- Source revision: `1f6813fbf`
- Executed at: 2026-09-08
- Environment: Remote GPU `LAB113-OPS`, public development Compose project
  `dipole-experience`
- Preconditions: legacy direct/group reply switches were `false`; governed
  inbound interactive switching was `true`.
- Scenario: a fresh user sent a unique temporary code to the assistant, then
  asked for that code in a second message. Each message created a distinct
  governed interactive Task.
- Result: both Tasks completed, each turn produced exactly one assistant reply,
  and the second reply contained the first-turn code supplied only through the
  conversation transcript.
- Remote log: `/data/admin1/dipole-evidence/agent-inbound-context-1f6813fb-20260908/e2e.log`
- Remote log SHA-256: `41a590c7fbc70c3223085803adb7a3f6b36ed7c2724894d7fa6ac481aedbc0aa`
- Public project containers after the run: `11`.

This is a controlled development experience check. It validates short-term
conversation context over two inbound Tasks. It does not enable persistent
Memory, establish long-horizon recall quality, or provide a task-success-rate
claim.
