<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'
import TSlideDownWrapper from '@/components/SlideDownWrapper/TSlideDownWrapper.vue'

const { isDefaultOpened } = defineProps<{
  isDefaultOpened?: boolean
  disableBorderOnExpand?: boolean
}>()

defineSlots<{
  default(): unknown
  secondary(): unknown
  details(): unknown
}>()

const isDropdownOpened = ref(isDefaultOpened)
</script>

<template>
  <div
    class="flex w-full flex-col border-b border-naturals-n5 px-2 py-4 text-xs text-naturals-n13 transition-all duration-500"
    :class="
      isDropdownOpened && !disableBorderOnExpand
        ? 'mt-1 rounded last-of-type:border-naturals-n6'
        : 'rounded-t-sm last-of-type:border-none'
    "
    role="row"
  >
    <div class="flex flex-col gap-1">
      <div class="flex items-center gap-1">
        <TIcon
          v-if="$slots.details"
          class="size-6 cursor-pointer rounded fill-current text-naturals-n11 transition-all duration-300 hover:bg-naturals-n7"
          :class="isDropdownOpened ? 'rotate-0' : '-rotate-180'"
          icon="drop-up"
          @click="isDropdownOpened = !isDropdownOpened"
        />

        <div class="min-w-0 flex-1 px-1">
          <slot></slot>
        </div>
      </div>

      <slot name="secondary"></slot>
    </div>

    <TSlideDownWrapper :expanded="isDropdownOpened">
      <div class="p-2">
        <slot name="details"></slot>
      </div>
    </TSlideDownWrapper>
  </div>
</template>
