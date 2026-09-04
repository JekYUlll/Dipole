#!/usr/bin/env bash
set -euo pipefail

# This narrow operational tool only provisions the Core-owned operator control
# plane. It never approves a Runtime candidate or issues a promotion grant.

usage() {
  cat <<'EOF'
Usage:
  manage-agent-promotion-operator-grant.sh grant|revoke [options]

Required options:
  --compose-project NAME        Existing Compose project to target.
  --compose-file PATH           Compose file used by that project; repeatable.
  --user UUID                   Operator receiving or losing the grant.
  --granted-by UUID             Different operator recording this change.
  --ticket REF                  Change ticket or approval reference.
  --reason TEXT                 Human-readable, single-line reason.

Grant-only options:
  --roles propose,review,revoke One or more roles to grant.
  --expires-at RFC3339_UTC      Example: 2026-09-05T12:00:00Z.

Optional:
  --tenant ID                   Defaults to dipole.
  --service NAME                MySQL Compose service, defaults to mysql.
  --database NAME               Database, defaults to dipole.
  --apply                       Execute. Without it the command is a dry run.

The script reads DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD only when --apply
is present. It streams that password over stdin to the container, avoiding
command-line password arguments. Every successful grant or revoke appends an
audit row. The action must be reviewed before running it in a shared project.
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 2
}

require_safe_id() {
  local label=$1 value=$2 max=$3
  [[ "$value" =~ ^[A-Za-z0-9._:-]+$ ]] && (( ${#value} <= max )) || die "invalid ${label}"
}

sql_quote() {
  printf '%s' "$1" | sed "s/'/''/g"
}

action=${1:-}
[[ "$action" == "grant" || "$action" == "revoke" ]] || { usage >&2; die "first argument must be grant or revoke"; }
shift

tenant_id=dipole
service=mysql
database=dipole
compose_project=
user_uuid=
granted_by_uuid=
ticket_ref=
reason=
roles=
expires_at=
apply=0
compose_files=()

while (( $# > 0 )); do
  case "$1" in
    --compose-project) compose_project=${2:-}; shift 2 ;;
    --compose-file) compose_files+=("${2:-}"); shift 2 ;;
    --tenant) tenant_id=${2:-}; shift 2 ;;
    --service) service=${2:-}; shift 2 ;;
    --database) database=${2:-}; shift 2 ;;
    --user) user_uuid=${2:-}; shift 2 ;;
    --granted-by) granted_by_uuid=${2:-}; shift 2 ;;
    --ticket) ticket_ref=${2:-}; shift 2 ;;
    --reason) reason=${2:-}; shift 2 ;;
    --roles) roles=${2:-}; shift 2 ;;
    --expires-at) expires_at=${2:-}; shift 2 ;;
    --apply) apply=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n "$compose_project" ]] || die "--compose-project is required"
