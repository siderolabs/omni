#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

source ./hack/test/vm-common.sh

dir=$(find_machine_dir "$1")

# restart the VM $1
echo "q" | socat - "unix-connect:${dir}/machine-${1}.monitor"
