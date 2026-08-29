#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cors_file="${root_dir}/configs/minio/platform-storage-cors.json"

ruby -rjson -e '
  rules = JSON.parse(File.read(ARGV.fetch(0))).fetch("CORSRules")
  abort "CORSRules must contain exactly one rule" unless rules.length == 1
  rule = rules.fetch(0)
  origins = rule.fetch("AllowedOrigins")
  abort "wildcard origin is forbidden" if origins.include?("*")
  abort "PUT must be allowed" unless rule.fetch("AllowedMethods").include?("PUT")
  abort "ETag must be exposed" unless rule.fetch("ExposeHeaders").include?("ETag")
  abort "CORS max age must be bounded" unless rule.fetch("MaxAgeSeconds") <= 3600
' "${cors_file}"

for compose_file in \
  "${root_dir}/docker-compose.yml" \
  "${root_dir}/deploy/compose/docker-compose.microservices.yml" \
  "${root_dir}/deploy/compose/docker-compose.dist.yml"; do
  grep -Fq 'platform-storage-cors.json:/policies/platform-storage-cors.json:ro' "${compose_file}"
  grep -Fq 'mc cors set local/dipole-files /policies/platform-storage-cors.json' "${compose_file}"
done

printf 'MinIO presigned-upload CORS contract: ok\n'
