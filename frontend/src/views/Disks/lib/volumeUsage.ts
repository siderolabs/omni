// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { MountStatusSpec } from '@/api/talos/block.pb'
import type { StorageServiceStatfsResponse } from '@/api/talos/machine/storage.pb'

/** How full a mounted volume's filesystem is, in bytes. */
export interface VolumeUsage {
  size: number
  used: number
  /** Free space a non-root user can use, which leaves out space reserved for root. */
  available: number
  /** Where the filesystem is mounted, e.g. /var. */
  mountedOn: string
}

/** A Talos volume's filesystem, and where it is mounted. */
export interface VolumeMount {
  volumeID: string
  target: string
}

/**
 * Returns where each Talos volume's own filesystem is mounted, as `talosctl df`
 * does: mounts with no source device have no filesystem of their own, and a
 * detached mount isn't reachable at its target.
 */
export function getVolumeMounts(mountStatuses: Resource<MountStatusSpec>[]): VolumeMount[] {
  const mounts = new Map<string, string>()

  for (const { spec } of mountStatuses) {
    const volumeID = spec.spec?.volumeID

    if (!volumeID || !spec.source || !spec.target || spec.detached || mounts.has(volumeID)) {
      continue
    }

    mounts.set(volumeID, spec.target)
  }

  return [...mounts].map(([volumeID, target]) => ({ volumeID, target }))
}

/** Returns each volume's usage, by volume ID, from the statfs result for its mount. */
export function getVolumeUsages(
  mounts: VolumeMount[],
  statfs: Record<string, StorageServiceStatfsResponse>,
): Map<string, VolumeUsage> {
  return new Map(
    mounts.flatMap(({ volumeID, target }) => {
      const size = Number(statfs[target]?.size || 0)

      if (!size) return []

      const { used, available } = statfs[target]

      return [
        [
          volumeID,
          { size, used: Number(used || 0), available: Number(available || 0), mountedOn: target },
        ],
      ]
    }),
  )
}
