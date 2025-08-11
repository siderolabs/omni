<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { toRefs } from 'vue'

type Props = {
  selected: boolean
  is?: string
}

const props = withDefaults(defineProps<Props>(), {
  is: 'div',
})
const { selected } = toRefs(props)
</script>

<template>
  <component :is="is" class="tab" :class="{ active: selected }">
    <slot />
  </component>
</template>

<style scoped>
@reference "../../../index.css";

.tab {
  @apply text-sm outline-hidden transition-colors hover:text-naturals-n13 focus:outline-hidden;
}

.active {
  @apply relative text-naturals-n13 outline-hidden focus:outline-hidden;
}

.active::before {
  @apply absolute block w-full animate-fadein bg-primary-p3;
  content: '';
  height: 2px;
  bottom: -15px;
}
</style>
