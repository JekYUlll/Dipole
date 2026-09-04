# Multipart Restart Smoke Receipt

## Scope

This receipt records an isolated Remote GPU run from source revision
`43d86704e274eb9d71a6c5dc518cc3c7ad145660` using Go `1.27.0`.
`scripts/smoke-minio-multipart-restart.sh` started a randomly named MinIO
container with an isolated persistent volume, wrote the first 5 MiB part,
restarted that MinIO container, then uploaded the second part and completed
the object.

## Verified Behavior

- Multipart state survived the MinIO restart.
- The completed object bytes matched both uploaded parts.
- The smoke exited with status `0`.
- The public `dipole-experience` project remained at 12 running containers.
- No candidate `dipole-multipart-restart` containers remained after cleanup.

## Boundary

This result covers a disposable MinIO fixture only. It does not prove browser
disconnect recovery, pre-signed upload behavior, Redis failure recovery,
cross-store reconciliation, production object-store availability, or a change
to the default `relay` upload path.
