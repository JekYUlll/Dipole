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
if [[ ! -d "${root_dir}/api/gen/go" || ! -f "${root_dir}/api/proto/dipole/message/v1/message.proto" ]]; then
  echo "protobuf sources and generated Go contracts must remain under api" >&2
  exit 1
fi
if [[ -d "${root_dir}/internal/transport/grpc/gen" ]]; then
  echo "generated protobuf contracts remain under internal transport" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/operations/README.md" || ! -f "${root_dir}/internal/operations/search/README.md" || ! -f "${root_dir}/internal/operations/sync/README.md" || ! -f "${root_dir}/internal/operations/cassandra/README.md" || ! -f "${root_dir}/internal/operations/agent/README.md" ]]; then
  echo "one-shot operations must be documented under internal/operations" >&2
  exit 1
fi
for legacy_operations_dir in backfill baseline cleanup cutover reconcile evidence; do
  if [[ -d "${root_dir}/internal/${legacy_operations_dir}" ]]; then
    echo "legacy cross-service operations directory remains under internal/${legacy_operations_dir}; use internal/operations/<service>" >&2
    exit 1
  fi
done
for expected_operations_dir in \
  internal/operations/agent/evidence \
  internal/operations/agent/reconcile \
  internal/operations/cassandra/backfill \
  internal/operations/cassandra/evidence \
  internal/operations/cassandra/reconcile \
  internal/operations/search/backfill \
  internal/operations/search/cleanup \
  internal/operations/search/cutover \
  internal/operations/search/reconcile \
  internal/operations/sync/backfill \
  internal/operations/sync/baseline \
  internal/operations/sync/evidence \
  internal/operations/sync/reconcile; do
  if [[ ! -d "${root_dir}/${expected_operations_dir}" ]]; then
    echo "service-scoped operations directory is missing: ${expected_operations_dir}" >&2
    exit 1
  fi
done
for legacy_search_runtime in search_alias_runtime.go search_archive_runtime.go search_backfill_runtime.go search_cleanup_runtime.go search_reconciliation_runtime.go search_snapshot_source.go; do
  if [[ -e "${root_dir}/internal/bootstrap/${legacy_search_runtime}" ]]; then
    echo "Search one-shot operation remains in service bootstrap: ${legacy_search_runtime}" >&2
    exit 1
  fi
done
if [[ -e "${root_dir}/internal/bootstrap/memory_lineage_backfill_runtime.go" ]]; then
  echo "Agent one-shot operation remains in service bootstrap" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/tools/agent-memory-lineage-backfill" --glob '*.go'; then
  echo "Agent lineage operation tool must use internal/operations/agent" >&2
  exit 1
fi
for legacy_sync_runtime in sync_baseline_runtime.go sync_replay_runtime.go; do
  if [[ -e "${root_dir}/internal/bootstrap/${legacy_sync_runtime}" ]]; then
    echo "Sync one-shot operation remains in service bootstrap: ${legacy_sync_runtime}" >&2
    exit 1
  fi
done
for legacy_cassandra_runtime in cassandra_backfill_runtime.go cassandra_archive_runtime.go cassandra_reconciliation_runtime.go; do
  if [[ -e "${root_dir}/internal/bootstrap/${legacy_cassandra_runtime}" ]]; then
    echo "Cassandra one-shot operation remains in service bootstrap: ${legacy_cassandra_runtime}" >&2
    exit 1
  fi
done
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/tools/sync-baseline" "${root_dir}/cmd/tools/sync-replay" "${root_dir}/cmd/tools/sync-reconcile" "${root_dir}/cmd/tools/cassandra-backfill" "${root_dir}/cmd/tools/cassandra-archive" "${root_dir}/cmd/tools/cassandra-reconcile" --glob '*.go'; then
  echo "Sync/Cassandra operation tools must use internal/operations" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/tools/search-alias" "${root_dir}/cmd/tools/search-archive" "${root_dir}/cmd/tools/search-backfill" "${root_dir}/cmd/tools/search-outbox-cleanup" "${root_dir}/cmd/tools/search-reconcile" --glob '*.go'; then
  echo "Search operation tools must use internal/operations/search" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/compat/README.md" || ! -d "${root_dir}/internal/compat/service" ]]; then
  echo "legacy compatibility adapters must be isolated under internal/compat" >&2
  exit 1
