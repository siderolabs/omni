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
LATEST_STABLE_OMNI=$(git tag -l --sort=-version:refname HEAD "v*" | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1)

TALOS_VERSION=1.11.2
ENABLE_TALOS_PRERELEASE_VERSIONS=false
ANOTHER_OMNI_VERSION="${ANOTHER_OMNI_VERSION:-$LATEST_STABLE_OMNI}"
KUBERNETES_VERSION=1.34.1

ARTIFACTS=_out
JOIN_TOKEN=testonly
RUN_DIR=$(pwd)
ENABLE_SECUREBOOT=${ENABLE_SECUREBOOT:-false}
TALEMU_CONTAINER_NAME=talemu
TALEMU_INFRA_PROVIDER_IMAGE=ghcr.io/siderolabs/talemu-infra-provider:latest
TEST_OUTPUTS_DIR=${GITHUB_WORKSPACE:-/tmp}/integration-test
INTEGRATION_PREPARE_TEST_ARGS="${INTEGRATION_PREPARE_TEST_ARGS:-}"

# Machine Counts: 8 machines in total
TOTAL_MACHINES=8

PARTIAL_CONFIG_MACHINES=3 # 3 machines: siderolink via partial config, UKI, no secure boot
NON_UKI_MACHINES=2        # 2 machines: siderolink via kernel args, non-UKI, no secure boot

KERNEL_ARGS_MACHINES=3 # 3 machines: siderolink via kernel args, UKI, no secure boot
SECURE_BOOT_MACHINES=0 # 0 machines: secure boot, UKI, siderolink via kernel args

if [[ "${ENABLE_SECUREBOOT}" == "true" ]]; then
  KERNEL_ARGS_MACHINES=1 # 1 machine: siderolink via kernel args, UKI, no secure boot
  SECURE_BOOT_MACHINES=2 # 2 machines: siderolink via kernel args, UKI, secure boot
fi

if [[ $((PARTIAL_CONFIG_MACHINES + NON_UKI_MACHINES + KERNEL_ARGS_MACHINES + SECURE_BOOT_MACHINES)) -ne $TOTAL_MACHINES ]]; then
  echo "Error: unexpected total machine count, exiting" >&2
  exit 1
fi

mkdir -p $TEST_OUTPUTS_DIR

# Download required artifacts.

mkdir -p ${ARTIFACTS}
chown -R ${SUDO_USER:-$(whoami)} ${ARTIFACTS}

[ -f ${ARTIFACTS}/integration-test-linux-amd64 ] || curl -L

[ -f ${ARTIFACTS}/talosctl ] || (crane export ghcr.io/siderolabs/talosctl:latest | tar x -C ${ARTIFACTS})

# Determine the local IP SideroLink API will listen on
LOCAL_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')

