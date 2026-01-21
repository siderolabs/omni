#!/usr/bin/env bash

# Copyright (c) 2026 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

set -eoux pipefail

TEST_CLASS="${TEST_CLASS:-none}"

echo "Checking test class: $TEST_CLASS"

case "$TEST_CLASS" in
"integration-talemu")
  echo "Starting Integration Tests with talemu..."
  ./hack/test/integration-talemu.sh
  ;;

"integration-qemu")
  echo "Starting Integration Tests with QEMU..."
  ./hack/test/integration-qemu.sh
  ;;

"e2e-qemu")
  echo "Starting End-to-End Tests with QEMU..."
  ./hack/test/e2e-qemu.sh
  ;;

"e2e-talemu")
  echo "Starting End-to-End Tests with talemu..."
  ./hack/test/e2e-talemu.sh
  ;;

*)
  # The catch-all (default) case if nothing matches
  echo "Error: Unknown TEST_CLASS '$TEST_CLASS'. Please use integration-talemu, integration-qemu, e2e-qemu or e2e-talemu."
  exit 1
  ;;
esac
