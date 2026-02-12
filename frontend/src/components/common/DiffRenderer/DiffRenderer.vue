<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useVirtualizer, type VirtualItem } from '@tanstack/vue-virtual'
import { refDebounced } from '@vueuse/core'
import { type ComponentPublicInstance, computed, markRaw, ref, useTemplateRef, watch } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import IconButton from '@/components/common/Button/IconButton.vue'
import TInput from '@/components/common/TInput/TInput.vue'

const { diff, withSearch } = defineProps<{
  diff: string
  withSearch?: boolean
}>()

const scrollContainer = useTemplateRef('scrollContainerRef')

const diffLines = computed(() => markRaw(diff.split('\n')))

const rowVirtualizer = useVirtualizer(
  computed(() => ({
    count: diffLines.value.length,
    getScrollElement: () => scrollContainer.value,
    estimateSize: () => 26,
    overscan: 10,
  })),
)

const virtualRows = computed(() => rowVirtualizer.value.getVirtualItems())
const totalHeight = computed(() => rowVirtualizer.value.getTotalSize())

const search = ref('')
const searchDebounced = refDebounced(search, 300)
const currentMatchIndex = ref(0)

watch(searchDebounced, () => {
  currentMatchIndex.value = 0
  scrollToMatch()
})

const matchedLines = computed(() => {
  if (!withSearch || !searchDebounced.value) return []

  const lowered = searchDebounced.value.toLowerCase()

  return diffLines.value.reduce<number[]>((prev, curr, i) => {
    if (curr.toLowerCase().includes(lowered)) {
      prev.push(i)
    }

    return prev
  }, [])
})

function scrollToMatch() {
  if (!matchedLines.value.length) return
  const targetLineIndex = matchedLines.value[currentMatchIndex.value]

  rowVirtualizer.value.scrollToIndex(targetLineIndex, { align: 'center' })
}

function nextMatch() {
  if (!matchedLines.value.length) return

  currentMatchIndex.value = (currentMatchIndex.value + 1) % matchedLines.value.length

  scrollToMatch()
}

function prevMatch() {
  if (!matchedLines.value.length) return

  currentMatchIndex.value =
    (currentMatchIndex.value - 1 + matchedLines.value.length) % matchedLines.value.length

  scrollToMatch()
}

function getRowKey(row: VirtualItem) {
  return row.key as number | string
}

function measureElement(el?: Element | ComponentPublicInstance | null) {
  if (el) rowVirtualizer.value.measureElement(el as Element)
}
</script>

<template>
  <div class="flex h-full flex-col gap-4">
    <div v-if="withSearch" class="flex items-center gap-2">
      <TInput v-model.trim="search" title="Search" class="grow" @keydown.enter="nextMatch" />

      <div class="flex gap-1">
        <IconButton
          :disabled="!matchedLines.length"
          icon="arrow-up"
          aria-label="Previous match"
          @click="prevMatch"
        />
        <IconButton
          :disabled="!matchedLines.length"
          icon="arrow-down"
          aria-label="Next match"
          @click="nextMatch"
        />
      </div>
    </div>

    <div ref="scrollContainerRef" class="min-h-0 flex-1 overflow-y-auto font-mono text-sm/relaxed">
      <div class="relative text-naturals-n14" :style="{ height: `${totalHeight}px` }">
        <div
          class="absolute top-0 left-0"
          :style="{ transform: `translateY(${virtualRows[0]?.start ?? 0}px)` }"
        >
          <div
            v-for="virtualRow in virtualRows"
            :key="getRowKey(virtualRow)"
            :ref="measureElement"
            :data-index="virtualRow.index"
            class="px-2 py-0.5 break-all whitespace-pre-wrap"
            :class="{
              'text-naturals-n10': diffLines[virtualRow.index].startsWith('#'),
              'text-green-g1': diffLines[virtualRow.index].startsWith('+'),
              'text-red-r1': diffLines[virtualRow.index].startsWith('-'),
              'text-blue-b1': diffLines[virtualRow.index].startsWith('@@'),
            }"
          >
            <WordHighlighter
              v-if="withSearch"
              :query="searchDebounced"
              :text-to-highlight="diffLines[virtualRow.index]"
              highlight-class="bg-naturals-n14"
            />
            <template v-else>{{ diffLines[virtualRow.index] }}</template>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
