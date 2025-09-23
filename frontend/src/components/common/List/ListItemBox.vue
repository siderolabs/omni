<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'

import TIcon from '../Icon/TIcon.vue'
import TSlideDownWrapper from '../SlideDownWrapper/TSlideDownWrapper.vue'

const { listID, itemID } = defineProps<{
  listID: string
  itemID: string
}>()

const expanded = useSessionStorage(`${listID}-expanded-${itemID}`, false)
</script>

<template>
  <div class="overflow-hidden rounded border border-naturals-n5">
    <div
      tabindex="0"
      role="button"
      :aria-expanded="expanded"
      class="flex cursor-pointer items-center gap-2 bg-naturals-n1 py-4 pr-4 pl-2 hover:bg-naturals-n3"
      @click="expanded = !expanded"
    >
      <div
        v-if="$slots.details"
        class="-my-1 flex items-center justify-center rounded-md border border-transparent bg-naturals-n4 transition-colors duration-200 hover:border-naturals-n7"
      >
        <TIcon
          :class="{ 'rotate-180': expanded }"
          class="transition-color h-5 w-5 transition-transform duration-250 hover:text-naturals-n13"
          icon="drop-up"
          aria-hidden="true"
        />
      </div>
      <slot></slot>
    </div>

    <TSlideDownWrapper :expanded="expanded">
      <slot name="details"></slot>
    </TSlideDownWrapper>
  </div>
</template>
