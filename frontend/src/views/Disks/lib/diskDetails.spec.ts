// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { DiskSpec } from '@/api/talos/block.pb'

import { getDiskDetails, getDiskIdentity, getDiskMediaType } from './diskDetails'

const disk = (spec: DiskSpec): Resource<DiskSpec> => ({ metadata: { id: 'sda' }, spec })

describe('getDiskIdentity', () => {
  test.each([
    [{ model: 'WDC WD40EFPX', serial: 'WD-123' }, ['WDC WD40EFPX', 'WD-123']],
    [{ model: 'QEMU HARDDISK' }, ['QEMU HARDDISK']],
    [{}, []],
  ])('%o -> %o', (spec, expected) => {
    expect(getDiskIdentity(disk(spec))).toEqual(expected)
  })
})

describe('getDiskMediaType', () => {
  test.each([
    [{ rotational: true }, 'HDD'],
    [{ rotational: false }, 'SSD'],
    [{}, 'SSD'],
    [{ cdrom: true, rotational: true }, undefined],
  ])('%o -> %s', (spec, expected) => {
    expect(getDiskMediaType(disk(spec))).toBe(expected)
  })
})

describe('getDiskDetails', () => {
  test('lists the identifiers the disk has, by-id and by-path links only', () => {
    expect(
      getDiskDetails(
        disk({
          wwid: 'naa.50014ee2b1234567',
          bus_path: '/pci0000:00/0000:00:17.0/ata1',
          symlinks: [
            '/dev/disk/by-diskseq/16',
            '/dev/disk/by-id/wwn-0x50014ee2b1234567',
            '/dev/disk/by-path/pci-0000:00:17.0-ata-1',
            '/dev/disk/by-uuid/81574e9b',
          ],
        }),
      ),
    ).toEqual([
      { label: 'WWID', values: ['naa.50014ee2b1234567'] },
      { label: 'Bus path', values: ['/pci0000:00/0000:00:17.0/ata1'] },
      { label: 'By ID', values: ['/dev/disk/by-id/wwn-0x50014ee2b1234567'] },
      { label: 'By path', values: ['/dev/disk/by-path/pci-0000:00:17.0-ata-1'] },
    ])
  })

  test('leaves out the bus path of a virtual device', () => {
    expect(getDiskDetails(disk({ bus_path: '/virtual' }))).toEqual([])
  })
})
