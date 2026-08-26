# Core Capability RPC v1

`CoreCapabilityService` exposes the minimum User, Contact, Group, and GroupMember snapshots required by Message Service.

- Metadata lookups require an authenticated `caller_service` and may omit an end-user principal.
- Authorization requests carry the acting user in both the request fields and audit context when available.
- A missing entity is represented by an empty snapshot so the existing `CoreCapability` semantics remain stable.
- Database and infrastructure details are returned as `INTERNAL`; storage errors do not cross the service boundary.
- Production traffic requires AD-013 service authentication and interceptor validation.
