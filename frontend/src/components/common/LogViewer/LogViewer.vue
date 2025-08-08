<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="logs-list">
    <div class="logs-list-heading">
      <div class="logs-list-heading-wrapper">
        <p class="logs-list-heading-name" v-if="withDate">Date</p>
        <p class="logs-list-heading-name">Message</p>
      </div>
      <div class="flex gap-6">
        <t-button icon="copy" type="compact" @click="copyLogs">Copy</t-button>
        <t-checkbox label="Follow Logs" :checked="follow" @click="() => (follow = !follow)" />
      </div>
    </div>
    <dynamic-scroller
      class="logs-view"
      ref="logView"
      @update="scrollToBottom"
      @resize="scrollToBottom"
      :emit-update="waitUpdate"
      :items="displayLogs"
      :min-item-size="200"
    >
      <template v-slot="{ item, index, active }">
        <dynamic-scroller-item
          class="logs-item grid-cols-6"
          :key="index"
          :active="active"
          :size-dependencies="[item.msg]"
          :item="item"
          :data-index="index"
        >
          <div class="logs-item-date" v-if="withDate">
            {{ item.date }}
          </div>
          <div class="logs-item-message">
            <WordHighlighter
              :query="searchOption"
              :textToHighlight="item.msg"
              highlightClass="bg-naturals-N14"
            />
          </div>
        </dynamic-scroller-item>
      </template>
    </dynamic-scroller>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'
import { computed, ref, toRefs, onMounted, onUpdated, watch, nextTick } from 'vue'
import type { LogLine } from '@/methods/logs'
import { DynamicScroller, DynamicScrollerItem } from 'vue-virtual-scroller'
import { copyText } from 'vue3-clipboard'

import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import WordHighlighter from 'vue-word-highlighter'

import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'

type Props = {
  logs: LogLine[]
  searchOption: string
  withDate?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  withDate: true,
})

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

<style scoped>
.logs-list {
  @apply flex flex-col;
}
.logs-list-heading {
  @apply flex w-full bg-naturals-N2 justify-between items-center;
  padding: 10px 16px;
  border-radius: 2px;
}
.logs-list-heading-wrapper {
  @apply flex w-full;
}
.logs-list-heading-name {
  @apply text-xs text-naturals-N13 w-full;
  min-width: 100px;
  max-width: 300px;
}
.logs-view {
  @apply flex flex-col w-full grow overflow-auto h-72;
}

.logs-item {
  @apply flex w-full;
  padding: 5px 0px 5px 16px;
}
.logs-item-date {
  @apply text-xs font-roboto w-full;
  min-width: 100px;
  max-width: 300px;
}
.logs-item-message {
  @apply w-full text-xs font-roboto break-all;
}
</style>
