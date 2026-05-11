<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useVirtualizer } from '@tanstack/vue-virtual'
import { useClipboard } from '@vueuse/core'
import { computed, ref, useTemplateRef, watch } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import TButton from '@/components/Button/TButton.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import type { LogLine } from '@/methods/logs'

const { searchOption, logs } = defineProps<{
  logs: LogLine[]
  searchOption: string
  withoutDate?: boolean
}>()

const { copy, copied } = useClipboard()

const follow = ref(true)
const scrollContainer = useTemplateRef('scrollContainer')

const filteredLogs = computed(() =>
  searchOption ? logs.filter((elem) => elem.msg.includes(searchOption)) : logs,
)

const virtualizer = useVirtualizer(
  computed(() => ({
    count: filteredLogs.value.length,
    getScrollElement: () => scrollContainer.value,
    estimateSize: () => 28,
    overscan: 5,
  })),
)

const copyLogs = () => {
  return copy(
    filteredLogs.value.map((item) => [item.date, item.msg].filter(Boolean).join(' ')).join('\n'),
  )
}

watch(
  () => [follow.value, filteredLogs.value.length],
  () => {
    if (follow.value) virtualizer.value.scrollToEnd()
  },
  { flush: 'post' },
)
</script>

<template>
  <div class="flex flex-col">
    <div class="flex w-full items-center justify-between rounded-xs bg-naturals-n2 px-4 py-2.5">
      <div class="flex w-full gap-8 text-xs text-naturals-n13">
        <p v-if="!withoutDate" class="w-35 shrink-0">Date</p>
        <p class="grow">Message</p>
      </div>
      <div class="flex gap-6">
        <TButton :icon="copied ? 'check' : 'copy'" size="sm" @click="copyLogs">
          {{ copied ? 'Copied' : 'Copy' }}
        </TButton>
        <TCheckbox v-model="follow" label="Follow Logs" />
      </div>
    </div>
    <div ref="scrollContainer" class="h-72 w-full grow overflow-auto">
      <div class="relative" :style="{ height: `${virtualizer.getTotalSize()}px` }">
        <div
          v-for="virtualRow in virtualizer.getVirtualItems()"
          :key="virtualRow.index"
          :ref="(el) => virtualizer.measureElement(el as HTMLElement)"
          :data-index="virtualRow.index"
          class="absolute top-0 left-0 flex w-full gap-8 py-1.25 pl-4 font-mono text-xs"
          :style="{ transform: `translateY(${virtualRow.start}px)` }"
        >
          <div v-if="!withoutDate" class="w-35 shrink-0">
            {{ filteredLogs[virtualRow.index].date }}
          </div>
          <div class="grow break-all">
            <WordHighlighter
              :query="searchOption"
              :text-to-highlight="filteredLogs[virtualRow.index].msg"
              highlight-class="bg-naturals-n14"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
