<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  icon: IconType
  danger?: boolean
}

withDefaults(defineProps<Props>(), {})
</script>

<template>
  <div
    class="flex w-full cursor-pointer items-center gap-2 px-3 py-2 *:hover:text-naturals-n12"
    :class="{ 'text-red-r1 *:hover:text-primary-p1': danger }"
  >
    <TIcon class="h-4 w-4 transition-colors" :icon="icon" />
    <span class="text-xs transition-colors">
      <slot />
    </span>
  </div>
</template>
