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

if ! git check-ignore --quiet docs/architecture-reference.md; then
  echo "local architecture reference must remain explicitly ignored" >&2
  exit 1
fi

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

echo "architecture documentation gate passed"
