// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'
import { nextTick } from 'vue'

import { useIdentity } from './identity'

describe('useIdentity', () => {
  test('defaults to null', async () => {
    const { identity, avatar, fullname } = useIdentity()

    expect(identity.value).toBeNull()
    expect(avatar.value).toBeNull()
    expect(fullname.value).toBeNull()
  })

  test('sets values', async () => {
    const { identity, avatar, fullname } = useIdentity()

    identity.value = '1'
    avatar.value = '2'
    fullname.value = '3'

    expect(identity.value).toBe('1')
    expect(avatar.value).toBe('2')
    expect(fullname.value).toBe('3')
  })

  test('clears values', async () => {
    const { identity, avatar, fullname, clear } = useIdentity()

    identity.value = '1'
    avatar.value = '2'
    fullname.value = '3'

    clear()

    expect(identity.value).toBeNull()
    expect(avatar.value).toBeNull()
    expect(fullname.value).toBeNull()
  })

  test('persists values', async () => {
    // First pass
    ;(() => {
      const { identity, avatar, fullname } = useIdentity()

      identity.value = '1'
      avatar.value = '2'
      fullname.value = '3'

      expect(identity.value).toBe('1')
      expect(avatar.value).toBe('2')
      expect(fullname.value).toBe('3')
    })()

    // actual storage is updated on the next tick
    await nextTick()

    // Second pass
    ;(() => {
      const { identity, avatar, fullname } = useIdentity()

      expect(identity.value).toBe('1')
      expect(avatar.value).toBe('2')
      expect(fullname.value).toBe('3')
    })()
  })
})
