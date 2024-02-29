#!/bin/sh

# Copyright (c) 2024 Sidero Labs, Inc.
#
# Use of this software is governed by the Business Source License
# included in the LICENSE file.

PLATFORM=$(uname -s | tr "[:upper:]" "[:lower:]")
ARCHITECTURE=""
case $(uname -m) in
  i386)   echo "32 bit architecture is not supported" exit 1 ;;
  i686)   echo "32 bit architecture is not supported" exit 1 ;;
  x86_64) ARCHITECTURE="amd64" ;;
  arm)    ARCHITECTURE="arm64" ;;
esac

sudo -E _out/omni-${PLATFORM}-${ARCHITECTURE} --port 8091 >_out/backend.log 2>&1 &
cd frontend && npm install && npm run serve
