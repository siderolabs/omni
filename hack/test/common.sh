#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Omni is served from "https://my-instance.omni.localhost:8099"
# Exposed services through workload proxying follow the pattern: "https://sngmph-my-instance.proxy-us.omni.localhost:8099/"
# The TLS key and cert, hack/certs/localhost-key.pem and hack/certs/localhost.pem contain the SANs:
# - localhost
# - *.localhost
# - my-instance.omni.localhost
# - *.my-instance.omni.localhost
#
# Write "my-instance.omni.localhost" to /etc/hosts to avoid problems with the name resolution.
echo "127.0.0.1 my-instance.omni.localhost" | tee -a /etc/hosts
echo "127.0.0.1 omni.localhost" | tee -a /etc/hosts

# Settings.

# Pick the previous stable Omni release the upgrade tests start from, based on the branch.
# E.g. when cutting:
# - v1.8.1 on release-1.8 -> start from v1.8.0  (latest patch of the same line)
# - v1.7.4 on release-1.7 -> start from v1.7.3  (its own line, not the newer v1.8.0)
# - a dev build on main   -> start from v1.8.0  (latest stable overall)
# Otherwise an older release branch grabs a newer minor and the upgrade becomes a downgrade.
# CI checks out a detached HEAD, so the branch comes from the environment.
TARGET_BRANCH="${GITHUB_BASE_REF:-${GITHUB_REF_NAME:-$(git rev-parse --abbrev-ref HEAD)}}"
STABLE_OMNI_TAGS=$(git tag -l --sort=-version:refname "v*" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$')
if [[ "${TARGET_BRANCH}" =~ ^release-([0-9]+)\.([0-9]+)$ ]]; then
  LATEST_STABLE_OMNI=$(awk -F'[v.]' -v major="${BASH_REMATCH[1]}" -v minor="${BASH_REMATCH[2]}" \
    '($2 < major) || ($2 == major && $3 <= minor) { print; exit }' <<<"${STABLE_OMNI_TAGS}")
else
  LATEST_STABLE_OMNI=$(head -n 1 <<<"${STABLE_OMNI_TAGS}")
fi

export TALOS_VERSION=1.13.5
export KUBERNETES_VERSION=1.36.1
# To use in:
# - Omni upgrade tests, to prevent Talos changes interfering with Omni changes
# - Talos minor upgrade tests
export STABLE_TALOS_VERSION=1.12.8
export ANOTHER_OMNI_VERSION="${ANOTHER_OMNI_VERSION:-$LATEST_STABLE_OMNI}"
export ANOTHER_KUBERNETES_VERSION=1.35.5

export INTEGRATION_PREPARE_TEST_ARGS="${INTEGRATION_PREPARE_TEST_ARGS:-}"
export ARTIFACTS=_out
export JOIN_TOKEN=testonly
export TEST_OUTPUTS_DIR=${GITHUB_WORKSPACE:-/tmp}/integration-test
export SLEEP_AFTER_FAILURE=${SLEEP_AFTER_FAILURE:-0}
export CI="${CI}"
export BASE_URL=https://my-instance.omni.localhost:8099/
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=dev-o-token
export AUTH_USERNAME="${AUTH0_TEST_USERNAME}"
export AUTH_PASSWORD="${AUTH0_TEST_PASSWORD}"
export AUTH0_CLIENT_ID="${AUTH0_CLIENT_ID}"
export AUTH0_DOMAIN="${AUTH0_DOMAIN}"
export OMNI_CONFIG="${TEST_OUTPUTS_DIR}/config.yaml"
export MAX_USERS="${MAX_USERS:-0}"
export MAX_SERVICE_ACCOUNTS="${MAX_SERVICE_ACCOUNTS:-0}"
export MAX_REGISTERED_MACHINES="${MAX_REGISTERED_MACHINES:-0}"
export REGISTRY_MIRROR_FLAGS=()
export REGISTRY_MIRROR_CONFIG=""
export REGISTRY_MIRRORS_BODY=""
export IMPORTED_CLUSTER_ARGS=()
export IMAGE_FACTORY_PUBLIC_URL="https://factory.talos.dev"
export IMAGE_FACTORY_ENTERPRISE_URL="https://factory.siderolabs.com"
export WITH_IMAGE_FACTORY_ENTERPRISE="${WITH_IMAGE_FACTORY_ENTERPRISE:-false}"
export OMNI_IMAGE_FACTORY_BASE_URL="${IMAGE_FACTORY_PUBLIC_URL}"
export FACTORY_API_URL="${OMNI_IMAGE_FACTORY_BASE_URL}"
export FACTORY_IMAGE_URL="${OMNI_IMAGE_FACTORY_BASE_URL}"
export FACTORY_CURL_AUTH="${FACTORY_CURL_AUTH:-}"
# Basic-auth args for curl calls against the factory schematic API. Empty for the public factory, so it expands to nothing when unset.
FACTORY_CURL_ARGS=()

RUN_DIR=$(pwd)
export RUN_DIR

# Determine the local IP SideroLink API will listen on
LOCAL_IP=$(ip -o route get to 8.8.8.8 | sed -n 's/.*src \([0-9.]\+\).*/\1/p')
export LOCAL_IP

mkdir -p "$TEST_OUTPUTS_DIR"

export ENABLE_TALOS_PRERELEASE_VERSIONS=true
VAULT_DOCKER_IMAGE=hashicorp/vault:1.18
MINIO_DOCKER_IMAGE=minio/minio
export WIREGUARD_IP=$LOCAL_IP

if [[ "${CI:-false}" == "true" ]]; then
  WIREGUARD_IP=172.20.0.1
fi

export MACHINE_API_IP="${MACHINE_API_IP:-$LOCAL_IP}"

# Prepare schematic with kernel args
function prepare_kernel_args_schematic() {
  set_factory_curl_args

  KERNEL_ARGS_SCHEMATIC=$(envsubst <hack/test/templates/kernel-args-schematic.yaml)

  curl "${FACTORY_CURL_ARGS[@]}" -X POST --data-binary "${KERNEL_ARGS_SCHEMATIC}" "${FACTORY_API_URL}/schematics" | jq -r '.id'
}

# Build registry mirror args.
function configure_registry_mirrors() {
  if [[ "${CI:-false}" == "true" ]]; then
    REGISTRY_MIRROR_FLAGS=()
    REGISTRY_MIRRORS_BODY="  mirrors:
"

    for registry in docker.io k8s.gcr.io quay.io gcr.io ghcr.io registry.k8s.io factory.talos.dev; do
      service="registry-${registry//./-}.ci.svc"
      addr=$(python3 -c "import socket; print(socket.gethostbyname('${service}'))")

      REGISTRY_MIRROR_FLAGS+=("--registry-mirror=${registry}=http://${addr}:5000")
      REGISTRY_MIRRORS_BODY+="    - ${registry}=http://${addr}:5000"
      REGISTRY_MIRRORS_BODY+=$'\n'
    done
  fi
}

function common_cleanup() {
  cd "${RUN_DIR}"
  rm -rf "${ARTIFACTS}/omni.db" "${ARTIFACTS}/etcd/"

  if [[ $PARTIAL_CONFIG_SERVER_PID -ne 0 ]]; then
    kill -9 "$PARTIAL_CONFIG_SERVER_PID" || true
  fi

  # In CI, SUDO_USER is set to be "worker", and these output directories are used in the subsequent job steps.
  chown -R "${SUDO_USER:-$(whoami)}" "${TEST_OUTPUTS_DIR}"
  chown -R "${SUDO_USER:-$(whoami)}" "${ARTIFACTS}"
}

function prepare_artifacts() {
  [[ -f "${ARTIFACTS}/talosctl" ]] || (crane export ghcr.io/siderolabs/talosctl:latest | tar x -C "${ARTIFACTS}")
  [[ -f "${ARTIFACTS}/mc" ]] || curl -Lo "${ARTIFACTS}/mc" https://dl.min.io/client/mc/release/linux-amd64/mc
  chmod +x "${ARTIFACTS}/mc"

  echo "talosctl version:"
  "${ARTIFACTS}/talosctl" version --client

  # Create omnictl downloads directory (required by the server) and copy the omnictl binaries in it.
  mkdir -p omnictl
  cp -p "${ARTIFACTS}"/omnictl-* omnictl/
}

VAULT_CONTAINER_NAME=vault-dev
function prepare_vault() {
  # Start Vault.
  docker run --rm -d --cap-add=IPC_LOCK -p 8200:8200 -e VAULT_DEV_ROOT_TOKEN_ID="${VAULT_TOKEN}" --name "${VAULT_CONTAINER_NAME}" "${VAULT_DOCKER_IMAGE}"

  sleep 10

  # Load key into Vault.
  docker cp ./internal/backend/runtime/omni/testdata/pgp/old_key.private "${VAULT_CONTAINER_NAME}":/tmp/old_key.private
  docker exec -e VAULT_ADDR="${VAULT_ADDR}" -e VAULT_TOKEN="${VAULT_TOKEN}" "${VAULT_CONTAINER_NAME}" \
    vault kv put -mount=secret omni-private-key \
    private-key=@/tmp/old_key.private

  sleep 5
}

function vault_cleanup() {
  docker rm -f "${VAULT_CONTAINER_NAME}" || true
}

MINIO_CONTAINER_NAME=minio-dev
function prepare_minio() {
  # args: access_key, secret_key
  declare -A args
  for arg in "$@"; do
    key="${arg%%=*}"
    val="${arg#*=}"
    args["$key"]="$val"
  done

  local access_key="${args[access_key]}"
  local secret_key="${args[secret_key]}"

  mkdir -p "${TEST_OUTPUTS_DIR}/minio/data"

  docker run --rm -d -p 9000:9000 \
    -v "${TEST_OUTPUTS_DIR}/minio/data:/data" -e MINIO_ROOT_USER="$access_key" -e MINIO_ROOT_PASSWORD="$secret_key" \
    --name "${MINIO_CONTAINER_NAME}" "${MINIO_DOCKER_IMAGE}" \
    server /data

  sleep 2

  "${ARTIFACTS}/mc" alias set myminio http://127.0.0.1:9000 "$access_key" "$secret_key"
  "${ARTIFACTS}/mc" mb myminio/mybucket || true
}

function minio_cleanup() {
  docker rm -f "${MINIO_CONTAINER_NAME}" || true
}

function prepare_omni_config() {
  # Credentials are passed to Omni via OMNI_IMAGE_FACTORY_USERNAME/PASSWORD so they stay out of the config.yaml that is uploaded as a CI artifact.
  local registries_body=""

  if [[ -n "${OMNI_IMAGE_FACTORY_BASE_URL:-}" ]]; then
    registries_body+="  imageFactoryBaseURL: ${OMNI_IMAGE_FACTORY_BASE_URL}"$'\n'
  fi

  registries_body+="${REGISTRY_MIRRORS_BODY}"

  if [[ -n "${registries_body}" ]]; then
    REGISTRY_MIRROR_CONFIG="registries:"$'\n'"${registries_body}"
  else
    REGISTRY_MIRROR_CONFIG=""
  fi

  export REGISTRY_MIRROR_CONFIG

  envsubst <hack/test/templates/omni-config.yaml >"${OMNI_CONFIG}"
}

PARTIAL_CONFIG_SERVER_PID=0
function prepare_partial_config() {
  export PARTIAL_CONFIG_PORT=12345

  local partial_config_dir="${ARTIFACTS}/partial-config"
  mkdir -p "${partial_config_dir}"
  envsubst <hack/test/templates/partial-machine-config.yaml >"${partial_config_dir}/config.yaml"

  # Start a simple HTTP server to serve the partial config
  python3 -m http.server "$PARTIAL_CONFIG_PORT" --bind "0.0.0.0" --directory "$partial_config_dir" >/dev/null 2>&1 &
  PARTIAL_CONFIG_SERVER_PID=$! # capture the PID to kill it in cleanup

  set_factory_curl_args

  local schematic
  schematic=$(envsubst <hack/test/templates/partial-config-schematic.yaml)

  curl "${FACTORY_CURL_ARGS[@]}" -X POST --data-binary "${schematic}" "${FACTORY_API_URL}/schematics" | jq -r '.id'
}

function prepare_talos_image() {
  set_factory_curl_args

  local schematic
  schematic=$(cat hack/test/templates/talos-image-schematic.yaml)

  curl "${FACTORY_CURL_ARGS[@]}" -X POST --data-binary "${schematic}" "${FACTORY_API_URL}/schematics" | jq -r '.id'
}

# Prepare a schematic whose image carries the machine config baked in via embeddedMachineConfiguration.
# Machines booted from it arrive at Omni already carrying user documents on top of the SideroLink connection docs, so
# there is nothing to serve over HTTP.
function prepare_embedded_config() {
  set_factory_curl_args

  local schematic
  schematic=$(envsubst <hack/test/templates/embedded-config-schematic.yaml)

  curl "${FACTORY_CURL_ARGS[@]}" -X POST --data-binary "${schematic}" "${FACTORY_API_URL}/schematics" | jq -r '.id'
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

function create_machines() {
  # args: name, count, cidr, secure_boot (true/false), uki (true/false), use_partial_config (true/false), talos_version, kernel_args_schematic_id, partial_config_schematic_id
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
  local talos_version="${args[talos_version]}"
  local kernel_args_schematic_id="${args[kernel_args_schematic_id]}"
  local partial_config_schematic_id="${args[partial_config_schematic_id]}"

  local schematic_id="${kernel_args_schematic_id}"
  if [[ "${use_partial_config}" == "true" ]]; then
    schematic_id="${partial_config_schematic_id}"
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
    "--skip-injecting-extra-cmdline"
    "--wait=false"
  )

  if [[ "${uki}" == "false" ]]; then
    cluster_create_args+=("--with-uefi=false")
  fi

  if [[ "${secure_boot}" == "true" ]]; then
    cluster_create_args+=(
      "--with-tpm2"
      "--disk-encryption-key-types=tpm"
      "--iso-path=${FACTORY_IMAGE_URL}/image/${schematic_id}/v${talos_version}/metal-amd64-secureboot.iso"
    )
  else
    cluster_create_args+=("--iso-path=${FACTORY_IMAGE_URL}/image/${schematic_id}/v${talos_version}/metal-amd64.iso")
  fi

  "${ARTIFACTS}/talosctl" cluster create dev \
    "${cluster_create_args[@]}"
}

function create_talos_cluster { # args: name, cp_count, wk_count, cidr, talos_version, skip_kubeconfig (true/false), allow_scheduling_on_control_planes (true/false)
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
  local talos_version="${args[talos_version]}"
  local skip_kubeconfig="${args[skip_kubeconfig]:-false}"
  local allow_scheduling_on_control_planes="${args[allow_scheduling_on_control_planes]:-false}"

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
    "--talosconfig=${TEST_OUTPUTS_DIR}/${name}/talosconfig"
    "--skip-kubeconfig=${skip_kubeconfig}"
    "--skip-injecting-extra-cmdline"
    "--with-apply-config"
    "--with-bootloader"
    "--kubernetes-version=${KUBERNETES_VERSION}"
    "--talos-version=${talos_version}"
    "--install-image=factory.talos.dev/metal-installer/${schematic_id}:v${talos_version}"
    "--iso-path=https://factory.talos.dev/image/${schematic_id}/v${talos_version}/metal-amd64.iso"
  )

  if [[ "${allow_scheduling_on_control_planes}" == "true" ]]; then
    cluster_create_args+=("--config-patch-control-plane={\"cluster\":{\"allowSchedulingOnControlPlanes\":true}}")
  fi

  "${ARTIFACTS}/talosctl" cluster create dev \
    "${cluster_create_args[@]}" \
    "${REGISTRY_MIRROR_FLAGS[@]}"

  IMPORTED_CLUSTER_ARGS+=("--talos.config-path=${TEST_OUTPUTS_DIR}/${name}/talosconfig")
  IMPORTED_CLUSTER_ARGS+=("--talos.cluster-state-path=${HOME}/.talos/clusters/${name}/state.yaml")
}

