// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { MountStatusSpec } from '@/api/talos/block.pb'

import { getVolumeMounts, getVolumeUsages } from './volumeUsage'

const mount = (volumeID: string, spec: MountStatusSpec): Resource<MountStatusSpec> => ({
  metadata: { id: volumeID },
  spec: { spec: { volumeID }, ...spec },
})

describe('getVolumeMounts', () => {
  test('keeps only mounts of a filesystem of their own', () => {
    expect(
      getVolumeMounts([
        mount('EPHEMERAL', { source: '/dev/nvme0n1p4', target: '/var' }),
        // A directory volume, with no filesystem of its own.
        mount('CRI', { source: '', target: '/var/lib/containerd' }),
        mount('u-data', { source: '/dev/sda1', target: '/var/mnt/data', detached: true }),
      ]),
    ).toEqual([{ volumeID: 'EPHEMERAL', target: '/var' }])
  })
})

describe('getVolumeUsages', () => {
  test('returns usage by volume ID, leaving out filesystems with no result', () => {
    expect(
      getVolumeUsages(
        [
          { volumeID: 'EPHEMERAL', target: '/var' },
          { volumeID: 'u-data', target: '/var/mnt/data' },
        ],
        { '/var': { size: '3000', used: '600', available: '2400' } },
      ),
    ).toEqual(
      new Map([['EPHEMERAL', { size: 3000, used: 600, available: 2400, mountedOn: '/var' }]]),
    )
  })
})
