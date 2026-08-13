// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { MachineInstallDiskStatusSpec } from '@/api/omni/specs/omni.pb'

import { installDiskFallbackItem, installDiskSelectItems } from './installdisk'

// mirrors a machine with an md array: two candidates, the array as an explicit-only pick, its
// members and a CD-ROM as ineligible disks
const status: MachineInstallDiskStatusSpec = {
  disk: '/dev/vda',
  disks: [
    { dev_path: '/dev/vda', selectable: true },
    { dev_path: '/dev/vdd', selectable: true },
    {
      dev_path: '/dev/md0',
      selectable: true,
      skip_reason: 'built on top of other disks',
      members: ['vdb', 'vdc'],
    },
    { dev_path: '/dev/vdb', skip_reason: 'a member of /dev/md0' },
    { dev_path: '/dev/vdc', skip_reason: 'a member of /dev/md0' },
    { dev_path: '/dev/sr0', skip_reason: 'a CD-ROM drive' },
  ],
}

describe('installDiskSelectItems', () => {
  test('renders the selectable disks in the backend order, labeling the stacked ones', () => {
    expect(installDiskSelectItems(status)).toEqual([
      { label: '/dev/vda', value: '/dev/vda', tooltip: undefined },
      { label: '/dev/vdd', value: '/dev/vdd', tooltip: undefined },
      {
        label: '/dev/md0 (on vdb, vdc)',
        value: '/dev/md0',
        tooltip: 'built on top of other disks',
      },
    ])
  })

  test('renders nothing without a status', () => {
    expect(installDiskSelectItems(undefined)).toEqual([])
  })
})

describe('installDiskFallbackItem', () => {
  test('disables the entry of a reported but non-selectable disk, keeping the reason', () => {
    expect(installDiskFallbackItem('/dev/vdb', status)).toEqual({
      label: '/dev/vdb',
      value: '/dev/vdb',
      tooltip: 'a member of /dev/md0',
      disabled: true,
    })
  })

  test('keeps the entry of an unreported disk pickable', () => {
    expect(installDiskFallbackItem('/dev/nvme0n1', status)).toEqual({
      label: '/dev/nvme0n1',
      value: '/dev/nvme0n1',
      tooltip: undefined,
      disabled: false,
    })
  })
})
