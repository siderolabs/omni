// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { DeviceSpec, DiskSpec, VolumeStatusSpec } from '@/api/talos/block.pb'
import type { LVMLogicalVolumeStatusSpec } from '@/api/talos/storage.pb'

import {
  classifyDeviceMapper,
  type DeviceMapperContext,
  getLogicalVolumeName,
  getLogicalVolumeRaidLevel,
  isDeviceMapper,
  isPartitionMapping,
} from './deviceMapper'

const disk = (id: string, spec: DiskSpec = {}): Resource<DiskSpec> => ({
  metadata: { id },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

const context = ({
  blockDevices = [],
  logicalVolumes = [],
  volumeStatuses = [],
}: {
  blockDevices?: Resource<DeviceSpec>[]
  logicalVolumes?: Resource<LVMLogicalVolumeStatusSpec>[]
  volumeStatuses?: Resource<VolumeStatusSpec>[]
} = {}): DeviceMapperContext => ({
  blockDevices: new Map(blockDevices.map((d) => [d.metadata.id!, d])),
  logicalVolumes,
  volumeStatuses,
})

// dm-0 is 253:0, and an LV reports the same kernel device number.
const lvContext = () =>
  context({
    blockDevices: [{ metadata: { id: 'dm-0' }, spec: { major: 253, minor: 0 } }],
    logicalVolumes: [
      {
        metadata: { id: 'lv' },
        spec: { fullName: 'vg0/data', kernelMajor: '253', kernelMinor: '0' },
      },
    ],
  })

describe('isDeviceMapper', () => {
  test.each([
    ['dm-0', {}, true],
    ['sda', {}, false],
    ['sda', { device_mapper_name: 'vg0-data' }, true],
  ])('%s %o -> %s', (id, spec, expected) => {
    expect(isDeviceMapper(disk(id, spec))).toBe(expected)
  })
})

describe('classifyDeviceMapper', () => {
  test.each([
    ['mpath', 'multipath'],
    ['lvm', 'lvm'],
    ['crypt', 'crypt'],
    ['dm', 'dm'],
  ])('uses the reported kind %s (Talos 1.15+)', (kind, expected) => {
    expect(classifyDeviceMapper(disk('dm-0', { device_mapper_kind: kind }), context())).toBe(
      expected,
    )
  })

  test.each([
    ['LVM-abc', 'lvm'],
    ['CRYPT-LUKS2-abc', 'crypt'],
    ['mpath-3600', 'multipath'],
    ['part1-mpath-3600', 'multipath'],
    ['something-else', 'dm'],
  ])('reads the kind from the dm-uuid symlink %s', (uuid, expected) => {
    expect(
      classifyDeviceMapper(
        disk('dm-0', {
          symlinks: [`/dev/disk/by-id/dm-uuid-${uuid}`, '/dev/disk/by-id/dm-name-x'],
        }),
        context(),
      ),
    ).toBe(expected)
  })

  test('matches a logical volume by kernel device number (Talos 1.14)', () => {
    expect(classifyDeviceMapper(disk('dm-0'), lvContext())).toBe('lvm')
  })

  test('recognises the device holding an encrypted volume', () => {
    const ctx = context({
      volumeStatuses: [
        {
          metadata: { id: 'EPHEMERAL' },
          spec: { encryptionProvider: 'luks2', mountLocation: '/dev/dm-0' },
        },
      ],
    })

    expect(classifyDeviceMapper(disk('dm-0'), ctx)).toBe('crypt')
  })

  test('falls back to a generic device-mapper device', () => {
    expect(classifyDeviceMapper(disk('dm-0'), context())).toBe('dm')
  })
})

describe('getLogicalVolumeName', () => {
  test('uses the logical volume status name', () => {
    expect(getLogicalVolumeName(disk('dm-0'), lvContext())).toBe('vg0/data')
  })

  test('decodes the device-mapper name of a logical volume', () => {
    expect(
      getLogicalVolumeName(
        disk('dm-0', { device_mapper_kind: 'lvm', device_mapper_name: 'my--vg-my--data' }),
        context(),
      ),
    ).toBe('my-vg/my-data')
  })

  test('is undefined for other devices', () => {
    expect(
      getLogicalVolumeName(
        disk('dm-0', { device_mapper_kind: 'crypt', device_mapper_name: 'luks-x' }),
        context(),
      ),
    ).toBeUndefined()
  })
})

describe('getLogicalVolumeRaidLevel', () => {
  const withLayout = (layout: string) => {
    const ctx = lvContext()
    ctx.logicalVolumes[0].spec.layout = layout

    return getLogicalVolumeRaidLevel(disk('dm-0'), ctx)
  }

  test.each([
    ['raid,raid1', 'raid1'],
    ['raid,raid10', 'raid10'],
    ['linear', undefined],
    ['', undefined],
  ])('%s -> %s', (layout, expected) => {
    expect(withLayout(layout)).toBe(expected)
  })
})

describe('isPartitionMapping', () => {
  test.each([
    [{ device_mapper_uuid: 'part1-mpath-3600a0980' }, true],
    [{ symlinks: ['/dev/disk/by-id/dm-uuid-part2-LVM-abc'] }, true],
    [{ device_mapper_uuid: 'mpath-3600a0980' }, false],
    [{}, false],
  ])('%o -> %s', (spec, expected) => {
    expect(isPartitionMapping(disk('dm-1', spec))).toBe(expected)
  })
})
