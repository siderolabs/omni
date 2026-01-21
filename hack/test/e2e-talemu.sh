#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Load common functions and variables.
source ./hack/test/common.sh

TALEMU_INFRA_PROVIDER_IMAGE=ghcr.io/siderolabs/talemu-infra-provider:latest
TALEMU_CONTAINER_NAME=talemu

function cleanup() {
  docker stop ${TALEMU_CONTAINER_NAME} || true
  docker logs ${TALEMU_CONTAINER_NAME} &>"$TEST_OUTPUTS_DIR/${TALEMU_CONTAINER_NAME}.log" || true
  docker rm -f ${TALEMU_CONTAINER_NAME} || true

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

docker pull "${TALEMU_INFRA_PROVIDER_IMAGE}"
docker run --name "${TALEMU_CONTAINER_NAME}" \
  --network host --cap-add=NET_ADMIN \
  -it -d \
  "${TALEMU_INFRA_PROVIDER_IMAGE}" \
  --create-service-account \
  --omni-api-endpoint="https://$LOCAL_IP:8099"

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
  -e PROJECT="talemu" \
  -v "${TEST_OUTPUTS_DIR}/e2e/playwright-report:/tmp/test/playwright-report" \
  --network=host \
  e2etest

# No cleanup here, as it runs in the CI as a container in a pod.
