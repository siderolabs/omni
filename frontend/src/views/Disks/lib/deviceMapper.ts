// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { DeviceSpec, DiskSpec, VolumeStatusSpec } from '@/api/talos/block.pb'
import type { LVMLogicalVolumeStatusSpec } from '@/api/talos/storage.pb'

type Disk = Resource<DiskSpec>

export type DeviceMapperKind = 'lvm' | 'crypt' | 'multipath' | 'dm'

/** What is known about a machine's device-mapper devices, beyond the disk itself. */
export interface DeviceMapperContext {
  /** Block devices by ID, for the kernel major/minor of each device. */
  blockDevices: Map<string, Resource<DeviceSpec>>
  /** Logical volumes, reported by Talos 1.14+. */
  logicalVolumes: Resource<LVMLogicalVolumeStatusSpec>[]
  volumeStatuses: Resource<VolumeStatusSpec>[]
}

export function isDeviceMapper(disk: Disk): boolean {
  return !!disk.spec.device_mapper_name || !!disk.metadata.id?.startsWith('dm-')
}

const dmUUIDSymlinkPrefix = '/dev/disk/by-id/dm-uuid-'

// The kind of a device-mapper device, from its DM UUID. The same rule Talos
// uses to fill device_mapper_kind: a partition takes the kind of what it is a
// partition of.
const kindFromUUID = (uuid: string): DeviceMapperKind => {
  const unpartitioned = uuid.replace(/^part\d+-/, '')

  if (unpartitioned.startsWith('mpath-')) return 'multipath'
  if (unpartitioned.startsWith('LVM-')) return 'lvm'
  if (unpartitioned.startsWith('CRYPT-')) return 'crypt'

  return 'dm'
}

const getDeviceMapperUUID = (disk: Disk) =>
  disk.spec.device_mapper_uuid ||
  disk.spec.symlinks
    ?.find((s) => s.startsWith(dmUUIDSymlinkPrefix))
    ?.slice(dmUUIDSymlinkPrefix.length)

/**
 * Whether the disk is a device-mapper mapping of one partition of another
 * device, such as kpartx creates for a multipath device. It takes the kind of
 * that device, so only its UUID tells it apart.
 */
export function isPartitionMapping(disk: Disk): boolean {
  return /^part\d+-/.test(getDeviceMapperUUID(disk) ?? '')
}

/** Returns the logical volume behind the device-mapper disk, matched by kernel device number. */
export function getLogicalVolume(
  disk: Disk,
  ctx: DeviceMapperContext,
): Resource<LVMLogicalVolumeStatusSpec> | undefined {
  const device = ctx.blockDevices.get(disk.metadata.id!)

  if (device?.spec.major === undefined || device.spec.minor === undefined) return

  return ctx.logicalVolumes.find(
    (lv) =>
      lv.spec.kernelMajor === String(device.spec.major) &&
      lv.spec.kernelMinor === String(device.spec.minor),
  )
}

/**
 * Classifies a device-mapper disk.
 *
 * Talos 1.15+ reports the kind directly. On older versions, it is worked out from
 * the dm-uuid symlink, the logical volumes Talos reports (1.14+), or the
 * encrypted volumes whose contents the device holds.
 */
export function classifyDeviceMapper(disk: Disk, ctx: DeviceMapperContext): DeviceMapperKind {
  switch (disk.spec.device_mapper_kind) {
    case 'mpath':
      return 'multipath'
    case 'lvm':
    case 'crypt':
    case 'dm':
      return disk.spec.device_mapper_kind
  }

  const uuid = getDeviceMapperUUID(disk)
  if (uuid) return kindFromUUID(uuid)

  if (getLogicalVolume(disk, ctx)) return 'lvm'

  if (
    ctx.volumeStatuses.some(
      (v) => v.spec.encryptionProvider && v.spec.mountLocation === disk.spec.dev_path,
    )
  ) {
    return 'crypt'
  }

  return 'dm'
}

// LVM joins the volume group and logical volume names with "-" in the
// device-mapper name, and doubles any "-" inside either name.
const mapperNameToLVM = (name: string) => {
  const match = /^((?:[^-]|--)+)-((?:[^-]|--)+)$/.exec(name)

  if (!match) return

  return `${match[1].replaceAll('--', '-')}/${match[2].replaceAll('--', '-')}`
}

/** Returns a logical volume's "vg/lv" name, or undefined for any other device. */
export function getLogicalVolumeName(disk: Disk, ctx: DeviceMapperContext): string | undefined {
  const lv = getLogicalVolume(disk, ctx)
  if (lv?.spec.fullName) return lv.spec.fullName

  if (disk.spec.device_mapper_name && classifyDeviceMapper(disk, ctx) === 'lvm') {
    return mapperNameToLVM(disk.spec.device_mapper_name)
  }
}

/**
 * Returns the RAID level of a logical volume, such as "raid1", from the layout
 * LVM reports for it ("raid,raid1"). Undefined for a volume which isn't RAID.
 */
export function getLogicalVolumeRaidLevel(
  disk: Disk,
  ctx: DeviceMapperContext,
): string | undefined {
  return getLogicalVolume(disk, ctx)
    ?.spec.layout?.split(',')
    .find((part) => /^raid\d+$/.test(part))
}