fi
for compatibility_readme in \
  internal/app/README.md \
  internal/data/mysql/README.md \
  internal/data/mysql/repository/README.md \
  internal/store/README.md; do
  if [[ ! -f "${root_dir}/${compatibility_readme}" ]]; then
    echo "compatibility directory is missing its ownership guide: ${compatibility_readme}" >&2
    exit 1
  fi
done
 # Compatibility roots may retain adapters and tests, but must not become a
 # new shared implementation area as services are extracted.
for compatibility_file in \
  internal/store/mysql_compat.go \
  internal/store/redis_compat.go; do
  if [[ ! -f "${root_dir}/${compatibility_file}" ]]; then
    echo "required compatibility adapter is missing: ${compatibility_file}" >&2
    exit 1
  fi
done
while IFS= read -r compatibility_file; do
  case "${compatibility_file}" in
    internal/app/agent_application_compat.go|internal/app/README.md|internal/store/README.md|internal/store/mysql_compat.go|internal/store/redis_compat.go|internal/data/mysql/README.md|internal/data/mysql/store_compat.go|internal/data/mysql/repository/README.md|internal/data/mysql/repository/agent_compat.go|internal/data/mysql/repository/core_compat.go|internal/data/mysql/repository/message_compat.go|internal/data/mysql/repository/search_index_compat.go|internal/data/mysql/repository/sync_compat.go|internal/app/*_test.go) ;;
    *)
      echo "unexpected file under compatibility roots: ${compatibility_file}" >&2
      exit 1
      ;;
  esac
done < <(git -C "${root_dir}" ls-files --cached --others --exclude-standard -- internal/app internal/store internal/data/mysql | sort -u)
if [[ ! -f "${root_dir}/internal/platform/cache/redis.go" || ! -f "${root_dir}/internal/platform/cache/redis_cache.go" ]]; then
	echo "shared Redis client and cache helpers must remain under internal/platform/cache" >&2
	exit 1
fi
if [[ -e "${root_dir}/internal/store/redis.go" ]]; then
	echo "legacy Redis client implementation remains under internal/store" >&2
	exit 1
fi
if rg --quiet 'github.com/JekYUlll/Dipole/internal/store' "${root_dir}/internal" "${root_dir}/cmd" --glob '*.go' --glob '!internal/store/*'; then
	echo "runtime Redis callers must use internal/platform/cache" >&2
	exit 1
fi
if rg --quiet 'platformHotGroup\.NewRedisDetector\(' "${root_dir}/internal/bootstrap" "${root_dir}/internal/server" --glob '*.go'; then
	echo "Hot Group production composition must inject the platform Redis client" >&2
	exit 1
fi
if rg --quiet 'platformPresence\.NewRedisPresence\(' "${root_dir}/internal/bootstrap" "${root_dir}/internal/server" --glob '*.go'; then
	echo "Presence production composition must inject the platform Redis client" >&2
	exit 1
fi
if rg --quiet 'platformRateLimit\.NewLimiter\(' "${root_dir}/internal/bootstrap" "${root_dir}/internal/server" "${root_dir}/internal/gateway" --glob '*.go'; then
	echo "Rate limiter production composition must inject the platform Redis client" >&2
	exit 1
fi
if [[ ! -f "${root_dir}/internal/platform/cassandra/README.md" || ! -f "${root_dir}/internal/platform/cassandra/timeline.go" ]]; then
  echo "shared Cassandra adapters must remain under internal/platform/cassandra" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/platform/storage/README.md" || ! -f "${root_dir}/internal/platform/storage/routing/message_store.go" || ! -f "${root_dir}/internal/platform/storage/shadow/message_store.go" ]]; then
  echo "storage migration decorators must remain under internal/platform/storage" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/platform/elasticsearch/README.md" || ! -f "${root_dir}/internal/platform/elasticsearch/index.go" || ! -f "${root_dir}/internal/platform/elasticsearch/schema/message_search_v1.json" ]]; then
  echo "shared Elasticsearch adapter and schema must remain under internal/platform/elasticsearch" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/platform/mysql/store.go" || ! -f "${root_dir}/internal/platform/mysql/global.go" || ! -f "${root_dir}/internal/platform/mysql/README.md" ]]; then
  echo "shared SQLC MySQL transaction boundary must remain under internal/platform/mysql" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/store/mysql.go" ]]; then
	echo "legacy MySQL global connection remains under internal/store" >&2
	exit 1
fi
if rg --quiet 'store\.(InitMySQL|InitMySQLWithConfig|SQLDB)' "${root_dir}/internal/bootstrap" "${root_dir}/cmd/tools" "${root_dir}/internal/platform/bloom" --glob '*.go'; then
	echo "MySQL global connection callers must use internal/platform/mysql" >&2
	exit 1
fi
if [[ -e "${root_dir}/internal/data/mysql/store.go" ]]; then
  echo "MySQL transaction implementation remains in legacy internal/data/mysql" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/platform/mysql/generated/db.go" || ! -f "${root_dir}/internal/platform/mysql/mapper/message.go" ]]; then
  echo "SQLC generated code and mapper must remain under internal/platform/mysql" >&2
  exit 1
fi
if [[ -d "${root_dir}/internal/data/mysql/generated" || -d "${root_dir}/internal/data/mysql/mapper" ]]; then
  echo "legacy SQLC generated or mapper directory remains under internal/data/mysql" >&2
  exit 1
fi
if rg --quiet -i 'gorm|gorm\.io' "${root_dir}/internal" "${root_dir}/cmd" "${root_dir}/db" --glob '*.go' --glob '!**/*_test.go'; then
  echo "production Go code must use database/sql and sqlc; GORM references remain" >&2
  exit 1
