<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Pod as V1Pod } from 'kubernetes-types/core/v1'
import { DateTime } from 'luxon'
import { computed, ref } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import TIcon from '@/components/common/Icon/TIcon.vue'
import TSlideDownWrapper from '@/components/common/SlideDownWrapper/TSlideDownWrapper.vue'
import TStatus from '@/components/common/Status/TStatus.vue'

const { item } = defineProps<{
  searchOption: string
  item: V1Pod
}>()

const isDropdownOpened = ref(false)

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
  <div
    class="relative mb-1 flex w-full min-w-md flex-col border py-4.75 pr-3.5 pl-2 transition-all duration-500"
    :class="
      isDropdownOpened
        ? 'rounded border-naturals-n5'
        : 'rounded-t-sm border-transparent border-b-naturals-n5 last-of-type:border-b-transparent'
    "
  >
    <ul class="flex w-full items-center justify-start">
      <li class="flex w-1/6 items-center text-xs text-naturals-n13">
        <TIcon
          class="mr-1 size-6 cursor-pointer rounded fill-current text-naturals-n11 transition-all duration-300 hover:bg-naturals-n7"
          :class="isDropdownOpened && '-rotate-180'"
          icon="drop-up"
          @click="isDropdownOpened = !isDropdownOpened"
        />

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

    <TSlideDownWrapper :expanded="isDropdownOpened" class="text-xs text-naturals-n12">
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
    </TSlideDownWrapper>
  </div>
</template>
