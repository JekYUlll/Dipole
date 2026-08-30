#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

document="docs/agent/agent-external-mcp.md"
required=(
  'external_mcp_shadow'
  'startExternalMcpProductionShadow'
  '基础 Compose 继续固定关闭'
  '共享 Shadow tenant'
)

for text in "${required[@]}"; do
  if ! rg --fixed-strings --quiet "${text}" "${document}"; then
    echo "External MCP documentation is missing required runtime boundary: ${text}" >&2
    exit 1
  fi
done

for stale in \
  '当前生产 `AgentTaskWorkerActivities` 未注册 `executeMcpDispatch`' \
  '生产 `index.ts` 尚未调用它' \
  '该 root 当前没有进入 `index.ts`/Compose'; do
  if rg --fixed-strings --quiet "${stale}" "${document}"; then
    echo "External MCP documentation contains stale runtime claim: ${stale}" >&2
    exit 1
  fi
done

echo "agent external MCP documentation gate passed"
