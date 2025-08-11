<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

defineProps<{
  label: {
    key: string
    id: string
    value: string
    color: string
    description?: string
    removable?: boolean
    icon?: string
  }
  removeLabel?: (key: string) => Promise<void>
}>()
</script>

<template>
  <component
    :is="label.description ? Tooltip : 'div'"
    :description="label.description"
    placement="bottom-start"
  >
    <span
      class="flex cursor-pointer items-center transition-all"
      :class="`resource-label label-${label.color}`"
      @click.stop="() => $emit('filterLabel', label)"
    >
      <TIcon v-if="label.icon" :icon="label.icon as IconType" class="-ml-1 mr-1 h-3.5 w-3.5" />
      <template v-if="label.value">
        {{ label.id }}:<span class="font-semibold">{{ label.value }}</span>
      </template>
      <span v-else class="font-semibold">
        {{ label.id }}
      </span>
      <TIcon
        v-if="label.removable && removeLabel"
        icon="close"
        class="destroy-label-button"
        @click.stop="() => removeLabel?.(label.key)"
      />
    </span>
  </component>
</template>

<style scoped>
.label-green {
  --label-h: 142;
  --label-s: 100;
  --label-l: 49;
}

.label-red {
  --label-h: 359;
  --label-s: 97;
  --label-l: 36;
}

.label-orange {
  --label-h: 15;
  --label-s: 90;
  --label-l: 44;
}

.label-violet {
  --label-h: 313;
  --label-s: 97;
  --label-l: 58;
}

.label-yellow {
  --label-h: 48;
  --label-s: 96;
  --label-l: 50;
}

.label-cyan {
  --label-h: 185;
  --label-s: 100;
  --label-l: 22;
}

.label-blue1 {
  --label-h: 211;
  --label-s: 76;
  --label-l: 48;
}

.label-blue2 {
  --label-h: 215;
  --label-s: 100;
  --label-l: 40;
}

.label-blue3 {
  --label-h: 256;
  --label-s: 81;
  --label-l: 50;
}

.label-light1 {
  --label-h: 0;
  --label-s: 65;
  --label-l: 74;
  --lighten-by: 0;
}

.label-light2 {
  --label-h: 13;
  --label-s: 81;
  --label-l: 87;
  --lighten-by: 0;
}

.label-light3 {
  --label-h: 48;
  --label-s: 96;
  --label-l: 87;
  --lighten-by: 0;
}

.label-light4 {
  --label-h: 128;
  --label-s: 32;
  --label-l: 81;
  --lighten-by: 0;
}

.label-light5 {
  --label-h: 184;
  --label-s: 29;
  --label-l: 80;
  --lighten-by: 0;
}

.label-light6 {
  --label-h: 208;
  --label-s: 70;
  --label-l: 86;
  --lighten-by: 0;
}

.label-light7 {
  --label-h: 257;
  --label-s: 81;
  --label-l: 87;
  --lighten-by: 0;
}

.destroy-label-button {
  @apply -mr-1 ml-1 inline-block h-3 w-3 cursor-pointer rounded-full transition-all hover:bg-naturals-N14 hover:text-naturals-N1;
}
</style>
