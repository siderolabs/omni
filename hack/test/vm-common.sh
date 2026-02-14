#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

# Find the cluster directory containing the given machine.
# Usage: dir=$(find_machine_dir "$machine_name")
find_machine_dir() {
  local machine="$1"

  for d in "${HOME}/.talos/clusters"/*; do
    if [[ -e "${d}/machine-${machine}.monitor" ]]; then
      echo "${d}"
      return
    fi
  done

  echo "Error: machine '${machine}' not found" >&2
  return 1
}
