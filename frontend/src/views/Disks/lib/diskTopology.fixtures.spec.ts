// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { CapturedMachine } from '../fixtures'
import {
  talos114LuksLvm,
  talos114LvmRaid1,
  talos114MdRaid1Failed,
  talos115LuksLvmRaid1,
  talos115LvmRaid1Degraded,
  talos115Maintenance,
  talos115MdRaid1Rebuilding,
  talos115MdRaid1Smart,
} from '../fixtures'
import {
  getArraysWithoutDevice,
  getMDArrayHealth,
  getVolumeGroupsWithoutDevice,
} from './diskHealth'
import { buildDiskTopology } from './diskTopology'
import { getDiskSegments } from './diskVolumes'
import { getFailedVolumes } from './volumeStatus'

// Resources captured from real machines. See fixtures/README.md.

const freeSpace = (machine: CapturedMachine, id: string) => {
  const node = buildDiskTopology(machine).byID.get(id)!

  return getDiskSegments(
    node.disk.spec.size!,
    node.volumes.map((v) => v.volume),
  )
    .filter((s) => s.type === 'unallocated')
    .reduce((sum, s) => sum + s.size, 0)
}

describe('Talos 1.14 with LVM RAID1 over vdc and vdd', () => {
  const topology = buildDiskTopology(talos114LvmRaid1)

  test('shows the logical volume, found by its kernel device number, backed by both disks', () => {
    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => n.id)).toEqual(['dm-4'])

    expect(topology.byID.get('dm-4')).toMatchObject({
      kind: 'lvm',
      name: 'data/data',
      raidLevel: 'raid1',
      backedBy: ['vdc', 'vdd'],
      health: { label: 'Healthy' },
    })

    for (const id of ['vdc', 'vdd']) {
      const disk = topology.byID.get(id)!

      expect(disk.usedBy).toEqual(['dm-4'])
      expect(disk.volumes.map((v) => v.usedBy)).toEqual([['dm-4']])
    }
  })

  test('lays out the system disk, whose EPHEMERAL partition is reported at offset 0', () => {
    const volumes = topology.byID.get('vdb')!.volumes.map((v) => v.volume)

    expect(volumes.map((v) => v.spec.partition_label)).toEqual([
      'EFI',
      'META',
      'STATE',
      'EPHEMERAL',
    ])

    // The partitions fill the disk but for GPT's own space, not 2.3 GB of it.
    expect(freeSpace(talos114LvmRaid1, 'vdb')).toBeLessThanOrEqual(2 * 1024 ** 2)
  })
})

describe('Talos 1.14 with encrypted volumes, one holding an LVM physical volume', () => {
  const topology = buildDiskTopology(talos114LuksLvm)

  test('hides the opened LUKS devices, found by their dm-uuid symlinks', () => {
    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => n.id)).toEqual(['dm-2'])

    expect(topology.byID.get('dm-2')).toMatchObject({
      kind: 'lvm',
      name: 'vg0/data',
      backedBy: ['vda'],
    })
  })

  test('shows both volumes as encrypted, and what the second holds', () => {
    const vda = topology.byID.get('vda')!

    expect(vda.usedBy).toEqual(['dm-2'])
    expect(
      vda.volumes.map((v) => [
        v.volume.spec.partition_label,
        v.volumeStatus?.spec.encryptionProvider,
        v.usedBy,
      ]),
    ).toEqual([
      ['u-secret', 'luks2', []],
      ['r-lvmdata', 'luks2', ['dm-2']],
    ])
  })
})

describe('Talos 1.14 with an MD RAID1 array that failed to be created', () => {
  test('has no device to show the array on, so its error stands alone', () => {
    const { mdArrays = [], disks } = talos114MdRaid1Failed

    expect(buildDiskTopology(talos114MdRaid1Failed).virtual.map((n) => n.id)).toEqual(['dm-4'])
    expect(getArraysWithoutDevice(mdArrays, disks).map((a) => a.spec.status)).toEqual(['error'])
  })
})

describe('Talos 1.15 in maintenance mode', () => {
  const topology = buildDiskTopology(talos115Maintenance)

  test('shows every disk, and none as the system disk', () => {
    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual).toEqual([])
    expect(topology.physical.some((n) => n.system)).toBe(false)
  })
})

