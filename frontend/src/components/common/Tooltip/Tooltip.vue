<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
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

<template>
  <Popper
    :offset-distance="offsetDistance.toFixed()"
    :offset-skid="offsetSkid.toFixed()"
    :placement="placement"
    :show="show && (!!description || !!$slots.description)"
    class="popper"
  >
    <template #content>
      <div
        class="z-50 rounded border border-naturals-N4 bg-naturals-N3 p-4 text-xs text-naturals-N12"
      >
        <p v-if="description" class="whitespace-pre">{{ description }}</p>
        <slot v-else name="description" />
      </div>
    </template>
    <div @mouseover="show = true" @mouseleave="show = false">
      <slot />
    </div>
  </Popper>
</template>

<style scoped>
.popper {
  margin: 0 !important;
  border: 0 !important;
  display: block !important;
  z-index: auto !important;
}
</style>
