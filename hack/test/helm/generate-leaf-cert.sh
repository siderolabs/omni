#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

# Generates a short-lived leaf TLS certificate signed by the committed CA.
# Usage: generate-leaf-cert.sh <ip>
# Outputs the paths to the generated cert and key on stdout (one per line).

set -eou pipefail

IP="${1:?usage: generate-leaf-cert.sh <ip>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CA_CRT="${SCRIPT_DIR}/certs/ca.crt"
CA_KEY="${SCRIPT_DIR}/certs/ca.key"

TLS_KEY=$(mktemp)
TLS_CSR=$(mktemp)
TLS_CRT=$(mktemp)
TLS_EXT=$(mktemp)

openssl ecparam -genkey -name prime256v1 -noout -out "${TLS_KEY}" 2>/dev/null
openssl req -new -key "${TLS_KEY}" -out "${TLS_CSR}" -subj "/CN=example.org" 2>/dev/null
printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectKeyIdentifier=hash\nauthorityKeyIdentifier=keyid:always\nsubjectAltName=DNS:*.example.org,DNS:example.org,DNS:*.omni-workload.example.org,IP:%s\n' "${IP}" >"${TLS_EXT}"
openssl x509 -req -in "${TLS_CSR}" -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
  -CAcreateserial -out "${TLS_CRT}" -days 90 -extfile "${TLS_EXT}" 2>/dev/null

echo "${TLS_CRT}"
echo "${TLS_KEY}"