# Prepare schematic with kernel args
KERNEL_ARGS_SCHEMATIC=$(
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
KERNEL_ARGS_SCHEMATIC_ID=$(curl -X POST --data-binary "${KERNEL_ARGS_SCHEMATIC}" https://factory.talos.dev/schematics | jq -r '.id')

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

PARTIAL_CONFIG_SERVER_PID=0

function cleanup() {
  cd "${RUN_DIR}"
  rm -rf ${ARTIFACTS}/omni.db ${ARTIFACTS}/etcd/

  if docker ps -a --format '{{.Names}}' | grep -q "^${TALEMU_CONTAINER_NAME}$"; then
    docker stop ${TALEMU_CONTAINER_NAME} || true
    docker logs ${TALEMU_CONTAINER_NAME} &>$TEST_OUTPUTS_DIR/${TALEMU_CONTAINER_NAME}.log || true
    docker rm -f ${TALEMU_CONTAINER_NAME} || true
  fi

  if [ $PARTIAL_CONFIG_SERVER_PID -ne 0 ]; then
    kill -9 $PARTIAL_CONFIG_SERVER_PID || true
  fi

  chown -R "${SUDO_USER:-$(whoami)}" "${TEST_OUTPUTS_DIR}"
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

export CI="${CI}"
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN=dev-o-token
export AUTH_USERNAME="${AUTH0_TEST_USERNAME}"
export AUTH_PASSWORD="${AUTH0_TEST_PASSWORD}"
export BASE_URL=https://my-instance.localhost:8099/
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
    stopLBsAfter: 15s # use a short duration to test turning lazy LBs on/off
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
  disableControllerRuntimeCache: false
" >${OMNI_CONFIG}

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
    --omni.kubernetes-version=${KUBERNETES_VERSION} \
    --omni.omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
    --omni.expected-machines=30 \
    --omni.provision-config-file=hack/test/provisionconfig.yaml \
    --omni.output-dir="${TEST_OUTPUTS_DIR}" \
    --omni.run-stats-check \
    --omni.embedded \
    --omni.config-path=${OMNI_CONFIG} \
    --omni.log-output=${TEST_OUTPUTS_DIR}/omni-emulator.log \
    --test.coverprofile=${ARTIFACTS}/coverage-emulator.txt \
    --test.timeout 10m \
    --test.parallel 10 \
    --test.failfast \
    --test.v \
    ${TALEMU_TEST_ARGS:-}

  docker rm -f "$PROMETHEUS_CONTAINER"
fi

function prepare_partial_config() {
  # Prepare partial machine config
  local port=12345

  local partial_config
  partial_config=$(
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

  local partial_config_dir="${ARTIFACTS}/partial-config"
  mkdir -p "${partial_config_dir}"
  echo "${partial_config}" >"${partial_config_dir}/config.yaml"

  # Start a simple HTTP server to serve the partial config
  python3 -m http.server $port --bind "$LOCAL_IP" --directory "$partial_config_dir" >/dev/null 2>&1 &
  PARTIAL_CONFIG_SERVER_PID=$! # capture the PID to kill it in cleanup

  local schematic
  schematic=$(
    cat <<EOF
customization:
  extraKernelArgs:
    - talos.config=http://$LOCAL_IP:$port/config.yaml
  systemExtensions:
    officialExtensions:
      - siderolabs/hello-world-service
EOF
  )

  curl -X POST --data-binary "${schematic}" https://factory.talos.dev/schematics | jq -r '.id'
}

PARTIAL_CONFIG_SCHEMATIC_ID=$(prepare_partial_config)

function prepare_talos_image() {

  local schematic
  schematic=$(
    cat <<EOF
customization:
  extraKernelArgs:
    - console=tty0
    - console=ttyS0
  systemExtensions:
    officialExtensions:
      - siderolabs/hello-world-service
EOF
  )

  curl -X POST --data-binary "${schematic}" https://factory.talos.dev/schematics | jq -r '.id'
}

function generate_non_masquerade_cidrs() {
  local exclude="$1"
  local found=false
  local result=()

  for i in {20..29}; do
    cidr="172.$i.0.0/24"
    if [[ "$cidr" == "$exclude" ]]; then
      found=true
      continue
    fi
    result+=("$cidr")
  done

  if [[ $found == false ]]; then
    echo "Error: '$exclude' is not in the 172.20.0.0/24..172.29.0.0/24 range" >&2
    return 1
  fi

  (
    IFS=,
    echo "${result[*]}"
  )
}

function create_machines() { # args: name, count, cidr, secure_boot (true/false), uki (true/false), use_partial_config (true/false)
  declare -A args
  for arg in "$@"; do
    key="${arg%%=*}"
    val="${arg#*=}"
    args["$key"]="$val"
  done

  local name="${args[name]}"
  local count="${args[count]}"
  local cidr="${args[cidr]}"
  local secure_boot="${args[secure_boot]}"
  local uki="${args[uki]}"
  local use_partial_config="${args[use_partial_config]}"

  local schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}"
  if [[ "${use_partial_config}" == "true" ]]; then
    schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  fi

  if [[ "${secure_boot}" == "true" && "${uki}" == "false" ]]; then
    echo "Error: secure_boot cannot be true when uki is false, as it always uses UKI" >&2
    return 1
  fi

  if [[ "${count}" -le 0 ]]; then
    return
  fi

  local non_masquerade_cidrs
  non_masquerade_cidrs=$(generate_non_masquerade_cidrs "${cidr}")

  local cluster_create_args=(
    "--provisioner=qemu"
    "--name=${name}"
    "--controlplanes=${count}"
    "--workers=0"
    "--mtu=1430"
    "--memory=3072"
    "--memory-workers=3072"
    "--cpus=3"
    "--cpus-workers=3"
    "--with-uuid-hostnames"
    "--cidr=${cidr}"
    "--no-masquerade-cidrs=${non_masquerade_cidrs}"
    "--skip-injecting-config"
    "--wait=false"
  )

  if [[ "${uki}" == "false" ]]; then
    cluster_create_args+=("--with-uefi=false")
  fi

  if [[ "${secure_boot}" == "true" ]]; then
    cluster_create_args+=(
      "--with-tpm2"
      "--disk-encryption-key-types=tpm"
      "--iso-path=https://factory.talos.dev/image/${schematic_id}/v${TALOS_VERSION}/metal-amd64-secureboot.iso"
    )
  else
    cluster_create_args+=("--iso-path=https://factory.talos.dev/image/${schematic_id}/v${TALOS_VERSION}/metal-amd64.iso")
  fi

  ${ARTIFACTS}/talosctl cluster create \
    "${cluster_create_args[@]:-}"
}

IMPORTED_CLUSTER_ARGS=()

function create_talos_cluster { # args: name, cp_count, wk_count, cidr
    declare -A args
    for arg in "$@"; do
      key="${arg%%=*}"
      val="${arg#*=}"
      args["$key"]="$val"
    done

    local name="${args[name]}"
    local cp_count="${args[cp_count]}"
    local wk_count="${args[wk_count]:-0}"
    local cidr="${args[cidr]}"

    local non_masquerade_cidrs
    non_masquerade_cidrs=$(generate_non_masquerade_cidrs "${cidr}")

    local schematic_id
    schematic_id=$(prepare_talos_image)

    local cluster_create_args=(
      "--provisioner=qemu"
      "--name=${name}"
      "--controlplanes=${cp_count}"
      "--workers=${wk_count}"
      "--mtu=1430"
      "--memory=3072"
      "--memory-workers=3072"
      "--cpus=3"
      "--cpus-workers=3"
      "--with-uuid-hostnames"
      "--cidr=${cidr}"
      "--no-masquerade-cidrs=${non_masquerade_cidrs}"
      "--talosconfig=$TEST_OUTPUTS_DIR/$name/talosconfig"
      "--skip-kubeconfig"
      "--with-apply-config"
      "--with-bootloader"
      "--kubernetes-version=$KUBERNETES_VERSION"
      "--talos-version=$TALOS_VERSION"
      "--install-image=factory.talos.dev/metal-installer/${schematic_id}:v$TALOS_VERSION"
      "--iso-path=https://factory.talos.dev/image/${schematic_id}/v$TALOS_VERSION/metal-amd64.iso"
    )

    # shellcheck disable=SC2068
    ${ARTIFACTS}/talosctl cluster create \
      ${cluster_create_args[@]:-} \
      ${REGISTRY_MIRROR_FLAGS[@]:-}

    IMPORTED_CLUSTER_ARGS+=("--talos.config-path=$TEST_OUTPUTS_DIR/$name/talosconfig")
    IMPORTED_CLUSTER_ARGS+=("--talos.cluster-state-path=$HOME/.talos/clusters/$name/state.yaml")
}

# Create machines.
create_machines name=test-partial-config count=${PARTIAL_CONFIG_MACHINES} cidr=172.20.0.0/24 secure_boot=false uki=true use_partial_config=true
create_machines name=test-kernel-args count=${KERNEL_ARGS_MACHINES} cidr=172.21.0.0/24 secure_boot=false uki=true use_partial_config=false
create_machines name=test-secure-boot count=${SECURE_BOOT_MACHINES} cidr=172.22.0.0/24 secure_boot=true uki=true use_partial_config=false
create_machines name=test-non-uki count=${NON_UKI_MACHINES} cidr=172.23.0.0/24 secure_boot=false uki=false use_partial_config=false

if [ "${CREATE_TALOS_CLUSTER:-false}" == "true" ]; then
  create_talos_cluster name=test-cluster-import cp_count=1 wk_count=1 cidr=172.28.0.0/24
fi

# todo: add --omni.sleep-after-failure="${SLEEP_AFTER_FAILURE}" \ below when ANOTHER_OMNI_VERSION starts to support it
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
    --omni.kubernetes-version=${KUBERNETES_VERSION} \
    --omni.omnictl-path=/_out/omnictl-linux-amd64 \
    --omni.expected-machines=${TOTAL_MACHINES} \
    --omni.embedded \
    --omni.config-path=/config.yaml \
    --omni.log-output=/outputs/omni-upgrade-prepare.log \
    --omni.ignore-unknown-fields \
    --test.failfast \
    --test.v \
    ${INTEGRATION_PREPARE_TEST_ARGS:-}

fi

# Run the integration test.
SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
  SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
  ${ARTIFACTS}/integration-test-linux-amd64 \
  --omni.talos-version=${TALOS_VERSION} \
  --omni.kubernetes-version=${KUBERNETES_VERSION} \
  --omni.omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
  --omni.expected-machines=${TOTAL_MACHINES} \
  --omni.embedded \
  --omni.config-path=${OMNI_CONFIG} \
  --omni.log-output=${TEST_OUTPUTS_DIR}/omni-integration.log \
  --test.failfast \
  --test.coverprofile=${ARTIFACTS}/coverage-integration.txt \
  --test.v \
  ${IMPORTED_CLUSTER_ARGS[@]:-} \
  ${INTEGRATION_TEST_ARGS:-}

if [ "${INTEGRATION_RUN_E2E_TEST:-true}" == "true" ]; then
  # write partial omni config
  echo "---
  services:
    api:
      endpoint: 0.0.0.0:8099
      advertisedURL: ${BASE_URL}
      certFile: hack/certs/localhost.pem
      keyFile: hack/certs/localhost-key.pem" >${TEST_OUTPUTS_DIR}/e2e-config.yaml

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
    "${REGISTRY_MIRROR_FLAGS[@]}" &

  # Run the e2e test.
  # the e2e tests are in a submodule and need to be executed in a container with playwright dependencies
  cd frontend/
  docker buildx build --load . -t e2etest
  docker run --rm \
    -e CI="$CI" \
    -e AUTH_PASSWORD="$AUTH_PASSWORD" \
    -e AUTH_USERNAME="$AUTH_USERNAME" \
    -e BASE_URL=$BASE_URL \
    -v ${TEST_OUTPUTS_DIR}/e2e/playwright-report:/tmp/test/playwright-report \
    --network=host \
    e2etest
fi

# No cleanup here, as it runs in the CI as a container in a pod.
