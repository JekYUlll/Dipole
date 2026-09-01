# Agent Read-Scope Confirmation Receipt

This receipt records a disposable Remote GPU observation of the interactive
read-shadow flow on clean revision `d591bc7592b3974c6ec33425371f66fd9d3e29ea`.
The isolated Compose project exposed Gateway only on loopback and retained the
default read-only Agent authority.

## Observed Flow

```mermaid
sequenceDiagram
    participant Owner as Authenticated owner
    participant Gateway
    participant Runtime as TS Agent Runtime
    participant Temporal
    participant Core as Core Capability

    Owner->>Gateway: start interactive read task
    Gateway->>Runtime: trusted principal + goal
    Runtime->>Temporal: durable task
    Temporal->>Core: conversation.list
    Core-->>Temporal: two visible conversations
    Temporal-->>Owner: waiting_input with two choices
    Owner->>Gateway: forged request ID
    Gateway-->>Owner: 409, task remains waiting_input
    Owner->>Gateway: confirmed offered choice
    Gateway->>Runtime: trusted owner input
    Runtime->>Temporal: resume signal
    Temporal->>Core: conversation.read for confirmed choice
    Temporal-->>Owner: completed digest
```

The Gateway accepted the authenticated task start (`202`). The Task paused in
`waiting_input` with two discovered choices. A forged request ID returned
`409` and preserved that state. The valid, offered choice returned `202`; the
Task and Run then completed. Persistent trajectory checks show exactly one
`conversation.list`, one authorized `conversation.read` for the selected
conversation, zero reads for the other discovered conversation, and one
`conversation_digest` Artifact. See [`receipt.json`](receipt.json).

## Boundary

This is a controlled two-conversation functional receipt, not an independent
evaluation window. It does not support claims about overall task success,
model quality, latency, shared-environment reliability, active write safety,
MCP, Memory, or public availability. Test account, token, task, run, message,
and conversation identifiers remain in the protected Remote GPU workspace.