(( ${#compose_files[@]} > 0 )) || die "at least one --compose-file is required"
[[ -n "$user_uuid" && -n "$granted_by_uuid" && -n "$ticket_ref" && -n "$reason" ]] || die "--user, --granted-by, --ticket, and --reason are required"
require_safe_id tenant "$tenant_id" 64
require_safe_id service "$service" 64
require_safe_id database "$database" 64
require_safe_id user "$user_uuid" 24
require_safe_id granted-by "$granted_by_uuid" 24
[[ "$user_uuid" != "$granted_by_uuid" ]] || die "--user and --granted-by must be different operators"
[[ "$ticket_ref" =~ ^[A-Za-z0-9._:/#-]+$ ]] && (( ${#ticket_ref} <= 128 )) || die "invalid ticket reference"
[[ "$reason" != *$'\n'* && "$reason" != *$'\r'* && ${#reason} -le 1000 ]] || die "reason must be a single line of at most 1000 characters"

can_propose=0
can_review=0
can_revoke=0
if [[ "$action" == "grant" ]]; then
  [[ -n "$roles" && -n "$expires_at" ]] || die "grant requires --roles and --expires-at"
  [[ "$expires_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || die "--expires-at must be RFC3339 UTC to the second"
  command -v date >/dev/null 2>&1 || die "date is required to validate --expires-at"
  normalized_expiry=$(date -u -d "$expires_at" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null) || die "--expires-at is not a valid UTC timestamp"
  [[ "$normalized_expiry" == "$expires_at" ]] || die "--expires-at is not a valid UTC timestamp"
  (( $(date -u -d "$expires_at" +%s) > $(date -u +%s) )) || die "--expires-at must be in the future"
  IFS=',' read -r -a role_list <<<"$roles"
  for role in "${role_list[@]}"; do
    case "$role" in
      propose) can_propose=1 ;;
      review) can_review=1 ;;
      revoke) can_revoke=1 ;;
      *) die "unsupported operator role: $role" ;;
    esac
  done
else
  [[ -z "$roles" && -z "$expires_at" ]] || die "revoke does not accept --roles or --expires-at"
fi

printf 'Operator promotion %s plan: tenant=%s user=%s granted_by=%s ticket=%s\n' \
  "$action" "$tenant_id" "$user_uuid" "$granted_by_uuid" "$ticket_ref"
if [[ "$action" == "grant" ]]; then
  printf 'roles=%s expires_at=%s\n' "$roles" "$expires_at"
fi
if (( ! apply )); then
  printf 'Dry run only. Re-run with --apply after reviewing this plan.\n'
  exit 0
fi

: "${DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD:?set DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD before --apply}"
command -v docker >/dev/null 2>&1 || die "docker is required for --apply"
command -v openssl >/dev/null 2>&1 || die "openssl is required for --apply"

compose=(docker compose -p "$compose_project")
for compose_file in "${compose_files[@]}"; do
  compose+=(-f "$compose_file")
done

audit_uuid=$(openssl rand -hex 32)
quoted_tenant=$(sql_quote "$tenant_id")
quoted_user=$(sql_quote "$user_uuid")
quoted_actor=$(sql_quote "$granted_by_uuid")
quoted_ticket=$(sql_quote "$ticket_ref")
quoted_reason=$(sql_quote "$reason")

if [[ "$action" == "grant" ]]; then
  sql=$(cat <<EOF
INSERT INTO agent_runtime_promotion_operator_grants (
  tenant_id, user_uuid, can_propose, can_review, can_revoke, granted_by_uuid,
  valid_from, expires_at, revoked_at
) VALUES (
  '${quoted_tenant}', '${quoted_user}', ${can_propose}, ${can_review}, ${can_revoke}, '${quoted_actor}',
  UTC_TIMESTAMP(3), STR_TO_DATE('${expires_at}', '%Y-%m-%dT%H:%i:%sZ'), NULL
) ON DUPLICATE KEY UPDATE
  can_propose = VALUES(can_propose), can_review = VALUES(can_review), can_revoke = VALUES(can_revoke),
  granted_by_uuid = VALUES(granted_by_uuid), valid_from = VALUES(valid_from), expires_at = VALUES(expires_at),
  revoked_at = NULL;
INSERT INTO agent_runtime_promotion_operator_grant_audits (
  audit_uuid, tenant_id, user_uuid, action, can_propose, can_review, can_revoke,
  granted_by_uuid, ticket_ref, reason, expires_at, occurred_at
) VALUES (
  '${audit_uuid}', '${quoted_tenant}', '${quoted_user}', 'granted', ${can_propose}, ${can_review}, ${can_revoke},
  '${quoted_actor}', '${quoted_ticket}', '${quoted_reason}', STR_TO_DATE('${expires_at}', '%Y-%m-%dT%H:%i:%sZ'), UTC_TIMESTAMP(3)
);
SELECT 'granted' AS action, audit_uuid FROM agent_runtime_promotion_operator_grant_audits WHERE audit_uuid = '${audit_uuid}';
EOF
)
else
  sql=$(cat <<EOF
UPDATE agent_runtime_promotion_operator_grants
SET revoked_at = UTC_TIMESTAMP(3)
WHERE tenant_id = '${quoted_tenant}' AND user_uuid = '${quoted_user}' AND revoked_at IS NULL;
SET @operator_grant_revoked = ROW_COUNT();
INSERT INTO agent_runtime_promotion_operator_grant_audits (
  audit_uuid, tenant_id, user_uuid, action, can_propose, can_review, can_revoke,
  granted_by_uuid, ticket_ref, reason, expires_at, occurred_at
)
SELECT '${audit_uuid}', tenant_id, user_uuid, 'revoked', can_propose, can_review, can_revoke,
  '${quoted_actor}', '${quoted_ticket}', '${quoted_reason}', expires_at, UTC_TIMESTAMP(3)
FROM agent_runtime_promotion_operator_grants
WHERE tenant_id = '${quoted_tenant}' AND user_uuid = '${quoted_user}' AND @operator_grant_revoked = 1;
SELECT IF(@operator_grant_revoked = 1, 'revoked', 'not_active') AS action, '${audit_uuid}' AS audit_uuid;
EOF
)
fi

result=$(printf '%s\n%s\n' "$DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD" "$sql" | "${compose[@]}" exec -T "$service" sh -ceu '
  IFS= read -r MYSQL_PWD
  export MYSQL_PWD
  mysql --socket=/var/run/mysqld/mysqld.sock --connect-timeout=2 -N -B -u root "$1"
' sh "$database")

if [[ "$action" == "revoke" && "$result" == not_active$'\t'* ]]; then
  printf 'No active operator grant was revoked.\n' >&2
  exit 1
fi
printf 'Applied operator promotion %s: %s\n' "$action" "$result"
