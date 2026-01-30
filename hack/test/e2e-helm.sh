#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# Load common functions and variables.
source ./hack/test/common.sh

DIAG_DIR="${TEST_OUTPUTS_DIR}/cluster-diagnostics"

function gather_cluster_diagnostics() {
  mkdir -p "${DIAG_DIR}/logs"

  echo "Gathering cluster diagnostics into ${DIAG_DIR}..."

  kubectl cluster-info dump >"${DIAG_DIR}/cluster-info-dump.txt" 2>&1 || true
  kubectl get nodes -o wide >"${DIAG_DIR}/nodes.txt" 2>&1 || true
  kubectl get all --all-namespaces -o wide >"${DIAG_DIR}/all-resources.txt" 2>&1 || true
  kubectl get events --all-namespaces --sort-by='.lastTimestamp' >"${DIAG_DIR}/events.txt" 2>&1 || true
  kubectl describe nodes >"${DIAG_DIR}/node-describe.txt" 2>&1 || true
  kubectl describe pods --all-namespaces >"${DIAG_DIR}/pod-describe.txt" 2>&1 || true
  kubectl describe deployments --all-namespaces >"${DIAG_DIR}/deployment-describe.txt" 2>&1 || true
  kubectl describe statefulsets --all-namespaces >"${DIAG_DIR}/statefulset-describe.txt" 2>&1 || true
  kubectl describe daemonsets --all-namespaces >"${DIAG_DIR}/daemonset-describe.txt" 2>&1 || true
  kubectl top nodes >"${DIAG_DIR}/top-nodes.txt" 2>&1 || true
  kubectl top pods --all-namespaces >"${DIAG_DIR}/top-pods.txt" 2>&1 || true
  helm list --all-namespaces >"${DIAG_DIR}/helm-releases.txt" 2>&1 || true

  # Dump logs for every pod across all namespaces.
  # Use a pipe instead of process substitution to avoid /dev/fd issues under sudo.
  kubectl get pods --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null | while IFS='/' read -r ns pod; do
    [ -z "${ns}" ] && continue
    kubectl logs -n "${ns}" "${pod}" --all-containers >"${DIAG_DIR}/logs/${ns}_${pod}.log" 2>&1 || true
    kubectl logs -n "${ns}" "${pod}" --all-containers --previous >"${DIAG_DIR}/logs/${ns}_${pod}-previous.log" 2>/dev/null || true
  done || true

  echo "Cluster diagnostics gathered."
}

function cleanup() {
  gather_cluster_diagnostics || true
  common_cleanup || true

  # Ensure the output directories are always accessible by the non-root CI user for artifact upload.
  chown -R "${SUDO_USER:-$(whoami)}" "${TEST_OUTPUTS_DIR}" || true
  chown -R "${SUDO_USER:-$(whoami)}" "${ARTIFACTS}" || true
}

trap cleanup EXIT SIGINT

# Download required artifacts.
prepare_artifacts

# Set kubeconfig env var to the exact path expected by the "make chart-e2e" target,
# so that create_talos_cluster will write it to that path, and the make target will simply find and use it there.
export KUBECONFIG="${ARTIFACTS}/kubeconfig"

# Prepare the Kubernetes cluster for the Helm e2e tests to run against.
create_talos_cluster name=test-e2e-helm cp_count=1 wk_count=0 cidr=172.24.0.0/24 talos_version="${TALOS_VERSION}" skip_kubeconfig=false allow_scheduling_on_control_planes=true

kubectl get node -owide --show-labels

# Build and push the omni image to the temp registry so the helm chart can use it.
TEMP_REGISTRY="${TEMP_REGISTRY:-127.0.0.1:5005}"
OMNI_IMAGE_TAG="test"
OMNI_IMAGE_REPO="${TEMP_REGISTRY}/siderolabs/omni"

echo "Building and pushing omni image to ${OMNI_IMAGE_REPO}:${OMNI_IMAGE_TAG}..."
make image-omni REGISTRY="${TEMP_REGISTRY}" TAG="${OMNI_IMAGE_TAG}" PUSH=true PLATFORM=linux/amd64

# Generate a values override file for the kuttl tests to use.
cat > deploy/helm/v2/e2e/testdata/image-override.yaml <<EOF
image:
  repository: ${OMNI_IMAGE_REPO}
  tag: ${OMNI_IMAGE_TAG}
  pullPolicy: Always
EOF

echo "Generated image override values:"
cat deploy/helm/v2/e2e/testdata/image-override.yaml

make helm-plugin-install
make kuttl-plugin-install

make chart-unittest
make chart-e2e
