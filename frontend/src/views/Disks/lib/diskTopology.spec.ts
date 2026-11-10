// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { DeviceSpec, DiscoveredVolumeSpec, DiskSpec } from '@/api/talos/block.pb'

import { buildDiskTopology, type DiskTopologyInput } from './diskTopology'

const disk = (id: string, spec: DiskSpec = {}): Resource<DiskSpec> => ({
  metadata: { id },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

const volume = (id: string, spec: DiscoveredVolumeSpec = {}): Resource<DiscoveredVolumeSpec> => ({
  metadata: { id },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

const device = (id: string, secondaries: string[] = []): Resource<DeviceSpec> => ({
  metadata: { id },
  spec: { secondaries },
})

const build = (input: Partial<DiskTopologyInput>) =>
  buildDiskTopology({
    disks: [],
    volumes: [],
    volumeStatuses: [],
    blockDevices: [],
    logicalVolumes: [],
    ...input,
  })

const summary = (topology: ReturnType<typeof build>) => ({
  physical: topology.physical.map((n) => n.id),
  virtual: topology.virtual.map((n) => n.id),
})

describe('buildDiskTopology', () => {
  test('links an MD array and its whole-disk members both ways', () => {
    const topology = build({
      disks: [disk('md127', { secondary_disks: ['sda', 'sdb'] }), disk('sdb'), disk('sda')],
      volumes: [
        volume('sda', { name: 'linux_raid_member' }),
        volume('sdb', { name: 'linux_raid_member' }),
      ],
      blockDevices: [device('md127', ['sda', 'sdb'])],
    })

    expect(summary(topology)).toEqual({ physical: ['sda', 'sdb'], virtual: ['md127'] })

    const md = topology.byID.get('md127')!
    expect(md.kind).toBe('md')
    expect(md.backedBy).toEqual(['sda', 'sdb'])

    const sda = topology.byID.get('sda')!
    expect(sda.usedBy).toEqual(['md127'])
    expect(sda.volumes.map((v) => v.usedBy)).toEqual([['md127']])
  })

  test('links the partitions an MD array is built from', () => {
    const topology = build({
      disks: [disk('sda'), disk('md0', { secondary_disks: ['sda'] })],
      volumes: [
        volume('sda1', { parent: 'sda', partition_index: 1, partition_label: 'EPHEMERAL' }),
        volume('sda2', { parent: 'sda', partition_index: 2, name: 'linux_raid_member' }),
      ],
      blockDevices: [device('md0', ['sda2'])],
    })

    expect(topology.byID.get('sda')!.volumes.map((v) => [v.volume.metadata.id, v.usedBy])).toEqual([
      ['sda1', []],
      ['sda2', ['md0']],
    ])
  })

  test('shows logical volumes by name, and hides encrypted devices behind them', () => {
    // sda2 -> dm-0 (LUKS) -> dm-1 (LVM vg0/data)
    const topology = build({
      disks: [
        disk('sda'),
        disk('dm-0', { device_mapper_kind: 'crypt', secondary_disks: ['sda'] }),
        disk('dm-1', {
          device_mapper_kind: 'lvm',
          device_mapper_name: 'vg0-data',
          secondary_disks: ['dm-0'],
        }),
      ],
      volumes: [volume('sda2', { parent: 'sda', name: 'luks' })],
      blockDevices: [device('dm-0', ['sda2']), device('dm-1', ['dm-0'])],
    })

    expect(summary(topology)).toEqual({ physical: ['sda'], virtual: ['dm-1'] })

    const lv = topology.byID.get('dm-1')!
    expect(lv).toMatchObject({ kind: 'lvm', name: 'vg0/data', backedBy: ['sda'] })
    expect(topology.byID.get('sda')!.usedBy).toEqual(['dm-1'])
    expect(topology.byID.get('sda')!.volumes[0].usedBy).toEqual(['dm-1'])
  })

  test('links a RAID logical volume through the sub-volumes Talos reports no disk for', () => {
    // From a Talos 1.14 machine: data/data is raid1 over sda and sdb. LVM builds it
    // from rmeta/rimage sub-volumes dm-0..dm-3, which are block devices but not disks.
    const lv = (fullName: string, minor: number, layout = 'linear') => ({
      metadata: { id: fullName.replace('/', '-') },
      spec: { fullName, layout, kernelMajor: '252', kernelMinor: String(minor) },
    })

    const blockDevice = (id: string, minor: number, secondaries: string[]) => ({
      metadata: { id },
      spec: { major: 252, minor, secondaries },
    })

    const topology = build({
      disks: [
        disk('sda'),
        disk('sdb'),
        disk('dm-4', {
          secondary_disks: ['dm-0', 'dm-1', 'dm-2', 'dm-3'],
          symlinks: ['/dev/disk/by-id/dm-uuid-LVM-ZaKCnapa371uG3AqIkknE7AdnObUDtdl'],
        }),
      ],
      volumes: [volume('sda', { name: 'lvm2-pv' }), volume('sdb', { name: 'lvm2-pv' })],
      blockDevices: [
        blockDevice('dm-0', 0, ['sda']),
        blockDevice('dm-1', 1, ['sda']),
        blockDevice('dm-2', 2, ['sdb']),
        blockDevice('dm-3', 3, ['sdb']),
        blockDevice('dm-4', 4, ['dm-0', 'dm-1', 'dm-2', 'dm-3']),
      ],
      logicalVolumes: [
        lv('data/data', 4, 'raid,raid1'),
        lv('data/data_rmeta_0', 0),
        lv('data/data_rimage_0', 1),
        lv('data/data_rmeta_1', 2),
        lv('data/data_rimage_1', 3),
      ],
    })

    expect(summary(topology)).toEqual({ physical: ['sda', 'sdb'], virtual: ['dm-4'] })
    expect(topology.byID.get('dm-4')).toMatchObject({
      kind: 'lvm',
      name: 'data/data',
      raidLevel: 'raid1',
      backedBy: ['sda', 'sdb'],
    })
    expect(topology.byID.get('sda')!.usedBy).toEqual(['dm-4'])
    expect(topology.byID.get('sda')!.volumes[0].usedBy).toEqual(['dm-4'])
  })

  test('folds multipath paths into the multipath device', () => {
    const topology = build({
      disks: [
        disk('sdb'),
        disk('sdc'),
        disk('dm-2', {
          device_mapper_kind: 'mpath',
          device_mapper_name: 'mpatha',
          secondary_disks: ['sdb', 'sdc'],
        }),
        disk('dm-3', { device_mapper_kind: 'lvm', secondary_disks: ['dm-2'] }),
      ],
    })

    expect(summary(topology)).toEqual({ physical: ['dm-2'], virtual: ['dm-3'] })
    expect(topology.byID.get('dm-2')).toMatchObject({
      kind: 'multipath',
      name: 'mpatha',
      paths: ['sdb', 'sdc'],
      backedBy: [],
      usedBy: ['dm-3'],
    })
    expect(topology.byID.get('dm-3')!.backedBy).toEqual(['dm-2'])
  })

  test('shows a partition mapping of a multipath device through the device', () => {
    // dm-1 maps partition 1 of the multipath device dm-0, and holds an LVM PV.
    const topology = build({
      disks: [
        disk('sdb'),
        disk('sdc'),
        disk('dm-0', {
          device_mapper_kind: 'mpath',
          device_mapper_name: 'mpatha',
          device_mapper_uuid: 'mpath-3600a0980',
          secondary_disks: ['sdb', 'sdc'],
        }),
        disk('dm-1', {
          device_mapper_kind: 'mpath',
          device_mapper_name: 'mpatha1',
          device_mapper_uuid: 'part1-mpath-3600a0980',
          secondary_disks: ['dm-0'],
        }),
        disk('dm-2', { device_mapper_kind: 'lvm', secondary_disks: ['dm-1'] }),
      ],
    })

    expect(summary(topology)).toEqual({ physical: ['dm-0'], virtual: ['dm-2'] })
    expect(topology.byID.get('dm-0')).toMatchObject({ paths: ['sdb', 'sdc'], usedBy: ['dm-2'] })
    expect(topology.byID.get('dm-2')!.backedBy).toEqual(['dm-0'])
  })

  test('hides loop devices and empty CD-ROMs', () => {
    const topology = build({
      disks: [disk('loop0'), disk('sr0', { cdrom: true }), disk('sr1', { cdrom: true })],
      volumes: [volume('sr0', { name: 'iso9660' }), volume('sr1')],
    })

    expect(summary(topology)).toEqual({ physical: ['sr0'], virtual: [] })
    expect(topology.byID.get('sr0')!.kind).toBe('cdrom')
  })

  test('sorts devices by ID, numerically', () => {
    const topology = build({ disks: [disk('sdb'), disk('nvme10n1'), disk('nvme2n1'), disk('sda')] })

    expect(summary(topology).physical).toEqual(['nvme2n1', 'nvme10n1', 'sda', 'sdb'])
  })

  test('copes with a cycle in the reported links', () => {
    const topology = build({
      disks: [
        disk('dm-0', { device_mapper_kind: 'crypt', secondary_disks: ['dm-1'] }),
        disk('dm-1', { device_mapper_kind: 'crypt', secondary_disks: ['dm-0'] }),
        disk('md0', { secondary_disks: ['dm-0'] }),
      ],
    })

    expect(topology.byID.get('md0')!.backedBy).toEqual([])
  })
})
