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

TALOS_VERSION=1.9.4
ARTIFACTS=_out
JOIN_TOKEN=testonly
RUN_DIR=$(pwd)
ENABLE_SECUREBOOT=${ENABLE_SECUREBOOT:-false}
KERNEL_ARGS_WORKERS_COUNT=2
TALEMU_CONTAINER_NAME=talemu
TALEMU_INFRA_PROVIDER_IMAGE=ghcr.io/siderolabs/talemu-infra-provider:latest
TEST_LOGS_DIR=/tmp/test-logs

mkdir -p $TEST_LOGS_DIR

# Download required artifacts.

mkdir -p ${ARTIFACTS}

[ -f ${ARTIFACTS}/talosctl ] || (crane export ghcr.io/siderolabs/talosctl:latest | tar x -C ${ARTIFACTS})

# Your image schematic ID is: cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f
#
# customization:
#   systemExtensions:
#     officialExtensions:
#       - siderolabs/hello-world-service
SCHEMATIC_ID="cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f"

# Build registry mirror args.

if [[ "${ENABLE_SECUREBOOT}" == "false" ]]; then
  KERNEL_ARGS_WORKERS_COUNT=4
fi

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

  if docker ps -a --format '{{.Names}}' | grep -q "^${TALEMU_CONTAINER_NAME}$"; then
    docker stop ${TALEMU_CONTAINER_NAME} || true
    docker logs ${TALEMU_CONTAINER_NAME} &>$TEST_LOGS_DIR/${TALEMU_CONTAINER_NAME}.log || true
    docker rm -f ${TALEMU_CONTAINER_NAME} || true
  fi
}

trap cleanup EXIT SIGINT

# Determine the local IP SideroLink API will listen on
LOCAL_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')

# Start Vault.

docker run --rm -d --cap-add=IPC_LOCK -p 8200:8200 -e 'VAULT_DEV_ROOT_TOKEN_ID=dev-o-token' --name vault-dev hashicorp/vault:1.18

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
export BASE_URL=https://my-instance.localhost:8099/
export VIDEO_DIR=""
export AUTH0_CLIENT_ID="${AUTH0_CLIENT_ID}"
export AUTH0_DOMAIN="${AUTH0_DOMAIN}"

# Create omnictl downloads directory (required by the server) and copy the omnictl binaries in it.
mkdir -p omnictl
cp -p ${ARTIFACTS}/omnictl-* omnictl/

SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
  nice -n 10 ${ARTIFACTS}/omni-linux-amd64 \
  --siderolink-wireguard-advertised-addr $LOCAL_IP:50180 \
  --siderolink-api-advertised-url "grpc://$LOCAL_IP:8090" \
  --auth-auth0-enabled true \
  --advertised-api-url "${BASE_URL}" \
  --auth-auth0-client-id "${AUTH0_CLIENT_ID}" \
  --auth-auth0-domain "${AUTH0_DOMAIN}" \
  --initial-users "${AUTH_USERNAME}" \
  --private-key-source "vault://secret/omni-private-key" \
  --public-key-files "internal/backend/runtime/omni/testdata/pgp/new_key.public" \
  --bind-addr 0.0.0.0:8099 \
  --key hack/certs/localhost-key.pem \
  --cert hack/certs/localhost.pem \
  --etcd-embedded-unsafe-fsync=true \
  --etcd-backup-s3 \
  --audit-log-dir /tmp/omni-data/audit-log \
  "${REGISTRY_MIRROR_FLAGS[@]}" \
  &

if [[ "${RUN_TALEMU_TESTS:-false}" == "true" ]]; then
  PROMETHEUS_CONTAINER=$(docker run --network host -p "9090:9090" -v "$(pwd)/hack/compose/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml" -it --rm -d prom/prometheus)

  docker pull "${TALEMU_INFRA_PROVIDER_IMAGE}"
  docker run --name $TALEMU_CONTAINER_NAME \
    --network host --cap-add=NET_ADMIN \
    -it -d \
    "${TALEMU_INFRA_PROVIDER_IMAGE}" \
    --create-service-account \
    --omni-api-endpoint="https://$LOCAL_IP:8099"

  SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
    ${ARTIFACTS}/integration-test-linux-amd64 \
    --endpoint https://my-instance.localhost:8099 \
    --talos-version=${TALOS_VERSION} \
    --omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
    --expected-machines=30 \
    --provision-config-file=hack/test/provisionconfig.yaml \
    --run-stats-check \
    -t 4m \
    -p 10 \
    ${TALEMU_TEST_ARGS:-}

  docker rm -f "$PROMETHEUS_CONTAINER"
fi

