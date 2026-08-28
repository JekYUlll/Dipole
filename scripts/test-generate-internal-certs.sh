#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
output_dir="$(mktemp -d)"

cleanup() {
  rm -rf "${output_dir}"
}
trap cleanup EXIT

INTERNAL_CERT_DIR="${output_dir}" "${SCRIPT_DIR}/generate-internal-certs.sh" >/dev/null 2>&1

for key in "${output_dir}"/*-key.pem; do
  mode="$(stat -c '%a' "${key}")"
  if [[ "${mode}" != "600" ]]; then
    printf 'private key %s has mode %s, want 600\n' "${key}" "${mode}" >&2
    exit 1
  fi
done

for certificate in "${output_dir}"/*.pem; do
  if [[ "${certificate}" == *-key.pem ]]; then
    continue
  fi
  mode="$(stat -c '%a' "${certificate}")"
  if [[ "${mode}" != "644" ]]; then
    printf 'certificate %s has mode %s, want 644\n' "${certificate}" "${mode}" >&2
    exit 1
  fi
done

echo "internal certificate permission test passed"
