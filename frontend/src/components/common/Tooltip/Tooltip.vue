<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <popper
    :offsetDistance="offsetDistance.toFixed()"
    :offsetSkid="offsetSkid.toFixed()"
    :placement="placement"
    :show="show && (!!description || !!$slots.description)"
    class="popper"
  >
    <template #content>
      <div
        class="text-xs bg-naturals-N3 border border-naturals-N4 rounded p-4 text-naturals-N12 z-50"
      >
        <p v-if="description" class="whitespace-pre">{{ description }}</p>
        <slot v-else name="description" />
      </div>
    </template>
    <div @mouseover="show = true" @mouseleave="show = false">
      <slot />
    </div>
  </popper>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Popper from 'vue3-popper'

type props = {
  description?: string
  offsetDistance?: number
  offsetSkid?: number
  placement?:
    | 'auto'
    | 'auto-start'
    | 'auto-end'
    | 'top'
    | 'top-start'
    | 'top-end'
    | 'bottom'
    | 'bottom-start'
    | 'bottom-end'
    | 'right'
    | 'right-start'
    | 'right-end'
    | 'left'
    | 'left-start'
    | 'left-end'
}

withDefaults(defineProps<props>(), {
  placement: 'auto-start',
  offsetDistance: 10,
  offsetSkid: 30,
})

const show = ref(false)
</script>

<style scoped>
.popper {
  margin: 0 !important;
  border: 0 !important;
  display: block !important;
  z-index: auto !important;
}
</style>
