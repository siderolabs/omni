<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T = unknown">
import { computed, ref, toRefs, watch } from 'vue'

import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  items: T[]
  perPage?: number
  searchOption: string
}

const props = withDefaults(defineProps<Props>(), {
  perPage: 8,
})

const { items, perPage, searchOption } = toRefs(props)
const DOTS = '...'
const currentPage = ref(1)
const totalPageCount = computed(() => {
  return Math.ceil(items.value.length / perPage.value)
})
const range = (start, end) => {
  const length = end - start + 1
  return Array.from({ length }, (_, idx) => idx + start)
}
const siblingCount = ref(1)
const paginationRange = computed(() => {
  const totalPageNumbers = siblingCount.value + 5
  if (totalPageNumbers >= totalPageCount.value) {
    return range(1, totalPageCount.value)
  }
  const leftSiblingIndex = Math.max(currentPage.value - siblingCount.value, 1)
  const rightSiblingIndex = Math.min(currentPage.value + siblingCount.value, totalPageCount.value)
  const shouldShowLeftDots = leftSiblingIndex > 2
  const shouldShowRightDots = rightSiblingIndex < totalPageCount.value - 2
  const firstPageIndex = 1
  const lastPageIndex = totalPageCount.value
  if (!shouldShowLeftDots && shouldShowRightDots) {
    const leftItemCount = 3 + 2 * siblingCount.value
    const leftRange = range(1, leftItemCount)
    return [...leftRange, DOTS, totalPageCount.value]
  }
  if (shouldShowLeftDots && !shouldShowRightDots) {
    const rightItemCount = 3 + 2 * siblingCount.value
    const rightRange = range(totalPageCount.value - rightItemCount + 1, totalPageCount.value)
    return [firstPageIndex, DOTS, ...rightRange]
  }
  if (shouldShowLeftDots && shouldShowRightDots) {
    const middleRange = range(leftSiblingIndex, rightSiblingIndex)
    return [firstPageIndex, DOTS, ...middleRange, DOTS, lastPageIndex]
  }

  return []
})
const lastPage = computed(() => {
  return paginationRange.value && paginationRange.value[paginationRange.value.length - 1]
})

const onNext = () => {
  if (currentPage.value === lastPage.value) return
  currentPage.value += 1
}

const onPrevious = () => {
  if (currentPage.value === 1) return
  currentPage.value -= 1
}
const onPageClick = (value) => {
  if (value !== DOTS) {
    currentPage.value = value
  }
}
watch(
  () => searchOption.value,
  () => {
    currentPage.value = 1
  },
)

const isInvisible = computed(() => {
  return (
    !totalPageCount.value ||
    currentPage.value === 0 ||
    (paginationRange.value && paginationRange.value.length < 2)
  )
})

const filteredItems = computed(() => {
  return items.value.slice((currentPage.value - 1) * perPage.value).slice(0, perPage.value)
})
</script>

<template>
  <slot :paginated-items="filteredItems" />
  <div class="pagination" :style="{ opacity: isInvisible ? 0 : 1 }">
    <TIcon
      icon="arrow-left"
      class="pagination__icon"
      :class="{ 'pagination__icon--passive': currentPage === 1 }"
      @click="onPrevious"
    />

    <div class="pagination__pages">
      <span
        v-for="item in paginationRange || []"
        :key="item"
        class="pagination__page-number"
        :class="{
          'pagination__page-number--active': item === currentPage,
          unhovered: item === DOTS,
        }"
        @click="() => onPageClick(item)"
      >
        {{ item }}
      </span>
    </div>
    <TIcon
      icon="arrow-right"
      class="pagination__icon"
      :class="{ 'pagination__icon--passive': currentPage === totalPageCount }"
      @click="onNext"
    />
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.pagination {
  @apply flex items-center justify-end pt-6;
}
.pagination__icon {
  @apply cursor-pointer fill-current text-naturals-n8 transition-all duration-200 hover:text-naturals-n10;
  width: 18px;
  height: 18px;
}
.pagination__icon--passive {
  @apply text-naturals-n6;
}
.pagination__icon:nth-child(1) {
  margin-right: 20px;
}
.pagination__pages {
  @apply flex items-center transition-all duration-200;
  margin-right: 20px;
}
.pagination__page-number {
  @apply flex h-7 w-7 cursor-pointer items-center justify-center rounded text-naturals-n8 transition-all duration-200 hover:text-naturals-n9;
  margin-right: 20px;
}
.unhovered {
  @apply cursor-default hover:text-naturals-n8;
}
.pagination__page-number:last-of-type {
  margin: 0;
}
.pagination__page-number--active {
  @apply bg-naturals-n4 text-naturals-n12;
}
</style>
