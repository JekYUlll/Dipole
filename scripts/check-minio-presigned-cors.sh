#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cors_file="${root_dir}/configs/minio/platform-storage-cors.xml"

ruby -rrexml/document -e '
  doc = REXML::Document.new(File.read(ARGV.fetch(0)))
  rules = doc.get_elements("CORSConfiguration/CORSRule")
  abort "CORSRule must contain exactly one rule" unless rules.length == 1
  rule = rules.fetch(0)
  origins = rule.get_elements("AllowedOrigin").map(&:text)
  abort "wildcard origin is forbidden" if origins.include?("*")
  methods = rule.get_elements("AllowedMethod").map(&:text)
  abort "PUT must be allowed" unless methods.include?("PUT")
  headers = rule.get_elements("ExposeHeader").map(&:text)
  abort "ETag must be exposed" unless headers.include?("ETag")
  max_age = Integer(rule.elements["MaxAgeSeconds"].text)
  abort "CORS max age must be bounded" unless max_age <= 3600
' "${cors_file}"

for compose_file in \
  "${root_dir}/docker-compose.yml" \
  "${root_dir}/deploy/compose/docker-compose.microservices.yml" \
  "${root_dir}/deploy/compose/docker-compose.dist.yml"; do
  if grep -Fq 'mc cors set local/dipole-files' "${compose_file}"; then
    echo "OSS MinIO does not support bucket CORS API: ${compose_file}" >&2
    exit 1
  fi
done

printf 'MinIO presigned-upload CORS policy: validated; OSS runtime uses gateway CORS\n'
