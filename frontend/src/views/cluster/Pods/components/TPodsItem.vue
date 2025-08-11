<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { V1Pod } from '@kubernetes/client-node'
import { DateTime } from 'luxon'
import { computed, ref, toRefs } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import TIcon from '@/components/common/Icon/TIcon.vue'
import TSlideDownWrapper from '@/components/common/SlideDownWrapper/TSlideDownWrapper.vue'
import TStatus from '@/components/common/Status/TStatus.vue'

type Props = {
  searchOption: string
  item: V1Pod
}

const props = defineProps<Props>()

const { item } = toRefs(props)

const isDropdownOpened = ref(false)

const readyContainers = computed(() =>
  item.value?.status?.containerStatuses?.filter((item: any) => item?.ready === true),
)

const restartCount = computed(() =>
  item.value?.status?.containerStatuses?.reduce((amount, reducer: any) => {
    return amount + reducer.restartCount
  }, 0),
)

const getAge = (age: string) => {
  const currentDate = DateTime.now()
  const currentAge = DateTime.fromISO(age)

  return currentDate.diff(currentAge, ['days', 'hours', 'minutes']).toFormat("dd'd' hh'h' mm'm'")
}
</script>

<template>
  <div class="row" :class="{ opened: isDropdownOpened }">
    <TSlideDownWrapper :is-slider-opened="isDropdownOpened">
      <template #head
        ><ul class="row-wrapper">
          <li class="row-item">
            <TIcon
              class="row-arrow"
              :class="{ 'row-arrow-right--pushed': isDropdownOpened }"
              icon="drop-up"
              @click="() => (isDropdownOpened = !isDropdownOpened)"
            />
            <span>
              <WordHighlighter
                :query="searchOption"
                :text-to-highlight="item.metadata?.namespace"
                highlight-class="bg-naturals-N14"
              />
            </span>
          </li>
          <li class="row-item">
            <WordHighlighter
              :query="searchOption"
              :text-to-highlight="item.metadata?.name"
              highlight-class="bg-naturals-N14"
            />
          </li>
          <li class="row-item">
            <TStatus :title="item.status?.phase" />
          </li>
          <li class="row-item row-item--spaced">
            <span>
              <WordHighlighter
                :query="searchOption"
                :text-to-highlight="item.spec?.nodeName"
                highlight-class="bg-naturals-N14"
            /></span>
          </li>
        </ul>
      </template>
      <template #body>
        <div class="row-info-box">
          <div class="row-info-item">
            <p class="row-info-title">Restarts</p>
            <p class="row-info-value">{{ restartCount }}</p>
          </div>
          <div class="row-info-item">
            <p class="row-info-title">Ready Containers</p>
            <p class="row-info-value">{{ readyContainers?.length }}</p>
          </div>
          <div class="row-info-item">
            <p class="row-info-title">Age</p>
            <p class="row-info-value">
              {{ item.status?.startTime ? getAge(String(item.status?.startTime)) : '' }}
            </p>
          </div>
          <div class="row-info-item">
            <p class="row-info-title">Pod IP</p>
            <p class="row-info-value">{{ item.status?.podIP }}</p>
          </div>
        </div>
        <div class="mt-5 flex flex-col gap-2 px-7">
          <div class="row-info-title font-bold">Containers</div>
          <div class="flex flex-wrap gap-2">
            <div
              v-for="container in item.spec?.containers"
              :key="container.name"
              class="rounded bg-naturals-N4 p-1 px-2 text-xs text-naturals-N12"
            >
              {{ container.image }}
            </div>
          </div>
        </div>
      </template>
    </TSlideDownWrapper>
  </div>
</template>

<style scoped>
.row {
  @apply relative mb-1 flex w-full flex-col items-center border border-transparent transition-all duration-500;
  min-width: 450px;
  padding: 19px 14px 19px 8px;
  border-bottom: 1px solid rgba(39, 41, 50, var(--tw-border-opacity));
  border-radius: 4px 4px 0 0;
}

.row:last-of-type {
  border-bottom: transparent;
}
.opened {
  @apply rounded border-naturals-N5;
}

.opened:last-of-type {
  border-bottom: 1px solid rgba(44, 46, 56, var(--tw-border-opacity));
}
.row-wrapper {
  @apply flex w-full items-center justify-start;
}

.row-item {
  @apply flex items-center text-xs text-naturals-N13;
}
.row-item:nth-child(1) {
  width: 18.1%;
}
.row-item:nth-child(2) {
  width: 32.1%;
}
.row-item:nth-child(3) {
  width: 16.5%;
}
.row-item:nth-child(4) {
  width: 33%;
}
.row-item--spaced {
  @apply flex justify-between;
}
.row-arrow {
  @apply mr-1 cursor-pointer rounded fill-current text-naturals-N11 transition-all duration-300 hover:bg-naturals-N7;
  transform: rotate(-180deg);
  width: 24px;
  height: 24px;
}
.row-arrow-right--pushed {
  transform: rotate(0deg);
}
.row-action-box {
  @apply absolute right-4;
}
.row-box-actions-item {
  @apply flex w-full cursor-pointer items-center;
  padding: 17px 14px;
}
.row-box-actions-item:first-of-type {
  padding: 17px 14px 6.5px 14px;
}
.row-box-actions-item:last-of-type {
  @apply border-t border-naturals-N4;
}
.row-actions-item-icon {
  @apply h-4 w-4 fill-current transition-colors;
  margin-right: 6px;
}
.row-actions-item-icon--delete {
  @apply text-red-R1;
}
.row-actions-item-text {
  @apply text-xs transition-colors;
}
.row-actions-item-text--delete {
  @apply text-red-R1;
}

.row-box-actions-item:hover .row-actions-item-icon {
  @apply text-naturals-N12;
}
.row-box-actions-item:hover .row-actions-item-text {
  @apply text-naturals-N12;
}
.row-box-actions-item:hover .row-actions-item-icon--delete {
  @apply text-red-600;
}
.row-box-actions-item:hover .row-actions-item-text--delete {
  @apply text-red-600;
}
.row-info-box {
  @apply flex items-center;
  padding-top: 26px;
}
.row-info-item {
  @apply flex flex-col text-xs text-naturals-N13;
}
.row-info-item:nth-child(1) {
  width: 18.1%;
  padding-left: 28px;
}
.row-info-item:nth-child(2) {
  width: 32.1%;
}
.row-info-item:nth-child(3) {
  width: 16.5%;
}
.row-info-item:nth-child(4) {
  width: 33%;
}
.row-info-title {
  @apply text-xs text-naturals-N12;
  margin-bottom: 7px;
}
.row-info-value {
  @apply text-xs;
}
.box-actions-list {
  @apply z-20 flex flex-col items-start justify-center rounded border border-naturals-N4 bg-naturals-N3;
  min-width: 161px;
}
</style>
