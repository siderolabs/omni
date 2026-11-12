// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { DiscoveredVolumeSpec, DiskSpec, VolumeStatusSpec } from '@/api/talos/block.pb'

type Volume = Resource<DiscoveredVolumeSpec>
type Disk = Resource<DiskSpec>

// The probe name Talos reports for a whole-disk volume that holds a partition
// table. Such a volume is the container of its partitions, not a volume of its own.
const partitionTableNames = new Set(['gpt'])

// Probe names of signatures which make a disk or partition a member of something
// else, rather than holding a filesystem. A filesystem label means nothing for
// these (an MD member's label is the array name), so they are named by type.
const memberSignatureNames: Record<string, string> = {
  linux_raid_member: 'RAID member',
  'lvm2-pv': 'LVM physical volume',
  zfs: 'ZFS member',
  bluestore: 'Ceph BlueStore',
}

const getMemberSignatureName = (name?: string) => {
  if (!name) return

  if (name.startsWith('luks')) return 'LUKS'

  return memberSignatureNames[name]
}

// Signatures whose space is handed over to a device built on top of them (an MD
// array, an LVM volume group), so the space is shown again on that device.
const claimedSignatures = new Set(['linux_raid_member', 'lvm2-pv'])

/** Whether the volume's space is used by another device rather than holding data itself. */
export function isClaimedVolume(volume: Volume): boolean {
  return claimedSignatures.has(volume.spec.name ?? '')
}

/**
 * Returns where the volume starts on its disk, or undefined when that isn't known.
 *
 * Talos 1.12+ reports it, but resets it to 0 when it rescans a partition on its
 * own, as it does after formatting one, e.g. EPHEMERAL
 * (https://github.com/siderolabs/talos/pull/14916). No partition can start at
 * 0, where the partition table is, so 0 is taken to mean unknown for a partition.
 */
export function getVolumeOffset(volume: Volume): number | undefined {
  const { offset, parent } = volume.spec

  return parent && !offset ? undefined : offset
}

/**
 * Returns the volumes laid out on the disk: its partitions, or when it has none,
 * the whole-disk volume if the disk itself carries a signature (a filesystem, an
 * MD RAID member, an LVM physical volume, ...).
 */
export function getDiskVolumes(disk: Disk, volumes: Volume[]): Volume[] {
  const partitions = volumes.filter((v) => v.spec.parent === disk.metadata.id)

  // Order by offset only when every partition's is known: comparing by offset
  // for some pairs and by index for others doesn't give a consistent order.
  if (partitions.every((v) => getVolumeOffset(v) !== undefined)) {
    partitions.sort((a, b) => getVolumeOffset(a)! - getVolumeOffset(b)!)
  } else {
    partitions.sort((a, b) => (a.spec.partition_index || 0) - (b.spec.partition_index || 0))
  }

  if (partitions.length) return partitions

  const wholeDisk = volumes.find((v) => v.metadata.id === disk.metadata.id)
  const name = wholeDisk?.spec.name

  return wholeDisk && name && !partitionTableNames.has(name) ? [wholeDisk] : []
}

/**
 * Returns the name to show for a volume: its partition label, or for a volume
 * with none, such as one taking a whole disk, the name Talos gives it.
 */
export function getVolumeDisplayName(
  volume: Volume,
  volumeStatus?: Resource<VolumeStatusSpec>,
): string {
  return (
    volume.spec.partition_label ||
    volumeStatus?.metadata.id ||
    getMemberSignatureName(volume.spec.name) ||
    volume.spec.label ||
    volume.metadata.id!
  )
}

export type DiskSegment =
  | { type: 'volume'; volume: Volume; size: number }
  | { type: 'unallocated'; offset: number; size: number }

/**
 * Splits the disk into volume and unallocated segments, in on-disk order.
 *
 * Talos 1.12+ reports each volume's offset, which places free space between
 * partitions where it actually is. When any offset is unknown, all free space
 * is put after the last volume.
 */
export function getDiskSegments(diskSize: number, volumes: Volume[]): DiskSegment[] {
  const segments: DiskSegment[] = []
  const positioned = volumes.length > 0 && volumes.every((v) => getVolumeOffset(v) !== undefined)

  let cursor = 0

  for (const volume of volumes) {
    const size = volume.spec.size || 0

    if (positioned) {
      const offset = getVolumeOffset(volume)!

      if (offset > cursor) {
        segments.push({ type: 'unallocated', offset: cursor, size: offset - cursor })
      }

      cursor = Math.max(cursor, offset + size)
    } else {
      cursor += size
    }

    segments.push({ type: 'volume', volume, size })
  }

  if (diskSize > cursor) {
    segments.push({ type: 'unallocated', offset: cursor, size: diskSize - cursor })
  }

  return segments
}
