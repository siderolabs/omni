<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, toRefs } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import {
  InfraProviderNamespace,
  InfraProviderStatusType,
  LabelIsStaticInfraProvider,
} from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TList from '@/components/common/List/TList.vue'

const infraProviderResources: WatchOptions = {
  resource: { type: InfraProviderStatusType, namespace: InfraProviderNamespace },
  runtime: Runtime.Omni,
  selectors: [`!${LabelIsStaticInfraProvider}`],
}

const props = defineProps<{
  infraProvider?: string
}>()

const emit = defineEmits(['update:infra-provider', 'update:idle-machine-count'])

const { infraProvider } = toRefs(props)

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

  emit('update:infra-provider', item.metadata.id)
}
</script>

<template>
  <div class="text-naturals-N13">Infrastructure Provider</div>
  <TList
    :key="infraProvider"
    :opts="infraProviderResources"
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
            <div class="flex flex-1 items-center gap-3 text-sm text-naturals-N13">
              <TIcon :svg-base-64="item.spec.icon" icon="cloud-connection" class="h-10 w-10" />
              <div class="flex flex-col gap-1">
                <div>{{ item.spec.name }}</div>
                <div class="rounded bg-naturals-N4 px-2 py-0.5 text-xs text-naturals-N11">
                  id:
                  <WordHighlighter
                    :query="searchQuery"
                    :text-to-highlight="item.metadata.id"
                    highlight-class="bg-naturals-N14"
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
.provider-item {
  @apply min-w-fit cursor-pointer rounded border border-naturals-N6 bg-naturals-N2 p-3 transition-colors duration-200 hover:border-naturals-N8 hover:bg-naturals-N3 max-md:flex-1;
}

.selected {
  @apply border-naturals-N9 bg-naturals-N3;
}
</style>
