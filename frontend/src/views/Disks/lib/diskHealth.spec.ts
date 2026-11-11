// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import {
  getArraysWithoutDevice,
  getHealthAlertType,
  getLogicalVolumeHealth,
  getMDArrayHealth,
  getSMARTDetails,
  getSMARTHealth,
  getVolumeGroupsWithoutDevice,
} from './diskHealth'

const members = ['/dev/sda', '/dev/sdb']

describe('getMDArrayHealth', () => {
  test.each([
    [{ status: 'ready', syncAction: 'idle', raidDevices: 2, members }, 'Healthy', 'success'],
    // As Talos reports a degraded mirror: two slots, one member.
    [
      { status: 'ready', syncAction: 'idle', raidDevices: 2, members: ['/dev/sda'] },
      'Degraded',
      'warning',
    ],
    // A spare is a member too, so a mirror with a spare isn't degraded.
    [{ status: 'ready', raidDevices: 2, members: [...members, '/dev/sdc'] }, 'Healthy', 'success'],
    // Talos reports any sync but idle as rebuilding, a routine check too.
    [{ status: 'rebuilding', syncAction: 'check', raidDevices: 2, members }, 'Checking', 'info'],
    [{ status: 'rebuilding', syncAction: 'repair', raidDevices: 2, members }, 'Repairing', 'info'],
    [
      { status: 'rebuilding', syncAction: 'recover', raidDevices: 2, members },
      'Rebuilding',
      'warning',
    ],
    [
      { status: 'rebuilding', syncAction: 'resync', raidDevices: 2, members },
      'Resyncing',
      'warning',
    ],
    [
      { status: 'rebuilding', syncAction: 'frozen', raidDevices: 2, members },
      'Sync frozen',
      'warning',
    ],
    [{ status: 'waiting', error: 'waiting for enough member disks' }, 'Waiting', 'info'],
    [{ status: 'error', error: 'mdadm failed' }, 'Failed', 'danger'],
  ])('%o -> %s', (array, label, tone) => {
    expect(getMDArrayHealth(array)).toMatchObject({ label, tone })
  })

  test('says how many members are present when degraded', () => {
    expect(
      getMDArrayHealth({ status: 'ready', raidDevices: 2, members: ['/dev/sda'] }).message,
    ).toBe('1 of 2 members are present.')
  })

  test('shows a degraded array checking as degraded', () => {
    expect(
      getMDArrayHealth({
        status: 'rebuilding',
        syncAction: 'check',
        raidDevices: 2,
        members: ['/dev/sda'],
      }).label,
    ).toBe('Degraded')
  })
})

describe('getHealthAlertType', () => {
  test.each([
    ['danger', 'error'],
    ['warning', 'warn'],
    ['info', 'info'],
    ['success', 'info'],
  ] as const)('%s -> %s', (tone, type) => {
    expect(getHealthAlertType({ label: '', tone, glyph: 'neutral' })).toBe(type)
  })
})

describe('getLogicalVolumeHealth', () => {
  test.each([
    [{ active: 'active' }, { name: 'vg0', missingPVCount: '0' }, 'Healthy'],
    [{ active: '' }, { name: 'vg0', missingPVCount: '0' }, 'Inactive'],
    [{ active: 'active' }, { name: 'vg0', missingPVCount: '1', partial: 'partial' }, 'Degraded'],
    [{ active: 'active' }, undefined, 'Healthy'],
  ])('%o in %o -> %s', (volume, group, label) => {
    expect(getLogicalVolumeHealth(volume, group).label).toBe(label)
  })

  test('says how many physical volumes are missing', () => {
    expect(
      getLogicalVolumeHealth({ active: 'active' }, { name: 'data', missingPVCount: '1' }).message,
    ).toBe('Volume group data is missing 1 physical volume.')
  })
})

describe('getSMARTHealth', () => {
  test('is healthy as Talos reports', () => {
    expect(getSMARTHealth({ healthy: true }).label).toBe('Healthy')
  })

  test('explains an NVMe critical warning when Talos gives no reason', () => {
    expect(getSMARTHealth({ healthy: false, dev_type: 'nvme', critical_warning: 4 }).message).toBe(
      'Reliability is degraded by media or internal errors.',
    )
    expect(getSMARTHealth({ healthy: false, critical_warning: 0b1010 }).message).toBe(
      'Temperature is outside its thresholds.\nMedia is read-only.',
    )
  })

  test('carries the reason a disk is failing', () => {
    expect(getSMARTHealth({ healthy: false, message: 'media errors: 12' })).toMatchObject({
      label: 'Failing',
      tone: 'danger',
      message: 'media errors: 12',
    })
  })
})

describe('getSMARTDetails', () => {
  test('lists what is reported, and only failing attributes', () => {
    expect(
      getSMARTDetails({
        temperature: 31,
        power_on_hours: 12345,
        percent_used: 3,
        attributes: [
          { name: 'Raw_Read_Error_Rate', failing: false },
          { name: 'Reallocated_Sector_Ct', failing: true },
        ],
      }),
    ).toEqual([
      { label: 'Temperature', values: ['31 °C'] },
      { label: 'Power-on hours', values: ['12,345'] },
      { label: 'Wear', values: ['3%'] },
      { label: 'Failing attributes', values: ['Reallocated_Sector_Ct'] },
    ])
  })
})

test('getSMARTDetails leaves out failing attributes when none are failing', () => {
  expect(
    getSMARTDetails({
      temperature: 30,
      attributes: [{ name: 'Raw_Read_Error_Rate', failing: false }],
    }),
  ).toEqual([{ label: 'Temperature', values: ['30 °C'] }])
})

describe('getArraysWithoutDevice', () => {
  test('returns arrays whose by-id path no disk has', () => {
    const arrays = [
      { metadata: { id: 'data' }, spec: { device: '/dev/disk/by-id/md-name-talos:data' } },
      { metadata: { id: 'logs' }, spec: { device: '/dev/disk/by-id/md-name-talos:logs' } },
    ]
    const disks = [
      { metadata: { id: 'md0' }, spec: { symlinks: ['/dev/disk/by-id/md-name-talos:data'] } },
    ]

    expect(getArraysWithoutDevice(arrays, disks).map((a) => a.metadata.id)).toEqual(['logs'])
  })
})

describe('getVolumeGroupsWithoutDevice', () => {
  const groups = [{ metadata: { id: 'vg0' }, spec: { name: 'vg0', missingPVCount: '1' } }]
  const volumes = [
    { metadata: { id: 'vg0-data' }, spec: { fullName: 'vg0/data', vgName: 'vg0', role: 'public' } },
  ]

  test('leaves out a group whose logical volume is shown, which shows the problem itself', () => {
    expect(getVolumeGroupsWithoutDevice(groups, volumes, ['vg0/data'])).toEqual([])
  })

  test('leaves out a healthy group', () => {
    expect(
      getVolumeGroupsWithoutDevice(
        [{ metadata: { id: 'vg0' }, spec: { name: 'vg0' } }],
        volumes,
        [],
      ),
    ).toEqual([])
  })
})
