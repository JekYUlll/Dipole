# Search RPC v1

`SearchService` exposes principal-scoped message search to trusted internal callers.

- The request carries identity only in authenticated `RequestContext.principal_user_id` and has no user ID or conversation-key field.
- Search Service resolves authorization scope from Core for every query; an empty scope returns an empty page without querying Elasticsearch.
- `page_size=0` uses the storage default, positive values are capped at 100, and negative values are rejected.
- Search results are projection snapshots. Message UUID and conversation Seq preserve message identity and timeline position.
- Infrastructure errors cross the boundary as bounded `INTERNAL` statuses without Elasticsearch response bodies.
- The first deployment allowlist contains `dipole-gateway`; no public HTTP route is added in this milestone.
