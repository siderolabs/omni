#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -euo pipefail

NAMESPACE="omni-e2e"

POD_NAME=$(kubectl get pod -n "${NAMESPACE}" -l app.kubernetes.io/name=omni -o jsonpath="{.items[0].metadata.name}")
kubectl wait --for=condition=Ready "pod/${POD_NAME}" -n "${NAMESPACE}"

kubectl debug -n="${NAMESPACE}" "${POD_NAME}" --image=busybox:1.36 --target=omni --profile=sysadmin --share-processes -- sleep 600

# Wait for the ephemeral container to be running.
echo "Waiting for ephemeral container to be running..."
for i in $(seq 1 30); do
  STATUS=$(kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath="{.status.ephemeralContainerStatuses[-1:].state.running}" 2>/dev/null || true)
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

# Pick the last ephemeral container (the one we just created).
EPHEMERAL_CONTAINER=$(kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath="{.spec.ephemeralContainers[-1:].name}")
kubectl cp "${NAMESPACE}/${POD_NAME}:/proc/1/root/tmp/initial-service-account-key" ../../testdata/sakey -c="${EPHEMERAL_CONTAINER}"

kubectl create secret generic omni-sa-key --from-file=sakey=../../testdata/sakey -n "${NAMESPACE}"
