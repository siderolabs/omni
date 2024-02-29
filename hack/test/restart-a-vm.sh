#!/bin/bash

# Copyright (c) 2024 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

# restart a VM UUID $1 in talosctl cluster created cluster 'talos-default'

echo "q" | socat - unix-connect:${HOME}/.talos/clusters/talos-default/machine-$1.monitor
