// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { DiscoveredVolumeSpec, DiskSpec } from '@/api/talos/block.pb'

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
 * Returns the volumes laid out on the disk: its partitions, or when it has none,
 * the whole-disk volume if the disk itself carries a signature (a filesystem, an
 * MD RAID member, an LVM physical volume, ...).
 */
export function getDiskVolumes(disk: Disk, volumes: Volume[]): Volume[] {
  const partitions = volumes
    .filter((v) => v.spec.parent === disk.metadata.id)
    .sort((a, b) => {
      if (a.spec.offset !== undefined && b.spec.offset !== undefined) {
        return a.spec.offset - b.spec.offset
      }

      return (a.spec.partition_index || 0) - (b.spec.partition_index || 0)
    })

  if (partitions.length) return partitions

  const wholeDisk = volumes.find((v) => v.metadata.id === disk.metadata.id)
  const name = wholeDisk?.spec.name

  return wholeDisk && name && !partitionTableNames.has(name) ? [wholeDisk] : []
}

/**
 * Returns the IDs of the disks built on top of the given disk, e.g. the MD array
 * a RAID member disk belongs to. Only disks in `candidates` are considered.
 */
export function getParentDiskIds(disk: Disk, candidates: Disk[]): string[] {
  return candidates
    .filter((d) => d.spec.secondary_disks?.includes(disk.metadata.id!))
    .map((d) => d.metadata.id!)
}

/**
 * Returns the IDs of the disks the given disk is built from, e.g. the members of
 * an MD array. Only disks in `candidates` are considered.
 */
export function getChildDiskIds(disk: Disk, candidates: Disk[]): string[] {
  const ids = new Set(candidates.map((d) => d.metadata.id))

  return (disk.spec.secondary_disks ?? []).filter((id) => ids.has(id))
}

export function getVolumeDisplayName(volume: Volume): string {
  return (
    volume.spec.partition_label ||
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
 * partitions where it actually is. Older versions don't, so all free space is
 * put after the last volume.
 */
export function getDiskSegments(diskSize: number, volumes: Volume[]): DiskSegment[] {
  const segments: DiskSegment[] = []
  const positioned = volumes.length > 0 && volumes.every((v) => v.spec.offset !== undefined)

  let cursor = 0

  for (const volume of volumes) {
    const size = volume.spec.size || 0

    if (positioned) {
      const offset = volume.spec.offset!

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
