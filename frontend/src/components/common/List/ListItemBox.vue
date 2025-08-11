<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import storageRef from '@/methods/storage'

import TIcon from '../Icon/TIcon.vue'
import TSlideDownWrapper from '../SlideDownWrapper/TSlideDownWrapper.vue'

const props = defineProps<{
  listID: string
  itemID: string
  defaultOpen?: boolean
}>()

const collapsed = storageRef(
  sessionStorage,
  `${props.listID}-collapsed-${props.itemID}`,
  !props.defaultOpen,
)

defineExpose({
  collapsed,
})
</script>

<template>
  <div class="list-item-box">
    <TSlideDownWrapper :is-slider-opened="!collapsed">
      <template #head>
        <div
          class="flex cursor-pointer items-center gap-2 bg-naturals-N1 py-4 pl-2 pr-4 hover:bg-naturals-N3"
          @click="
            () => {
              collapsed = !collapsed
            }
          "
        >
          <div v-if="$slots.title" class="mx-2">
            <slot name="title"></slot>
          </div>
          <div v-if="$slots.details" class="expand-button">
            <TIcon
              :class="{ 'rotate-180': !collapsed }"
              class="transition-color duration-250 h-5 w-5 transition-transform hover:text-naturals-N13"
              icon="drop-up"
            />
          </div>
          <slot></slot>
        </div>
      </template>
      <template v-if="!collapsed && $slots.details" #body>
        <slot name="details"></slot>
      </template>
    </TSlideDownWrapper>
  </div>
</template>

<style>
.list-item-box {
  @apply overflow-hidden rounded border border-naturals-N5;
}

.collapse-button {
  @apply mr-1 cursor-pointer rounded fill-current text-naturals-N11 transition-all duration-300 hover:bg-naturals-N7;
  transform: rotate(-180deg);
  width: 24px;
  height: 24px;
}

.collapse-button-pushed {
  transform: rotate(0);
}

.expand-button {
  @apply -my-1 flex items-center justify-center rounded-md border border-transparent bg-naturals-N4 transition-colors duration-200 hover:border-naturals-N7;
}
</style>
