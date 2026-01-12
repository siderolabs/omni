#!/bin/bash

# Copyright (c) 2025 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Omni is served from "https://my-instance.localhost:8099"
# Exposed services through workload proxying follow the pattern: "https://sngmph-my-instance.proxy-us.localhost:8099/"
# The TLS key and cert, hack/certs/localhost-key.pem and hack/certs/localhost.pem contain the SANs:
# - localhost
# - *.localhost
# - my-instance.localhost
# - *.my-instance.localhost
#
# Write "my-instance.localhost" to /etc/hosts to avoid problems with the name resolution.
echo "127.0.0.1 my-instance.localhost" | tee -a /etc/hosts

# Settings.
ARTIFACTS=_out
JOIN_TOKEN=testonly
RUN_DIR=$(pwd)
TEST_OUTPUTS_DIR=${GITHUB_WORKSPACE:-/tmp}/integration-test-e2e
ENABLE_TALOS_PRERELEASE_VERSIONS=${ENABLE_TALOS_PRERELEASE_VERSIONS:-true}

mkdir -p "$TEST_OUTPUTS_DIR"

# Download required artifacts.

mkdir -p ${ARTIFACTS}

[ -f ${ARTIFACTS}/talosctl ] || (crane export ghcr.io/siderolabs/talosctl:latest | tar x -C ${ARTIFACTS})

echo "Talosctl Version:"
${ARTIFACTS}/talosctl version --client

# Determine the local IP SideroLink API will listen on
LOCAL_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')
WIREGUARD_IP=$LOCAL_IP

if [[ "${CI:-false}" == "true" ]]; then
  WIREGUARD_IP=172.20.0.1
fi

# Build registry mirror args.
if [[ "${CI:-false}" == "true" ]]; then
  REGISTRY_MIRROR_FLAGS=()
  REGISTRY_MIRROR_CONFIG="
registries:
  mirrors:
"

  for registry in docker.io k8s.gcr.io quay.io gcr.io ghcr.io registry.k8s.io factory.talos.dev; do
    service="registry-${registry//./-}.ci.svc"
    addr=$(python3 -c "import socket; print(socket.gethostbyname('${service}'))")

    REGISTRY_MIRROR_FLAGS+=("--registry-mirror=${registry}=http://${addr}:5000")
    REGISTRY_MIRROR_CONFIG+="    - ${registry}=http://${addr}:5000"
    REGISTRY_MIRROR_CONFIG+=$'\n'
  done
else
  # use the value from the environment, if present
  REGISTRY_MIRROR_FLAGS=("${REGISTRY_MIRROR_FLAGS:-}")
  REGISTRY_MIRROR_CONFIG="${REGISTRY_MIRROR_CONFIG:-}"
fi

VAULT_CONTAINER_NAME=vault-dev-e2e
MINIO_PID=0
OMNI_PID=0

function cleanup() {
  cd "${RUN_DIR}"

  if [ $OMNI_PID -ne 0 ]; then
    kill $OMNI_PID || true
  fi

  if docker ps -a --format '{{.Names}}' | grep -q "^${VAULT_CONTAINER_NAME}$"; then
    docker stop ${VAULT_CONTAINER_NAME} || true
    docker rm -f ${VAULT_CONTAINER_NAME} || true
  fi

  if [ $MINIO_PID -ne 0 ]; then
    kill $MINIO_PID || true
  fi

  # In CI, SUDO_USER is set to be "worker", and these output directories are used in the subsequent job steps.
  chown -R "${SUDO_USER:-$(whoami)}" "${TEST_OUTPUTS_DIR}"
  chown -R "${SUDO_USER:-$(whoami)}" "${ARTIFACTS}"
}

trap cleanup EXIT SIGINT

# Start Vault.
docker run --rm -d --cap-add=IPC_LOCK -p 8200:8200 -e 'VAULT_DEV_ROOT_TOKEN_ID=dev-o-token' --name ${VAULT_CONTAINER_NAME} hashicorp/vault:1.18

sleep 10

# Load key into Vault.
docker cp internal/backend/runtime/omni/testdata/pgp/old_key.private ${VAULT_CONTAINER_NAME}:/tmp/old_key.private
docker exec -e VAULT_ADDR='http://0.0.0.0:8200' -e VAULT_TOKEN=dev-o-token ${VAULT_CONTAINER_NAME} \
  vault kv put -mount=secret omni-private-key \
  private-key=@/tmp/old_key.private

sleep 5

