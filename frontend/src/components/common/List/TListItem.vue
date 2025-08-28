<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'

import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSlideDownWrapper from '@/components/common/SlideDownWrapper/TSlideDownWrapper.vue'

const selected = defineModel<boolean>('selected')

const props = defineProps<{
  isSelectable?: boolean
  isDefaultOpened?: boolean
  disableBorderOnExpand?: boolean
}>()

const { isDefaultOpened } = toRefs(props)

const isDropdownOpened = ref(isDefaultOpened?.value as boolean)
</script>

<template>
  <div class="row" :class="{ opened: isDropdownOpened && !disableBorderOnExpand }">
    <TSlideDownWrapper :is-slider-opened="isDropdownOpened">
      <template #head>
        <div class="flex gap-1" :class="{ 'items-center': !isSelectable }">
          <div
            v-if="$slots.details"
            class="flex flex-col items-center gap-2"
            :class="{ 'mt-1.5': isSelectable }"
          >
            <TCheckbox v-if="isSelectable" :checked="selected" @click="selected = !selected" />

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
      </template>
      <template #body>
        <div class="row-details">
          <slot name="details"></slot>
        </div>
      </template>
    </TSlideDownWrapper>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.row {
  @apply flex w-full flex-col border border-transparent px-2 py-4 text-xs text-naturals-n13 transition-all duration-500;
  border-bottom: 1px solid rgba(39, 41, 50);
  border-radius: 4px 4px 0 0;
}

.row-head {
  @apply min-w-0 flex-1 px-1;
}

.row:last-of-type {
  border-bottom: transparent;
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
