<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toValue, watchEffect } from 'vue'

import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import TButton from '@/components/Button/TButton.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import Modal from '@/components/Modals/Modal.vue'
import { getDocsLink } from '@/methods'
import { showError } from '@/notification'

interface Props {
  id: string
  config?: string
  talosVersion?: string
}

const { id, config } = defineProps<Props>()

const emit = defineEmits<{
  save: [config: string]
}>()

const open = defineModel<boolean>('open', { default: false })

const configChanges = ref('')
const isSaving = ref(false)

watchEffect(() => {
  if (open.value) return

  configChanges.value = ''
  isSaving.value = false
})

watchEffect(() => {
  if (!open.value) return

  configChanges.value = config ?? ''
})

const saveAndClose = async () => {
  isSaving.value = true

  const config = toValue(configChanges)

  try {
    await ManagementService.ValidateConfig({ config })

    emit('save', config)

    open.value = false
  } catch (e) {
    showError(
      e instanceof RequestError && e.code === Code.INVALID_ARGUMENT
        ? 'The Config is Invalid'
        : 'Failed to Validate the Config',
      e instanceof Error ? e.message : String(e),
    )
  } finally {
    isSaving.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Edit Config Patch"
    action-label="Save"
    disable-content-padding
    content-class="w-3xl max-w-screen"
    :loading="isSaving"
    @confirm="saveAndClose"
  >
    <template #description>{{ id }}</template>

    <div class="flex flex-col gap-3">
      <div class="self-end px-8">
        <TButton
          is="a"
          :href="getDocsLink('talos', '/reference/configuration/overview', { talosVersion })"
          rel="noopener noreferrer"
          target="_blank"
          icon="question"
          variant="subtle"
          size="xs"
          icon-position="left"
        >
          Config Reference
        </TButton>
      </div>

      <CodeEditor
        v-model="configChanges"
        :talos-version="talosVersion"
        class="h-80 w-full bg-naturals-n2"
      />
    </div>
  </Modal>
</template>
