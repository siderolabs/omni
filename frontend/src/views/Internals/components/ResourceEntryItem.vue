<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import { relativeISO } from '@/methods/time'

defineProps<{
  item: Resource
  search: string
  selected?: boolean
  /** Virtual resources are generated per request, so their update time is meaningless. */
  hideUpdated?: boolean
}>()

defineEmits<{
  open: []
}>()
</script>

<template>
  <div
    role="row"
    tabindex="0"
    :aria-selected="selected"
    :class="{ 'bg-naturals-n3': selected }"
    class="col-span-full grid cursor-pointer grid-cols-subgrid items-center py-2.5 select-none hover:bg-white/5"
    @click="$emit('open')"
    @keydown.enter.prevent="$emit('open')"
    @keydown.space.prevent="$emit('open')"
  >
    <div role="cell" class="truncate pl-2 text-naturals-n14">
      <WordHighlighter
        :query="search"
        :text-to-highlight="item.metadata.id"
        highlight-class="bg-naturals-n14"
      />
    </div>

    <div role="cell" class="truncate">{{ item.metadata.phase }}</div>
    <div role="cell" class="truncate">{{ item.metadata.version }}</div>
    <div role="cell" class="truncate">{{ item.metadata.owner }}</div>

    <div role="cell" class="truncate pr-2">
      <time v-if="!hideUpdated && item.metadata.updated" :datetime="item.metadata.updated">
        {{ relativeISO(item.metadata.updated) }}
      </time>
    </div>
  </div>
</template>
