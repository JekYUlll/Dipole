#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
evidence_path="${DIPOLE_AGENT_CREDENTIAL_LIFECYCLE_EVIDENCE:-$root_dir/agent-runtime/.artifacts/external-mcp-credential-lifecycle.json}"

mkdir -p "$(dirname "$evidence_path")"
rm -f "$evidence_path"

cd "$root_dir/agent-runtime"
export DIPOLE_AGENT_CREDENTIAL_LIFECYCLE_DRILL=true
export DIPOLE_AGENT_CREDENTIAL_LIFECYCLE_EVIDENCE="$evidence_path"
npm test -- --run src/runtime/external-mcp-credential-lifecycle-drill.integration.test.ts

test -s "$evidence_path"
test "$(stat -c '%a' "$evidence_path")" = "600"
npm run mcp:credential-drill:check -- --evidence="$evidence_path"