fi
if rg --quiet -i 'gorm\.io' "${root_dir}/go.mod" "${root_dir}/go.sum"; then
  echo "GORM module dependencies remain after the sqlc migration" >&2
  exit 1
fi
if compgen -G "${root_dir}/internal/service/*.go" >/dev/null 2>&1; then
  echo "legacy internal/service implementation remains; use internal/services or internal/compat" >&2
  exit 1
fi
for legacy_mysql_operation in \
  agent_memory_lineage_backfill.go \
  cassandra_backfill.go \
  search_backfill.go \
  search_cleanup.go \
  sync_baseline.go \
  sync_replay.go; do
  if [[ -e "${root_dir}/internal/data/mysql/${legacy_mysql_operation}" ]]; then
    echo "MySQL operation adapter remains in legacy internal/data/mysql: ${legacy_mysql_operation}" >&2
    exit 1
  fi
done
if [[ -d "${root_dir}/internal/data/migration" || -d "${root_dir}/internal/data/mysqlconfig" ]]; then
  echo "MySQL platform support remains in legacy internal/data; use internal/platform/mysql" >&2
  exit 1
fi
for expected_mysql_operation_dir in \
  internal/operations/agent/memorylineage/mysql \
  internal/operations/cassandra/backfill/mysql \
  internal/operations/search/backfill/mysql \
  internal/operations/search/cleanup/mysql \
  internal/operations/sync/baseline/mysql \
  internal/operations/sync/replay/mysql \
  internal/platform/mysql/config \
  internal/platform/mysql/migration \
  internal/platform/mysql/testutil; do
  if [[ ! -d "${root_dir}/${expected_mysql_operation_dir}" ]]; then
    echo "MySQL operation/platform directory is missing: ${expected_mysql_operation_dir}" >&2
    exit 1
  fi
