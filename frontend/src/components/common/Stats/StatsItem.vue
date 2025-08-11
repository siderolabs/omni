<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'

import { primary } from '@/vars/colors'

import type { IconType } from '../Icon/TIcon.vue'
import TIcon from '../Icon/TIcon.vue'

withDefaults(
  defineProps<{
    icon: IconType
    count: number
    units?: string
    text?: string
    pluralizedText?: string
    color?: string
    hideZero?: boolean
  }>(),
  {
    color: primary.P2,
  },
)
</script>

<template>
  <div v-if="!(hideZero && count === 0)" class="flex items-center gap-2">
    <TIcon class="h-5 w-5" :icon="icon" :style="{ color }" />
    <div class="text-md whitespace-nowrap text-naturals-N10">
      <span class="text-naturals-N13">{{ count }}{{ units }}</span>
      {{ pluralizedText ? pluralize(pluralizedText, count, false) : '' }}{{ text }}
    </div>
  </div>
</template>
