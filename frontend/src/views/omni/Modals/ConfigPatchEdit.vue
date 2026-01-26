<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { defineAsyncComponent, ref, watchEffect } from 'vue'

import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import TButton from '@/components/common/Button/TButton.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { closeModal } from '@/modal'

const CodeEditor = defineAsyncComponent(
  () => import('@/components/common/CodeEditor/CodeEditor.vue'),
)

interface Props {
  onSave: (config: string, id?: string) => void
  id: string
  config: string
  talosVersion?: string
}

const { id, config, onSave } = defineProps<Props>()

const configChanges = ref('')

watchEffect(() => {
  configChanges.value = config
})

const err = ref<null | { message: string; title: string }>(null)

const close = () => {
  closeModal()
}

const saveAndClose = async () => {
  err.value = null

  try {
    await ManagementService.ValidateConfig({ config: configChanges.value })
    onSave?.(configChanges.value, id)

    close()
  } catch (e) {
    err.value = {
      title:
        e instanceof RequestError && e.code === Code.INVALID_ARGUMENT
          ? 'The Config is Invalid'
          : 'Failed to Validate the Config',
      message: e instanceof Error ? e.message : String(e),
    }
  }
}
</script>

<template>
  <div class="modal-window" style="min-height: 350px">
    <div class="mt-7 mb-4 flex items-center px-8">
      <h1 class="heading">Edit Config Patch</h1>
      <div class="flex flex-1 justify-end">
        <TButton
          is="a"
          :href="getDocsLink('talos', '/reference/configuration/overview', { talosVersion })"
          rel="noopener noreferrer"
          target="_blank"
          icon="question"
          type="subtle"
          size="xs"
          icon-position="left"
        >
          Config Reference
        </TButton>
      </div>
    </div>

    <h2 class="mb-3 px-8 text-sm text-naturals-n14">
      {{ id }}
    </h2>

    <div class="font-sm relative flex-1 overflow-y-auto bg-naturals-n2">
      <TAlert
        v-if="err"
        class="absolute inset-x-8 top-16 z-50"
        type="error"
        :title="err.title"
        :dismiss="{ action: () => (err = null), name: 'Close' }"
      >
        {{ err.message }}
      </TAlert>

      <CodeEditor
        :value="configChanges"
        :talos-version="talosVersion"
        @update:value="(updated) => (configChanges = updated)"
      />
    </div>
    <div class="flex justify-between gap-4 rounded-b bg-naturals-n3 p-4">
      <TButton type="secondary" @click="close">Cancel</TButton>
      <TButton @click="saveAndClose">Save</TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-2/3 w-2/3 p-0;
}
.heading {
  @apply text-xl text-naturals-n14;
}
</style>
