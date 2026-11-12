// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { Resource } from '@/api/grpc'
import type { MountStatusSpec } from '@/api/talos/block.pb'
import type { StorageServiceStatfsResponse } from '@/api/talos/machine/storage.pb'
import type { DiskTopologyInput } from '@/views/Disks/lib/diskTopology'

import talos114LuksLvmJSON from './talos-1.14-luks-lvm.json'
import talos114LvmRaid1JSON from './talos-1.14-lvm-raid1.json'
import talos114MdRaid1FailedJSON from './talos-1.14-md-raid1-failed.json'
import talos115LuksLvmRaid1JSON from './talos-1.15-luks-lvm-raid1.json'
import talos115LvmRaid1DegradedJSON from './talos-1.15-lvm-raid1-degraded.json'
import talos115MaintenanceJSON from './talos-1.15-maintenance.json'
import talos115MdRaid1RebuildingJSON from './talos-1.15-md-raid1-rebuilding.json'
import talos115MdRaid1SmartJSON from './talos-1.15-md-raid1-smart.json'

/** The disks view's resources, captured from a real machine. See README.md. */
export interface CapturedMachine extends DiskTopologyInput {
  talosVersion: string
  /** Where Talos volumes are mounted. Talos 1.15+ fixtures captured after usage was added only. */
  mountStatuses?: Resource<MountStatusSpec>[]
  /** Talos's Statfs result for each mount target, from `talosctl df -b`. */
  statfs?: Record<string, StorageServiceStatfsResponse>
}

/**
 * A Talos 1.14 QEMU machine with an LVM RAID1 logical volume, data/data, over
 * vdc and vdd. vdb is the system disk.
 */
export const talos114LvmRaid1: CapturedMachine = talos114LvmRaid1JSON

/**
 * A Talos 1.14 QEMU machine with two encrypted volumes on vda: a user volume,
 * u-secret, and a raw volume, r-lvmdata, holding an LVM physical volume for
 * the logical volume vg0/data. vdb is the system disk.
 */
export const talos114LuksLvm: CapturedMachine = talos114LuksLvmJSON

/**
 * A Talos 1.14 QEMU machine on which Talos failed to create an MD RAID1 array
 * over vdc and vdd, because an LVM volume group still held them.
 */
export const talos114MdRaid1Failed: CapturedMachine = talos114MdRaid1FailedJSON

/** A Talos 1.15 QEMU machine in maintenance mode. */
export const talos115Maintenance: CapturedMachine = talos115MaintenanceJSON

/**
 * A Talos 1.15 QEMU machine with emulated NVMe and SATA disks: on vda, an
 * encrypted user volume, u-secret, and an encrypted raw volume holding the PV of
 * vg0/secure; LVM RAID1 data/data over sda and vdb; and a user volume on each
 * logical volume, with how full each filesystem is. nvme0n1 is the system disk.
 */
export const talos115LuksLvmRaid1: CapturedMachine = talos115LuksLvmRaid1JSON

/**
 * A Talos 1.15 QEMU machine with emulated NVMe and SATA disks, monitored by
 * SMART, the NVMe disk failing with a critical warning set through QEMU, and an
 * MD RAID1 array, md127, over sda and vdb, holding a user volume.
 * It is also configured with an array, spare, over vda alone, which waits for
 * a second member, and a user volume, u-nowhere, whose selector matches no
 * disk. nvme0n1 is the system disk.
 */
export const talos115MdRaid1Smart: CapturedMachine = talos115MdRaid1SmartJSON

/**
 * The machine of talos115MdRaid1Smart, rebuilding md127 after vdb was wiped and
 * the machine restarted. Its NVMe disk is healthy again.
 */
export const talos115MdRaid1Rebuilding: CapturedMachine = talos115MdRaid1RebuildingJSON

/**
 * A Talos 1.15 QEMU machine whose LVM RAID1 logical volume, data/data, over sda
 * and vdb, is inactive after vdb was wiped and the machine restarted: its
 * volume group is missing a physical volume. nvme0n1 is the system disk.
 */
export const talos115LvmRaid1Degraded: CapturedMachine = talos115LvmRaid1DegradedJSON
