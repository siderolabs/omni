#!/usr/bin/env bash

# Copyright (c) 2025 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Load common functions and variables.
source ./hack/test/common.sh

TALEMU_INFRA_PROVIDER_IMAGE=ghcr.io/siderolabs/talemu-infra-provider:latest
TALEMU_CONTAINER_NAME=talemu
PROMETHEUS_CONTAINER_NAME=talemu-prom

function cleanup() {
  docker stop ${TALEMU_CONTAINER_NAME} || true
  docker logs ${TALEMU_CONTAINER_NAME} &>"$TEST_OUTPUTS_DIR/${TALEMU_CONTAINER_NAME}.log" || true
  docker rm -f ${TALEMU_CONTAINER_NAME} || true

  docker stop ${PROMETHEUS_CONTAINER_NAME} || true
  docker rm -f ${PROMETHEUS_CONTAINER_NAME} || true

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

prepare_omni_config

docker run --name ${PROMETHEUS_CONTAINER_NAME} \
  --network host -p "9090:9090" \
  -v "$(pwd)/hack/compose/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml" \
  -it --rm -d prom/prometheus

docker pull "${TALEMU_INFRA_PROVIDER_IMAGE}"
docker run --name "${TALEMU_CONTAINER_NAME}" \
  --network host --cap-add=NET_ADMIN \
  -it -d \
  "${TALEMU_INFRA_PROVIDER_IMAGE}" \
  --create-service-account \
  --omni-api-endpoint="https://$LOCAL_IP:8099"

SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
  SSL_CERT_DIR=hack/certs:/etc/ssl/certs \
  "${ARTIFACTS}"/integration-test-linux-amd64 \
  --omni.talos-version="${TALOS_VERSION}" \
  --omni.stable-talos-version="${STABLE_TALOS_VERSION}" \
  --omni.kubernetes-version="${KUBERNETES_VERSION}" \
  --omni.another-kubernetes-version="${ANOTHER_KUBERNETES_VERSION}" \
  --omni.omnictl-path=${ARTIFACTS}/omnictl-linux-amd64 \
  --omni.expected-machines=30 \
  --omni.provision-config-file=hack/test/provisionconfig.yaml \
  --omni.run-stats-check \
  --omni.embedded \
  --omni.config-path="${OMNI_CONFIG}" \
  --omni.output-dir="${TEST_OUTPUTS_DIR}" \
  --omni.log-output="${TEST_OUTPUTS_DIR}/omni-emulator.log" \
  --omni.sleep-after-failure="${SLEEP_AFTER_FAILURE}" \
  --test.coverprofile="${ARTIFACTS}"/coverage-emulator.txt \
  --test.timeout 10m \
  --test.parallel 10 \
  --test.failfast \
  --test.v \
  ${TALEMU_TEST_ARGS:-}

# No cleanup here, as it runs in the CI as a container in a pod.