describe('Talos 1.15 with LUKS, LVM on LUKS and LVM RAID1', () => {
  // Talos 1.15 reports the device-mapper kind, so no logical volume statuses are needed.
  test.each([
    ['with', talos115LuksLvmRaid1],
    ['without', { ...talos115LuksLvmRaid1, logicalVolumes: [] }],
  ])('finds the logical volumes %s logical volume statuses', (_, machine) => {
    const topology = buildDiskTopology(machine)

    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => [n.id, n.kind, n.name, n.backedBy])).toEqual([
      ['dm-4', 'lvm', 'data/data', ['vdc', 'vdd']],
      ['dm-7', 'lvm', 'vg0/secure', ['vda']],
    ])

    expect(topology.byID.get('vda')!.volumes.map((v) => v.usedBy)).toEqual([[], ['dm-7']])
  })

  test('reports the RAID level and health of the logical volumes', () => {
    const topology = buildDiskTopology(talos115LuksLvmRaid1)

    expect(topology.byID.get('dm-4')).toMatchObject({
      raidLevel: 'raid1',
      health: { label: 'Healthy' },
    })
    expect(topology.byID.get('dm-7')!.health?.label).toBe('Healthy')
  })
})

describe("Talos 1.15 with SMART, an MD RAID1 array over sda and vdb, and config that can't be met", () => {
  const topology = buildDiskTopology(talos115MdRaid1Smart)

  test('marks the system disk', () => {
    expect([...topology.byID.values()].filter((n) => n.system).map((n) => n.id)).toEqual([
      'nvme0n1',
    ])
  })

  test('shows SMART health for the NVMe and SATA disks, and none for virtio', () => {
    expect(topology.physical.map((n) => [n.id, n.health?.label, n.smart?.spec.dev_type])).toEqual([
      ['nvme0n1', 'Failing', 'nvme'],
      ['sda', 'Healthy', 'sata'],
      ['vda', undefined, undefined],
      ['vdb', undefined, undefined],
    ])
  })

  test('explains the NVMe critical warning, which Talos sends no message for', () => {
    expect(topology.byID.get('nvme0n1')!.health?.message).toBe(
      'Reliability is degraded by media or internal errors.',
    )
  })

  test('shows the array with its level, health and user volume', () => {
    const md127 = topology.byID.get('md127')!

    expect(md127).toMatchObject({
      kind: 'md',
      raidLevel: 'raid1',
      backedBy: ['sda', 'vdb'],
      health: { label: 'Healthy' },
    })
    expect(
      md127.volumes.map((v) => [v.volume.spec.partition_label, v.volumeStatus?.spec.phase]),
    ).toEqual([['u-raid-data', 'ready']])
  })

  test('reports partition offsets, including the user volume on the array', () => {
    const offsets = (id: string) =>
      topology.byID
        .get(id)!
        .volumes.map((v) => [v.volume.spec.partition_label, v.volume.spec.offset])

    // Talos 1.15.0-beta.0 still reports 0 for a partition it has just formatted
    // (seen for u-raid-data before this machine restarted), but not after a rescan.
    expect(offsets('nvme0n1')).toEqual([
      ['EFI', 1048576],
      ['META', 2204106752],
      ['STATE', 2205155328],
      ['EPHEMERAL', 2310012928],
    ])
    expect(offsets('md127')).toEqual([['u-raid-data', 1048576]])
  })

  test('reports an array configured with too few disks as waiting, with no device', () => {
    const { mdArrays = [], disks } = talos115MdRaid1Smart

    expect(
      getArraysWithoutDevice(mdArrays, disks).map((a) => [a.metadata.id, getMDArrayHealth(a.spec)]),
    ).toEqual([
      [
        'spare',
        expect.objectContaining({
          label: 'Waiting',
          message: 'waiting for enough member disks: matched 1, required 2',
        }),
      ],
    ])
  })

  test('reports a user volume whose selector matches no disk as failed', () => {
    expect(
      getFailedVolumes(talos115MdRaid1Smart.volumeStatuses).map((v) => [
        v.metadata.id,
        v.spec.errorMessage,
      ]),
    ).toEqual([['u-nowhere', 'no disks matched selector for volume']])
  })
})

describe('Talos 1.15 rebuilding an MD RAID1 array after a member was wiped', () => {
  test('shows the array as rebuilding', () => {
    expect(buildDiskTopology(talos115MdRaid1Rebuilding).byID.get('md127')!.health).toMatchObject({
      label: 'Rebuilding',
      tone: 'warning',
    })
  })
})

describe('Talos 1.15 with an LVM RAID1 volume group missing a physical volume', () => {
  const topology = buildDiskTopology(talos115LvmRaid1Degraded)

  test('has no device for the inactive logical volume', () => {
    expect(topology.virtual).toEqual([])
  })

  test('reports the volume group, with its inactive logical volume, on its own', () => {
    const { volumeGroups = [], logicalVolumes } = talos115LvmRaid1Degraded

    expect(getVolumeGroupsWithoutDevice(volumeGroups, logicalVolumes, [])).toEqual([
      {
        id: 'data',
        health: expect.objectContaining({
          label: 'Degraded',
          message:
            'Volume group data is missing 1 physical volume.\nIts logical volume data/data is inactive.',
        }),
      },
    ])
  })
})
