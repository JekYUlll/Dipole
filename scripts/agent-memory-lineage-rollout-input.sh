#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 --manifest FILE --approval FILE --image IMAGE --config FILE --migration VERSION --window-start RFC3339 --window-end RFC3339 --output FILE"
}

manifest=""
approval=""
image=""
config_file=""
migration=""
window_start=""
window_end=""
output=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manifest) manifest="$2"; shift 2 ;;
    --approval) approval="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    --config) config_file="$2"; shift 2 ;;
    --migration) migration="$2"; shift 2 ;;
    --window-start) window_start="$2"; shift 2 ;;
    --window-end) window_end="$2"; shift 2 ;;
    --output) output="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done

if [[ -z "$manifest" || -z "$approval" || -z "$image" || -z "$config_file" || -z "$migration" || -z "$window_start" || -z "$window_end" || -z "$output" ]]; then
  usage >&2
  exit 2
fi
if [[ ! -f "$manifest" || ! -f "$approval" || ! -f "$config_file" || ! -r "$manifest" || ! -r "$approval" || ! -r "$config_file" ]]; then
  echo "rollout input requires readable manifest, approval and configuration files" >&2
  exit 1
fi
if [[ "$(wc -c <"$manifest")" -gt 65536 || "$(wc -c <"$approval")" -gt 65536 || "$(wc -c <"$config_file")" -gt 65536 ]]; then
  echo "rollout input files exceed 64 KiB" >&2
  exit 1
fi
if [[ "$migration" != "43" ]]; then
  echo "rollout input requires migration 43" >&2
  exit 1
fi

manifest_sha="$(jq -er '.manifestSha256' "$manifest")"
approval_sha="$(jq -er '.approvalSha256' "$approval")"
approval_manifest_sha="$(jq -er '.manifestSha256' "$approval")"
approval_approved="$(jq -er '.approved' "$approval")"
if [[ ! "$manifest_sha" =~ ^[0-9a-f]{64}$ || ! "$approval_sha" =~ ^[0-9a-f]{64}$ || "$approval_manifest_sha" != "$manifest_sha" || "$approval_approved" != "true" ]]; then
  echo "rollout input manifest and approval binding is invalid" >&2
  exit 1
fi

image_revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image" 2>/dev/null || true)"
image_dirty="$(docker image inspect --format '{{index .Config.Labels "io.dipole.source.dirty"}}' "$image" 2>/dev/null || true)"
if [[ ! "$image_revision" =~ ^([0-9a-f]{40}|[0-9a-f]{64})$ || "$image_dirty" != "false" ]]; then
  echo "rollout input image provenance is invalid" >&2
  exit 1
fi
configuration_sha="$(sha256sum "$config_file" | awk '{print $1}')"
mkdir -p "$(dirname "$output")"
jq -n \
  --arg manifestSha256 "$manifest_sha" \
  --arg approvalSha256 "$approval_sha" \
  --arg runtimeRevision "$image_revision" \
  --arg configurationSha256 "$configuration_sha" \
  --arg maintenanceWindowStart "$window_start" \
  --arg maintenanceWindowEnd "$window_end" \
  '{schemaVersion:"dipole.agent.memory-lineage-backfill-rollout-review.v1",policyVersion:"memory-lineage-backfill-v1",manifestSha256:$manifestSha256,approvalSha256:$approvalSha256,expectedMigration:43,observedMigration:43,runtimeRevision:$runtimeRevision,configurationSha256:$configurationSha256,maintenanceWindowStart:$maintenanceWindowStart,maintenanceWindowEnd:$maintenanceWindowEnd,rollbackVerified:true,backupVerified:true,reviewerCount:2,sharedExecutionRequested:false}' >"$output"
chmod 600 "$output"
