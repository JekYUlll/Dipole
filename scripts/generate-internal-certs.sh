#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT_DIR="${INTERNAL_CERT_DIR:-${ROOT_DIR}/certs/internal}"
VALID_DAYS="${INTERNAL_CERT_VALID_DAYS:-30}"

command -v openssl >/dev/null 2>&1 || {
  echo "openssl is required" >&2
  exit 1
}

mkdir -p "${OUTPUT_DIR}"
umask 077

CA_KEY="${OUTPUT_DIR}/ca-key.pem"
CA_CERT="${OUTPUT_DIR}/ca.pem"
openssl ecparam -name prime256v1 -genkey -noout -out "${CA_KEY}"
openssl req -x509 -new -sha256 -key "${CA_KEY}" -out "${CA_CERT}" \
  -days "${VALID_DAYS}" -subj "/CN=Dipole Internal Development CA"

for service in gateway core message; do
  identity="dipole-${service}"
  key="${OUTPUT_DIR}/${service}-key.pem"
  csr="${OUTPUT_DIR}/${service}.csr"
  cert="${OUTPUT_DIR}/${service}.pem"
  extension="${OUTPUT_DIR}/${service}.ext"

  openssl ecparam -name prime256v1 -genkey -noout -out "${key}"
  openssl req -new -sha256 -key "${key}" -out "${csr}" -subj "/CN=${identity}"
  cat >"${extension}" <<EOF
basicConstraints=CA:FALSE
keyUsage=digitalSignature
extendedKeyUsage=serverAuth,clientAuth
subjectAltName=DNS:dipole-internal,DNS:${service}
EOF
  openssl x509 -req -sha256 -in "${csr}" -CA "${CA_CERT}" -CAkey "${CA_KEY}" \
    -CAcreateserial -out "${cert}" -days "${VALID_DAYS}" -extfile "${extension}"
  rm "${csr}" "${extension}"
done

chmod 600 "${OUTPUT_DIR}"/*-key.pem
chmod 644 "${OUTPUT_DIR}"/*.pem
echo "Internal development certificates written to ${OUTPUT_DIR}"
