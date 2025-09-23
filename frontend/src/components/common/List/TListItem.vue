<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'

import TIcon from '@/components/common/Icon/TIcon.vue'
import TSlideDownWrapper from '@/components/common/SlideDownWrapper/TSlideDownWrapper.vue'

const props = defineProps<{
  isDefaultOpened?: boolean
  disableBorderOnExpand?: boolean
}>()

const { isDefaultOpened } = toRefs(props)

const isDropdownOpened = ref(isDefaultOpened?.value as boolean)
</script>

<template>
  <div class="row" :class="{ opened: isDropdownOpened && !disableBorderOnExpand }">
    <div class="flex items-center gap-1">
      <div v-if="$slots.details" class="flex flex-col items-center gap-2">
        <TIcon
          class="row-arrow"
          :class="{ pushed: isDropdownOpened }"
          icon="drop-up"
          @click="() => (isDropdownOpened = !isDropdownOpened)"
        />
      </div>

      <div class="row-head">
        <slot></slot>
      </div>
    </div>

    <TSlideDownWrapper :expanded="isDropdownOpened">
      <div class="row-details">
        <slot name="details"></slot>
      </div>
    </TSlideDownWrapper>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.row {
  @apply flex w-full flex-col rounded-t-sm border border-transparent border-b-naturals-n5 px-2 py-4 text-xs text-naturals-n13 transition-all duration-500 last-of-type:border-b-transparent;
}

.row-head {
  @apply min-w-0 flex-1 px-1;
}

.opened {
  @apply mt-1 rounded border-naturals-n5;
}

.opened:last-of-type {
  @apply border-b border-solid border-b-naturals-n6;
}

.row-wrapper {
  @apply flex w-full items-center justify-start;
}

.row-arrow {
  @apply cursor-pointer rounded fill-current text-naturals-n11 transition-all duration-300 hover:bg-naturals-n7;
  transform: rotate(-180deg);
  width: 24px;
  height: 24px;
}

.pushed {
  transform: rotate(0deg);
}

.row-details {
  @apply p-2;
}
</style>
