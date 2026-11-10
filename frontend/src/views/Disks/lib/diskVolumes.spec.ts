// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { DiscoveredVolumeSpec, DiskSpec } from '@/api/talos/block.pb'

import {
  getDiskSegments,
  getDiskVolumes,
  getVolumeDisplayName,
  isClaimedVolume,
} from './diskVolumes'

const disk = (id: string, spec: DiskSpec = {}): Resource<DiskSpec> => ({
  metadata: { id },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

const volume = (id: string, spec: DiscoveredVolumeSpec = {}): Resource<DiscoveredVolumeSpec> => ({
  metadata: { id },
  spec: { dev_path: `/dev/${id}`, ...spec },
})

describe('getDiskVolumes', () => {
  test('returns partitions ordered by offset when known', () => {
    const volumes = [
      volume('sda', { type: 'disk', name: 'gpt' }),
      volume('sda2', { parent: 'sda', partition_index: 1, offset: 200 }),
      volume('sda1', { parent: 'sda', partition_index: 2, offset: 100 }),
      volume('sdb1', { parent: 'sdb', partition_index: 1, offset: 100 }),
    ]

    expect(getDiskVolumes(disk('sda'), volumes).map((v) => v.metadata.id)).toEqual(['sda1', 'sda2'])
  })

  test('falls back to partition index without offsets', () => {
    const volumes = [
      volume('sda2', { parent: 'sda', partition_index: 2 }),
      volume('sda1', { parent: 'sda', partition_index: 1 }),
    ]

    expect(getDiskVolumes(disk('sda'), volumes).map((v) => v.metadata.id)).toEqual(['sda1', 'sda2'])
  })

  test.each(['linux_raid_member', 'lvm2-pv', 'luks', 'xfs'])(
    'returns the whole-disk volume of an unpartitioned disk with a %s signature',
    (name) => {
      const volumes = [volume('sda', { type: 'disk', name })]

      expect(getDiskVolumes(disk('sda'), volumes)).toEqual(volumes)
    },
  )

  test('ignores the whole-disk volume of a partition table without partitions', () => {
    expect(getDiskVolumes(disk('sda'), [volume('sda', { type: 'disk', name: 'gpt' })])).toEqual([])
  })

  test('ignores the whole-disk volume of a blank disk', () => {
    expect(getDiskVolumes(disk('sda'), [volume('sda', { type: 'disk' })])).toEqual([])
  })
})

describe('isClaimedVolume', () => {
  test.each([
    ['linux_raid_member', true],
    ['lvm2-pv', true],
    ['xfs', false],
    ['luks', false],
    [undefined, false],
  ])('%s -> %s', (name, expected) => {
    expect(isClaimedVolume(volume('sda', { name }))).toBe(expected)
  })
})

describe('getVolumeDisplayName', () => {
  test('prefers the partition label', () => {
    expect(getVolumeDisplayName(volume('sda1', { partition_label: 'EPHEMERAL', label: 'x' }))).toBe(
      'EPHEMERAL',
    )
  })

  test('names member signatures by type instead of by their label', () => {
    expect(
      getVolumeDisplayName(volume('sda', { name: 'linux_raid_member', label: 'talos:0' })),
    ).toBe('RAID member')
    expect(getVolumeDisplayName(volume('sda', { name: 'luks2' }))).toBe('LUKS')
  })

  test('uses the filesystem label, then the volume ID', () => {
    expect(getVolumeDisplayName(volume('sda', { name: 'xfs', label: 'data' }))).toBe('data')
    expect(getVolumeDisplayName(volume('sda', { name: 'xfs' }))).toBe('sda')
  })
})

describe('getDiskSegments', () => {
  test('places free space between volumes by offset', () => {
    const a = volume('sda1', { offset: 10, size: 20 })
    const b = volume('sda2', { offset: 50, size: 30 })

    expect(getDiskSegments(100, [a, b])).toEqual([
      { type: 'unallocated', offset: 0, size: 10 },
      { type: 'volume', volume: a, size: 20 },
      { type: 'unallocated', offset: 30, size: 20 },
      { type: 'volume', volume: b, size: 30 },
      { type: 'unallocated', offset: 80, size: 20 },
    ])
  })

  test('puts all free space at the end without offsets', () => {
    const a = volume('sda1', { size: 20 })
    const b = volume('sda2', { size: 30 })

    expect(getDiskSegments(100, [a, b])).toEqual([
      { type: 'volume', volume: a, size: 20 },
      { type: 'volume', volume: b, size: 30 },
      { type: 'unallocated', offset: 50, size: 50 },
    ])
  })

  test('fills the disk with a whole-disk volume', () => {
    const a = volume('sda', { offset: 0, size: 100 })

    expect(getDiskSegments(100, [a])).toEqual([{ type: 'volume', volume: a, size: 100 }])
  })

  test('treats an empty disk as unallocated', () => {
    expect(getDiskSegments(100, [])).toEqual([{ type: 'unallocated', offset: 0, size: 100 }])
  })
})
