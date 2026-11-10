// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import {
  talos114LuksLvm,
  talos114LvmRaid1,
  talos114Maintenance,
  talos114MdRaid1,
  talos115LuksLvmRaid1,
  talos115Maintenance,
  talos115MdRaid1,
} from '../fixtures'
import { buildDiskTopology } from './diskTopology'
import { getDiskSegments } from './diskVolumes'

describe('buildDiskTopology on captured machines', () => {
  test('Talos 1.14 in maintenance mode', () => {
    const topology = buildDiskTopology(talos114Maintenance)

    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual).toEqual([])
  })

  test('Talos 1.14 with an LVM RAID1 logical volume over vdc and vdd', () => {
    const topology = buildDiskTopology(talos114LvmRaid1)

    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => n.id)).toEqual(['dm-4'])

    expect(topology.byID.get('dm-4')).toMatchObject({
      kind: 'lvm',
      name: 'data/data',
      raidLevel: 'raid1',
      backedBy: ['vdc', 'vdd'],
    })

    for (const id of ['vdc', 'vdd']) {
      const disk = topology.byID.get(id)!

      expect(disk.usedBy).toEqual(['dm-4'])
      expect(disk.volumes.map((v) => v.usedBy)).toEqual([['dm-4']])
    }
  })

  test('Talos 1.14 system disk, whose EPHEMERAL partition is reported at offset 0', () => {
    const vdb = buildDiskTopology(talos114LvmRaid1).byID.get('vdb')!
    const volumes = vdb.volumes.map((v) => v.volume)

    expect(volumes.map((v) => v.spec.partition_label)).toEqual([
      'EFI',
      'META',
      'STATE',
      'EPHEMERAL',
    ])

    // The partitions fill the disk but for GPT's own space, not 2.3 GB of it.
    const free = getDiskSegments(vdb.disk.spec.size!, volumes)
      .filter((s) => s.type === 'unallocated')
      .reduce((sum, s) => sum + s.size, 0)

    expect(free).toBeLessThanOrEqual(2 * 1024 ** 2)
  })

  test('Talos 1.14 with an MD RAID1 array over vdc and vdd, holding a user volume', () => {
    const topology = buildDiskTopology(talos114MdRaid1)

    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => n.id)).toEqual(['md0'])

    const md0 = topology.byID.get('md0')!
    expect(md0).toMatchObject({ kind: 'md', backedBy: ['vdc', 'vdd'] })
    expect(
      md0.volumes.map((v) => [v.volume.spec.partition_label, v.volumeStatus?.spec.phase]),
    ).toEqual([['u-raid-data', 'ready']])

    for (const id of ['vdc', 'vdd']) {
      expect(topology.byID.get(id)!.usedBy).toEqual(['md0'])
    }
  })

  test('Talos 1.14 with encrypted volumes, one holding an LVM physical volume', () => {
    const topology = buildDiskTopology(talos114LuksLvm)

    // The two opened LUKS devices, dm-0 and dm-1, are not shown.
    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => n.id)).toEqual(['dm-2'])

    expect(topology.byID.get('dm-2')).toMatchObject({
      kind: 'lvm',
      name: 'vg0/data',
      backedBy: ['vda'],
    })

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

  test('Talos 1.15 in maintenance mode', () => {
    const topology = buildDiskTopology(talos115Maintenance)

    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual).toEqual([])
  })

  test('Talos 1.15 with an MD RAID1 array over vdc and vdd', () => {
    const topology = buildDiskTopology(talos115MdRaid1)

    expect(topology.virtual.map((n) => n.id)).toEqual(['md0'])
    expect(topology.byID.get('md0')).toMatchObject({ kind: 'md', backedBy: ['vdc', 'vdd'] })
  })

  // Talos 1.15 reports the device-mapper kind, so no logical volume statuses are needed.
  test.each([
    ['with', talos115LuksLvmRaid1],
    ['without', { ...talos115LuksLvmRaid1, logicalVolumes: [] }],
  ])('Talos 1.15 with LVM on LUKS and LVM RAID1, %s logical volume statuses', (_, machine) => {
    const topology = buildDiskTopology(machine)

    expect(topology.physical.map((n) => n.id)).toEqual(['vda', 'vdb', 'vdc', 'vdd'])
    expect(topology.virtual.map((n) => [n.id, n.kind, n.name, n.backedBy])).toEqual([
      ['dm-4', 'lvm', 'data/data', ['vdc', 'vdd']],
      ['dm-7', 'lvm', 'vg0/secure', ['vda']],
    ])

    expect(topology.byID.get('vda')!.volumes.map((v) => v.usedBy)).toEqual([[], ['dm-7']])
  })

  test('Talos 1.15 reports the RAID level of an LVM RAID1 logical volume', () => {
    expect(buildDiskTopology(talos115LuksLvmRaid1).byID.get('dm-4')!.raidLevel).toBe('raid1')
  })

  test('Talos 1.15 reports partition offsets after formatting EPHEMERAL', () => {
    const vdb = buildDiskTopology(talos115MdRaid1).byID.get('vdb')!

    expect(vdb.volumes.map((v) => [v.volume.spec.partition_label, v.volume.spec.offset])).toEqual([
      ['EFI', 1048576],
      ['META', 2204106752],
      ['STATE', 2205155328],
      ['EPHEMERAL', 2310012928],
    ])
  })
})
