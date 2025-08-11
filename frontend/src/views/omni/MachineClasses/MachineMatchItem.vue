<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, toRefs } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import TListItem from '@/components/common/List/TListItem.vue'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'

const props = defineProps<{
  machine: Resource<MachineStatusSpec>
  searchQuery?: string
}>()

defineEmits(['filterLabels'])

const { machine } = toRefs(props)

const machineName = computed(() => {
  return machine?.value?.spec?.network?.hostname ?? machine?.value?.metadata?.id
})
</script>

<template>
  <TListItem>
    <template #default>
      <div class="flex items-center text-xs text-naturals-N13">
        <div class="flex flex-1 items-center gap-2">
          <router-link
            :to="{ name: 'MachineLogs', params: { machine: machine?.metadata?.id } }"
            class="list-item-link pr-2"
          >
            <WordHighlighter
              :query="searchQuery ?? ''"
              split-by-space
              :text-to-highlight="machineName"
              highlight-class="bg-naturals-N14"
            />
          </router-link>
          <ItemLabels :resource="machine" @filter-label="(e) => $emit('filterLabels', e)" />
        </div>
        <div class="w-8 flex-initial">
          <div class="flex justify-end" />
        </div>
      </div>
    </template>
  </TListItem>
</template>
