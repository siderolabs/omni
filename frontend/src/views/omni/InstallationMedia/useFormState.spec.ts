// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { waitFor } from '@testing-library/vue'
import { beforeEach, describe, expect, test } from 'vitest'

import { SchematicBootloader } from '@/api/omni/management/management.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { type FormState, useFormState } from '@/views/omni/InstallationMedia/useFormState'

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

  test('clears cloud/sbc when hardwareType changes', async () => {
    const { formState } = useFormState()

    formState.value.hardwareType = 'sbc'
    formState.value.sbcType = 'board'
    formState.value.overlayOptions = 'option'

    expect(formState.value.hardwareType).toBe('sbc')
    expect(formState.value.sbcType).toBe('board')
    expect(formState.value.overlayOptions).toBe('option')

    formState.value.hardwareType = 'metal'

    await waitFor(() => {
      expect(formState.value.hardwareType).toBe('metal')
      expect(formState.value.cloudPlatform).toBeUndefined()
      expect(formState.value.sbcType).toBeUndefined()
      expect(formState.value.overlayOptions).toBeUndefined()
    })

    formState.value.hardwareType = 'cloud'
    formState.value.cloudPlatform = 'aws'

    await waitFor(() => {
      expect(formState.value.hardwareType).toBe('cloud')
      expect(formState.value.cloudPlatform).toBe('aws')
      expect(formState.value.sbcType).toBeUndefined()
      expect(formState.value.overlayOptions).toBeUndefined()
    })

    formState.value.hardwareType = 'metal'

    await waitFor(() => {
      expect(formState.value.hardwareType).toBe('metal')
      expect(formState.value.cloudPlatform).toBeUndefined()
      expect(formState.value.sbcType).toBeUndefined()
      expect(formState.value.overlayOptions).toBeUndefined()
    })
  })

  test('resets arch if cloud/sbc changes', async () => {
    const { formState } = useFormState()

    formState.value.sbcType = 'board'

    await waitFor(() => {
      expect(formState.value.machineArch).toBe(PlatformConfigSpecArch.ARM64)
    })

    formState.value.sbcType = undefined
    formState.value.cloudPlatform = 'aws'

    await waitFor(() => {
      expect(formState.value.machineArch).toBeUndefined()
    })

    formState.value.cloudPlatform = undefined
    formState.value.hardwareType = 'metal'

    await waitFor(() => {
      expect(formState.value.machineArch).toBeUndefined()
    })
  })

  test.each<[string, FormState, boolean]>([
    ['UnknownStep', {}, true],
    ['InstallationMediaCreateEntry', {}, false],
    ['InstallationMediaCreateEntry', { hardwareType: 'metal' }, true],
    ['InstallationMediaCreateTalosVersion', {}, false],
    ['InstallationMediaCreateTalosVersion', { talosVersion: '1.2.3' }, false],
    ['InstallationMediaCreateTalosVersion', { joinToken: 'xyz' }, false],
    ['InstallationMediaCreateTalosVersion', { talosVersion: '1.2.3', joinToken: 'xyz' }, true],
    ['InstallationMediaCreateCloudProvider', {}, false],
    ['InstallationMediaCreateCloudProvider', { cloudPlatform: 'aws' }, true],
    ['InstallationMediaCreateSBCType', {}, false],
    ['InstallationMediaCreateSBCType', { sbcType: 'board' }, true],
    ['InstallationMediaCreateMachineArch', {}, false],
    ['InstallationMediaCreateMachineArch', { machineArch: PlatformConfigSpecArch.AMD64 }, true],
    ['InstallationMediaCreateExtraArgs', {}, false],
    ['InstallationMediaCreateExtraArgs', { bootloader: SchematicBootloader.BOOT_AUTO }, true],
  ])('isValidStep for %s %j should be %s', (step, state, expected) => {
    const { formState, isStepValid } = useFormState()

    formState.value = state

    expect(isStepValid(step)).toBe(expected)
  })
})
