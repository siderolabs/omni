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

HOST_CLUSTER_NAME="test-e2e-helm"
TEST_MACHINES_CLUSTER_NAME="test-helm-machines"

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

# Configure registry mirrors.
configure_registry_mirrors

# Set kubeconfig env var.
export KUBECONFIG="${TEST_OUTPUTS_DIR}/$HOST_CLUSTER_NAME/kubeconfig"

# If using a localhost registry (e.g., when running this test locally), add a mirror so the Talos node can reach it via the bridge gateway.
TEMP_REGISTRY="${TEMP_REGISTRY:-127.0.0.1:5005}"
if [[ "${TEMP_REGISTRY}" == 127.0.0.1:* ]]; then
  REGISTRY_MIRROR_FLAGS+=("--registry-mirror=${TEMP_REGISTRY}=http://172.24.0.1:${TEMP_REGISTRY##*:}")
fi

# Prepare the single-node Talos cluster.
create_talos_cluster name=$HOST_CLUSTER_NAME cp_count=1 wk_count=0 cidr=172.24.0.0/24 talos_version="${TALOS_VERSION}" skip_kubeconfig=false allow_scheduling_on_control_planes=true

kubectl get node -owide --show-labels

# Determine the node IP dynamically.
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
echo "Node IP: ${NODE_IP}"

# Add DNS entries for the test domains.
# example.org is needed for workload proxy: the test client resolves the base domain to connect, then uses the Host header for routing.
echo "${NODE_IP} example.org omni.example.org omni-siderolink.example.org omni-k8s.example.org" >>/etc/hosts

# Generate a short-lived leaf TLS certificate signed by the committed CA.
read -r TLS_CRT TLS_KEY <<<"$(hack/test/helm/generate-leaf-cert.sh "${NODE_IP}" | tr '\n' ' ')"

# Create the TLS secret for Traefik's default certificate store.
kubectl create secret tls example-org-wildcard-tls \
  --cert="${TLS_CRT}" \
  --key="${TLS_KEY}" \
  -n kube-system

# Install Traefik ingress controller.
helm repo add traefik https://traefik.github.io/charts
helm repo update traefik
helm upgrade --install traefik traefik/traefik \
  --namespace kube-system \
  --values hack/test/helm/traefik-values.yaml \
  --wait --timeout 300s

# Deploy a single-node external etcd for Omni's storage backend.
read -r ETCD_SERVER_CRT ETCD_SERVER_KEY ETCD_CLIENT_CRT ETCD_CLIENT_KEY <<< \
  "$(hack/test/helm/generate-etcd-certs.sh | tr '\n' ' ')"
kubectl apply -f hack/test/helm/etcd.yaml
kubectl create secret generic etcd-certs -n etcd \
  --from-file=ca.crt=hack/test/helm/certs/ca.crt \
  --from-file=server.crt="${ETCD_SERVER_CRT}" \
  --from-file=server.key="${ETCD_SERVER_KEY}"
kubectl rollout status deployment/etcd -n etcd --timeout=120s

# Build and push the omni image to the temp registry so the helm chart can use it.
OMNI_IMAGE_TAG="test-$(git rev-parse --short HEAD)"
OMNI_IMAGE_REPO="${TEMP_REGISTRY}/siderolabs/omni"

echo "Building and pushing omni image to ${OMNI_IMAGE_REPO}:${OMNI_IMAGE_TAG}..."
make image-omni REGISTRY="${TEMP_REGISTRY}" TAG="${OMNI_IMAGE_TAG}" PUSH=true PLATFORM=linux/amd64 WITH_DEBUG=true

# Run Helm chart unit tests.
make helm-plugin-install
make chart-unittest

# Install Omni via Helm.
OMNI_NAMESPACE="omni"

# Create the namespace with privileged pod security - Omni requires NET_ADMIN and hostPath (/dev/net/tun).
kubectl create namespace "${OMNI_NAMESPACE}"
kubectl label namespace "${OMNI_NAMESPACE}" pod-security.kubernetes.io/enforce=privileged

# Create the etcd client cert secret so Omni can connect to external etcd.
kubectl create secret generic etcd-client-certs -n "${OMNI_NAMESPACE}" \
  --from-file=ca.crt=hack/test/helm/certs/ca.crt \
  --from-file=client.crt="${ETCD_CLIENT_CRT}" \
  --from-file=client.key="${ETCD_CLIENT_KEY}"

# Build a JSON array of registry mirrors for envsubst.
REGISTRY_MIRRORS_JSON="[]"
mirrors_items=()
for flag in ${REGISTRY_MIRROR_FLAGS[@]+"${REGISTRY_MIRROR_FLAGS[@]}"}; do
  mirror="${flag#--registry-mirror=}"
  [ -z "${mirror}" ] && continue
  mirrors_items+=("\"${mirror}\"")
