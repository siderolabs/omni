// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { coerce, gte } from 'semver'

/** Talos containers (ContainerConfig) arrived in Talos 1.15. Prereleases such as 1.15.0-beta.0 count. */
export function supportsContainers(talosVersion?: string) {
  const version = coerce(talosVersion)

  return version !== null && gte(version, '1.15.0')
}
