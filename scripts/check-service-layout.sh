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
if [[ ! -f "${root_dir}/internal/compat/README.md" || ! -d "${root_dir}/internal/compat/service" ]]; then
  echo "legacy compatibility adapters must be isolated under internal/compat" >&2
  exit 1
fi
for compat_file in admin_compat.go auth_compat.go contact_compat.go conversation_compat.go file_compat.go group_compat.go message_event_compat.go session_compat.go sync_compat.go token_compat.go user_compat.go; do
  if [[ ! -f "${root_dir}/internal/compat/service/${compat_file}" ]]; then
    echo "missing compatibility adapter: internal/compat/service/${compat_file}" >&2
    exit 1
  fi
done
if ! git -C "${root_dir}" ls-files --error-unmatch docs/architecture/SERVICE-BOUNDARIES.md >/dev/null 2>&1; then
  echo "service boundary manifest is not tracked: docs/architecture/SERVICE-BOUNDARIES.md" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search/application/search.go" ]]; then
  echo "Search application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search/infrastructure/mysql/search_index.go" ]]; then
  echo "Search index repository implementation is outside the Search service boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/data/mysql/repository/store.go" || -e "${root_dir}/internal/data/mysql/repository/uuid_helpers.go" ]]; then
  echo "unused shared MySQL repository support file remains after service extraction" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/data/mysql/repository/search_index.go" ]]; then
  echo "legacy Search index repository remains in shared repository package" >&2
  exit 1
fi
if ! rg --quiet 'services/search/infrastructure/mysql' "${root_dir}/internal/app/repositories.go"; then
  echo "Search composition must use Search-owned index repository" >&2
  exit 1
fi
if rg --quiet 'func (New|new)CoreProcessRepositories|type CoreProcessRepositories struct' "${root_dir}/internal/app" --glob '*.go' --glob '!core_repository_compat.go'; then
  echo "Core repository composition must live in the Core service infrastructure" >&2
  exit 1
fi
for legacy_core_file in cached_user_store.go cached_group_store.go cached_contact_store.go; do
  if [[ -e "${root_dir}/internal/app/${legacy_core_file}" ]]; then
    echo "Core cache adapter remains in aggregate app package: ${legacy_core_file}" >&2
    exit 1
  fi
done
if ! rg --quiet 'coremysql.NewProcessRepositories' "${root_dir}/internal/bootstrap/core_runtime.go"; then
  echo "standalone Core runtime must use Core-owned repository composition" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/application/application.go" ]]; then
  echo "Sync application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/domain/sync_service.go" ]]; then
  echo "Sync domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/infrastructure/mysql/sync_repository.go" ]]; then
  echo "Sync MySQL repository is outside its service boundary" >&2
  exit 1
fi
if rg --quiet 'type SyncProcessRepositories struct|func NewSyncProcessRepositories' "${root_dir}/internal/app" --glob '*.go' --glob '!sync_repository_compat.go'; then
  echo "Sync repository composition must live in the Sync service infrastructure" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/infrastructure/mysql/composition.go" ]]; then
  echo "Sync process repository composition is outside the Sync service boundary" >&2
  exit 1
fi
if rg --quiet 'internal/app(/|["`])' "${root_dir}/internal/bootstrap/sync_runtime.go"; then
  echo "standalone Sync runtime must not depend on aggregate internal/app composition" >&2
  exit 1
fi
if ! rg --quiet 'services/sync/infrastructure/mysql' "${root_dir}/internal/bootstrap/sync_runtime.go"; then
  echo "standalone Sync runtime must use Sync-owned repository composition" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/application/application.go" ]]; then
  echo "Message application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/domain/message_event.go" ]]; then
  echo "Message event domain implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/domain/sync_projection.go" ]]; then
  echo "Message Sync projection implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/infrastructure/mysql/message_repository.go" ]]; then
  echo "Message MySQL repository is outside its service boundary" >&2
  exit 1
fi
if rg --quiet 'internal/app(/|["`])' "${root_dir}/internal/bootstrap/message_runtime.go"; then
  echo "standalone Message runtime must not depend on aggregate internal/app composition" >&2
  exit 1
fi
if ! rg --quiet 'services/message/infrastructure/mysql' "${root_dir}/internal/bootstrap/message_runtime.go"; then
  echo "standalone Message runtime must use Message-owned repository composition" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/application.go" ]]; then
  echo "Core capability implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/bootstrap/core_runtime.go" ]]; then
  echo "standalone Core composition root is missing" >&2
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
if [[ ! -f "${root_dir}/internal/services/core/infrastructure/mysql/user.go" ]]; then
  echo "Core MySQL repository implementation is outside the Core service boundary" >&2
  exit 1
fi
for legacy_core_repository in admin.go contact.go conversation.go file.go group.go user.go; do
  if [[ -e "${root_dir}/internal/data/mysql/repository/${legacy_core_repository}" ]]; then
    echo "legacy Core MySQL implementation remains in shared repository package: ${legacy_core_repository}" >&2
    exit 1
  fi
