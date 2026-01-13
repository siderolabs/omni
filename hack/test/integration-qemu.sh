#!/usr/bin/env bash

# Copyright (c) 2025 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Load common functions and variables.
source ./hack/test/common.sh

QEMU_TALOS_VERSION=${TALOS_VERSION}
WITH_QEMU_TALOS_VERSION_OVERRIDE=${WITH_QEMU_TALOS_VERSION_OVERRIDE:-false}
if [[ $WITH_QEMU_TALOS_VERSION_OVERRIDE == "true" ]]; then
  QEMU_TALOS_VERSION=$STABLE_TALOS_VERSION
fi

ENABLE_SECUREBOOT=${ENABLE_SECUREBOOT:-false}

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
  create_machines name=test-partial-config count=${PARTIAL_CONFIG_MACHINES} cidr=172.20.0.0/24 secure_boot=false uki=true use_partial_config=true talos_version="${QEMU_TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  create_machines name=test-kernel-args count=${KERNEL_ARGS_MACHINES} cidr=172.21.0.0/24 secure_boot=false uki=true use_partial_config=false talos_version="${QEMU_TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  create_machines name=test-secure-boot count=${SECURE_BOOT_MACHINES} cidr=172.22.0.0/24 secure_boot=true uki=true use_partial_config=false talos_version="${QEMU_TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
  create_machines name=test-non-uki count=${NON_UKI_MACHINES} cidr=172.23.0.0/24 secure_boot=false uki=false use_partial_config=false talos_version="${QEMU_TALOS_VERSION}" \
    kernel_args_schematic_id="${KERNEL_ARGS_SCHEMATIC_ID}" partial_config_schematic_id="${PARTIAL_CONFIG_SCHEMATIC_ID}"
fi

# Create a Talos cluster to be imported.
if [ "${CREATE_TALOS_CLUSTER:-false}" == "true" ]; then
  create_talos_cluster name=test-cluster-import cp_count=3 wk_count=3 cidr=172.28.0.0/24 talos_version="${QEMU_TALOS_VERSION}"
fi

if [ -n "$ANOTHER_OMNI_VERSION" ] && [ -n "$INTEGRATION_PREPARE_TEST_ARGS" ]; then
  docker run \
    --cap-add=NET_ADMIN \
    --device=/dev/net/tun \
    -v "$(pwd)/hack/certs:/hack/certs" \
    -v "$(pwd)/${ARTIFACTS}:/_out/" \
    -v "${TEST_OUTPUTS_DIR}:/outputs/" \
    -v "${OMNI_CONFIG}:/config.yaml" \
    -v "$(pwd)/omnictl/:/omnictl/" \
    -v "$(pwd)/internal/backend/runtime/omni/testdata/pgp/:/internal/backend/runtime/omni/testdata/pgp/" \
    -e SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
    -e SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
    --network host \
    "ghcr.io/siderolabs/omni-integration-test:${ANOTHER_OMNI_VERSION}" \
    --omni.talos-version="${TALOS_VERSION}" \
    --omni.stable-talos-version="${STABLE_TALOS_VERSION}" \
    --omni.kubernetes-version="${KUBERNETES_VERSION}" \
    --omni.another-kubernetes-version="${ANOTHER_KUBERNETES_VERSION}" \
    --omni.omnictl-path=/_out/omnictl-linux-amd64 \
    --omni.expected-machines=${TOTAL_MACHINES} \
    --omni.embedded \
    --omni.config-path=/config.yaml \
    --omni.output-dir=/outputs \
    --omni.log-output=/outputs/omni-upgrade-prepare.log \
    --omni.sleep-after-failure="${SLEEP_AFTER_FAILURE}" \
    --omni.ignore-unknown-fields \
    --test.failfast \
    --test.v \
    ${INTEGRATION_PREPARE_TEST_ARGS:-}
fi

# shellcheck disable=SC2068
# Run the integration test.
SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
  SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
  "${ARTIFACTS}"/integration-test-linux-amd64 \
  --omni.talos-version="${TALOS_VERSION}" \
  --omni.stable-talos-version="${STABLE_TALOS_VERSION}" \
  --omni.kubernetes-version="${KUBERNETES_VERSION}" \
  --omni.another-kubernetes-version="${ANOTHER_KUBERNETES_VERSION}" \
  --omni.omnictl-path="${ARTIFACTS}"/omnictl-linux-amd64 \
  --omni.expected-machines=${TOTAL_MACHINES} \
  --omni.embedded \
  --omni.config-path="${OMNI_CONFIG}" \
  --omni.output-dir="${TEST_OUTPUTS_DIR}" \
  --omni.log-output="${TEST_OUTPUTS_DIR}/omni-integration.log" \
  --omni.sleep-after-failure="${SLEEP_AFTER_FAILURE}" \
  --test.failfast \
  --test.coverprofile="${ARTIFACTS}"/coverage-integration.txt \
  --test.v \
  ${IMPORTED_CLUSTER_ARGS[@]:-} \
  ${INTEGRATION_TEST_ARGS:-}

# No cleanup here, as it runs in the CI as a container in a pod.
