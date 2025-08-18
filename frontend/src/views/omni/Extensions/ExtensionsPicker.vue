<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosExtensionsType } from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import Watch from '@/components/common/Watch/Watch.vue'

const emit = defineEmits(['update:model-value'])

const props = defineProps<{
  talosVersion: string
  modelValue: Record<string, boolean>
  immutableExtensions?: Record<string, boolean>
  indeterminate?: boolean
  showDescriptions?: boolean
}>()

const { modelValue } = toRefs(props)
const filterExtensions = ref<string>('')

const filteredExtensions = (items: TalosExtensionsSpecInfo[]) => {
  if (!filterExtensions.value) {
    return items
  }

  return items.filter((item) => item.name?.includes(filterExtensions.value))
}

const changed = ref(false)

const updateExtension = (extension: TalosExtensionsSpecInfo) => {
  const newState = {
    ...modelValue.value,
  }

  newState[extension.name!] = !newState[extension.name!]

  emit('update:model-value', newState)

  changed.value = true
}
</script>

<template>
  <div class="flex min-h-36 flex-col gap-2 overflow-hidden">
    <TInput v-model="filterExtensions" icon="search" />

    <div class="grid grid-cols-4 bg-naturals-n4 py-2 pl-2 text-xs text-naturals-n13 uppercase">
      <div class="col-span-2">Name</div>
      <div>Version</div>
      <div>Author</div>
    </div>

    <Watch
      v-if="talosVersion"
      :opts="{
        resource: {
          id: talosVersion,
          type: TalosExtensionsType,
          namespace: DefaultNamespace,
        },
        runtime: Runtime.Omni,
      }"
      display-always
    >
      <template #default="{ data }">
        <div v-if="data?.spec.items" class="flex flex-col overflow-auto">
          <div
            v-for="extension in filteredExtensions(data.spec.items)"
            :key="extension.name"
            class="flex cursor-pointer items-center gap-2 border-b border-naturals-n6 p-2 transition-colors hover:bg-naturals-n5"
            @click="() => updateExtension(extension)"
          >
            <TCheckbox
              :icon="indeterminate && !changed ? 'minus' : 'check'"
              class="col-span-2"
              :disabled="immutableExtensions?.[extension.name!]"
              :checked="modelValue[extension.name!]"
            />
            <div class="grid flex-1 grid-cols-4 items-center gap-1">
              <div class="col-span-2 text-xs text-naturals-n13">
                <WordHighlighter
                  :query="filterExtensions"
                  :text-to-highlight="extension.name!.slice('siderolabs/'.length)"
                  highlight-class="bg-naturals-n14"
                />
              </div>
              <div class="text-xs text-naturals-n13">{{ extension.version }}</div>
              <div class="text-xs text-naturals-n13">{{ extension.author }}</div>
              <div v-if="extension.description && showDescriptions" class="col-span-4 text-xs">
                {{ extension.description }}
              </div>
            </div>
          </div>
        </div>
        <div v-else class="flex items-center gap-1 p-4 text-xs text-primary-p2">
          <TIcon class="h-3 w-3" icon="warning" />No extensions available for this Talos version
        </div>
      </template>
    </Watch>
  </div>
</template>
