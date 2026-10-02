// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import { TalosVersionType } from '@/api/resources'

import { talosUpgradeTargets } from './talosUpgradeTargets'

const version = (
  v: string,
  upgradable: string[],
  opts: Partial<TalosVersionSpec> = {},
): Resource<TalosVersionSpec> => ({
  metadata: { id: v, type: TalosVersionType },
  spec: { version: v, upgradable_talos_versions: upgradable, ...opts },
})

// 1.7 and 1.8 are deprecated (below MinTalosVersion), 1.10.0 is beyond the supported cap
const versionMap = new Map(
  [
    version('1.7.5', ['1.7.6', '1.8.0', '1.8.1'], { deprecated: true }),
    version('1.7.6', ['1.7.5', '1.8.0', '1.8.1'], { deprecated: true }),
    version('1.8.0', ['1.7.5', '1.7.6', '1.8.1', '1.9.0', '1.9.1'], { deprecated: true }),
    version('1.8.1', ['1.7.5', '1.7.6', '1.8.0', '1.9.0', '1.9.1'], { deprecated: true }),
    version('1.9.0', ['1.8.0', '1.8.1', '1.9.1', '1.10.0']),
    version('1.9.1', ['1.8.0', '1.8.1', '1.9.0', '1.10.0']),
    version('1.10.0', ['1.9.0', '1.9.1'], { unsupported: true }),
  ].map((v) => [v.metadata.id!, v]),
)

describe('talosUpgradeTargets', () => {
  test('no current version', () => {
    expect(talosUpgradeTargets(versionMap, undefined)).toEqual({})
  })

  test('offers newer deprecated targets to a machine on a deprecated version', () => {
    expect(talosUpgradeTargets(versionMap, '1.7.5')).toEqual({
      '1.8': { versions: ['1.8.1', '1.8.0'], unsupported: false },
      '1.7': { versions: ['1.7.6', '1.7.5'], unsupported: false },
    })

    expect(talosUpgradeTargets(versionMap, '1.7.6')).toEqual({
      '1.8': { versions: ['1.8.1', '1.8.0'], unsupported: false },
      '1.7': { versions: ['1.7.6'], unsupported: false },
    })

    expect(talosUpgradeTargets(versionMap, '1.8.1')).toEqual({
      '1.9': { versions: ['1.9.1', '1.9.0'], unsupported: false },
      '1.8': { versions: ['1.8.1'], unsupported: false },
    })
  })

  test('hides deprecated targets from a machine on a supported version', () => {
    expect(talosUpgradeTargets(versionMap, '1.9.0')).toEqual({
      '1.10': { versions: [], unsupported: true },
      '1.9': { versions: ['1.9.1', '1.9.0'], unsupported: false },
    })
  })

  test('keeps an untracked current version', () => {
    expect(talosUpgradeTargets(versionMap, '1.6.0')).toEqual({})
  })
})
