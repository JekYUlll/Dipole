#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_dir="$(mktemp -d)"
mkdir -p "${fixture_dir}/bin" "${fixture_dir}/manifests"

cat >"${fixture_dir}/manifests/task-a.json" <<'JSON'
{"schemaVersion":"dipole.agent.shadow-eval-manifest.v1","candidateVersion":"agent-runtime@test","taskId":"task-a","runId":"run-a"}
JSON

cat >"${fixture_dir}/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "inspect" ]]; then
  if [[ "$3" == *"revision"* ]]; then
    printf '%s\n' "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  else
    printf '%s\n' "${DIPOLE_TEST_AGENT_IMAGE_DIRTY:-false}"
  fi
  exit 0
fi

if [[ "$1" == "compose" ]]; then
  for argument in "$@"; do
    if [[ "${argument}" == "ps" ]]; then
      printf '%s\n' "fake-agent-container"
      exit 0
    fi
    if [[ "${argument}" == "exec" ]]; then
      if [[ "$*" == *"shadow-eval-summary-cli.js"* ]]; then
        printf '%s\n' '{"schemaVersion":"dipole.agent.shadow-eval-summary-report.v1","decision":"eligible"}'
      else
        printf '%s\n' '{"schemaVersion":"dipole.agent.shadow-eval-report.v1","decision":"eligible"}'
      fi
      if [[ "${DIPOLE_TEST_SHADOW_EVAL_FAILURE:-false}" == "true" ]]; then
        exit 2
      fi
      exit 0
    fi
  done
  exit 0
fi

exit 99
DOCKER
chmod +x "${fixture_dir}/bin/docker"

export PATH="${fixture_dir}/bin:${PATH}"
export COMPOSE_PROJECT_NAME="agent-shadow-eval-fixture"
export DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_DIR="${fixture_dir}/manifests"
export DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_SET_SHA256="$("${root_dir}/scripts/hash-agent-shadow-eval-manifest-set.sh" "${fixture_dir}/manifests")"
export DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR="${fixture_dir}/window"
export DIPOLE_AGENT_SHADOW_EVAL_MIN_MANIFESTS=1
"${root_dir}/scripts/collect-agent-shadow-eval-window.sh" >/dev/null
jq -e '.source.runtimeRevision == "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' \
  "${fixture_dir}/window/summary-input.json" >/dev/null
jq -e --arg sha256 "${DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_SET_SHA256}" \
  '.schemaVersion == "dipole.agent.shadow-eval-manifest-set-receipt.v2" and .manifestSetSha256 == $sha256 and .manifestCount == 1 and .minimumManifestCount == 1' \
  "${fixture_dir}/window/manifest-set.json" >/dev/null

export DIPOLE_AGENT_SHADOW_EVAL_MIN_MANIFESTS=2
export DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR="${fixture_dir}/minimum-window"
set +e
"${root_dir}/scripts/collect-agent-shadow-eval-window.sh" >/dev/null 2>"${fixture_dir}/minimum.err"
status=$?
set -e
[[ "${status}" == "2" ]]
grep -qx 'reviewed Shadow Eval manifest count is below required minimum' "${fixture_dir}/minimum.err"
[[ ! -e "${fixture_dir}/minimum-window" ]]

export DIPOLE_AGENT_SHADOW_EVAL_MIN_MANIFESTS=1

export DIPOLE_TEST_SHADOW_EVAL_FAILURE=true
export DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR="${fixture_dir}/failed-window"
set +e
"${root_dir}/scripts/collect-agent-shadow-eval-window.sh" >/dev/null
status=$?
set -e
[[ "${status}" == "2" ]]
jq -e '.schemaVersion == "dipole.agent.shadow-eval-manifest-set-receipt.v2"' \
  "${fixture_dir}/failed-window/manifest-set.json" >/dev/null
unset DIPOLE_TEST_SHADOW_EVAL_FAILURE

export DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_SET_SHA256="$(printf 'b%.0s' {1..64})"
export DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR="${fixture_dir}/drift-window"
set +e
"${root_dir}/scripts/collect-agent-shadow-eval-window.sh" >/dev/null 2>"${fixture_dir}/drift.err"
status=$?
set -e
[[ "${status}" == "2" ]]
grep -qx 'Shadow Eval manifest set SHA-256 does not match reviewed input' "${fixture_dir}/drift.err"

export DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_SET_SHA256="$("${root_dir}/scripts/hash-agent-shadow-eval-manifest-set.sh" "${fixture_dir}/manifests")"

export DIPOLE_TEST_AGENT_IMAGE_DIRTY=true
export DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR="${fixture_dir}/dirty-window"
set +e
"${root_dir}/scripts/collect-agent-shadow-eval-window.sh" >/dev/null 2>"${fixture_dir}/dirty.err"
status=$?
set -e
[[ "${status}" == "2" ]]
grep -qx 'Agent image provenance is missing or dirty' "${fixture_dir}/dirty.err"

printf '%s\n' 'agent shadow eval window fixture passed'
