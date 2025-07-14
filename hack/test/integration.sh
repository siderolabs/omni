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

TALOS_VERSION=1.10.2
ENABLE_TALOS_PRERELEASE_VERSIONS=false
ANOTHER_OMNI_VERSION="${ANOTHER_OMNI_VERSION:-latest}"

ARTIFACTS=_out
JOIN_TOKEN=testonly
RUN_DIR=$(pwd)
ENABLE_SECUREBOOT=${ENABLE_SECUREBOOT:-false}
KERNEL_ARGS_WORKERS_COUNT=2
TALEMU_CONTAINER_NAME=talemu
TALEMU_INFRA_PROVIDER_IMAGE=ghcr.io/siderolabs/talemu-infra-provider:latest
TEST_OUTPUTS_DIR=/tmp/integration-test
INTEGRATION_PREPARE_TEST_ARGS="${INTEGRATION_PREPARE_TEST_ARGS:-}"

mkdir -p $TEST_OUTPUTS_DIR

# Download required artifacts.

mkdir -p ${ARTIFACTS}

[ -f ${ARTIFACTS}/talosctl ] || (crane export ghcr.io/siderolabs/talosctl:latest | tar x -C ${ARTIFACTS})

# Determine the local IP SideroLink API will listen on
LOCAL_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')

