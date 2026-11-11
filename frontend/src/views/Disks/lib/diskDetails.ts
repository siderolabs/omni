// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { DiskSpec } from '@/api/talos/block.pb'

type Disk = Resource<DiskSpec>

export interface DiskDetail {
  label: string
  values: string[]
}

/** Returns what identifies the disk's hardware: its model and serial number, when known. */
export function getDiskIdentity(disk: Disk): string[] {
  return [disk.spec.model, disk.spec.serial].filter((v): v is string => !!v)
}

/** Returns whether the disk is solid-state or spinning, or undefined for a CD-ROM. */
export function getDiskMediaType(disk: Disk): 'SSD' | 'HDD' | undefined {
  if (disk.spec.cdrom) return

  return disk.spec.rotational ? 'HDD' : 'SSD'
}

const symlinksUnder = (disk: Disk, dir: string) =>
  disk.spec.symlinks?.filter((s) => s.startsWith(dir)) ?? []

/**
 * Returns the disk's identifiers beyond its name, for looking it up or
 * selecting it in machine config. Only those the disk has are returned.
 */
export function getDiskDetails(disk: Disk): DiskDetail[] {
  const { wwid, firmware_version: firmware, bus_path: busPath } = disk.spec

  const details: DiskDetail[] = [
    { label: 'WWID', values: wwid ? [wwid] : [] },
    { label: 'Firmware', values: firmware ? [firmware] : [] },
    // Talos reports /virtual for devices with no bus, such as MD arrays.
    { label: 'Bus path', values: busPath && busPath !== '/virtual' ? [busPath] : [] },
    { label: 'By ID', values: symlinksUnder(disk, '/dev/disk/by-id/') },
    { label: 'By path', values: symlinksUnder(disk, '/dev/disk/by-path/') },
  ]

  return details.filter((d) => d.values.length)
}