done
if [[ -d "${root_dir}/internal/data/elasticsearch" ]]; then
  echo "legacy Elasticsearch adapter directory remains under internal/data" >&2
  exit 1
fi
if [[ -d "${root_dir}/internal/data/routing" || -d "${root_dir}/internal/data/shadow" ]]; then
  echo "legacy storage decorator directories remain under internal/data" >&2
  exit 1
fi
for runtime_consumer_dir in internal/operations internal/platform internal/services internal/bootstrap internal/server internal/gateway internal/transport; do
  if rg --quiet 'internal/data/mysql/repository' "${root_dir}/${runtime_consumer_dir}" --glob '*.go'; then
    echo "new runtime code must use service-owned MySQL repositories; legacy repository aliases are compatibility-only" >&2
    exit 1
  fi
done
if [[ ! -f "${root_dir}/internal/services/sync/infrastructure/kafka/projector.go" ]]; then
  echo "Sync Kafka projector must remain under the Sync service boundary" >&2
  exit 1
fi
if [[ -d "${root_dir}/internal/projector/sync" ]]; then
  echo "legacy Sync projector directory remains outside the Sync service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search/infrastructure/kafka/projector.go" ]]; then
  echo "Search Indexer Kafka projector must remain under the Search service boundary" >&2
  exit 1
fi
if [[ -d "${root_dir}/internal/projector/search" ]]; then
  echo "legacy Search projector directory remains outside the Search service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/infrastructure/cassandra/projector.go" ]]; then
  echo "Message Cassandra projector must remain under the Message service boundary" >&2
  exit 1
fi
if rg --quiet 'internal/services/core/domain/' "${root_dir}/internal/services/message" --glob '*.go'; then
  echo "Message service must not import Core domain implementations" >&2
  exit 1
fi
if [[ -d "${root_dir}/internal/projector/cassandra" ]]; then
  echo "legacy Cassandra projector directory remains outside the Message service boundary" >&2
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
if ! rg --quiet 'services/search/infrastructure/mysql' "${root_dir}/internal/bootstrap/embedded/repositories.go"; then
  echo "Search composition must use Search-owned index repository" >&2
  exit 1
fi
if rg --quiet 'func (New|new)CoreProcessRepositories|type CoreProcessRepositories struct' "${root_dir}/internal/app" --glob '*.go'; then
  echo "Core repository composition must live in the Core service infrastructure" >&2
  exit 1
fi
for legacy_core_file in cached_user_store.go cached_group_store.go cached_contact_store.go; do
  if [[ -e "${root_dir}/internal/app/${legacy_core_file}" ]]; then
    echo "Core cache adapter remains in aggregate app package: ${legacy_core_file}" >&2
    exit 1
  fi
done
if ! rg --quiet 'coremysql.NewProcessRepositories' "${root_dir}/internal/services/core/bootstrap/runtime.go"; then
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
if rg --quiet 'type SyncProcessRepositories struct|func NewSyncProcessRepositories' "${root_dir}/internal/app" --glob '*.go'; then
  echo "Sync repository composition must live in the Sync service infrastructure" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/infrastructure/mysql/composition.go" ]]; then
  echo "Sync process repository composition is outside the Sync service boundary" >&2
  exit 1
fi
if rg --quiet 'internal/app(/|["`])' "${root_dir}/internal/services/sync/bootstrap/runtime.go"; then
  echo "standalone Sync runtime must not depend on aggregate internal/app composition" >&2
  exit 1
fi
if ! rg --quiet 'services/sync/infrastructure/mysql' "${root_dir}/internal/services/sync/bootstrap/runtime.go"; then
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
if rg --quiet 'internal/app(/|["`])' "${root_dir}/internal/services/message/bootstrap/runtime.go"; then
  echo "standalone Message runtime must not depend on aggregate internal/app composition" >&2
  exit 1
