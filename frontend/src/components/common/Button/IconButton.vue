<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { type ButtonHTMLAttributes } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

interface Props extends /* @vue-ignore */ ButtonHTMLAttributes {
  icon?: IconType
  danger?: boolean
  iconClasses?: unknown
}

defineProps<Props>()
</script>

<template>
  <button
    class="h-6 w-6 rounded align-top text-naturals-n11 transition-all duration-100 hover:bg-naturals-n7 hover:text-naturals-n13 disabled:pointer-events-none disabled:cursor-not-allowed disabled:text-naturals-n7"
    :class="{ 'text-red-r1': danger }"
  >
    <slot>
      <TIcon class="h-full w-full cursor-pointer p-1" :class="iconClasses" :icon="icon" />
    </slot>
  </button>
</template>
