// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import type { Resource } from '@/api/grpc'
import type { VolumeStatusSpec } from '@/api/talos/block.pb'

import { isVolumePrepared } from './volumeStatus'
import { getFailedVolumes, getVolumePhaseAppearance } from './volumeStatus'

const status = (id: string, spec: VolumeStatusSpec): Resource<VolumeStatusSpec> => ({
  metadata: { id },
  spec,
})

describe('isVolumePrepared', () => {
  test.each([
    ['ready', true],
    ['prepared', true],
    ['closed', true],
    ['provisioned', false],
    ['failed', false],
    [undefined, false],
  ])('%s -> %s', (phase, expected) => {
    expect(isVolumePrepared(phase)).toBe(expected)
  })
})

describe('getVolumePhaseAppearance', () => {
  test.each([
    ['ready', 'Ready', 'success'],
    ['failed', 'Failed', 'danger'],
    ['waiting', 'Waiting', 'info'],
    ['missing', 'Missing', 'info'],
  ])('%s', (phase, label, tone) => {
    expect(getVolumePhaseAppearance(phase)).toMatchObject({ label, tone })
  })

  test('falls back for an unknown phase', () => {
    expect(getVolumePhaseAppearance('new-phase')).toEqual({
      label: 'new-phase',
      tone: 'info',
      glyph: 'unknown',
    })
    expect(getVolumePhaseAppearance()).toMatchObject({ label: 'Unknown' })
  })
})

describe('getFailedVolumes', () => {
  test('returns only failed volumes, sorted by ID', () => {
    const statuses = [
      status('u-logs', { phase: 'failed' }),
      status('EPHEMERAL', { phase: 'ready' }),
      status('u-data', { phase: 'failed' }),
    ]

    expect(getFailedVolumes(statuses).map((s) => s.metadata.id)).toEqual(['u-data', 'u-logs'])
  })
})
