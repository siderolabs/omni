#!/usr/bin/env bash

# Copyright (c) 2025 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Load common functions and variables.
source ./hack/test/common.sh

# Machine Counts: 3 machines in total
TOTAL_MACHINES=3

PARTIAL_CONFIG_MACHINES=1 # 1 machine: siderolink via partial config, UKI, no secure boot
NON_UKI_MACHINES=1        # 1 machine: siderolink via kernel args, non-UKI, no secure boot

KERNEL_ARGS_MACHINES=1 # 1 machine: siderolink via kernel args, UKI, no secure boot
SECURE_BOOT_MACHINES=0 # 0 machines: secure boot, UKI, siderolink via kernel args

if [[ $((PARTIAL_CONFIG_MACHINES + NON_UKI_MACHINES + KERNEL_ARGS_MACHINES + SECURE_BOOT_MACHINES)) -ne $TOTAL_MACHINES ]]; then
  echo "Error: unexpected total machine count, exiting" >&2
  exit 1
fi

function cleanup() {
  # Copy the machine and related logs from ~/.talos/clusters to the test outputs dir.
  mkdir -p "${TEST_OUTPUTS_DIR}/clusters/"
  (cd ~/.talos/clusters && find . -name "*.log" -print0 | xargs -0 cp -p --parents -t "${TEST_OUTPUTS_DIR}/clusters") || true

  common_cleanup
  vault_cleanup
  minio_cleanup
}

trap cleanup EXIT SIGINT

# Download required artifacts.
prepare_artifacts

# Build registry mirror args.
configure_registry_mirrors

# Start Vault.
prepare_vault

# Start MinIO server.
prepare_minio access_key="access" secret_key="secret123"

# Prepare omni config.
prepare_omni_config

PARTIAL_CONFIG_SCHEMATIC_ID=$(prepare_partial_config)
KERNEL_ARGS_SCHEMATIC_ID=$(prepare_kernel_args_schematic)

# Create machines.
if [ "${CREATE_QEMU_MACHINES:-true}" == "true" ]; then
  create_machines name=test-partial-config count=${PARTIAL_CONFIG_MACHINES} cidr=172.20.0.0/24 secure_boot=false uki=true use_partial_config=true talos_version="${TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  create_machines name=test-kernel-args count=${KERNEL_ARGS_MACHINES} cidr=172.21.0.0/24 secure_boot=false uki=true use_partial_config=false talos_version="${TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  create_machines name=test-secure-boot count=${SECURE_BOOT_MACHINES} cidr=172.22.0.0/24 secure_boot=true uki=true use_partial_config=false talos_version="${TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  create_machines name=test-non-uki count=${NON_UKI_MACHINES} cidr=172.23.0.0/24 secure_boot=false uki=false use_partial_config=false talos_version="${TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
fi

SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
  nice -n 10 "${ARTIFACTS}"/omni-linux-amd64 --config-path "${OMNI_CONFIG}" \
  --etcd-embedded-unsafe-fsync=true \
  --etcd-backup-s3 \
  "${REGISTRY_MIRROR_FLAGS[@]}" &

# Run the e2e test.
# the e2e tests are in a submodule and need to be executed in a container with playwright dependencies
cd frontend/
docker buildx build --load . -t e2etest
docker run --rm \
  -e CI="$CI" \
  -e AUTH_PASSWORD="$AUTH_PASSWORD" \
  -e AUTH_USERNAME="$AUTH_USERNAME" \
  -e BASE_URL="$BASE_URL" \
  -e PROJECT="qemu" \
  -v "${TEST_OUTPUTS_DIR}/e2e/playwright-report:/tmp/test/playwright-report" \
  --network=host \
  e2etest

# No cleanup here, as it runs in the CI as a container in a pod.