done
if ! rg --quiet 'services/core/infrastructure/mysql' "${root_dir}/internal/app/repositories.go"; then
  echo "Core process composition must use Core-owned MySQL repositories" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/infrastructure/mysql/composition.go" ]]; then
  echo "Core process repository composition is outside the Core service boundary" >&2
  exit 1
fi
if rg --quiet 'github.com/JekYUlll/Dipole/internal/store' "${root_dir}/internal/services/core/domain" --glob '*.go' --glob '!*_test.go'; then
  echo "Core domain implementation must not depend directly on aggregate internal/store" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/file_service.go" ]]; then
  echo "legacy Core file implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/auth_service.go" || -e "${root_dir}/internal/compat/service/token_service.go" ]]; then
  echo "legacy Core auth implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/admin_service.go" ]]; then
  echo "legacy Core admin implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/session_service.go" ]]; then
  echo "legacy Core session implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/user_service.go" ]]; then
  echo "legacy Core user implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/contact_service.go" ]]; then
  echo "legacy Core contact implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/conversation_service.go" ]]; then
  echo "legacy Core conversation implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/sync_service.go" ]]; then
  echo "legacy Sync implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/message_event.go" || -e "${root_dir}/internal/compat/service/message_sync_projection.go" ]]; then
  echo "legacy Message event implementation remains under internal/service" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/data/mysql/repository/message.go" ]]; then
  echo "legacy Message MySQL repository remains in shared repository package" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/data/mysql/repository/sync.go" || -e "${root_dir}/internal/data/mysql/repository/sync_projection.go" || -e "${root_dir}/internal/data/mysql/repository/sync_hydrator.go" ]]; then
  echo "legacy Sync MySQL implementation remains in shared repository package" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/compat/service/group_service.go" ]]; then
  echo "legacy Core group implementation remains under internal/service" >&2
  exit 1
fi
if ! rg --quiet '^type ProcessRepositories struct' "${root_dir}/internal/services/core/infrastructure/mysql/composition.go"; then
  echo "Core process repository composition is missing" >&2
  exit 1
fi
if ! rg --quiet '^type ProcessRepositories struct' "${root_dir}/internal/services/agent/infrastructure/mysql/composition.go"; then
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
if [[ ! -f "${root_dir}/internal/services/agent/infrastructure/mysql/agent_policy.go" ]]; then
  echo "Agent MySQL repository implementation is outside the Agent service boundary" >&2
  exit 1
fi
if rg --quiet 'github.com/JekYUlll/Dipole/internal/app(/|["`])' "${root_dir}/internal/services/agent" --glob '*.go'; then
  echo "Agent service implementation or contract tests must not depend on aggregate internal/app" >&2
  exit 1
fi
for agent_application in agent_approval_grant.go agent_approval_service.go agent_task_control.go agent_definition_catalog.go agent_memory_candidate_promotion.go agent_task_workflow_projection.go agent_mcp_readiness_evidence.go agent_mcp_tool_round.go agent_tool_invocation_audit.go agent_runtime_promotion_evidence.go agent_workflow_repair_audit.go agent_artifact.go agent_memory_owner.go agent_subscription.go agent_command.go agent_capability.go agent_workflow_repair_execution.go agent_workflow_repair_executor.go agent_execution_policy.go agent_mcp_tool_terminal.go agent_memory.go agent_message_command_execution.go agent_runtime_promotion_control.go agent_runtime_promotion.go; do
  if [[ ! -f "${root_dir}/internal/services/agent/application/${agent_application}" ]]; then
    echo "Agent application implementation is outside the Agent service boundary: ${agent_application}" >&2
    exit 1
  fi
  if [[ -e "${root_dir}/internal/app/${agent_application}" ]]; then
    echo "legacy Agent application implementation remains under internal/app: ${agent_application}" >&2
    exit 1
  fi
done
if [[ ! -f "${root_dir}/internal/app/agent_application_compat.go" ]]; then
  echo "embedded Agent application compatibility boundary is missing" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/app/agent_repository_compat.go" ]]; then
  echo "embedded Agent repository compatibility boundary is missing" >&2
  exit 1
fi
for legacy_agent_repository in agent_policy.go agent_memory.go agent_artifact.go agent_tool_invocation.go agent_task_timeline.go agent_runtime_promotion_control.go agent_mcp_tool_round.go agent_mcp_readiness_evidence.go ai_call_log.go; do
  if [[ -e "${root_dir}/internal/data/mysql/repository/${legacy_agent_repository}" ]]; then
    echo "legacy Agent MySQL implementation remains in shared repository package: ${legacy_agent_repository}" >&2
    exit 1
  fi
done
if ! rg --quiet 'services/agent/infrastructure/mysql' "${root_dir}/internal/app/repositories.go"; then
  echo "Agent process composition must use Agent-owned MySQL repositories" >&2
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