# Start Minio.
[ -f ${ARTIFACTS}/minio ] || curl -Lo ${ARTIFACTS}/minio https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x ${ARTIFACTS}/minio && MINIO_ACCESS_KEY=access MINIO_SECRET_KEY=secret123 ${ARTIFACTS}/minio server minio-server/export &
MINIO_PID=$!

sleep 2

[ -f ${ARTIFACTS}/mc ] || curl -Lo ${ARTIFACTS}/mc https://dl.min.io/client/mc/release/linux-amd64/mc
chmod +x ${ARTIFACTS}/mc && ${ARTIFACTS}/mc alias set myminio http://127.0.0.1:9000 access secret123 && ${ARTIFACTS}/mc mb myminio/mybucket || true

# Set up environment variables.
export CI="${CI}"
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN=dev-o-token
export AUTH_USERNAME="${AUTH0_TEST_USERNAME}"
export AUTH_PASSWORD="${AUTH0_TEST_PASSWORD}"
export BASE_URL=https://my-instance.localhost:8099/
export AUTH0_CLIENT_ID="${AUTH0_CLIENT_ID}"
export AUTH0_DOMAIN="${AUTH0_DOMAIN}"

# Create omnictl downloads directory (required by the server) and copy the omnictl binaries in it.
mkdir -p omnictl
cp -p ${ARTIFACTS}/omnictl-* omnictl/

# Launch empty Talos VMs.
${ARTIFACTS}/talosctl cluster create \
    --provisioner=qemu \
    --controlplanes=3 \
    --workers=0 \
    --wait=false \
    --memory=1024 \
    --memory-workers=1024 \
    --with-uuid-hostnames \
    --name=e2e-test-machines \
    --cidr=172.20.0.0/24 \
    --no-masquerade-cidrs=172.21.0.0/24,172.22.0.0/24 \
    --skip-injecting-config \
    --skip-injecting-extra-cmdline \
    --extra-boot-kernel-args "siderolink.api=grpc://$LOCAL_IP:8090?jointoken=${JOIN_TOKEN} talos.events.sink=[fdae:41e4:649b:9303::1]:8090 talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092"

sleep 5

# Write partial omni config
echo "---
storage:
  sqlite:
    path: ${TEST_OUTPUTS_DIR}/sqlite.db
services:
  api:
    endpoint: 0.0.0.0:8099
    advertisedURL: ${BASE_URL}
    certFile: hack/certs/localhost.pem
    keyFile: hack/certs/localhost-key.pem" >"${TEST_OUTPUTS_DIR}/e2e-config.yaml"

# Start Omni in the background.
echo "Starting Omni..."
SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
  nice -n 10 ${ARTIFACTS}/omni-linux-amd64 --config-path "${TEST_OUTPUTS_DIR}/e2e-config.yaml" \
  --siderolink-wireguard-advertised-addr $LOCAL_IP:50180 \
  --siderolink-api-advertised-url "grpc://$LOCAL_IP:8090" \
  --event-sink-port 8091 \
  --auth-auth0-enabled true \
  --auth-auth0-client-id "${AUTH0_CLIENT_ID}" \
  --auth-auth0-domain "${AUTH0_DOMAIN}" \
  --initial-users "${AUTH_USERNAME}" \
  --private-key-source "vault://secret/omni-private-key" \
  --public-key-files "internal/backend/runtime/omni/testdata/pgp/new_key.public" \
  --etcd-embedded-unsafe-fsync=true \
  --etcd-backup-s3 \
  --join-tokens-mode strict \
  --audit-log-dir "${TEST_OUTPUTS_DIR}/audit-log" \
  --config-data-compression-enabled \
  --enable-talos-pre-release-versions="${ENABLE_TALOS_PRERELEASE_VERSIONS}" \
  --enable-cluster-import \
  "${REGISTRY_MIRROR_FLAGS[@]}" \
  >"${TEST_OUTPUTS_DIR}/omni-e2e.log" 2>&1 &

# Run the e2e test.
# the e2e tests are in a submodule and need to be executed in a container with playwright dependencies
cd frontend/
docker buildx build --load . -t e2etest
docker run --rm \
  -e CI="$CI" \
  -e AUTH_PASSWORD="$AUTH_PASSWORD" \
  -e AUTH_USERNAME="$AUTH_USERNAME" \
  -e BASE_URL=$BASE_URL \
  -v "${TEST_OUTPUTS_DIR}/e2e/playwright-report:/tmp/test/playwright-report" \
  --network=host \
  --add-host="my-instance.localhost:127.0.0.1" \
  e2etest

# No cleanup here, as it runs in the CI as a container in a pod.
