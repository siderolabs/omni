<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { ClassValue } from 'vue'

import { cn } from '@/methods/utils'

defineProps<{
  name: string
  value?: string | number | boolean
  class?: ClassValue
}>()
</script>

<template>
  <div :class="cn('flex w-full items-center justify-between gap-4', $props.class)">
    <div class="min-w-10 text-xs text-naturals-n11">{{ name }}</div>
    <div class="flex items-center gap-1 text-xs text-naturals-n13">
      <span
        v-if="value"
        class="mr-0.5 text-right break-all [&_svg]:mr-0.5 [&_svg]:text-right [&_svg]:break-all"
      >
        {{ value }}
      </span>

      <slot />
    </div>
  </div>
</template>
