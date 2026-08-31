# Agent Interactive Control Receipt: 2026-09-01

## Scope

This development receipt verifies the authenticated, read-only interactive Agent
Task path on an isolated Remote GPU Compose project.

| Check | Result |
| --- | --- |
| Register authenticated user | HTTP `200` |
| Create interactive task | HTTP `202` |
| Read task state after admission | HTTP `200` |
| Durable Workflow state | `completed` |
| Agent image revision | `c9f3f424` |
| Runtime mode | `shadow`, read-only |
| Provider route | `deepseek/deepseek-v4-flash` with thinking disabled |

The request asked for an unread-conversation summary and did not grant message
write, MCP, external MCP, or active authority. No tokens, user identifiers,
message bodies, prompts, or model output are retained in this receipt.

## Interpretation

The check verifies that Gateway authentication, trusted control forwarding,
Kafka admission, Core ownership authorization, Temporal execution, and the
terminal task read path can converge in one isolated environment. The first
read may precede asynchronous admission; callers should poll until a durable
Task projection is available.

Gateway and Core remained at the previous clean candidate while only the Agent
image was rebuilt for `c9f3f424`. This is a focused regression receipt for the
terminal-read repair, not a same-revision release receipt, production evidence,
an active-authority promotion, or a task-success-rate claim.
