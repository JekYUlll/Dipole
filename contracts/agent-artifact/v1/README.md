# Agent Artifact v1

`dipole.agent.artifact.v1` defines an immutable Task output. MySQL stores the
binding and content evidence; MinIO stores the body under a content-addressed
object key.

Creation is limited to the authenticated `dipole-agent` runtime and an exact
Task/Run binding. User retrieval is limited to the Task principal. Exact
creation replay returns the original Artifact; a version or content conflict
fails closed. v1 exposes no update, delete, public URL, message-send, or active
runtime authority.

The body is capped at 1 MiB and metadata at 16 KiB. Consumers must verify
`content_sha256` and `size_bytes` after reading the object.
