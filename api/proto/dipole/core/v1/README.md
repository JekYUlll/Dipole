# Core Capability RPC v1

`CoreCapabilityService` exposes the minimum User, Contact, Group, and GroupMember snapshots required by Message Service.

- Metadata lookups require an authenticated `caller_service` and may omit an end-user principal.
- Resource-specific authorization requests carry the acting user in both request fields and audit context when the resource contract requires it.
- Search scope requests carry the acting user only as the authenticated `RequestContext.principal`; callers cannot submit a user ID or conversation-key scope.
- Search scope contains direct Conversation projections plus normal or dismissed groups where the principal still has a membership row. A stale group Conversation projection grants no access.
- A missing entity is represented by an empty snapshot so the existing `CoreCapability` semantics remain stable.
- Database and infrastructure details are returned as `INTERNAL`; storage errors do not cross the service boundary.
- Production traffic requires AD-013 service authentication and interceptor validation.
