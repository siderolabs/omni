<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, onUnmounted, ref, toRefs, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    isSliderOpened?: boolean
  }>(),
  {},
)

const { isSliderOpened } = toRefs(props)

const body: Ref<Element | undefined> = ref()

const resizeObserver = new ResizeObserver(function () {
  elementHeight.value = body.value?.clientHeight ?? 0
})

onUnmounted(() => {
  resizeObserver.disconnect()
})

const elementHeight = ref(0)

watch(
  () => body.value,
  () => {
    resizeObserver.disconnect()

    if (!body.value) {
      return
    }

    resizeObserver.observe(body.value)
  },
)

const height = computed(() => {
  return isSliderOpened.value ? `${elementHeight.value}px` : '0'
})
</script>

<template>
  <div class="wrapper">
    <div class="wrapper-box">
      <div class="wrapper-head">
        <slot name="head" />
      </div>
      <div class="wrapper-body-box">
        <div class="wrapper-body" :style="{ height }">
          <div ref="body">
            <slot name="body" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.wrapper {
  @apply flex w-full flex-col;
}

.wrapper-body {
  @apply overflow-hidden transition-all duration-300 ease-in-out;
}
.wrapper-body-box {
  position: relative;
}
</style>
