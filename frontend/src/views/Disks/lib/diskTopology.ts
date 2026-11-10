// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type {
  DeviceSpec,
  DiscoveredVolumeSpec,
  DiskSpec,
  VolumeStatusSpec,
} from '@/api/talos/block.pb'
import type { LVMLogicalVolumeStatusSpec } from '@/api/talos/storage.pb'

import {
  classifyDeviceMapper,
  type DeviceMapperContext,
  type DeviceMapperKind,
  getLogicalVolumeName,
  getLogicalVolumeRaidLevel,
  isDeviceMapper,
  isPartitionMapping,
} from './deviceMapper'
import { getDiskVolumes } from './diskVolumes'

export type DiskNodeKind = 'disk' | 'cdrom' | 'md' | DeviceMapperKind

export interface DiskNodeVolume {
  volume: Resource<DiscoveredVolumeSpec>
  volumeStatus?: Resource<VolumeStatusSpec>
  /** Shown devices built on this volume, e.g. the MD array a partition is a member of. */
  usedBy: string[]
}

export interface DiskNode {
  id: string
  /** The name to show: "vg0/data" for a logical volume, the mapper name of another device-mapper device. */
  name: string
  kind: DiskNodeKind
  disk: Resource<DiskSpec>
  volumes: DiskNodeVolume[]
  /** Shown devices built on this disk. */
  usedBy: string[]
  /** Shown devices this disk is built from. */
  backedBy: string[]
  /** The disks a multipath device reaches its storage through. */
  paths: string[]
  /** The RAID level of a RAID logical volume, such as "raid1". */
  raidLevel?: string
}

export interface DiskTopology {
  /** Disks with storage of their own, including multipath devices. */
  physical: DiskNode[]
  /** Devices built from other disks: MD arrays, logical volumes, other device-mapper devices. */
  virtual: DiskNode[]
  /** Every shown device, by ID. */
  byID: Map<string, DiskNode>
}

export interface DiskTopologyInput {
  disks: Resource<DiskSpec>[]
  volumes: Resource<DiscoveredVolumeSpec>[]
  volumeStatuses: Resource<VolumeStatusSpec>[]
  blockDevices: Resource<DeviceSpec>[]
  logicalVolumes: Resource<LVMLogicalVolumeStatusSpec>[]
}

const physicalKinds = new Set<DiskNodeKind>(['disk', 'cdrom', 'multipath'])

const byID = (a: DiskNode, b: DiskNode) => a.id.localeCompare(b.id, undefined, { numeric: true })

const classifyDisk = (disk: Resource<DiskSpec>, ctx: DeviceMapperContext): DiskNodeKind => {
  if (isDeviceMapper(disk)) return classifyDeviceMapper(disk, ctx)
  if (disk.metadata.id!.startsWith('md')) return 'md'
  if (disk.spec.cdrom) return 'cdrom'

  return 'disk'
}

/**
 * Builds the machine's storage topology: which disks are shown, and which
 * devices are built on which.
 *
 * Disk-level links come from `Disk.secondary_disks`. Volume-level links, such as
 * an MD array built on a partition, come from `BlockDevice.secondaries`. Both
 * are followed through devices which are not shown (an encrypted device is shown
 * through the volume it holds), so a logical volume on LUKS on sda2 is backed by
 * sda.
 *
 * Some devices are not disks at all: Talos reports no Disk for the hidden
 * sub-volumes LVM builds a RAID logical volume from. Links through those come
 * from `BlockDevice.secondaries` instead.
 */
export function buildDiskTopology(input: DiskTopologyInput): DiskTopology {
  const ctx: DeviceMapperContext = {
    blockDevices: new Map(input.blockDevices.map((d) => [d.metadata.id!, d])),
    logicalVolumes: input.logicalVolumes,
    volumeStatuses: input.volumeStatuses,
  }

  const disks = input.disks.filter(
    (d) =>
      !d.metadata.id?.startsWith('loop') &&
      // Talos always creates a DiscoveredVolume for a CD-ROM drive, but leaves
      // `name` (the filesystem type) empty when no media is present.
      (!d.spec.cdrom || input.volumes.some((v) => v.metadata.id === d.metadata.id && v.spec.name)),
  )

  const diskByID = new Map(disks.map((d) => [d.metadata.id!, d]))
  const kinds = new Map(disks.map((d) => [d.metadata.id!, classifyDisk(d, ctx)]))

  // A partition mapping is shown as a partition of the device it maps.
  const partitionMappings = new Set(
    disks.filter((d) => isDeviceMapper(d) && isPartitionMapping(d)).map((d) => d.metadata.id!),
  )

  const isMultipath = (id: string) => kinds.get(id) === 'multipath' && !partitionMappings.has(id)

  const multipathPaths = new Set(
    disks.filter((d) => isMultipath(d.metadata.id!)).flatMap((d) => d.spec.secondary_disks ?? []),
  )

  // An encrypted device is shown through the volume it holds, and a multipath
  // path through its multipath device.
  const isShown = (id: string) =>
    diskByID.has(id) &&
    kinds.get(id) !== 'crypt' &&
    !multipathPaths.has(id) &&
    !partitionMappings.has(id)

  // Follows `related` from `id` to the nearest shown devices, through any which are not.
  const resolve = (id: string, related: (id: string) => string[], seen = new Set([id])) =>
    related(id).flatMap((next): string[] => {
      if (seen.has(next)) return []
      seen.add(next)

      return isShown(next) ? [next] : resolve(next, related, seen)
    })

  const diskParents = (id: string) => [
    ...disks.filter((d) => d.spec.secondary_disks?.includes(id)).map((d) => d.metadata.id!),
    ...input.blockDevices
      .filter((d) => !diskByID.has(d.metadata.id!) && d.spec.secondaries?.includes(id))
      .map((d) => d.metadata.id!),
  ]

  const diskChildren = (id: string) => {
    if (isMultipath(id)) return []

    const disk = diskByID.get(id)

    return disk
      ? (disk.spec.secondary_disks ?? [])
      : (ctx.blockDevices.get(id)?.spec.secondaries ?? [])
  }

  const deviceParents = (id: string) =>
    input.blockDevices.filter((d) => d.spec.secondaries?.includes(id)).map((d) => d.metadata.id!)

  const nodes = disks
    .filter((d) => isShown(d.metadata.id!))
    .map((disk): DiskNode => {
      const id = disk.metadata.id!
      const kind = kinds.get(id)!

      return {
        id,
        name:
          (kind === 'lvm' && getLogicalVolumeName(disk, ctx)) || disk.spec.device_mapper_name || id,
        kind,
        disk,
        volumes: getDiskVolumes(disk, input.volumes).map((volume) => ({
          volume,
          volumeStatus: input.volumeStatuses.find((s) => s.spec.location === volume.spec.dev_path),
          usedBy: [...new Set(resolve(volume.metadata.id!, deviceParents))],
        })),
        usedBy: [...new Set(resolve(id, diskParents))],
        backedBy: [...new Set(resolve(id, diskChildren))],
        paths: kind === 'multipath' ? (disk.spec.secondary_disks ?? []) : [],
        raidLevel: kind === 'lvm' ? getLogicalVolumeRaidLevel(disk, ctx) : undefined,
      }
    })
    .sort(byID)

  return {
    physical: nodes.filter((n) => physicalKinds.has(n.kind)),
    virtual: nodes.filter((n) => !physicalKinds.has(n.kind)),
    byID: new Map(nodes.map((n) => [n.id, n])),
  }
}