# Prepare schematic with kernel args
SCHEMATIC=$(
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

SCHEMATIC_ID=$(curl -X POST --data-binary "${SCHEMATIC}" https://factory.talos.dev/schematics | jq -r '.id')

# Build registry mirror args.

if [[ "${ENABLE_SECUREBOOT}" == "false" ]]; then
  KERNEL_ARGS_WORKERS_COUNT=4
fi

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
    REGISTRY_MIRROR_CONFIG+="    - ${registry}=http://${addr}:5000\n"e
  done
else
  # use the value from the environment, if present
  REGISTRY_MIRROR_FLAGS=("${REGISTRY_MIRROR_FLAGS:-}")
  REGISTRY_MIRROR_CONFIG="${REGISTRY_MIRROR_CONFIG:-}"
fi

function cleanup() {
  cd "${RUN_DIR}"
  rm -rf ${ARTIFACTS}/omni.db ${ARTIFACTS}/etcd/

  if docker ps -a --format '{{.Names}}' | grep -q "^${TALEMU_CONTAINER_NAME}$"; then
    docker stop ${TALEMU_CONTAINER_NAME} || true
    docker logs ${TALEMU_CONTAINER_NAME} &>$TEST_OUTPUTS_DIR/${TALEMU_CONTAINER_NAME}.log || true
    docker rm -f ${TALEMU_CONTAINER_NAME} || true
  fi
}

trap cleanup EXIT SIGINT

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
export OMNI_CONFIG="${TEST_OUTPUTS_DIR}/config.yaml"

# Create omnictl downloads directory (required by the server) and copy the omnictl binaries in it.
mkdir -p omnictl
cp -p ${ARTIFACTS}/omnictl-* omnictl/

echo "---
services:
  api:
    endpoint: 0.0.0.0:8099
    advertisedURL: ${BASE_URL}
    certFile: hack/certs/localhost.pem
    keyFile: hack/certs/localhost-key.pem
  metrics:
    endpoint: 0.0.0.0:2122
  kubernetesProxy:
    endpoint: localhost:8095
  siderolink:
    wireGuard:
      endpoint: ${LOCAL_IP}:50180
      advertisedEndpoint: ${LOCAL_IP}:50180
    joinTokensMode: strict
  machineAPI:
    endpoint: ${LOCAL_IP}:8090
    advertisedURL: grpc://${LOCAL_IP}:8090
  workloadProxy:
    enabled: true
    subdomain: proxy-us
auth:
  auth0:
    enabled: true
    clientID: ${AUTH0_CLIENT_ID}
    domain: ${AUTH0_DOMAIN}
    initialUsers:
      - ${AUTH_USERNAME}
etcdBackup:
  s3Enabled: true
${REGISTRY_MIRROR_CONFIG}

storage:
  vault:
    url: http://127.0.0.1:8200
    token: dev-o-token
  secondary:
    path: ${ARTIFACTS}/secondary-storage/bolt.db
  default:
    kind: etcd
    etcd:
      endpoints:
        - http://localhost:2379
      dialKeepAliveTime: 30s
      dialKeepAliveTimeout: 5s
      caFile: etcd/ca.crt
      certFile: etcd/client.crt
      keyFile: etcd/client.key
      embedded: true
      privateKeySource: \"vault://secret/omni-private-key\"
      publicKeyFiles:
        - internal/backend/runtime/omni/testdata/pgp/new_key.public
      embeddedUnsafeFsync: true
      embeddedDBPath: ${ARTIFACTS}/etcd/
logs:
  machine:
    storage:
      enabled: true
      path: /tmp/omni-data/machine-log
      flushPeriod: 10m
      flushJitter: 0.1
  audit:
    path: ${TEST_OUTPUTS_DIR}/audit-log
features:
  enableTalosPreReleaseVersions: ${ENABLE_TALOS_PRERELEASE_VERSIONS}
  enableConfigDataCompression: true
  enableBreakGlassConfigs: true
  enableClusterImport: true
  disableControllerRuntimeCache: false" > ${OMNI_CONFIG}

if [[ "${RUN_TALEMU_TESTS:-false}" == "true" ]]; then
  PROMETHEUS_CONTAINER=$(docker run --network host -p "9090:9090" -v "$(pwd)/hack/compose/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml" -it --rm -d prom/prometheus)

  docker pull "${TALEMU_INFRA_PROVIDER_IMAGE}"
  docker run --name $TALEMU_CONTAINER_NAME \
    --network host --cap-add=NET_ADMIN \
    -it -d \
    "${TALEMU_INFRA_PROVIDER_IMAGE}" \
    --create-service-account \
    --omni-api-endpoint="https://$LOCAL_IP:8099"

  SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
  SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
    ${ARTIFACTS}/integration-test-linux-amd64 \
    --omni.talos-version=${TALOS_VERSION} \
    --omni.omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
    --omni.expected-machines=30 \
    --omni.provision-config-file=hack/test/provisionconfig.yaml \
    --omni.output-dir="${TEST_OUTPUTS_DIR}" \
    --omni.run-stats-check \
    --omni.embedded \
    --omni.config-path ${OMNI_CONFIG} \
    --omni.log-output ${TEST_OUTPUTS_DIR}/omni-emulator.log \
    --test.coverprofile=${ARTIFACTS}/coverage-emulator.txt \
    --test.timeout 10m \
    --test.parallel 10 \
    --test.failfast \
    --test.v \
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
    --iso-path="https://factory.talos.dev/image/${SCHEMATIC_ID}/v${TALOS_VERSION}/metal-amd64-secureboot.iso" \
    --disk-encryption-key-types=tpm
fi

if [ -n "$ANOTHER_OMNI_VERSION" ] && [ -n "$INTEGRATION_PREPARE_TEST_ARGS" ]; then
  docker run \
    --cap-add=NET_ADMIN \
    --device=/dev/net/tun \
    -v $(pwd)/hack/certs:/hack/certs \
    -v $(pwd)/${ARTIFACTS}:/_out/ \
    -v ${TEST_OUTPUTS_DIR}:/outputs/ \
    -v ${OMNI_CONFIG}:/config.yaml \
    -v $(pwd)/omnictl/:/omnictl/ \
    -v $(pwd)/internal/backend/runtime/omni/testdata/pgp/:/internal/backend/runtime/omni/testdata/pgp/ \
    -e SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
    -e SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
    --network host \
    ghcr.io/siderolabs/omni-integration-test:${ANOTHER_OMNI_VERSION} \
    --omni.talos-version=${TALOS_VERSION} \
    --omni.omnictl-path=/_out/omnictl-linux-amd64 \
    --omni.expected-machines=8 \
    --omni.embedded \
    --omni.config-path /config.yaml \
    --omni.log-output=/outputs/omni-upgrade-prepare.log \
    --test.failfast \
    --test.v \
    ${INTEGRATION_PREPARE_TEST_ARGS:-}
fi

# Run the integration test.

SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
  ${ARTIFACTS}/integration-test-linux-amd64 \
  --omni.talos-version=${TALOS_VERSION} \
  --omni.omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
  --omni.expected-machines=8 `# equal to the masters+workers above` \
  --omni.embedded \
  --omni.config-path ${OMNI_CONFIG} \
  --omni.log-output=${TEST_OUTPUTS_DIR}/omni-integration.log \
  --test.failfast \
  --test.coverprofile=${ARTIFACTS}/coverage-integration.txt \
  --test.v \
  ${INTEGRATION_TEST_ARGS:-}

if [ "${INTEGRATION_RUN_E2E_TEST:-true}" == "true" ]; then
  # write partial omni config
  echo "---
  services:
    api:
      endpoint: 0.0.0.0:8099
      advertisedURL: ${BASE_URL}
      certFile: hack/certs/localhost.pem
      keyFile: hack/certs/localhost-key.pem" > ${TEST_OUTPUTS_DIR}/e2e-config.yaml


  SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
    nice -n 10 ${ARTIFACTS}/omni-linux-amd64 --config-path ${TEST_OUTPUTS_DIR}/e2e-config.yaml \
    --siderolink-wireguard-advertised-addr $LOCAL_IP:50180 \
    --siderolink-api-advertised-url "grpc://$LOCAL_IP:8090" \
    --auth-auth0-enabled true \
    --auth-auth0-client-id "${AUTH0_CLIENT_ID}" \
    --auth-auth0-domain "${AUTH0_DOMAIN}" \
    --initial-users "${AUTH_USERNAME}" \
    --private-key-source "vault://secret/omni-private-key" \
    --public-key-files "internal/backend/runtime/omni/testdata/pgp/new_key.public" \
    --etcd-embedded-unsafe-fsync=true \
    --etcd-backup-s3 \
    --join-tokens-mode strict \
    --audit-log-dir ${TEST_OUTPUTS_DIR}/audit-log \
    --config-data-compression-enabled \
    --enable-talos-pre-release-versions="${ENABLE_TALOS_PRERELEASE_VERSIONS}" \
    --enable-cluster-import \
    "${REGISTRY_MIRROR_FLAGS[@]}"&

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