fi
if ! rg --quiet 'services/message/infrastructure/mysql' "${root_dir}/internal/services/message/bootstrap/runtime.go"; then
  echo "standalone Message runtime must use Message-owned repository composition" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/application/application.go" ]]; then
  echo "Core capability implementation is outside its service boundary" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/bootstrap/runtime.go" ]]; then
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
if ! rg --quiet 'services/core/infrastructure/mysql' "${root_dir}/internal/bootstrap/embedded/repositories.go"; then
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
# The aggregate app facade is a compatibility boundary. Keep production code
# from depending on it so new standalone services cannot bypass service roots.
while IFS= read -r app_importer; do
  relative_importer="${app_importer#"${root_dir}/"}"
  case "${relative_importer}" in
    internal/app/*)
      ;;
    *)
      echo "production code must not depend on aggregate internal/app: ${relative_importer}" >&2
      exit 1
      ;;
  esac
done < <(rg -l 'github.com/JekYUlll/Dipole/internal/app(/|["`])' "${root_dir}" --glob '*.go' --glob '!**/*_test.go' || true)
for legacy_agent_repository in agent_policy.go agent_memory.go agent_artifact.go agent_tool_invocation.go agent_task_timeline.go agent_runtime_promotion_control.go agent_mcp_tool_round.go agent_mcp_readiness_evidence.go ai_call_log.go; do
  if [[ -e "${root_dir}/internal/data/mysql/repository/${legacy_agent_repository}" ]]; then
    echo "legacy Agent MySQL implementation remains in shared repository package: ${legacy_agent_repository}" >&2
    exit 1
  fi
done
if ! rg --quiet 'services/agent/infrastructure/mysql' "${root_dir}/internal/bootstrap/embedded/repositories.go"; then
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

if [[ ! -f "${root_dir}/internal/services/search/bootstrap/entrypoint.go" || ! -f "${root_dir}/internal/services/search/bootstrap/README.md" ]]; then
  echo "Search bootstrap boundary is missing" >&2
  exit 1
fi
if ! rg --quiet 'internal/services/search/bootstrap' "${root_dir}/cmd/services/search/main.go"; then
  echo "Search entrypoint must use its service-owned bootstrap boundary" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/services/search/main.go"; then
  echo "Search entrypoint must not depend directly on shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/bootstrap/entrypoint.go" || ! -f "${root_dir}/internal/services/message/bootstrap/README.md" ]]; then
  echo "Message bootstrap boundary is missing" >&2
  exit 1
fi
if ! rg --quiet 'internal/services/message/bootstrap' "${root_dir}/cmd/services/message/main.go"; then
  echo "Message entrypoint must use its service-owned bootstrap boundary" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/services/message/main.go"; then
  echo "Message entrypoint must not depend directly on shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/bootstrap/entrypoint.go" || ! -f "${root_dir}/internal/services/sync/bootstrap/README.md" ]]; then
  echo "Sync bootstrap boundary is missing" >&2
  exit 1
fi
if ! rg --quiet 'internal/services/sync/bootstrap' "${root_dir}/cmd/services/sync/main.go"; then
  echo "Sync entrypoint must use its service-owned bootstrap boundary" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/services/sync/main.go"; then
  echo "Sync entrypoint must not depend directly on shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/gateway/bootstrap/entrypoint.go" || ! -f "${root_dir}/internal/services/gateway/bootstrap/README.md" ]]; then
  echo "Gateway bootstrap boundary is missing" >&2
  exit 1
fi
if ! rg --quiet 'internal/services/gateway/bootstrap' "${root_dir}/cmd/services/gateway/main.go"; then
  echo "Gateway entrypoint must use its service-owned bootstrap boundary" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/services/gateway/main.go"; then
  echo "Gateway entrypoint must not depend directly on shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/core/bootstrap/entrypoint.go" || ! -f "${root_dir}/internal/services/core/bootstrap/README.md" ]]; then
  echo "Core bootstrap boundary is missing" >&2
  exit 1
