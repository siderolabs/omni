<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <component :is="is" class="tab" :class="{ active: selected }">
    <slot/>
  </component>
</template>

<script setup lang="ts">
import { toRefs } from "vue";

type Props = {
  selected: boolean;
  is?: string
};

const props = withDefaults(defineProps<Props>(),
  {
    is: "div"
  },
);
const { selected } = toRefs(props);
</script>

<style scoped>
.tab {
  @apply text-sm hover:text-naturals-N13 transition-colors focus:outline-none outline-none;
}

.active {
  @apply text-naturals-N13 relative focus:outline-none outline-none;
}

.active::before {
  @apply block absolute bg-primary-P3 w-full animate-fadein;
  content: "";
  height: 2px;
  bottom: -15px;
}
</style>
