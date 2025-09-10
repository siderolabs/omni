<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'

import TButton from '@/components/common/Button/TButton.vue'
import { closeModal } from '@/modal'
import ExtensionsPicker from '@/views/omni/Extensions/ExtensionsPicker.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

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

<template>
  <div class="modal-window flex flex-col gap-4" style="height: 90%">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Set Extensions</h3>
      <CloseButton @click="close" />
    </div>

    <ExtensionsPicker v-model="requestedExtensions" :talos-version="talosVersion" class="flex-1" />

    <div class="flex justify-between gap-4">
      <TButton type="secondary" @click="close">Cancel</TButton>
      <div class="flex gap-4">
        <TButton type="highlighted" @click="() => updateExtensions(requestedExtensions)">
          Save
        </TButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-auto w-1/2 p-8;
}

.heading {
  @apply flex items-center justify-between text-xl text-naturals-n14;
}
</style>
