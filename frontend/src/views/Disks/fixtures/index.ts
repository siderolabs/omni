// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import type { DiskTopologyInput } from '@/views/Disks/lib/diskTopology'

import talos114LuksLvmJSON from './talos-1.14-luks-lvm.json'
import talos114LvmRaid1JSON from './talos-1.14-lvm-raid1.json'
import talos114MaintenanceJSON from './talos-1.14-maintenance.json'
import talos114MdRaid1JSON from './talos-1.14-md-raid1.json'
import talos115LuksLvmRaid1JSON from './talos-1.15-luks-lvm-raid1.json'
import talos115MaintenanceJSON from './talos-1.15-maintenance.json'
import talos115MdRaid1JSON from './talos-1.15-md-raid1.json'

/** The disks view's resources, captured from a real machine. See README.md. */
export interface CapturedMachine extends DiskTopologyInput {
  talosVersion: string
}

/** A Talos 1.14 QEMU machine in maintenance mode. */
export const talos114Maintenance: CapturedMachine = talos114MaintenanceJSON

/**
 * A Talos 1.14 QEMU machine with an LVM RAID1 logical volume, data/data, over
 * vdc and vdd. vdb is the system disk.
 */
export const talos114LvmRaid1: CapturedMachine = talos114LvmRaid1JSON

/**
 * A Talos 1.14 QEMU machine with an MD RAID1 array, md0, over vdc and vdd, and
 * a user volume, u-raid-data, on the array. vdb is the system disk.
 */
export const talos114MdRaid1: CapturedMachine = talos114MdRaid1JSON

/**
 * A Talos 1.14 QEMU machine with two encrypted volumes on vda: a user volume,
 * u-secret, and a raw volume, r-lvmdata, holding an LVM physical volume for
 * the logical volume vg0/data. vdb is the system disk.
 */
export const talos114LuksLvm: CapturedMachine = talos114LuksLvmJSON

/** A Talos 1.15 QEMU machine in maintenance mode. */
export const talos115Maintenance: CapturedMachine = talos115MaintenanceJSON

/** The same setup as talos114MdRaid1, on Talos 1.15. */
export const talos115MdRaid1: CapturedMachine = talos115MdRaid1JSON

/**
 * A Talos 1.15 QEMU machine with the setup of talos114LuksLvm on vda, its
 * logical volume named vg0/secure, and an LVM RAID1 logical volume, data/data,
 * over vdc and vdd. vdb is the system disk.
 */
export const talos115LuksLvmRaid1: CapturedMachine = talos115LuksLvmRaid1JSON
