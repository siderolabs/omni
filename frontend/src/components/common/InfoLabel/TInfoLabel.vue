<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'

import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  label?: string
  info?: string
}

const props = defineProps<Props>()

const { label, info } = toRefs(props)

const isInfoVisible = ref(false)
</script>

<template>
  <div
    class="label"
    @mouseenter="() => (isInfoVisible = true)"
    @mouseleave="() => (isInfoVisible = false)"
  >
    <div v-if="!!label" class="label__text">{{ label }}</div>
    <div class="label__icon-wrapper">
      <TIcon class="label__icon" icon="info" />
      <TAnimation>
        <div v-show="isInfoVisible" class="label__info-wrapper">
          <div class="label__info">
            {{ info }}
          </div>
        </div>
      </TAnimation>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.label {
  @apply flex cursor-default items-center justify-start;
}
.label__text {
  @apply mr-1 text-xs;
}
.label__icon {
  @apply relative mr-3 h-4 w-4 fill-current;
}
.label__icon-wrapper {
  @apply relative flex items-center justify-start;
}
.label__info {
  @apply relative flex items-center justify-center rounded border border-naturals-n4 bg-naturals-n3 px-3 py-2 text-xs text-naturals-n10;
  width: 150px;
  z-index: 0;
}
.label__info::before {
  content: '';
  @apply absolute block h-3 w-3 border border-t-0 border-r-0 border-naturals-n4 bg-naturals-n3;
  border-radius: 1px;
  transform: rotate(45deg);
  left: -6px;
  top: calc(50% - 6px);
  z-index: -1;
}
.label__info-wrapper {
  @apply absolute;
  right: -155px;
}
</style>
