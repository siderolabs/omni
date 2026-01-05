<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { GrpcTunnelMode, type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import type { FormState } from '@/views/omni/InstallationMedia/InstallationMediaCreate.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const { open, formState } = defineProps<{
  open?: boolean
  formState: FormState
}>()

const emit = defineEmits<{
  close: []
  saved: [name: string]
}>()

const name = ref('')
const saving = ref(false)

const { data: existingPreset, loading: existingPresetLoading } =
  useResourceWatch<InstallationMediaConfigSpec>(() => ({
    skip: !name.value || saving.value,
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: InstallationMediaConfigType,
      id: name.value,
    },
  }))

const arch = computed(() => {
  switch (formState.machineArch) {
    case PlatformConfigSpecArch.AMD64:
      return 'amd64'
    case PlatformConfigSpecArch.ARM64:
      return 'arm64'
    default:
      return formState.hardwareType === 'sbc' ? 'arm64' : undefined
  }
})

async function save() {
  try {
    saving.value = true

    await ResourceService.Create<Resource<InstallationMediaConfigSpec>>(
      {
        metadata: {
          namespace: DefaultNamespace,
          type: InstallationMediaConfigType,
          id: name.value,
        },
        spec: {
          architecture: arch.value,
          bootloader: formState.bootloader,
          cloud:
            formState.hardwareType === 'cloud' ? { platform: formState.cloudPlatform } : undefined,
          sbc:
            formState.hardwareType === 'sbc'
              ? {
                  overlay: formState.sbcType,
                  overlay_options: formState.overlayOptions,
                }
              : undefined,
          grpc_tunnel: formState.useGrpcTunnel ? GrpcTunnelMode.ENABLED : GrpcTunnelMode.DISABLED,
          talos_version: formState.talosVersion,
          install_extensions: formState.systemExtensions,
          join_token: formState.joinToken,
          kernel_args: formState.cmdline,
          machine_labels: formState.machineUserLabels
            ? Object.entries(formState.machineUserLabels).reduce<Record<string, string>>(
                (prev, [key, { value }]) => ({ ...prev, [key]: value }),
                {},
              )
            : undefined,
          secure_boot: formState.secureBoot,
        },
      },
      withRuntime(Runtime.Omni),
    )

    emit('saved', name.value)

    name.value = ''
  } catch (error) {
    showError('Error', error.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-30 flex items-center justify-center bg-naturals-n0/90"
    :class="!open && 'hidden'"
  >
    <div class="modal-window w-auto gap-4">
      <div class="flex items-center justify-between">
        <h3 class="text-base text-naturals-n14">Save preset</h3>

        <CloseButton @click="$emit('close')" />
      </div>

      <div class="flex flex-col gap-1">
        <TInput v-model.trim="name" title="Name" />
        <p v-if="name && existingPreset" class="ml-2.5 text-xs text-red-r1">Name already in use</p>
      </div>

      <div class="flex items-center justify-end gap-2">
        <TSpinner v-show="saving" class="size-4" />

        <TButton
          :disabled="saving || !name || existingPresetLoading || !!existingPreset"
          type="highlighted"
          @click="save"
        >
          Save
        </TButton>
      </div>
    </div>
  </div>
</template>
