#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
expected_services=(core gateway message sync search search-indexer)
if [[ ! -f "${root_dir}/cmd/services/README.md" ]]; then
  echo "service entrypoint index is missing: cmd/services/README.md" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/docs/architecture/SERVICE-BOUNDARIES.md" ]]; then
  echo "service boundary manifest is missing: docs/architecture/SERVICE-BOUNDARIES.md" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/services/README.md" ]]; then
  echo "polyglot service directory index is missing: services/README.md" >&2
  exit 1
fi
if ! git -C "${root_dir}" ls-files --error-unmatch docs/architecture/SERVICE-BOUNDARIES.md >/dev/null 2>&1; then
  echo "service boundary manifest is not tracked: docs/architecture/SERVICE-BOUNDARIES.md" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search/application/search.go" ]]; then
  echo "Search application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/application/application.go" ]]; then
  echo "Sync application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/application/application.go" ]]; then
  echo "Message application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/application.go" ]]; then
  echo "Core capability implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/conversation.go" ]]; then
  echo "Core conversation application is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/user.go" ]]; then
  echo "Core user application is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/contact.go" ]]; then
  echo "Core contact application is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/group.go" ]]; then
  echo "Core group application is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/file.go" ]]; then
  echo "Core file application is outside its service boundary" >&2
  exit 1
fi
for core_app in auth admin session; do
  if [[ ! -f "${root_dir}/internal/services/core/application/${core_app}.go" ]]; then
    echo "Core ${core_app} application is outside its service boundary" >&2
    exit 1
  fi
done
if [[ ! -f "${root_dir}/internal/services/core/domain/group/group_service.go" ]]; then
  echo "Core group domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/file/file_service.go" ]]; then
  echo "Core file domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/auth/auth_service.go" || ! -f "${root_dir}/internal/services/core/domain/auth/token_service.go" ]]; then
  echo "Core auth domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/admin/admin_service.go" ]]; then
  echo "Core admin domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/session/session_service.go" ]]; then
  echo "Core session domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/user/user_service.go" ]]; then
  echo "Core user domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/contact/contact_service.go" ]]; then
  echo "Core contact domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/domain/conversation/conversation_service.go" ]]; then
  echo "Core conversation domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/file_service.go" ]]; then
  echo "legacy Core file implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/auth_service.go" || -e "${root_dir}/internal/service/token_service.go" ]]; then
  echo "legacy Core auth implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/admin_service.go" ]]; then
  echo "legacy Core admin implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/session_service.go" ]]; then
  echo "legacy Core session implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/user_service.go" ]]; then
  echo "legacy Core user implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/contact_service.go" ]]; then
  echo "legacy Core contact implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/conversation_service.go" ]]; then
  echo "legacy Core conversation implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/service/group_service.go" ]]; then
  echo "legacy Core group implementation remains under internal/service" >&2
  exit 1
fi
if ! rg --quiet '^type CoreProcessRepositories struct' "${root_dir}/internal/app/repositories.go"; then
  echo "Core process repository composition is missing" >&2
  exit 1
fi
if ! rg --quiet '^type AgentProcessRepositories struct' "${root_dir}/internal/app/repositories.go"; then
  echo "Agent process repository composition is missing" >&2
  exit 1
fi
if rg --quiet '^type LocalCoreCapability struct' "${root_dir}/internal/app"; then
  echo "legacy shared Core capability implementation remains under internal/app" >&2
  exit 1
fi
if rg --quiet '^type LocalMessageApplication struct' "${root_dir}/internal/app"; then
  echo "legacy shared Message application implementation remains under internal/app" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/app/sync.go" || -e "${root_dir}/internal/app/sync_test.go" ]]; then
  echo "legacy shared Sync application path remains under internal/app" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/app/search.go" || -e "${root_dir}/internal/app/search_test.go" ]]; then
  echo "legacy shared Search application path remains under internal/app" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/gateway/search_handler.go" ]]; then
  echo "Gateway Search HTTP handler is outside the Gateway boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/services/agent-runtime/package.json" || ! -f "${root_dir}/services/agent-runtime/src/index.ts" ]]; then
  echo "Agent Runtime is outside the services boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/agent/legacy/service.go" ]]; then
  echo "Go/Eino compatibility baseline is outside the Agent service boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/modules/ai" ]]; then
  echo "legacy Agent module remains under internal/modules/ai" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/services/realtime-delivery/CMakeLists.txt" || ! -f "${root_dir}/services/realtime-delivery/src/main.cpp" ]]; then
  echo "Realtime Delivery is outside the services boundary" >&2
  exit 1
fi
if git -C "${root_dir}" ls-files --error-unmatch agent-runtime >/dev/null 2>&1 || \
  git -C "${root_dir}" ls-files --error-unmatch realtime-delivery >/dev/null 2>&1; then
  echo "legacy polyglot service directory remains at repository root" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/handler/http" ]]; then
  echo "legacy shared HTTP handler path remains under internal/handler/http" >&2
  exit 1
fi
for service in "${expected_services[@]}"; do
  if [[ ! -f "${root_dir}/cmd/services/${service}/main.go" ]]; then
    echo "missing service entrypoint: cmd/services/${service}/main.go" >&2
    exit 1
  fi
done

for legacy in server gateway message-service sync-service search-service search-indexer; do
  if [[ -e "${root_dir}/cmd/${legacy}" ]]; then
    echo "legacy service entrypoint remains at cmd/${legacy}" >&2
    exit 1
  fi
done

if [[ -n "$(find "${root_dir}/cmd" -mindepth 1 -maxdepth 1 -type d ! -name services ! -name tools -print -quit)" ]]; then
  echo "unclassified command directory remains directly under cmd/" >&2
  exit 1
fi

echo "service command layout: ok"
