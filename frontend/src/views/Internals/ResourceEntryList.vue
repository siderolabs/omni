<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useVirtualizer } from '@tanstack/vue-virtual'
import { refDebounced } from '@vueuse/core'
import { useRouteQuery } from '@vueuse/router'
import { dump } from 'js-yaml'
import {
  type ComponentPublicInstance,
  computed,
  ref,
  type UnwrapRef,
  useTemplateRef,
  watch,
} from 'vue'

import { MetaNamespace, ResourceDefinitionType, VirtualNamespace } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { downloadFile } from '@/methods'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceList } from '@/methods/useResourceList'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ResourceBreadcrumbs from '@/views/Internals/components/ResourceBreadcrumbs.vue'
import ResourceEntryItem from '@/views/Internals/components/ResourceEntryItem.vue'
import ResourceEntryPanel from '@/views/Internals/components/ResourceEntryPanel.vue'
import {
  resourceDefinitionID,
  type ResourceDefinitionSpec,
  withoutClientFields,
} from '@/views/Internals/lib/resourceDefinition'
import { type ResourceTarget, useResourceRuntime } from '@/views/Internals/lib/useResourceRuntime'

const { target, type } = defineProps<{ target: ResourceTarget; type: string }>()

const { machine, watchOptions } = useResourceRuntime(() => target)

const { data: definition, loading: definitionLoading } = useResourceGet<ResourceDefinitionSpec>(
  () => ({
    ...watchOptions.value,
    resource: {
      namespace: MetaNamespace,
      type: ResourceDefinitionType,
      id: resourceDefinitionID(type),
    },
  }),
)

const namespace = computed(() => definition.value?.spec.defaultNamespace)

const searchInput = useRouteQuery('q', '')
const search = refDebounced(searchInput, 300)

// Virtual resources are computed on request and don't support watching a whole
// kind, so they can only be listed.
const isVirtual = computed(() => !machine.value && namespace.value === VirtualNamespace)

const {
  data: watchData,
  loading: watchLoading,
  err: watchErr,
} = useResourceWatch(() => ({
  ...watchOptions.value,
  skip: !namespace.value || isVirtual.value,
  resource: {
    namespace: namespace.value,
    type,
  },
}))

const {
  data: listData,
  loading: listLoading,
  error: listErr,
  loadData: reloadList,
} = useResourceList(() => ({
  ...watchOptions.value,
  skip: !isVirtual.value,
  resource: {
    namespace: namespace.value,
    type,
  },
}))

function refresh() {
  reloadList(new AbortController())
}

const data = computed(() =>
  (isVirtual.value ? listData.value : watchData.value).map(withoutClientFields),
)
const loading = computed(() => (isVirtual.value ? listLoading.value : watchLoading.value))
const err = computed(() => (isVirtual.value ? listErr.value?.message : watchErr.value))

const items = computed(() => {
  const query = search.value.trim().toLowerCase()
  if (!query) return data.value

  return data.value.filter((item) => JSON.stringify(item).toLowerCase().includes(query))
})

function exportResources(format: 'json' | 'yaml') {
  // YAML is exported as a multi-document stream, matching `omnictl get -o yaml`.
  const content =
    format === 'json'
      ? JSON.stringify(items.value, null, 2)
      : items.value.map((item) => dump(item, { noRefs: true })).join('---\n')

  const blob = new Blob([content], {
    type: format === 'json' ? 'application/json' : 'application/yaml',
  })
  const url = window.URL.createObjectURL(blob)

  downloadFile(url, `${type}.${format}`)
  window.URL.revokeObjectURL(url)
}

// The selected ID controls whether the panel is open, while the panel keeps showing the
// last selected resource so its content doesn't disappear while it animates closed.
const selectedId = useRouteQuery<string | undefined>('id')
const panelId = ref(selectedId.value)

watch(selectedId, (id) => {
  if (id) panelId.value = id
})

const panelItem = computed(() => data.value.find((i) => i.metadata.id === panelId.value))

// Close the side panel when looking at a different set of resources
watch([() => type, machine], () => (selectedId.value = undefined))

function toggleSelected(id: string) {
  selectedId.value = selectedId.value === id ? undefined : id
}

const scrollContainer = useTemplateRef('scrollContainer')

type VirtualizerOptions = UnwrapRef<Parameters<typeof useVirtualizer>[0]>

const rowVirtualizer = useVirtualizer(
  computed<VirtualizerOptions>(() => ({
    count: items.value.length,
    getScrollElement: () => scrollContainer.value,
    estimateSize: () => 40,
    getItemKey: (index) => items.value[index].metadata.id!,
    overscan: 5,
  })),
)

