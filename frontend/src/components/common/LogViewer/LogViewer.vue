<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'

import type { Component } from 'vue'
import { computed, nextTick, onMounted, onUpdated, ref, toRefs, watch } from 'vue'
import { DynamicScroller, DynamicScrollerItem } from 'vue-virtual-scroller'
import WordHighlighter from 'vue-word-highlighter'
import { copyText } from 'vue3-clipboard'

import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import type { LogLine } from '@/methods/logs'

type Props = {
  logs: LogLine[]
  searchOption: string
  withoutDate?: boolean
}

const props = withDefaults(defineProps<Props>(), {})

const follow = ref(true)
const logView: Component = ref(null)
const { searchOption, logs } = toRefs(props)

const waitUpdate = ref(false)

const scrollToBottom = () => {
  if (!logView.value || !follow.value) {
    return
  }

  waitUpdate.value = false
  nextTick(() => {
    logView.value.scrollToItem(filteredLogs.value.length)
  })
}

const displayLogs = computed(() => {
  return filteredLogs.value.map((item: LogLine, index: number) => {
    return {
      ...item,
      id: index,
    }
  })
})

const filteredLogs = computed(() => {
  if (!searchOption.value) {
    return logs.value
  }

  return logs.value.filter((elem: LogLine) => {
    return elem?.msg.includes(searchOption.value)
  })
})

const copyLogs = () => {
  return copyText(
    filteredLogs.value
      .map((item: LogLine) => {
        const arr: string[] = []
        if (item.date) {
          arr.push(item.date)
        }

        arr.push(item.msg)

        return arr.join(' ')
      })
      .join('\n'),
    undefined,
    () => {},
  )
}

onMounted(scrollToBottom)
onUpdated(scrollToBottom)

watch(follow, scrollToBottom)
watch(logs, () => {
  waitUpdate.value = true

  scrollToBottom()
})
</script>

<template>
  <div class="logs-list">
    <div class="logs-list-heading">
      <div class="logs-list-heading-wrapper">
        <p v-if="!withoutDate" class="logs-list-heading-name">Date</p>
        <p class="logs-list-heading-name">Message</p>
      </div>
      <div class="flex gap-6">
        <TButton icon="copy" type="compact" @click="copyLogs">Copy</TButton>
        <TCheckbox label="Follow Logs" :checked="follow" @click="() => (follow = !follow)" />
      </div>
    </div>
    <DynamicScroller
      ref="logView"
      class="logs-view"
      :emit-update="waitUpdate"
      :items="displayLogs"
      :min-item-size="200"
      @update="scrollToBottom"
      @resize="scrollToBottom"
    >
      <template #default="{ item, index, active }">
        <DynamicScrollerItem
          :key="index"
          class="logs-item grid-cols-6"
          :active="active"
          :size-dependencies="[item.msg]"
          :item="item"
          :data-index="index"
        >
          <div v-if="!withoutDate" class="logs-item-date">
            {{ item.date }}
          </div>
          <div class="logs-item-message">
            <WordHighlighter
              :query="searchOption"
              :text-to-highlight="item.msg"
              highlight-class="bg-naturals-N14"
            />
          </div>
        </DynamicScrollerItem>
      </template>
    </DynamicScroller>
  </div>
</template>

<style scoped>
.logs-list {
  @apply flex flex-col;
}
.logs-list-heading {
  @apply flex w-full items-center justify-between bg-naturals-N2;
  padding: 10px 16px;
  border-radius: 2px;
}
.logs-list-heading-wrapper {
  @apply flex w-full;
}
.logs-list-heading-name {
  @apply w-full text-xs text-naturals-N13;
  min-width: 100px;
  max-width: 300px;
}
.logs-view {
  @apply flex h-72 w-full grow flex-col overflow-auto;
}

.logs-item {
  @apply flex w-full;
  padding: 5px 0px 5px 16px;
}
.logs-item-date {
  @apply w-full font-roboto text-xs;
  min-width: 100px;
  max-width: 300px;
}
.logs-item-message {
  @apply w-full break-all font-roboto text-xs;
}
</style>
