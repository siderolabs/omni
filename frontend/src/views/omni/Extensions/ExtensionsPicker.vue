<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { TalosExtensionsSpec, TalosExtensionsSpecInfo } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, TalosExtensionsType } from '@/api/resources'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { talosVersion, immutableExtensions = {} } = defineProps<{
  talosVersion: string
  immutableExtensions?: Record<string, boolean>
  indeterminate?: boolean
  showDescriptions?: boolean
}>()

const modelValue = defineModel<Record<string, boolean>>({ required: true })
const filterExtensions = ref('')

const { data } = useResourceWatch<TalosExtensionsSpec>(() => ({
  skip: !talosVersion,
  resource: {
    id: talosVersion,
    type: TalosExtensionsType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const filteredExtensions = computed(() => {
  const items = data.value?.spec.items

  if (!items || !filterExtensions.value) return items

  return items.filter((item) => item.name?.includes(filterExtensions.value))
})

const changed = ref(false)

const updateExtension = (extension: TalosExtensionsSpecInfo, enabled: boolean) => {
  if (immutableExtensions[extension.name!]) return

  modelValue.value = {
    ...modelValue.value,
    [extension.name!]: enabled,
  }

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

    <div v-if="data?.spec.items" class="flex flex-col overflow-auto">
      <div
        v-for="extension in filteredExtensions"
        :key="extension.name"
        class="grid grid-cols-4 gap-1 border-b border-naturals-n6 p-2 transition-colors hover:bg-naturals-n5"
        role="button"
        @click="updateExtension(extension, !modelValue[extension.name!])"
      >
        <div class="col-span-2 flex items-center gap-2 text-xs text-naturals-n13">
          <TCheckbox
            :indeterminate="indeterminate && !changed"
            :disabled="immutableExtensions[extension.name!]"
            :model-value="modelValue[extension.name!]"
            class="pointer-events-none"
          />

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
    <div v-else class="flex items-center gap-1 p-4 text-xs text-primary-p2">
      <TIcon class="h-3 w-3" icon="warning" />
      No extensions available for this Talos version
    </div>
  </div>
</template>
