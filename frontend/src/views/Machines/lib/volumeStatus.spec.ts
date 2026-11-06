// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import { isVolumePrepared } from './volumeStatus'

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
