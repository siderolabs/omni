#!/bin/bash

# Copyright (c) 2025 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

dir=""

# find the cluster machine $1 belongs to
for d in "${HOME}"/.talos/clusters/*; do
  if [ -e "${d}/machine-$1.monitor" ]; then
    dir="${d}"

    break
  fi
done

# wipe the VM $1
echo "s" | socat - "unix-connect:${dir}/machine-$1.monitor"

disk="${dir}/machine-$1-0.disk"

size=$(du -bs "${disk}" | cut -f1)

rm "${disk}"

truncate -s "${size}" "${disk}"

echo "q" | socat - "unix-connect:${dir}/machine-$1.monitor"

# connect the VM to omni
cluster=$(basename "${dir}")
if [ "$cluster" == "test-1" ]; then
  sleep 10
  $3 apply-config --insecure --nodes "$2" --file "$4"
fi
