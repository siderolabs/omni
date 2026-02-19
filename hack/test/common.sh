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
LATEST_STABLE_OMNI=$(git tag -l --sort=-version:refname HEAD "v*" | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1)

export TALOS_VERSION=1.12.4
export KUBERNETES_VERSION=1.35.1
# To use in:
# - Omni upgrade tests, to prevent Talos changes interfering with Omni changes
# - Talos minor upgrade tests
export STABLE_TALOS_VERSION=1.11.6
export ANOTHER_OMNI_VERSION="${ANOTHER_OMNI_VERSION:-$LATEST_STABLE_OMNI}"
export ANOTHER_KUBERNETES_VERSION=1.34.4

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
export REGISTRY_MIRROR_FLAGS=()
export REGISTRY_MIRROR_CONFIG=""
export IMPORTED_CLUSTER_ARGS=()

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

# Prepare schematic with kernel args
function prepare_kernel_args_schematic() {
  KERNEL_ARGS_SCHEMATIC=$(envsubst <hack/test/templates/kernel-args-schematic.yaml)

  curl -X POST --data-binary "${KERNEL_ARGS_SCHEMATIC}" https://factory.talos.dev/schematics | jq -r '.id'
}

# Build registry mirror args.
function configure_registry_mirrors() {
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
    -v "${TEST_OUTPUTS_DIR}/minio/data:/data" -e MINIO_ACCESS_KEY="$access_key" -e MINIO_SECRET_KEY="$secret_key" \
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

  local schematic
  schematic=$(envsubst <hack/test/templates/partial-config-schematic.yaml)

  curl -X POST --data-binary "${schematic}" https://factory.talos.dev/schematics | jq -r '.id'
}

function prepare_talos_image() {
  local schematic
  schematic=$(cat hack/test/templates/talos-image-schematic.yaml)

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
      "--iso-path=https://factory.talos.dev/image/${schematic_id}/v${talos_version}/metal-amd64-secureboot.iso"
    )
  else
    cluster_create_args+=("--iso-path=https://factory.talos.dev/image/${schematic_id}/v${talos_version}/metal-amd64.iso")
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

# No cleanup here, as it runs in the CI as a container in a pod.
