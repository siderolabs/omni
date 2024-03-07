#!/bin/bash

# Copyright (c) 2024 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Settings.

TALOS_VERSION=1.6.4
ARTIFACTS=_out
JOIN_TOKEN=testonly
RUN_DIR=$(pwd)

# Download required artifacts.

mkdir -p ${ARTIFACTS}

[ -f ${ARTIFACTS}/talosctl ] || (crane export ghcr.io/siderolabs/talosctl:latest | tar x -C ${ARTIFACTS})

# Your image schematic ID is: cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f
#
# customization:
#     systemExtensions:
#         officialExtensions:
#             - siderolabs/hello-world-service
SCHEMATIC_ID="cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f"

# Build registry mirror args.

if [[ "${CI:-false}" == "true" ]]; then
  REGISTRY_MIRROR_FLAGS=()

  for registry in docker.io k8s.gcr.io quay.io gcr.io ghcr.io registry.k8s.io factory.talos.dev; do
    service="registry-${registry//./-}.ci.svc"
    addr=$(python3 -c "import socket; print(socket.gethostbyname('${service}'))")

    REGISTRY_MIRROR_FLAGS+=("--registry-mirror=${registry}=http://${addr}:5000")
  done
else
  # use the value from the environment, if present
  REGISTRY_MIRROR_FLAGS=("${REGISTRY_MIRROR_FLAGS:-}")
fi

function cleanup() {
  cd "${RUN_DIR}"
  rm -rf ${ARTIFACTS}/omni.db ${ARTIFACTS}/etcd/
}

trap cleanup EXIT SIGINT

# Determine the local IP SideroLink API will listen on
LOCAL_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')

# Start Vault.

docker run --rm -d --cap-add=IPC_LOCK -p 8200:8200 -e 'VAULT_DEV_ROOT_TOKEN_ID=dev-o-token' --name vault-dev hashicorp/vault:1.15

sleep 10

# Load key into Vault.

docker cp internal/backend/runtime/omni/testdata/pgp/old_key.private vault-dev:/tmp/old_key.private
docker exec -e VAULT_ADDR='http://0.0.0.0:8200' -e VAULT_TOKEN=dev-o-token vault-dev \
    vault kv put -mount=secret omni-private-key \
    private-key=@/tmp/old_key.private

sleep 5

[ -f ${ARTIFACTS}/minio ] || curl -Lo ${ARTIFACTS}/minio https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x ${ARTIFACTS}/minio && MINIO_ACCESS_KEY=access MINIO_SECRET_KEY=secret123 ${ARTIFACTS}/minio server minio-server/export &

sleep 2

[ -f ${ARTIFACTS}/mc ] || curl -Lo ${ARTIFACTS}/mc https://dl.min.io/client/mc/release/linux-amd64/mc
chmod +x ${ARTIFACTS}/mc && ${ARTIFACTS}/mc alias set myminio http://127.0.0.1:9000 access secret123 && ${ARTIFACTS}/mc mb myminio/mybucket


# Launch Omni in the background.

export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN=dev-o-token
export AUTH_USERNAME="${AUTH0_TEST_USERNAME}"
export AUTH_PASSWORD="${AUTH0_TEST_PASSWORD}"
export BASE_URL=https://localhost:8099/
export VIDEO_DIR=""

# Create omnictl downloads directory (required by the server) and copy the omnictl binaries in it.
mkdir -p omnictl
cp -p ${ARTIFACTS}/omnictl-* omnictl/

SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
nice -n 10 ${ARTIFACTS}/omni-linux-amd64 \
    --siderolink-wireguard-advertised-addr 172.20.0.1:50180 \
    --siderolink-api-advertised-url "grpc://$LOCAL_IP:8090" \
    --auth-auth0-enabled true \
    --advertised-api-url "${BASE_URL}" \
    --auth-auth0-client-id jS47T064CfFE1IqqIYZ8HfYLkZhFoa1Y \
    --auth-auth0-domain sidero-omni-dev.us.auth0.com \
    --initial-users "${AUTH_USERNAME}" \
    --private-key-source "vault://secret/omni-private-key" \
    --public-key-files "internal/backend/runtime/omni/testdata/pgp/new_key.public" \
    --bind-addr 0.0.0.0:8099 \
    --key hack/certs/localhost-key.pem \
    --cert hack/certs/localhost.pem \
    --etcd-embedded-unsafe-fsync=true \
    --etcd-backup-s3 \
    "${REGISTRY_MIRROR_FLAGS[@]}" \
    &

# Launch empty Talos VMs.
${ARTIFACTS}/talosctl cluster create \
    --provisioner=qemu \
    --cidr=172.20.0.0/24 \
    --controlplanes=1 \
    --workers=7 \
    --skip-injecting-config \
    --extra-boot-kernel-args "siderolink.api=grpc://$LOCAL_IP:8090?jointoken=${JOIN_TOKEN} talos.events.sink=[fdae:41e4:649b:9303::1]:8090 talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092" \
    --vmlinuz-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/kernel-amd64" \
    --initrd-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/initramfs-amd64.xz" \
    --wait=false \
    --mtu=1430 \
    --memory=3072 \
    --memory-workers=3072 \
    --cpus=3 \
    --cpus-workers=3 \
    --with-init-node \
    --with-uuid-hostnames

sleep 5

# Run the integration test.

SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
${ARTIFACTS}/integration-test-linux-amd64 \
    --endpoint https://localhost:8099 \
    --talos-version=${TALOS_VERSION} \
    --expected-machines=8 `# equal to the masters+workers above` \
    ${INTEGRATION_TEST_ARGS:-}

if [ "${INTEGRATION_RUN_E2E_TEST:-true}" == "true" ]; then
  # Run the e2e test.
  # the e2e tests are in a submodule and need to be executed in a container with playwright dependencies
  cd internal/e2e-tests/
  docker buildx build --load . -t e2etest
  docker run --rm \
      -e AUTH_PASSWORD=$AUTH_PASSWORD \
      -e AUTH_USERNAME=$AUTH_USERNAME \
      -e BASE_URL=$BASE_URL \
      -e VIDEO_DIR="$VIDEO_DIR" \
      --network=host \
      e2etest
fi

# No cleanup here, as it runs in the CI as a container in a pod.
