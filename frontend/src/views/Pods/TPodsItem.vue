<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Pod as V1Pod } from 'kubernetes-types/core/v1'
import { DateTime } from 'luxon'
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import TIcon from '@/components/Icon/TIcon.vue'
import TStatus from '@/components/Status/TStatus.vue'

const { item } = defineProps<{
  searchOption: string
  item: V1Pod
}>()

const readyContainers = computed(() =>
  item.status?.containerStatuses?.filter((item) => item?.ready),
)

const restartCount = computed(() =>
  item.status?.containerStatuses?.reduce((amount, reducer) => amount + reducer.restartCount, 0),
)

const getAge = (age: string) =>
  DateTime.now()
    .diff(DateTime.fromISO(age), ['days', 'hours', 'minutes'])
    .toFormat("dd'd' hh'h' mm'm'")
</script>

<template>
  <CollapsibleRoot
    v-slot="{ open }"
    class="group relative mb-1 flex w-full min-w-md flex-col border py-4.75 pr-3.5 pl-2 transition-all duration-500 not-data-[state=open]:rounded-t-sm not-data-[state=open]:border-transparent not-data-[state=open]:border-b-naturals-n5 not-data-[state=open]:last-of-type:border-b-transparent data-[state=open]:rounded data-[state=open]:border-naturals-n5"
  >
    <ul class="flex w-full items-center justify-start">
      <li class="flex w-1/6 items-center gap-1 text-xs text-naturals-n13">
        <CollapsibleTrigger
          class="cursor-pointer rounded transition-colors hover:bg-naturals-n7"
          :aria-label="open ? 'Collapse details' : 'Expand details'"
        >
          <TIcon
            class="size-6 cursor-pointer text-naturals-n11 transition-transform duration-300 group-data-[state=open]:-rotate-180"
            icon="drop-up"
            aria-hidden="true"
          />
        </CollapsibleTrigger>

        <WordHighlighter
          :query="searchOption"
          :text-to-highlight="item.metadata?.namespace"
          highlight-class="bg-naturals-n14"
        />
      </li>

      <li class="flex w-1/3 items-center text-xs text-naturals-n13">
        <WordHighlighter
          :query="searchOption"
          :text-to-highlight="item.metadata?.name"
          highlight-class="bg-naturals-n14"
        />
      </li>

      <li class="flex w-1/6 items-center text-xs text-naturals-n13">
        <TStatus :title="item.status?.phase" />
      </li>

      <li class="flex w-1/3 items-center justify-between text-xs text-naturals-n13">
        <WordHighlighter
          :query="searchOption"
          :text-to-highlight="item.spec?.nodeName"
          highlight-class="bg-naturals-n14"
        />
      </li>
    </ul>

    <CollapsibleContent class="collapsible-content overflow-hidden text-xs text-naturals-n12">
      <div class="flex items-center pt-6.5">
        <div class="flex w-1/6 flex-col gap-1.75 pl-7">
          <p>Restarts</p>
          <p class="text-naturals-n13">{{ restartCount }}</p>
        </div>

        <div class="flex w-1/3 flex-col gap-1.75">
          <p>Ready Containers</p>
          <p class="text-naturals-n13">{{ readyContainers?.length }}</p>
        </div>

        <div class="flex w-1/6 flex-col gap-1.75">
          <p>Age</p>
          <p class="text-naturals-n13">
            {{ item.status?.startTime ? getAge(item.status.startTime) : '' }}
          </p>
        </div>

        <div class="flex w-1/6 flex-col gap-1.75">
          <p>Pod IP</p>
          <p class="text-naturals-n13">{{ item.status?.podIP }}</p>
        </div>
      </div>

      <div class="mt-5 flex flex-col gap-3.75 px-7">
        <div class="font-bold">Containers</div>
        <div class="flex flex-wrap gap-2">
          <div
            v-for="container in item.spec?.containers"
            :key="container.name"
            class="rounded bg-naturals-n4 p-1 px-2"
          >
            {{ container.image }}
          </div>
        </div>
      </div>
    </CollapsibleContent>
  </CollapsibleRoot>
</template>

<style scoped>
.collapsible-content[data-state='open'] {
  animation: slideDown 200ms ease-out;
}

.collapsible-content[data-state='closed'] {
  animation: slideUp 200ms ease-out;
}

@keyframes slideDown {
  from {
    height: 0;
  }
  to {
    height: var(--reka-collapsible-content-height);
  }
}

@keyframes slideUp {
  from {
    height: var(--reka-collapsible-content-height);
  }
  to {
    height: 0;
  }
}
</style>