# Prepare partial machine config
PARTIAL_CONFIG=$(
  cat <<EOF
apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://$LOCAL_IP:8090?jointoken=${JOIN_TOKEN}
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8090'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: 'tcp://[fdae:41e4:649b:9303::1]:8092'
EOF
)
PARTIAL_CONFIG_DIR="${ARTIFACTS}/partial-config"
mkdir -p "${PARTIAL_CONFIG_DIR}"
echo "${PARTIAL_CONFIG}" >"${PARTIAL_CONFIG_DIR}/controlplane.yaml"
echo "${PARTIAL_CONFIG}" >"${PARTIAL_CONFIG_DIR}/worker.yaml"

# Partial config, no secure boot
${ARTIFACTS}/talosctl cluster create \
  --provisioner=qemu \
  --controlplanes=1 \
  --workers=2 \
  --wait=false \
  --mtu=1430 \
  --memory=3072 \
  --memory-workers=3072 \
  --cpus=3 \
  --cpus-workers=3 \
  --with-uuid-hostnames \
  \
  --name test-1 \
  --cidr=172.20.0.0/24 \
  --no-masquerade-cidrs=172.21.0.0/24,172.22.0.0/24 \
  --input-dir="${PARTIAL_CONFIG_DIR}" \
  --vmlinuz-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/kernel-amd64" \
  --initrd-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/initramfs-amd64.xz"

# Kernel Args, no secure boot
${ARTIFACTS}/talosctl cluster create \
  --provisioner=qemu \
  --controlplanes=1 \
  --workers=${KERNEL_ARGS_WORKERS_COUNT} \
  --wait=false \
  --mtu=1430 \
  --memory=3072 \
  --memory-workers=3072 \
  --cpus=3 \
  --cpus-workers=3 \
  --with-uuid-hostnames \
  \
  --name test-2 \
  --skip-injecting-config \
  --with-init-node \
  --cidr=172.21.0.0/24 \
  --no-masquerade-cidrs=172.20.0.0/24,172.22.0.0/24 \
  --extra-boot-kernel-args "siderolink.api=grpc://$LOCAL_IP:8090?jointoken=${JOIN_TOKEN} talos.events.sink=[fdae:41e4:649b:9303::1]:8090 talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092" \
  --vmlinuz-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/kernel-amd64" \
  --initrd-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/initramfs-amd64.xz"

# Prepare schematic with kernel args for secure boot
SECURE_BOOT_SCHEMATIC=$(
  cat <<EOF
customization:
  extraKernelArgs:
    - siderolink.api=grpc://$LOCAL_IP:8090?jointoken=${JOIN_TOKEN}
    - talos.events.sink=[fdae:41e4:649b:9303::1]:8090
    - talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092
  systemExtensions:
    officialExtensions:
      - siderolabs/hello-world-service
EOF
)

SECURE_BOOT_SCHEMATIC_ID=$(curl -X POST --data-binary "${SECURE_BOOT_SCHEMATIC}" https://factory.talos.dev/schematics | jq -r '.id')

if [[ "${ENABLE_SECUREBOOT}" == "true" ]]; then
  # Kernel args, secure boot
  ${ARTIFACTS}/talosctl cluster create \
    --provisioner=qemu \
    --controlplanes=1 \
    --workers=1 \
    --wait=false \
    --mtu=1430 \
    --memory=3072 \
    --memory-workers=3072 \
    --cpus=3 \
    --cpus-workers=3 \
    --with-uuid-hostnames \
    \
    --name test-3 \
    --skip-injecting-config \
    --with-init-node \
    --cidr=172.22.0.0/24 \
    --no-masquerade-cidrs=172.20.0.0/24,172.21.0.0/24 \
    --with-tpm2 \
    --iso-path="https://factory.talos.dev/image/${SECURE_BOOT_SCHEMATIC_ID}/v${TALOS_VERSION}/metal-amd64-secureboot.iso" \
    --disk-encryption-key-types=tpm
fi

sleep 5

# Run the integration test.

SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
  ${ARTIFACTS}/integration-test-linux-amd64 \
  --endpoint https://my-instance.localhost:8099 \
  --talos-version=${TALOS_VERSION} \
  --omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
  --expected-machines=8 `# equal to the masters+workers above` \
  ${INTEGRATION_TEST_ARGS:-}

if [ "${INTEGRATION_RUN_E2E_TEST:-true}" == "true" ]; then
  # Run the e2e test.
  # the e2e tests are in a submodule and need to be executed in a container with playwright dependencies
  cd internal/e2e-tests/
  docker buildx build --load . -t e2etest
  docker run --rm \
    -e AUTH_PASSWORD="$AUTH_PASSWORD" \
    -e AUTH_USERNAME="$AUTH_USERNAME" \
    -e BASE_URL=$BASE_URL \
    -e VIDEO_DIR="$VIDEO_DIR" \
    --network=host \
    e2etest
fi

# No cleanup here, as it runs in the CI as a container in a pod.
