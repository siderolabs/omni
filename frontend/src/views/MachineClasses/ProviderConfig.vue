<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import {
  InfraProviderNamespace,
  InfraProviderStatusType,
  LabelIsStaticInfraProvider,
} from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TList from '@/components/List/TList.vue'

const infraProvider = defineModel<string>('infraProvider')

const selectingProvider = ref(false)
const showAllProviders = computed(() => {
  return selectingProvider.value || !infraProvider.value
})

const filterProviders = (items: Resource<InfraProviderStatusSpec>[]) => {
  if (showAllProviders.value) {
    return items
  }

  return items.filter((item) => item?.metadata.id === infraProvider.value)
}

const setInfraProvider = (item: Resource<InfraProviderStatusSpec>) => {
  if (!showAllProviders.value) {
    selectingProvider.value = true

    return
  }

  selectingProvider.value = false
  infraProvider.value = item.metadata.id
}
</script>

<template>
  <div class="text-content-default">Infrastructure Provider</div>
  <TList
    :key="infraProvider"
    :opts="{
      type: undefined as unknown as InfraProviderStatusSpec,
      resource: {
        type: InfraProviderStatusType,
        namespace: InfraProviderNamespace,
      },
      runtime: Runtime.Omni,
      selectors: [`!${LabelIsStaticInfraProvider}`],
    }"
    :search="showAllProviders"
    class="mb-1"
  >
    <template #default="{ items, searchQuery }">
      <div class="flex gap-2 max-md:flex-wrap md:flex-col">
        <div
          v-for="item in filterProviders(items)"
          :key="item.metadata.id"
          class="provider-item"
          :class="{ selected: item.metadata.id === infraProvider && showAllProviders }"
          @click="() => setInfraProvider(item)"
        >
          <div class="flex items-center gap-2">
            <div class="flex flex-1 items-center gap-3 text-sm text-content-default">
              <TIcon :svg-base-64="item.spec.icon" icon="cloud-connection" class="h-10 w-10" />
              <div class="flex flex-col gap-1">
                <div>{{ item.spec.name }}</div>
                <div class="rounded bg-surface-hover px-2 py-0.5 text-xs text-content-secondary">
                  id:
                  <WordHighlighter
                    :query="searchQuery"
                    :text-to-highlight="item.metadata.id"
                    highlight-class="bg-surface-inverse"
                  />
                </div>
              </div>
              <div class="flex-1 pr-3 text-right text-xs max-md:hidden">
                {{ item.spec.description }}
              </div>
            </div>
            <IconButton v-if="!showAllProviders" icon="edit" />
          </div>
        </div>
      </div>
    </template>
  </TList>
</template>

<style scoped>
@reference "../../index.css";

.provider-item {
  @apply min-w-fit cursor-pointer rounded border border-border-strong bg-surface-card p-3 transition-colors duration-200 hover:border-border-strong hover:bg-surface-raised max-md:flex-1;
}

.selected {
  @apply border-border-strong bg-surface-raised;
}
</style>