fi
if ! rg --quiet 'internal/services/core/bootstrap' "${root_dir}/cmd/services/core/main.go"; then
  echo "Core entrypoint must use its service-owned bootstrap boundary" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/services/core/main.go"; then
  echo "Core entrypoint must not depend directly on shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search-indexer/bootstrap/entrypoint.go" || ! -f "${root_dir}/internal/services/search-indexer/bootstrap/README.md" ]]; then
  echo "Search Indexer bootstrap boundary is missing" >&2
  exit 1
fi
if ! rg --quiet 'internal/services/search-indexer/bootstrap' "${root_dir}/cmd/services/search-indexer/main.go"; then
  echo "Search Indexer entrypoint must use its service-owned bootstrap boundary" >&2
  exit 1
fi
if rg --quiet 'internal/bootstrap' "${root_dir}/cmd/services/search-indexer/main.go"; then
  echo "Search Indexer entrypoint must not depend directly on shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/platform/runtime/metrics.go" || ! -f "${root_dir}/internal/platform/runtime/readiness.go" || ! -f "${root_dir}/internal/platform/runtime/README.md" ]]; then
  echo "shared runtime metrics and readiness platform boundary is missing" >&2
  exit 1
fi
for runtime_file in runtime.go core_runtime.go cassandra_projector_runtime.go; do
  if [[ -f "${root_dir}/internal/bootstrap/${runtime_file}" ]] && ! rg --quiet 'internal/platform/runtime' "${root_dir}/internal/bootstrap/${runtime_file}"; then
    echo "runtime bootstrap must use internal/platform/runtime: ${runtime_file}" >&2
    exit 1
  fi
done

for runtime_file in runtime.go core_runtime.go cassandra_projector_runtime.go; do
  if [[ -f "${root_dir}/internal/bootstrap/${runtime_file}" ]] && ! rg --quiet 'ConfigureDependencyReadiness|BindRPCReadiness' "${root_dir}/internal/bootstrap/${runtime_file}"; then
    echo "runtime bootstrap must use platform readiness orchestration: ${runtime_file}" >&2
    exit 1
  fi
done
if [[ -e "${root_dir}/internal/bootstrap/search_indexer_runtime.go" ]]; then
  echo "Search Indexer runtime remains in shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search-indexer/bootstrap/runtime.go" ]] || ! rg --quiet 'internal/services/search-indexer/infrastructure|internal/services/search/infrastructure/kafka' "${root_dir}/internal/services/search-indexer/bootstrap/runtime.go"; then
  echo "Search Indexer runtime must remain under its service bootstrap boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/bootstrap/search_runtime.go" ]]; then
  echo "Search runtime remains in shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search/bootstrap/runtime.go" ]] || ! rg --quiet 'internal/services/search/application' "${root_dir}/internal/services/search/bootstrap/runtime.go"; then
  echo "Search runtime must remain under its service bootstrap boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/bootstrap/sync_runtime.go" || -e "${root_dir}/internal/bootstrap/sync_database_boundary.go" ]]; then
  echo "Sync runtime or database boundary remains in shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/sync/bootstrap/runtime.go" ]] || ! rg --quiet 'internal/services/sync/infrastructure' "${root_dir}/internal/services/sync/bootstrap/runtime.go"; then
  echo "Sync runtime must remain under its service bootstrap boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/bootstrap/message_runtime.go" ]]; then
  echo "Message runtime remains in shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/message/bootstrap/runtime.go" ]] || ! rg --quiet 'internal/services/message/infrastructure' "${root_dir}/internal/services/message/bootstrap/runtime.go"; then
  echo "Message runtime must remain under its service bootstrap boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/bootstrap/gateway_runtime.go" ]]; then
  echo "Gateway runtime remains in shared bootstrap" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/gateway/bootstrap/runtime.go" ]] || ! rg --quiet 'internal/gateway|internal/platform/cache|internal/platform/presence' "${root_dir}/internal/services/gateway/bootstrap/runtime.go"; then
  echo "Gateway runtime must remain under its service bootstrap boundary" >&2
  exit 1
fi

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
