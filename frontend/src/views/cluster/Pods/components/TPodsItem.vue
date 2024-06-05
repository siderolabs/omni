<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="row" :class="{ opened: isDropdownOpened }">
    <t-slide-down-wrapper :isSliderOpened="isDropdownOpened">
      <template v-slot:head
        ><ul class="row-wrapper">
          <li class="row-item">
            <t-icon
              @click="() => (isDropdownOpened = !isDropdownOpened)"
              class="row-arrow"
              :class="{ 'row-arrow-right--pushed': isDropdownOpened }"
              icon="drop-up"
            />
            <span>
              <WordHighlighter
                :query="searchOption"
                :textToHighlight="item.metadata?.namespace"
                highlightClass="bg-naturals-N14"
              />
            </span>
          </li>
          <li class="row-item">
            <WordHighlighter
              :query="searchOption"
              :textToHighlight="item.metadata?.name"
              highlightClass="bg-naturals-N14"
            />
          </li>
          <li class="row-item">
            <t-status :title="item.status?.phase" />
          </li>
          <li class="row-item row-item--spaced">
            <span>
              <WordHighlighter
                :query="searchOption"
                :textToHighlight="item.spec?.nodeName"
                highlightClass="bg-naturals-N14"
            /></span>
          </li>
        </ul>
      </template>
      <template v-slot:body>
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
            <p class="row-info-value">{{ item.status?.startTime ? getAge(item.status?.startTime) : "" }}</p>
          </div>
          <div class="row-info-item">
            <p class="row-info-title">Pod IP</p>
            <p class="row-info-value">{{ item.status?.podIP }}</p>
          </div>
        </div>
        <div class="flex flex-col gap-2 px-7 mt-5">
          <div class="row-info-title font-bold">
            Containers
          </div>
          <div class="flex flex-wrap gap-2">
            <div class="text-xs text-naturals-N12 rounded bg-naturals-N4 p-1 px-2" v-for="container in item.spec?.containers" :key="container.name">
              {{ container.image }}
            </div>
          </div>
        </div>
      </template>
    </t-slide-down-wrapper>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRefs } from "vue";
import { DateTime } from "luxon";
import { V1Pod } from "@kubernetes/client-node";

import TIcon from "@/components/common/Icon/TIcon.vue";
import TStatus from "@/components/common/Status/TStatus.vue";
import TSlideDownWrapper from "@/components/common/SlideDownWrapper/TSlideDownWrapper.vue";
import WordHighlighter from "vue-word-highlighter";

type Props = {
  searchOption: string;
  item: V1Pod,
};

const props = defineProps<Props>()

const { item } = toRefs(props);

const isDropdownOpened = ref(false);

const readyContainers = computed(() =>
  item.value?.status?.containerStatuses?.filter((item: any) => item?.ready === true)
);

const restartCount = computed(() =>
  item.value?.status?.containerStatuses?.reduce((amount, reducer: any) => {
    return amount + reducer.restartCount;
  }, 0)
);

const getAge = (age: string) => {
  const currentDate = DateTime.now();
  const currentAge = DateTime.fromISO(age);

  return currentDate
    .diff(currentAge, ["days", "hours", "minutes"])
    .toFormat("dd'd' hh'h' mm'm'");
};
</script>

<style scoped>
.row {
  @apply relative w-full border border-transparent  flex flex-col items-center mb-1 transition-all duration-500;
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
  @apply w-full flex justify-start items-center;
}

.row-item {
  @apply text-xs text-naturals-N13 flex items-center;
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
  @apply fill-current text-naturals-N11 hover:bg-naturals-N7 transition-all rounded duration-300 cursor-pointer mr-1;
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
  @apply flex items-center cursor-pointer w-full;
  padding: 17px 14px;
}
.row-box-actions-item:first-of-type {
  padding: 17px 14px 6.5px 14px;
}
.row-box-actions-item:last-of-type {
  @apply border-t border-naturals-N4;
}
.row-actions-item-icon {
  @apply w-4 h-4 fill-current transition-colors;
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
  @apply text-xs text-naturals-N13 flex flex-col;
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
  @apply bg-naturals-N3 rounded border border-naturals-N4 flex justify-center flex-col items-start z-20;
  min-width: 161px;
}
</style>
