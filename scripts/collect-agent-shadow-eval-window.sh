#!/usr/bin/env bash
set -euo pipefail

# This collector only evaluates reviewer-supplied manifests against a running
# read-shadow Agent. It never creates tasks, changes runtime flags, or labels evidence.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root_dir="$(cd "${script_dir}/.." && pwd)"
project_name="${COMPOSE_PROJECT_NAME:?COMPOSE_PROJECT_NAME is required}"
manifest_dir="${DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_DIR:?DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_DIR is required}"
manifest_set_sha256="${DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_SET_SHA256:?DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_SET_SHA256 is required}"
output_dir="${DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR:?DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR is required}"
minimum_manifest_count="${DIPOLE_AGENT_SHADOW_EVAL_MIN_MANIFESTS:-1}"
compose_env_file="${COMPOSE_ENV_FILE:-}"
compose_overlays="${COMPOSE_OVERLAYS:-}"
compose_file="${root_dir}/deploy/compose/docker-compose.microservices.yml"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

[[ -d "${manifest_dir}" ]] || { echo "reviewed manifest directory is missing" >&2; exit 2; }
[[ ! -e "${output_dir}" ]] || { echo "Shadow Eval window output already exists" >&2; exit 2; }
[[ "${manifest_set_sha256}" =~ ^[a-f0-9]{64}$ ]] || { echo "Shadow Eval manifest set SHA-256 is invalid" >&2; exit 2; }
[[ "${minimum_manifest_count}" =~ ^[1-9][0-9]*$ ]] && (( minimum_manifest_count <= 1000 )) || {
  echo "Shadow Eval minimum manifest count must be an integer between 1 and 1000" >&2
  exit 2
}

compose_args=(-p "${project_name}" -f "${compose_file}")
if [[ -n "${compose_env_file}" ]]; then
  compose_args=(--env-file "${compose_env_file}" "${compose_args[@]}")
fi
if [[ -n "${compose_overlays}" ]]; then
  IFS=':' read -r -a overlays <<<"${compose_overlays}"
  for overlay in "${overlays[@]}"; do
    [[ -n "${overlay}" ]] || { echo "COMPOSE_OVERLAYS contains an empty path" >&2; exit 2; }
    compose_args+=(-f "${root_dir}/${overlay}")
  done
fi

compose() {
  docker compose "${compose_args[@]}" "$@"
}

# Bind the evidence window to the image that executed the evaluation.
agent_container="$(compose ps -q agent)"
[[ -n "${agent_container}" ]] || { echo "Agent container is not running" >&2; exit 2; }
runtime_revision="$(docker inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${agent_container}" 2>/dev/null || true)"
runtime_dirty="$(docker inspect --format '{{index .Config.Labels "io.dipole.source.dirty"}}' "${agent_container}" 2>/dev/null || true)"
[[ "${runtime_revision}" =~ ^[a-f0-9]{40}$ && "${runtime_dirty}" == "false" ]] || {
  echo "Agent image provenance is missing or dirty" >&2
  exit 2
}

mapfile -d '' manifests < <(find "${manifest_dir}" -maxdepth 1 -type f -name '*.json' -print0 | sort -z)
(( ${#manifests[@]} > 0 )) || { echo "reviewed manifest directory has no JSON files" >&2; exit 2; }
(( ${#manifests[@]} >= minimum_manifest_count )) || {
  echo "reviewed Shadow Eval manifest count is below required minimum" >&2
  exit 2
}
actual_manifest_set_sha256="$("${script_dir}/hash-agent-shadow-eval-manifest-set.sh" "${manifest_dir}")"
[[ "${actual_manifest_set_sha256}" == "${manifest_set_sha256}" ]] || {
  echo "Shadow Eval manifest set SHA-256 does not match reviewed input" >&2
  exit 2
}

mkdir -p "${output_dir}/reports"
index=0
candidate_version=""
for manifest in "${manifests[@]}"; do
  jq -e '
    .schemaVersion == "dipole.agent.shadow-eval-manifest.v1" and
    (.candidateVersion | type == "string") and
    (.taskId | type == "string") and
    (.runId | type == "string")
  ' "${manifest}" >/dev/null || { echo "invalid reviewed Shadow Eval manifest: ${manifest}" >&2; exit 2; }
  manifest_candidate_version="$(jq -r '.candidateVersion' "${manifest}")"
  if [[ -z "${candidate_version}" ]]; then
    candidate_version="${manifest_candidate_version}"
  elif [[ "${candidate_version}" != "${manifest_candidate_version}" ]]; then
    echo "reviewed Shadow Eval manifests must share one candidate version" >&2
    exit 2
  fi

  printf -v ordinal '%03d' "$((index + 1))"
  container_manifest="/tmp/dipole-shadow-eval-manifest-${ordinal}.json"
  report_path="${output_dir}/reports/report-${ordinal}.json"
  compose cp "${manifest}" "agent:${container_manifest}" >/dev/null

  set +e
  compose exec -T agent node dist/evals/shadow-eval-cli.js "--manifest=${container_manifest}" >"${report_path}"
  eval_status=$?
  set -e
  if [[ "${eval_status}" != "0" && "${eval_status}" != "2" ]]; then
    echo "Shadow Eval failed closed for manifest ${manifest}" >&2
    exit "${eval_status}"
  fi
  jq -e '.schemaVersion == "dipole.agent.shadow-eval-report.v1"' "${report_path}" >/dev/null
  index=$((index + 1))
done

finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
jq -n \
  --arg sha256 "${actual_manifest_set_sha256}" \
  --arg candidate_version "${candidate_version}" \
  --argjson manifest_count "${#manifests[@]}" \
  --argjson minimum_manifest_count "${minimum_manifest_count}" \
  '{schemaVersion: "dipole.agent.shadow-eval-manifest-set-receipt.v1", manifestSetSha256: $sha256, candidateVersion: $candidate_version, manifestCount: $manifest_count, minimumManifestCount: $minimum_manifest_count}' \
  >"${output_dir}/manifest-set.json"
jq -s \
  --arg revision "${runtime_revision}" \
  --arg started_at "${started_at}" \
  --arg finished_at "${finished_at}" \
  '{schemaVersion: "dipole.agent.shadow-eval-summary-input.v1", source: {kind: "reviewed_shadow", environment: "isolated", runtimeRevision: $revision, windowStart: $started_at, windowEnd: $finished_at}, reports: .}' \
  "${output_dir}/reports"/*.json >"${output_dir}/summary-input.json"

compose cp "${output_dir}/summary-input.json" agent:/tmp/dipole-shadow-eval-summary-input.json >/dev/null
set +e
compose exec -T agent node dist/evals/shadow-eval-summary-cli.js --input=/tmp/dipole-shadow-eval-summary-input.json >"${output_dir}/summary.json"
summary_status=$?
set -e
if [[ "${summary_status}" != "0" && "${summary_status}" != "2" ]]; then
  echo "Shadow Eval summary failed closed" >&2
  exit "${summary_status}"
fi
jq -e '.schemaVersion == "dipole.agent.shadow-eval-summary-report.v1"' "${output_dir}/summary.json" >/dev/null

printf 'Agent Shadow Eval window collected: reports=%s output=%s status=%s\n' "${index}" "${output_dir}" "${summary_status}"
exit "${summary_status}"
