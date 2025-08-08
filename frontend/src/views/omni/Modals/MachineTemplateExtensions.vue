<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window flex flex-col gap-4" style="height: 90%">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Set Extensions</h3>
      <close-button @click="close" />
    </div>

    <extensions-picker v-model="requestedExtensions" :talos-version="talosVersion" class="flex-1" />

    <div class="flex justify-between gap-4">
      <t-button @click="close" type="secondary"> Cancel </t-button>
      <div class="flex gap-4">
        <t-button @click="() => updateExtensions(requestedExtensions)" type="highlighted">
          Save
        </t-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import ExtensionsPicker from '@/views/omni/Extensions/ExtensionsPicker.vue'
import { closeModal } from '@/modal'
import { ref, toRefs } from 'vue'

const props = defineProps<{
  talosVersion: string
  modelValue?: string[]
  onSave: (e?: string[]) => void
}>()

const { talosVersion, modelValue } = toRefs(props)

const close = () => {
  closeModal()
}

const requestedExtensions = ref<Record<string, boolean>>({})

if (modelValue.value) {
  for (const key of modelValue.value) {
    requestedExtensions.value[key] = true
  }
}

const updateExtensions = (extensions?: Record<string, boolean>) => {
  if (extensions === undefined) {
    props.onSave()
  } else {
    const list: string[] = []
    for (const key in extensions) {
      if (!extensions[key]) {
        continue
      }

      list.push(key)
    }

    list.sort()

    props.onSave(list)
  }

  close()
}
</script>

<style scoped>
.modal-window {
  @apply w-1/2 h-auto p-8;
}

.heading {
  @apply flex justify-between items-center text-xl text-naturals-N14;
}
</style>
