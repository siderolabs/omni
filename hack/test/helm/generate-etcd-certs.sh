#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

# Generates etcd server and client TLS certificates signed by the committed CA.
# Usage: generate-etcd-certs.sh
# Outputs four temp file paths on stdout (one per line):
#   server.crt, server.key, client.crt, client.key

set -eou pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CA_CRT="${SCRIPT_DIR}/certs/ca.crt"
CA_KEY="${SCRIPT_DIR}/certs/ca.key"

# --- Server certificate ---
SERVER_KEY=$(mktemp)
SERVER_CSR=$(mktemp)
SERVER_CRT=$(mktemp)
SERVER_EXT=$(mktemp)

openssl ecparam -genkey -name prime256v1 -noout -out "${SERVER_KEY}" 2>/dev/null
openssl req -new -key "${SERVER_KEY}" -out "${SERVER_CSR}" -subj "/CN=etcd" 2>/dev/null

cat >"${SERVER_EXT}" <<'EXTEOF'
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth,clientAuth
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid:always
subjectAltName=DNS:etcd.etcd.svc.cluster.local,DNS:etcd.etcd.svc,DNS:etcd.etcd,DNS:localhost,IP:127.0.0.1
EXTEOF

openssl x509 -req -in "${SERVER_CSR}" -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
  -CAcreateserial -out "${SERVER_CRT}" -days 90 -extfile "${SERVER_EXT}" 2>/dev/null

# --- Client certificate ---
CLIENT_KEY=$(mktemp)
CLIENT_CSR=$(mktemp)
CLIENT_CRT=$(mktemp)
CLIENT_EXT=$(mktemp)

openssl ecparam -genkey -name prime256v1 -noout -out "${CLIENT_KEY}" 2>/dev/null
openssl req -new -key "${CLIENT_KEY}" -out "${CLIENT_CSR}" -subj "/CN=omni-etcd-client" 2>/dev/null

cat >"${CLIENT_EXT}" <<'EXTEOF'
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=clientAuth
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid:always
EXTEOF

openssl x509 -req -in "${CLIENT_CSR}" -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
  -CAcreateserial -out "${CLIENT_CRT}" -days 90 -extfile "${CLIENT_EXT}" 2>/dev/null

echo "${SERVER_CRT}"
echo "${SERVER_KEY}"
echo "${CLIENT_CRT}"
echo "${CLIENT_KEY}"
