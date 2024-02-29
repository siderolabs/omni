<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="wrapper">
    <div class="wrapper-box">
      <div class="wrapper-head">
        <slot name="head"/>
      </div>
      <div class="wrapper-body-box">
        <div
          class="wrapper-body"
          :style="{ height }"
        >
          <div ref="body">
            <slot name="body"/>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Ref, ref, toRefs, computed, watch, onUnmounted } from "vue";

const props = withDefaults(defineProps<{
  isSliderOpened?: boolean,
}>(), {
  isSliderOpened: false
});

const { isSliderOpened } = toRefs(props);

const body: Ref<Element | undefined> = ref();


const resizeObserver = new ResizeObserver(function() {
  elementHeight.value = body.value?.clientHeight ?? 0;
});

onUnmounted(() => {
  resizeObserver.disconnect();
});

const elementHeight = ref(0);

watch(() => body.value, () => {
  resizeObserver.disconnect();

  if (!body.value) {
    return;
  }

  resizeObserver.observe(body.value);
});

const height = computed(() => {
  return isSliderOpened.value ? `${elementHeight.value}px` : '0';
});
</script>

<style scoped>
.wrapper {
  @apply w-full flex flex-col;
}

.wrapper-body {
  @apply overflow-hidden transition-all duration-300 ease-in-out;
}
.wrapper-body-box {
  position: relative;
}
</style>
