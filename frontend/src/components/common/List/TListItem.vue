<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="row" :class="{ opened: isDropdownOpened && !disableBorderOnExpand }">
    <t-slide-down-wrapper :isSliderOpened="isDropdownOpened">
      <template #head>
        <div class="flex items-center">
          <span v-if="$slots.details">
            <t-icon
              @click="() => (isDropdownOpened = !isDropdownOpened)"
              class="row-arrow"
              :class="{ pushed: isDropdownOpened }"
              icon="drop-up"
              />
          </span>
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
    </t-slide-down-wrapper>
  </div>
</template>

<script setup lang="ts">
import TIcon from "@/components/common/Icon/TIcon.vue";
import { toRefs, ref } from "vue";

import TSlideDownWrapper from "@/components/common/SlideDownWrapper/TSlideDownWrapper.vue";

const props = defineProps<{
  isDefaultOpened?: boolean,
  disableBorderOnExpand?: boolean,
}>();

const { isDefaultOpened } = toRefs(props);

const isDropdownOpened = ref(isDefaultOpened?.value as boolean);
</script>

<style scoped>
.row {
  @apply w-full border border-transparent flex flex-col items-center transition-all duration-500 text-xs text-naturals-N13 px-2 py-4;
  min-width: 450px;
  border-bottom: 1px solid rgba(39, 41, 50);
  border-radius: 4px 4px 0 0;
}

.row-head {
  @apply px-1 flex-1;
}

.row:last-of-type {
  border-bottom: transparent;
}

.opened {
  @apply rounded border-naturals-N5 mt-1;
}

.opened:last-of-type {
  border-bottom: 1px solid rgba(44, 46, 56, var(--tw-border-opacity));
}

.row-wrapper {
  @apply w-full flex justify-start items-center;
}

.row-arrow {
  @apply fill-current text-naturals-N11 hover:bg-naturals-N7 transition-all rounded duration-300 cursor-pointer mr-1;
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
