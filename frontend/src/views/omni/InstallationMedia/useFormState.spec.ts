// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test } from 'vitest'

import { useFormState } from '@/views/omni/InstallationMedia/useFormState'

describe('useFormState', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  test('stores state in storage', async () => {
    const { formState } = useFormState()

    formState.value.hardwareType = 'cloud'
    formState.value.talosVersion = '1.9.0'

    await waitFor(() => {
      const stored = JSON.parse(sessionStorage.getItem('_installation_media_form') || '{}')

      expect(stored.hardwareType).toBe('cloud')
      expect(stored.talosVersion).toBe('1.9.0')
    })
  })

  test('loads state from storage', () => {
    sessionStorage.setItem(
      '_installation_media_form',
      JSON.stringify({
        hardwareType: 'metal',
        talosVersion: '1.8.0',
      }),
    )

    const stored = useFormState().formState.value

    expect(stored.hardwareType).toBe('metal')
    expect(stored.talosVersion).toBe('1.8.0')
  })
})
