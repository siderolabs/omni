#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Set before sourcing common.sh so it selects the auth0 branch at load time.
export AUTH_PROVIDER=auth0

# Load common functions and variables.
source ./hack/test/common.sh

function cleanup() {
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

export MAX_USERS=5
export MAX_SERVICE_ACCOUNTS=5

prepare_omni_config

SIDEROLINK_DEV_JOIN_TOKEN="${JOIN_TOKEN}" \
  nice -n 10 "${ARTIFACTS}"/omni-linux-amd64 --config-path "${OMNI_CONFIG}" \
  --etcd-embedded-unsafe-fsync=true \
  --etcd-backup-s3 \
  "${REGISTRY_MIRROR_FLAGS[@]}" >"${TEST_OUTPUTS_DIR}/omni-e2e.log" 2>&1 &

# Run the e2e test.
# the e2e tests are in a submodule and need to be executed in a container with playwright dependencies
#
# Keep this container off the host network. Machine provisioning adds and removes host
# network interfaces, and Chromium aborts every in-flight request (including the resource
# watch streams feeding the UI) with ERR_NETWORK_CHANGED when it sees an interface change
# in its own network namespace, so on the host network the UI tests randomly flake.
cd frontend/
docker buildx build --load . -t e2etest
docker run --rm \
  -e CI="$CI" \
  -e AUTH_PASSWORD="$AUTH_PASSWORD" \
  -e AUTH_USERNAME="$AUTH_USERNAME" \
  -e BASE_URL="$BASE_URL" \
  -e PROJECT="auth0" \
  -v "${TEST_OUTPUTS_DIR}/e2e/playwright-report:/tmp/test/playwright-report" \
  --add-host=my-instance.omni.localhost:host-gateway \
  e2etest

# No cleanup here, as it runs in the CI as a container in a pod.
