# Web Sync Observability Preflight Receipt

- Candidate revision: `2c81464e9f3631f39c2ae1e29247066b4c9b075b`
- Environment: isolated Remote GPU Compose project
- Scope: loopback-only Gateway `18080`, Prometheus `19090`, Alertmanager `19093`
- Result: passed

The preflight rebuilt `migrate`, `core`, `message`, `sync`, and `gateway` with
the candidate OCI revision before startup. Prometheus reported healthy required
targets for Core, Message, Sync, and Gateway, and loaded the five Web Sync
recording rules required by the observation protocol. The project removed its
containers and volumes after completion; the public `dipole-experience` stack
remained healthy.

This receipt does not include a browser Sync shadow session, client comparison
traffic, a candidate Web bundle, a 24-hour observation window, or promotion
authority.
