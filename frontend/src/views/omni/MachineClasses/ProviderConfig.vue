<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="text-naturals-N13">Infrastructure Provider</div>
  <t-list
    :opts="infraProviderResources"
    :key="infraProvider"
    :search="showAllProviders"
    class="mb-1"
  >
    <template #default="{ items, searchQuery }">
      <div class="flex md:flex-col gap-2 max-md:flex-wrap">
        <div
          v-for="item in filterProviders(items)"
          class="provider-item"
          :key="item.metadata.id"
          :class="{ selected: item.metadata.id === infraProvider && showAllProviders }"
          @click="() => setInfraProvider(item)"
        >
          <div class="flex gap-2 items-center">
            <div class="flex items-center text-sm text-naturals-N13 gap-3 flex-1">
              <t-icon :svg-base-64="item.spec.icon" icon="cloud-connection" class="w-10 h-10" />
              <div class="flex flex-col gap-1">
                <div>{{ item.spec.name }}</div>
                <div class="text-xs text-naturals-N11 px-2 py-0.5 rounded bg-naturals-N4">
                  id:
                  <WordHighlighter
                    :query="searchQuery"
                    :textToHighlight="item.metadata.id"
                    highlightClass="bg-naturals-N14"
                  />
                </div>
              </div>
              <div class="flex-1 text-right text-xs pr-3 max-md:hidden">
                {{ item.spec.description }}
              </div>
            </div>
            <icon-button icon="edit" v-if="!showAllProviders" />
          </div>
        </div>
      </div>
    </template>
  </t-list>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import {
  InfraProviderNamespace,
  InfraProviderStatusType,
  LabelIsStaticInfraProvider,
} from '@/api/resources'
import { computed, ref, toRefs } from 'vue'

import TIcon from '@/components/common/Icon/TIcon.vue'
import WordHighlighter from 'vue-word-highlighter'
import IconButton from '@/components/common/Button/IconButton.vue'
import TList from '@/components/common/List/TList.vue'
import type { WatchOptions } from '@/api/watch'

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

<style scoped>
.provider-item {
  @apply p-3 rounded border border-naturals-N6 bg-naturals-N2 cursor-pointer transition-colors duration-200 hover:bg-naturals-N3 hover:border-naturals-N8 max-md:flex-1 min-w-fit;
}

.selected {
  @apply border-naturals-N9 bg-naturals-N3;
}
</style>
