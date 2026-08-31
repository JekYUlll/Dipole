#!/usr/bin/env bash
set -euo pipefail

# This collector only evaluates reviewer-supplied manifests against a running
# read-shadow Agent. It never creates tasks, changes runtime flags, or labels evidence.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root_dir="$(cd "${script_dir}/.." && pwd)"
project_name="${COMPOSE_PROJECT_NAME:?COMPOSE_PROJECT_NAME is required}"
manifest_dir="${DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_DIR:?DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_DIR is required}"
output_dir="${DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR:?DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR is required}"
compose_env_file="${COMPOSE_ENV_FILE:-}"
compose_overlays="${COMPOSE_OVERLAYS:-}"
compose_file="${root_dir}/deploy/compose/docker-compose.microservices.yml"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
runtime_revision="$(git -C "${root_dir}" rev-parse HEAD)"

[[ -d "${manifest_dir}" ]] || { echo "reviewed manifest directory is missing" >&2; exit 2; }
[[ ! -e "${output_dir}" ]] || { echo "Shadow Eval window output already exists" >&2; exit 2; }

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

mapfile -d '' manifests < <(find "${manifest_dir}" -maxdepth 1 -type f -name '*.json' -print0 | sort -z)
(( ${#manifests[@]} > 0 )) || { echo "reviewed manifest directory has no JSON files" >&2; exit 2; }

mkdir -p "${output_dir}/reports"
index=0
for manifest in "${manifests[@]}"; do
  jq -e '
    .schemaVersion == "dipole.agent.shadow-eval-manifest.v1" and
    (.candidateVersion | type == "string") and
    (.taskId | type == "string") and
    (.runId | type == "string")
  ' "${manifest}" >/dev/null || { echo "invalid reviewed Shadow Eval manifest: ${manifest}" >&2; exit 2; }

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
