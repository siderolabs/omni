<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import { formStateToPreset } from '@/views/omni/InstallationMedia/formStateToPreset'
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
        spec: formStateToPreset(formState),
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
