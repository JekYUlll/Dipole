# Message RPC v1

`dipole.message.v1.MessageService` is the first remote-compatible boundary for the existing `application.MessageApplication`.

## Trust and deadlines

- The service is internal. Only an authenticated Gateway or Core process may create `dipole.common.v1.RequestContext`.
- Production transport must use service authentication before accepting `principal_user_id`; the RPC server is not a public client endpoint.
- Callers set a deadline on every request. The initial budget is 3 seconds for commands and 2 seconds for history queries.
- A command deadline does not imply rollback after persistence starts. Retries use the same `client_message_id`.

## Idempotency and pagination

- Command idempotency is scoped by `(principal_user_id, client_message_id)`.
- Existing clients may omit `client_message_id` during the compatibility window; retry-capable clients must provide it.
- `page_size=0` uses the existing application default, and the application caps oversized pages. Negative values return `INVALID_ARGUMENT`.
- Direct history uses `before_id`. Group history uses the `before_id`/`after_id` oneof so both directions cannot be supplied together.
- Offline history keeps the legacy `after_id` cursor until Sync Query becomes the primary device protocol.

## Error mapping

| gRPC code | Meaning |
| --- | --- |
| `UNAUTHENTICATED` | Missing trusted principal |
| `INVALID_ARGUMENT` | Missing/invalid target, content, file, page size, or cursor |
| `NOT_FOUND` | Target user or group does not exist |
| `PERMISSION_DENIED` | Friendship or group membership is insufficient |
| `ALREADY_EXISTS` | Idempotency key conflicts with another target |
| `FAILED_PRECONDITION` | Target or file exists but is unavailable |
| `INTERNAL` | Unclassified application or infrastructure failure |

Known domain failures also carry `ErrorDetail.reason`. Clients map this structured reason back to their local domain error and do not parse status text.

## Compatibility

- Published field numbers and RPC names are immutable within v1.
- New fields must be optional for old consumers. Removed fields stay reserved in the proto before reuse is considered.
- Breaking request, response, authorization, or cursor semantics require a new protobuf package version.
