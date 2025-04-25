<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="list-item-box">
    <t-slide-down-wrapper :isSliderOpened="!collapsed">
      <template #head>
        <div @click="() => { collapsed = !collapsed }"
          class="flex items-center bg-naturals-N1 pl-2 pr-4 py-4 hover:bg-naturals-N3 cursor-pointer gap-2">
          <div class="mx-2" v-if="$slots.title">
            <slot name="title"></slot>
          </div>
          <div class="expand-button" v-if="$slots.details">
            <t-icon
                :class="{'rotate-180': !collapsed}"
                class="w-5 h-5 hover:text-naturals-N13 transition-color transition-transform duration-250"
                icon="drop-up"
                />
          </div>
          <slot></slot>
        </div>
      </template>
      <template #body v-if="!collapsed && $slots.details">
        <slot name="details"></slot>
      </template>
    </t-slide-down-wrapper>
  </div>
</template>


<script setup lang="ts">
import TSlideDownWrapper from "../SlideDownWrapper/TSlideDownWrapper.vue";
import TIcon from "../Icon/TIcon.vue";

import storageRef from "@/methods/storage";

const props = defineProps<{
  listID: string
  itemID: string
  defaultOpen?: boolean
}>();

const collapsed = storageRef(sessionStorage, `${props.listID}-collapsed-${props.itemID}`, !props.defaultOpen);

defineExpose({
  collapsed,
});
</script>

<style>
.list-item-box {
  @apply border border-naturals-N5 rounded overflow-hidden;
}

.collapse-button {
  @apply fill-current text-naturals-N11 hover:bg-naturals-N7 transition-all rounded duration-300 cursor-pointer mr-1;
  transform: rotate(-180deg);
  width: 24px;
  height: 24px;
}

.collapse-button-pushed {
  transform: rotate(0);
}

.expand-button {
  @apply rounded-md bg-naturals-N4 -my-1 transition-colors duration-200 border border-transparent hover:border-naturals-N7 flex items-center justify-center;
}
</style>
