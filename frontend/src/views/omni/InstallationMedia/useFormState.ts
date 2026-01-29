// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useSessionStorage } from '@vueuse/core'
import { watch } from 'vue'

import type { SchematicBootloader } from '@/api/omni/management/management.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import type { LabelSelectItem } from '@/components/common/Labels/Labels.vue'

export type HardwareType = 'metal' | 'cloud' | 'sbc'

export interface FormState {
  hardwareType?: HardwareType
  talosVersion?: string
  useGrpcTunnel?: boolean
  joinToken?: string
  machineUserLabels?: Record<string, LabelSelectItem>
  machineArch?: PlatformConfigSpecArch
  secureBoot?: boolean
  cloudPlatform?: string
  sbcType?: string
  systemExtensions?: string[]
  cmdline?: string
  overlayOptions?: string
  bootloader?: SchematicBootloader
}

export function useFormState() {
  const formState = useSessionStorage<FormState>('_installation_media_form', {})

  function isStepValid(step: string) {
    switch (step) {
      case 'InstallationMediaCreateEntry':
        return !!formState.value.hardwareType
      case 'InstallationMediaCreateTalosVersion':
        return !!formState.value.talosVersion && !!formState.value.joinToken
      case 'InstallationMediaCreateCloudProvider':
        return !!formState.value.cloudPlatform
      case 'InstallationMediaCreateSBCType':
        return !!formState.value.sbcType
      case 'InstallationMediaCreateMachineArch':
        return !!formState.value.machineArch
      case 'InstallationMediaCreateExtraArgs':
        return typeof formState.value.bootloader !== 'undefined'
      default:
        return true
    }
  }

  // Reset cloud/sbc if we change hardware type or talos version (talos versions change sbc/cloud list)
  watch(
    () => [formState.value.hardwareType, formState.value.talosVersion],
    ([hardwareType]) => {
      switch (hardwareType) {
        case 'metal':
          formState.value.cloudPlatform = undefined
          formState.value.sbcType = undefined
          formState.value.overlayOptions = undefined
          break
        case 'cloud':
          formState.value.sbcType = undefined
          formState.value.overlayOptions = undefined
          break
        case 'sbc':
          formState.value.cloudPlatform = undefined
          break
      }
    },
  )

  // Reset arch, if we change sbc/cloud
  watch(
    () => [formState.value.sbcType, formState.value.cloudPlatform],
    ([sbc]) => {
      formState.value.machineArch = sbc ? PlatformConfigSpecArch.ARM64 : undefined
    },
  )

  return { formState, isStepValid }
}
