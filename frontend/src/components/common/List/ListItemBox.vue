<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'

import TIcon from '../Icon/TIcon.vue'
import TSlideDownWrapper from '../SlideDownWrapper/TSlideDownWrapper.vue'

const props = defineProps<{
  listID: string
  itemID: string
  regionId?: string
  labelId?: string
  defaultOpen?: boolean
}>()

const expanded = useSessionStorage(`${props.listID}-expanded-${props.itemID}`, props.defaultOpen)

defineExpose({
  collapsed: !expanded,
})
</script>

<template>
  <div class="list-item-box">
    <TSlideDownWrapper :is-slider-opened="expanded">
      <template #head>
        <div
          tabindex="0"
          role="button"
          :aria-expanded="expanded"
          :aria-controls="regionId"
          :aria-labelledby="labelId"
          class="flex cursor-pointer items-center gap-2 bg-naturals-n1 py-4 pr-4 pl-2 hover:bg-naturals-n3"
          @click="expanded = !expanded"
        >
          <div v-if="$slots.details" class="expand-button">
            <TIcon
              :class="{ 'rotate-180': expanded }"
              class="transition-color h-5 w-5 transition-transform duration-250 hover:text-naturals-n13"
              icon="drop-up"
              aria-hidden="true"
            />
          </div>
          <slot></slot>
        </div>
      </template>
      <template v-if="expanded && $slots.details" #body>
        <slot name="details"></slot>
      </template>
    </TSlideDownWrapper>
  </div>
</template>

<style>
@reference "../../../index.css";

.list-item-box {
  @apply overflow-hidden rounded border border-naturals-n5;
}

.collapse-button {
  @apply mr-1 cursor-pointer rounded fill-current text-naturals-n11 transition-all duration-300 hover:bg-naturals-n7;
  transform: rotate(-180deg);
  width: 24px;
  height: 24px;
}

.collapse-button-pushed {
  transform: rotate(0);
}

.expand-button {
  @apply -my-1 flex items-center justify-center rounded-md border border-transparent bg-naturals-n4 transition-colors duration-200 hover:border-naturals-n7;
}
</style>