function set_factory_curl_args() {
  FACTORY_CURL_ARGS=()
  if [[ -n "${FACTORY_CURL_AUTH:-}" ]]; then
    FACTORY_CURL_ARGS=(-u "${FACTORY_CURL_AUTH}")
  fi
}

function configure_image_factory() {
  if [[ "${WITH_IMAGE_FACTORY_ENTERPRISE}" != "true" ]]; then
    return
  fi

  # Configure env vars for Image Factory Enterprise
  : "${IMAGE_FACTORY_ENTERPRISE_USERNAME:?IMAGE_FACTORY_ENTERPRISE_USERNAME must be set when WITH_IMAGE_FACTORY_ENTERPRISE=true}"
  : "${IMAGE_FACTORY_ENTERPRISE_PASSWORD:?IMAGE_FACTORY_ENTERPRISE_PASSWORD must be set when WITH_IMAGE_FACTORY_ENTERPRISE=true}"
  export OMNI_IMAGE_FACTORY_BASE_URL="${IMAGE_FACTORY_ENTERPRISE_URL}"
  export OMNI_IMAGE_FACTORY_USERNAME="${IMAGE_FACTORY_ENTERPRISE_USERNAME}"
  export OMNI_IMAGE_FACTORY_PASSWORD="${IMAGE_FACTORY_ENTERPRISE_PASSWORD}"

  export FACTORY_API_URL="${OMNI_IMAGE_FACTORY_BASE_URL}"
  export FACTORY_CURL_AUTH="${OMNI_IMAGE_FACTORY_USERNAME}:${OMNI_IMAGE_FACTORY_PASSWORD}"

  local proto="${OMNI_IMAGE_FACTORY_BASE_URL%%://*}"
  local host="${OMNI_IMAGE_FACTORY_BASE_URL#*://}"
  export FACTORY_IMAGE_URL="${proto}://${OMNI_IMAGE_FACTORY_USERNAME}:${OMNI_IMAGE_FACTORY_PASSWORD}@${host}"
}

# No cleanup here, as it runs in the CI as a container in a pod.