done
if [ ${#mirrors_items[@]} -gt 0 ]; then
  REGISTRY_MIRRORS_JSON="[$(IFS=,; echo "${mirrors_items[*]}")]"
fi
export REGISTRY_MIRRORS_JSON

# Render the Helm values template with envsubst.
export OMNI_IMAGE_REPO OMNI_IMAGE_TAG AUTH0_CLIENT_ID AUTH0_DOMAIN AUTH0_TEST_USERNAME NODE_IP JOIN_TOKEN
RENDERED_VALUES=$(mktemp)
envsubst < hack/test/helm/omni-values.yaml.envsubst > "${RENDERED_VALUES}"

helm upgrade --install omni deploy/helm/v2/omni/ \
  --namespace "${OMNI_NAMESPACE}" \
  --values "${RENDERED_VALUES}" \
  --wait --timeout 300s

# Wait for Omni pod to be ready.
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=omni -n "${OMNI_NAMESPACE}" --timeout=120s

# Extract the initial service account key from the Omni pod.
OMNI_POD=$(kubectl get pod -n "${OMNI_NAMESPACE}" -l app.kubernetes.io/name=omni -o jsonpath="{.items[0].metadata.name}")

# Launch an ephemeral debug container to access the pod filesystem.
kubectl debug -n "${OMNI_NAMESPACE}" "${OMNI_POD}" \
  --image=busybox:1.36 --target=omni --profile=sysadmin --share-processes -- sleep 600

# Wait for the ephemeral container to be running.
echo "Waiting for ephemeral container to be running..."
for i in $(seq 1 30); do
  STATUS=$(kubectl get pod "${OMNI_POD}" -n "${OMNI_NAMESPACE}" -o jsonpath="{.status.ephemeralContainerStatuses[-1:].state.running}" 2>/dev/null || true)
  if [ -n "${STATUS}" ]; then
    echo "Ephemeral container is running."
    break
  fi
  if [ "${i}" -eq 30 ]; then
    echo "Timed out waiting for ephemeral container to start."
    exit 1
  fi
  sleep 2
done

# Copy the service account key from the pod.
EPHEMERAL_CONTAINER=$(kubectl get pod "${OMNI_POD}" -n "${OMNI_NAMESPACE}" -o jsonpath="{.spec.ephemeralContainers[-1:].name}")
kubectl cp "${OMNI_NAMESPACE}/${OMNI_POD}:/proc/1/root/tmp/initial-service-account-key" "${ARTIFACTS}/omni-sa-key" -c="${EPHEMERAL_CONTAINER}"

echo "Service account key extracted to ${ARTIFACTS}/omni-sa-key"
cat "${ARTIFACTS}/omni-sa-key"

# Build a schematic with SideroLink kernel args pointing to the Helm-deployed Omni.
HELM_SCHEMATIC=$(
  cat <<EOF
customization:
  extraKernelArgs:
    - siderolink.api=grpc://${NODE_IP}:30090?jointoken=${JOIN_TOKEN}
    - talos.events.sink=[fdae:41e4:649b:9303::1]:8091
    - talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092
    - console=tty0
    - console=ttyS0
  systemExtensions:
    officialExtensions:
      - siderolabs/hello-world-service
EOF
)

HELM_SCHEMATIC_ID=$(curl -X POST --data-binary "${HELM_SCHEMATIC}" https://factory.talos.dev/schematics | jq -r '.id')
echo "Schematic ID: ${HELM_SCHEMATIC_ID}"

# Create a single machine to connect to Omni.
create_machines name=$TEST_MACHINES_CLUSTER_NAME count=1 cidr=172.25.0.0/24 secure_boot=false uki=true use_partial_config=false talos_version="${TALOS_VERSION}" \
  kernel_args_schematic_id="${HELM_SCHEMATIC_ID}" partial_config_schematic_id=""

# Run the integration tests against the Helm-deployed Omni instance.
OMNI_ENDPOINT=https://omni.example.org \
  OMNI_SERVICE_ACCOUNT_KEY=$(cat "${ARTIFACTS}/omni-sa-key") \
  SIDEROLINK_DEV_JOIN_TOKEN=${JOIN_TOKEN} \
  SSL_CERT_DIR=hack/test/helm/certs:/etc/ssl/certs \
  "${ARTIFACTS}"/integration-test-linux-amd64 \
  --omni.endpoint=https://omni.example.org \
  --omni.expected-machines=1 \
  --omni.talos-version="${TALOS_VERSION}" \
  --omni.kubernetes-version="${KUBERNETES_VERSION}" \
  --omni.omnictl-path="${ARTIFACTS}"/omnictl-linux-amd64 \
  --omni.output-dir="${TEST_OUTPUTS_DIR}" \
  --omni.log-output="${TEST_OUTPUTS_DIR}/omni-helm-integration.log" \
  --test.run "TestIntegration/Suites/(CleanState|SingleNodeWorkloadProxy)$" \
  --test.v