const virtualRows = computed(() => rowVirtualizer.value.getVirtualItems())
const totalSize = computed(() => rowVirtualizer.value.getTotalSize())

function measureElement(el: Element | ComponentPublicInstance | null) {
  if (el) rowVirtualizer.value.measureElement(el as Element)
}

const tableHeaders = ['ID', 'Phase', 'Version', 'Owner', 'Updated']
</script>

<template>
  <PageContainer class="flex h-full flex-col">
    <div class="mb-2 flex items-start justify-between gap-1">
      <div role="heading" aria-level="1" :aria-label="type">
        <ResourceBreadcrumbs
          :runtime="target.runtime"
          :machine="target.runtime === 'talos' ? target.machine : undefined"
          :type
        />
      </div>
      <div class="flex gap-2">
        <TButton :disabled="!items.length" @click="exportResources('yaml')">Export YAML</TButton>
        <TButton :disabled="!items.length" @click="exportResources('json')">Export JSON</TButton>
      </div>
    </div>

    <p
      v-if="definition"
      class="mb-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-naturals-n13"
    >
      Type
      <span class="resource-label font-mono">{{ definition.spec.type }}</span>
      <span aria-hidden="true" class="text-naturals-n8">·</span>
      Namespace
      <span class="resource-label font-mono">{{ namespace }}</span>
    </p>

    <div class="mb-4 flex flex-wrap gap-2">
      <TInput v-model="searchInput" class="grow" icon="search" title="Search" />
      <TButton
        v-if="isVirtual"
        icon="refresh"
        icon-position="left"
        :disabled="loading"
        @click="refresh"
      >
        Refresh
      </TButton>
    </div>

    <!-- The side panel only shares space with the table, the header above stays full width -->
    <div class="relative flex min-h-0 grow gap-2">
      <div class="flex min-w-0 grow flex-col">
        <TAlert v-if="!definitionLoading && !definition" type="error" title="Unknown resource type">
          No resource definition exists for {{ type }}.
        </TAlert>

        <TAlert
          v-else-if="err"
          type="error"
          :title="isVirtual ? 'Failed to list resources' : 'Failed to watch resources'"
        >
          {{ err }}
        </TAlert>

        <div v-else-if="loading || definitionLoading" class="flex grow items-center justify-center">
          <TSpinner class="size-6" />
        </div>

        <TAlert v-else-if="!items.length" type="info" title="No resources found" />

        <div
          v-else
          role="grid"
          class="flex min-h-0 flex-1 flex-col overflow-hidden text-xs text-naturals-n13"
        >
          <div
            role="rowgroup"
            class="grid shrink-0 grid-cols-[minmax(0,2fr)_100px_80px_minmax(0,1fr)_140px] gap-x-2 bg-naturals-n2 text-left"
          >
            <div
              v-for="header in tableHeaders"
              :key="header"
              role="columnheader"
              class="py-2 uppercase first:pl-2 last:pr-2"
            >
              {{ header }}
            </div>
          </div>

          <div ref="scrollContainer" role="rowgroup" class="min-h-0 flex-1 overflow-y-auto">
            <div class="relative" :style="{ height: `${totalSize}px` }">
              <div
                class="absolute top-0 left-0 w-full"
                :style="{ transform: `translateY(${virtualRows[0]?.start ?? 0}px)` }"
              >
                <div
                  v-for="vRow in virtualRows"
                  :key="vRow.key.toString()"
                  :ref="measureElement"
                  :data-index="vRow.index"
                  class="grid grid-cols-[minmax(0,2fr)_100px_80px_minmax(0,1fr)_140px] gap-x-2 border-t border-naturals-n5"
                >
                  <ResourceEntryItem
                    :item="items[vRow.index]"
                    :search
                    :selected="items[vRow.index].metadata.id === selectedId"
                    :hide-updated="isVirtual"
                    @open="toggleSelected(items[vRow.index].metadata.id!)"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <p v-if="items.length" class="mt-2 text-xs text-naturals-n10">
          {{ items.length }} of {{ data.length }} resources
        </p>
      </div>

      <div
        class="shrink-0 overflow-hidden max-lg:absolute max-lg:inset-0 max-lg:z-10 lg:transition-all"
        :class="selectedId ? 'max-lg:w-full lg:w-xl' : 'pointer-events-none opacity-0 lg:w-0'"
        :inert="!selectedId"
      >
        <ResourceEntryPanel
          :id="panelId"
          :item="panelItem"
          :search
          :sensitive="definition?.spec.sensitivity === 'sensitive'"
          class="h-full"
          @close="selectedId = undefined"
        />
      </div>
    </div>
  </PageContainer>
</template>
