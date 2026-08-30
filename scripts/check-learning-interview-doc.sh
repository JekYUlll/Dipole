#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

index_document="docs/guides/PROJECT-LEARNING-AND-INTERVIEW.md"
default_documents=(
  "docs/guides/DIPOLE-IM-LEARNING-AND-INTERVIEW.md"
  "docs/guides/DIPOLE-AGENT-LEARNING-AND-INTERVIEW.md"
)
if [[ -n "${DIPOLE_LEARNING_INTERVIEW_DOCUMENT:-}" ]]; then
  documents=("${DIPOLE_LEARNING_INTERVIEW_DOCUMENT}")
else
  documents=("${default_documents[@]}")
  if [[ ! -f "${index_document}" ]] || ! git ls-files --error-unmatch "${index_document}" >/dev/null 2>&1; then
    echo "learning and interview index is missing or untracked: ${index_document}" >&2
    exit 1
  fi
  for document in "${default_documents[@]}"; do
    if ! rg --fixed-strings --quiet "$(basename "${document}")" "${index_document}"; then
      echo "learning and interview index does not link: ${document}" >&2
      exit 1
    fi
  done
  for index in README.md docs/README.md; do
    if ! rg --fixed-strings --quiet "PROJECT-LEARNING-AND-INTERVIEW.md" "${index}"; then
      echo "learning and interview index is not linked from: ${index}" >&2
      exit 1
    fi
  done
fi

required_sections=(
  "## 1. 使用规则"
  "### 滚动维护契约"
  "### 能力卡片模板与索引"
  "## 2. 一句话定位"
  "## 3. 简历描述"
  "## 4. 现场介绍"
  "## 5. 可展开的工程故事"
)
required_card_fields=(
  "- **状态：**"
  "- **简历句：**"
  "- **对外表述：**"
  "- **演示：**"
  "- **证据：**"
  "- **追问：**"
  "- **限制：**"
  "- **下一步：**"
  "- **复核条件：**"
)
for document in "${documents[@]}"; do
  if [[ ! -f "${document}" ]] || ! git ls-files --error-unmatch "${document}" >/dev/null 2>&1; then
    echo "learning and interview document is missing or untracked: ${document}" >&2
    exit 1
  fi
  for section in "${required_sections[@]}"; do
    if ! rg --fixed-strings --quiet "${section}" "${document}"; then
      echo "learning and interview document is missing section: ${section}: ${document}" >&2
      exit 1
    fi
  done
  for field in "${required_card_fields[@]}"; do
    if ! rg --fixed-strings --quiet -- "${field}" "${document}"; then
      echo "learning and interview document is missing card field: ${field}: ${document}" >&2
      exit 1
    fi
  done
done

echo "learning and interview documentation gate passed"
