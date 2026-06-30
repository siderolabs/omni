<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { type InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import { formStateToPreset } from '@/views/InstallationMedia/formStateToPreset'
import type { FormState } from '@/views/InstallationMedia/useFormState'

const { formState } = defineProps<{
  formState: FormState
}>()

const open = defineModel<boolean>('open', { default: false })

const emit = defineEmits<{
  saved: [name: string]
}>()

const name = ref('')
const saving = ref(false)

watchEffect(() => {
  if (open.value) return

  name.value = ''
  saving.value = false
})

const { data: existingPreset, loading: existingPresetLoading } =
  useResourceWatch<InstallationMediaConfigSpec>(() => ({
    skip: !open.value || !name.value || saving.value,
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

    showSuccess(`Preset saved as ${name.value}`)

    open.value = false
  } catch (error) {
    showError('Error', error instanceof Error ? error.message : String(error))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Save preset"
    :loading="saving"
    action-label="Save"
    :action-disabled="!name || existingPresetLoading || !!existingPreset"
    @confirm="save"
  >
    <div class="flex flex-col gap-1">
      <TInput v-model.trim="name" title="Name" :focus="open" />
      <p v-if="name && existingPreset" class="ml-2.5 text-xs text-red-r1">Name already in use</p>
    </div>
  </Modal>
</template>
