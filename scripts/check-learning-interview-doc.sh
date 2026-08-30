#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

default_document="docs/guides/PROJECT-LEARNING-AND-INTERVIEW.md"
document="${DIPOLE_LEARNING_INTERVIEW_DOCUMENT:-${default_document}}"
if [[ ! -f "${document}" ]]; then
  echo "learning and interview document is missing: ${document}" >&2
  exit 1
fi

if [[ "${document}" == "${default_document}" ]]; then
  if ! git ls-files --error-unmatch "${document}" >/dev/null 2>&1; then
    echo "learning and interview document is not tracked: ${document}" >&2
    exit 1
  fi

  for index in README.md docs/README.md; do
    if ! rg --fixed-strings --quiet "PROJECT-LEARNING-AND-INTERVIEW.md" "${index}"; then
      echo "learning and interview document is not linked from: ${index}" >&2
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
for section in "${required_sections[@]}"; do
  if ! rg --fixed-strings --quiet "${section}" "${document}"; then
    echo "learning and interview document is missing section: ${section}" >&2
    exit 1
  fi
done

if ! rg --fixed-strings --quiet "[面试问答](INTERVIEW-QA.md)" "${document}"; then
  echo "learning and interview document is missing the interview Q&A entry" >&2
  exit 1
fi

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
for field in "${required_card_fields[@]}"; do
  if ! rg --fixed-strings --quiet -- "${field}" "${document}"; then
    echo "learning and interview document is missing card field: ${field}" >&2
    exit 1
  fi
done

echo "learning and interview documentation gate passed"
