// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import { supportsContainers } from '@/methods/containers'

describe('supportsContainers', () => {
  test.each([
    ['v1.15.0', true],
    ['1.15.0-beta.0', true],
    ['v1.16.2', true],
    ['v1.14.6', false],
    ['', false],
    [undefined, false],
  ])('%s → %s', (version, expected) => {
    expect(supportsContainers(version)).toBe(expected)
  })
})
