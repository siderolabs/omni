// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { compare } from 'semver'

import type { Resource } from '@/api/grpc'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { majorMinorVersion } from '@/methods'

export interface TalosVersionGroup {
  versions: string[]
  unsupported: boolean
}

// Groups the Talos versions a machine running a tracked currentVersion can move to by major.minor,
// newest first, including the current version itself.
//
// Deprecated targets are offered only as upgrades from a deprecated current version, so a machine
// stranded on an old Talos can climb out through the deprecated releases, while it is never offered
// a downgrade into them, and an up-to-date machine never sees them. This mirrors the backend upgrade
// target checks.
export function talosUpgradeTargets(
  versionMap: Map<string, Resource<TalosVersionSpec>>,
  currentVersion?: string,
) {
  if (!currentVersion) return {}

  const current = versionMap.get(currentVersion)
  const currentDeprecated = !!current?.spec.deprecated
  const targets = current?.spec.upgradable_talos_versions ?? []

  return [currentVersion, ...targets]
    .map((v) => versionMap.get(v))
    .filter(
      (v): v is NonNullable<typeof v> =>
        !!v &&
        (!v.spec.deprecated ||
          v.spec.version === currentVersion ||
          (currentDeprecated && compare(v.spec.version!, currentVersion) > 0)),
    )
    .sort((a, b) => compare(b.spec.version!, a.spec.version!))
    .reduce<Record<string, TalosVersionGroup>>((prev, { spec: { unsupported, version } }) => {
      const majorMinor = majorMinorVersion(version!)

      prev[majorMinor] ||= {
        versions: [],
        unsupported: true,
      }

      if (!unsupported || version === currentVersion) {
        prev[majorMinor].versions.push(version!)
        prev[majorMinor].unsupported = false
      }

      return prev
    }, {})
}
