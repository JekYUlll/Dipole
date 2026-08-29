#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

manifest="docs/architecture-docs.manifest"
if [[ ! -f "${manifest}" ]]; then
  echo "architecture document manifest is missing: ${manifest}" >&2
  exit 1
fi

while IFS= read -r document; do
  [[ -z "${document}" || "${document}" == \#* ]] && continue
  if [[ ! -f "${document}" ]]; then
    echo "required architecture document is missing: ${document}" >&2
    exit 1
  fi
  if ! git ls-files --error-unmatch "${document}" >/dev/null 2>&1; then
    echo "required architecture document is not tracked: ${document}" >&2
    exit 1
  fi
done <"${manifest}"

if rg --quiet '^docs/\*\.md$' .gitignore; then
  echo "blanket docs/*.md ignore rule hides architecture documents" >&2
  exit 1
fi

allowed_docs_root=(README.md architecture-docs.manifest technical-architecture.svg .gitkeep)
while IFS= read -r document; do
  case " ${allowed_docs_root[*]} " in
    *" ${document} "*) ;;
    *)
      echo "docs root file must be organized under a category: ${document}" >&2
      exit 1
      ;;
  esac
done < <(find docs -maxdepth 1 -type f -printf '%f\n' | sort)

legacy_docs=(
  docs/agent-artifact-reconcile.md
  docs/agent-external-mcp.md
  docs/agent-mcp-authorization.md
  docs/agent-memory-observation.md
  docs/agent-otel-operations.md
  docs/agent-subscription-shadow.md
  docs/AGENT-TIMELINE-REPAIR-OPERATIONS.md
  docs/architecture-reference.md
  docs/cache-strategy.md
  docs/development-roadmap.md
  docs/interview-qa.md
  docs/load-test-report.md
  docs/message-storage-and-sync-model.md
  docs/message-sync-strategy.md
  docs/tls-setup.md
  docs/TODO.md
)
for document in "${legacy_docs[@]}"; do
  if [[ -e "${document}" ]]; then
    echo "legacy documentation path remains: ${document}" >&2
    exit 1
  fi
done

allowed_root_docs=(README.md CHANGELOG.md AGENTS.md CLAUDE.md)
while IFS= read -r document; do
  case " ${allowed_root_docs[*]} " in
    *" ${document} "*) ;;
    *)
      echo "root markdown document must be organized under docs/: ${document}" >&2
      exit 1
      ;;
  esac
done < <(find . -maxdepth 1 -type f -name '*.md' -printf '%f\n' | sort)

allowed_root_dirs=(api benchmarks cmd configs contracts db deploy design docs frontend internal scripts services)
while IFS= read -r directory; do
  case " ${allowed_root_dirs[*]} " in
    *" ${directory} "*) ;;
    *)
      echo "root source directory must be classified in repository structure: ${directory}" >&2
      exit 1
      ;;
  esac
done < <(git ls-files | awk -F/ 'NF > 1 {print $1}' | sort -u)

echo "architecture documentation gate passed"
